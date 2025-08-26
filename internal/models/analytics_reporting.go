package models

import (
	"time"

	"gorm.io/datatypes"
)

type AssessmentAnalytics struct {
	ID           uint `json:"id" gorm:"primaryKey"`
	AssessmentID uint `json:"assessment_id" gorm:"not null;uniqueIndex"`

	// Attempt statistics
	TotalAttempts     int `json:"total_attempts"`
	CompletedAttempts int `json:"completed_attempts"`
	AbandonedAttempts int `json:"abandoned_attempts"`

	// Score statistics
	AverageScore      float64 `json:"average_score"`
	MedianScore       float64 `json:"median_score"`
	HighestScore      float64 `json:"highest_score"`
	LowestScore       float64 `json:"lowest_score"`
	StandardDeviation float64 `json:"standard_deviation"`

	// Time statistics
	AverageTimeSpent int `json:"average_time_spent"` // seconds
	MedianTimeSpent  int `json:"median_time_spent"`

	// Pass/fail statistics
	PassRate    float64 `json:"pass_rate"` // 0.0 - 1.0
	PassedCount int     `json:"passed_count"`
	FailedCount int     `json:"failed_count"`

	// Distribution data
	ScoreDistribution datatypes.JSON `json:"score_distribution" gorm:"type:jsonb"` // []ScoreBucket
	TimeDistribution  datatypes.JSON `json:"time_distribution" gorm:"type:jsonb"`  // []TimeBucket

	LastCalculatedAt time.Time `json:"last_calculated_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Relations
	Assessment Assessment `json:"assessment" gorm:"foreignKey:AssessmentID"`
}

type QuestionAnalytics struct {
	ID         uint `json:"id" gorm:"primaryKey"`
	QuestionID uint `json:"question_id" gorm:"not null;uniqueIndex"`

	// Response statistics
	TotalResponses   int `json:"total_responses"`
	CorrectResponses int `json:"correct_responses"`

	// Performance metrics
	DifficultyIndex     float64 `json:"difficulty_index"`     // % who got it right
	DiscriminationIndex float64 `json:"discrimination_index"` // How well it separates high/low performers

	// Score statistics
	AverageScore     float64 `json:"average_score"`
	AverageTimeSpent int     `json:"average_time_spent"` // seconds

	// Option analysis (for MC questions)
	OptionStats datatypes.JSON `json:"option_stats" gorm:"type:jsonb"` // []OptionStat

	// Performance by difficulty groups
	TopQuartileCorrect    float64 `json:"top_quartile_correct"`
	BottomQuartileCorrect float64 `json:"bottom_quartile_correct"`

	LastCalculatedAt time.Time `json:"last_calculated_at"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`

	// Relations
	Question Question `json:"question" gorm:"foreignKey:QuestionID"`
}
