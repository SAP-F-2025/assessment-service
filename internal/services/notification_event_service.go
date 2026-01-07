package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/events"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
)

// NotificationEventService handles sending notifications through event publishing
// This replaces the direct notification service with an event-driven approach
type NotificationEventService interface {
	// Assessment notifications (with group scope)
	NotifyAssessmentPublished(ctx context.Context, assessmentID uint, groupID uint) error
	NotifyAssessmentExpiring(ctx context.Context, assessmentID uint, groupID uint, hoursRemaining int) error

	// Attempt notifications
	NotifyAttemptStarted(ctx context.Context, attemptID uint) error
	NotifyAttemptSubmitted(ctx context.Context, attemptID uint) error
	NotifyAttemptGraded(ctx context.Context, attemptID uint) error

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
	validator      *validator.Validator
}

func NewNotificationEventService(
	repo repositories.Repository,
	eventPublisher events.EventPublisher,
	logger *slog.Logger,
	validator *validator.Validator,
) NotificationEventService {
	return &notificationEventService{
		repo:           repo,
		eventPublisher: eventPublisher,
		logger:         logger,
		validator:      validator,
	}
}

// ===== ASSESSMENT NOTIFICATIONS =====

func (s *notificationEventService) NotifyAssessmentPublished(ctx context.Context, assessmentID uint, groupID uint) error {
	s.logger.Info("Publishing assessment published event",
		"assessment_id", assessmentID,
		"group_id", groupID)

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, nil, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get group info
	groupName := ""
	if groupID > 0 {
		group, err := s.getGroupInfo(ctx, groupID)
		if err == nil && group != nil {
			groupName = group.DisplayName
		}
	}

	// Get enrolled students from the group
	studentIDs := s.getEnrolledStudentIDsFromGroup(ctx, groupID)

	// Create and publish event
	event := events.NewAssessmentPublishedEvent(
		assessmentID,
		assessment.Title,
		groupID,
		groupName,
		assessment.DueDate,
		assessment.Duration,
		studentIDs,
		assessment.CreatedBy,
	)

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

func (s *notificationEventService) NotifyAssessmentExpiring(ctx context.Context, assessmentID uint, groupID uint, hoursRemaining int) error {
	s.logger.Info("Publishing assessment expiring event",
		"assessment_id", assessmentID,
		"group_id", groupID,
		"hours_remaining", hoursRemaining)

	// Get assessment details
	assessment, err := s.repo.Assessment().GetByIDWithDetails(ctx, nil, assessmentID)
	if err != nil {
		return fmt.Errorf("failed to get assessment: %w", err)
	}

	// Get group info
	groupName := ""
	if groupID > 0 {
		group, err := s.getGroupInfo(ctx, groupID)
		if err == nil && group != nil {
			groupName = group.DisplayName
		}
	}

	// Get enrolled students who haven't completed the assessment
	studentIDs := s.getStudentsWithIncompleteAssessment(ctx, assessmentID, groupID)

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
			GroupID:         groupID,
			GroupName:       groupName,
			HoursRemaining:  hoursRemaining,
			StudentIDs:      studentIDs,
			DueDate:         *assessment.DueDate,
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

	// Get groupID from assessment_groups if available
	groupID := s.getGroupIDForAssessment(ctx, attempt.AssessmentID)

	// Create and publish event
	event := events.NewAttemptStartedEvent(
		attemptID,
		attempt.AssessmentID,
		attempt.Assessment.Title,
		groupID,
		attempt.StudentID,
		*attempt.StartedAt,
		&attempt.Assessment.Duration,
		attempt.Assessment.CreatedBy, // teacher who created the assessment
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

	// Get groupID from assessment_groups if available
	groupID := s.getGroupIDForAssessment(ctx, attempt.AssessmentID)

	// Check if pending grade from attempt answers
	isPendingGrade := s.checkIsPendingGrade(ctx, attemptID)

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
			GroupID:         groupID,
			StudentID:       attempt.StudentID,
			SubmittedAt:     *attempt.CompletedAt,
			Score:           &attempt.Score,
			MaxScore:        &attempt.MaxScore,
			Passed:          &attempt.Passed,
			IsPendingGrade:  isPendingGrade,
			CreatorID:       attempt.Assessment.CreatedBy, // teacher who created the assessment
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

	// Get groupID from assessment_groups if available
	groupID := s.getGroupIDForAssessment(ctx, attempt.AssessmentID)

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
			GroupID:         groupID,
			StudentID:       attempt.StudentID,
			GradedAt:        time.Now(),
			Score:           attempt.Score,
			MaxScore:        attempt.MaxScore,
			Percentage:      attempt.Percentage,
			Passed:          attempt.Passed,
			GraderID:        attempt.Assessment.CreatedBy,
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
		"0", // TODO: Get sender ID from context
	)

	return s.eventPublisher.PublishNotificationEvent(ctx, event)
}

// ===== HELPER METHODS =====

// getGroupInfo retrieves group information by ID
func (s *notificationEventService) getGroupInfo(ctx context.Context, groupID uint) (*models.Group, error) {
	if groupID == 0 {
		return nil, nil
	}

	group, err := s.repo.Group().GetByID(ctx, nil, groupID)
	if err != nil {
		s.logger.Debug("Failed to get group info", "group_id", groupID, "error", err)
		return nil, err
	}

	return group, nil
}

// getGroupIDForAssessment returns the first group ID associated with an assessment
func (s *notificationEventService) getGroupIDForAssessment(ctx context.Context, assessmentID uint) uint {
	groups, err := s.repo.AssessmentGroup().GetGroupsByAssessment(ctx, nil, assessmentID)
	if err != nil || len(groups) == 0 {
		s.logger.Debug("No groups found for assessment", "assessment_id", assessmentID)
		return 0
	}

	// Return the first group ID
	return groups[0].ID
}

// getEnrolledStudentIDsFromGroup returns student IDs from a specific group
func (s *notificationEventService) getEnrolledStudentIDsFromGroup(ctx context.Context, groupID uint) []string {
	if groupID == 0 {
		return []string{}
	}

	members, err := s.repo.Group().GetMembers(ctx, nil, groupID)
	if err != nil {
		s.logger.Debug("Failed to get group members", "group_id", groupID, "error", err)
		return []string{}
	}

	// Filter for members with role "member" (students)
	studentIDs := make([]string, 0, len(members))
	for _, member := range members {
		if member.Role == models.GroupMemberRoleMember {
			studentIDs = append(studentIDs, member.UserID)
		}
	}

	return studentIDs
}

// getStudentsWithIncompleteAssessment returns students who haven't completed the assessment
func (s *notificationEventService) getStudentsWithIncompleteAssessment(ctx context.Context, assessmentID uint, groupID uint) []string {
	// Get all students in the group
	allStudents := s.getEnrolledStudentIDsFromGroup(ctx, groupID)
	if len(allStudents) == 0 {
		return []string{}
	}

	// Get all attempts for this assessment (completed or not)
	completedStatus := models.AttemptCompleted
	attempts, _, err := s.repo.Attempt().GetByAssessment(ctx, nil, assessmentID, repositories.AttemptFilters{
		Status: &completedStatus,
	})
	if err != nil {
		s.logger.Debug("Failed to get attempts", "assessment_id", assessmentID, "error", err)
		return allStudents // Return all if can't check
	}

	// Create a set of students who have completed
	completedStudents := make(map[string]bool)
	for _, attempt := range attempts {
		completedStudents[attempt.StudentID] = true
	}

	// Filter out students who have completed
	incompleteStudents := make([]string, 0)
	for _, studentID := range allStudents {
		if !completedStudents[studentID] {
			incompleteStudents = append(incompleteStudents, studentID)
		}
	}

	return incompleteStudents
}

// checkIsPendingGrade checks if an attempt has answers requiring manual grading
func (s *notificationEventService) checkIsPendingGrade(ctx context.Context, attemptID uint) bool {
	// Use AreAllAnswersGraded to check if any answer still needs grading
	allGraded, err := s.repo.Answer().AreAllAnswersGraded(ctx, nil, attemptID)
	if err != nil {
		s.logger.Debug("Failed to check grading status", "attempt_id", attemptID, "error", err)
		return false
	}

	// If not all graded, there are pending grades
	return !allGraded
}

type GradingStats struct {
	TotalAttempts     int
	AutoGradedCount   int
	ManualGradedCount int
}

func (s *notificationEventService) getGradingStats(ctx context.Context, assessmentID uint) GradingStats {
	completedStatus := models.AttemptCompleted
	attempts, _, err := s.repo.Attempt().GetByAssessment(ctx, nil, assessmentID, repositories.AttemptFilters{
		Status: &completedStatus,
	})
	if err != nil {
		s.logger.Debug("Failed to get attempts for grading stats", "assessment_id", assessmentID, "error", err)
		return GradingStats{}
	}

	stats := GradingStats{
		TotalAttempts: len(attempts),
	}

	for _, attempt := range attempts {
		// Check if attempt has pending grades
		isPending := s.checkIsPendingGrade(ctx, attempt.ID)
		if isPending {
			stats.ManualGradedCount++
		} else {
			stats.AutoGradedCount++
		}
	}

	return stats
}
