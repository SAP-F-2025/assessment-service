package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/events"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/utils"
)

// NotificationEventService handles sending notifications through event publishing
// This replaces the direct notification service with an event-driven approach
type NotificationEventService interface {
	// Assessment notifications
	NotifyAssessmentPublished(ctx context.Context, assessmentID uint) error
	NotifyAssessmentExpiring(ctx context.Context, assessmentID uint, hoursRemaining int) error
	NotifyAssessmentExpired(ctx context.Context, assessmentID uint) error

	// Attempt notifications
	NotifyAttemptStarted(ctx context.Context, attemptID uint) error
	NotifyAttemptSubmitted(ctx context.Context, attemptID uint) error
	NotifyAttemptGraded(ctx context.Context, attemptID uint) error
	NotifyAttemptTimeWarning(ctx context.Context, attemptID uint, minutesRemaining int) error

	// Grading notifications
	NotifyGradingCompleted(ctx context.Context, assessmentID uint) error
	NotifyManualGradingRequired(ctx context.Context, assessmentID uint, questionCount int) error

	// System notifications
	SendBulkNotification(ctx context.Context, userIDs []uint, notification *NotificationRequest) error
}

type NotificationRequest struct {
	Type        models.NotificationType     `json:"type"`
	Title       string                      `json:"title"`
	Message     string                      `json:"message"`
	Priority    models.NotificationPriority `json:"priority"`
	ActionURL   *string                     `json:"action_url,omitempty"`
	Metadata    map[string]interface{}      `json:"metadata,omitempty"`
	ScheduledAt *time.Time                  `json:"scheduled_at,omitempty"`
}
type notificationEventService struct {
	repo           repositories.Repository
	eventPublisher events.EventPublisher
	logger         *slog.Logger
	validator      *utils.Validator
}

func NewNotificationEventService(
	repo repositories.Repository,
	eventPublisher events.EventPublisher,
	logger *slog.Logger,
	validator *utils.Validator,
) NotificationEventService {
	return &notificationEventService{
		repo:           repo,
		eventPublisher: eventPublisher,
		logger:         logger,
		validator:      validator,
	}
}

// ===== ASSESSMENT NOTIFICATIONS =====

func (s *notificationEventService) NotifyAssessmentPublished(ctx context.Context, assessmentID uint) error {
	s.logger.Info("Publishing assessment published event", "assessment_id", assessmentID)

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, nil, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get enrolled students (placeholder - implement based on your enrollment system)
	studentIDs := s.getEnrolledStudentIDs(ctx, assessmentID)

	// Create and publish event
	event := events.NewAssessmentPublishedEvent(
		assessmentID,
		assessment.Title,
		assessment.DueDate,
		assessment.Duration,
		studentIDs,
		assessment.CreatedBy,
	)

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

func (s *notificationEventService) NotifyAssessmentExpiring(ctx context.Context, assessmentID uint, hoursRemaining int) error {
	s.logger.Info("Publishing assessment expiring event",
		"assessment_id", assessmentID,
		"hours_remaining", hoursRemaining)

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, nil, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get enrolled students who haven't completed the assessment
	studentIDs := s.getStudentsWithIncompleteAssessment(ctx, assessmentID)

	// Create and publish event
	event := &events.NotificationEvent{
		ID:        events.GenerateEventID(),
		Type:      events.EventAssessmentExpiring,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: events.AssessmentExpiringEvent{
			AssessmentID:    assessmentID,
			AssessmentTitle: assessment.Title,
			HoursRemaining:  hoursRemaining,
			StudentIDs:      studentIDs,
			DueDate:         *assessment.DueDate,
		},
	}

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

func (s *notificationEventService) NotifyAssessmentExpired(ctx context.Context, assessmentID uint) error {
	s.logger.Info("Publishing assessment expired event", "assessment_id", assessmentID)

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, nil, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get all enrolled students
	studentIDs := s.getEnrolledStudentIDs(ctx, assessmentID)

	// Create and publish event
	event := &events.NotificationEvent{
		ID:        events.GenerateEventID(),
		Type:      events.EventAssessmentExpired,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: events.AssessmentExpiredEvent{
			AssessmentID:    assessmentID,
			AssessmentTitle: assessment.Title,
			ExpiredAt:       time.Now(),
			StudentIDs:      studentIDs,
			CreatorID:       assessment.CreatedBy,
		},
	}

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

// ===== ATTEMPT NOTIFICATIONS =====

func (s *notificationEventService) NotifyAttemptStarted(ctx context.Context, attemptID uint) error {
	s.logger.Info("Publishing attempt started event", "attempt_id", attemptID)

	// Get attempt with assessment details
	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, nil, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Create and publish event
	event := events.NewAttemptStartedEvent(
		attemptID,
		attempt.AssessmentID,
		attempt.Assessment.Title,
		attempt.StudentID,
		*attempt.StartedAt,
		&attempt.Assessment.Duration,
	)

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

func (s *notificationEventService) NotifyAttemptSubmitted(ctx context.Context, attemptID uint) error {
	s.logger.Info("Publishing attempt submitted event", "attempt_id", attemptID)

	// Get attempt with assessment details
	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, nil, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Create and publish event
	event := &events.NotificationEvent{
		ID:        events.GenerateEventID(),
		Type:      events.EventAttemptSubmitted,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: events.AttemptSubmittedEvent{
			AttemptID:       attemptID,
			AssessmentID:    attempt.AssessmentID,
			AssessmentTitle: attempt.Assessment.Title,
			StudentID:       attempt.StudentID,
			SubmittedAt:     *attempt.CompletedAt,
			Score:           &attempt.Score,
			Passed:          &attempt.Passed,
			GradingRequired: s.requiresManualGrading(ctx, attemptID),
		},
	}

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

func (s *notificationEventService) NotifyAttemptGraded(ctx context.Context, attemptID uint) error {
	s.logger.Info("Publishing attempt graded event", "attempt_id", attemptID)

	// Get attempt with assessment details
	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, nil, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Create and publish event
	event := &events.NotificationEvent{
		ID:        events.GenerateEventID(),
		Type:      events.EventAttemptGraded,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: events.AttemptGradedEvent{
			AttemptID:       attemptID,
			AssessmentID:    attempt.AssessmentID,
			AssessmentTitle: attempt.Assessment.Title,
			StudentID:       attempt.StudentID,
			GradedAt:        time.Now(),
			Score:           attempt.Score,
			MaxScore:        attempt.MaxScore,
			Percentage:      attempt.Percentage,
			Passed:          attempt.Passed,
			GraderID:        attempt.Assessment.CreatedBy, // or actual grader ID
		},
	}

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

func (s *notificationEventService) NotifyAttemptTimeWarning(ctx context.Context, attemptID uint, minutesRemaining int) error {
	s.logger.Info("Publishing attempt time warning event",
		"attempt_id", attemptID,
		"minutes_remaining", minutesRemaining)

	// Get attempt with assessment details
	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, nil, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Create and publish event
	event := &events.NotificationEvent{
		ID:        events.GenerateEventID(),
		Type:      events.EventAttemptTimeWarning,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: events.AttemptTimeWarningEvent{
			AttemptID:        attemptID,
			AssessmentID:     attempt.AssessmentID,
			AssessmentTitle:  attempt.Assessment.Title,
			StudentID:        attempt.StudentID,
			MinutesRemaining: minutesRemaining,
			WarningTime:      time.Now(),
		},
	}

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

// ===== GRADING NOTIFICATIONS =====

func (s *notificationEventService) NotifyGradingCompleted(ctx context.Context, assessmentID uint) error {
	s.logger.Info("Publishing grading completed event", "assessment_id", assessmentID)

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, nil, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get grading statistics
	stats := s.getGradingStats(ctx, assessmentID)

	// Create and publish event
	event := &events.NotificationEvent{
		ID:        events.GenerateEventID(),
		Type:      events.EventGradingCompleted,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: events.GradingCompletedEvent{
			AssessmentID:      assessmentID,
			AssessmentTitle:   assessment.Title,
			CompletedAt:       time.Now(),
			TotalAttempts:     stats.TotalAttempts,
			AutoGradedCount:   stats.AutoGradedCount,
			ManualGradedCount: stats.ManualGradedCount,
			CreatorID:         assessment.CreatedBy,
		},
	}

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

func (s *notificationEventService) NotifyManualGradingRequired(ctx context.Context, assessmentID uint, questionCount int) error {
	s.logger.Info("Publishing manual grading required event",
		"assessment_id", assessmentID,
		"question_count", questionCount)

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, nil, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get pending attempt IDs that require manual grading
	pendingAttemptIDs := s.getPendingManualGradingAttempts(ctx, assessmentID)

	// Get potential grader IDs (teachers, admins)
	graderIDs := s.getAvailableGraderIDs(ctx, assessmentID)

	// Create and publish event
	event := &events.NotificationEvent{
		ID:        events.GenerateEventID(),
		Type:      events.EventManualGradingRequired,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: events.ManualGradingRequiredEvent{
			AssessmentID:      assessmentID,
			AssessmentTitle:   assessment.Title,
			RequiredAt:        time.Now(),
			QuestionCount:     questionCount,
			PendingAttemptIDs: pendingAttemptIDs,
			CreatorID:         assessment.CreatedBy,
			GraderIDs:         graderIDs,
		},
	}

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

// ===== SYSTEM NOTIFICATIONS =====

func (s *notificationEventService) SendBulkNotification(ctx context.Context, userIDs []uint, notification *NotificationRequest) error {
	s.logger.Info("Publishing bulk notification event",
		"recipient_count", len(userIDs),
		"notification_type", notification.Type)

	// Create and publish event
	event := events.NewBulkNotificationEvent(
		userIDs,
		notification.Type,
		notification.Title,
		notification.Message,
		notification.Priority,
		notification.ActionURL,
		notification.Metadata,
		notification.ScheduledAt,
		0, // TODO: Get sender ID from context
	)

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

// ===== HELPER METHODS =====

// These methods should be implemented based on your specific business logic
// For now, they return placeholder data

func (s *notificationEventService) getEnrolledStudentIDs(ctx context.Context, assessmentID uint) []uint {
	// TODO: Implement based on your enrollment/class management system
	// This might involve querying a separate enrollment service or database table
	s.logger.Debug("Getting enrolled student IDs", "assessment_id", assessmentID)
	return []uint{} // Placeholder
}

func (s *notificationEventService) getStudentsWithIncompleteAssessment(ctx context.Context, assessmentID uint) []uint {
	// TODO: Query students who are enrolled but haven't completed the assessment
	s.logger.Debug("Getting students with incomplete assessment", "assessment_id", assessmentID)
	return []uint{} // Placeholder
}

func (s *notificationEventService) requiresManualGrading(ctx context.Context, attemptID uint) bool {
	// TODO: Check if the attempt has essay questions or other manually graded components
	s.logger.Debug("Checking if attempt requires manual grading", "attempt_id", attemptID)
	return false // Placeholder
}

type GradingStats struct {
	TotalAttempts     int
	AutoGradedCount   int
	ManualGradedCount int
}

func (s *notificationEventService) getGradingStats(ctx context.Context, assessmentID uint) GradingStats {
	// TODO: Get actual grading statistics from the database
	s.logger.Debug("Getting grading statistics", "assessment_id", assessmentID)
	return GradingStats{} // Placeholder
}

func (s *notificationEventService) getPendingManualGradingAttempts(ctx context.Context, assessmentID uint) []uint {
	// TODO: Query attempts that need manual grading
	s.logger.Debug("Getting pending manual grading attempts", "assessment_id", assessmentID)
	return []uint{} // Placeholder
}

func (s *notificationEventService) getAvailableGraderIDs(ctx context.Context, assessmentID uint) []uint {
	// TODO: Get teachers/admins who can grade this assessment
	s.logger.Debug("Getting available grader IDs", "assessment_id", assessmentID)
	return []uint{} // Placeholder
}
