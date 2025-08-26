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

type attemptService struct {
	repo      repositories.Repository
	logger    *slog.Logger
	validator *utils.Validator
}

func NewAttemptService(repo repositories.Repository, logger *slog.Logger, validator *utils.Validator) AttemptService {
	return &attemptService{
		repo:      repo,
		logger:    logger,
		validator: validator,
	}
}

// ===== CORE ATTEMPT OPERATIONS =====

func (s *attemptService) Start(ctx context.Context, req *StartAttemptRequest, studentID uint) (*AttemptResponse, error) {
	s.logger.Info("Starting assessment attempt",
		"assessment_id", req.AssessmentID,
		"student_id", studentID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check if student can start the assessment
	canStart, err := s.CanStart(ctx, req.AssessmentID, studentID)
	if err != nil {
		return nil, err
	}
	if !canStart {
		return nil, ErrAttemptCannotStart
	}

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, req.AssessmentID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAssessmentNotFound
		}
		return nil, fmt.Errorf("failed to get assessment: %w", err)
	}

	// Check if student already has an active attempt
	currentAttempt, err := s.GetCurrentAttempt(ctx, req.AssessmentID, studentID)
	if err != nil && err != ErrAttemptNotFound {
		return nil, err
	}

	if currentAttempt != nil && currentAttempt.Status == models.AttemptStatusInProgress {
		s.logger.Info("Resuming existing attempt", "attempt_id", currentAttempt.ID)
		return currentAttempt, nil
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

	// Create new attempt
	attempt := &models.AssessmentAttempt{
		AssessmentID: req.AssessmentID,
		StudentID:    studentID,
		Status:       models.AttemptStatusInProgress,
		StartedAt:    time.Now(),
		TimeLimit:    assessment.Duration * 60, // Convert minutes to seconds
	}

	// Calculate end time
	endTime := attempt.StartedAt.Add(time.Duration(attempt.TimeLimit) * time.Second)
	attempt.EndTime = &endTime

	if err = txRepo.Attempt().Create(ctx, attempt); err != nil {
		return nil, fmt.Errorf("failed to create attempt: %w", err)
	}

	// Initialize answers for all questions
	if err = s.initializeAttemptAnswers(ctx, txRepo, attempt, assessment); err != nil {
		return nil, fmt.Errorf("failed to initialize answers: %w", err)
	}

	// Commit transaction
	if err = txRepo.(repositories.TransactionRepository).Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Assessment attempt started successfully",
		"attempt_id", attempt.ID,
		"assessment_id", req.AssessmentID,
		"student_id", studentID)

	// Return attempt with questions
	return s.GetByIDWithDetails(ctx, attempt.ID, studentID)
}

func (s *attemptService) Resume(ctx context.Context, attemptID uint, studentID uint) (*AttemptResponse, error) {
	s.logger.Info("Resuming assessment attempt",
		"attempt_id", attemptID,
		"student_id", studentID)

	// Check if attempt exists and belongs to student
	attempt, err := s.repo.Attempt().GetByID(ctx, attemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAttemptNotFound
		}
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}

	// Verify ownership
	if attempt.StudentID != studentID {
		return nil, NewPermissionError(studentID, attemptID, "attempt", "resume", "not owned by student")
	}

	// Check if attempt can be resumed
	if attempt.Status != models.AttemptStatusInProgress {
		return nil, ErrAttemptNotActive
	}

	// Check if attempt has expired
	if attempt.EndTime != nil && time.Now().After(*attempt.EndTime) {
		// Auto-submit expired attempt
		if err := s.HandleTimeout(ctx, attemptID); err != nil {
			s.logger.Error("Failed to handle timeout", "attempt_id", attemptID, "error", err)
		}
		return nil, ErrAttemptTimeExpired
	}

	s.logger.Info("Assessment attempt resumed successfully", "attempt_id", attemptID)

	// Return attempt with questions
	return s.GetByIDWithDetails(ctx, attemptID, studentID)
}

func (s *attemptService) Submit(ctx context.Context, req *SubmitAttemptRequest, studentID uint) (*AttemptResponse, error) {
	s.logger.Info("Submitting assessment attempt",
		"attempt_id", req.AttemptID,
		"student_id", studentID,
		"answers_count", len(req.Answers))

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, req.AttemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAttemptNotFound
		}
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}

	// Verify ownership
	if attempt.StudentID != studentID {
		return nil, NewPermissionError(studentID, req.AttemptID, "attempt", "submit", "not owned by student")
	}

	// Check if already submitted
	if attempt.Status == models.AttemptStatusSubmitted {
		return nil, ErrAttemptAlreadySubmitted
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

	// Update all answers
	for _, answerReq := range req.Answers {
		if err := s.updateAttemptAnswer(ctx, txRepo, req.AttemptID, answerReq, studentID); err != nil {
			return nil, fmt.Errorf("failed to update answer for question %d: %w", answerReq.QuestionID, err)
		}
	}

	// Update attempt status
	attempt.Status = models.AttemptStatusSubmitted
	attempt.SubmittedAt = timePtr(time.Now())
	if req.TimeSpent != nil {
		attempt.TimeSpent = req.TimeSpent
	}
	if req.EndReason != "" {
		attempt.EndReason = &req.EndReason
	}

	if err = txRepo.Attempt().Update(ctx, attempt); err != nil {
		return nil, fmt.Errorf("failed to update attempt: %w", err)
	}

	// Commit transaction
	if err = txRepo.(repositories.TransactionRepository).Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	s.logger.Info("Assessment attempt submitted successfully",
		"attempt_id", req.AttemptID,
		"student_id", studentID)

	// Auto-grade if possible
	go func() {
		gradingService := NewGradingService(s.repo, s.logger, s.validator)
		if _, err := gradingService.AutoGradeAttempt(context.Background(), req.AttemptID); err != nil {
			s.logger.Error("Failed to auto-grade attempt", "attempt_id", req.AttemptID, "error", err)
		}
	}()

	// Return updated attempt
	return s.GetByIDWithDetails(ctx, req.AttemptID, studentID)
}

func (s *attemptService) SubmitAnswer(ctx context.Context, attemptID uint, req *SubmitAnswerRequest, studentID uint) error {
	s.logger.Info("Submitting answer",
		"attempt_id", attemptID,
		"question_id", req.QuestionID,
		"student_id", studentID)

	// Validate request
	if err := s.validator.Validate(req); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, attemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrAttemptNotFound
		}
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Verify ownership
	if attempt.StudentID != studentID {
		return NewPermissionError(studentID, attemptID, "attempt", "submit_answer", "not owned by student")
	}

	// Check if attempt is active
	if attempt.Status != models.AttemptStatusInProgress {
		return ErrAttemptNotActive
	}

	// Check if attempt has expired
	if attempt.EndTime != nil && time.Now().After(*attempt.EndTime) {
		return ErrAttemptTimeExpired
	}

	// Update answer
	if err := s.updateAttemptAnswer(ctx, s.repo, attemptID, *req, studentID); err != nil {
		return fmt.Errorf("failed to update answer: %w", err)
	}

	s.logger.Info("Answer submitted successfully",
		"attempt_id", attemptID,
		"question_id", req.QuestionID)

	return nil
}

// ===== GET OPERATIONS =====

func (s *attemptService) GetByID(ctx context.Context, id uint, userID uint) (*AttemptResponse, error) {
	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAttemptNotFound
		}
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}

	// Check access permission
	canAccess, err := s.canAccessAttempt(ctx, attempt, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "attempt", "read", "not owner or insufficient permissions")
	}

	return s.buildAttemptResponse(ctx, attempt, userID, false), nil
}

func (s *attemptService) GetByIDWithDetails(ctx context.Context, id uint, userID uint) (*AttemptResponse, error) {
	// Get attempt with details
	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, id)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAttemptNotFound
		}
		return nil, fmt.Errorf("failed to get attempt with details: %w", err)
	}

	// Check access permission
	canAccess, err := s.canAccessAttempt(ctx, attempt, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, id, "attempt", "read", "not owner or insufficient permissions")
	}

	return s.buildAttemptResponse(ctx, attempt, userID, true), nil
}

func (s *attemptService) GetCurrentAttempt(ctx context.Context, assessmentID uint, studentID uint) (*AttemptResponse, error) {
	// Get current attempt for student
	attempt, err := s.repo.Attempt().GetCurrentAttempt(ctx, assessmentID, studentID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return nil, ErrAttemptNotFound
		}
		return nil, fmt.Errorf("failed to get current attempt: %w", err)
	}

	return s.buildAttemptResponse(ctx, attempt, studentID, false), nil
}

// ===== LIST OPERATIONS =====

func (s *attemptService) List(ctx context.Context, filters repositories.AttemptFilters, userID uint) ([]*AttemptResponse, int64, error) {
	// Get user role for permission filtering
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return nil, 0, err
	}

	// Filter based on user role
	if userRole == models.RoleStudent {
		filters.StudentID = &userID
	}

	attempts, total, err := s.repo.Attempt().List(ctx, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list attempts: %w", err)
	}

	// Build response
	responses := make([]*AttemptResponse, len(attempts))
	for i, attempt := range attempts {
		responses[i] = s.buildAttemptResponse(ctx, attempt, userID, false)
	}

	return responses, total, nil
}

func (s *attemptService) GetByStudent(ctx context.Context, studentID uint, filters repositories.AttemptFilters) ([]*AttemptResponse, int64, error) {
	// Set student filter
	filters.StudentID = &studentID

	attempts, total, err := s.repo.Attempt().GetByStudent(ctx, studentID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get attempts by student: %w", err)
	}

	// Build response
	responses := make([]*AttemptResponse, len(attempts))
	for i, attempt := range attempts {
		responses[i] = s.buildAttemptResponse(ctx, attempt, studentID, false)
	}

	return responses, total, nil
}

func (s *attemptService) GetByAssessment(ctx context.Context, assessmentID uint, filters repositories.AttemptFilters, userID uint) ([]*AttemptResponse, int64, error) {
	// Check if user can access assessment attempts
	assessmentService := NewAssessmentService(s.repo, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, 0, err
	}
	if !canAccess {
		return nil, 0, NewPermissionError(userID, assessmentID, "assessment", "view_attempts", "not owner or insufficient permissions")
	}

	attempts, total, err := s.repo.Attempt().GetByAssessment(ctx, assessmentID, filters)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get attempts by assessment: %w", err)
	}

	// Build response
	responses := make([]*AttemptResponse, len(attempts))
	for i, attempt := range attempts {
		responses[i] = s.buildAttemptResponse(ctx, attempt, userID, false)
	}

	return responses, total, nil
}

// Continue in next part...
