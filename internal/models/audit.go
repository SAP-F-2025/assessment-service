package models

import (
	"time"

	"gorm.io/datatypes"
)

type AuditEventType string

const (
	AuditAssessmentCreated   AuditEventType = "assessment_created"
	AuditAssessmentUpdated   AuditEventType = "assessment_updated"
	AuditAssessmentDeleted   AuditEventType = "assessment_deleted"
	AuditAssessmentPublished AuditEventType = "assessment_published"
	AuditQuestionCreated     AuditEventType = "question_created"
	AuditQuestionUpdated     AuditEventType = "question_updated"
	AuditQuestionDeleted     AuditEventType = "question_deleted"
	AuditAttemptStarted      AuditEventType = "attempt_started"
	AuditAttemptCompleted    AuditEventType = "attempt_completed"
	AuditAnswerSubmitted     AuditEventType = "answer_submitted"
	AuditGradeUpdated        AuditEventType = "grade_updated"
	AuditUserLogin           AuditEventType = "user_login"
	AuditUserLogout          AuditEventType = "user_logout"
	AuditPermissionChanged   AuditEventType = "permission_changed"
	AuditDataExported        AuditEventType = "data_exported"
	AuditProctoringViolation AuditEventType = "proctoring_violation"
)

type AuditLog struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	EventType AuditEventType `json:"event_type" gorm:"not null;index"`

	// Actor information
	UserID    uint     `json:"user_id" gorm:"not null;index"`
	UserEmail string   `json:"user_email" gorm:"not null;size:255"`
	UserRole  UserRole `json:"user_role" gorm:"not null"`

	// Target information
	TargetType string `json:"target_type" gorm:"size:50;index"` // assessment, question, attempt, user
	TargetID   *uint  `json:"target_id" gorm:"index"`

	// Event details
	Description string         `json:"description" gorm:"not null;type:text"`
	Changes     datatypes.JSON `json:"changes" gorm:"type:jsonb"`  // Before/after values
	Metadata    datatypes.JSON `json:"metadata" gorm:"type:jsonb"` // Additional context

	// Request context
	IPAddress string  `json:"ip_address" gorm:"size:45"`
	UserAgent string  `json:"user_agent" gorm:"type:text"`
	RequestID *string `json:"request_id" gorm:"size:36"`

	// Compliance
	ComplianceLevel string `json:"compliance_level" gorm:"size:20"`      // low, medium, high, critical
	RetentionPeriod int    `json:"retention_period" gorm:"default:2555"` // days

	CreatedAt time.Time `json:"created_at" gorm:"index"`

	// Relations
	User User `json:"user" gorm:"foreignKey:UserID"`
}
