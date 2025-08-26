package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// AssessmentRepository interface for assessment-specific operations
type AssessmentRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, assessment *models.Assessment) error
	GetByID(ctx context.Context, id uint) (*models.Assessment, error)
	GetByIDWithDetails(ctx context.Context, id uint) (*models.Assessment, error) // Include questions, settings
	Update(ctx context.Context, assessment *models.Assessment) error
	Delete(ctx context.Context, id uint) error // Soft delete

	// Query operations
	List(ctx context.Context, filters AssessmentFilters) ([]*models.Assessment, int64, error)
	GetByCreator(ctx context.Context, creatorID uint, filters AssessmentFilters) ([]*models.Assessment, int64, error)
	GetByStatus(ctx context.Context, status models.AssessmentStatus, limit, offset int) ([]*models.Assessment, error)
	Search(ctx context.Context, query string, filters AssessmentFilters) ([]*models.Assessment, int64, error)

	// Status management
	UpdateStatus(ctx context.Context, id uint, status models.AssessmentStatus) error
	GetExpiredAssessments(ctx context.Context) ([]*models.Assessment, error)
	BulkUpdateStatus(ctx context.Context, ids []uint, status models.AssessmentStatus) error

	// Permission checks
	IsOwner(ctx context.Context, assessmentID, userID uint) (bool, error)
	CanAccess(ctx context.Context, assessmentID, userID uint, role models.UserRole) (bool, error)

	// Statistics and analytics
	GetAssessmentStats(ctx context.Context, id uint) (*AssessmentStats, error)
	GetCreatorStats(ctx context.Context, creatorID uint) (*CreatorStats, error)
	GetPopularAssessments(ctx context.Context, limit int) ([]*models.Assessment, error)

	// Validation helpers
	ExistsByTitle(ctx context.Context, title string, creatorID uint, excludeID *uint) (bool, error)
	HasAttempts(ctx context.Context, id uint) (bool, error)
	HasActiveAttempts(ctx context.Context, id uint) (bool, error)

	// Settings management
	UpdateSettings(ctx context.Context, assessmentID uint, settings *models.AssessmentSettings) error
	GetSettings(ctx context.Context, assessmentID uint) (*models.AssessmentSettings, error)

	UpdateDuration(ctx context.Context, assessmentID uint, duration int) error
	UpdateMaxAttempts(ctx context.Context, assessmentID uint, maxAttempts int) error
}

// AssessmentSettingsRepository interface for assessment settings operations
type AssessmentSettingsRepository interface {
	Create(ctx context.Context, settings *models.AssessmentSettings) error
	GetByAssessmentID(ctx context.Context, assessmentID uint) (*models.AssessmentSettings, error)
	Update(ctx context.Context, settings *models.AssessmentSettings) error
	Delete(ctx context.Context, assessmentID uint) error

	// Bulk operations
	CreateDefault(ctx context.Context, assessmentID uint) error
	GetMultiple(ctx context.Context, assessmentIDs []uint) (map[uint]*models.AssessmentSettings, error)
}
