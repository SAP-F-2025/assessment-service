package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

// ===== TIME MANAGEMENT =====

func (s *attemptService) GetTimeRemaining(ctx context.Context, attemptID uint, studentID string) (int, error) {
	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, nil, attemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return 0, ErrAttemptNotFound
		}
		return 0, fmt.Errorf("failed to get attempt: %w", err)
	}

	// Verify ownership
	if attempt.StudentID != studentID {
		return 0, NewPermissionError(studentID, attemptID, "attempt", "get_time_remaining", "not owned by student")
	}

	// Check if attempt is active
	if attempt.Status != models.AttemptInProgress {
		return 0, ErrAttemptNotActive
	}

	// Calculate time remaining
	if attempt.EndedAt == nil {
		return 0, nil // No time limit
	}

	remaining := int(time.Until(*attempt.EndedAt).Seconds())
	if remaining < 0 {
		return 0, nil // Time expired
	}

	return remaining, nil
}

func (s *attemptService) ExtendTime(ctx context.Context, attemptID uint, minutes int, userID string) error {
	s.logger.Info("Extending attempt time",
		"attempt_id", attemptID,
		"minutes", minutes,
		"user_id", userID)

	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return err
	}

	// Only teachers/admins can extend time
	if userRole != models.RoleTeacher && userRole != models.RoleAdmin {
		return NewPermissionError(userID, attemptID, "attempt", "extend_time", "insufficient permissions")
	}

	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, nil, attemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrAttemptNotFound
		}
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Check if user can access the assessment
	assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, attempt.AssessmentID, userID)
	if err != nil {
		return err
	}
	if !canAccess {
		return NewPermissionError(userID, attempt.AssessmentID, "assessment", "extend_attempt_time", "not owner or insufficient permissions")
	}

	// Check if attempt is active
	if attempt.Status != models.AttemptInProgress {
		return ErrAttemptNotActive
	}

	// Extend time
	if attempt.EndedAt != nil {
		newEndTime := attempt.EndedAt.Add(time.Duration(minutes) * time.Minute)
		attempt.EndedAt = &newEndTime
	}

	if err := s.repo.Attempt().Update(ctx, nil, attempt); err != nil {
		return fmt.Errorf("failed to extend attempt time: %w", err)
	}

	s.logger.Info("Attempt time extended successfully",
		"attempt_id", attemptID,
		"new_end_time", attempt.EndedAt)

	return nil
}

func (s *attemptService) HandleTimeout(ctx context.Context, attemptID uint) error {
	s.logger.Info("Handling attempt timeout", "attempt_id", attemptID)

	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, nil, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Check if attempt is active
	if attempt.Status != models.AttemptInProgress {
		return nil // Already handled
	}

	// Update attempt status to timeout
	attempt.Status = models.AttemptTimeOut
	timeoutReason := models.AttemptEndReasonTimeout
	attempt.EndReason = &timeoutReason
	attempt.CompletedAt = timePtr(time.Now())

	if err := s.repo.Attempt().Update(ctx, nil, attempt); err != nil {
		return fmt.Errorf("failed to update attempt status: %w", err)
	}

	s.logger.Info("Attempt timeout handled successfully", "attempt_id", attemptID)

	// Auto-grade timed out attempt
	go func() {
		gradingService := NewGradingService(s.db, s.repo, s.logger, s.validator)
		if _, err := gradingService.AutoGradeAttempt(context.Background(), attemptID); err != nil {
			s.logger.Error("Failed to auto-grade timed out attempt", "attempt_id", attemptID, "error", err)
		}
	}()

	return nil
}

// ===== VALIDATION =====

func (s *attemptService) CanStart(ctx context.Context, assessmentID uint, studentID string) (bool, error) {
	// Check if assessment is available for taking
	assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
	canTake, err := assessmentService.CanTake(ctx, assessmentID, studentID)
	if err != nil {
		return false, err
	}
	if !canTake {
		return false, nil
	}

	// Get assessment to check attempt limits
	assessment, err := s.repo.Assessment().GetByID(ctx, nil, assessmentID)
	if err != nil {
		return false, err
	}

	// Check attempt count
	attemptCount, err := s.GetAttemptCount(ctx, assessmentID, studentID)
	if err != nil {
		return false, err
	}

	if attemptCount >= assessment.MaxAttempts {
		return false, nil
	}

	// Check if student has an active attempt
	currentAttempt, err := s.GetCurrentAttempt(ctx, assessmentID, studentID)
	if err != nil && err != ErrAttemptNotFound {
		return false, err
	}

	// If there's an active attempt, can resume but not start new
	if currentAttempt != nil && currentAttempt.Status == models.AttemptInProgress {
		// Check if it has expired
		if currentAttempt.EndedAt != nil && time.Now().After(*currentAttempt.EndedAt) {
			// Auto-handle timeout
			if err := s.HandleTimeout(ctx, currentAttempt.ID); err != nil {
				s.logger.Error("Failed to handle expired attempt", "attempt_id", currentAttempt.ID, "error", err)
			}
			return true, nil // Can start new attempt after timeout
		}
		return false, nil // Has active attempt, should resume instead
	}

	return true, nil
}

func (s *attemptService) GetAttemptCount(ctx context.Context, assessmentID uint, studentID string) (int, error) {
	count, err := s.repo.Attempt().GetAttemptCount(ctx, nil, studentID, assessmentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get attempt count: %w", err)
	}
	return count, nil
}

func (s *attemptService) IsAttemptActive(ctx context.Context, attemptID uint) (bool, error) {
	attempt, err := s.repo.Attempt().GetByID(ctx, nil, attemptID)
	if err != nil {
		return false, err
	}

	if attempt.Status != models.AttemptInProgress {
		return false, nil
	}

	// Check if time expired
	if attempt.EndedAt != nil && time.Now().After(*attempt.EndedAt) {
		return false, nil
	}

	return true, nil
}

// ===== STATISTICS =====

func (s *attemptService) GetStats(ctx context.Context, assessmentID uint, userID string) (*repositories.AttemptStats, error) {
	// Check access permission
	assessmentService := NewAssessmentService(s.repo, nil, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, assessmentID, "assessment", "view_stats", "not owner or insufficient permissions")
	}

	stats, err := s.repo.Attempt().GetAssessmentAttemptStats(ctx, nil, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attempt stats: %w", err)
	}

	return stats, nil
}

// ===== HELPER FUNCTIONS =====

func (s *attemptService) getUserRole(ctx context.Context, userID string) (models.UserRole, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.Role, nil
}

func (s *attemptService) canAccessAttempt(ctx context.Context, attempt *models.AssessmentAttempt, userID string) (bool, error) {
	// Get user role
	userRole, err := s.getUserRole(ctx, userID)
	if err != nil {
		return false, err
	}

	// Students can only access their own attempts
	if userRole == models.RoleStudent {
		return attempt.StudentID == userID, nil
	}

	// Teachers/Admins can access attempts for their assessments
	if userRole == models.RoleTeacher || userRole == models.RoleAdmin {
		assessmentService := NewAssessmentService(s.repo, s.db, s.logger, s.validator)
		return assessmentService.CanAccess(ctx, attempt.AssessmentID, userID)
	}

	return false, nil
}

func (s *attemptService) buildAttemptResponse(ctx context.Context, attempt *models.AssessmentAttempt, userID string, includeQuestions bool) *AttemptResponse {
	response := &AttemptResponse{
		AssessmentAttempt: attempt,
	}

	// Determine permissions
	response.CanSubmit = attempt.Status == models.AttemptInProgress &&
		attempt.StudentID == userID &&
		(attempt.EndedAt == nil || time.Now().Before(*attempt.EndedAt))

	response.CanResume = response.CanSubmit

	// Include questions if requested and user is the student
	if includeQuestions && attempt.StudentID == userID {
		questions, err := s.getAttemptQuestions(ctx, attempt.AssessmentID)
		if err != nil {
			s.logger.Error("Failed to get attempt questions", "attempt_id", attempt.ID, "error", err)
		} else {
			response.Questions = questions
		}
	}

	return response
}

func (s *attemptService) getAttemptQuestions(ctx context.Context, assessmentId uint) ([]QuestionForAttempt, error) {
	// Get assessment questions with answers
	assessmentQuestions, err := s.repo.AssessmentQuestion().GetQuestionsForAssessment(ctx, nil, assessmentId)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment questions: %w", err)
	}

	questions := make([]QuestionForAttempt, len(assessmentQuestions))
	for i, aq := range assessmentQuestions {
		copyAq := *aq // Create a copy to avoid modifying the original
		questions[i] = QuestionForAttempt{
			Question: &copyAq,
			IsFirst:  i == 0,
			IsLast:   i == len(assessmentQuestions)-1,
		}
	}

	return questions, nil
}

func (s *attemptService) initializeAttemptAnswers(ctx context.Context, tx *gorm.DB, attempt *models.AssessmentAttempt, assessment *models.Assessment) error {
	// Get all questions for the assessment
	assessmentQuestions, err := s.repo.AssessmentQuestion().GetByAssessment(ctx, tx, assessment.ID)
	if err != nil {
		return fmt.Errorf("failed to get assessment questions: %w", err)
	}

	// Create empty answers for all questions
	answers := make([]*models.StudentAnswer, len(assessmentQuestions))
	for i, aq := range assessmentQuestions {
		answers[i] = &models.StudentAnswer{
			AttemptID:  attempt.ID,
			QuestionID: aq.QuestionID,
			Answer:     nil, // Empty initially
			Flagged:    false,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
	}

	// Batch create answers
	if err := s.repo.Answer().CreateBatch(ctx, tx, answers); err != nil {
		return fmt.Errorf("failed to create initial answers: %w", err)
	}

	return nil
}

func (s *attemptService) updateAttemptAnswer(ctx context.Context, tx *gorm.DB, attemptID uint, req SubmitAnswerRequest, studentID string) error {
	// Get existing answer
	answer, err := s.repo.Answer().GetByAttemptAndQuestion(ctx, tx, attemptID, req.QuestionID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			// Create new answer if doesn't exist
			answer = &models.StudentAnswer{
				AttemptID:  attemptID,
				QuestionID: req.QuestionID,
			}
		} else {
			return fmt.Errorf("failed to get existing answer: %w", err)
		}
	}

	// Convert answer data to JSON
	if req.AnswerData != nil {
		answerBytes, err := json.Marshal(req.AnswerData)
		if err != nil {
			return fmt.Errorf("failed to marshal answer data: %w", err)
		}
		answer.Answer = answerBytes
	}

	answer.UpdatedAt = time.Now()

	if req.TimeSpent != nil {
		answer.TimeSpent = *req.TimeSpent
	}

	// Upsert answer
	if answer.ID == 0 {
		if err := s.repo.Answer().Create(ctx, s.db, answer); err != nil {
			return fmt.Errorf("failed to create answer: %w", err)
		}
	} else {
		if err := s.repo.Answer().Update(ctx, s.db, answer); err != nil {
			return fmt.Errorf("failed to update answer: %w", err)
		}
	}

	return nil
}
