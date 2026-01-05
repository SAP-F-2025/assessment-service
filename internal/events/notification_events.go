package events

import (
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/google/uuid"
)

// EventType represents different types of notification events
type EventType string

const (
	// Assessment events
	EventAssessmentPublished EventType = "assessment.published"
	EventAssessmentExpiring  EventType = "assessment.expiring"

	// Attempt events
	EventAttemptStarted   EventType = "attempt.started"
	EventAttemptSubmitted EventType = "attempt.submitted"
	EventAttemptGraded    EventType = "attempt.graded"

	// System events
	EventBulkNotification EventType = "system.bulk_notification"
)

// NotificationEvent is the base event structure for all notification events
type NotificationEvent struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Source    string                 `json:"source"`
	Version   string                 `json:"version"`
	Data      interface{}            `json:"data"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

// ===== ASSESSMENT EVENT PAYLOADS =====

// AssessmentPublishedEvent - sent when an assessment is published to a group
type AssessmentPublishedEvent struct {
	AssessmentID    uint       `json:"assessment_id"`
	AssessmentTitle string     `json:"assessment_title"`
	GroupID         uint       `json:"group_id"`
	GroupName       string     `json:"group_name"`
	DueDate         *time.Time `json:"due_date,omitempty"`
	Duration        int        `json:"duration"`    // minutes
	StudentIDs      []string   `json:"student_ids"` // recipients
	CreatorID       string     `json:"creator_id"`  // teacher who created
}

// AssessmentExpiringEvent - reminder sent before assessment due date
type AssessmentExpiringEvent struct {
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	GroupID         uint      `json:"group_id"`
	GroupName       string    `json:"group_name"`
	HoursRemaining  int       `json:"hours_remaining"`
	StudentIDs      []string  `json:"student_ids"` // students who haven't completed
	DueDate         time.Time `json:"due_date"`
}

// ===== ATTEMPT EVENT PAYLOADS =====

// AttemptStartedEvent - sent when student starts an attempt
type AttemptStartedEvent struct {
	AttemptID       uint      `json:"attempt_id"`
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	GroupID         uint      `json:"group_id,omitempty"`
	StudentID       string    `json:"student_id"`
	StartedAt       time.Time `json:"started_at"`
	TimeLimit       *int      `json:"time_limit,omitempty"` // minutes
	CreatorID       string    `json:"creator_id"`           // teacher who created the assessment
}

// AttemptSubmittedEvent - sent when student submits an attempt
type AttemptSubmittedEvent struct {
	AttemptID       uint      `json:"attempt_id"`
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	GroupID         uint      `json:"group_id,omitempty"`
	StudentID       string    `json:"student_id"`
	SubmittedAt     time.Time `json:"submitted_at"`
	Score           *float64  `json:"score,omitempty"`
	MaxScore        *float64  `json:"max_score,omitempty"`
	Passed          *bool     `json:"passed,omitempty"`
	IsPendingGrade  bool      `json:"is_pending_grade"` // requires manual grading
	CreatorID       string    `json:"creator_id"`       // teacher who created the assessment
}

// AttemptGradedEvent - sent when an attempt is fully graded (for student)
type AttemptGradedEvent struct {
	AttemptID       uint      `json:"attempt_id"`
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	GroupID         uint      `json:"group_id,omitempty"`
	StudentID       string    `json:"student_id"` // recipient
	GradedAt        time.Time `json:"graded_at"`
	Score           float64   `json:"score"`
	MaxScore        float64   `json:"max_score"`
	Percentage      float64   `json:"percentage"`
	Passed          bool      `json:"passed"`
	GraderID        string    `json:"grader_id,omitempty"` // teacher who graded
}

// ===== SYSTEM EVENT PAYLOADS =====

// BulkNotificationEvent - for sending bulk notifications
type BulkNotificationEvent struct {
	RecipientIDs []uint                      `json:"recipient_ids"`
	Type         models.NotificationType     `json:"type"`
	Title        string                      `json:"title"`
	Message      string                      `json:"message"`
	Priority     models.NotificationPriority `json:"priority"`
	ActionURL    *string                     `json:"action_url,omitempty"`
	Metadata     map[string]interface{}      `json:"metadata,omitempty"`
	ScheduledAt  *time.Time                  `json:"scheduled_at,omitempty"`
	SenderID     string                      `json:"sender_id"`
}

// ===== EVENT FACTORY FUNCTIONS =====

// NewAssessmentPublishedEvent creates an assessment published event
func NewAssessmentPublishedEvent(assessmentID uint, title string, groupID uint, groupName string, dueDate *time.Time, duration int, studentIDs []string, creatorID string) *NotificationEvent {
	return &NotificationEvent{
		ID:        generateEventID(),
		Type:      EventAssessmentPublished,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: AssessmentPublishedEvent{
			AssessmentID:    assessmentID,
			AssessmentTitle: title,
			GroupID:         groupID,
			GroupName:       groupName,
			DueDate:         dueDate,
			Duration:        duration,
			StudentIDs:      studentIDs,
			CreatorID:       creatorID,
		},
	}
}

// NewAttemptStartedEvent creates an attempt started event
func NewAttemptStartedEvent(attemptID, assessmentID uint, title string, groupID uint, studentID string, startedAt time.Time, timeLimit *int, creatorID string) *NotificationEvent {
	return &NotificationEvent{
		ID:        generateEventID(),
		Type:      EventAttemptStarted,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: AttemptStartedEvent{
			AttemptID:       attemptID,
			AssessmentID:    assessmentID,
			AssessmentTitle: title,
			GroupID:         groupID,
			StudentID:       studentID,
			StartedAt:       startedAt,
			TimeLimit:       timeLimit,
			CreatorID:       creatorID,
		},
	}
}

// NewBulkNotificationEvent creates a bulk notification event
func NewBulkNotificationEvent(recipientIDs []uint, notificationType models.NotificationType, title, message string, priority models.NotificationPriority, actionURL *string, metadata map[string]interface{}, scheduledAt *time.Time, senderID string) *NotificationEvent {
	return &NotificationEvent{
		ID:        generateEventID(),
		Type:      EventBulkNotification,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: BulkNotificationEvent{
			RecipientIDs: recipientIDs,
			Type:         notificationType,
			Title:        title,
			Message:      message,
			Priority:     priority,
			ActionURL:    actionURL,
			Metadata:     metadata,
			ScheduledAt:  scheduledAt,
			SenderID:     senderID,
		},
	}
}

// Helper function to generate unique event IDs
func generateEventID() string {
	return uuid.NewString()
}

// GenerateEventID is the exported version for external use
func GenerateEventID() string {
	return generateEventID()
}
