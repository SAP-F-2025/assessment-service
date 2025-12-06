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

// GetGroupAssessments retrieves all assessments assigned to a group with detailed information
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

	// Get member info to check role
	var userRole *models.GroupMemberRole
	if isMember {
		members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
		if err != nil {
			return nil, fmt.Errorf("failed to get group members: %w", err)
		}
		for _, member := range members {
			if member.UserID == userID {
				userRole = &member.Role
				break
			}
		}
	}

	// Determine if user is a regular member (not owner) for populating member-specific fields
	isMemberRole := userRole != nil && *userRole == models.GroupMemberRoleMember

	// Get assigned assessments
	assessments, err := s.repo.AssessmentGroup().GetAssessmentsByGroup(ctx, nil, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group assessments: %w", err)
	}

	assessmentIDs := make([]uint, len(assessments))
	for i, a := range assessments {
		assessmentIDs[i] = a.ID
	}

	// Batch load settings
	var settingsMap = make(map[uint]models.AssessmentSettings)
	var allSettings []models.AssessmentSettings
	if err := s.db.WithContext(ctx).Where("assessment_id IN ?", assessmentIDs).Find(&allSettings).Error; err == nil {
		for _, setting := range allSettings {
			settingsMap[setting.AssessmentID] = setting
		}
	}

	// Batch load questions count and total points
	type AssessmentStats struct {
		AssessmentID   uint
		QuestionsCount int64
		TotalPoints    int
	}
	var statsMap = make(map[uint]AssessmentStats)
	var stats []AssessmentStats
	s.db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Select("assessment_id, COUNT(*) as questions_count, COALESCE(SUM(points), 0) as total_points").
		Where("assessment_id IN ?", assessmentIDs).
		Group("assessment_id").
		Scan(&stats)
	for _, stat := range stats {
		statsMap[stat.AssessmentID] = stat
	}

	// Batch load student attempts if student
	var attemptCountMap = make(map[uint]int)
	var hasActiveMap = make(map[uint]bool)
	var bestScoreMap = make(map[uint]*float64)
	var lastAttemptDateMap = make(map[uint]*time.Time)
	var canStartMap = make(map[uint]bool)

	if isMemberRole {
		// Batch load attempt counts
		type AttemptCount struct {
			AssessmentID uint
			Count        int
		}
		var attemptCounts []AttemptCount
		s.db.WithContext(ctx).
			Model(&models.AssessmentAttempt{}).
			Select("assessment_id, COUNT(*) as count").
			Where("student_id = ? AND assessment_id IN ?", userID, assessmentIDs).
			Group("assessment_id").
			Scan(&attemptCounts)
		for _, ac := range attemptCounts {
			attemptCountMap[ac.AssessmentID] = ac.Count
		}

		// Batch check active attempts
		type ActiveAttempt struct {
			AssessmentID uint
		}
		var activeAttempts []ActiveAttempt
		s.db.WithContext(ctx).
			Model(&models.AssessmentAttempt{}).
			Select("DISTINCT assessment_id").
			Where("student_id = ? AND assessment_id IN ? AND status = ?", userID, assessmentIDs, models.AttemptInProgress).
			Scan(&activeAttempts)
		for _, aa := range activeAttempts {
			hasActiveMap[aa.AssessmentID] = true
		}

		// Batch load best scores and last attempt dates
		type BestAttempt struct {
			AssessmentID    uint
			BestScore       *float64
			LastAttemptDate *time.Time
		}
		var bestAttempts []BestAttempt
		s.db.WithContext(ctx).
			Model(&models.AssessmentAttempt{}).
			Select("assessment_id, MAX(score) as best_score, MAX(completed_at) as last_attempt_date").
			Where("student_id = ? AND assessment_id IN ? AND status = ?", userID, assessmentIDs, models.AttemptCompleted).
			Group("assessment_id").
			Scan(&bestAttempts)
		for _, ba := range bestAttempts {
			bestScoreMap[ba.AssessmentID] = ba.BestScore
			lastAttemptDateMap[ba.AssessmentID] = ba.LastAttemptDate
		}

		// Batch check can start (simplified - you may want to optimize this further)
		for _, assessment := range assessments {
			validation, err := s.repo.Attempt().CanStartAttempt(ctx, s.db, userID, assessment.ID)
			canStartMap[assessment.ID] = err == nil && validation != nil && validation.CanStart
		}
	}

	// Build detailed assessment responses
	assessmentItems := make([]*GroupAssessmentItem, 0, len(assessments))
	for _, assessment := range assessments {
		// Check if assessment is expired
		isExpired := false
		if assessment.DueDate != nil && assessment.DueDate.Before(time.Now()) {
			isExpired = true
		}

		// Get data from maps (no queries!)
		setting, _ := settingsMap[assessment.ID]
		stat := statsMap[assessment.ID]

		item := &GroupAssessmentItem{
			ID:             assessment.ID,
			Title:          assessment.Title,
			Description:    assessment.Description,
			Duration:       assessment.Duration,
			PassingScore:   float64(assessment.PassingScore),
			Status:         assessment.Status,
			DueDate:        assessment.DueDate,
			QuestionsCount: int(stat.QuestionsCount),
			TotalPoints:    stat.TotalPoints,
			Settings:       setting,
			IsExpired:      isExpired,
			CanEdit:        assessment.CreatedBy == userID,
			CanDelete:      assessment.CreatedBy == userID,
			CanTake:        true, // Member can take assessments assigned to their group
		}

		// Populate member-specific fields (from maps - no queries!)
		if isMemberRole {
			attemptCount := attemptCountMap[assessment.ID]
			hasActive := hasActiveMap[assessment.ID]
			bestScore := bestScoreMap[assessment.ID]
			lastAttemptDate := lastAttemptDateMap[assessment.ID]
			canStart := canStartMap[assessment.ID]

			item.AttemptsUsed = &attemptCount
			maxAttempts := assessment.MaxAttempts
			item.MaxAttempts = &maxAttempts
			item.CanStart = &canStart
			item.HasActiveAttempt = &hasActive
			item.BestScore = bestScore
			item.LastAttemptDate = lastAttemptDate
		}

		assessmentItems = append(assessmentItems, item)
	}

	return &GroupAssessmentListResponse{
		GroupID:     groupID,
		Assessments: assessmentItems,
		TotalCount:  len(assessmentItems),
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

	// Check 3: Is user an owner or co-owner member of the group?
	// Both owner and co-owner can assign assessments to the group
	members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
	if err != nil {
		return false, err
	}

	for _, member := range members {
		if member.UserID == userID {
			if member.Role == models.GroupMemberRoleOwner || member.Role == models.GroupMemberRoleCoOwner {
				return true, nil
			}
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
