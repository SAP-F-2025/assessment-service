package models

import (
	"time"

	"gorm.io/gorm"
)

// AssessmentGroup represents M:N relationship between assessments and groups
// Controls which groups have access to which assessments (PRIVATE visibility model)
type AssessmentGroup struct {
	ID           uint `json:"id" gorm:"primaryKey"`
	AssessmentID uint `json:"assessment_id" gorm:"not null;index"`
	GroupID      uint `json:"group_id" gorm:"not null;index"`

	// Assignment metadata
	AssignedAt time.Time `json:"assigned_at" gorm:"not null;autoCreateTime"`
	AssignedBy string    `json:"assigned_by" gorm:"not null;size:255;index"` // Teacher who assigned

	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"` // Soft delete for audit

	// Relations
	Assessment *Assessment `json:"assessment,omitempty" gorm:"foreignKey:AssessmentID"`
	Group      *Group      `json:"group,omitempty" gorm:"foreignKey:GroupID"`
}

// TableName specifies the table name for AssessmentGroup model
func (AssessmentGroup) TableName() string {
	return "assessment_groups"
}
