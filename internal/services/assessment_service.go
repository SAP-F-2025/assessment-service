package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
)

type assessmentService struct {
	repo      repositories.Repository
	logger    *slog.Logger
	validator *utils.Validator
}

func NewAssessmentService(repo repositories.Repository, logger *slog.Logger, validator *utils.Validator) AssessmentService {
	return &assessmentService{
		repo:      repo,
		logger:    logger,
		validator: validator,
	}
}

// ===== CORE CRUD OPERATIONS =====

func (s *assessmentService) Create(ctx context.Context, req *CreateAssessmentRequest, creatorID uint) (*AssessmentResponse, error) {
	s.logger.Info("Creating assessment", "creator_id", creatorID, "title", req.Title)

	// Validate request with business rules
	if errors := s.validator.GetBusinessValidator().ValidateAssessmentCreate(req); len(errors) > 0 {
		return nil, errors
	}

	// Check user permissions
	canCreate, err := s.canCreateAssessment(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("permission check failed: %w", err)
	}
	if !canCreate {
		return nil, NewPermissionError(creatorID, 0, "assessment", "create", "insufficient role permissions")
	}

	// Validate business rules
	if err := s.validateCreateRequest(ctx, req, creatorID); err != nil {
		return nil, err
	}

	// Begin transaction
	txRepo, err := s.repo.(repositories.TransactionRepository).Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			txRepo.(repositories.TransactionRepository).Rollback(ctx)
		}
	}()

	// Create assessment
	assessment := &models.Assessment{
		Title:        req.Title,
		Description:  req.Description,
		Duration:     req.Duration,
		Status:       models.StatusDraft,
		PassingScore: req.PassingScore,
		MaxAttempts:  req.MaxAttempts,
		TimeWarning:  300, // Default 5 minutes
		DueDate:      req.DueDate,
		CreatedBy:    creatorID,
		Version:      1,
	}

	if req.TimeWarning != nil {
		assessment.TimeWarning = *req.TimeWarning
	}

	if err = txRepo.Assessment().Create(ctx, assessment); err != nil {
		return nil, fmt.Errorf("failed to create assessment: %w", err)
	}

	// Create settings
	settings := s.buildAssessmentSettings(assessment.ID, req.Settings)
	if err = txRepo.AssessmentSettings().Create(ctx, settings); err != nil {
		return nil, fmt.Errorf("failed to create assessment settings: %w", err)
	}

	// Add questions if provided
	if len(req.Questions) > 0 {
		if err = s.addQuestionsToAssessment(ctx, txRepo, assessment.ID, req.Questions, creatorID); err != nil {
			return nil, fmt.Errorf("failed to add questions: %w", err)
		}
	}

	// Commit transaction
	if err = txRepo.(repositories.TransactionRepository).Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Assessment created successfully", "assessment_id", assessment.ID)

	// Return response
	return s.GetByIDWithDetails(ctx, assessment.ID, creatorID)
}

func (s *assessmentService) GetByID(ctx context.Context, id uint, userID uint) (*AssessmentResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "assessment", "read", "not owner or insufficient permissions")
	}

	// Get assessment
	assessment, err := s.repo.Assessment().GetByID(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	return s.buildAssessmentResponse(ctx, assessment, userID), nil
}

func (s *assessmentService) GetByIDWithDetails(ctx context.Context, id uint, userID uint) (*AssessmentResponse, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "assessment", "read", "not owner or insufficient permissions")
	}

	// Get assessment with details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("failed to get assessment with details: %w", err)
	}

	return s.buildAssessmentResponse(ctx, assessment, userID), nil
}

func (s *assessmentService) Update(ctx context.Context, id uint, req *UpdateAssessmentRequest, userID uint) (*AssessmentResponse, error) {
	s.logger.Info("Updating assessment", "assessment_id", id, "user_id", userID)

	// Get current assessment for validation
	assessment, err := s.repo.Assessment().GetByID(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	// Validate request with business rules
	if errors := s.validator.GetBusinessValidator().ValidateAssessmentUpdate(req, assessment); len(errors) > 0 {
		return nil, errors
	}

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, NewPermissionError(userID, id, "assessment", "update", "not owner or assessment not editable")
	}

	// Validate business rules for update
	if err := s.validateUpdateRequest(ctx, req, assessment, userID); err != nil {
		return nil, err
	}

	// Begin transaction
	txRepo, err := s.repo.(repositories.TransactionRepository).Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			txRepo.(repositories.TransactionRepository).Rollback(ctx)
		}
	}()

	// Apply updates
	s.applyAssessmentUpdates(assessment, req)

	// Update assessment
	if err = txRepo.Assessment().Update(ctx, assessment); err != nil {
		return nil, fmt.Errorf("failed to update assessment: %w", err)
	}

	// Update settings if provided
	if req.Settings != nil {
		settings, err := txRepo.AssessmentSettings().GetByAssessmentID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get assessment settings: %w", err)
		}

		s.applySettingsUpdates(settings, req.Settings)

		if err = txRepo.AssessmentSettings().Update(ctx, settings); err != nil {
			return nil, fmt.Errorf("failed to update assessment settings: %w", err)
		}
	}

	// Commit transaction
	if err = txRepo.(repositories.TransactionRepository).Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Assessment updated successfully", "assessment_id", id)

	// Return updated assessment
	return s.GetByIDWithDetails(ctx, id, userID)
}

func (s *assessmentService) Delete(ctx context.Context, id uint, userID uint) error {
	s.logger.Info("Deleting assessment", "assessment_id", id, "user_id", userID)

	// Check delete permission
	canDelete, err := s.CanDelete(ctx, id, userID)
	if err != nil {
		return err
	}
	if !canDelete {
		return NewPermissionError(userID, id, "assessment", "delete", "not owner or assessment has attempts")
	}

	// Soft delete
	if err := s.repo.Assessment().Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete assessment: %w", err)
	}

	s.logger.Info("Assessment deleted successfully", "assessment_id", id)
	return nil
}

// ===== LIST AND SEARCH OPERATIONS =====

func (s *assessmentService) List(ctx context.Context, filters repositories.AssessmentFilters, userID uint) (*AssessmentListResponse, error) {
	// For non-admin users, limit to their own assessments
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	if userRole != models.RoleAdmin {
		filters.CreatedBy = &userID
	}

	assessments, total, err := s.repo.Assessment().List(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list assessments: %w", err)
	}

	// Build response
	response := &AssessmentListResponse{
		Assessments: make([]*AssessmentResponse, len(assessments)),
		Total:       total,
		Page:        filters.Offset / max(filters.Limit, 1),
		Size:        filters.Limit,
	}

	for i, assessment := range assessments {
		response.Assessments[i] = s.buildAssessmentResponse(ctx, assessment, userID)
	}

	return response, nil
}

func (s *assessmentService) GetByCreator(ctx context.Context, creatorID uint, filters repositories.AssessmentFilters) (*AssessmentListResponse, error) {
	// Set creator filter
	filters.CreatedBy = &creatorID

	assessments, total, err := s.repo.Assessment().GetByCreator(ctx, creatorID, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessments by creator: %w", err)
	}

	// Build response
	response := &AssessmentListResponse{
		Assessments: make([]*AssessmentResponse, len(assessments)),
		Total:       total,
		Page:        filters.Offset / max(filters.Limit, 1),
		Size:        filters.Limit,
	}

	for i, assessment := range assessments {
		response.Assessments[i] = s.buildAssessmentResponse(ctx, assessment, creatorID)
	}

	return response, nil
}

func (s *assessmentService) Search(ctx context.Context, query string, filters repositories.AssessmentFilters, userID uint) (*AssessmentListResponse, error) {
	// For non-admin users, limit to their own assessments
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, err
	}

	if userRole != models.RoleAdmin {
		filters.CreatedBy = &userID
	}

	assessments, total, err := s.repo.Assessment().Search(ctx, query, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to search assessments: %w", err)
	}

	// Build response
	response := &AssessmentListResponse{
		Assessments: make([]*AssessmentResponse, len(assessments)),
		Total:       total,
		Page:        filters.Offset / max(filters.Limit, 1),
		Size:        filters.Limit,
	}

	for i, assessment := range assessments {
		response.Assessments[i] = s.buildAssessmentResponse(ctx, assessment, userID)
	}

	return response, nil
}

// ===== STATUS MANAGEMENT =====

func (s *assessmentService) UpdateStatus(ctx context.Context, id uint, req *UpdateStatusRequest, userID uint) error {
	s.logger.Info("Updating assessment status", "assessment_id", id, "new_status", req.Status, "user_id", userID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, id, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, id, "assessment", "update_status", "not owner or insufficient permissions")
	}

	// Get current assessment
	assessment, err := s.repo.Assessment().GetByID(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrAssessmentNotFound
		}
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Validate status transition
	if err := s.validateStatusTransition(ctx, assessment, req.Status); err != nil {
		return err
	}

	// Update status
	assessment.Status = req.Status
	assessment.UpdatedAt = time.Now()

	if err := s.repo.Assessment().Update(ctx, assessment); err != nil {
		return fmt.Errorf("failed to update assessment status: %w", err)
	}

	s.logger.Info("Assessment status updated successfully",
		"assessment_id", id,
		"new_status", req.Status,
		"reason", req.Reason)

	return nil
}

func (s *assessmentService) Publish(ctx context.Context, id uint, userID uint) error {
	return s.UpdateStatus(ctx, id, &UpdateStatusRequest{
		Status: models.StatusActive,
		Reason: stringPtr("Published by user"),
	}, userID)
}

func (s *assessmentService) Archive(ctx context.Context, id uint, userID uint) error {
	return s.UpdateStatus(ctx, id, &UpdateStatusRequest{
		Status: models.StatusArchived,
		Reason: stringPtr("Archived by user"),
	}, userID)
}

// ===== QUESTION MANAGEMENT =====

func (s *assessmentService) AddQuestion(ctx context.Context, assessmentID, questionID uint, order int, points *int, userID uint) error {
	s.logger.Info("Adding question to assessment",
		"assessment_id", assessmentID,
		"question_id", questionID,
		"order", order,
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "add_question", "not owner or assessment not editable")
	}

	// Verify question exists and user has access
	questionService := NewQuestionService(s.repo, s.logger, s.validator)
	canAccessQuestion, err := questionService.CanAccess(ctx, questionID, userID)
	if err != nil {
		return err
	}
	if !canAccessQuestion {
		return NewPermissionError(userID, questionID, "question", "access", "question not found or access denied")
	}

	// Add question to assessment
	if err := s.repo.AssessmentQuestion().AddQuestion(ctx, assessmentID, questionID, order, points); err != nil {
		return fmt.Errorf("failed to add question to assessment: %w", err)
	}

	s.logger.Info("Question added to assessment successfully",
		"assessment_id", assessmentID,
		"question_id", questionID)

	return nil
}

func (s *assessmentService) RemoveQuestion(ctx context.Context, assessmentID, questionID uint, userID uint) error {
	s.logger.Info("Removing question from assessment",
		"assessment_id", assessmentID,
		"question_id", questionID,
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "remove_question", "not owner or assessment not editable")
	}

	// Remove question from assessment
	if err := s.repo.AssessmentQuestion().RemoveQuestion(ctx, assessmentID, questionID); err != nil {
		return fmt.Errorf("failed to remove question from assessment: %w", err)
	}

	s.logger.Info("Question removed from assessment successfully",
		"assessment_id", assessmentID,
		"question_id", questionID)

	return nil
}

func (s *assessmentService) ReorderQuestions(ctx context.Context, assessmentID uint, orders []repositories.QuestionOrder, userID uint) error {
	s.logger.Info("Reordering assessment questions",
		"assessment_id", assessmentID,
		"question_count", len(orders),
		"user_id", userID)

	// Check edit permission
	canEdit, err := s.CanEdit(ctx, assessmentID, userID)
	if err != nil {
		return err
	}
	if !canEdit {
		return NewPermissionError(userID, assessmentID, "assessment", "reorder_questions", "not owner or assessment not editable")
	}

	// Reorder questions
	if err := s.repo.AssessmentQuestion().ReorderQuestions(ctx, assessmentID, orders); err != nil {
		return fmt.Errorf("failed to reorder questions: %w", err)
	}

	s.logger.Info("Assessment questions reordered successfully", "assessment_id", assessmentID)

	return nil
}

// ===== STATISTICS AND ANALYTICS =====

func (s *assessmentService) GetStats(ctx context.Context, id uint, userID uint) (*repositories.AssessmentStats, error) {
	// Check access permission
	canAccess, err := s.CanAccess(ctx, id, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "assessment", "view_stats", "not owner or insufficient permissions")
	}

	stats, err := s.repo.Assessment().GetStats(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment stats: %w", err)
	}

	return stats, nil
}

func (s *assessmentService) GetCreatorStats(ctx context.Context, creatorID uint) (*repositories.CreatorStats, error) {
	stats, err := s.repo.Assessment().GetCreatorStats(ctx, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to get creator stats: %w", err)
	}

	return stats, nil
}

// Continue in next part...
