package models

import (
	"time"

	"gorm.io/datatypes"
)

type ImportJobStatus string

const (
	ImportPending          ImportJobStatus = "pending"
	ImportProcessing       ImportJobStatus = "processing"
	ImportCompleted        ImportJobStatus = "completed"
	ImportFailed           ImportJobStatus = "failed"
	ImportValidationFailed ImportJobStatus = "validation_failed"
)

type ImportJob struct {
	ID           string `json:"id" gorm:"primaryKey;size:36"` // UUID
	AssessmentID *uint  `json:"assessment_id" gorm:"index"`   // null for question bank import
	BankID       *uint  `json:"bank_id" gorm:"index"`         // null for assessment import
	UserID       string `json:"user_id" gorm:"not null;index;size:255"`

	// File info
	FileName string `json:"file_name" gorm:"not null;size:255"`
	FileType string `json:"file_type" gorm:"not null;size:20"` // xlsx, csv, json
	FileSize int64  `json:"file_size" gorm:"not null"`
	FilePath string `json:"file_path" gorm:"not null;size:500"`

	// Job status
	Status   ImportJobStatus `json:"status" gorm:"default:pending;index"`
	Progress int             `json:"progress" gorm:"default:0"` // 0-100

	// Processing info
	TotalRows     int `json:"total_rows"`
	ProcessedRows int `json:"processed_rows"`
	SuccessCount  int `json:"success_count"`
	ErrorCount    int `json:"error_count"`

	// Results
	Errors  datatypes.JSON `json:"errors" gorm:"type:jsonb"` // []ImportValidationError
	Summary datatypes.JSON `json:"summary" gorm:"type:jsonb"`

	// Timestamps
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`

	// Relations
	Assessment *Assessment   `json:"assessment" gorm:"foreignKey:AssessmentID"`
	Bank       *QuestionBank `json:"bank" gorm:"foreignKey:BankID"`
	User       User          `json:"user" gorm:"foreignKey:UserID"`
}

type ImportValidationError struct {
	Row     int    `json:"row"`
	Column  string `json:"column"`
	Message string `json:"message"`
	Value   string `json:"value"`
	Code    string `json:"code"`
}
