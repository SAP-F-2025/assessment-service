package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"gorm.io/gorm"
)

// AssessmentRepository interface for assessment-specific operations
type AssessmentRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, tx *gorm.DB, assessment *models.Assessment) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.Assessment, error)
	GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.Assessment, error) // Include questions, settings
	Update(ctx context.Context, tx *gorm.DB, assessment *models.Assessment) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error // Soft delete

	// Query operations
	List(ctx context.Context, tx *gorm.DB, filters AssessmentFilters) ([]*models.Assessment, int64, error)
	GetByCreator(ctx context.Context, tx *gorm.DB, creatorID string, filters AssessmentFilters) ([]*models.Assessment, int64, error)
	GetByStatus(ctx context.Context, tx *gorm.DB, status models.AssessmentStatus, limit, offset int) ([]*models.Assessment, error)
	Search(ctx context.Context, tx *gorm.DB, query string, filters AssessmentFilters) ([]*models.Assessment, int64, error)

	// Status management
	UpdateStatus(ctx context.Context, tx *gorm.DB, id uint, status models.AssessmentStatus) error
	GetExpiredAssessments(ctx context.Context, tx *gorm.DB) ([]*models.Assessment, error)
	BulkUpdateStatus(ctx context.Context, tx *gorm.DB, ids []uint, status models.AssessmentStatus) error

	// Permission checks
	IsOwner(ctx context.Context, tx *gorm.DB, assessmentID uint, userID string) (bool, error)
	CanAccess(ctx context.Context, tx *gorm.DB, assessmentID uint, userID string, role models.UserRole) (bool, error)

	// Statistics and analytics
	GetAssessmentStats(ctx context.Context, tx *gorm.DB, id uint) (*AssessmentStats, error)
	GetCreatorStats(ctx context.Context, tx *gorm.DB, creatorID string) (*CreatorStats, error)
	GetPopularAssessments(ctx context.Context, tx *gorm.DB, limit int) ([]*models.Assessment, error)

	// Validation helpers
	ExistsByTitle(ctx context.Context, tx *gorm.DB, title string, creatorID string, excludeID *uint) (bool, error)
	HasAttempts(ctx context.Context, tx *gorm.DB, id uint) (bool, error)
	HasActiveAttempts(ctx context.Context, tx *gorm.DB, id uint) (bool, error)

	// Settings management
	UpdateSettings(ctx context.Context, tx *gorm.DB, assessmentID uint, settings *models.AssessmentSettings) error
	GetSettings(ctx context.Context, tx *gorm.DB, assessmentID uint) (*models.AssessmentSettings, error)

	UpdateDuration(ctx context.Context, tx *gorm.DB, assessmentID uint, duration int) error
	UpdateMaxAttempts(ctx context.Context, tx *gorm.DB, assessmentID uint, maxAttempts int) error
}

// AssessmentSettingsRepository interface for assessment settings operations
type AssessmentSettingsRepository interface {
	Create(ctx context.Context, tx *gorm.DB, settings *models.AssessmentSettings) error
	GetByAssessmentID(ctx context.Context, tx *gorm.DB, assessmentID uint) (*models.AssessmentSettings, error)
	Update(ctx context.Context, tx *gorm.DB, settings *models.AssessmentSettings) error
	Delete(ctx context.Context, tx *gorm.DB, assessmentID uint) error

	// Bulk operations
	CreateDefault(ctx context.Context, tx *gorm.DB, assessmentID uint) error
	GetMultiple(ctx context.Context, tx *gorm.DB, assessmentIDs []uint) (map[uint]*models.AssessmentSettings, error)
}
