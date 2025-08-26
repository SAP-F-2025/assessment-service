package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/cache"
	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type AttemptPostgreSQL struct {
	db           *gorm.DB
	helpers      *SharedHelpers
	cacheManager *cache.CacheManager
}

func NewAttemptPostgreSQL(db *gorm.DB, redisClient *redis.Client) repositories.AttemptRepository {
	return &AttemptPostgreSQL{
		db:           db,
		helpers:      NewSharedHelpers(db),
		cacheManager: cache.NewCacheManager(redisClient),
	}
}

func (a AttemptPostgreSQL) Create(ctx context.Context, attempt *models.AssessmentAttempt) error {
	return a.db.WithContext(ctx).Create(attempt).Error
}

func (a AttemptPostgreSQL) GetByID(ctx context.Context, id uint) (*models.AssessmentAttempt, error) {
	// Cache active attempts for performance
	cacheKey := fmt.Sprintf("id:%d", id)
	var attempt models.AssessmentAttempt

	err := a.cacheManager.Fast.CacheOrExecute(ctx, cacheKey, &attempt, cache.FastCacheConfig.TTL, func() (interface{}, error) {
		var dbAttempt models.AssessmentAttempt
		if err := a.db.WithContext(ctx).First(&dbAttempt, id).Error; err != nil {
			return nil, fmt.Errorf("failed to get attempt: %w", err)
		}
		return &dbAttempt, nil
	})

	return &attempt, err
}

func (a AttemptPostgreSQL) GetByIDWithDetails(ctx context.Context, id uint) (*models.AssessmentAttempt, error) {
	var attempt models.AssessmentAttempt
	if err := a.db.WithContext(ctx).
		Preload("Student").
		Preload("Assessment").
		Preload("ProctoringEvents").
		First(&attempt, id).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (a AttemptPostgreSQL) Update(ctx context.Context, attempt *models.AssessmentAttempt) error {
	return a.db.WithContext(ctx).Save(attempt).Error
}

func (a AttemptPostgreSQL) Delete(ctx context.Context, id uint) error {
	return a.db.WithContext(ctx).Delete(&models.AssessmentAttempt{}, id).Error
}

func (a AttemptPostgreSQL) List(ctx context.Context, filters repositories.AttemptFilters) ([]*models.AssessmentAttempt, int64, error) {
	var attempts []*models.AssessmentAttempt
	var total int64

	// apply filter first
	query := a.db.WithContext(ctx).Model(&models.AssessmentAttempt{})
	query = a.applyFiltersAttempt(query, filters)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// then apply pagination and sorting
	query = a.applyPaginationAndSortAttempt(query, filters)

	if err := query.Preload("Student").Preload("Assessment").Find(&attempts).Error; err != nil {
		return nil, 0, err
	}

	return attempts, total, nil
}

func (a AttemptPostgreSQL) GetByStudent(ctx context.Context, studentID uint, filters repositories.AttemptFilters) ([]*models.AssessmentAttempt, int64, error) {
	filters.StudentID = &studentID
	return a.List(ctx, filters)
}

func (a AttemptPostgreSQL) GetByAssessment(ctx context.Context, assessmentID uint, filters repositories.AttemptFilters) ([]*models.AssessmentAttempt, error) {
	var attempts []*models.AssessmentAttempt

	query := a.db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("assessment_id = ?", assessmentID)
	query = a.applyFiltersAttempt(query, filters)
	query = a.applyPaginationAndSortAttempt(query, filters)

	if err := query.Preload("Student").Preload("Assessment").Preload("ProctoringEvents").Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a AttemptPostgreSQL) GetByStudentAndAssessment(ctx context.Context, studentID, assessmentID uint) ([]*models.AssessmentAttempt, error) {
	var attempts []*models.AssessmentAttempt
	if err := a.db.WithContext(ctx).
		Where("student_id = ? AND assessment_id = ?", studentID, assessmentID).
		Preload("Student").
		Preload("Assessment").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a AttemptPostgreSQL) GetActiveAttempt(ctx context.Context, studentID, assessmentID uint) (*models.AssessmentAttempt, error) {
	var attempt models.AssessmentAttempt
	if err := a.db.WithContext(ctx).
		Where("student_id = ? AND assessment_id = ? AND status = ?", studentID, assessmentID,
			models.AttemptInProgress).
		Preload("Student").
		Preload("Assessment").
		First(&attempt).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &attempt, nil
}

func (a AttemptPostgreSQL) HasActiveAttempt(ctx context.Context, studentID, assessmentID uint) (bool, error) {
	var count int64
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND assessment_id = ? AND status = ?", studentID, assessmentID, models.AttemptInProgress).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (a AttemptPostgreSQL) GetActiveAttempts(ctx context.Context, studentID uint) ([]*models.AssessmentAttempt, error) {
	var attempts []*models.AssessmentAttempt
	if err := a.db.WithContext(ctx).
		Where("student_id = ? AND status = ?", studentID, models.AttemptInProgress).
		Preload("Student").
		Preload("Assessment").
		Preload("ProctoringEvents").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a AttemptPostgreSQL) UpdateStatus(ctx context.Context, id uint, status models.AttemptStatus) error {
	return a.db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("id = ?", id).Update("status", status).Error
}

func (a AttemptPostgreSQL) BulkUpdateStatus(ctx context.Context, ids []uint, status models.AttemptStatus) error {
	return a.helpers.BulkUpdateAttemptStatus(ctx, ids, status)
}

func (a AttemptPostgreSQL) GetByStatus(ctx context.Context, status models.AttemptStatus, limit, offset int) ([]*models.AssessmentAttempt, error) {
	var attempts []*models.AssessmentAttempt
	query := a.db.WithContext(ctx).Where("status = ?", status)

	if limit > 0 {
		query = query.Limit(limit)
	}

	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Preload("Student").Preload("Assessment").Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a AttemptPostgreSQL) UpdateTimeRemaining(ctx context.Context, id uint, timeRemaining int) error {
	return a.db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("id = ?", id).Update("time_remaining", timeRemaining).Error
}

func (a AttemptPostgreSQL) GetInProgressAttempts(ctx context.Context) ([]*models.AssessmentAttempt, error) {
	var attempts []*models.AssessmentAttempt
	if err := a.db.WithContext(ctx).
		Where("status = ?", models.AttemptInProgress).
		Preload("Student").
		Preload("Assessment").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a AttemptPostgreSQL) GetTimedOutAttempts(ctx context.Context) ([]*models.AssessmentAttempt, error) {
	var attempts []*models.AssessmentAttempt
	if err := a.db.WithContext(ctx).
		Where("status = ? AND time_remaining <= 0", models.AttemptInProgress).
		Preload("Student").
		Preload("Assessment").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a AttemptPostgreSQL) GetExpiredAttempts(ctx context.Context, cutoffTime time.Time) ([]*models.AssessmentAttempt, error) {
	var attempts []*models.AssessmentAttempt
	if err := a.db.WithContext(ctx).
		Where("status = ? AND started_at <= ?", models.AttemptInProgress, cutoffTime).
		Preload("Student").
		Preload("Assessment").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a AttemptPostgreSQL) UpdateProgress(ctx context.Context, id uint, currentQuestionIndex, questionsAnswered int) error {
	return a.db.WithContext(ctx).Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"current_question_index": currentQuestionIndex,
			"questions_answered":     questionsAnswered,
		}).Error
}

func (a AttemptPostgreSQL) GetProgress(ctx context.Context, id uint) (*repositories.AttemptProgress, error) {
	var attempt models.AssessmentAttempt
	if err := a.db.WithContext(ctx).Preload("Assessment").
		First(&attempt, id).Error; err != nil {
		return nil, err
	}

	timeSpent := int(time.Now().UTC().Sub(*attempt.StartedAt).Minutes())
	return &repositories.AttemptProgress{
		AttemptID:            id,
		CurrentQuestionIndex: attempt.CurrentQuestionIndex,
		QuestionsAnswered:    attempt.QuestionsAnswered,
		TotalQuestions:       attempt.TotalQuestions,
		ProgressPercentage:   (float64(attempt.QuestionsAnswered) / float64(attempt.TotalQuestions)) * 100,
		TimeSpent:            timeSpent,
		TimeRemaining:        attempt.Assessment.Duration - timeSpent,
		IsReview:             false,
	}, nil
}

func (a AttemptPostgreSQL) UpdateScore(ctx context.Context, id uint, score, percentage float64, passed bool) error {
	return a.db.WithContext(ctx).Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"score":      score,
			"percentage": percentage,
			"passed":     passed,
		}).Error
}

func (a AttemptPostgreSQL) CompleteAttempt(ctx context.Context, id uint, completedAt time.Time, finalScore float64) error {
	return a.db.WithContext(ctx).Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       models.AttemptCompleted,
			"completed_at": completedAt,
			"score":        finalScore,
		}).Error
}

func (a AttemptPostgreSQL) GetAttemptCount(ctx context.Context, studentID, assessmentID uint) (int, error) {
	count, err := a.helpers.CountAttemptsByStudent(ctx, assessmentID, studentID)
	return int(count), err
}

func (a AttemptPostgreSQL) GetAssessmentAttemptStats(ctx context.Context, assessmentID uint) (*repositories.AttemptStats, error) {
	var stats repositories.AttemptStats

	totalAttempts, err := a.helpers.CountAttempts(ctx, assessmentID)
	if err != nil {
		return nil, err
	}

	// Status Breakdown using helper
	statusBreakdown := make(map[models.AttemptStatus]int)
	statuses := []models.AttemptStatus{models.AttemptInProgress, models.AttemptCompleted, models.AttemptAbandoned, models.AttemptTimeOut}
	for _, status := range statuses {
		count, err := a.helpers.CountAttemptsByStatus(ctx, assessmentID, status)
		if err != nil {
			return nil, err
		}
		statusBreakdown[status] = int(count)
	}

	// Aggregate stats in single query
	var avgScore, avgTimeSpent float64
	var completedCount, passedCount int64

	a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("assessment_id = ? AND status = ?", assessmentID, models.AttemptCompleted).
		Select("AVG(score), AVG(time_spent), COUNT(*), SUM(CASE WHEN passed = true THEN 1 ELSE 0 END)").
		Row().Scan(&avgScore, &avgTimeSpent, &completedCount, &passedCount)

	passRate := float64(0)
	if completedCount > 0 {
		passRate = float64(passedCount) / float64(completedCount)
	}

	completionRate := float64(0)
	if totalAttempts > 0 {
		completionRate = float64(completedCount) / float64(totalAttempts)
	}

	stats = repositories.AttemptStats{
		TotalAttempts:    int(totalAttempts),
		StatusBreakdown:  statusBreakdown,
		AverageScore:     avgScore,
		AverageTimeSpent: int(avgTimeSpent),
		PassRate:         passRate,
		CompletionRate:   completionRate,
	}

	return &stats, nil
}

func (a AttemptPostgreSQL) GetStudentAttemptStats(ctx context.Context, studentID uint) (*repositories.StudentAttemptStats, error) {
	var stats repositories.StudentAttemptStats

	var totalAttempts int64
	var completedAttempts int64
	var inProgressAttempts int64
	var avgScore float64
	var bestScore float64
	var totalTimeSpent int64
	var assessmentCount int64
	var passedCount int64
	var statusBreakdown = make(map[models.AttemptStatus]int)

	// Total Attempts
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ?", studentID).
		Count(&totalAttempts).Error; err != nil {
		return nil, err
	}

	// Completed Attempts
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ?", studentID, models.AttemptCompleted).
		Count(&completedAttempts).Error; err != nil {
		return nil, err
	}

	// In-Progress Attempts
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ?", studentID, models.AttemptInProgress).
		Count(&inProgressAttempts).Error; err != nil {
		return nil, err
	}

	// Average Score
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ?", studentID, models.AttemptCompleted).
		Select("AVG(score)").Scan(&avgScore).Error; err != nil {
		return nil, err
	}

	// Best Score
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ?", studentID, models.AttemptCompleted).
		Select("MAX(score)").Scan(&bestScore).Error; err != nil {
		return nil, err
	}

	// Total Time Spent
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ?", studentID, models.AttemptCompleted).
		Select("SUM(time_spent)").Scan(&totalTimeSpent).Error; err != nil {
		return nil, err
	}

	// Distinct Assessments Attempted
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ?", studentID).
		Distinct("assessment_id").
		Count(&assessmentCount).Error; err != nil {
		return nil, err
	}

	// Passed Attempts
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND status = ? AND passed = true", studentID, models.AttemptCompleted).
		Count(&passedCount).Error; err != nil {
		return nil, err
	}

	// Status Breakdown
	var statuses = []models.AttemptStatus{models.AttemptInProgress, models.AttemptCompleted, models.AttemptAbandoned, models.AttemptTimeOut}
	for _, status := range statuses {
		var count int64
		if err := a.db.WithContext(ctx).
			Model(&models.AssessmentAttempt{}).
			Where("student_id = ? AND status = ?", studentID, status).
			Count(&count).Error; err != nil {
			return nil, err
		}
		statusBreakdown[status] = int(count)
	}

	stats = repositories.StudentAttemptStats{
		TotalAttempts:      int(totalAttempts),
		CompletedAttempts:  int(completedAttempts),
		InProgressAttempts: int(inProgressAttempts),
		AverageScore:       avgScore,
		BestScore:          bestScore,
		TotalTimeSpent:     int(totalTimeSpent),
		AssessmentsCount:   int(assessmentCount),
		PassedCount:        int(passedCount),
		StatusBreakdown:    statusBreakdown,
	}

	return &stats, nil
}

func (a AttemptPostgreSQL) GetAttemptsByDateRange(ctx context.Context, from, to time.Time) ([]*models.AssessmentAttempt, error) {
	var attempts []*models.AssessmentAttempt
	if err := a.db.WithContext(ctx).
		Where("created_at >= ? AND created_at <= ?", from, to).
		Preload("Student").
		Preload("Assessment").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a AttemptPostgreSQL) CanStartAttempt(ctx context.Context, studentID, assessmentID uint) (*repositories.AttemptValidation, error) {
	return a.helpers.ValidateAttemptEligibility(ctx, assessmentID, studentID)
}

func (a AttemptPostgreSQL) GetNextAttemptNumber(ctx context.Context, studentID, assessmentID uint) (int, error) {
	count, err := a.helpers.CountAttemptsByStudent(ctx, assessmentID, studentID)
	return int(count) + 1, err
}

// GetRemainingAttempts calculates remaining attempts for a student
func (a AttemptPostgreSQL) GetRemainingAttempts(ctx context.Context, assessmentID, studentID uint) (int, error) {
	return a.helpers.GetRemainingAttempts(ctx, assessmentID, studentID)
}

func (a AttemptPostgreSQL) HasCompletedAttempts(ctx context.Context, studentID, assessmentID uint) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND assessment_id = ? AND status = ?", studentID, assessmentID, models.AttemptCompleted).
		Count(&count).Error
	return count > 0, err
}

func (a AttemptPostgreSQL) UpdateSessionData(ctx context.Context, id uint, sessionData interface{}) error {
	return a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Update("session_data", sessionData).Error
}

func (a AttemptPostgreSQL) GetSessionData(ctx context.Context, id uint) (interface{}, error) {
	var sessionData interface{}
	if err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Select("session_data").
		Scan(&sessionData).Error; err != nil {
		return nil, err
	}

	return sessionData, nil
}

// applyFiltersAttempt applies common filters to a query
func (a AttemptPostgreSQL) applyFiltersAttempt(query *gorm.DB, filters repositories.AttemptFilters) *gorm.DB {
	return a.helpers.ApplyAttemptFilters(query, filters)
}

// applyPaginationAndSortAttempt applies pagination and sorting to a query
func (a AttemptPostgreSQL) applyPaginationAndSortAttempt(query *gorm.DB, filters repositories.AttemptFilters) *gorm.DB {
	return a.helpers.ApplyPaginationAndSort(query, filters.SortBy, filters.SortOrder, filters.Limit, filters.Offset)
}

// ===== ANSWER REPOSITORY IMPLEMENTATION =====

// AnswerPostgreSQL implements the AnswerRepository interface
type AnswerPostgreSQL struct {
	db           *gorm.DB
	helpers      *SharedHelpers
	cacheManager *cache.CacheManager
}

// NewAnswerPostgreSQL creates a new answer repository instance
func NewAnswerPostgreSQL(db *gorm.DB, redisClient *redis.Client) repositories.AnswerRepository {
	return &AnswerPostgreSQL{
		db:           db,
		helpers:      NewSharedHelpers(db),
		cacheManager: cache.NewCacheManager(redisClient),
	}
}

// ===== BASIC CRUD OPERATIONS =====

// Create creates a new student answer
func (ar *AnswerPostgreSQL) Create(ctx context.Context, answer *models.StudentAnswer) error {
	if err := ar.db.WithContext(ctx).Create(answer).Error; err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	// Invalidate related caches
	ar.cacheManager.Fast.Delete(ctx,
		fmt.Sprintf("attempt:%d:answers", answer.AttemptID),
		fmt.Sprintf("attempt:%d:question:%d", answer.AttemptID, answer.QuestionID),
	)

	return nil
}

// GetByID retrieves an answer by ID with caching
func (ar *AnswerPostgreSQL) GetByID(ctx context.Context, id uint) (*models.StudentAnswer, error) {
	cacheKey := fmt.Sprintf("answer:id:%d", id)
	var answer models.StudentAnswer

	err := ar.cacheManager.Fast.CacheOrExecute(ctx, cacheKey, &answer, cache.FastCacheConfig.TTL, func() (interface{}, error) {
		var dbAnswer models.StudentAnswer
		if err := ar.db.WithContext(ctx).First(&dbAnswer, id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("answer not found with ID %d", id)
			}
			return nil, fmt.Errorf("failed to get answer: %w", err)
		}
		return &dbAnswer, nil
	})

	return &answer, err
}

// Update updates an existing answer
func (ar *AnswerPostgreSQL) Update(ctx context.Context, answer *models.StudentAnswer) error {
	if err := ar.db.WithContext(ctx).Save(answer).Error; err != nil {
		return fmt.Errorf("failed to update answer: %w", err)
	}

	// Invalidate caches
	ar.cacheManager.Fast.Delete(ctx,
		fmt.Sprintf("answer:id:%d", answer.ID),
		fmt.Sprintf("attempt:%d:answers", answer.AttemptID),
		fmt.Sprintf("attempt:%d:question:%d", answer.AttemptID, answer.QuestionID),
	)

	return nil
}

// Delete removes an answer
func (ar *AnswerPostgreSQL) Delete(ctx context.Context, id uint) error {
	// Get answer first to invalidate caches
	answer, err := ar.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := ar.db.WithContext(ctx).Delete(&models.StudentAnswer{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete answer: %w", err)
	}

	// Invalidate caches
	ar.cacheManager.Fast.Delete(ctx,
		fmt.Sprintf("answer:id:%d", id),
		fmt.Sprintf("attempt:%d:answers", answer.AttemptID),
		fmt.Sprintf("attempt:%d:question:%d", answer.AttemptID, answer.QuestionID),
	)

	return nil
}

// ===== BULK OPERATIONS =====

// CreateBatch creates multiple answers in a batch
func (ar *AnswerPostgreSQL) CreateBatch(ctx context.Context, answers []*models.StudentAnswer) error {
	if len(answers) == 0 {
		return nil
	}

	return ar.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.CreateInBatches(answers, 100).Error; err != nil {
			return fmt.Errorf("failed to create answers batch: %w", err)
		}

		// Invalidate caches for all affected attempts
		attemptIDs := make(map[uint]bool)
		for _, answer := range answers {
			attemptIDs[answer.AttemptID] = true
		}

		for attemptID := range attemptIDs {
			ar.cacheManager.Fast.InvalidatePattern(ctx, fmt.Sprintf("attempt:%d:*", attemptID))
		}

		return nil
	})
}

// UpdateBatch updates multiple answers in a batch
func (ar *AnswerPostgreSQL) UpdateBatch(ctx context.Context, answers []*models.StudentAnswer) error {
	if len(answers) == 0 {
		return nil
	}

	return ar.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, answer := range answers {
			if err := tx.Save(answer).Error; err != nil {
				return fmt.Errorf("failed to update answer ID %d: %w", answer.ID, err)
			}
		}

		// Invalidate caches for all affected attempts
		attemptIDs := make(map[uint]bool)
		for _, answer := range answers {
			attemptIDs[answer.AttemptID] = true
		}

		for attemptID := range attemptIDs {
			ar.cacheManager.Fast.InvalidatePattern(ctx, fmt.Sprintf("attempt:%d:*", attemptID))
		}

		return nil
	})
}

// UpsertAnswer creates or updates an answer
func (ar *AnswerPostgreSQL) UpsertAnswer(ctx context.Context, answer *models.StudentAnswer) error {
	// Check if answer already exists
	existing, err := ar.GetByAttemptAndQuestion(ctx, answer.AttemptID, answer.QuestionID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing answer: %w", err)
	}

	if existing != nil {
		// Update existing answer
		answer.ID = existing.ID
		return ar.Update(ctx, answer)
	}

	// Create new answer
	return ar.Create(ctx, answer)
}

// ===== QUERY OPERATIONS =====

// GetByAttempt retrieves all answers for an attempt with caching
func (ar *AnswerPostgreSQL) GetByAttempt(ctx context.Context, attemptID uint) ([]*models.StudentAnswer, error) {
	cacheKey := fmt.Sprintf("attempt:%d:answers", attemptID)
	var answers []*models.StudentAnswer

	err := ar.cacheManager.Fast.CacheOrExecute(ctx, cacheKey, &answers, cache.FastCacheConfig.TTL, func() (interface{}, error) {
		var dbAnswers []*models.StudentAnswer
		if err := ar.db.WithContext(ctx).
			Where("attempt_id = ?", attemptID).
			Order("question_id ASC").
			Find(&dbAnswers).Error; err != nil {
			return nil, fmt.Errorf("failed to get answers by attempt: %w", err)
		}
		return dbAnswers, nil
	})

	return answers, err
}

// GetByAttemptAndQuestion retrieves a specific answer for an attempt and question
func (ar *AnswerPostgreSQL) GetByAttemptAndQuestion(ctx context.Context, attemptID, questionID uint) (*models.StudentAnswer, error) {
	cacheKey := fmt.Sprintf("attempt:%d:question:%d", attemptID, questionID)
	var answer models.StudentAnswer

	err := ar.cacheManager.Fast.CacheOrExecute(ctx, cacheKey, &answer, cache.FastCacheConfig.TTL, func() (interface{}, error) {
		var dbAnswer models.StudentAnswer
		if err := ar.db.WithContext(ctx).
			Where("attempt_id = ? AND question_id = ?", attemptID, questionID).
			First(&dbAnswer).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, gorm.ErrRecordNotFound
			}
			return nil, fmt.Errorf("failed to get answer: %w", err)
		}
		return &dbAnswer, nil
	})

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, gorm.ErrRecordNotFound
	}

	return &answer, err
}

// GetByQuestion retrieves answers for a specific question
func (ar *AnswerPostgreSQL) GetByQuestion(ctx context.Context, questionID uint, filters repositories.AnswerFilters) ([]*models.StudentAnswer, error) {
	query := ar.db.WithContext(ctx).Where("question_id = ?", questionID)
	query = ar.applyAnswerFilters(query, filters)

	var answers []*models.StudentAnswer
	if err := query.Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get answers by question: %w", err)
	}

	return answers, nil
}

// GetByStudent retrieves answers for a specific student
func (ar *AnswerPostgreSQL) GetByStudent(ctx context.Context, studentID uint, filters repositories.AnswerFilters) ([]*models.StudentAnswer, error) {
	query := ar.db.WithContext(ctx).
		Joins("JOIN assessment_attempts aa ON aa.id = student_answers.attempt_id").
		Where("aa.student_id = ?", studentID)
	query = ar.applyAnswerFilters(query, filters)

	var answers []*models.StudentAnswer
	if err := query.Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get answers by student: %w", err)
	}

	return answers, nil
}

// ===== GRADING OPERATIONS =====

// UpdateGrade updates the grade for an answer
func (ar *AnswerPostgreSQL) UpdateGrade(ctx context.Context, id uint, score float64, isCorrect *bool, feedback *string, graderID uint) error {
	now := time.Now()
	updates := map[string]interface{}{
		"score":     score,
		"graded_by": graderID,
		"graded_at": &now,
	}

	if isCorrect != nil {
		updates["is_correct"] = *isCorrect
	}
	if feedback != nil {
		updates["feedback"] = *feedback
	}

	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update grade: %w", err)
	}

	// Invalidate cache
	ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", id))

	return nil
}

// BulkGrade updates grades for multiple answers
func (ar *AnswerPostgreSQL) BulkGrade(ctx context.Context, grades []repositories.AnswerGrade) error {
	if len(grades) == 0 {
		return nil
	}

	return ar.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()

		for _, grade := range grades {
			updates := map[string]interface{}{
				"score":     grade.Score,
				"graded_by": grade.GraderID,
				"graded_at": &now,
			}

			if grade.Feedback != nil {
				updates["feedback"] = *grade.Feedback
			}

			if err := tx.Model(&models.StudentAnswer{}).
				Where("id = ?", grade.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to update grade for answer %d: %w", grade.ID, err)
			}

			// Invalidate cache
			ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", grade.ID))
		}

		return nil
	})
}

// GetPendingGrading retrieves answers pending manual grading
func (ar *AnswerPostgreSQL) GetPendingGrading(ctx context.Context, teacherID uint) ([]*models.StudentAnswer, error) {
	var answers []*models.StudentAnswer
	if err := ar.db.WithContext(ctx).
		Joins("JOIN assessment_attempts aa ON aa.id = student_answers.attempt_id").
		Joins("JOIN assessments a ON a.id = aa.assessment_id").
		Where("a.created_by = ? AND student_answers.graded_at IS NULL", teacherID).
		Preload("Attempt").
		Preload("Question").
		Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending grading: %w", err)
	}

	return answers, nil
}

// GetGradedAnswers retrieves answers graded by a specific teacher
func (ar *AnswerPostgreSQL) GetGradedAnswers(ctx context.Context, graderID uint, filters repositories.AnswerFilters) ([]*models.StudentAnswer, error) {
	query := ar.db.WithContext(ctx).Where("graded_by = ?", graderID)
	query = ar.applyAnswerFilters(query, filters)

	var answers []*models.StudentAnswer
	if err := query.Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get graded answers: %w", err)
	}

	return answers, nil
}

// ===== ANSWER TRACKING =====

// UpdateAnswerHistory updates the history of answer changes
func (ar *AnswerPostgreSQL) UpdateAnswerHistory(ctx context.Context, id uint, newAnswer interface{}) error {
	// Get current answer
	_, err := ar.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Create history entry
	_ = repositories.AnswerHistoryEntry{
		Timestamp: time.Now(),
		Answer:    newAnswer,
		Action:    "updated",
	}

	// In a full implementation, you would store this in a separate history table
	// For now, we'll just update the last_modified_at field
	now := time.Now()
	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("id = ?", id).
		Update("last_modified_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update answer history: %w", err)
	}

	// Invalidate cache
	ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", id))

	return nil
}

// GetAnswerHistory retrieves the history of answer changes
func (ar *AnswerPostgreSQL) GetAnswerHistory(ctx context.Context, id uint) ([]repositories.AnswerHistoryEntry, error) {
	// This would require a separate answer_history table in a full implementation
	// For now, return empty history
	return []repositories.AnswerHistoryEntry{}, nil
}

// FlagAnswer flags/unflags an answer for review
func (ar *AnswerPostgreSQL) FlagAnswer(ctx context.Context, id uint, flagged bool) error {
	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("id = ?", id).
		Update("is_flagged", flagged).Error; err != nil {
		return fmt.Errorf("failed to flag answer: %w", err)
	}

	// Invalidate cache
	ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", id))

	return nil
}

// GetFlaggedAnswers retrieves flagged answers for an attempt
func (ar *AnswerPostgreSQL) GetFlaggedAnswers(ctx context.Context, attemptID uint) ([]*models.StudentAnswer, error) {
	var answers []*models.StudentAnswer
	if err := ar.db.WithContext(ctx).
		Where("attempt_id = ? AND is_flagged = true", attemptID).
		Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get flagged answers: %w", err)
	}

	return answers, nil
}

// ===== TIME TRACKING =====

// UpdateTimeSpent updates the time spent on an answer
func (ar *AnswerPostgreSQL) UpdateTimeSpent(ctx context.Context, id uint, timeSpent int) error {
	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("id = ?", id).
		Update("time_spent", timeSpent).Error; err != nil {
		return fmt.Errorf("failed to update time spent: %w", err)
	}

	// Invalidate cache
	ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", id))

	return nil
}

// GetTimeSpentByQuestion retrieves time spent per question for an attempt
func (ar *AnswerPostgreSQL) GetTimeSpentByQuestion(ctx context.Context, attemptID uint) (map[uint]int, error) {
	var results []struct {
		QuestionID uint `json:"question_id"`
		TimeSpent  int  `json:"time_spent"`
	}

	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Select("question_id, time_spent").
		Where("attempt_id = ?", attemptID).
		Find(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get time spent by question: %w", err)
	}

	timeSpent := make(map[uint]int)
	for _, result := range results {
		timeSpent[result.QuestionID] = result.TimeSpent
	}

	return timeSpent, nil
}

// ===== STATISTICS AND ANALYTICS =====

// GetAnswerStats retrieves statistics for a question
func (ar *AnswerPostgreSQL) GetAnswerStats(ctx context.Context, questionID uint) (*repositories.AnswerStats, error) {
	stats := &repositories.AnswerStats{
		QuestionID:         questionID,
		AnswerDistribution: make(map[string]int),
	}

	// Get total answers
	var totalAnswers int64
	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("question_id = ?", questionID).
		Count(&totalAnswers).Error; err != nil {
		return nil, fmt.Errorf("failed to count answers: %w", err)
	}
	stats.TotalAnswers = int(totalAnswers)

	// Get correct answers
	var correctAnswers int64
	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("question_id = ? AND is_correct = true", questionID).
		Count(&correctAnswers).Error; err != nil {
		return nil, fmt.Errorf("failed to count correct answers: %w", err)
	}
	stats.CorrectAnswers = int(correctAnswers)

	if totalAnswers > 0 {
		stats.CorrectRate = float64(correctAnswers) / float64(totalAnswers)
	}

	// Get average score and time
	var avgResult struct {
		AvgScore float64
		AvgTime  int
	}
	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Select("AVG(score) as avg_score, AVG(time_spent) as avg_time").
		Where("question_id = ?", questionID).
		Scan(&avgResult).Error; err != nil {
		return nil, fmt.Errorf("failed to get averages: %w", err)
	}

	stats.AverageScore = avgResult.AvgScore
	stats.AverageTimeSpent = avgResult.AvgTime

	return stats, nil
}

// GetStudentAnswerStats retrieves answer statistics for a student
func (ar *AnswerPostgreSQL) GetStudentAnswerStats(ctx context.Context, studentID uint) (*repositories.StudentAnswerStats, error) {
	stats := &repositories.StudentAnswerStats{
		StudentID:         studentID,
		AnswersByType:     make(map[models.QuestionType]int),
		PerformanceByDiff: make(map[models.DifficultyLevel]float64),
	}

	// Get total answers through attempts
	var totalAnswers int64
	if err := ar.db.WithContext(ctx).
		Table("student_answers sa").
		Joins("JOIN assessment_attempts aa ON aa.id = sa.attempt_id").
		Where("aa.student_id = ?", studentID).
		Count(&totalAnswers).Error; err != nil {
		return nil, fmt.Errorf("failed to count student answers: %w", err)
	}
	stats.TotalAnswers = int(totalAnswers)

	// Get correct answers
	var correctAnswers int64
	if err := ar.db.WithContext(ctx).
		Table("student_answers sa").
		Joins("JOIN assessment_attempts aa ON aa.id = sa.attempt_id").
		Where("aa.student_id = ? AND sa.is_correct = true", studentID).
		Count(&correctAnswers).Error; err != nil {
		return nil, fmt.Errorf("failed to count correct answers: %w", err)
	}
	stats.CorrectAnswers = int(correctAnswers)

	if totalAnswers > 0 {
		stats.CorrectRate = float64(correctAnswers) / float64(totalAnswers)
	}

	// Get averages
	var avgResult struct {
		AvgScore float64
		AvgTime  int
	}
	if err := ar.db.WithContext(ctx).
		Table("student_answers sa").
		Joins("JOIN assessment_attempts aa ON aa.id = sa.attempt_id").
		Select("AVG(sa.score) as avg_score, AVG(sa.time_spent) as avg_time").
		Where("aa.student_id = ?", studentID).
		Scan(&avgResult).Error; err != nil {
		return nil, fmt.Errorf("failed to get student averages: %w", err)
	}

	stats.AverageScore = avgResult.AvgScore
	stats.TotalTimeSpent = avgResult.AvgTime

	// Get flagged count
	var flaggedCount int64
	if err := ar.db.WithContext(ctx).
		Table("student_answers sa").
		Joins("JOIN assessment_attempts aa ON aa.id = sa.attempt_id").
		Where("aa.student_id = ? AND sa.is_flagged = true", studentID).
		Count(&flaggedCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count flagged answers: %w", err)
	}
	stats.FlaggedCount = int(flaggedCount)

	return stats, nil
}

// GetAnswerDistribution retrieves the distribution of answers for a question
func (ar *AnswerPostgreSQL) GetAnswerDistribution(ctx context.Context, questionID uint) (*repositories.AnswerDistribution, error) {
	distribution := &repositories.AnswerDistribution{
		QuestionID:   questionID,
		Distribution: make(map[string]int),
	}

	// Get question type
	var question models.Question
	if err := ar.db.WithContext(ctx).Select("type").First(&question, questionID).Error; err != nil {
		return nil, fmt.Errorf("failed to get question type: %w", err)
	}
	distribution.QuestionType = question.Type

	// Get total answers
	var totalAnswers int64
	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("question_id = ?", questionID).
		Count(&totalAnswers).Error; err != nil {
		return nil, fmt.Errorf("failed to count total answers: %w", err)
	}
	distribution.TotalAnswers = int(totalAnswers)

	// For now, return basic distribution
	// In a full implementation, you would parse the JSON answers and create distribution

	return distribution, nil
}

// ===== VALIDATION =====

// HasAnswer checks if an answer exists for an attempt and question
func (ar *AnswerPostgreSQL) HasAnswer(ctx context.Context, attemptID, questionID uint) (bool, error) {
	cacheKey := fmt.Sprintf("has:attempt:%d:question:%d", attemptID, questionID)

	exists, err := ar.cacheManager.Exists.GetString(ctx, cacheKey)
	if err == nil {
		return exists == "true", nil
	}

	var count int64
	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("attempt_id = ? AND question_id = ?", attemptID, questionID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check answer existence: %w", err)
	}

	hasAnswer := count > 0
	// Cache with short TTL
	ar.cacheManager.Exists.SetString(ctx, cacheKey, fmt.Sprintf("%t", hasAnswer), cache.ExistsCacheConfig.TTL)

	return hasAnswer, nil
}

// GetAnsweredQuestions retrieves list of answered question IDs for an attempt
func (ar *AnswerPostgreSQL) GetAnsweredQuestions(ctx context.Context, attemptID uint) ([]uint, error) {
	var questionIDs []uint
	if err := ar.db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("attempt_id = ?", attemptID).
		Pluck("question_id", &questionIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to get answered questions: %w", err)
	}

	return questionIDs, nil
}

// GetUnansweredQuestions retrieves list of unanswered question IDs for an attempt
func (ar *AnswerPostgreSQL) GetUnansweredQuestions(ctx context.Context, attemptID uint) ([]uint, error) {
	// Get all question IDs for the assessment
	var allQuestionIDs []uint
	if err := ar.db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN assessment_attempts aa ON aa.assessment_id = aq.assessment_id").
		Where("aa.id = ?", attemptID).
		Pluck("aq.question_id", &allQuestionIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessment questions: %w", err)
	}

	// Get answered question IDs
	answeredIDs, err := ar.GetAnsweredQuestions(ctx, attemptID)
	if err != nil {
		return nil, err
	}

	// Create map for quick lookup
	answeredMap := make(map[uint]bool)
	for _, id := range answeredIDs {
		answeredMap[id] = true
	}

	// Find unanswered questions
	var unansweredIDs []uint
	for _, id := range allQuestionIDs {
		if !answeredMap[id] {
			unansweredIDs = append(unansweredIDs, id)
		}
	}

	return unansweredIDs, nil
}

// ===== HELPER METHODS =====

// applyAnswerFilters applies common answer filters to a query
func (ar *AnswerPostgreSQL) applyAnswerFilters(query *gorm.DB, filters repositories.AnswerFilters) *gorm.DB {
	if filters.IsGraded != nil {
		if *filters.IsGraded {
			query = query.Where("graded_at IS NOT NULL")
		} else {
			query = query.Where("graded_at IS NULL")
		}
	}
	if filters.GradedBy != nil {
		query = query.Where("graded_by = ?", *filters.GradedBy)
	}
	if filters.DateFrom != nil {
		query = query.Where("created_at >= ?", *filters.DateFrom)
	}
	if filters.DateTo != nil {
		query = query.Where("created_at <= ?", *filters.DateTo)
	}

	// Apply pagination
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}
	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}

	return query
}
