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

func (a *AttemptPostgreSQL) Create(ctx context.Context, tx *gorm.DB, attempt *models.AssessmentAttempt) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Create(attempt).Error
}

func (a *AttemptPostgreSQL) GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	// Cache active attempts for performance
	//cacheKey := fmt.Sprintf("id:%d", id)

	var dbAttempt models.AssessmentAttempt
	if err := db.WithContext(ctx).First(&dbAttempt, id).Error; err != nil {
		return nil, fmt.Errorf("failed to get attempt: %w", err)
	}
	return &dbAttempt, nil
}

func (a *AttemptPostgreSQL) GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	var attempt models.AssessmentAttempt
	if err := db.WithContext(ctx).
		Preload("Student").
		Preload("Assessment").
		Preload("Assessment.Settings").
		Preload("Answers").
		Preload("Answers.Question").
		// Preload("ProctoringEvents").
		First(&attempt, id).Error; err != nil {
		return nil, err
	}
	return &attempt, nil
}

func (a *AttemptPostgreSQL) Update(ctx context.Context, tx *gorm.DB, attempt *models.AssessmentAttempt) error {
	db := a.getDB(tx)
	// Use Select to avoid cascading to associations (Student, Assessment, etc.)
	// This prevents FK constraint errors when Student doesn't exist in DB yet
	return db.WithContext(ctx).Model(attempt).Select("*").Omit("Student", "Assessment", "Answers", "ProctoringEvents").Updates(attempt).Error
}

func (a *AttemptPostgreSQL) Delete(ctx context.Context, tx *gorm.DB, id uint) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Delete(&models.AssessmentAttempt{}, id).Error
}

func (a *AttemptPostgreSQL) List(ctx context.Context, tx *gorm.DB, filters repositories.AttemptFilters) ([]*models.AssessmentAttempt, int64, error) {
	db := a.getDB(tx)
	var attempts []*models.AssessmentAttempt
	var total int64

	// apply filter first
	query := db.WithContext(ctx).Model(&models.AssessmentAttempt{})
	query = a.applyFiltersAttempt(query, filters)

	if filters.IsTeacherView {
		query = query.Joins("JOIN assessments ON assessments.id = assessment_attempts.assessment_id AND "+
			"assessments.deleted_at IS NULL").
			Where("assessments.created_by = ?", filters.TeacherId)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// then apply pagination and sorting
	query = a.applyPaginationAndSortAttempt(query, filters)

	if err := query.Preload("Student").
		Preload("Assessment").
		Preload("Assessment.Settings").
		Find(&attempts).Error; err != nil {
		return nil, 0, err
	}

	return attempts, total, nil
}

func (a *AttemptPostgreSQL) GetByStudent(ctx context.Context, tx *gorm.DB, studentID string, filters repositories.AttemptFilters) ([]*models.AssessmentAttempt, int64, error) {
	filters.UserID = &studentID
	return a.List(ctx, tx, filters)
}

func (a *AttemptPostgreSQL) GetByAssessment(ctx context.Context, tx *gorm.DB, assessmentID uint, filters repositories.AttemptFilters) ([]*models.AssessmentAttempt, int64, error) {
	db := a.getDB(tx)
	var attempts []*models.AssessmentAttempt
	var total int64

	query := db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("assessment_id = ?", assessmentID)
	query = a.applyFiltersAttempt(query, filters)

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = a.applyPaginationAndSortAttempt(query, filters)

	if err := query.Preload("Student").Preload("Assessment").Preload("Assessment.Settings").Find(&attempts).Error; err != nil {
		return nil, 0, err
	}

	return attempts, total, nil
}

//func (a *AttemptPostgreSQL) GetByStudentAndAssessment(ctx context.Context, tx *gorm.DB, studentID, assessmentID uint) ([]*models.AssessmentAttempt, error) {
//	db := a.getDB(tx)
//	var attempts []*models.AssessmentAttempt
//	if err := db.WithContext(ctx).
//		Where("student_id = ? AND assessment_id = ?", studentID, assessmentID).
//		Preload("Student").
//		Preload("Assessment").
//		Find(&attempts).Error; err != nil {
//		return nil, err
//	}
//
//	return attempts, nil
//}

// GetByStudentAndAssessment retrieves all attempts by a student for a specific assessment
func (a *AttemptPostgreSQL) GetByStudentAndAssessment(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) ([]*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	var attempts []*models.AssessmentAttempt
	if err := db.WithContext(ctx).
		Where("student_id = ? AND assessment_id = ?", studentID, assessmentID).
		Order("created_at DESC").
		Find(&attempts).Error; err != nil {
		return nil, fmt.Errorf("failed to get attempts by student and assessment: %w", err)
	}
	return attempts, nil
}

func (a *AttemptPostgreSQL) GetActiveAttempt(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	var attempt models.AssessmentAttempt
	if err := db.WithContext(ctx).
		Where("student_id = ? AND assessment_id = ? AND status = ?", studentID, assessmentID,
			models.AttemptInProgress).
		Preload("Student").
		Preload("Assessment").
		First(&attempt).Error; err != nil {
		//if errors.Is(err, gorm.ErrRecordNotFound) {
		//	return nil, NotExis
		//}
		return nil, err
	}

	return &attempt, nil
}

func (a *AttemptPostgreSQL) HasActiveAttempt(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (bool, error) {
	db := a.getDB(tx)
	var count int64
	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND assessment_id = ? AND status = ?", studentID, assessmentID, models.AttemptInProgress).
		Count(&count).Error; err != nil {
		return false, err
	}

	return count > 0, nil
}

func (a *AttemptPostgreSQL) GetActiveAttempts(ctx context.Context, tx *gorm.DB, studentID string) ([]*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	var attempts []*models.AssessmentAttempt
	if err := db.WithContext(ctx).
		Where("student_id = ? AND status = ?", studentID, models.AttemptInProgress).
		Preload("Student").
		Preload("Assessment").
		Preload("ProctoringEvents").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a *AttemptPostgreSQL) UpdateStatus(ctx context.Context, tx *gorm.DB, id uint, status models.AttemptStatus) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("id = ?", id).Update("status", status).Error
}

func (a *AttemptPostgreSQL) BulkUpdateStatus(ctx context.Context, tx *gorm.DB, ids []uint, status models.AttemptStatus) error {
	return a.helpers.BulkUpdateAttemptStatus(ctx, ids, status)
}

func (a *AttemptPostgreSQL) GetByStatus(ctx context.Context, tx *gorm.DB, status models.AttemptStatus, limit, offset int) ([]*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	var attempts []*models.AssessmentAttempt
	query := db.WithContext(ctx).Where("status = ?", status)

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

func (a *AttemptPostgreSQL) UpdateTimeRemaining(ctx context.Context, tx *gorm.DB, id uint, timeRemaining int) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("id = ?", id).Update("time_remaining", timeRemaining).Error
}

func (a *AttemptPostgreSQL) GetInProgressAttempts(ctx context.Context, tx *gorm.DB) ([]*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	var attempts []*models.AssessmentAttempt
	if err := db.WithContext(ctx).
		Where("status = ?", models.AttemptInProgress).
		Preload("Student").
		Preload("Assessment").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a *AttemptPostgreSQL) GetTimedOutAttempts(ctx context.Context, tx *gorm.DB) ([]*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	var attempts []*models.AssessmentAttempt
	if err := db.WithContext(ctx).
		Where("status = ? AND time_remaining <= 0", models.AttemptInProgress).
		Preload("Student").
		Preload("Assessment").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a *AttemptPostgreSQL) GetExpiredAttempts(ctx context.Context, tx *gorm.DB, cutoffTime time.Time) ([]*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	var attempts []*models.AssessmentAttempt
	if err := db.WithContext(ctx).
		Where("status = ? AND started_at <= ?", models.AttemptInProgress, cutoffTime).
		Preload("Student").
		Preload("Assessment").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a *AttemptPostgreSQL) UpdateProgress(ctx context.Context, tx *gorm.DB, id uint, currentQuestionIndex, questionsAnswered int) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"current_question_index": currentQuestionIndex,
			"questions_answered":     questionsAnswered,
		}).Error
}

func (a *AttemptPostgreSQL) GetProgress(ctx context.Context, tx *gorm.DB, id uint) (*repositories.AttemptProgress, error) {
	db := a.getDB(tx)
	var attempt models.AssessmentAttempt
	if err := db.WithContext(ctx).Preload("Assessment").
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

func (a *AttemptPostgreSQL) UpdateScore(ctx context.Context, tx *gorm.DB, id uint, score, percentage float64, passed bool) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"score":      score,
			"percentage": percentage,
			"passed":     passed,
		}).Error
}

func (a *AttemptPostgreSQL) CompleteAttempt(ctx context.Context, tx *gorm.DB, id uint, completedAt time.Time, finalScore float64) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       models.AttemptCompleted,
			"completed_at": completedAt,
			"score":        finalScore,
		}).Error
}

func (a *AttemptPostgreSQL) GetAttemptCount(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (int, error) {
	count, err := a.helpers.CountAttemptsByStudent(ctx, assessmentID, studentID)
	return int(count), err
}

// GetAssessmentAttemptStats retrieves statistics for assessment attempts
// Optimized: consolidated 6 queries into 2
func (a *AttemptPostgreSQL) GetAssessmentAttemptStats(ctx context.Context, tx *gorm.DB, assessmentID uint) (*repositories.AttemptStats, error) {
	db := a.getDB(tx) // Fixed: use getDB(tx) instead of a.db

	// Query 1: All counts and aggregates in single query
	var result struct {
		TotalAttempts   int64   `gorm:"column:total_attempts"`
		InProgressCount int64   `gorm:"column:in_progress_count"`
		CompletedCount  int64   `gorm:"column:completed_count"`
		AbandonedCount  int64   `gorm:"column:abandoned_count"`
		TimeOutCount    int64   `gorm:"column:time_out_count"`
		PassedCount     int64   `gorm:"column:passed_count"`
		AvgScore        float64 `gorm:"column:avg_score"`
		AvgTimeSpent    float64 `gorm:"column:avg_time_spent"`
	}

	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Select(`
			COUNT(*) as total_attempts,
			COUNT(CASE WHEN status = ? THEN 1 END) as in_progress_count,
			COUNT(CASE WHEN status = ? THEN 1 END) as completed_count,
			COUNT(CASE WHEN status = ? THEN 1 END) as abandoned_count,
			COUNT(CASE WHEN status = ? THEN 1 END) as time_out_count,
			SUM(CASE WHEN status = ? AND passed = true THEN 1 ELSE 0 END) as passed_count,
			AVG(CASE WHEN status = ? THEN score END) as avg_score,
			AVG(CASE WHEN status = ? THEN time_spent END) as avg_time_spent
		`, models.AttemptInProgress, models.AttemptCompleted, models.AttemptAbandoned, models.AttemptTimeOut,
			models.AttemptCompleted, models.AttemptCompleted, models.AttemptCompleted).
		Where("assessment_id = ?", assessmentID).
		Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessment attempt stats: %w", err)
	}

	passRate := float64(0)
	if result.CompletedCount > 0 {
		passRate = float64(result.PassedCount) / float64(result.CompletedCount)
	}

	completionRate := float64(0)
	if result.TotalAttempts > 0 {
		completionRate = float64(result.CompletedCount) / float64(result.TotalAttempts)
	}

	return &repositories.AttemptStats{
		TotalAttempts: int(result.TotalAttempts),
		StatusBreakdown: map[models.AttemptStatus]int{
			models.AttemptInProgress: int(result.InProgressCount),
			models.AttemptCompleted:  int(result.CompletedCount),
			models.AttemptAbandoned:  int(result.AbandonedCount),
			models.AttemptTimeOut:    int(result.TimeOutCount),
		},
		AverageScore:     result.AvgScore,
		AverageTimeSpent: int(result.AvgTimeSpent),
		PassRate:         passRate,
		CompletionRate:   completionRate,
	}, nil
}

// GetStudentAttemptStats retrieves statistics for a student's attempts
// Optimized: consolidated 11 queries into 2
func (a *AttemptPostgreSQL) GetStudentAttemptStats(ctx context.Context, tx *gorm.DB, studentID string) (*repositories.StudentAttemptStats, error) {
	db := a.getDB(tx) // Fixed: use getDB(tx) instead of a.db

	// Query 1: All counts and aggregates in single query
	var result struct {
		TotalAttempts     int64   `gorm:"column:total_attempts"`
		CompletedAttempts int64   `gorm:"column:completed_attempts"`
		InProgressCount   int64   `gorm:"column:in_progress_count"`
		AbandonedCount    int64   `gorm:"column:abandoned_count"`
		TimeOutCount      int64   `gorm:"column:time_out_count"`
		PassedCount       int64   `gorm:"column:passed_count"`
		AvgScore          float64 `gorm:"column:avg_score"`
		BestScore         float64 `gorm:"column:best_score"`
		TotalTimeSpent    int64   `gorm:"column:total_time_spent"`
	}

	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Select(`
			COUNT(*) as total_attempts,
			COUNT(CASE WHEN status = ? THEN 1 END) as completed_attempts,
			COUNT(CASE WHEN status = ? THEN 1 END) as in_progress_count,
			COUNT(CASE WHEN status = ? THEN 1 END) as abandoned_count,
			COUNT(CASE WHEN status = ? THEN 1 END) as time_out_count,
			SUM(CASE WHEN status = ? AND passed = true THEN 1 ELSE 0 END) as passed_count,
			COALESCE(AVG(CASE WHEN status = ? AND is_graded = true THEN score END), 0) as avg_score,
			COALESCE(MAX(CASE WHEN status = ? AND is_graded = true THEN score END), 0) as best_score,
			COALESCE(SUM(CASE WHEN status = ? THEN time_spent END), 0) as total_time_spent
		`, models.AttemptCompleted, models.AttemptInProgress, models.AttemptAbandoned, models.AttemptTimeOut,
			models.AttemptCompleted, models.AttemptCompleted, models.AttemptCompleted, models.AttemptCompleted).
		Where("student_id = ?", studentID).
		Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to get student attempt stats: %w", err)
	}

	// Query 2: Distinct assessments count
	var assessmentCount int64
	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ?", studentID).
		Distinct("assessment_id").
		Count(&assessmentCount).Error; err != nil {
		return nil, fmt.Errorf("failed to count distinct assessments: %w", err)
	}

	return &repositories.StudentAttemptStats{
		TotalAttempts:      int(result.TotalAttempts),
		CompletedAttempts:  int(result.CompletedAttempts),
		InProgressAttempts: int(result.InProgressCount),
		AverageScore:       result.AvgScore,
		BestScore:          result.BestScore,
		TotalTimeSpent:     int(result.TotalTimeSpent),
		AssessmentsCount:   int(assessmentCount),
		PassedCount:        int(result.PassedCount),
		StatusBreakdown: map[models.AttemptStatus]int{
			models.AttemptInProgress: int(result.InProgressCount),
			models.AttemptCompleted:  int(result.CompletedAttempts),
			models.AttemptAbandoned:  int(result.AbandonedCount),
			models.AttemptTimeOut:    int(result.TimeOutCount),
		},
	}, nil
}

func (a *AttemptPostgreSQL) GetAttemptsByDateRange(ctx context.Context, tx *gorm.DB, from, to time.Time) ([]*models.AssessmentAttempt, error) {
	db := a.getDB(tx)
	var attempts []*models.AssessmentAttempt
	if err := db.WithContext(ctx).
		Where("created_at >= ? AND created_at <= ?", from, to).
		Preload("Student").
		Preload("Assessment").
		Find(&attempts).Error; err != nil {
		return nil, err
	}

	return attempts, nil
}

func (a *AttemptPostgreSQL) CanStartAttempt(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (*repositories.AttemptValidation, error) {
	return a.helpers.ValidateAttemptEligibility(ctx, assessmentID, studentID)
}

func (a *AttemptPostgreSQL) GetNextAttemptNumber(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (int, error) {
	count, err := a.helpers.CountAttemptsByStudent(ctx, assessmentID, studentID)
	return int(count) + 1, err
}

// GetRemainingAttempts calculates remaining attempts for a student
func (a *AttemptPostgreSQL) GetRemainingAttempts(ctx context.Context, assessmentID uint, studentID string) (int, error) {
	return a.helpers.GetRemainingAttempts(ctx, assessmentID, studentID)
}

func (a *AttemptPostgreSQL) HasCompletedAttempts(ctx context.Context, tx *gorm.DB, studentID string, assessmentID uint) (bool, error) {
	db := a.getDB(tx)
	var count int64
	err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("student_id = ? AND assessment_id = ? AND status = ?", studentID, assessmentID, models.AttemptCompleted).
		Count(&count).Error
	return count > 0, err
}

func (a *AttemptPostgreSQL) UpdateSessionData(ctx context.Context, tx *gorm.DB, id uint, sessionData interface{}) error {
	db := a.getDB(tx)
	return db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Update("session_data", sessionData).Error
}

func (a *AttemptPostgreSQL) GetSessionData(ctx context.Context, tx *gorm.DB, id uint) (interface{}, error) {
	db := a.getDB(tx)
	var sessionData interface{}
	if err := db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("id = ?", id).
		Select("session_data").
		Scan(&sessionData).Error; err != nil {
		return nil, err
	}

	return sessionData, nil
}

// applyFiltersAttempt applies common filters to a query
func (a *AttemptPostgreSQL) applyFiltersAttempt(query *gorm.DB, filters repositories.AttemptFilters) *gorm.DB {
	return a.helpers.ApplyAttemptFilters(query, filters)
}

// getDB returns the transaction DB if provided, otherwise returns the default DB
func (a *AttemptPostgreSQL) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return a.db
}

// applyPaginationAndSortAttempt applies pagination and sorting to a query
func (a *AttemptPostgreSQL) applyPaginationAndSortAttempt(query *gorm.DB, filters repositories.AttemptFilters) *gorm.DB {
	return a.helpers.ApplyPaginationAndSort(query, filters.SortBy, filters.SortOrder, filters.Limit, filters.Offset)
}

// ===== ANSWER REPOSITORY IMPLEMENTATION =====

// AnswerPostgreSQL implements the AnswerRepository interface
type AnswerPostgreSQL struct {
	db           *gorm.DB
	helpers      *SharedHelpers
	cacheManager *cache.CacheManager
}

func (ar *AnswerPostgreSQL) AreAllAnswersGraded(ctx context.Context, tx *gorm.DB, attemptID uint) (bool, error) {
	db := ar.getDB(tx)
	var isAllGraded bool

	err := db.WithContext(ctx).Model(&models.StudentAnswer{}).
		Select("bool_and(is_graded)").
		Where("attempt_id = ?", attemptID).
		Scan(&isAllGraded).Error
	if err != nil {
		return false, fmt.Errorf("failed to check if all answers are graded: %w", err)
	}

	return isAllGraded, nil
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
func (ar *AnswerPostgreSQL) Create(ctx context.Context, tx *gorm.DB, answer *models.StudentAnswer) error {
	db := ar.getDB(tx)
	if err := db.WithContext(ctx).Create(answer).Error; err != nil {
		return fmt.Errorf("failed to create answer: %w", err)
	}

	// Invalidate related caches
	//ar.cacheManager.Fast.Delete(ctx,
	//	fmt.Sprintf("attempt:%d:answers", answer.AttemptID),
	//	fmt.Sprintf("attempt:%d:question:%d", answer.AttemptID, answer.QuestionID),
	//)

	return nil
}

// GetByID retrieves an answer by ID with caching
func (ar *AnswerPostgreSQL) GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.StudentAnswer, error) {
	db := ar.getDB(tx)

	var dbAnswer models.StudentAnswer
	if err := db.WithContext(ctx).First(&dbAnswer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("answer not found with ID %d", id)
		}
		return nil, fmt.Errorf("failed to get answer: %w", err)
	}
	return &dbAnswer, nil
}

// GetByIDWithDetails retrieves an answer by ID with related data
func (ar *AnswerPostgreSQL) GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.StudentAnswer, error) {
	db := ar.getDB(tx)
	var answer models.StudentAnswer
	if err := db.WithContext(ctx).
		Preload("Attempt").
		Preload("Question").
		First(&answer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("answer not found with ID %d", id)
		}
		return nil, fmt.Errorf("failed to get answer with details: %w", err)
	}
	return &answer, nil
}

// Update updates an existing answer
func (ar *AnswerPostgreSQL) Update(ctx context.Context, tx *gorm.DB, answer *models.StudentAnswer) error {
	db := ar.getDB(tx)
	newAnswer := &models.StudentAnswer{
		ID: answer.ID,
	}
	// Use Updates instead of Save to avoid cascading to associations
	// This prevents foreign key constraint errors when associations are loaded
	if err := db.WithContext(ctx).Model(newAnswer).Updates(map[string]interface{}{
		"answer":            answer.Answer,
		"score":             answer.Score,
		"max_score":         answer.MaxScore,
		"is_correct":        answer.IsCorrect,
		"graded_by":         answer.GradedBy,
		"graded_at":         answer.GradedAt,
		"feedback":          answer.Feedback,
		"time_spent":        answer.TimeSpent,
		"first_answered_at": answer.FirstAnsweredAt,
		"last_modified_at":  answer.LastModifiedAt,
		"answer_history":    answer.AnswerHistory,
		"flagged":           answer.Flagged,
		"is_graded":         answer.IsGraded,
		"updated_at":        answer.UpdatedAt,
	}).Error; err != nil {
		return fmt.Errorf("failed to update answer: %w", err)
	}

	// Invalidate caches
	//ar.cacheManager.Fast.Delete(ctx,
	//	fmt.Sprintf("answer:id:%d", answer.ID),
	//	fmt.Sprintf("attempt:%d:answers", answer.AttemptID),
	//	fmt.Sprintf("attempt:%d:question:%d", answer.AttemptID, answer.QuestionID),
	//)

	return nil
}

// Delete removes an answer
func (ar *AnswerPostgreSQL) Delete(ctx context.Context, tx *gorm.DB, id uint) error {
	db := ar.getDB(tx)
	// Get answer first to invalidate caches
	//answer, err := ar.GetByID(ctx, tx, id)
	//if err != nil {
	//	return err
	//}

	if err := db.WithContext(ctx).Delete(&models.StudentAnswer{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete answer: %w", err)
	}

	// Invalidate caches
	//ar.cacheManager.Fast.Delete(ctx,
	//	fmt.Sprintf("answer:id:%d", id),
	//	fmt.Sprintf("attempt:%d:answers", answer.AttemptID),
	//	fmt.Sprintf("attempt:%d:question:%d", answer.AttemptID, answer.QuestionID),
	//)

	return nil
}

// ===== BULK OPERATIONS =====

// CreateBatch creates multiple answers in a batch
func (ar *AnswerPostgreSQL) CreateBatch(ctx context.Context, tx *gorm.DB, answers []*models.StudentAnswer) error {
	if len(answers) == 0 {
		return nil
	}

	db := ar.getDB(tx)
	return db.WithContext(ctx).Transaction(func(txInner *gorm.DB) error {
		if err := txInner.CreateInBatches(answers, 100).Error; err != nil {
			return fmt.Errorf("failed to create answers batch: %w", err)
		}

		// Invalidate caches for all affected attempts
		attemptIDs := make(map[uint]bool)
		for _, answer := range answers {
			attemptIDs[answer.AttemptID] = true
		}

		//for attemptID := range attemptIDs {
		//	ar.cacheManager.Fast.InvalidatePattern(ctx, fmt.Sprintf("attempt:%d:*", attemptID))
		//}

		return nil
	})
}

// UpdateBatch updates multiple answers in a batch
func (ar *AnswerPostgreSQL) UpdateBatch(ctx context.Context, tx *gorm.DB, answers []*models.StudentAnswer) error {
	if len(answers) == 0 {
		return nil
	}

	db := ar.getDB(tx)
	return db.WithContext(ctx).Transaction(func(txInner *gorm.DB) error {
		// Batch update using GORM
		for _, answer := range answers {
			if err := txInner.Model(&models.StudentAnswer{}).Where("id = ?", answer.ID).Updates(map[string]interface{}{
				"answer":            answer.Answer,
				"score":             answer.Score,
				"max_score":         answer.MaxScore,
				"is_correct":        answer.IsCorrect,
				"graded_by":         answer.GradedBy,
				"graded_at":         answer.GradedAt,
				"feedback":          answer.Feedback,
				"time_spent":        answer.TimeSpent,
				"first_answered_at": answer.FirstAnsweredAt,
				"last_modified_at":  answer.LastModifiedAt,
				"answer_history":    answer.AnswerHistory,
				"flagged":           answer.Flagged,
				"is_graded":         answer.IsGraded,
			}).Error; err != nil {
				return fmt.Errorf("failed to update answer ID %d: %w", answer.ID, err)
			}
		}

		// Invalidate caches
		//attemptIDs := make(map[uint]bool)
		//for _, answer := range answers {
		//	ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", answer.ID))
		//	attemptIDs[answer.AttemptID] = true
		//}
		//for attemptID := range attemptIDs {
		//	ar.cacheManager.Fast.InvalidatePattern(ctx, fmt.Sprintf("attempt:%d:*", attemptID))
		//}

		return nil
	})
}

// UpsertAnswer creates or updates an answer
func (ar *AnswerPostgreSQL) UpsertAnswer(ctx context.Context, tx *gorm.DB, answer *models.StudentAnswer) error {
	// Check if answer already exists
	existing, err := ar.GetByAttemptAndQuestion(ctx, tx, answer.AttemptID, answer.QuestionID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("failed to check existing answer: %w", err)
	}

	if existing != nil {
		// Update existing answer
		answer.ID = existing.ID
		return ar.Update(ctx, tx, answer)
	}

	// Create new answer
	return ar.Create(ctx, tx, answer)
}

// ===== QUERY OPERATIONS =====

// GetByAttempt retrieves all answers for an attempt with caching
func (ar *AnswerPostgreSQL) GetByAttempt(ctx context.Context, tx *gorm.DB, attemptID uint) ([]*models.StudentAnswer, error) {
	db := ar.getDB(tx)

	var dbAnswers []*models.StudentAnswer
	if err := db.WithContext(ctx).
		Where("attempt_id = ?", attemptID).
		Order("question_id ASC").
		Preload("Question").
		Find(&dbAnswers).Error; err != nil {
		return nil, fmt.Errorf("failed to get answers by attempt: %w", err)
	}
	return dbAnswers, nil
}

// GetByAttemptAndQuestion retrieves a specific answer for an attempt and question
func (ar *AnswerPostgreSQL) GetByAttemptAndQuestion(ctx context.Context, tx *gorm.DB, attemptID, questionID uint) (*models.StudentAnswer, error) {
	db := ar.getDB(tx)
	var dbAnswer models.StudentAnswer
	if err := db.WithContext(ctx).
		Where("attempt_id = ? AND question_id = ?", attemptID, questionID).
		First(&dbAnswer).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, gorm.ErrRecordNotFound
		}
		return nil, fmt.Errorf("failed to get answer: %w", err)
	}

	return &dbAnswer, nil
}

// GetByQuestion retrieves answers for a specific question
func (ar *AnswerPostgreSQL) GetByQuestion(ctx context.Context, tx *gorm.DB, questionID uint, filters repositories.AnswerFilters) ([]*models.StudentAnswer, error) {
	db := ar.getDB(tx)
	query := db.WithContext(ctx).Where("question_id = ?", questionID)
	query = ar.applyAnswerFilters(query, filters)

	var answers []*models.StudentAnswer
	if err := query.Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get answers by question: %w", err)
	}

	return answers, nil
}

// GetByStudent retrieves answers for a specific student
func (ar *AnswerPostgreSQL) GetByStudent(ctx context.Context, tx *gorm.DB, studentID string, filters repositories.AnswerFilters) ([]*models.StudentAnswer, error) {
	db := ar.getDB(tx)
	query := db.WithContext(ctx).
		Joins("JOIN assessment_attempts aa ON aa.id = student_answers.attempt_id AND aa.deleted_at IS NULL").
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
func (ar *AnswerPostgreSQL) UpdateGrade(ctx context.Context, tx *gorm.DB, id uint, score float64, isCorrect *bool, feedback *string, graderID string) error {
	db := ar.getDB(tx)
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

	if err := db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("id = ?", id).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failed to update grade: %w", err)
	}

	// Invalidate cache
	//ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", id))

	return nil
}

// BulkGrade updates grades for multiple answers
func (ar *AnswerPostgreSQL) BulkGrade(ctx context.Context, tx *gorm.DB, grades []repositories.AnswerGrade) error {
	if len(grades) == 0 {
		return nil
	}

	db := ar.getDB(tx)
	return db.WithContext(ctx).Transaction(func(txInner *gorm.DB) error {
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

			if err := txInner.Model(&models.StudentAnswer{}).
				Where("id = ?", grade.ID).
				Updates(updates).Error; err != nil {
				return fmt.Errorf("failed to update grade for answer %d: %w", grade.ID, err)
			}

			// Invalidate cache
			// ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", grade.ID))
		}

		return nil
	})
}

// GetPendingGrading retrieves answers pending manual grading
func (ar *AnswerPostgreSQL) GetPendingGrading(ctx context.Context, tx *gorm.DB, teacherID string) ([]*models.StudentAnswer, error) {
	db := ar.getDB(tx)
	var answers []*models.StudentAnswer
	if err := db.WithContext(ctx).
		Joins("JOIN assessment_attempts aa ON aa.id = student_answers.attempt_id AND aa.deleted_at IS NULL").
		Joins("JOIN assessments a ON a.id = aa.assessment_id AND a.deleted_at IS NULL").
		Where("a.created_by = ? AND student_answers.graded_at IS NULL", teacherID).
		Preload("Attempt").
		Preload("Question").
		Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get pending grading: %w", err)
	}

	return answers, nil
}

// GetGradedAnswers retrieves answers graded by a specific teacher
func (ar *AnswerPostgreSQL) GetGradedAnswers(ctx context.Context, tx *gorm.DB, graderID string, filters repositories.AnswerFilters) ([]*models.StudentAnswer, error) {
	db := ar.getDB(tx)
	query := db.WithContext(ctx).Where("graded_by = ?", graderID)
	query = ar.applyAnswerFilters(query, filters)

	var answers []*models.StudentAnswer
	if err := query.Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get graded answers: %w", err)
	}

	return answers, nil
}

// ===== ANSWER TRACKING =====

// UpdateAnswerHistory updates the history of answer changes
func (ar *AnswerPostgreSQL) UpdateAnswerHistory(ctx context.Context, tx *gorm.DB, id uint, newAnswer interface{}) error {
	db := ar.getDB(tx)
	// Get current answer
	_, err := ar.GetByID(ctx, tx, id)
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
	if err := db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("id = ?", id).
		Update("last_modified_at", &now).Error; err != nil {
		return fmt.Errorf("failed to update answer history: %w", err)
	}

	// Invalidate cache
	// ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", id))

	return nil
}

// GetAnswerHistory retrieves the history of answer changes
func (ar *AnswerPostgreSQL) GetAnswerHistory(ctx context.Context, tx *gorm.DB, id uint) ([]repositories.AnswerHistoryEntry, error) {
	// This would require a separate answer_history table in a full implementation
	// For now, return empty history
	return []repositories.AnswerHistoryEntry{}, nil
}

// FlagAnswer flags/unflags an answer for review
func (ar *AnswerPostgreSQL) FlagAnswer(ctx context.Context, tx *gorm.DB, id uint, flagged bool) error {
	db := ar.getDB(tx)
	if err := db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("id = ?", id).
		Update("is_flagged", flagged).Error; err != nil {
		return fmt.Errorf("failed to flag answer: %w", err)
	}

	// Invalidate cache
	// ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", id))

	return nil
}

// GetFlaggedAnswers retrieves flagged answers for an attempt
func (ar *AnswerPostgreSQL) GetFlaggedAnswers(ctx context.Context, tx *gorm.DB, attemptID uint) ([]*models.StudentAnswer, error) {
	db := ar.getDB(tx)
	var answers []*models.StudentAnswer
	if err := db.WithContext(ctx).
		Where("attempt_id = ? AND is_flagged = true", attemptID).
		Find(&answers).Error; err != nil {
		return nil, fmt.Errorf("failed to get flagged answers: %w", err)
	}

	return answers, nil
}

// ===== TIME TRACKING =====

// UpdateTimeSpent updates the time spent on an answer
func (ar *AnswerPostgreSQL) UpdateTimeSpent(ctx context.Context, tx *gorm.DB, id uint, timeSpent int) error {
	db := ar.getDB(tx)
	if err := db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("id = ?", id).
		Update("time_spent", timeSpent).Error; err != nil {
		return fmt.Errorf("failed to update time spent: %w", err)
	}

	// Invalidate cache
	// ar.cacheManager.Fast.Delete(ctx, fmt.Sprintf("answer:id:%d", id))

	return nil
}

// GetTimeSpentByQuestion retrieves time spent per question for an attempt
func (ar *AnswerPostgreSQL) GetTimeSpentByQuestion(ctx context.Context, tx *gorm.DB, attemptID uint) (map[uint]int, error) {
	db := ar.getDB(tx)
	var results []struct {
		QuestionID uint `json:"question_id"`
		TimeSpent  int  `json:"time_spent"`
	}

	if err := db.WithContext(ctx).
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
// Optimized: uses single query with conditional aggregates instead of 3 separate queries
func (ar *AnswerPostgreSQL) GetAnswerStats(ctx context.Context, tx *gorm.DB, questionID uint) (*repositories.AnswerStats, error) {
	db := ar.getDB(tx)

	var result struct {
		TotalAnswers   int64   `gorm:"column:total_answers"`
		CorrectAnswers int64   `gorm:"column:correct_answers"`
		AvgScore       float64 `gorm:"column:avg_score"`
		AvgTime        int     `gorm:"column:avg_time"`
	}

	if err := db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Select(`
			COUNT(*) as total_answers,
			COUNT(CASE WHEN is_correct = true THEN 1 END) as correct_answers,
			AVG(score) as avg_score,
			AVG(time_spent) as avg_time
		`).
		Where("question_id = ?", questionID).
		Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to get answer stats: %w", err)
	}

	correctRate := 0.0
	if result.TotalAnswers > 0 {
		correctRate = float64(result.CorrectAnswers) / float64(result.TotalAnswers)
	}

	return &repositories.AnswerStats{
		QuestionID:         questionID,
		TotalAnswers:       int(result.TotalAnswers),
		CorrectAnswers:     int(result.CorrectAnswers),
		CorrectRate:        correctRate,
		AverageScore:       result.AvgScore,
		AverageTimeSpent:   result.AvgTime,
		AnswerDistribution: make(map[string]int),
	}, nil
}

// GetStudentAnswerStats retrieves answer statistics for a student
// Optimized: uses single query with conditional aggregates instead of 4 separate queries
func (ar *AnswerPostgreSQL) GetStudentAnswerStats(ctx context.Context, tx *gorm.DB, studentID string) (*repositories.StudentAnswerStats, error) {
	db := ar.getDB(tx)

	var result struct {
		TotalAnswers   int64   `gorm:"column:total_answers"`
		CorrectAnswers int64   `gorm:"column:correct_answers"`
		AvgScore       float64 `gorm:"column:avg_score"`
		AvgTime        int     `gorm:"column:avg_time"`
		FlaggedCount   int64   `gorm:"column:flagged_count"`
	}

	if err := db.WithContext(ctx).
		Table("student_answers sa").
		Select(`
			COUNT(*) as total_answers,
			COUNT(CASE WHEN sa.is_correct = true THEN 1 END) as correct_answers,
			AVG(sa.score) as avg_score,
			AVG(sa.time_spent) as avg_time,
			COUNT(CASE WHEN sa.is_flagged = true THEN 1 END) as flagged_count
		`).
		Joins("JOIN assessment_attempts aa ON aa.id = sa.attempt_id AND aa.deleted_at IS NULL").
		Where("aa.student_id = ?", studentID).
		Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to get student answer stats: %w", err)
	}

	correctRate := 0.0
	if result.TotalAnswers > 0 {
		correctRate = float64(result.CorrectAnswers) / float64(result.TotalAnswers)
	}

	return &repositories.StudentAnswerStats{
		StudentID:         studentID,
		TotalAnswers:      int(result.TotalAnswers),
		CorrectAnswers:    int(result.CorrectAnswers),
		CorrectRate:       correctRate,
		AverageScore:      result.AvgScore,
		TotalTimeSpent:    result.AvgTime,
		FlaggedCount:      int(result.FlaggedCount),
		AnswersByType:     make(map[models.QuestionType]int),
		PerformanceByDiff: make(map[models.DifficultyLevel]float64),
	}, nil
}

// GetAnswerDistribution retrieves the distribution of answers for a question
func (ar *AnswerPostgreSQL) GetAnswerDistribution(ctx context.Context, tx *gorm.DB, questionID uint) (*repositories.AnswerDistribution, error) {
	db := ar.getDB(tx)
	distribution := &repositories.AnswerDistribution{
		QuestionID:   questionID,
		Distribution: make(map[string]int),
	}

	// Get question type
	var question models.Question
	if err := db.WithContext(ctx).Select("type").First(&question, questionID).Error; err != nil {
		return nil, fmt.Errorf("failed to get question type: %w", err)
	}
	distribution.QuestionType = question.Type

	// Get total answers
	var totalAnswers int64
	if err := db.WithContext(ctx).
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

// GetGradingStats retrieves grading statistics for a specific assessment
// Uses attempt-based metrics: is_graded = true means attempt is graded
// Supports optional filtering by groupID (only count attempts from students in the group)
func (ar *AnswerPostgreSQL) GetGradingStats(ctx context.Context, tx *gorm.DB, assessmentID uint, groupID *uint) (*repositories.GradingStats, error) {
	db := ar.getDB(tx)

	var result struct {
		TotalAttempts  int64   `gorm:"column:total_attempts"`
		GradedAttempts int64   `gorm:"column:graded_attempts"`
		AvgScore       float64 `gorm:"column:avg_score"`
	}

	query := db.WithContext(ctx).
		Table("assessment_attempts aa").
		Select(`
			COUNT(aa.id) as total_attempts,
			COUNT(CASE WHEN aa.is_graded = true THEN 1 END) as graded_attempts,
			COALESCE(AVG(CASE WHEN aa.is_graded = true THEN aa.score END), 0) as avg_score
		`).
		Where("aa.assessment_id = ? AND aa.deleted_at IS NULL", assessmentID)

	// Filter by group - only count attempts from students who are members of the group
	if groupID != nil {
		query = query.Joins("JOIN group_members gm ON gm.user_id = aa.student_id").
			Where("gm.group_id = ?", *groupID)
	}

	if err := query.Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to get grading stats: %w", err)
	}

	return &repositories.GradingStats{
		TotalAttempts:   int(result.TotalAttempts),
		GradedAttempts:  int(result.GradedAttempts),
		PendingAttempts: int(result.TotalAttempts - result.GradedAttempts),
		AverageScore:    result.AvgScore,
	}, nil
}

// GetGradingStatsOverview retrieves overall grading statistics across all assessments
// Uses attempt-based metrics: is_graded = true means attempt is graded
// Supports filtering by teacherID and/or groupID
func (ar *AnswerPostgreSQL) GetGradingStatsOverview(ctx context.Context, tx *gorm.DB, teacherID *string, groupID *uint) (*repositories.GradingStatsOverview, error) {
	db := ar.getDB(tx)

	var result struct {
		TotalAssessments int64   `gorm:"column:total_assessments"`
		TotalAttempts    int64   `gorm:"column:total_attempts"`
		GradedAttempts   int64   `gorm:"column:graded_attempts"`
		AvgScore         float64 `gorm:"column:avg_score"`
	}

	query := db.WithContext(ctx).
		Table("assessment_attempts aa").
		Select(`
			COUNT(DISTINCT a.id) as total_assessments,
			COUNT(aa.id) as total_attempts,
			COUNT(CASE WHEN aa.is_graded = true THEN 1 END) as graded_attempts,
			COALESCE(AVG(CASE WHEN aa.is_graded = true THEN aa.score END), 0) as avg_score
		`).
		Joins("JOIN assessments a ON a.id = aa.assessment_id AND a.deleted_at IS NULL").
		Where("aa.deleted_at IS NULL")

	// Filter by group if provided
	if groupID != nil {
		query = query.Joins("JOIN assessment_groups ag ON ag.assessment_id = a.id AND ag.deleted_at IS NULL").
			Where("ag.group_id = ?", *groupID)
	}

	// Filter by teacher if provided
	if teacherID != nil {
		query = query.Where("a.created_by = ?", *teacherID)
	}

	if err := query.Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("failed to get grading stats overview: %w", err)
	}

	return &repositories.GradingStatsOverview{
		TotalAssessments: int(result.TotalAssessments),
		TotalAttempts:    int(result.TotalAttempts),
		GradedAttempts:   int(result.GradedAttempts),
		PendingAttempts:  int(result.TotalAttempts - result.GradedAttempts),
		AverageScore:     result.AvgScore,
	}, nil
}

// ===== VALIDATION =====

// HasAnswer checks if an answer exists for an attempt and question
func (ar *AnswerPostgreSQL) HasAnswer(ctx context.Context, tx *gorm.DB, attemptID, questionID uint) (bool, error) {
	db := ar.getDB(tx)

	var count int64
	if err := db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("attempt_id = ? AND question_id = ?", attemptID, questionID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check answer existence: %w", err)
	}

	hasAnswer := count > 0
	// Cache with short TTL
	// ar.cacheManager.Exists.SetString(ctx, cacheKey, fmt.Sprintf("%t", hasAnswer), cache.ExistsCacheConfig.TTL)

	return hasAnswer, nil
}

// GetAnsweredQuestions retrieves list of answered question IDs for an attempt
func (ar *AnswerPostgreSQL) GetAnsweredQuestions(ctx context.Context, tx *gorm.DB, attemptID uint) ([]uint, error) {
	db := ar.getDB(tx)
	var questionIDs []uint
	if err := db.WithContext(ctx).
		Model(&models.StudentAnswer{}).
		Where("attempt_id = ?", attemptID).
		Pluck("question_id", &questionIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to get answered questions: %w", err)
	}

	return questionIDs, nil
}

// GetUnansweredQuestions retrieves list of unanswered question IDs for an attempt
func (ar *AnswerPostgreSQL) GetUnansweredQuestions(ctx context.Context, tx *gorm.DB, attemptID uint) ([]uint, error) {
	db := ar.getDB(tx)
	// Get all question IDs for the assessment
	var allQuestionIDs []uint
	if err := db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN assessment_attempts aa ON aa.assessment_id = aq.assessment_id AND aa.deleted_at IS NULL AND aq.deleted_at IS NULL").
		Where("aa.id = ?", attemptID).
		Pluck("aq.question_id", &allQuestionIDs).Error; err != nil {
		return nil, fmt.Errorf("failed to get assessment questions: %w", err)
	}

	// Get answered question IDs
	answeredIDs, err := ar.GetAnsweredQuestions(ctx, tx, attemptID)
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

// getDB returns the transaction DB if provided, otherwise returns the default DB
func (ar *AnswerPostgreSQL) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return ar.db
}

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
