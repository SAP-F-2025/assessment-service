package repositories

import (
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// ===== SHARED FILTER STRUCTS =====

type AssessmentFilters struct {
	Status    *models.AssessmentStatus `json:"status"`
	CreatedBy *uint                    `json:"created_by"`
	DateFrom  *time.Time               `json:"date_from"`
	DateTo    *time.Time               `json:"date_to"`
	Limit     int                      `json:"limit"`
	Offset    int                      `json:"offset"`
	SortBy    string                   `json:"sort_by"`    // "created_at", "title", "due_date"
	SortOrder string                   `json:"sort_order"` // "asc", "desc"
}

type QuestionFilters struct {
	Type       *models.QuestionType    `json:"type"`
	Difficulty *models.DifficultyLevel `json:"difficulty"`
	CategoryID *uint                   `json:"category_id"`
	CreatedBy  *uint                   `json:"created_by"`
	Tags       []string                `json:"tags"`
	Limit      int                     `json:"limit"`
	Offset     int                     `json:"offset"`
	SortBy     string                  `json:"sort_by"`
	SortOrder  string                  `json:"sort_order"`
}

type RandomQuestionFilters struct {
	CategoryID *uint                   `json:"category_id"`
	Difficulty *models.DifficultyLevel `json:"difficulty"`
	Type       *models.QuestionType    `json:"type"`
	ExcludeIDs []uint                  `json:"exclude_ids"`
	Count      int                     `json:"count"`
}

type AttemptFilters struct {
	Status    models.AttemptStatus `json:"status"`
	StudentID *uint                `json:"student_id"`
	DateFrom  *time.Time           `json:"date_from"`
	DateTo    *time.Time           `json:"date_to"`
	Limit     int                  `json:"limit"`
	Offset    int                  `json:"offset"`
	SortBy    string               `json:"sort_by"`    // "created_at", "title", "due_date"
	SortOrder string               `json:"sort_order"` // "asc", "desc"
}

type AnswerFilters struct {
	IsGraded *bool      `json:"is_graded"`
	GradedBy *uint      `json:"graded_by"`
	DateFrom *time.Time `json:"date_from"`
	DateTo   *time.Time `json:"date_to"`
	Limit    int        `json:"limit"`
	Offset   int        `json:"offset"`
}

// ===== SHARED HELPER STRUCTS =====

type QuestionOrder struct {
	QuestionID uint `json:"question_id"`
	Order      int  `json:"order"`
}

type AnswerGrade struct {
	ID       uint    `json:"id"`
	Score    float64 `json:"score"`
	Feedback *string `json:"feedback"`
	GraderID uint    `json:"grader_id"`
}

// ===== SHARED STATISTICS STRUCTS =====

type AssessmentStats struct {
	TotalAttempts     int     `json:"total_attempts"`
	CompletedAttempts int     `json:"completed_attempts"`
	AverageScore      float64 `json:"average_score"`
	PassRate          float64 `json:"pass_rate"`
	AverageTimeSpent  int     `json:"average_time_spent"`
	QuestionCount     int     `json:"question_count"`
	TotalPoints       int     `json:"total_points"`
}

type CreatorStats struct {
	TotalAssessments  int `json:"total_assessments"`
	ActiveAssessments int `json:"active_assessments"`
	DraftAssessments  int `json:"draft_assessments"`
	TotalQuestions    int `json:"total_questions"`
	TotalAttempts     int `json:"total_attempts"`
}

type QuestionStats struct {
	UsageCount       int     `json:"usage_count"`
	CorrectRate      float64 `json:"correct_rate"`
	AverageScore     float64 `json:"average_score"`
	AverageTimeSpent int     `json:"average_time_spent"`
	DifficultyActual float64 `json:"difficulty_actual"`
}

type QuestionUsageStats struct {
	TotalQuestions    int                            `json:"total_questions"`
	QuestionsByType   map[models.QuestionType]int    `json:"questions_by_type"`
	QuestionsByDiff   map[models.DifficultyLevel]int `json:"questions_by_difficulty"`
	MostUsedQuestions []*QuestionUsageStat           `json:"most_used_questions"`
}

type QuestionUsageStat struct {
	QuestionID  uint    `json:"question_id"`
	Title       string  `json:"title"`
	UsageCount  int     `json:"usage_count"`
	CorrectRate float64 `json:"correct_rate"`
}

type AttemptStats struct {
	TotalAttempts    int                          `json:"total_attempts"`
	StatusBreakdown  map[models.AttemptStatus]int `json:"status_breakdown"`
	AverageScore     float64                      `json:"average_score"`
	AverageTimeSpent int                          `json:"average_time_spent"`
	PassRate         float64                      `json:"pass_rate"`
	CompletionRate   float64                      `json:"completion_rate"`
}

type GradingStats struct {
	TotalAnswers   int     `json:"total_answers"`
	GradedAnswers  int     `json:"graded_answers"`
	PendingAnswers int     `json:"pending_answers"`
	AutoGraded     int     `json:"auto_graded"`
	ManualGraded   int     `json:"manual_graded"`
	AverageScore   float64 `json:"average_score"`
}
