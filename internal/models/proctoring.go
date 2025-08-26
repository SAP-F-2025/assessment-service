package models

import (
	"time"

	"gorm.io/datatypes"
)

type ProctoringEventType string

const (
	EventTabSwitch        ProctoringEventType = "tab_switch"
	EventWindowBlur       ProctoringEventType = "window_blur"
	EventFullscreenExit   ProctoringEventType = "fullscreen_exit"
	EventMultipleFaces    ProctoringEventType = "multiple_faces"
	EventNoFace           ProctoringEventType = "no_face"
	EventSuspiciousObject ProctoringEventType = "suspicious_object"
	EventAudioDetection   ProctoringEventType = "audio_detection"
	EventRightClick       ProctoringEventType = "right_click"
	EventCopyPaste        ProctoringEventType = "copy_paste"
	EventScreenshot       ProctoringEventType = "screenshot"
)

type ProctoringEvent struct {
	ID        uint                `json:"id" gorm:"primaryKey"`
	AttemptID uint                `json:"attempt_id" gorm:"not null;index"`
	Type      ProctoringEventType `json:"type" gorm:"not null;index"`

	// Event data
	Data     datatypes.JSON `json:"data" gorm:"type:jsonb"`
	Severity int            `json:"severity" gorm:"default:1"` // 1-5 (low to critical)

	// Evidence
	ScreenshotURL *string `json:"screenshot_url"`
	VideoURL      *string `json:"video_url"`
	AudioURL      *string `json:"audio_url"`

	// Context
	QuestionID *uint  `json:"question_id" gorm:"index"`
	TimeOffset int    `json:"time_offset"` // Seconds from attempt start
	UserAgent  string `json:"user_agent" gorm:"type:text"`
	IPAddress  string `json:"ip_address" gorm:"size:45"`

	// Review status
	ReviewStatus string     `json:"review_status" gorm:"default:pending"` // pending, reviewed, dismissed
	ReviewedBy   *uint      `json:"reviewed_by"`
	ReviewedAt   *time.Time `json:"reviewed_at"`
	ReviewNotes  *string    `json:"review_notes" gorm:"type:text"`

	CreatedAt time.Time `json:"created_at"`

	// Relations
	Attempt  AssessmentAttempt `json:"attempt" gorm:"foreignKey:AttemptID"`
	Question *Question         `json:"question" gorm:"foreignKey:QuestionID"`
	Reviewer *User             `json:"reviewer" gorm:"foreignKey:ReviewedBy"`
}
