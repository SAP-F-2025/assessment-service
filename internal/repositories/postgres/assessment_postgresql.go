package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

type AssessmentPostgreSQL struct {
	db      *gorm.DB
	helpers *SharedHelpers
}

func NewAssessmentPostgreSQL(db *gorm.DB) repositories.AssessmentRepository {
	return &AssessmentPostgreSQL{
		db:      db,
		helpers: NewSharedHelpers(db),
	}
}

// Create creates a new assessment with default settings
func (a *AssessmentPostgreSQL) Create(ctx context.Context, assessment *models.Assessment) error {
	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Check title uniqueness for creator
		exists, err := a.ExistsByTitle(ctx, assessment.Title, assessment.CreatedBy, nil)
		if err != nil {
			return fmt.Errorf("failed to check title uniqueness: %w", err)
		}
		if exists {
			return fmt.Errorf("assessment with title '%s' already exists for this creator", assessment.Title)
		}

		// Create the assessment
		assessment.Status = models.StatusDraft
		assessment.Version = 1
		if err := tx.Create(assessment).Error; err != nil {
			return fmt.Errorf("failed to create assessment: %w", err)
		}

		// Create default settings
		settings := &models.AssessmentSettings{
			AssessmentID:        assessment.ID,
			RandomizeQuestions:  false,
			RandomizeOptions:    false,
			QuestionsPerPage:    1,
			ShowProgressBar:     true,
			ShowResults:         true,
			ShowCorrectAnswers:  true,
			ShowScoreBreakdown:  true,
			AllowRetake:         false,
			RetakeDelay:         0,
			TimeLimitEnforced:   true,
			AutoSubmitOnTimeout: true,
		}

		if err := tx.Create(settings).Error; err != nil {
			return fmt.Errorf("failed to create assessment settings: %w", err)
		}

		return nil
	})
}

// GetByID retrieves an assessment by ID
func (a *AssessmentPostgreSQL) GetByID(ctx context.Context, id uint) (*models.Assessment, error) {
	var assessment models.Assessment
	err := a.db.WithContext(ctx).
		Preload("Creator").
		First(&assessment, id).Error

	if err != nil {
		return nil, err
	}

	return &assessment, nil
}

// GetByIDWithDetails retrieves an assessment with full details (questions, settings)
func (a *AssessmentPostgreSQL) GetByIDWithDetails(ctx context.Context, id uint) (*models.Assessment, error) {
	var assessment models.Assessment
	err := a.db.WithContext(ctx).
		Preload("Creator").
		Preload("Settings").
		Preload("Questions", func(db *gorm.DB) *gorm.DB {
			return db.Order("order ASC")
		}).
		Preload("Questions.Question").
		First(&assessment, id).Error

	if err != nil {
		return nil, err
	}

	// Calculate computed fields
	a.calculateComputedFields(&assessment)

	return &assessment, nil
}

// Update updates an assessment
func (a *AssessmentPostgreSQL) Update(ctx context.Context, assessment *models.Assessment) error {
	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get current assessment for validation
		var currentAssessment models.Assessment
		if err := tx.First(&currentAssessment, assessment.ID).Error; err != nil {
			return fmt.Errorf("assessment not found: %w", err)
		}

		// Check title uniqueness if title changed
		if assessment.Title != currentAssessment.Title {
			exists, err := a.ExistsByTitle(ctx, assessment.Title, assessment.CreatedBy, &assessment.ID)
			if err != nil {
				return fmt.Errorf("failed to check title uniqueness: %w", err)
			}
			if exists {
				return fmt.Errorf("assessment with title '%s' already exists for this creator", assessment.Title)
			}
		}

		// Validate business rules for active assessments
		if currentAssessment.Status == models.StatusActive {
			// Check if assessment has attempts
			hasAttempts, err := a.HasAttempts(ctx, assessment.ID)
			if err != nil {
				return fmt.Errorf("failed to check attempts: %w", err)
			}

			if hasAttempts {
				// Restrict modifications for assessments with attempts
				if assessment.Duration != currentAssessment.Duration {
					return fmt.Errorf("cannot change duration for active assessment with attempts")
				}
				if assessment.MaxAttempts < currentAssessment.MaxAttempts {
					return fmt.Errorf("cannot decrease max attempts for assessment with existing attempts")
				}
			}
		}

		// Increment version
		assessment.Version = currentAssessment.Version + 1
		assessment.UpdatedAt = time.Now()

		// Update assessment
		if err := tx.Save(assessment).Error; err != nil {
			return fmt.Errorf("failed to update assessment: %w", err)
		}

		return nil
	})
}

// Delete soft deletes an assessment
func (a *AssessmentPostgreSQL) Delete(ctx context.Context, id uint) error {
	// Check if assessment has attempts before deleting
	hasAttempts, err := a.HasAttempts(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check attempts: %w", err)
	}
	if hasAttempts {
		return fmt.Errorf("cannot delete assessment with existing attempts")
	}

	return a.db.WithContext(ctx).Delete(&models.Assessment{}, id).Error
}

// List retrieves assessments with filters and pagination
func (a *AssessmentPostgreSQL) List(ctx context.Context, filters repositories.AssessmentFilters) ([]*models.Assessment, int64, error) {
	query := a.db.WithContext(ctx).Model(&models.Assessment{})

	// Apply filters
	query = a.applyFilters(query, filters)

	// Count total
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	query = a.applyPaginationAndSort(query, filters)

	// Execute query
	var assessments []*models.Assessment
	err := query.Preload("Creator").Find(&assessments).Error
	if err != nil {
		return nil, 0, err
	}

	// Calculate computed fields for each assessment
	for _, assessment := range assessments {
		a.calculateComputedFields(assessment)
	}

	return assessments, total, nil
}

// GetByCreator retrieves assessments created by a specific user
func (a *AssessmentPostgreSQL) GetByCreator(ctx context.Context, creatorID uint, filters repositories.AssessmentFilters) ([]*models.Assessment, int64, error) {
	filters.CreatedBy = &creatorID
	return a.List(ctx, filters)
}

// GetByStatus retrieves assessments by status with pagination
func (a *AssessmentPostgreSQL) GetByStatus(ctx context.Context, status models.AssessmentStatus, limit, offset int) ([]*models.Assessment, error) {
	var assessments []*models.Assessment
	err := a.db.WithContext(ctx).
		Where("status = ?", status).
		Preload("Creator").
		Limit(limit).
		Offset(offset).
		Order("created_at DESC").
		Find(&assessments).Error

	if err != nil {
		return nil, err
	}

	return assessments, nil
}

// Search performs full-text search on assessments
func (a *AssessmentPostgreSQL) Search(ctx context.Context, query string, filters repositories.AssessmentFilters) ([]*models.Assessment, int64, error) {
	db := a.db.WithContext(ctx).Model(&models.Assessment{})

	// Full-text search
	if query != "" {
		searchQuery := fmt.Sprintf("%%%s%%", query)
		db = db.Where("title ILIKE ? OR description ILIKE ?", searchQuery, searchQuery)
	}

	// Apply other filters
	db = a.applyFilters(db, filters)

	// Count total
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination and ordering
	db = a.applyPaginationAndSort(db, filters)

	// Execute query
	var assessments []*models.Assessment
	err := db.Preload("Creator").Find(&assessments).Error
	if err != nil {
		return nil, 0, err
	}

	return assessments, total, nil
}

// UpdateStatus updates the status of an assessment
func (a *AssessmentPostgreSQL) UpdateStatus(ctx context.Context, id uint, status models.AssessmentStatus) error {
	return a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     status,
			"updated_at": time.Now(),
		}).Error
}

// GetExpiredAssessments retrieves assessments that have passed their due date
func (a *AssessmentPostgreSQL) GetExpiredAssessments(ctx context.Context) ([]*models.Assessment, error) {
	var assessments []*models.Assessment
	err := a.db.WithContext(ctx).
		Where("status = ? AND due_date IS NOT NULL AND due_date < ?", models.StatusActive, time.Now()).
		Preload("Creator").
		Find(&assessments).Error

	return assessments, err
}

// AutoExpireAssessments automatically expires assessments past due date
func (a *AssessmentPostgreSQL) AutoExpireAssessments(ctx context.Context) (int, error) {
	result := a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("status = ? AND due_date IS NOT NULL AND due_date < ?", models.StatusActive, time.Now()).
		Updates(map[string]interface{}{
			"status":     models.StatusExpired,
			"updated_at": time.Now(),
		})

	return int(result.RowsAffected), result.Error
}

// GetAssessmentsNearExpiry gets assessments expiring within specified duration
func (a *AssessmentPostgreSQL) GetAssessmentsNearExpiry(ctx context.Context, withinDuration time.Duration) ([]*models.Assessment, error) {
	var assessments []*models.Assessment
	expiryTime := time.Now().Add(withinDuration)

	err := a.db.WithContext(ctx).
		Where("status = ? AND due_date IS NOT NULL AND due_date BETWEEN ? AND ?",
			models.StatusActive, time.Now(), expiryTime).
		Preload("Creator").
		Find(&assessments).Error

	return assessments, err
}

// BulkUpdateStatus updates the status of multiple assessments
func (a *AssessmentPostgreSQL) BulkUpdateStatus(ctx context.Context, ids []uint, status models.AssessmentStatus) error {
	return a.helpers.BulkUpdateAssessmentStatus(ctx, ids, status)
}

// IsOwner checks if a user is the owner of an assessment
func (a *AssessmentPostgreSQL) IsOwner(ctx context.Context, assessmentID, userID uint) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("id = ? AND created_by = ?", assessmentID, userID).
		Count(&count).Error

	return count > 0, err
}

// CanAccess checks if a user can access an assessment based on role
func (a *AssessmentPostgreSQL) CanAccess(ctx context.Context, assessmentID, userID uint, role models.UserRole) (bool, error) {
	// Admins can access everything
	if role == models.RoleAdmin {
		return true, nil
	}

	// Teachers can access their own assessments
	if role == models.RoleTeacher {
		return a.IsOwner(ctx, assessmentID, userID)
	}

	// Students can only access active assessments they're enrolled in
	if role == models.RoleStudent {
		// Check if assessment is active
		var assessment models.Assessment
		err := a.db.WithContext(ctx).
			Select("status").
			First(&assessment, assessmentID).Error
		if err != nil {
			return false, err
		}

		return assessment.Status == models.StatusActive, nil
	}

	return false, nil
}

// GetAssessmentStats retrieves statistics for an assessment
func (a *AssessmentPostgreSQL) GetAssessmentStats(ctx context.Context, id uint) (*repositories.AssessmentStats, error) {
	stats := &repositories.AssessmentStats{}

	// Use helper for total attempts
	totalAttempts, err := a.helpers.CountAttempts(ctx, id)
	if err != nil {
		return nil, err
	}

	// Use helper for completed attempts
	completedAttempts, err := a.helpers.CountAttemptsByStatus(ctx, id, models.AttemptCompleted)
	if err != nil {
		return nil, err
	}

	// Get assessment passing score
	assessment, err := a.helpers.GetAssessmentBasicInfo(ctx, id)
	if err != nil {
		return nil, err
	}

	// Aggregate stats in fewer queries
	var avgScore, avgTimeSpent float64
	var passedAttempts int64
	if completedAttempts > 0 {
		a.db.WithContext(ctx).
			Model(&models.AssessmentAttempt{}).
			Select("AVG(score), AVG(time_spent), SUM(CASE WHEN score >= ? THEN 1 ELSE 0 END)", assessment.PassingScore).
			Where("assessment_id = ? AND status = ?", id, models.AttemptCompleted).
			Row().
			Scan(&avgScore, &avgTimeSpent, &passedAttempts)
	}

	passRate := float64(0)
	if completedAttempts > 0 {
		passRate = float64(passedAttempts) / float64(completedAttempts) * 100
	}

	// Get question stats in single query
	var questionCount, totalPoints int64
	a.db.WithContext(ctx).
		Model(&models.AssessmentQuestion{}).
		Select("COUNT(*), COALESCE(SUM(points), 0)").
		Where("assessment_id = ?", id).
		Row().
		Scan(&questionCount, &totalPoints)

	stats.TotalAttempts = int(totalAttempts)
	stats.CompletedAttempts = int(completedAttempts)
	stats.AverageScore = avgScore
	stats.PassRate = passRate
	stats.AverageTimeSpent = int(avgTimeSpent)
	stats.QuestionCount = int(questionCount)
	stats.TotalPoints = int(totalPoints)

	return stats, nil
}

// GetCreatorStats retrieves statistics for a creator
func (a *AssessmentPostgreSQL) GetCreatorStats(ctx context.Context, creatorID uint) (*repositories.CreatorStats, error) {
	stats := &repositories.CreatorStats{}

	// Total assessments
	var totalAssessments int64
	a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("created_by = ?", creatorID).
		Count(&totalAssessments)

	// Active assessments
	var activeAssessments int64
	a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("created_by = ? AND status = ?", creatorID, models.StatusActive).
		Count(&activeAssessments)

	// Draft assessments
	var draftAssessments int64
	a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("created_by = ? AND status = ?", creatorID, models.StatusDraft).
		Count(&draftAssessments)

	// Total questions (from assessments created by this user)
	var totalQuestions int64
	a.db.WithContext(ctx).
		Table("assessment_questions aq").
		Joins("JOIN assessments a ON aq.assessment_id = a.id").
		Where("a.created_by = ?", creatorID).
		Count(&totalQuestions)

	// Total attempts on creator's assessments
	var totalAttempts int64
	a.db.WithContext(ctx).
		Table("assessment_attempts att").
		Joins("JOIN assessments a ON att.assessment_id = a.id").
		Where("a.created_by = ?", creatorID).
		Count(&totalAttempts)

	stats.TotalAssessments = int(totalAssessments)
	stats.ActiveAssessments = int(activeAssessments)
	stats.DraftAssessments = int(draftAssessments)
	stats.TotalQuestions = int(totalQuestions)
	stats.TotalAttempts = int(totalAttempts)

	return stats, nil
}

// GetPopularAssessments retrieves the most attempted assessments
func (a *AssessmentPostgreSQL) GetPopularAssessments(ctx context.Context, limit int) ([]*models.Assessment, error) {
	var assessments []*models.Assessment

	err := a.db.WithContext(ctx).
		Table("assessments a").
		Select("a.*, COUNT(att.id) as attempt_count").
		Joins("LEFT JOIN assessment_attempts att ON a.id = att.assessment_id").
		Where("a.status = ?", models.StatusActive).
		Group("a.id").
		Order("attempt_count DESC").
		Limit(limit).
		Preload("Creator").
		Find(&assessments).Error

	return assessments, err
}

// ExistsByTitle checks if an assessment with the same title exists for a creator
func (a *AssessmentPostgreSQL) ExistsByTitle(ctx context.Context, title string, creatorID uint, excludeID *uint) (bool, error) {
	query := a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("title = ? AND created_by = ?", title, creatorID)

	if excludeID != nil {
		query = query.Where("id != ?", *excludeID)
	}

	var count int64
	err := query.Count(&count).Error
	return count > 0, err
}

// HasAttempts checks if an assessment has any attempts
func (a *AssessmentPostgreSQL) HasAttempts(ctx context.Context, id uint) (bool, error) {
	count, err := a.helpers.CountAttempts(ctx, id)
	return count > 0, err
}

// HasActiveAttempts checks if an assessment has any active/in-progress attempts
func (a *AssessmentPostgreSQL) HasActiveAttempts(ctx context.Context, id uint) (bool, error) {
	var count int64
	err := a.db.WithContext(ctx).
		Model(&models.AssessmentAttempt{}).
		Where("assessment_id = ? AND status IN ?", id, []models.AttemptStatus{
			models.AttemptInProgress,
			models.AttemptCompleted,
		}).
		Count(&count).Error

	return count > 0, err
}

// UpdateSettings updates assessment settings
func (a *AssessmentPostgreSQL) UpdateSettings(ctx context.Context, assessmentID uint, settings *models.AssessmentSettings) error {
	settings.AssessmentID = assessmentID
	return a.db.WithContext(ctx).
		Model(&models.AssessmentSettings{}).
		Where("assessment_id = ?", assessmentID).
		Updates(settings).Error
}

// GetSettings retrieves assessment settings
func (a *AssessmentPostgreSQL) GetSettings(ctx context.Context, assessmentID uint) (*models.AssessmentSettings, error) {
	var settings models.AssessmentSettings
	err := a.db.WithContext(ctx).
		Where("assessment_id = ?", assessmentID).
		First(&settings).Error

	if err != nil {
		return nil, err
	}

	return &settings, nil
}

// UpdateDuration updates assessment duration with business rules
func (a *AssessmentPostgreSQL) UpdateDuration(ctx context.Context, assessmentID uint, duration int) error {
	// Validate duration range (5-300 minutes as per docs)
	if duration < 5 || duration > 300 {
		return fmt.Errorf("duration must be between 5 and 300 minutes")
	}

	// Check if assessment can be modified
	var assessment models.Assessment
	err := a.db.WithContext(ctx).
		Select("status").
		First(&assessment, assessmentID).Error
	if err != nil {
		return err
	}

	// Only allow duration change for Draft assessments
	if assessment.Status != models.StatusDraft {
		return fmt.Errorf("can only modify duration for draft assessments")
	}

	return a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("id = ?", assessmentID).
		Update("duration", duration).Error
}

// UpdateMaxAttempts updates max attempts with business rules
func (a *AssessmentPostgreSQL) UpdateMaxAttempts(ctx context.Context, assessmentID uint, maxAttempts int) error {
	// Validate max attempts range (1-10 as per docs)
	if maxAttempts < 1 || maxAttempts > 10 {
		return fmt.Errorf("max attempts must be between 1 and 10")
	}

	// Get current max attempts
	var currentMaxAttempts int
	err := a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Select("max_attempts").
		Where("id = ?", assessmentID).
		Scan(&currentMaxAttempts).Error
	if err != nil {
		return err
	}

	// Check if assessment has attempts
	hasAttempts, err := a.HasAttempts(ctx, assessmentID)
	if err != nil {
		return err
	}

	// If has attempts, only allow increasing max attempts
	if hasAttempts && maxAttempts < currentMaxAttempts {
		return fmt.Errorf("cannot decrease max attempts when assessment has existing attempts")
	}

	return a.db.WithContext(ctx).
		Model(&models.Assessment{}).
		Where("id = ?", assessmentID).
		Update("max_attempts", maxAttempts).Error
}

// Helper methods

// applyFilters applies common filters to a query
func (a *AssessmentPostgreSQL) applyFilters(query *gorm.DB, filters repositories.AssessmentFilters) *gorm.DB {
	return a.helpers.ApplyAssessmentFilters(query, filters)
}

// applyPaginationAndSort applies pagination and sorting to a query
func (a *AssessmentPostgreSQL) applyPaginationAndSort(query *gorm.DB, filters repositories.AssessmentFilters) *gorm.DB {
	return a.helpers.ApplyPaginationAndSort(query, filters.SortBy, filters.SortOrder, filters.Limit, filters.Offset)
}

// calculateComputedFields calculates computed fields for an assessment
func (a *AssessmentPostgreSQL) calculateComputedFields(assessment *models.Assessment) {
	// Calculate questions count
	assessment.QuestionsCount = len(assessment.Questions)

	// Calculate total points
	totalPoints := 0
	for _, aq := range assessment.Questions {
		totalPoints += *aq.Points
	}
	assessment.TotalPoints = totalPoints

	// Calculate attempt count
	assessment.AttemptCount = len(assessment.Attempts)

	// Calculate average score
	if len(assessment.Attempts) > 0 {
		totalScore := 0.0
		completedCount := 0
		for _, attempt := range assessment.Attempts {
			if attempt.Status == models.AttemptCompleted {
				totalScore += attempt.Score
				completedCount++
			}
		}
		if completedCount > 0 {
			assessment.AvgScore = totalScore / float64(completedCount)
		}
	}
}
