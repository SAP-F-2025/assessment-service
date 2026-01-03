package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"gorm.io/gorm"
)

// GroupInviteFilters defines filters for listing group invites
type GroupInviteFilters struct {
	GroupID    uint   // Filter by group ID
	Type       string // Filter by invite type (link, code)
	ActiveOnly bool   // Only show non-expired, non-exhausted invites
	CreatedBy  string // Filter by creator
	Limit      int    // Results per page
	Offset     int    // Offset for pagination
}

// GroupInviteRepository defines the interface for group invite data access
type GroupInviteRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, tx *gorm.DB, invite *models.GroupInvite) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.GroupInvite, error)
	Update(ctx context.Context, tx *gorm.DB, invite *models.GroupInvite) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error

	// Lookup by identifiers
	GetByToken(ctx context.Context, tx *gorm.DB, token string) (*models.GroupInvite, error)
	GetByCode(ctx context.Context, tx *gorm.DB, code string) (*models.GroupInvite, error)

	// List operations
	GetByGroupID(ctx context.Context, tx *gorm.DB, groupID uint) ([]*models.GroupInvite, error)
	List(ctx context.Context, tx *gorm.DB, filters GroupInviteFilters) ([]*models.GroupInvite, int64, error)

	// Usage tracking
	IncrementUsesCount(ctx context.Context, tx *gorm.DB, id uint) error

	// Validation
	ExistsByToken(ctx context.Context, tx *gorm.DB, token string) (bool, error)
	ExistsByCode(ctx context.Context, tx *gorm.DB, code string) (bool, error)
}
