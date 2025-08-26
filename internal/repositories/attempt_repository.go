package repositories

import (
	"context"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// AttemptRepository interface for assessment attempt operations
type AttemptRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, attempt *models.AssessmentAttempt) error
	GetByID(ctx context.Context, id uint) (*models.AssessmentAttempt, error)
	GetByIDWithDetails(ctx context.Context, id uint) (*models.AssessmentAttempt, error) // Include answers, assessment
	Update(ctx context.Context, attempt *models.AssessmentAttempt) error
	Delete(ctx context.Context, id uint) error

	// Query operations
	List(ctx context.Context, filters AttemptFilters) ([]*models.AssessmentAttempt, int64, error)
	GetByStudent(ctx context.Context, studentID uint, filters AttemptFilters) ([]*models.AssessmentAttempt, error)
	GetByAssessment(ctx context.Context, assessmentID uint, filters AttemptFilters) ([]*models.AssessmentAttempt, error)
	GetByStudentAndAssessment(ctx context.Context, studentID, assessmentID uint) ([]*models.AssessmentAttempt, error)

	// Active attempt management
	GetActiveAttempt(ctx context.Context, studentID, assessmentID uint) (*models.AssessmentAttempt, error)
	HasActiveAttempt(ctx context.Context, studentID, assessmentID uint) (bool, error)
	GetActiveAttempts(ctx context.Context, studentID uint) ([]*models.AssessmentAttempt, error)

	// Status management
	UpdateStatus(ctx context.Context, id uint, status models.AttemptStatus) error
	BulkUpdateStatus(ctx context.Context, ids []uint, status models.AttemptStatus) error
	GetByStatus(ctx context.Context, status models.AttemptStatus, limit, offset int) ([]*models.AssessmentAttempt, error)

	// Time management
	UpdateTimeRemaining(ctx context.Context, id uint, timeRemaining int) error
	GetInProgressAttempts(ctx context.Context) ([]*models.AssessmentAttempt, error)
	GetTimedOutAttempts(ctx context.Context) ([]*models.AssessmentAttempt, error)
	GetExpiredAttempts(ctx context.Context, cutoffTime time.Time) ([]*models.AssessmentAttempt, error)

	// Progress tracking
	UpdateProgress(ctx context.Context, id uint, currentQuestionIndex, questionsAnswered int) error
	GetProgress(ctx context.Context, id uint) (*AttemptProgress, error)

	// Scoring and completion
	UpdateScore(ctx context.Context, id uint, score, percentage float64, passed bool) error
	CompleteAttempt(ctx context.Context, id uint, completedAt time.Time, finalScore float64) error

	// Statistics and analytics
	GetAttemptCount(ctx context.Context, studentID, assessmentID uint) (int, error)
	GetAssessmentAttemptStats(ctx context.Context, assessmentID uint) (*AttemptStats, error)
	GetStudentAttemptStats(ctx context.Context, studentID uint) (*StudentAttemptStats, error)
	GetAttemptsByDateRange(ctx context.Context, from, to time.Time) ([]*models.AssessmentAttempt, error)

	// Validation and checks
	CanStartAttempt(ctx context.Context, studentID, assessmentID uint) (*AttemptValidation, error)
	GetNextAttemptNumber(ctx context.Context, studentID, assessmentID uint) (int, error)
	HasCompletedAttempts(ctx context.Context, studentID, assessmentID uint) (bool, error)

	// Session management
	UpdateSessionData(ctx context.Context, id uint, sessionData interface{}) error
	GetSessionData(ctx context.Context, id uint) (interface{}, error)
}

// AnswerRepository interface for student answer operations
type AnswerRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, answer *models.StudentAnswer) error
	GetByID(ctx context.Context, id uint) (*models.StudentAnswer, error)
	Update(ctx context.Context, answer *models.StudentAnswer) error
	Delete(ctx context.Context, id uint) error

	// Bulk operations
	CreateBatch(ctx context.Context, answers []*models.StudentAnswer) error
	UpdateBatch(ctx context.Context, answers []*models.StudentAnswer) error
	UpsertAnswer(ctx context.Context, answer *models.StudentAnswer) error // Create or update

	// Query operations
	GetByAttempt(ctx context.Context, attemptID uint) ([]*models.StudentAnswer, error)
	GetByAttemptAndQuestion(ctx context.Context, attemptID, questionID uint) (*models.StudentAnswer, error)
	GetByQuestion(ctx context.Context, questionID uint, filters AnswerFilters) ([]*models.StudentAnswer, error)
	GetByStudent(ctx context.Context, studentID uint, filters AnswerFilters) ([]*models.StudentAnswer, error)

	// Grading operations
	UpdateGrade(ctx context.Context, id uint, score float64, isCorrect *bool, feedback *string, graderID uint) error
	BulkGrade(ctx context.Context, grades []AnswerGrade) error
	GetPendingGrading(ctx context.Context, teacherID uint) ([]*models.StudentAnswer, error)
	GetGradedAnswers(ctx context.Context, graderID uint, filters AnswerFilters) ([]*models.StudentAnswer, error)

	// Answer tracking
	UpdateAnswerHistory(ctx context.Context, id uint, newAnswer interface{}) error
	GetAnswerHistory(ctx context.Context, id uint) ([]AnswerHistoryEntry, error)
	FlagAnswer(ctx context.Context, id uint, flagged bool) error
	GetFlaggedAnswers(ctx context.Context, attemptID uint) ([]*models.StudentAnswer, error)

	// Time tracking
	UpdateTimeSpent(ctx context.Context, id uint, timeSpent int) error
	GetTimeSpentByQuestion(ctx context.Context, attemptID uint) (map[uint]int, error)

	// Statistics and analytics
	GetAnswerStats(ctx context.Context, questionID uint) (*AnswerStats, error)
	GetStudentAnswerStats(ctx context.Context, studentID uint) (*StudentAnswerStats, error)
	GetAnswerDistribution(ctx context.Context, questionID uint) (*AnswerDistribution, error)

	// Validation
	HasAnswer(ctx context.Context, attemptID, questionID uint) (bool, error)
	GetAnsweredQuestions(ctx context.Context, attemptID uint) ([]uint, error)
	GetUnansweredQuestions(ctx context.Context, attemptID uint) ([]uint, error)
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
	StudentID         uint                               `json:"student_id"`
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
