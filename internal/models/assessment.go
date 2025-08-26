package models

import (
	"time"

	"gorm.io/gorm"
)

type AssessmentStatus string

const (
	StatusDraft    AssessmentStatus = "Draft"
	StatusActive   AssessmentStatus = "Active"
	StatusExpired  AssessmentStatus = "Expired"
	StatusArchived AssessmentStatus = "Archived"
)

type Assessment struct {
	ID           uint             `json:"id" gorm:"primaryKey"`
	Title        string           `json:"title" gorm:"not null;size:200;index" validate:"required,min=1,max=200"`
	Description  *string          `json:"description" gorm:"type:text" validate:"omitempty,max=1000"`
	Duration     int              `json:"duration" gorm:"not null" validate:"required,min=5,max=300"`
	Status       AssessmentStatus `json:"status" gorm:"default:Draft;index" validate:"omitempty,oneof=Draft Active Expired Archived"`
	PassingScore int              `json:"passing_score" gorm:"not null" validate:"required,min=0,max=100"`
	MaxAttempts  int              `json:"max_attempts" gorm:"default:1" validate:"min=1,max=10"`
	TimeWarning  int              `json:"time_warning" gorm:"default:300"` // Warning time in seconds
	DueDate      *time.Time       `json:"due_date"`

	// Metadata
	CreatedBy uint           `json:"created_by" gorm:"not null;index"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Version control
	Version int `json:"version" gorm:"default:1"`

	// Relations
	Settings  AssessmentSettings   `json:"settings" gorm:"foreignKey:AssessmentID"`
	Questions []AssessmentQuestion `json:"questions" gorm:"foreignKey:AssessmentID"`
	Attempts  []AssessmentAttempt  `json:"attempts" gorm:"foreignKey:AssessmentID"`
	Creator   User                 `json:"creator" gorm:"foreignKey:CreatedBy"`

	// Computed fields (not stored)
	QuestionsCount int     `json:"questions_count" gorm:"-"`
	TotalPoints    int     `json:"total_points" gorm:"-"`
	AttemptCount   int     `json:"attempt_count" gorm:"-"`
	AvgScore       float64 `json:"avg_score" gorm:"-"`
}

type AssessmentSettings struct {
	AssessmentID uint `json:"assessment_id" gorm:"primaryKey"`

	// Question Display Settings
	RandomizeQuestions bool `json:"randomize_questions" gorm:"default:false"`
	RandomizeOptions   bool `json:"randomize_options" gorm:"default:false"`
	QuestionsPerPage   int  `json:"questions_per_page" gorm:"default:1"`
	ShowProgressBar    bool `json:"show_progress_bar" gorm:"default:true"`

	// Result Settings
	ShowResults        bool `json:"show_results" gorm:"default:true"`
	ShowCorrectAnswers bool `json:"show_correct_answers" gorm:"default:true"`
	ShowScoreBreakdown bool `json:"show_score_breakdown" gorm:"default:true"`

	// Attempt Settings
	AllowRetake bool `json:"allow_retake" gorm:"default:false"`
	RetakeDelay int  `json:"retake_delay" gorm:"default:0"` // Minutes

	// Time Settings
	TimeLimitEnforced   bool `json:"time_limit_enforced" gorm:"default:true"`
	AutoSubmitOnTimeout bool `json:"auto_submit_on_timeout" gorm:"default:true"`

	// Proctoring Settings
	RequireWebcam               bool `json:"require_webcam" gorm:"default:false"`
	PreventTabSwitching         bool `json:"prevent_tab_switching" gorm:"default:false"`
	PreventRightClick           bool `json:"prevent_right_click" gorm:"default:false"`
	PreventCopyPaste            bool `json:"prevent_copy_paste" gorm:"default:false"`
	RequireIdentityVerification bool `json:"require_identity_verification" gorm:"default:false"`
	RequireFullScreen           bool `json:"require_full_screen" gorm:"default:false"`

	// Accessibility Settings
	AllowScreenReader  bool `json:"allow_screen_reader" gorm:"default:false"`
	FontSizeAdjustment int  `json:"font_size_adjustment" gorm:"default:0"` // -2 to +2
	HighContrastMode   bool `json:"high_contrast_mode" gorm:"default:false"`
}

func (Assessment) TableName() string {
	return "assessments"
}
