package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"gorm.io/gorm"
)

// AssessmentQuestionRepository interface for assessment-question relationship operations
type AssessmentQuestionRepository interface {
	// Basic operations
	Create(ctx context.Context, tx *gorm.DB, assessmentQuestion *models.AssessmentQuestion) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.AssessmentQuestion, error)
	Update(ctx context.Context, tx *gorm.DB, assessmentQuestion *models.AssessmentQuestion) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error

	// Relationship management
	AddQuestion(ctx context.Context, tx *gorm.DB, assessmentID, questionID uint, order int, points *int) error
	RemoveQuestion(ctx context.Context, tx *gorm.DB, assessmentID, questionID uint) error
	AddQuestions(ctx context.Context, tx *gorm.DB, assessmentID uint, questionIDs []uint) error
	RemoveQuestions(ctx context.Context, tx *gorm.DB, assessmentID uint, questionIDs []uint) error
	GetQuestionAssessmentByAssessmentIdAndQuestionId(ctx context.Context, tx *gorm.DB, assessmentId, questionId uint) (*models.AssessmentQuestion, error)

	// Order management
	UpdateOrder(ctx context.Context, tx *gorm.DB, assessmentID uint, questionOrders []QuestionOrder) error
	ReorderQuestions(ctx context.Context, tx *gorm.DB, assessmentID uint, questionIDs []QuestionOrder) error
	GetMaxOrder(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error)
	GetNextOrder(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error)

	// Query operations
	GetByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.AssessmentQuestion, error)
	GetByAssessmentOrdered(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.AssessmentQuestion, error)
	GetByQuestion(ctx context.Context, tx *gorm.DB, questionID uint) ([]*models.AssessmentQuestion, error)
	GetQuestionsForAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) ([]*models.Question, error)
	GetAssessmentsForQuestion(ctx context.Context, tx *gorm.DB, questionID uint) ([]*models.Assessment, error)

	// Bulk operations
	CreateBatch(ctx context.Context, tx *gorm.DB, assessmentQuestions []*models.AssessmentQuestion) error
	UpdateBatch(ctx context.Context, tx *gorm.DB, assessmentQuestions []*models.AssessmentQuestion) error
	DeleteByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint) error
	DeleteByQuestion(ctx context.Context, tx *gorm.DB, questionID uint) error

	// Validation and checks
	Exists(ctx context.Context, tx *gorm.DB, assessmentID, questionID uint) (bool, error)
	GetQuestionCount(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error)
	GetAssessmentCount(ctx context.Context, tx *gorm.DB, questionID uint) (int, error)

	// Points management
	UpdatePoints(ctx context.Context, tx *gorm.DB, assessmentID, questionID uint, points int) error
	GetTotalPoints(ctx context.Context, tx *gorm.DB, assessmentID uint) (int, error)
	GetPointsDistribution(ctx context.Context, tx *gorm.DB, assessmentID uint) (map[uint]int, error)

	// Advanced queries
	GetQuestionsByType(ctx context.Context, tx *gorm.DB, assessmentID uint, questionType models.QuestionType) ([]*models.Question, error)
	GetQuestionsByDifficulty(ctx context.Context, tx *gorm.DB, assessmentID uint, difficulty models.DifficultyLevel) ([]*models.Question, error)
	GetRandomizedQuestions(ctx context.Context, tx *gorm.DB, assessmentID uint, seed int64) ([]*models.Question, error)

	// Statistics
	GetAssessmentQuestionStats(ctx context.Context, tx *gorm.DB, assessmentID uint) (*AssessmentQuestionStats, error)
	GetQuestionUsageInAssessments(ctx context.Context, tx *gorm.DB, questionID uint) (*QuestionAssessmentUsage, error)
}

// ===== ADDITIONAL STRUCTS =====

type AssessmentQuestionStats struct {
	AssessmentID       uint                           `json:"assessment_id"`
	TotalQuestions     int                            `json:"total_questions"`
	TotalPoints        int                            `json:"total_points"`
	QuestionsByType    map[models.QuestionType]int    `json:"questions_by_type"`
	QuestionsByDiff    map[models.DifficultyLevel]int `json:"questions_by_difficulty"`
	AvgPointsPerQ      float64                        `json:"avg_points_per_question"`
	PointsDistribution map[int]int                    `json:"points_distribution"` // points -> count
}

type QuestionAssessmentUsage struct {
	QuestionID       uint     `json:"question_id"`
	UsedInCount      int      `json:"used_in_count"`
	TotalAttempts    int      `json:"total_attempts"`
	AverageScore     float64  `json:"average_score"`
	AssessmentTitles []string `json:"assessment_titles"`
}
