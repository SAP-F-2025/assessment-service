package models

import (
	"time"

	"gorm.io/datatypes"
)

type NotificationType string
type NotificationPriority int // Add missing type

const (
	// Notification types
	NotificationAssessmentPublished NotificationType = "assessment_published"
	NotificationAssessmentDue       NotificationType = "assessment_due"
	NotificationResultAvailable     NotificationType = "result_available"
	NotificationProctoringViolation NotificationType = "proctoring_violation"
	NotificationAssessmentExpired   NotificationType = "assessment_expired"
	NotificationQuestionBankShared  NotificationType = "question_bank_shared"
	NotificationImportCompleted     NotificationType = "import_completed"
	NotificationSystemMaintenance   NotificationType = "system_maintenance"

	// Priority levels
	PriorityLow      NotificationPriority = 1
	PriorityNormal   NotificationPriority = 2
	PriorityHigh     NotificationPriority = 3
	PriorityCritical NotificationPriority = 4
)

type Notification struct {
	ID      uint             `json:"id" gorm:"primaryKey"`
	Type    NotificationType `json:"type" gorm:"not null;index"`
	Title   string           `json:"title" gorm:"not null;size:255"`
	Message string           `json:"message" gorm:"type:text"`

	// Recipients
	RecipientID   *uint     `json:"recipient_id" gorm:"index"` // null for broadcast
	RecipientRole *UserRole `json:"recipient_role"`            // null for specific user

	// Related entities
	AssessmentID *uint `json:"assessment_id" gorm:"index"`
	AttemptID    *uint `json:"attempt_id" gorm:"index"`

	// Delivery settings
	Channels datatypes.JSON `json:"channels" gorm:"type:jsonb"` // ["email", "push", "in_app"]
	Priority int            `json:"priority" gorm:"default:1"`  // 1-5

	// Status
	SentAt         *time.Time `json:"sent_at"`
	ReadAt         *time.Time `json:"read_at"`
	DeliveryStatus string     `json:"delivery_status" gorm:"default:pending"`

	// Scheduling
	ScheduledFor *time.Time `json:"scheduled_for"`
	ExpiresAt    *time.Time `json:"expires_at"`

	CreatedAt time.Time `json:"created_at"`
	CreatedBy uint      `json:"created_by" gorm:"not null"`

	// Relations
	Recipient  *User              `json:"recipient" gorm:"foreignKey:RecipientID"`
	Assessment *Assessment        `json:"assessment" gorm:"foreignKey:AssessmentID"`
	Attempt    *AssessmentAttempt `json:"attempt" gorm:"foreignKey:AttemptID"`
	Creator    User               `json:"creator" gorm:"foreignKey:CreatedBy"`
}
