package events

import (
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// EventType represents different types of notification events
type EventType string

const (
	// Assessment events
	EventAssessmentPublished EventType = "assessment.published"
	EventAssessmentExpiring  EventType = "assessment.expiring"
	EventAssessmentExpired   EventType = "assessment.expired"

	// Attempt events
	EventAttemptStarted     EventType = "attempt.started"
	EventAttemptSubmitted   EventType = "attempt.submitted"
	EventAttemptGraded      EventType = "attempt.graded"
	EventAttemptTimeWarning EventType = "attempt.time_warning"

	// Grading events
	EventGradingCompleted      EventType = "grading.completed"
	EventManualGradingRequired EventType = "grading.manual_required"

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

// Assessment notification event payloads

type AssessmentPublishedEvent struct {
	AssessmentID    uint       `json:"assessment_id"`
	AssessmentTitle string     `json:"assessment_title"`
	DueDate         *time.Time `json:"due_date,omitempty"`
	Duration        int        `json:"duration"` // minutes
	StudentIDs      []uint     `json:"student_ids"`
	CreatorID       uint       `json:"creator_id"`
}

type AssessmentExpiringEvent struct {
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	HoursRemaining  int       `json:"hours_remaining"`
	StudentIDs      []uint    `json:"student_ids"`
	DueDate         time.Time `json:"due_date"`
}

type AssessmentExpiredEvent struct {
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	ExpiredAt       time.Time `json:"expired_at"`
	StudentIDs      []uint    `json:"student_ids"`
	CreatorID       uint      `json:"creator_id"`
}

// Attempt notification event payloads

type AttemptStartedEvent struct {
	AttemptID       uint      `json:"attempt_id"`
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	StudentID       uint      `json:"student_id"`
	StartedAt       time.Time `json:"started_at"`
	TimeLimit       *int      `json:"time_limit,omitempty"` // minutes
}

type AttemptSubmittedEvent struct {
	AttemptID       uint      `json:"attempt_id"`
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	StudentID       uint      `json:"student_id"`
	SubmittedAt     time.Time `json:"submitted_at"`
	Score           *float64  `json:"score,omitempty"`
	Passed          *bool     `json:"passed,omitempty"`
	GradingRequired bool      `json:"grading_required"`
}

type AttemptGradedEvent struct {
	AttemptID       uint      `json:"attempt_id"`
	AssessmentID    uint      `json:"assessment_id"`
	AssessmentTitle string    `json:"assessment_title"`
	StudentID       uint      `json:"student_id"`
	GradedAt        time.Time `json:"graded_at"`
	Score           float64   `json:"score"`
	MaxScore        int       `json:"max_score"`
	Percentage      float64   `json:"percentage"`
	Passed          bool      `json:"passed"`
	GraderID        uint      `json:"grader_id"`
}

type AttemptTimeWarningEvent struct {
	AttemptID        uint      `json:"attempt_id"`
	AssessmentID     uint      `json:"assessment_id"`
	AssessmentTitle  string    `json:"assessment_title"`
	StudentID        uint      `json:"student_id"`
	MinutesRemaining int       `json:"minutes_remaining"`
	WarningTime      time.Time `json:"warning_time"`
}

// Grading notification event payloads

type GradingCompletedEvent struct {
	AssessmentID      uint      `json:"assessment_id"`
	AssessmentTitle   string    `json:"assessment_title"`
	CompletedAt       time.Time `json:"completed_at"`
	TotalAttempts     int       `json:"total_attempts"`
	AutoGradedCount   int       `json:"auto_graded_count"`
	ManualGradedCount int       `json:"manual_graded_count"`
	CreatorID         uint      `json:"creator_id"`
}

type ManualGradingRequiredEvent struct {
	AssessmentID      uint      `json:"assessment_id"`
	AssessmentTitle   string    `json:"assessment_title"`
	RequiredAt        time.Time `json:"required_at"`
	QuestionCount     int       `json:"question_count"`
	PendingAttemptIDs []uint    `json:"pending_attempt_ids"`
	CreatorID         uint      `json:"creator_id"`
	GraderIDs         []uint    `json:"grader_ids"`
}

// System notification event payload

type BulkNotificationEvent struct {
	RecipientIDs []uint                      `json:"recipient_ids"`
	Type         models.NotificationType     `json:"type"`
	Title        string                      `json:"title"`
	Message      string                      `json:"message"`
	Priority     models.NotificationPriority `json:"priority"`
	ActionURL    *string                     `json:"action_url,omitempty"`
	Metadata     map[string]interface{}      `json:"metadata,omitempty"`
	ScheduledAt  *time.Time                  `json:"scheduled_at,omitempty"`
	SenderID     uint                        `json:"sender_id"`
}

// Event factory functions

func NewAssessmentPublishedEvent(assessmentID uint, title string, dueDate *time.Time, duration int, studentIDs []uint, creatorID uint) *NotificationEvent {
	return &NotificationEvent{
		ID:        generateEventID(),
		Type:      EventAssessmentPublished,
		Timestamp: time.Now(),
		Source:    "assessment-service",
		Version:   "1.0",
		Data: AssessmentPublishedEvent{
			AssessmentID:    assessmentID,
			AssessmentTitle: title,
			DueDate:         dueDate,
			Duration:        duration,
			StudentIDs:      studentIDs,
			CreatorID:       creatorID,
		},
	}
}

func NewAttemptStartedEvent(attemptID, assessmentID uint, title string, studentID uint, startedAt time.Time, timeLimit *int) *NotificationEvent {
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
			StudentID:       studentID,
			StartedAt:       startedAt,
			TimeLimit:       timeLimit,
		},
	}
}

func NewBulkNotificationEvent(recipientIDs []uint, notificationType models.NotificationType, title, message string, priority models.NotificationPriority, actionURL *string, metadata map[string]interface{}, scheduledAt *time.Time, senderID uint) *NotificationEvent {
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
	// You can use UUID library here
	return time.Now().Format("20060102150405") + "-" + "uuid-placeholder"
}

// GenerateEventID is the exported version for external use
func GenerateEventID() string {
	return generateEventID()
}
