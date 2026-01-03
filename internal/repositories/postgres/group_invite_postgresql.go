package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

type GroupInvitePostgreSQL struct {
	db *gorm.DB
}

func NewGroupInvitePostgreSQL(db *gorm.DB) repositories.GroupInviteRepository {
	return &GroupInvitePostgreSQL{
		db: db,
	}
}

// getDB returns the transaction DB if provided, otherwise returns the default DB
func (gi *GroupInvitePostgreSQL) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return gi.db
}

// ===== BASIC CRUD OPERATIONS =====

// Create creates a new group invite
func (gi *GroupInvitePostgreSQL) Create(ctx context.Context, tx *gorm.DB, invite *models.GroupInvite) error {
	db := gi.getDB(tx)
	if err := db.WithContext(ctx).Create(invite).Error; err != nil {
		return fmt.Errorf("failed to create group invite: %w", err)
	}
	return nil
}

// GetByID retrieves a group invite by ID
func (gi *GroupInvitePostgreSQL) GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.GroupInvite, error) {
	db := gi.getDB(tx)
	var invite models.GroupInvite

	if err := db.WithContext(ctx).Preload("Group").First(&invite, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("group invite not found with ID %d", id)
		}
		return nil, fmt.Errorf("failed to get group invite: %w", err)
	}

	return &invite, nil
}

// Update updates a group invite
func (gi *GroupInvitePostgreSQL) Update(ctx context.Context, tx *gorm.DB, invite *models.GroupInvite) error {
	db := gi.getDB(tx)
	if err := db.WithContext(ctx).Save(invite).Error; err != nil {
		return fmt.Errorf("failed to update group invite: %w", err)
	}
	return nil
}

// Delete soft deletes a group invite
func (gi *GroupInvitePostgreSQL) Delete(ctx context.Context, tx *gorm.DB, id uint) error {
	db := gi.getDB(tx)
	if err := db.WithContext(ctx).Delete(&models.GroupInvite{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete group invite: %w", err)
	}
	return nil
}

// ===== LOOKUP BY IDENTIFIERS =====

// GetByToken retrieves a group invite by token
func (gi *GroupInvitePostgreSQL) GetByToken(ctx context.Context, tx *gorm.DB, token string) (*models.GroupInvite, error) {
	db := gi.getDB(tx)
	var invite models.GroupInvite

	if err := db.WithContext(ctx).
		Preload("Group").
		Where("token = ?", token).
		First(&invite).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("group invite not found with token")
		}
		return nil, fmt.Errorf("failed to get group invite by token: %w", err)
	}

	return &invite, nil
}

// GetByCode retrieves a group invite by code
func (gi *GroupInvitePostgreSQL) GetByCode(ctx context.Context, tx *gorm.DB, code string) (*models.GroupInvite, error) {
	db := gi.getDB(tx)
	var invite models.GroupInvite

	if err := db.WithContext(ctx).
		Preload("Group").
		Where("code = ?", code).
		First(&invite).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("group invite not found with code")
		}
		return nil, fmt.Errorf("failed to get group invite by code: %w", err)
	}

	return &invite, nil
}

// ===== LIST OPERATIONS =====

// GetByGroupID retrieves all invites for a group
func (gi *GroupInvitePostgreSQL) GetByGroupID(ctx context.Context, tx *gorm.DB, groupID uint) ([]*models.GroupInvite, error) {
	db := gi.getDB(tx)
	var invites []*models.GroupInvite

	if err := db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("created_at DESC").
		Find(&invites).Error; err != nil {
		return nil, fmt.Errorf("failed to get group invites: %w", err)
	}

	return invites, nil
}

// List retrieves a paginated list of group invites with filters
func (gi *GroupInvitePostgreSQL) List(ctx context.Context, tx *gorm.DB, filters repositories.GroupInviteFilters) ([]*models.GroupInvite, int64, error) {
	db := gi.getDB(tx)

	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 10
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}

	query := db.WithContext(ctx).Model(&models.GroupInvite{})

	// Apply filters
	if filters.GroupID > 0 {
		query = query.Where("group_id = ?", filters.GroupID)
	}

	if filters.Type != "" {
		query = query.Where("type = ?", filters.Type)
	}

	if filters.CreatedBy != "" {
		query = query.Where("created_by = ?", filters.CreatedBy)
	}

	if filters.ActiveOnly {
		now := time.Now()
		query = query.Where("(expires_at IS NULL OR expires_at > ?)", now)
		query = query.Where("(max_uses IS NULL OR uses_count < max_uses)")
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count group invites: %w", err)
	}

	// Get paginated results
	var invites []*models.GroupInvite
	if err := query.
		Preload("Group").
		Order("created_at DESC").
		Limit(filters.Limit).
		Offset(filters.Offset).
		Find(&invites).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list group invites: %w", err)
	}

	return invites, total, nil
}

// ===== USAGE TRACKING =====

// IncrementUsesCount increments the uses_count for an invite
func (gi *GroupInvitePostgreSQL) IncrementUsesCount(ctx context.Context, tx *gorm.DB, id uint) error {
	db := gi.getDB(tx)

	result := db.WithContext(ctx).
		Model(&models.GroupInvite{}).
		Where("id = ?", id).
		Update("uses_count", gorm.Expr("uses_count + 1"))

	if result.Error != nil {
		return fmt.Errorf("failed to increment invite uses count: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("group invite not found with ID %d", id)
	}

	return nil
}

// ===== VALIDATION =====

// ExistsByToken checks if an invite exists by token
func (gi *GroupInvitePostgreSQL) ExistsByToken(ctx context.Context, tx *gorm.DB, token string) (bool, error) {
	db := gi.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.GroupInvite{}).
		Where("token = ?", token).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check invite token existence: %w", err)
	}

	return count > 0, nil
}

// ExistsByCode checks if an invite exists by code
func (gi *GroupInvitePostgreSQL) ExistsByCode(ctx context.Context, tx *gorm.DB, code string) (bool, error) {
	db := gi.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.GroupInvite{}).
		Where("code = ?", code).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check invite code existence: %w", err)
	}

	return count > 0, nil
}
