package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
)

// ===== TIME MANAGEMENT =====

func (s *attemptService) GetTimeRemaining(ctx context.Context, attemptID uint, studentID uint) (int, error) {
	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, attemptID)
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
	if attempt.Status != models.AttemptStatusInProgress {
		return 0, ErrAttemptNotActive
	}

	// Calculate time remaining
	if attempt.EndTime == nil {
		return 0, nil // No time limit
	}

	remaining := int(time.Until(*attempt.EndTime).Seconds())
	if remaining < 0 {
		return 0, nil // Time expired
	}

	return remaining, nil
}

func (s *attemptService) ExtendTime(ctx context.Context, attemptID uint, minutes int, userID uint) error {
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
	attempt, err := s.repo.Attempt().GetByID(ctx, attemptID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrAttemptNotFound
		}
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Check if user can access the assessment
	assessmentService := NewAssessmentService(s.repo, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, attempt.AssessmentID, userID)
	if err != nil {
		return err
	}
	if !canAccess {
		return NewPermissionError(userID, attempt.AssessmentID, "assessment", "extend_attempt_time", "not owner or insufficient permissions")
	}

	// Check if attempt is active
	if attempt.Status != models.AttemptStatusInProgress {
		return ErrAttemptNotActive
	}

	// Extend time
	if attempt.EndTime != nil {
		newEndTime := attempt.EndTime.Add(time.Duration(minutes) * time.Minute)
		attempt.EndTime = &newEndTime
	}

	if err := s.repo.Attempt().Update(ctx, attempt); err != nil {
		return fmt.Errorf("failed to extend attempt time: %w", err)
	}

	s.logger.Info("Attempt time extended successfully",
		"attempt_id", attemptID,
		"new_end_time", attempt.EndTime)

	return nil
}

func (s *attemptService) HandleTimeout(ctx context.Context, attemptID uint) error {
	s.logger.Info("Handling attempt timeout", "attempt_id", attemptID)

	// Get attempt
	attempt, err := s.repo.Attempt().GetByID(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Check if attempt is active
	if attempt.Status != models.AttemptStatusInProgress {
		return nil // Already handled
	}

	// Update attempt status to timeout
	attempt.Status = models.AttemptStatusTimedOut
	timeoutReason := models.AttemptEndReasonTimeout
	attempt.EndReason = &timeoutReason
	attempt.SubmittedAt = timePtr(time.Now())

	if err := s.repo.Attempt().Update(ctx, attempt); err != nil {
		return fmt.Errorf("failed to update attempt status: %w", err)
	}

	s.logger.Info("Attempt timeout handled successfully", "attempt_id", attemptID)

	// Auto-grade timed out attempt
	go func() {
		gradingService := NewGradingService(s.repo, s.logger, s.validator)
		if _, err := gradingService.AutoGradeAttempt(context.Background(), attemptID); err != nil {
			s.logger.Error("Failed to auto-grade timed out attempt", "attempt_id", attemptID, "error", err)
		}
	}()

	return nil
}

// ===== VALIDATION =====

func (s *attemptService) CanStart(ctx context.Context, assessmentID uint, studentID uint) (bool, error) {
	// Check if assessment is available for taking
	assessmentService := NewAssessmentService(s.repo, s.logger, s.validator)
	canTake, err := assessmentService.CanTake(ctx, assessmentID, studentID)
	if err != nil {
		return false, err
	}
	if !canTake {
		return false, nil
	}

	// Get assessment to check attempt limits
	assessment, err := s.repo.Assessment().GetByID(ctx, assessmentID)
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
	if currentAttempt != nil && currentAttempt.Status == models.AttemptStatusInProgress {
		// Check if it has expired
		if currentAttempt.EndTime != nil && time.Now().After(*currentAttempt.EndTime) {
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

func (s *attemptService) GetAttemptCount(ctx context.Context, assessmentID uint, studentID uint) (int, error) {
	count, err := s.repo.Attempt().GetStudentAttemptCount(ctx, assessmentID, studentID)
	if err != nil {
		return 0, fmt.Errorf("failed to get attempt count: %w", err)
	}
	return count, nil
}

func (s *attemptService) IsAttemptActive(ctx context.Context, attemptID uint) (bool, error) {
	attempt, err := s.repo.Attempt().GetByID(ctx, attemptID)
	if err != nil {
		return false, err
	}

	if attempt.Status != models.AttemptStatusInProgress {
		return false, nil
	}

	// Check if time expired
	if attempt.EndTime != nil && time.Now().After(*attempt.EndTime) {
		return false, nil
	}

	return true, nil
}

// ===== STATISTICS =====

func (s *attemptService) GetStats(ctx context.Context, assessmentID uint, userID uint) (*repositories.AttemptStats, error) {
	// Check access permission
	assessmentService := NewAssessmentService(s.repo, s.logger, s.validator)
	canAccess, err := assessmentService.CanAccess(ctx, assessmentID, userID)
	if err != nil {
		return nil, err
	}
	if !canAccess {
		return nil, NewPermissionError(userID, assessmentID, "assessment", "view_stats", "not owner or insufficient permissions")
	}

	stats, err := s.repo.Attempt().GetStats(ctx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attempt stats: %w", err)
	}

	return stats, nil
}

// ===== HELPER FUNCTIONS =====

func (s *attemptService) getUserRole(ctx context.Context, userID uint) (models.UserRole, error) {
	user, err := s.repo.User().GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("failed to get user: %w", err)
	}
	return user.Role, nil
}

func (s *attemptService) canAccessAttempt(ctx context.Context, attempt *models.AssessmentAttempt, userID uint) (bool, error) {
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
		assessmentService := NewAssessmentService(s.repo, s.logger, s.validator)
		return assessmentService.CanAccess(ctx, attempt.AssessmentID, userID)
	}

	return false, nil
}

func (s *attemptService) buildAttemptResponse(ctx context.Context, attempt *models.AssessmentAttempt, userID uint, includeQuestions bool) *AttemptResponse {
	response := &AttemptResponse{
		AssessmentAttempt: attempt,
	}

	// Determine permissions
	response.CanSubmit = attempt.Status == models.AttemptStatusInProgress &&
		attempt.StudentID == userID &&
		(attempt.EndTime == nil || time.Now().Before(*attempt.EndTime))

	response.CanResume = response.CanSubmit

	// Include questions if requested and user is the student
	if includeQuestions && attempt.StudentID == userID {
		questions, err := s.getAttemptQuestions(ctx, attempt.ID)
		if err != nil {
			s.logger.Error("Failed to get attempt questions", "attempt_id", attempt.ID, "error", err)
		} else {
			response.Questions = questions
		}
	}

	return response
}

func (s *attemptService) getAttemptQuestions(ctx context.Context, attemptID uint) ([]QuestionForAttempt, error) {
	// Get assessment questions with answers
	assessmentQuestions, err := s.repo.AssessmentQuestion().GetByAttemptWithAnswers(ctx, attemptID)
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment questions: %w", err)
	}

	questions := make([]QuestionForAttempt, len(assessmentQuestions))
	for i, aq := range assessmentQuestions {
		questions[i] = QuestionForAttempt{
			Question:       aq.Question,
			Order:          aq.Order,
			Points:         aq.Points,
			ExistingAnswer: aq.Answer, // Answer if exists
			IsFirst:        i == 0,
			IsLast:         i == len(assessmentQuestions)-1,
		}
	}

	return questions, nil
}

func (s *attemptService) initializeAttemptAnswers(ctx context.Context, txRepo repositories.Repository, attempt *models.AssessmentAttempt, assessment *models.Assessment) error {
	// Get all questions for the assessment
	assessmentQuestions, err := txRepo.AssessmentQuestion().GetByAssessment(ctx, assessment.ID)
	if err != nil {
		return fmt.Errorf("failed to get assessment questions: %w", err)
	}

	// Create empty answers for all questions
	answers := make([]*models.StudentAnswer, len(assessmentQuestions))
	for i, aq := range assessmentQuestions {
		answers[i] = &models.StudentAnswer{
			AttemptID:  attempt.ID,
			QuestionID: aq.QuestionID,
			AnswerData: nil, // Empty initially
			IsSkipped:  false,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}
	}

	// Batch create answers
	if err := txRepo.Answer().CreateBatch(ctx, answers); err != nil {
		return fmt.Errorf("failed to create initial answers: %w", err)
	}

	return nil
}

func (s *attemptService) updateAttemptAnswer(ctx context.Context, repo repositories.Repository, attemptID uint, req SubmitAnswerRequest, studentID uint) error {
	// Get existing answer
	answer, err := repo.Answer().GetByAttemptAndQuestion(ctx, attemptID, req.QuestionID)
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
		answer.AnswerData = answerBytes
	}

	// Update answer fields
	answer.IsSkipped = req.IsSkipped
	answer.IsBookmarked = req.IsBookmarked
	answer.UpdatedAt = time.Now()

	if req.TimeSpent != nil {
		answer.TimeSpent = req.TimeSpent
	}

	// Upsert answer
	if answer.ID == 0 {
		if err := repo.Answer().Create(ctx, answer); err != nil {
			return fmt.Errorf("failed to create answer: %w", err)
		}
	} else {
		if err := repo.Answer().Update(ctx, answer); err != nil {
			return fmt.Errorf("failed to update answer: %w", err)
		}
	}

	return nil
}

// ===== UTILITY FUNCTIONS =====

func timePtr(t time.Time) *time.Time {
	return &t
}
