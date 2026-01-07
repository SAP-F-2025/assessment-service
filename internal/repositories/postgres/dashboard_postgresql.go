package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

type dashboardRepository struct {
	db *gorm.DB
}

func NewDashboardRepository(db *gorm.DB) repositories.DashboardRepository {
	return &dashboardRepository{db: db}
}

func (r *dashboardRepository) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}

// applyTeacherFilter applies teacher filter if teacherID is provided
func (r *dashboardRepository) applyTeacherFilter(db *gorm.DB, teacherID *string, tableName string) *gorm.DB {
	if teacherID != nil && *teacherID != "" {
		// Apply filter based on table
		switch tableName {
		case "assessments", "questions", "question_banks":
			return db.Where(tableName+".created_by = ?", *teacherID)
		case "assessment_attempts":
			// For attempts, join with assessments and filter by assessment creator
			// Also filter out soft-deleted assessments
			return db.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
				Where("assessments.created_by = ?", *teacherID)
		}
	}
	return db
}

// ===== DASHBOARD STATS =====

func (r *dashboardRepository) GetTotalAssessments(ctx context.Context, tx *gorm.DB, teacherID *string) (int64, error) {
	db := r.getDB(tx)
	var count int64

	query := db.WithContext(ctx).Model(&models.Assessment{})
	query = r.applyTeacherFilter(query, teacherID, "assessments")

	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get total assessments: %w", err)
	}

	return count, nil
}

func (r *dashboardRepository) GetTotalQuestions(ctx context.Context, tx *gorm.DB, teacherID *string) (int64, error) {
	db := r.getDB(tx)
	var count int64

	query := db.WithContext(ctx).Model(&models.Question{})
	query = r.applyTeacherFilter(query, teacherID, "questions")

	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get total questions: %w", err)
	}

	return count, nil
}

func (r *dashboardRepository) GetTotalQuestionBanks(ctx context.Context, tx *gorm.DB, teacherID *string) (int64, error) {
	db := r.getDB(tx)
	var count int64

	query := db.WithContext(ctx).Model(&models.QuestionBank{})
	query = r.applyTeacherFilter(query, teacherID, "question_banks")

	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get total question banks: %w", err)
	}

	return count, nil
}

func (r *dashboardRepository) GetTotalAttempts(ctx context.Context, tx *gorm.DB, teacherID *string) (int64, error) {
	db := r.getDB(tx)
	var count int64

	query := db.WithContext(ctx).Model(&models.AssessmentAttempt{})

	// For attempts, we need special handling since we join with assessments
	if teacherID != nil && *teacherID != "" {
		query = query.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
			Where("assessments.created_by = ?", *teacherID)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get total attempts: %w", err)
	}

	return count, nil
}

func (r *dashboardRepository) GetActiveUsers(ctx context.Context, tx *gorm.DB, teacherID *string, days int) (int64, error) {
	db := r.getDB(tx)
	var count int64

	startDate := time.Now().AddDate(0, 0, -days)

	query := db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("assessment_attempts.created_at >= ?", startDate)

	// Filter by teacher if provided
	if teacherID != nil && *teacherID != "" {
		query = query.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
			Where("assessments.created_by = ?", *teacherID)
	}

	if err := query.Distinct("student_id").Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to get active users: %w", err)
	}

	return count, nil
}

// ===== METRICS =====

func (r *dashboardRepository) GetCompletionRate(ctx context.Context, tx *gorm.DB, teacherID *string) (float64, error) {
	db := r.getDB(tx)

	var total int64
	var completed int64

	// Get total attempts
	totalQuery := db.WithContext(ctx).Model(&models.AssessmentAttempt{})
	if teacherID != nil && *teacherID != "" {
		totalQuery = totalQuery.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
			Where("assessments.created_by = ?", *teacherID)
	}
	if err := totalQuery.Count(&total).Error; err != nil {
		return 0, fmt.Errorf("failed to get total attempts: %w", err)
	}

	if total == 0 {
		return 0, nil
	}

	// Get completed attempts
	completedQuery := db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("assessment_attempts.status = ?", models.AttemptCompleted)
	if teacherID != nil && *teacherID != "" {
		completedQuery = completedQuery.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
			Where("assessments.created_by = ?", *teacherID)
	}
	if err := completedQuery.Count(&completed).Error; err != nil {
		return 0, fmt.Errorf("failed to get completed attempts: %w", err)
	}

	return float64(completed) / float64(total) * 100, nil
}

func (r *dashboardRepository) GetAverageScore(ctx context.Context, tx *gorm.DB, teacherID *string) (float64, error) {
	db := r.getDB(tx)

	var result struct {
		AvgScore float64
	}

	query := db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("assessment_attempts.status = ?", models.AttemptCompleted)
	if teacherID != nil && *teacherID != "" {
		query = query.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
			Where("assessments.created_by = ?", *teacherID)
	}

	if err := query.Select("COALESCE(AVG(CASE WHEN assessment_attempts.is_graded = true THEN assessment_attempts.score END), 0) as avg_score").Scan(&result).Error; err != nil {
		return 0, fmt.Errorf("failed to get average score: %w", err)
	}

	return result.AvgScore, nil
}

func (r *dashboardRepository) GetPassRate(ctx context.Context, tx *gorm.DB, teacherID *string) (float64, error) {
	db := r.getDB(tx)

	var totalCompleted int64
	var passed int64

	// Get total completed attempts
	totalQuery := db.WithContext(ctx).Model(&models.AssessmentAttempt{}).Where("assessment_attempts.status = ?", models.AttemptCompleted)
	if teacherID != nil && *teacherID != "" {
		totalQuery = totalQuery.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
			Where("assessments.created_by = ?", *teacherID)
	}
	if err := totalQuery.Count(&totalCompleted).Error; err != nil {
		return 0, fmt.Errorf("failed to get total completed attempts: %w", err)
	}

	if totalCompleted == 0 {
		return 0, nil
	}

	// Get passed attempts
	passedQuery := db.WithContext(ctx).Model(&models.AssessmentAttempt{}).
		Where("assessment_attempts.status = ? AND assessment_attempts.passed = ?", models.AttemptCompleted, true)
	if teacherID != nil && *teacherID != "" {
		passedQuery = passedQuery.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
			Where("assessments.created_by = ?", *teacherID)
	}
	if err := passedQuery.Count(&passed).Error; err != nil {
		return 0, fmt.Errorf("failed to get passed attempts: %w", err)
	}

	return float64(passed) / float64(totalCompleted) * 100, nil
}

// ===== TRENDS =====

func (r *dashboardRepository) GetTrendChange(ctx context.Context, tx *gorm.DB, teacherID *string, entity string, days int) (float64, error) {
	db := r.getDB(tx)

	currentPeriodStart := time.Now().AddDate(0, 0, -days)
	previousPeriodStart := time.Now().AddDate(0, 0, -days*2)
	previousPeriodEnd := currentPeriodStart

	var currentCount int64
	var previousCount int64

	var model interface{}
	var tableName string
	switch entity {
	case "assessments":
		model = &models.Assessment{}
		tableName = "assessments"
	case "attempts":
		model = &models.AssessmentAttempt{}
		tableName = "assessment_attempts"
	case "questions":
		model = &models.Question{}
		tableName = "questions"
	default:
		return 0, fmt.Errorf("unsupported entity: %s", entity)
	}

	// Current period count
	currentQuery := db.WithContext(ctx).Model(model)
	if entity == "assessments" || entity == "questions" {
		currentQuery = currentQuery.Where(tableName+".created_at >= ?", currentPeriodStart)
		currentQuery = r.applyTeacherFilter(currentQuery, teacherID, tableName)
	} else if entity == "attempts" {
		currentQuery = currentQuery.Where(tableName+".created_at >= ?", currentPeriodStart)
		if teacherID != nil && *teacherID != "" {
			currentQuery = currentQuery.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
				Where("assessments.created_by = ?", *teacherID)
		}
	}
	if err := currentQuery.Count(&currentCount).Error; err != nil {
		return 0, fmt.Errorf("failed to get current count: %w", err)
	}

	// Previous period count
	previousQuery := db.WithContext(ctx).Model(model)
	if entity == "assessments" || entity == "questions" {
		previousQuery = previousQuery.Where(tableName+".created_at >= ? AND "+tableName+".created_at < ?",
			previousPeriodStart, previousPeriodEnd)
		previousQuery = r.applyTeacherFilter(previousQuery, teacherID, tableName)
	} else if entity == "attempts" {
		previousQuery = previousQuery.Where(tableName+".created_at >= ? AND "+tableName+".created_at < ?",
			previousPeriodStart, previousPeriodEnd)
		if teacherID != nil && *teacherID != "" {
			previousQuery = previousQuery.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
				Where("assessments.created_by = ?", *teacherID)
		}
	}
	if err := previousQuery.Count(&previousCount).Error; err != nil {
		return 0, fmt.Errorf("failed to get previous count: %w", err)
	}

	if previousCount == 0 {
		if currentCount > 0 {
			return 100, nil
		}
		return 0, nil
	}

	change := float64(currentCount-previousCount) / float64(previousCount) * 100
	return change, nil
}

// ===== ACTIVITY TRENDS =====

// GetActivityTrends retrieves activity trend data for the dashboard
// Optimized: uses single aggregate query instead of N+1 loop queries (21-36 queries → 1 query)
func (r *dashboardRepository) GetActivityTrends(ctx context.Context, tx *gorm.DB, teacherID *string, period string) ([]repositories.ActivityTrendData, error) {
	db := r.getDB(tx)

	var startDate time.Time
	var dateTrunc string
	var expectedPeriods int

	now := time.Now()

	switch period {
	case "week":
		// Last 7 days including today
		startDate = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -6)
		dateTrunc = "day"
		expectedPeriods = 7
	case "month":
		// Current week + 3 complete weeks before
		// First, find Monday of current week
		weekday := now.Weekday()
		daysFromMonday := int(weekday) - 1
		if weekday == time.Sunday {
			daysFromMonday = 6
		}
		currentWeekMonday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -daysFromMonday)
		// Go back 3 weeks from current week's Monday
		startDate = currentWeekMonday.AddDate(0, 0, -21) // 3 weeks = 21 days
		dateTrunc = "week"
		expectedPeriods = 4
	case "year":
		// Current month + 11 complete months before
		currentMonthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
		startDate = currentMonthStart.AddDate(0, -11, 0) // 11 months back
		dateTrunc = "month"
		expectedPeriods = 12
	default:
		return nil, fmt.Errorf("unsupported period: %s", period)
	}

	// Single aggregate query with DATE_TRUNC
	var rawResults []struct {
		PeriodDate   time.Time `gorm:"column:period_date"`
		Attempts     int64     `gorm:"column:attempts"`
		Users        int64     `gorm:"column:users"`
		AverageScore float64   `gorm:"column:avg_score"`
	}

	query := db.WithContext(ctx).
		Table("assessment_attempts").
		Select(`
			DATE_TRUNC(?, assessment_attempts.created_at) as period_date,
			COUNT(*) as attempts,
			COUNT(DISTINCT assessment_attempts.student_id) as users,
			COALESCE(AVG(CASE WHEN assessment_attempts.status = ? THEN assessment_attempts.score END), 0) as avg_score
		`, dateTrunc, models.AttemptCompleted).
		Where("assessment_attempts.created_at >= ?", startDate).
		Where("assessment_attempts.deleted_at IS NULL")

	// Apply teacher filter if provided
	if teacherID != nil && *teacherID != "" {
		query = query.Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
			Where("assessments.created_by = ?", *teacherID)
	}

	if err := query.Group("period_date").
		Order("period_date ASC").
		Scan(&rawResults).Error; err != nil {
		return nil, fmt.Errorf("failed to get activity trends: %w", err)
	}

	// Build result map for quick lookup
	resultMap := make(map[string]struct {
		Attempts     int64
		Users        int64
		AverageScore float64
	})
	for _, r := range rawResults {
		key := r.PeriodDate.Format("2006-01-02")
		resultMap[key] = struct {
			Attempts     int64
			Users        int64
			AverageScore float64
		}{r.Attempts, r.Users, r.AverageScore}
	}

	// Generate complete results with all expected periods (fill gaps with zeros)
	var results []repositories.ActivityTrendData

	switch period {
	case "week":
		for i := 0; i < expectedPeriods; i++ {
			date := startDate.AddDate(0, 0, i)
			key := date.Format("2006-01-02")
			data := resultMap[key]
			results = append(results, repositories.ActivityTrendData{
				Period:       date.Format("Mon"),
				Attempts:     data.Attempts,
				Users:        data.Users,
				AverageScore: data.AverageScore,
				Date:         date,
			})
		}
	case "month":
		// Each period is a week starting from startDate (which is a Monday)
		for i := 0; i < expectedPeriods; i++ {
			weekMonday := startDate.AddDate(0, 0, i*7)
			key := weekMonday.Format("2006-01-02")
			data := resultMap[key]

			// Label: show date range of the week (e.g., "06/12-12/12")
			weekEnd := weekMonday.AddDate(0, 0, 6)
			periodLabel := fmt.Sprintf("%02d/%02d-%02d/%02d",
				weekMonday.Day(), weekMonday.Month(),
				weekEnd.Day(), weekEnd.Month())

			results = append(results, repositories.ActivityTrendData{
				Period:       periodLabel,
				Attempts:     data.Attempts,
				Users:        data.Users,
				AverageScore: data.AverageScore,
				Date:         weekMonday,
			})
		}
	case "year":
		for i := 0; i < expectedPeriods; i++ {
			monthStart := startDate.AddDate(0, i, 0)
			key := monthStart.Format("2006-01-02")
			data := resultMap[key]

			// Label: actual month name (e.g., "Jan", "Feb", "Mar")
			periodLabel := monthStart.Format("Jan")

			results = append(results, repositories.ActivityTrendData{
				Period:       periodLabel,
				Attempts:     data.Attempts,
				Users:        data.Users,
				AverageScore: data.AverageScore,
				Date:         monthStart,
			})
		}
	}

	return results, nil
}

// ===== RECENT ACTIVITIES =====

func (r *dashboardRepository) GetRecentActivities(ctx context.Context, tx *gorm.DB, teacherID *string, limit int) ([]repositories.RecentActivityData, error) {
	db := r.getDB(tx)

	var activities []repositories.RecentActivityData

	// Get recent completed assessments
	var attempts []struct {
		ID              uint
		StudentID       string
		AssessmentID    uint
		AssessmentTitle string
		Score           float64
		CreatedAt       time.Time
		UserName        string
	}

	query := db.WithContext(ctx).
		Table("assessment_attempts").
		Select("assessment_attempts.id, assessment_attempts.student_id, assessment_attempts.assessment_id, "+
			"assessments.title as assessment_title, assessment_attempts.score, assessment_attempts.completed_at as created_at, "+
			"users.full_name as user_name").
		Joins("LEFT JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
		Joins("LEFT JOIN users ON assessment_attempts.student_id = users.id").
		Where("assessment_attempts.status = ? AND assessment_attempts.deleted_at IS NULL", models.AttemptCompleted)

	// Filter by teacher if provided
	if teacherID != nil && *teacherID != "" {
		query = query.Where("assessments.created_by = ?", *teacherID)
	}

	if err := query.Order("assessment_attempts.completed_at DESC").
		Limit(limit).
		Scan(&attempts).Error; err != nil {
		return nil, fmt.Errorf("failed to get recent activities: %w", err)
	}

	for _, attempt := range attempts {
		activities = append(activities, repositories.RecentActivityData{
			ID:              attempt.ID,
			UserID:          attempt.StudentID,
			UserName:        attempt.UserName,
			Action:          "completed_assessment",
			AssessmentID:    &attempt.AssessmentID,
			AssessmentTitle: &attempt.AssessmentTitle,
			Score:           &attempt.Score,
			CreatedAt:       attempt.CreatedAt,
		})
	}

	return activities, nil
}

// ===== QUESTION DISTRIBUTION =====

func (r *dashboardRepository) GetQuestionDistribution(ctx context.Context, tx *gorm.DB, teacherID *string) ([]repositories.QuestionDistributionData, error) {
	db := r.getDB(tx)

	var results []struct {
		Type  string
		Count int64
	}

	query := db.WithContext(ctx).Model(&models.Question{}).
		Select("type, COUNT(*) as count")

	// Filter by teacher if provided
	if teacherID != nil && *teacherID != "" {
		query = query.Where("created_by = ?", *teacherID)
	}

	if err := query.Group("type").Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get question distribution: %w", err)
	}

	// Calculate total for percentages
	var total int64
	for _, r := range results {
		total += r.Count
	}

	var distribution []repositories.QuestionDistributionData
	for _, r := range results {
		percentage := float64(0)
		if total > 0 {
			percentage = float64(r.Count) / float64(total) * 100
		}

		distribution = append(distribution, repositories.QuestionDistributionData{
			Type:       r.Type,
			Count:      r.Count,
			Percentage: percentage,
		})
	}

	return distribution, nil
}

// ===== PERFORMANCE BY CATEGORY =====

func (r *dashboardRepository) GetPerformanceByCategory(ctx context.Context, tx *gorm.DB, teacherID *string, limit int) ([]repositories.CategoryPerformanceData, error) {
	db := r.getDB(tx)

	var results []struct {
		CategoryID    *uint
		CategoryName  string
		AverageScore  float64
		TotalAttempts int64
	}

	// Query to get performance by category through questions in assessments
	query := db.WithContext(ctx).
		Table("assessment_attempts").
		Select("question_categories.id as category_id, "+
			"COALESCE(question_categories.name, 'Uncategorized') as category_name, "+
			"AVG(assessment_attempts.score) as average_score, "+
			"COUNT(DISTINCT assessment_attempts.id) as total_attempts").
		Joins("JOIN assessments ON assessment_attempts.assessment_id = assessments.id AND assessments.deleted_at IS NULL").
		Joins("JOIN assessment_questions ON assessments.id = assessment_questions.assessment_id AND assessment_questions.deleted_at IS NULL").
		Joins("JOIN questions ON assessment_questions.question_id = questions.id AND questions.deleted_at IS NULL").
		Joins("LEFT JOIN question_categories ON questions.category_id = question_categories.id").
		Where("assessment_attempts.status = ? AND assessment_attempts.deleted_at IS NULL", models.AttemptCompleted)

	// Filter by teacher if provided
	if teacherID != nil && *teacherID != "" {
		query = query.Where("assessments.created_by = ?", *teacherID)
	}

	if err := query.Group("question_categories.id, question_categories.name").
		Order("total_attempts DESC").
		Limit(limit).
		Scan(&results).Error; err != nil {
		return nil, fmt.Errorf("failed to get performance by category: %w", err)
	}

	var performance []repositories.CategoryPerformanceData
	for _, r := range results {
		performance = append(performance, repositories.CategoryPerformanceData{
			CategoryID:    r.CategoryID,
			CategoryName:  r.CategoryName,
			AverageScore:  r.AverageScore,
			TotalAttempts: r.TotalAttempts,
		})
	}

	return performance, nil
}
