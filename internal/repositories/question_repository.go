package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// QuestionRepository interface for question-specific operations
type QuestionRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, question *models.Question) error
	GetByID(ctx context.Context, id uint) (*models.Question, error)
	GetByIDWithDetails(ctx context.Context, id uint) (*models.Question, error) // Include attachments, category
	Update(ctx context.Context, question *models.Question) error
	Delete(ctx context.Context, id uint) error

	// Bulk operations
	CreateBatch(ctx context.Context, questions []*models.Question) error
	UpdateBatch(ctx context.Context, questions []*models.Question) error
	GetByIDs(ctx context.Context, ids []uint) ([]*models.Question, error)
	DeleteBatch(ctx context.Context, ids []uint) error

	// Query operations
	List(ctx context.Context, filters QuestionFilters) ([]*models.Question, int64, error)
	GetByCreator(ctx context.Context, creatorID uint, filters QuestionFilters) ([]*models.Question, int64, error)
	GetByCategory(ctx context.Context, categoryID uint, filters QuestionFilters) ([]*models.Question, error)
	GetByType(ctx context.Context, questionType models.QuestionType, filters QuestionFilters) ([]*models.Question, error)
	GetByDifficulty(ctx context.Context, difficulty models.DifficultyLevel, limit, offset int) ([]*models.Question, error)
	Search(ctx context.Context, query string, filters QuestionFilters) ([]*models.Question, int64, error)

	// Assessment-specific queries
	GetByAssessment(ctx context.Context, assessmentID uint) ([]*models.Question, error)
	GetRandomQuestions(ctx context.Context, filters RandomQuestionFilters) ([]*models.Question, error)
	GetQuestionBank(ctx context.Context, creatorID uint, filters QuestionBankFilters) ([]*models.Question, int64, error)

	// Advanced filtering
	GetByTags(ctx context.Context, tags []string, filters QuestionFilters) ([]*models.Question, error)
	GetSimilarQuestions(ctx context.Context, questionID uint, limit int) ([]*models.Question, error)

	// Statistics and analytics
	GetQuestionStats(ctx context.Context, id uint) (*QuestionStats, error)
	GetUsageStats(ctx context.Context, creatorID uint) (*QuestionUsageStats, error)
	GetPerformanceStats(ctx context.Context, questionID uint) (*QuestionPerformanceStats, error)

	// Validation and checks
	ExistsByText(ctx context.Context, text string, creatorID uint, excludeID *uint) (bool, error)
	IsUsedInAssessments(ctx context.Context, id uint) (bool, error)
	GetUsageCount(ctx context.Context, id uint) (int, error)

	// Content management
	UpdateContent(ctx context.Context, id uint, content interface{}) error
}

// QuestionCategoryRepository interface for question category operations
type QuestionCategoryRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, category *models.QuestionCategory) error
	GetByID(ctx context.Context, id uint) (*models.QuestionCategory, error)
	GetByIDWithChildren(ctx context.Context, id uint) (*models.QuestionCategory, error)
	Update(ctx context.Context, category *models.QuestionCategory) error
	Delete(ctx context.Context, id uint) error

	// Hierarchy operations
	GetByCreator(ctx context.Context, creatorID uint) ([]*models.QuestionCategory, error)
	GetRootCategories(ctx context.Context, creatorID uint) ([]*models.QuestionCategory, error)
	GetChildren(ctx context.Context, parentID uint) ([]*models.QuestionCategory, error)
	GetHierarchy(ctx context.Context, creatorID uint) ([]*models.QuestionCategory, error)
	GetPath(ctx context.Context, categoryID uint) ([]*models.QuestionCategory, error)

	// Tree operations
	MoveCategory(ctx context.Context, categoryID uint, newParentID *uint) error
	GetDescendants(ctx context.Context, categoryID uint) ([]*models.QuestionCategory, error)
	UpdatePath(ctx context.Context, categoryID uint) error

	// Validation
	ExistsByName(ctx context.Context, name string, creatorID uint, parentID *uint) (bool, error)
	HasQuestions(ctx context.Context, id uint) (bool, error)
	HasChildren(ctx context.Context, id uint) (bool, error)
	ValidateHierarchy(ctx context.Context, categoryID uint, parentID *uint) error

	// Statistics
	GetCategoryStats(ctx context.Context, categoryID uint) (*CategoryStats, error)
	GetCategoriesWithCounts(ctx context.Context, creatorID uint) ([]*CategoryWithCount, error)
}

// QuestionAttachmentRepository interface for question attachment operations
type QuestionAttachmentRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, attachment *models.QuestionAttachment) error
	GetByID(ctx context.Context, id uint) (*models.QuestionAttachment, error)
	Update(ctx context.Context, attachment *models.QuestionAttachment) error
	Delete(ctx context.Context, id uint) error

	// Query operations
	GetByQuestion(ctx context.Context, questionID uint) ([]*models.QuestionAttachment, error)
	GetByQuestions(ctx context.Context, questionIDs []uint) (map[uint][]*models.QuestionAttachment, error)

	// Bulk operations
	CreateBatch(ctx context.Context, attachments []*models.QuestionAttachment) error
	DeleteByQuestion(ctx context.Context, questionID uint) error

	// File management
	GetOrphanedAttachments(ctx context.Context) ([]*models.QuestionAttachment, error)
	UpdateOrder(ctx context.Context, questionID uint, attachmentOrders []AttachmentOrder) error
}

// ===== ADDITIONAL FILTER STRUCTS =====

type QuestionBankFilters struct {
	CategoryID     *uint                   `json:"category_id"`
	Type           *models.QuestionType    `json:"type"`
	Difficulty     *models.DifficultyLevel `json:"difficulty"`
	Tags           []string                `json:"tags"`
	UsageCountMin  *int                    `json:"usage_count_min"`
	UsageCountMax  *int                    `json:"usage_count_max"`
	CorrectRateMin *float64                `json:"correct_rate_min"`
	CorrectRateMax *float64                `json:"correct_rate_max"`
	Limit          int                     `json:"limit"`
	Offset         int                     `json:"offset"`
	SortBy         string                  `json:"sort_by"`
	SortOrder      string                  `json:"sort_order"`
}

type AttachmentOrder struct {
	AttachmentID uint `json:"attachment_id"`
	Order        int  `json:"order"`
}

// ===== ADDITIONAL STATISTICS STRUCTS =====

type QuestionPerformanceStats struct {
	TotalAttempts      int            `json:"total_attempts"`
	CorrectAnswers     int            `json:"correct_answers"`
	CorrectRate        float64        `json:"correct_rate"`
	AverageScore       float64        `json:"average_score"`
	AverageTimeSpent   int            `json:"average_time_spent"`
	DifficultyActual   float64        `json:"difficulty_actual"`
	AnswerDistribution map[string]int `json:"answer_distribution"` // For MC questions
}

type CategoryStats struct {
	QuestionCount    int                            `json:"question_count"`
	SubcategoryCount int                            `json:"subcategory_count"`
	QuestionsByType  map[models.QuestionType]int    `json:"questions_by_type"`
	QuestionsByDiff  map[models.DifficultyLevel]int `json:"questions_by_difficulty"`
	TotalUsage       int                            `json:"total_usage"`
}

type CategoryWithCount struct {
	*models.QuestionCategory
	QuestionCount int `json:"question_count"`
	DirectCount   int `json:"direct_count"` // Questions directly in this category
	TotalCount    int `json:"total_count"`  // Including subcategories
}
