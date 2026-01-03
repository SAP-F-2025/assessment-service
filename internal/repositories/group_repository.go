package repositories

import (
	"context"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"gorm.io/gorm"
)

// GroupFilters defines filters for listing groups
type GroupFilters struct {
	Query        string // Search query for name or display name
	Type         string // Filter by group type
	CreatedBy    string // Filter by creator
	MemberUserID string // Filter groups where user is a member (for access control)
	Limit        int    // Results per page
	Offset       int    // Offset for pagination
}

type GroupRepository interface {
	// Basic CRUD operations
	Create(ctx context.Context, tx *gorm.DB, group *models.Group) error
	GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.Group, error)
	GetByIDWithMembers(ctx context.Context, tx *gorm.DB, id uint) (*models.Group, error)
	GetByName(ctx context.Context, tx *gorm.DB, name string) (*models.Group, error)
	Update(ctx context.Context, tx *gorm.DB, group *models.Group) error
	Delete(ctx context.Context, tx *gorm.DB, id uint) error

	// List and search operations
	List(ctx context.Context, tx *gorm.DB, filters GroupFilters) ([]*models.Group, int64, error)
	GetByCreator(ctx context.Context, tx *gorm.DB, creatorID string) ([]*models.Group, error)

	// Group membership operations
	AddMember(ctx context.Context, tx *gorm.DB, member *models.GroupMember) error
	UpdateMember(ctx context.Context, tx *gorm.DB, member *models.GroupMember) error
	RemoveMember(ctx context.Context, tx *gorm.DB, groupID uint, userID string) error
	GetMembers(ctx context.Context, tx *gorm.DB, groupID uint) ([]*models.GroupMember, error)
	GetMembersByUserID(ctx context.Context, tx *gorm.DB, userID string) ([]*models.GroupMember, error)
	IsMember(ctx context.Context, tx *gorm.DB, groupID uint, userID string) (bool, error)

	// Validation
	ExistsByID(ctx context.Context, tx *gorm.DB, id uint) (bool, error)
	ExistsByName(ctx context.Context, tx *gorm.DB, name string) (bool, error)
}
