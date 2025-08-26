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

// NotificationService handles sending notifications for assessment events
type NotificationService interface {
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

	// Notification management
	GetUserNotifications(ctx context.Context, userID uint, filters NotificationFilters) ([]*models.Notification, error)
	MarkNotificationRead(ctx context.Context, notificationID uint, userID uint) error
	MarkAllNotificationsRead(ctx context.Context, userID uint) error
}

type notificationService struct {
	repo      repositories.Repository
	logger    *slog.Logger
	validator *utils.Validator
}

func NewNotificationService(repo repositories.Repository, logger *slog.Logger, validator *utils.Validator) NotificationService {
	return &notificationService{
		repo:      repo,
		logger:    logger,
		validator: validator,
	}
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

type NotificationFilters struct {
	Type     *models.NotificationType     `json:"type,omitempty"`
	Priority *models.NotificationPriority `json:"priority,omitempty"`
	IsRead   *bool                        `json:"is_read,omitempty"`
	Limit    int                          `json:"limit"`
	Offset   int                          `json:"offset"`
}

// ===== ASSESSMENT NOTIFICATIONS =====

func (s *notificationService) NotifyAssessmentPublished(ctx context.Context, assessmentID uint) error {
	s.logger.Info("Sending assessment published notifications", "assessment_id", assessmentID)

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get enrolled students (TODO: implement enrollment system)
	// For now, we'll skip this and just log
	s.logger.Info("Assessment published notification would be sent to enrolled students",
		"assessment_id", assessmentID,
		"assessment_title", assessment.Title)

	// TODO: Get enrolled students and send notifications
	// studentIDs := s.getEnrolledStudents(ctx, assessmentID)
	//
	// notification := &NotificationRequest{
	// 	Type:     models.NotificationTypeAssessmentPublished,
	// 	Title:    "New Assessment Available",
	// 	Message:  fmt.Sprintf("Assessment '%s' is now available for taking", assessment.Title),
	// 	Priority: models.NotificationPriorityMedium,
	// 	ActionURL: stringPtr(fmt.Sprintf("/assessments/%d", assessmentID)),
	// 	Metadata: map[string]interface{}{
	// 		"assessment_id": assessmentID,
	// 		"due_date":      assessment.DueDate,
	// 	},
	// }
	//
	// return s.SendBulkNotification(ctx, studentIDs, notification)

	return nil
}

func (s *notificationService) NotifyAssessmentExpiring(ctx context.Context, assessmentID uint, hoursRemaining int) error {
	s.logger.Info("Sending assessment expiring notifications",
		"assessment_id", assessmentID,
		"hours_remaining", hoursRemaining)

	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get students who haven't completed the assessment
	studentsWithIncompleteAttempts, err := s.getStudentsWithIncompleteAttempts(ctx, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get students with incomplete attempts: %w", err)
	}

	if len(studentsWithIncompleteAttempts) == 0 {
		return nil // No one to notify
	}

	notification := &NotificationRequest{
		Type:      models.NotificationTypeAssessmentExpiring,
		Title:     "Assessment Expiring Soon",
		Message:   fmt.Sprintf("Assessment '%s' expires in %d hours", assessment.Title, hoursRemaining),
		Priority:  models.NotificationPriorityHigh,
		ActionURL: stringPtr(fmt.Sprintf("/assessments/%d", assessmentID)),
		Metadata: map[string]interface{}{
			"assessment_id":   assessmentID,
			"hours_remaining": hoursRemaining,
			"due_date":        assessment.DueDate,
		},
	}

	return s.SendBulkNotification(ctx, studentsWithIncompleteAttempts, notification)
}

func (s *notificationService) NotifyAssessmentExpired(ctx context.Context, assessmentID uint) error {
	s.logger.Info("Sending assessment expired notifications", "assessment_id", assessmentID)

	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Notify the assessment creator
	notification := &NotificationRequest{
		Type:      models.NotificationTypeAssessmentExpired,
		Title:     "Assessment Expired",
		Message:   fmt.Sprintf("Assessment '%s' has expired", assessment.Title),
		Priority:  models.NotificationPriorityMedium,
		ActionURL: stringPtr(fmt.Sprintf("/assessments/%d/results", assessmentID)),
		Metadata: map[string]interface{}{
			"assessment_id": assessmentID,
		},
	}

	return s.sendNotificationToUser(ctx, assessment.CreatedBy, notification)
}

// ===== ATTEMPT NOTIFICATIONS =====

func (s *notificationService) NotifyAttemptStarted(ctx context.Context, attemptID uint) error {
	s.logger.Debug("Attempt started notification", "attempt_id", attemptID)

	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Notify assessment creator about attempt start
	notification := &NotificationRequest{
		Type:      models.NotificationTypeAttemptStarted,
		Title:     "Student Started Assessment",
		Message:   fmt.Sprintf("A student has started assessment '%s'", attempt.Assessment.Title),
		Priority:  models.NotificationPriorityLow,
		ActionURL: stringPtr(fmt.Sprintf("/assessments/%d/attempts", attempt.AssessmentID)),
		Metadata: map[string]interface{}{
			"attempt_id":    attemptID,
			"assessment_id": attempt.AssessmentID,
			"student_id":    attempt.StudentID,
		},
	}

	return s.sendNotificationToUser(ctx, attempt.Assessment.CreatedBy, notification)
}

func (s *notificationService) NotifyAttemptSubmitted(ctx context.Context, attemptID uint) error {
	s.logger.Info("Sending attempt submitted notifications", "attempt_id", attemptID)

	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Notify student about submission
	studentNotification := &NotificationRequest{
		Type:      models.NotificationTypeAttemptSubmitted,
		Title:     "Assessment Submitted",
		Message:   fmt.Sprintf("Your assessment '%s' has been submitted successfully", attempt.Assessment.Title),
		Priority:  models.NotificationPriorityMedium,
		ActionURL: stringPtr(fmt.Sprintf("/attempts/%d/results", attemptID)),
		Metadata: map[string]interface{}{
			"attempt_id":    attemptID,
			"assessment_id": attempt.AssessmentID,
		},
	}

	if err := s.sendNotificationToUser(ctx, attempt.StudentID, studentNotification); err != nil {
		s.logger.Error("Failed to notify student about submission", "error", err)
	}

	// Notify teacher about submission
	teacherNotification := &NotificationRequest{
		Type:      models.NotificationTypeAttemptSubmitted,
		Title:     "Assessment Submission Received",
		Message:   fmt.Sprintf("A student has submitted assessment '%s'", attempt.Assessment.Title),
		Priority:  models.NotificationPriorityMedium,
		ActionURL: stringPtr(fmt.Sprintf("/assessments/%d/attempts/%d", attempt.AssessmentID, attemptID)),
		Metadata: map[string]interface{}{
			"attempt_id":    attemptID,
			"assessment_id": attempt.AssessmentID,
			"student_id":    attempt.StudentID,
		},
	}

	return s.sendNotificationToUser(ctx, attempt.Assessment.CreatedBy, teacherNotification)
}

func (s *notificationService) NotifyAttemptGraded(ctx context.Context, attemptID uint) error {
	s.logger.Info("Sending attempt graded notifications", "attempt_id", attemptID)

	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	// Notify student about grading completion
	var message string
	if attempt.IsPassing != nil && *attempt.IsPassing {
		message = fmt.Sprintf("Congratulations! You passed assessment '%s' with %.1f%%",
			attempt.Assessment.Title, *attempt.Percentage)
	} else {
		message = fmt.Sprintf("Your assessment '%s' has been graded. Score: %.1f%%",
			attempt.Assessment.Title, *attempt.Percentage)
	}

	notification := &NotificationRequest{
		Type:      models.NotificationTypeAttemptGraded,
		Title:     "Assessment Graded",
		Message:   message,
		Priority:  models.NotificationPriorityHigh,
		ActionURL: stringPtr(fmt.Sprintf("/attempts/%d/results", attemptID)),
		Metadata: map[string]interface{}{
			"attempt_id":    attemptID,
			"assessment_id": attempt.AssessmentID,
			"score":         attempt.TotalScore,
			"percentage":    attempt.Percentage,
			"is_passing":    attempt.IsPassing,
		},
	}

	return s.sendNotificationToUser(ctx, attempt.StudentID, notification)
}

func (s *notificationService) NotifyAttemptTimeWarning(ctx context.Context, attemptID uint, minutesRemaining int) error {
	s.logger.Debug("Sending time warning notification",
		"attempt_id", attemptID,
		"minutes_remaining", minutesRemaining)

	attempt, err := s.repo.Attempt().GetByIDWithDetails(ctx, attemptID)
	if err != nil {
		return fmt.Errorf("failed to get attempt: %w", err)
	}

	notification := &NotificationRequest{
		Type:      models.NotificationTypeTimeWarning,
		Title:     "Time Warning",
		Message:   fmt.Sprintf("You have %d minutes remaining for assessment '%s'", minutesRemaining, attempt.Assessment.Title),
		Priority:  models.NotificationPriorityHigh,
		ActionURL: stringPtr(fmt.Sprintf("/attempts/%d/continue", attemptID)),
		Metadata: map[string]interface{}{
			"attempt_id":        attemptID,
			"minutes_remaining": minutesRemaining,
		},
	}

	return s.sendNotificationToUser(ctx, attempt.StudentID, notification)
}

// ===== GRADING NOTIFICATIONS =====

func (s *notificationService) NotifyGradingCompleted(ctx context.Context, assessmentID uint) error {
	s.logger.Info("Sending grading completed notification", "assessment_id", assessmentID)

	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	notification := &NotificationRequest{
		Type:      models.NotificationTypeGradingCompleted,
		Title:     "Grading Completed",
		Message:   fmt.Sprintf("All submissions for assessment '%s' have been graded", assessment.Title),
		Priority:  models.NotificationPriorityMedium,
		ActionURL: stringPtr(fmt.Sprintf("/assessments/%d/results", assessmentID)),
		Metadata: map[string]interface{}{
			"assessment_id": assessmentID,
		},
	}

	return s.sendNotificationToUser(ctx, assessment.CreatedBy, notification)
}

func (s *notificationService) NotifyManualGradingRequired(ctx context.Context, assessmentID uint, questionCount int) error {
	s.logger.Info("Sending manual grading required notification",
		"assessment_id", assessmentID,
		"question_count", questionCount)

	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	notification := &NotificationRequest{
		Type:      models.NotificationTypeManualGradingRequired,
		Title:     "Manual Grading Required",
		Message:   fmt.Sprintf("Assessment '%s' has %d questions requiring manual grading", assessment.Title, questionCount),
		Priority:  models.NotificationPriorityHigh,
		ActionURL: stringPtr(fmt.Sprintf("/assessments/%d/grading", assessmentID)),
		Metadata: map[string]interface{}{
			"assessment_id":  assessmentID,
			"question_count": questionCount,
		},
	}

	return s.sendNotificationToUser(ctx, assessment.CreatedBy, notification)
}

// ===== BULK NOTIFICATIONS =====

func (s *notificationService) SendBulkNotification(ctx context.Context, userIDs []uint, notification *NotificationRequest) error {
	s.logger.Info("Sending bulk notification",
		"user_count", len(userIDs),
		"type", notification.Type)

	if len(userIDs) == 0 {
		return nil
	}

	// Create notifications for all users
	notifications := make([]*models.Notification, len(userIDs))
	now := time.Now()

	for i, userID := range userIDs {
		notifications[i] = &models.Notification{
			UserID:    userID,
			Type:      notification.Type,
			Title:     notification.Title,
			Message:   notification.Message,
			Priority:  notification.Priority,
			ActionURL: notification.ActionURL,
			Metadata:  notification.Metadata,
			IsRead:    false,
			CreatedAt: now,
		}

		if notification.ScheduledAt != nil {
			notifications[i].ScheduledAt = notification.ScheduledAt
		}
	}

	// Batch create notifications
	if err := s.repo.Notification().CreateBatch(ctx, notifications); err != nil {
		return fmt.Errorf("failed to create bulk notifications: %w", err)
	}

	s.logger.Info("Bulk notification sent successfully", "notification_count", len(notifications))

	// TODO: Send real-time notifications via WebSocket/SSE
	// TODO: Send email/SMS notifications based on user preferences

	return nil
}

// ===== NOTIFICATION MANAGEMENT =====

func (s *notificationService) GetUserNotifications(ctx context.Context, userID uint, filters NotificationFilters) ([]*models.Notification, error) {
	// Set default limit if not specified
	if filters.Limit <= 0 {
		filters.Limit = 50
	}

	notifications, err := s.repo.Notification().GetByUser(ctx, userID, repositories.NotificationFilters{
		Type:     filters.Type,
		Priority: filters.Priority,
		IsRead:   filters.IsRead,
		Limit:    filters.Limit,
		Offset:   filters.Offset,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user notifications: %w", err)
	}

	return notifications, nil
}

func (s *notificationService) MarkNotificationRead(ctx context.Context, notificationID uint, userID uint) error {
	// Get notification to verify ownership
	notification, err := s.repo.Notification().GetByID(ctx, notificationID)
	if err != nil {
		if repositories.IsNotFoundError(err) {
			return ErrNotFound
		}
		return fmt.Errorf("failed to get notification: %w", err)
	}

	// Verify ownership
	if notification.UserID != userID {
		return NewPermissionError(userID, notificationID, "notification", "mark_read", "not owned by user")
	}

	// Mark as read
	notification.IsRead = true
	notification.ReadAt = timePtr(time.Now())

	if err := s.repo.Notification().Update(ctx, notification); err != nil {
		return fmt.Errorf("failed to update notification: %w", err)
	}

	return nil
}

func (s *notificationService) MarkAllNotificationsRead(ctx context.Context, userID uint) error {
	if err := s.repo.Notification().MarkAllReadByUser(ctx, userID); err != nil {
		return fmt.Errorf("failed to mark all notifications read: %w", err)
	}

	s.logger.Info("Marked all notifications as read", "user_id", userID)
	return nil
}

// ===== HELPER FUNCTIONS =====

func (s *notificationService) sendNotificationToUser(ctx context.Context, userID uint, notification *NotificationRequest) error {
	return s.SendBulkNotification(ctx, []uint{userID}, notification)
}

func (s *notificationService) getStudentsWithIncompleteAttempts(ctx context.Context, assessmentID uint) ([]uint, error) {
	// TODO: Implement logic to get students who haven't completed the assessment
	// This would involve:
	// 1. Get all enrolled students for the assessment
	// 2. Check which ones don't have a completed attempt
	// 3. Return their user IDs

	// For now, return empty slice
	return []uint{}, nil
}

func stringPtr(s string) *string {
	return &s
}

func timePtr(t time.Time) *time.Time {
	return &t
}
