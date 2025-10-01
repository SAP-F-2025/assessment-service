package repositories

import (
	"context"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"gorm.io/gorm"
)

// AttemptRepository interface for assessment attempt operations
type AttemptRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, tx *gorm.DB, attempt *models.AssessmentAttempt) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.AssessmentAttempt, error)
	GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.AssessmentAttempt, error) // Include answers, assessment
	Update(ctx context.Context, tx *gorm.DB, attempt *models.AssessmentAttempt) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error

	// Query operations
	List(ctx context.Context, tx *gorm.DB, filters AttemptFilters) ([]*models.AssessmentAttempt, int64, error)
	GetByStudent(ctx context.Context, tx *gorm.DB, studentID string, filters AttemptFilters) ([]*models.AssessmentAttempt, int64, error)
	GetByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint, filters AttemptFilters) ([]*models.AssessmentAttempt, int64, error)
	GetByStudentAndAssessment(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) ([]*models.AssessmentAttempt, error)

	// Active attempt management
	GetActiveAttempt(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (*models.AssessmentAttempt, error)
	HasActiveAttempt(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (bool, error)
	GetActiveAttempts(ctx context.Context, tx *gorm.DB, studentID string) ([]*models.AssessmentAttempt, error)

	// Status management
	UpdateStatus(ctx context.Context, tx *gorm.DB, id uint, status models.AttemptStatus) error
	BulkUpdateStatus(ctx context.Context, tx *gorm.DB, ids []uint, status models.AttemptStatus) error
	GetByStatus(ctx context.Context, tx *gorm.DB, status models.AttemptStatus, limit, offset int) ([]*models.AssessmentAttempt, error)

	// Time management
	UpdateTimeRemaining(ctx context.Context, tx *gorm.DB, id uint, timeRemaining int) error
	GetInProgressAttempts(ctx context.Context, tx *gorm.DB) ([]*models.AssessmentAttempt, error)
	GetTimedOutAttempts(ctx context.Context, tx *gorm.DB) ([]*models.AssessmentAttempt, error)
	GetExpiredAttempts(ctx context.Context, tx *gorm.DB, cutoffTime time.Time) ([]*models.AssessmentAttempt, error)

	// Progress tracking
	UpdateProgress(ctx context.Context, tx *gorm.DB, id uint, currentQuestionIndex, questionsAnswered int) error
	GetProgress(ctx context.Context, tx *gorm.DB, id uint) (*AttemptProgress, error)

	// Scoring and completion
	UpdateScore(ctx context.Context, tx *gorm.DB, id uint, score, percentage float64, passed bool) error
	CompleteAttempt(ctx context.Context, tx *gorm.DB, id uint, completedAt time.Time, finalScore float64) error

	// Statistics and analytics
	GetAttemptCount(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (int, error)
	GetAssessmentAttemptStats(ctx context.Context, tx *gorm.DB, assessmentID uint) (*AttemptStats, error)
	GetStudentAttemptStats(ctx context.Context, tx *gorm.DB, studentID string) (*StudentAttemptStats, error)
	GetAttemptsByDateRange(ctx context.Context, tx *gorm.DB, from, to time.Time) ([]*models.AssessmentAttempt, error)

	// Validation and checks
	CanStartAttempt(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (*AttemptValidation, error)
	GetNextAttemptNumber(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (int, error)
	HasCompletedAttempts(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (bool, error)

	// Session management
	UpdateSessionData(ctx context.Context, tx *gorm.DB, id uint, sessionData interface{}) error
	GetSessionData(ctx context.Context, tx *gorm.DB, id uint) (interface{}, error)
}

// AnswerRepository interface for student answer operations
type AnswerRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, tx *gorm.DB, answer *models.StudentAnswer) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.StudentAnswer, error)
	Update(ctx context.Context, tx *gorm.DB, answer *models.StudentAnswer) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error

	// Bulk operations
	CreateBatch(ctx context.Context, tx *gorm.DB, answers []*models.StudentAnswer) error
	UpdateBatch(ctx context.Context, tx *gorm.DB, answers []*models.StudentAnswer) error
	UpsertAnswer(ctx context.Context, tx *gorm.DB, answer *models.StudentAnswer) error // Create or update

	// Query operations
	GetByAttempt(ctx context.Context, tx *gorm.DB, attemptID uint) ([]*models.StudentAnswer, error)
	GetByAttemptAndQuestion(ctx context.Context, tx *gorm.DB, attemptID, questionID uint) (*models.StudentAnswer, error)
	GetByQuestion(ctx context.Context, tx *gorm.DB, questionID uint, filters AnswerFilters) ([]*models.StudentAnswer, error)
	GetByStudent(ctx context.Context, tx *gorm.DB, studentID string, filters AnswerFilters) ([]*models.StudentAnswer, error)

	// Grading operations
	UpdateGrade(ctx context.Context, tx *gorm.DB, id uint, score float64, isCorrect *bool, feedback *string, graderID string) error
	BulkGrade(ctx context.Context, tx *gorm.DB, grades []AnswerGrade) error
	GetPendingGrading(ctx context.Context, tx *gorm.DB, teacherID string) ([]*models.StudentAnswer, error)
	GetGradedAnswers(ctx context.Context, tx *gorm.DB, graderID string, filters AnswerFilters) ([]*models.StudentAnswer, error)

	// Answer tracking
	UpdateAnswerHistory(ctx context.Context, tx *gorm.DB, id uint, newAnswer interface{}) error
	GetAnswerHistory(ctx context.Context, tx *gorm.DB, id uint) ([]AnswerHistoryEntry, error)
	FlagAnswer(ctx context.Context, tx *gorm.DB, id uint, flagged bool) error
	GetFlaggedAnswers(ctx context.Context, tx *gorm.DB, attemptID uint) ([]*models.StudentAnswer, error)

	// Time tracking
	UpdateTimeSpent(ctx context.Context, tx *gorm.DB, id uint, timeSpent int) error
	GetTimeSpentByQuestion(ctx context.Context, tx *gorm.DB, attemptID uint) (map[uint]int, error)

	// Statistics and analytics
	GetAnswerStats(ctx context.Context, tx *gorm.DB, questionID uint) (*AnswerStats, error)
	GetStudentAnswerStats(ctx context.Context, tx *gorm.DB, studentID string) (*StudentAnswerStats, error)
	GetAnswerDistribution(ctx context.Context, tx *gorm.DB, questionID uint) (*AnswerDistribution, error)
	GetGradingStats(ctx context.Context, tx *gorm.DB, assessmentID uint) (*GradingStats, error)
	GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.StudentAnswer, error)

	// Validation
	HasAnswer(ctx context.Context, tx *gorm.DB, attemptID, questionID uint) (bool, error)
	GetAnsweredQuestions(ctx context.Context, tx *gorm.DB, attemptID uint) ([]uint, error)
	GetUnansweredQuestions(ctx context.Context, tx *gorm.DB, attemptID uint) ([]uint, error)

	AreAllAnswersGraded(ctx context.Context, tx *gorm.DB, attemptID uint) (bool, error)
}

// ===== ADDITIONAL STRUCTS =====

type AttemptProgress struct {
	AttemptID            uint    `json:"attempt_id"`
	CurrentQuestionIndex int     `json:"current_question_index"`
	QuestionsAnswered    int     `json:"questions_answered"`
	TotalQuestions       int     `json:"total_questions"`
	ProgressPercentage   float64 `json:"progress_percentage"`
	TimeSpent            int     `json:"time_spent"`
	TimeRemaining        int     `json:"time_remaining"`
	IsReview             bool    `json:"is_review"`
}

type AttemptValidation struct {
	CanStart         bool                    `json:"can_start"`
	Reason           string                  `json:"reason"`
	AttemptsUsed     int                     `json:"attempts_used"`
	MaxAttempts      int                     `json:"max_attempts"`
	NextAttemptTime  *time.Time              `json:"next_attempt_time"`
	AssessmentStatus models.AssessmentStatus `json:"assessment_status"`
}

type StudentAttemptStats struct {
	TotalAttempts      int                          `json:"total_attempts"`
	CompletedAttempts  int                          `json:"completed_attempts"`
	InProgressAttempts int                          `json:"in_progress_attempts"`
	AverageScore       float64                      `json:"average_score"`
	BestScore          float64                      `json:"best_score"`
	TotalTimeSpent     int                          `json:"total_time_spent"`
	AssessmentsCount   int                          `json:"assessments_count"`
	PassedCount        int                          `json:"passed_count"`
	StatusBreakdown    map[models.AttemptStatus]int `json:"status_breakdown"`
}

type AnswerHistoryEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	Answer    interface{} `json:"answer"`
	Action    string      `json:"action"` // "created", "updated", "submitted"
}

type AnswerStats struct {
	QuestionID         uint           `json:"question_id"`
	TotalAnswers       int            `json:"total_answers"`
	CorrectAnswers     int            `json:"correct_answers"`
	CorrectRate        float64        `json:"correct_rate"`
	AverageScore       float64        `json:"average_score"`
	AverageTimeSpent   int            `json:"average_time_spent"`
	AnswerDistribution map[string]int `json:"answer_distribution"`
	CommonMistakes     []string       `json:"common_mistakes"`
}

type StudentAnswerStats struct {
	StudentID         string                             `json:"student_id"`
	TotalAnswers      int                                `json:"total_answers"`
	CorrectAnswers    int                                `json:"correct_answers"`
	CorrectRate       float64                            `json:"correct_rate"`
	AverageScore      float64                            `json:"average_score"`
	TotalTimeSpent    int                                `json:"total_time_spent"`
	AnswersByType     map[models.QuestionType]int        `json:"answers_by_type"`
	PerformanceByDiff map[models.DifficultyLevel]float64 `json:"performance_by_difficulty"`
	FlaggedCount      int                                `json:"flagged_count"`
}

type AnswerDistribution struct {
	QuestionID    uint                `json:"question_id"`
	QuestionType  models.QuestionType `json:"question_type"`
	TotalAnswers  int                 `json:"total_answers"`
	Distribution  map[string]int      `json:"distribution"` // Answer option -> count
	CorrectAnswer string              `json:"correct_answer"`
	CorrectCount  int                 `json:"correct_count"`
}
