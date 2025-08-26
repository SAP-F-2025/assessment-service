package postgres

import (
	"context"
	"errors"
	"fmt"
	"math/rand"

	"github.com/SAP-F-2025/assessment-service/internal/cache"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AssessmentQuestionPostgreSQL struct {
	db           *gorm.DB
	helpers      *SharedHelpers
	cacheManager *cache.CacheManager
}

func NewAssessmentQuestionPostgreSQL(db *gorm.DB, redisClient *redis.Client) repositories.AssessmentQuestionRepository {
	return &AssessmentQuestionPostgreSQL{
		db:           db,
		helpers:      NewSharedHelpers(db),
		cacheManager: cache.NewCacheManager(redisClient),
	}
}

// ===== BASIC OPERATIONS =====

// Create creates a new assessment-question relationship
func (aq *AssessmentQuestionPostgreSQL) Create(ctx context.Context, assessmentQuestion *models.AssessmentQuestion) error {
	if err := aq.db.WithContext(ctx).Create(assessmentQuestion).Error; err != nil {
		return fmt.Errorf("failed to create assessment question: %w", err)
	}
	return nil
}

// GetByID retrieves an assessment-question relationship by ID
func (aq *AssessmentQuestionPostgreSQL) GetByID(ctx context.Context, id uint) (*models.AssessmentQuestion, error) {
	var assessmentQuestion models.AssessmentQuestion
	if err := aq.db.WithContext(ctx).
		Preload("Assessment").
		Preload("Question").
		First(&assessmentQuestion, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("assessment question not found with ID %d", id)
		}
		return nil, fmt.Errorf("failed to get assessment question: %w", err)
	}
	return &assessmentQuestion, nil
}

// Update updates an assessment-question relationship
func (aq *AssessmentQuestionPostgreSQL) Update(ctx context.Context, assessmentQuestion *models.AssessmentQuestion) error {
	if err := aq.db.WithContext(ctx).Save(assessmentQuestion).Error; err != nil {
		return fmt.Errorf("failed to update assessment question: %w", err)
	}
	return nil
}

// Delete removes an assessment-question relationship
func (aq *AssessmentQuestionPostgreSQL) Delete(ctx context.Context, id uint) error {
	if err := aq.db.WithContext(ctx).Delete(&models.AssessmentQuestion{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete assessment question: %w", err)
	}
	return nil
}

// ===== RELATIONSHIP MANAGEMENT =====

// AddQuestion adds a question to an assessment with specified order and points
func (aq *AssessmentQuestionPostgreSQL) AddQuestion(ctx context.Context, assessmentID, questionID uint, order int, points *int) error {
	// Check if relationship already exists
	exists, err := aq.Exists(ctx, assessmentID, questionID)
	if err != nil {
		return fmt.Errorf("failed to check if relationship exists: %w", err)
	}
	if exists {
		return fmt.Errorf("question %d is already added to assessment %d", questionID, assessmentID)
	}

	// If order is 0, get next order
	if order == 0 {
		order, err = aq.GetNextOrder(ctx, assessmentID)
		if err != nil {
			return fmt.Errorf("failed to get next order: %w", err)
		}
	}

	assessmentQuestion := &models.AssessmentQuestion{
		AssessmentID: assessmentID,
		QuestionID:   questionID,
		Order:        order,
		Points:       points,
		Required:     true,
	}

	return aq.Create(ctx, assessmentQuestion)
}

// RemoveQuestion removes a question from an assessment
func (aq *AssessmentQuestionPostgreSQL) RemoveQuestion(ctx context.Context, assessmentID, questionID uint) error {
	result := aq.db.WithContext(ctx).
		Where("assessment_id = ? AND question_id = ?", assessmentID, questionID).
		Delete(&models.AssessmentQuestion{})

	if result.Error != nil {
		return fmt.Errorf("failed to remove question from assessment: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no relationship found between assessment %d and question %d", assessmentID, questionID)
	}

	return nil
}

// AddQuestions adds multiple questions to an assessment
func (aq *AssessmentQuestionPostgreSQL) AddQuestions(ctx context.Context, assessmentID uint, questionIDs []uint) error {
	if len(questionIDs) == 0 {
		return nil
	}

	return aq.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get next order
		nextOrder, err := aq.GetNextOrder(ctx, assessmentID)
		if err != nil {
			return fmt.Errorf("failed to get next order: %w", err)
		}

		// Create assessment questions
		assessmentQuestions := make([]*models.AssessmentQuestion, len(questionIDs))
		for i, questionID := range questionIDs {
			// Check if relationship already exists
			exists, err := aq.Exists(ctx, assessmentID, questionID)
			if err != nil {
				return fmt.Errorf("failed to check if relationship exists for question %d: %w", questionID, err)
			}
			if exists {
				return fmt.Errorf("question %d is already added to assessment %d", questionID, assessmentID)
			}

			assessmentQuestions[i] = &models.AssessmentQuestion{
				AssessmentID: assessmentID,
				QuestionID:   questionID,
				Order:        nextOrder + i,
				Required:     true,
			}
		}

		return aq.CreateBatch(ctx, assessmentQuestions)
	})
}

// RemoveQuestions removes multiple questions from an assessment
func (aq *AssessmentQuestionPostgreSQL) RemoveQuestions(ctx context.Context, assessmentID uint, questionIDs []uint) error {
	if len(questionIDs) == 0 {
		return nil
	}

	result := aq.db.WithContext(ctx).
		Where("assessment_id = ? AND question_id IN ?", assessmentID, questionIDs).
		Delete(&models.AssessmentQuestion{})

	if result.Error != nil {
		return fmt.Errorf("failed to remove questions from assessment: %w", result.Error)
	}

	return nil
}

// ===== ORDER MANAGEMENT =====

// UpdateOrder updates the order of questions in an assessment
func (aq *AssessmentQuestionPostgreSQL) UpdateOrder(ctx context.Context, assessmentID uint, questionOrders []repositories.QuestionOrder) error {
	if len(questionOrders) == 0 {
		return nil
	}

	return aq.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, qo := range questionOrders {
			result := tx.Model(&models.AssessmentQuestion{}).
				Where("assessment_id = ? AND question_id = ?", assessmentID, qo.QuestionID).
				Update("order", qo.Order)

			if result.Error != nil {
				return fmt.Errorf("failed to update order for question %d: %w", qo.QuestionID, result.Error)
			}

			if result.RowsAffected == 0 {
				return fmt.Errorf("no relationship found for assessment %d and question %d", assessmentID, qo.QuestionID)
			}
		}
		return nil
	})
}

// ReorderQuestions reorders questions based on provided order
func (aq *AssessmentQuestionPostgreSQL) ReorderQuestions(ctx context.Context, assessmentID uint, questionIDs []uint) error {
	questionOrders := make([]repositories.QuestionOrder, len(questionIDs))
	for i, questionID := range questionIDs {
		questionOrders[i] = repositories.QuestionOrder{
			QuestionID: questionID,
			Order:      i + 1,
		}
	}
	return aq.UpdateOrder(ctx, assessmentID, questionOrders)
}

// GetMaxOrder gets the maximum order value for questions in an assessment
func (aq *AssessmentQuestionPostgreSQL) GetMaxOrder(ctx context.Context, assessmentID uint) (int, error) {
	var maxOrder int
	err := aq.db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ?", assessmentID).
		Select("COALESCE(MAX(\"order\"), 0)").
		Scan(&maxOrder).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get max order: %w", err)
	}

	return maxOrder, nil
}

// GetNextOrder gets the next order value for adding a question
func (aq *AssessmentQuestionPostgreSQL) GetNextOrder(ctx context.Context, assessmentID uint) (int, error) {
	maxOrder, err := aq.GetMaxOrder(ctx, assessmentID)
	if err != nil {
		return 0, err
	}
	return maxOrder + 1, nil
}

// ===== QUERY OPERATIONS =====

// GetByAssessment retrieves all assessment-question relationships for an assessment
func (aq *AssessmentQuestionPostgreSQL) GetByAssessment(ctx context.Context, assessmentID uint) ([]*models.AssessmentQuestion, error) {
	var assessmentQuestions []*models.AssessmentQuestion
	if err := aq.db.WithContext(ctx).
		Where("assessment_id = ?", assessmentID).
		Find(&assessmentQuestions).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessment questions: %w", err)
	}
	return assessmentQuestions, nil
}

// GetByAssessmentOrdered retrieves assessment-question relationships ordered by order field
func (aq *AssessmentQuestionPostgreSQL) GetByAssessmentOrdered(ctx context.Context, assessmentID uint) ([]*models.AssessmentQuestion, error) {
	var assessmentQuestions []*models.AssessmentQuestion
	if err := aq.db.WithContext(ctx).
		Where("assessment_id = ?", assessmentID).
		Order("\"order\" ASC").
		Find(&assessmentQuestions).Error; err != nil {
		return nil, fmt.Errorf("failed to get ordered assessment questions: %w", err)
	}
	return assessmentQuestions, nil
}

// GetByQuestion retrieves all assessment-question relationships for a question
func (aq *AssessmentQuestionPostgreSQL) GetByQuestion(ctx context.Context, questionID uint) ([]*models.AssessmentQuestion, error) {
	var assessmentQuestions []*models.AssessmentQuestion
	if err := aq.db.WithContext(ctx).
		Where("question_id = ?", questionID).
		Find(&assessmentQuestions).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessment questions by question: %w", err)
	}
	return assessmentQuestions, nil
}

// GetQuestionsForAssessment retrieves all questions for an assessment in order
func (aq *AssessmentQuestionPostgreSQL) GetQuestionsForAssessment(ctx context.Context, assessmentID uint) ([]*models.Question, error) {
	var questions []*models.Question
	if err := aq.db.WithContext(ctx).
		Table("questions").
		Joins("JOIN assessment_questions aq ON aq.question_id = questions.id").
		Where("aq.assessment_id = ?", assessmentID).
		Order("aq.\"order\" ASC").
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions for assessment: %w", err)
	}
	return questions, nil
}

// GetAssessmentsForQuestion retrieves all assessments that use a question
func (aq *AssessmentQuestionPostgreSQL) GetAssessmentsForQuestion(ctx context.Context, questionID uint) ([]*models.Assessment, error) {
	var assessments []*models.Assessment
	if err := aq.db.WithContext(ctx).
		Table("assessments").
		Joins("JOIN assessment_questions aq ON aq.assessment_id = assessments.id").
		Where("aq.question_id = ?", questionID).
		Find(&assessments).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessments for question: %w", err)
	}
	return assessments, nil
}

// ===== BULK OPERATIONS =====

// CreateBatch creates multiple assessment-question relationships
func (aq *AssessmentQuestionPostgreSQL) CreateBatch(ctx context.Context, assessmentQuestions []*models.AssessmentQuestion) error {
	if len(assessmentQuestions) == 0 {
		return nil
	}

	if err := aq.db.WithContext(ctx).CreateInBatches(assessmentQuestions, 100).Error; err != nil {
		return fmt.Errorf("failed to create assessment questions batch: %w", err)
	}
	return nil
}

// UpdateBatch updates multiple assessment-question relationships
func (aq *AssessmentQuestionPostgreSQL) UpdateBatch(ctx context.Context, assessmentQuestions []*models.AssessmentQuestion) error {
	if len(assessmentQuestions) == 0 {
		return nil
	}

	return aq.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, assessmentQuestion := range assessmentQuestions {
			if err := tx.Save(assessmentQuestion).Error; err != nil {
				return fmt.Errorf("failed to update assessment question ID %d: %w", assessmentQuestion.ID, err)
			}
		}
		return nil
	})
}

// DeleteByAssessment removes all questions from an assessment
func (aq *AssessmentQuestionPostgreSQL) DeleteByAssessment(ctx context.Context, assessmentID uint) error {
	if err := aq.db.WithContext(ctx).
		Where("assessment_id = ?", assessmentID).
		Delete(&models.AssessmentQuestion{}).Error; err != nil {
		return fmt.Errorf("failed to delete assessment questions by assessment: %w", err)
	}
	return nil
}

// DeleteByQuestion removes a question from all assessments
func (aq *AssessmentQuestionPostgreSQL) DeleteByQuestion(ctx context.Context, questionID uint) error {
	if err := aq.db.WithContext(ctx).
		Where("question_id = ?", questionID).
		Delete(&models.AssessmentQuestion{}).Error; err != nil {
		return fmt.Errorf("failed to delete assessment questions by question: %w", err)
	}
	return nil
}

// ===== VALIDATION AND CHECKS =====

// Exists checks if an assessment-question relationship exists
func (aq *AssessmentQuestionPostgreSQL) Exists(ctx context.Context, assessmentID, questionID uint) (bool, error) {
	var count int64
	err := aq.db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ? AND question_id = ?", assessmentID, questionID).
		Count(&count).Error

	if err != nil {
		return false, fmt.Errorf("failed to check assessment question existence: %w", err)
	}

	return count > 0, nil
}

// GetQuestionCount gets the number of questions in an assessment
func (aq *AssessmentQuestionPostgreSQL) GetQuestionCount(ctx context.Context, assessmentID uint) (int, error) {
	var count int64
	err := aq.db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ?", assessmentID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get question count: %w", err)
	}

	return int(count), nil
}

// GetAssessmentCount gets the number of assessments using a question
func (aq *AssessmentQuestionPostgreSQL) GetAssessmentCount(ctx context.Context, questionID uint) (int, error) {
	var count int64
	err := aq.db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("question_id = ?", questionID).
		Count(&count).Error

	if err != nil {
		return 0, fmt.Errorf("failed to get assessment count: %w", err)
	}

	return int(count), nil
}

// ===== POINTS MANAGEMENT =====

// UpdatePoints updates the points for a specific question in an assessment
func (aq *AssessmentQuestionPostgreSQL) UpdatePoints(ctx context.Context, assessmentID, questionID uint, points int) error {
	result := aq.db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Where("assessment_id = ? AND question_id = ?", assessmentID, questionID).
		Update("points", points)

	if result.Error != nil {
		return fmt.Errorf("failed to update points: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no relationship found between assessment %d and question %d", assessmentID, questionID)
	}

	return nil
}

// GetTotalPoints calculates the total points for all questions in an assessment
func (aq *AssessmentQuestionPostgreSQL) GetTotalPoints(ctx context.Context, assessmentID uint) (int, error) {
	var totalPoints int

	// Use COALESCE to handle NULL points (use question default points)
	err := aq.db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN questions q ON q.id = aq.question_id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("SUM(COALESCE(aq.points, q.points))").
		Scan(&totalPoints).Error

	if err != nil {
		return 0, fmt.Errorf("failed to calculate total points: %w", err)
	}

	return totalPoints, nil
}

// GetPointsDistribution returns the distribution of points across questions
func (aq *AssessmentQuestionPostgreSQL) GetPointsDistribution(ctx context.Context, assessmentID uint) (map[uint]int, error) {
	var results []struct {
		QuestionID uint `json:"question_id"`
		Points     int  `json:"points"`
	}

	err := aq.db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN questions q ON q.id = aq.question_id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("aq.question_id, COALESCE(aq.points, q.points) as points").
		Find(&results).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get points distribution: %w", err)
	}

	distribution := make(map[uint]int)
	for _, result := range results {
		distribution[result.QuestionID] = result.Points
	}

	return distribution, nil
}

// ===== ADVANCED QUERIES =====

// GetQuestionsByType retrieves questions of a specific type from an assessment
func (aq *AssessmentQuestionPostgreSQL) GetQuestionsByType(ctx context.Context, assessmentID uint, questionType models.QuestionType) ([]*models.Question, error) {
	var questions []*models.Question
	if err := aq.db.WithContext(ctx).
		Table("questions").
		Joins("JOIN assessment_questions aq ON aq.question_id = questions.id").
		Where("aq.assessment_id = ? AND questions.type = ?", assessmentID, questionType).
		Order("aq.\"order\" ASC").
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions by type: %w", err)
	}
	return questions, nil
}

// GetQuestionsByDifficulty retrieves questions of a specific difficulty from an assessment
func (aq *AssessmentQuestionPostgreSQL) GetQuestionsByDifficulty(ctx context.Context, assessmentID uint, difficulty models.DifficultyLevel) ([]*models.Question, error) {
	var questions []*models.Question
	if err := aq.db.WithContext(ctx).
		Table("questions").
		Joins("JOIN assessment_questions aq ON aq.question_id = questions.id").
		Where("aq.assessment_id = ? AND questions.difficulty = ?", assessmentID, difficulty).
		Order("aq.\"order\" ASC").
		Find(&questions).Error; err != nil {
		return nil, fmt.Errorf("failed to get questions by difficulty: %w", err)
	}
	return questions, nil
}

// GetRandomizedQuestions retrieves questions in randomized order using provided seed
func (aq *AssessmentQuestionPostgreSQL) GetRandomizedQuestions(ctx context.Context, assessmentID uint, seed int64) ([]*models.Question, error) {
	questions, err := aq.GetQuestionsForAssessment(ctx, assessmentID)
	if err != nil {
		return nil, err
	}

	// Randomize using provided seed
	r := rand.New(rand.NewSource(seed))
	r.Shuffle(len(questions), func(i, j int) {
		questions[i], questions[j] = questions[j], questions[i]
	})

	return questions, nil
}

// ===== STATISTICS =====

// GetAssessmentQuestionStats retrieves comprehensive statistics for an assessment
func (aq *AssessmentQuestionPostgreSQL) GetAssessmentQuestionStats(ctx context.Context, assessmentID uint) (*repositories.AssessmentQuestionStats, error) {
	stats := &repositories.AssessmentQuestionStats{
		AssessmentID:       assessmentID,
		QuestionsByType:    make(map[models.QuestionType]int),
		QuestionsByDiff:    make(map[models.DifficultyLevel]int),
		PointsDistribution: make(map[int]int),
	}

	// Get total questions and points
	questionCount, err := aq.GetQuestionCount(ctx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get question count: %w", err)
	}
	stats.TotalQuestions = questionCount

	totalPoints, err := aq.GetTotalPoints(ctx, assessmentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total points: %w", err)
	}
	stats.TotalPoints = totalPoints

	if questionCount > 0 {
		stats.AvgPointsPerQ = float64(totalPoints) / float64(questionCount)
	}

	// Get questions by type
	var typeResults []struct {
		Type  models.QuestionType `json:"type"`
		Count int                 `json:"count"`
	}
	err = aq.db.WithContext(ctx).
		Table("questions q").
		Joins("JOIN assessment_questions aq ON aq.question_id = q.id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("q.type, COUNT(*) as count").
		Group("q.type").
		Find(&typeResults).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get questions by type: %w", err)
	}
	for _, result := range typeResults {
		stats.QuestionsByType[result.Type] = result.Count
	}

	// Get questions by difficulty
	var diffResults []struct {
		Difficulty models.DifficultyLevel `json:"difficulty"`
		Count      int                    `json:"count"`
	}
	err = aq.db.WithContext(ctx).
		Table("questions q").
		Joins("JOIN assessment_questions aq ON aq.question_id = q.id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("q.difficulty, COUNT(*) as count").
		Group("q.difficulty").
		Find(&diffResults).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get questions by difficulty: %w", err)
	}
	for _, result := range diffResults {
		stats.QuestionsByDiff[result.Difficulty] = result.Count
	}

	// Get points distribution
	var pointsResults []struct {
		Points int `json:"points"`
		Count  int `json:"count"`
	}
	err = aq.db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN questions q ON q.id = aq.question_id").
		Where("aq.assessment_id = ?", assessmentID).
		Select("COALESCE(aq.points, q.points) as points, COUNT(*) as count").
		Group("COALESCE(aq.points, q.points)").
		Find(&pointsResults).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get points distribution: %w", err)
	}
	for _, result := range pointsResults {
		stats.PointsDistribution[result.Points] = result.Count
	}

	return stats, nil
}

// GetQuestionUsageInAssessments retrieves usage statistics for a question across assessments
func (aq *AssessmentQuestionPostgreSQL) GetQuestionUsageInAssessments(ctx context.Context, questionID uint) (*repositories.QuestionAssessmentUsage, error) {
	usage := &repositories.QuestionAssessmentUsage{
		QuestionID: questionID,
	}

	// Get usage count
	usageCount, err := aq.GetAssessmentCount(ctx, questionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get usage count: %w", err)
	}
	usage.UsedInCount = usageCount

	// Get assessment titles
	var titles []string
	err = aq.db.WithContext(ctx).
		Table("assessments a").
		Joins("JOIN assessment_questions aq ON aq.assessment_id = a.id").
		Where("aq.question_id = ?", questionID).
		Pluck("a.title", &titles).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get assessment titles: %w", err)
	}
	usage.AssessmentTitles = titles

	// Note: TotalAttempts and AverageScore would require answers/attempts tables
	// which are not implemented in this basic version

	return usage, nil
}
