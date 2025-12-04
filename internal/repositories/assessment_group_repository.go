package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"gorm.io/gorm"
)

// AssessmentGroupRepository handles assessment-group assignment operations
type AssessmentGroupRepository interface {
	// ===== ASSIGNMENT OPERATIONS =====

	// AssignToGroups assigns an assessment to multiple groups (bulk operation)
	// Creates assignment records with assigned_by tracking
	AssignToGroups(ctx context.Context, tx *gorm.DB, assessmentID uint, groupIDs []uint, assignedBy string) error

	// UnassignFromGroups removes an assessment from multiple groups (soft delete)
	UnassignFromGroups(ctx context.Context, tx *gorm.DB, assessmentID uint, groupIDs []uint) error

	// AssignToGroup assigns an assessment to a single group
	AssignToGroup(ctx context.Context, tx *gorm.DB, assignment *models.AssessmentGroup) error

	// UnassignFromGroup removes an assessment from a single group
	UnassignFromGroup(ctx context.Context, tx *gorm.DB, assessmentID, groupID uint) error

	// ===== QUERY OPERATIONS =====

	// GetGroupsByAssessment retrieves all groups assigned to an assessment
	GetGroupsByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.Group, error)

	// GetGroupsByAssessmentWithDetails retrieves groups with full AssessmentGroup details
	GetGroupsByAssessmentWithDetails(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.AssessmentGroup, error)

	// GetAssessmentsByGroup retrieves all assessments assigned to a group
	GetAssessmentsByGroup(ctx context.Context, tx *gorm.DB, groupID uint) ([]*models.Assessment, error)

	// GetAssessmentsByGroupWithDetails retrieves assessments with assignment metadata
	GetAssessmentsByGroupWithDetails(ctx context.Context, tx *gorm.DB, groupID uint) ([]*models.AssessmentGroup, error)

	// GetStudentAssessments retrieves assessments visible to a student based on group membership
	// This is the KEY method for PRIVATE visibility implementation
	GetStudentAssessments(ctx context.Context, tx *gorm.DB, studentID string, statusFilter models.AssessmentStatus) ([]*models.Assessment, error)

	// ===== VALIDATION =====

	// IsAssigned checks if an assessment is assigned to a specific group
	IsAssigned(ctx context.Context, tx *gorm.DB, assessmentID, groupID uint) (bool, error)

	// GetAssignmentCount returns number of groups an assessment is assigned to
	GetAssignmentCount(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error)

	// ExistsByID checks if an assignment exists by ID
	ExistsByID(ctx context.Context, tx *gorm.DB, id uint) (bool, error)
}
