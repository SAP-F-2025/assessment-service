package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"gorm.io/gorm"
)

type assessmentGroupService struct {
	repo      repositories.Repository
	db        *gorm.DB
	logger    *slog.Logger
	validator *validator.Validator
}

func NewAssessmentGroupService(repo repositories.Repository, db *gorm.DB, logger *slog.Logger, validator *validator.Validator) AssessmentGroupService {
	return &assessmentGroupService{
		repo:      repo,
		db:        db,
		logger:    logger,
		validator: validator,
	}
}

// ===== ASSIGNMENT OPERATIONS =====

// AssignToGroups implements the assignment logic with comprehensive validation
func (s *assessmentGroupService) AssignToGroups(ctx context.Context, assessmentID uint, req *AssignAssessmentToGroupsRequest, userID string) error {
	s.logger.Info("Assigning assessment to groups",
		"assessment_id", assessmentID,
		"group_ids", req.GroupIDs,
		"user_id", userID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// 1. Validate assessment exists and is not Draft
	assessment, err := s.repo.Assessment().GetByID(ctx, nil, assessmentID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrAssessmentNotFound
		}
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	if assessment.Status != models.StatusActive {
		return ErrCannotAssignInActiveAssessment
	}

	// 2. Check if user is assessment creator (fast path)
	isCreator := assessment.CreatedBy == userID

	// 3. Validate all groups exist and check permissions
	for _, groupID := range req.GroupIDs {
		// Check group exists
		exists, err := s.repo.Group().ExistsByID(ctx, nil, groupID)
		if err != nil {
			return fmt.Errorf("failed to check group existence: %w", err)
		}
		if !exists {
			return fmt.Errorf("group %d not found", groupID)
		}

		// Check permissions: skip if already creator
		if !isCreator {
			canAssign, err := s.CanAssignToGroup(ctx, assessmentID, groupID, userID)
			if err != nil {
				return err
			}
			if !canAssign {
				return NewPermissionError(userID, groupID, "assessment-group", "assign",
					"must be assessment creator, group owner, or teacher member")
			}
		}
	}

	// 4. Perform assignment in transaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		return s.repo.AssessmentGroup().AssignToGroups(ctx, tx, assessmentID, req.GroupIDs, userID)
	})

	if err != nil {
		s.logger.Error("Failed to assign assessment to groups", "error", err, "assessment_id", assessmentID)
		return fmt.Errorf("failed to assign assessment to groups: %w", err)
	}

	s.logger.Info("Assessment assigned to groups successfully",
		"assessment_id", assessmentID,
		"group_count", len(req.GroupIDs))

	return nil
}

// UnassignFromGroups removes assessment from groups with permission checks
func (s *assessmentGroupService) UnassignFromGroups(ctx context.Context, assessmentID uint, req *UnassignAssessmentFromGroupsRequest, userID string) error {
	s.logger.Info("Unassigning assessment from groups",
		"assessment_id", assessmentID,
		"group_ids", req.GroupIDs,
		"user_id", userID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// 1. Validate assessment exists
	assessment, err := s.repo.Assessment().GetByID(ctx, nil, assessmentID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrAssessmentNotFound
		}
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// 2. Check if user is assessment creator (fast path)
	isCreator := assessment.CreatedBy == userID

	// 3. Check permissions for each group
	for _, groupID := range req.GroupIDs {
		// Skip permission check if creator
		if !isCreator {
			canUnassign, err := s.CanUnassignFromGroup(ctx, assessmentID, groupID, userID)
			if err != nil {
				return err
			}
			if !canUnassign {
				return NewPermissionError(userID, groupID, "assessment-group", "unassign",
					"must be assessment creator, group owner, or teacher member")
			}
		}
	}

	// 4. Perform unassignment in transaction
	err = s.db.Transaction(func(tx *gorm.DB) error {
		return s.repo.AssessmentGroup().UnassignFromGroups(ctx, tx, assessmentID, req.GroupIDs)
	})

	if err != nil {
		s.logger.Error("Failed to unassign assessment from groups", "error", err, "assessment_id", assessmentID)
		return fmt.Errorf("failed to unassign assessment from groups: %w", err)
	}

	s.logger.Info("Assessment unassigned from groups successfully",
		"assessment_id", assessmentID,
		"group_count", len(req.GroupIDs))

	return nil
}

// ===== QUERY OPERATIONS =====

// GetAssignedGroups retrieves all groups assigned to an assessment
func (s *assessmentGroupService) GetAssignedGroups(ctx context.Context, assessmentID uint, userID string) (*AssessmentGroupAssignmentResponse, error) {
	// Check if user can access this information
	// Assessment creator can always see assigned groups
	assessment, err := s.repo.Assessment().GetByID(ctx, nil, assessmentID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	// TODO: Also allow group owners/teachers to see if their group is assigned
	if assessment.CreatedBy != userID {
		// For now, only creator can see assigned groups
		// Future: check if user is owner/teacher of any assigned group
		return nil, NewPermissionError(userID, assessmentID, "assessment-group", "view",
			"only assessment creator can view assigned groups")
	}

	// Get assigned groups
	groups, err := s.repo.AssessmentGroup().GetGroupsByAssessment(ctx, nil, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assigned groups: %w", err)
	}

	// Build group responses
	groupResponses := make([]*GroupResponse, 0, len(groups))
	for _, group := range groups {
		groupResponses = append(groupResponses, &GroupResponse{
			Group:       group,
			CanEdit:     false, // Not relevant in this context
			CanDelete:   false,
			CanManage:   false,
			MemberCount: 0, // TODO: Could populate if needed
			IsOwner:     group.CreatedBy == userID,
			IsMember:    false,
			MemberRole:  nil,
		})
	}

	return &AssessmentGroupAssignmentResponse{
		AssessmentID: assessmentID,
		Groups:       groupResponses,
		TotalGroups:  len(groupResponses),
	}, nil
}

// GetGroupAssessments retrieves all assessments assigned to a group
// TODO: Phải đủ thông tin như GetStudentAssessments
func (s *assessmentGroupService) GetGroupAssessments(ctx context.Context, groupID uint, userID string) (*GroupAssessmentListResponse, error) {
	// Check if user is member of the group
	isMember, err := s.repo.Group().IsMember(ctx, nil, groupID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check group membership: %w", err)
	}

	// Check if user is group owner
	group, err := s.repo.Group().GetByID(ctx, nil, groupID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrGroupNotFound
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	isOwner := group.CreatedBy == userID

	if !isMember && !isOwner {
		return nil, NewPermissionError(userID, groupID, "assessment-group", "view",
			"must be group member or owner")
	}

	// Get assigned assessments
	assessments, err := s.repo.AssessmentGroup().GetAssessmentsByGroup(ctx, nil, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group assessments: %w", err)
	}

	// Build assessment responses
	assessmentResponses := make([]*AssessmentResponse, 0, len(assessments))
	for _, assessment := range assessments {
		// Check if assessment is expired
		isExpired := false
		if assessment.DueDate != nil && assessment.DueDate.Before(time.Now()) {
			isExpired = true
		}

		// Basic response - could enhance with more details if needed
		assessmentResponses = append(assessmentResponses, &AssessmentResponse{
			Assessment: assessment,
			CanEdit:    assessment.CreatedBy == userID,
			CanDelete:  assessment.CreatedBy == userID,
			CanTake:    true, // Member can take assessments assigned to their group
			IsExpired:  isExpired,
		})
	}

	return &GroupAssessmentListResponse{
		GroupID:     groupID,
		Assessments: assessmentResponses,
		TotalCount:  len(assessmentResponses),
	}, nil
}

// ===== PERMISSION CHECKS =====

// CanAssignToGroup implements permission check logic
// User can assign if they are:
// 1. Assessment creator, OR
// 2. Group owner (created_by), OR
// 3. Teacher member of the group
func (s *assessmentGroupService) CanAssignToGroup(ctx context.Context, assessmentID, groupID uint, userID string) (bool, error) {
	// Check 1: Is user the assessment creator?
	assessment, err := s.repo.Assessment().GetByID(ctx, nil, assessmentID)
	if err != nil {
		return false, err
	}
	if assessment.CreatedBy == userID {
		return true, nil
	}

	// Check 2: Is user the group owner (creator)?
	group, err := s.repo.Group().GetByID(ctx, nil, groupID)
	if err != nil {
		return false, err
	}
	if group.CreatedBy == userID {
		return true, nil
	}

	// Check 3: Is user a teacher member of the group?
	members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
	if err != nil {
		return false, err
	}

	for _, member := range members {
		if member.UserID == userID && member.Role == models.GroupMemberRoleTeacher {
			return true, nil
		}
	}

	return false, nil
}

// CanUnassignFromGroup checks if user can remove assignment
// Same permissions as assignment: creator, group owner, or teacher member
func (s *assessmentGroupService) CanUnassignFromGroup(ctx context.Context, assessmentID, groupID uint, userID string) (bool, error) {
	// Same logic as CanAssignToGroup
	return s.CanAssignToGroup(ctx, assessmentID, groupID, userID)
}
