package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

type AssessmentGroupPostgreSQL struct {
	db *gorm.DB
}

func NewAssessmentGroupPostgreSQL(db *gorm.DB) repositories.AssessmentGroupRepository {
	return &AssessmentGroupPostgreSQL{db: db}
}

func (ag *AssessmentGroupPostgreSQL) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return ag.db
}

// ===== ASSIGNMENT OPERATIONS =====

// AssignToGroups - Bulk assignment with duplicate prevention
func (ag *AssessmentGroupPostgreSQL) AssignToGroups(ctx context.Context, tx *gorm.DB, assessmentID uint, groupIDs []uint, assignedBy string) error {
	if len(groupIDs) == 0 {
		return nil
	}

	db := ag.getDB(tx)

	// Check for existing assignments to prevent duplicates
	var existingAssignments []models.AssessmentGroup
	err := db.WithContext(ctx).
		Where("assessment_id = ? AND group_id IN ?", assessmentID, groupIDs).
		Find(&existingAssignments).Error
	if err != nil {
		return fmt.Errorf("failed to check existing assignments: %w", err)
	}

	// Filter out groups that are already assigned
	existingGroupIDs := make(map[uint]bool)
	for _, assignment := range existingAssignments {
		existingGroupIDs[assignment.GroupID] = true
	}

	newGroupIDs := []uint{}
	for _, groupID := range groupIDs {
		if !existingGroupIDs[groupID] {
			newGroupIDs = append(newGroupIDs, groupID)
		}
	}

	if len(newGroupIDs) == 0 {
		return fmt.Errorf("all groups are already assigned to this assessment")
	}

	// Create new assignments
	assignments := make([]models.AssessmentGroup, len(newGroupIDs))
	now := time.Now()
	for i, groupID := range newGroupIDs {
		assignments[i] = models.AssessmentGroup{
			AssessmentID: assessmentID,
			GroupID:      groupID,
			AssignedAt:   now,
			AssignedBy:   assignedBy,
		}
	}

	if err := db.WithContext(ctx).CreateInBatches(assignments, 100).Error; err != nil {
		return fmt.Errorf("failed to create assignments: %w", err)
	}

	return nil
}

// UnassignFromGroups - Soft delete for audit trail
func (ag *AssessmentGroupPostgreSQL) UnassignFromGroups(ctx context.Context, tx *gorm.DB, assessmentID uint, groupIDs []uint) error {
	if len(groupIDs) == 0 {
		return nil
	}

	db := ag.getDB(tx)

	result := db.WithContext(ctx).
		Where("assessment_id = ? AND group_id IN ?", assessmentID, groupIDs).
		Delete(&models.AssessmentGroup{})

	if result.Error != nil {
		return fmt.Errorf("failed to unassign groups: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no assignments found to remove")
	}

	return nil
}

// AssignToGroup assigns an assessment to a single group
func (ag *AssessmentGroupPostgreSQL) AssignToGroup(ctx context.Context, tx *gorm.DB, assignment *models.AssessmentGroup) error {
	db := ag.getDB(tx)

	// Check if already assigned
	isAssigned, err := ag.IsAssigned(ctx, tx, assignment.AssessmentID, assignment.GroupID)
	if err != nil {
		return err
	}
	if isAssigned {
		return fmt.Errorf("assessment %d is already assigned to group %d", assignment.AssessmentID, assignment.GroupID)
	}

	if err := db.WithContext(ctx).Create(assignment).Error; err != nil {
		return fmt.Errorf("failed to create assignment: %w", err)
	}

	return nil
}

// UnassignFromGroup removes an assessment from a single group
func (ag *AssessmentGroupPostgreSQL) UnassignFromGroup(ctx context.Context, tx *gorm.DB, assessmentID, groupID uint) error {
	db := ag.getDB(tx)

	result := db.WithContext(ctx).
		Where("assessment_id = ? AND group_id = ?", assessmentID, groupID).
		Delete(&models.AssessmentGroup{})

	if result.Error != nil {
		return fmt.Errorf("failed to unassign group: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("assignment not found")
	}

	return nil
}

// ===== QUERY OPERATIONS =====

// GetStudentAssessments - KEY METHOD for PRIVATE visibility
// Returns assessments that student can see based on their group memberships
func (ag *AssessmentGroupPostgreSQL) GetStudentAssessments(ctx context.Context, tx *gorm.DB, studentID string, statusFilter models.AssessmentStatus) ([]*models.Assessment, error) {
	db := ag.getDB(tx)

	query := db.WithContext(ctx).
		Table("assessments a").
		Select("DISTINCT a.*").
		Joins("INNER JOIN assessment_groups ag ON ag.assessment_id = a.id AND ag.deleted_at IS NULL").
		Joins("INNER JOIN group_members gm ON gm.group_id = ag.group_id AND gm.deleted_at IS NULL").
		Where("gm.user_id = ?", studentID)

	// Apply status filter
	if statusFilter != "" {
		query = query.Where("a.status = ?", statusFilter)
	} else {
		// Default: only show Active assessments
		query = query.Where("a.status = ?", models.StatusActive)
	}

	// Show all assessments including expired ones
	// Expired status will be calculated at service layer

	var assessments []*models.Assessment
	if err := query.Order("a.created_at DESC").Find(&assessments).Error; err != nil {
		return nil, fmt.Errorf("failed to get student assessments: %w", err)
	}

	return assessments, nil
}

// GetGroupsByAssessment retrieves all groups assigned to an assessment
func (ag *AssessmentGroupPostgreSQL) GetGroupsByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.Group, error) {
	db := ag.getDB(tx)

	var groups []*models.Group
	err := db.WithContext(ctx).
		Table("groups g").
		Joins("INNER JOIN assessment_groups ag ON ag.group_id = g.id AND ag.deleted_at IS NULL").
		Where("ag.assessment_id = ?", assessmentID).
		Order("g.name ASC").
		Find(&groups).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get groups for assessment: %w", err)
	}

	return groups, nil
}

// GetGroupsByAssessmentWithDetails retrieves groups with full AssessmentGroup details
func (ag *AssessmentGroupPostgreSQL) GetGroupsByAssessmentWithDetails(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.AssessmentGroup, error) {
	db := ag.getDB(tx)

	var assignments []*models.AssessmentGroup
	err := db.WithContext(ctx).
		Preload("Group").
		Where("assessment_id = ?", assessmentID).
		Order("assigned_at DESC").
		Find(&assignments).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get groups with details for assessment: %w", err)
	}

	return assignments, nil
}

// GetAssessmentsByGroup retrieves all assessments assigned to a group
func (ag *AssessmentGroupPostgreSQL) GetAssessmentsByGroup(ctx context.Context, tx *gorm.DB, groupID uint) ([]*models.Assessment, error) {
	db := ag.getDB(tx)

	var assessments []*models.Assessment
	err := db.WithContext(ctx).
		Table("assessments a").
		Joins("INNER JOIN assessment_groups ag ON ag.assessment_id = a.id AND ag.deleted_at IS NULL").
		Where("ag.group_id = ?", groupID).
		Order("a.created_at DESC").
		Find(&assessments).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get assessments for group: %w", err)
	}

	return assessments, nil
}

// GetAssessmentsByGroupWithDetails retrieves assessments with assignment metadata
func (ag *AssessmentGroupPostgreSQL) GetAssessmentsByGroupWithDetails(ctx context.Context, tx *gorm.DB, groupID uint) ([]*models.AssessmentGroup, error) {
	db := ag.getDB(tx)

	var assignments []*models.AssessmentGroup
	err := db.WithContext(ctx).
		Preload("Assessment").
		Preload("Assessment.Settings").
		Where("group_id = ?", groupID).
		Order("assigned_at DESC").
		Find(&assignments).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get assessments with details for group: %w", err)
	}

	return assignments, nil
}

// ===== VALIDATION =====

// IsAssigned checks if an assessment is assigned to a specific group
func (ag *AssessmentGroupPostgreSQL) IsAssigned(ctx context.Context, tx *gorm.DB, assessmentID, groupID uint) (bool, error) {
	db := ag.getDB(tx)

	var count int64
	err := db.WithContext(ctx).
		Model(&models.AssessmentGroup{}).
		Where("assessment_id = ? AND group_id = ?", assessmentID, groupID).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check assignment: %w", err)
	}

	return count > 0, nil
}

// GetAssignmentCount returns number of groups an assessment is assigned to
func (ag *AssessmentGroupPostgreSQL) GetAssignmentCount(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error) {
	db := ag.getDB(tx)

	var count int64
	err := db.WithContext(ctx).
		Model(&models.AssessmentGroup{}).
		Where("assessment_id = ?", assessmentID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to count assignments: %w", err)
	}

	return int(count), nil
}

// ExistsByID checks if an assignment exists by ID
func (ag *AssessmentGroupPostgreSQL) ExistsByID(ctx context.Context, tx *gorm.DB, id uint) (bool, error) {
	db := ag.getDB(tx)

	var count int64
	err := db.WithContext(ctx).
		Model(&models.AssessmentGroup{}).
		Where("id = ?", id).
		Count(&count).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check assignment existence: %w", err)
	}

	return count > 0, nil
}
