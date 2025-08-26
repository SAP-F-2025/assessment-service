package models

import (
	"time"

	"gorm.io/datatypes"
)

type GradingSchemeType string

const (
	GradingPoints     GradingSchemeType = "points"
	GradingPercentage GradingSchemeType = "percentage"
	GradingLetter     GradingSchemeType = "letter"
	GradingCustom     GradingSchemeType = "custom"
)

type GradingScheme struct {
	ID           uint              `json:"id" gorm:"primaryKey"`
	AssessmentID uint              `json:"assessment_id" gorm:"not null;index"`
	Name         string            `json:"name" gorm:"not null;size:100"`
	Type         GradingSchemeType `json:"type" gorm:"not null"`

	// Grading configuration
	PassingScore float64 `json:"passing_score" validate:"min=0,max=100"`
	MaxScore     int     `json:"max_score"`

	// Grade ranges (for letter/custom grading)
	GradeRanges datatypes.JSON `json:"grade_ranges" gorm:"type:jsonb"` // []GradeRange

	// Category weightings
	Weightings datatypes.JSON `json:"weightings" gorm:"type:jsonb"` // []CategoryWeight

	// Settings
	RoundTo       int     `json:"round_to" gorm:"default:2"` // Decimal places
	UseDropLowest int     `json:"use_drop_lowest"`           // Drop N lowest scores
	BonusPoints   float64 `json:"bonus_points"`
	CurvePoints   float64 `json:"curve_points"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Assessment Assessment `json:"assessment" gorm:"foreignKey:AssessmentID"`
}

type GradeRange struct {
	MinScore float64  `json:"min_score"`
	MaxScore float64  `json:"max_score"`
	Grade    string   `json:"grade"`
	GPA      *float64 `json:"gpa"`
	Label    string   `json:"label"`
	Color    string   `json:"color"`
}

type CategoryWeight struct {
	CategoryID *uint   `json:"category_id"` // null for uncategorized
	Weight     float64 `json:"weight"`      // 0.0 - 1.0
	DropLowest int     `json:"drop_lowest"`
}
