package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

type AttemptPostgreSQL struct {
	db      *gorm.DB
	helpers *SharedHelpers
}

func NewAttemptPostgreSQL(db *gorm.DB) repositories.AttemptRepository {
	return &AttemptPostgreSQL{
		db:      db,
		helpers: NewSharedHelpers(db),
	}
}

func (a AttemptPostgreSQL) Create(ctx context.Context, attempt *models.AssessmentAttempt) error {
	return a.db.WithContext(ctx).Create(attempt).Error
}

func (a AttemptPostgreSQL) GetByID(ctx context.Context, id uint) (*models.AssessmentAttempt, error) {
	var attempt models.AssessmentAttempt
	if err := a.db.WithContext(ctx).First(&attempt, id).Error; err != nil {
		return nil, err
	}

	return &attempt, nil
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
