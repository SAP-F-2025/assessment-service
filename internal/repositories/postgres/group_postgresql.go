package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

type GroupPostgreSQL struct {
	db *gorm.DB
}

func NewGroupPostgreSQL(db *gorm.DB) repositories.GroupRepository {
	return &GroupPostgreSQL{
		db: db,
	}
}

// getDB returns the transaction DB if provided, otherwise returns the default DB
func (g *GroupPostgreSQL) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return g.db
}

// ===== BASIC CRUD OPERATIONS =====

// Create creates a new group
func (g *GroupPostgreSQL) Create(ctx context.Context, tx *gorm.DB, group *models.Group) error {
	db := g.getDB(tx)
	if err := db.WithContext(ctx).Create(group).Error; err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}
	return nil
}

// GetByID retrieves a group by ID
func (g *GroupPostgreSQL) GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.Group, error) {
	db := g.getDB(tx)
	var group models.Group

	if err := db.WithContext(ctx).First(&group, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("group not found with ID %d", id)
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	return &group, nil
}

// GetByIDWithMembers retrieves a group with its members
func (g *GroupPostgreSQL) GetByIDWithMembers(ctx context.Context, tx *gorm.DB, id uint) (*models.Group, error) {
	db := g.getDB(tx)
	var group models.Group

	if err := db.WithContext(ctx).
		Preload("Members").
		First(&group, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("group not found with ID %d", id)
		}
		return nil, fmt.Errorf("failed to get group with members: %w", err)
	}

	return &group, nil
}

// GetByName retrieves a group by name
func (g *GroupPostgreSQL) GetByName(ctx context.Context, tx *gorm.DB, name string) (*models.Group, error) {
	db := g.getDB(tx)
	var group models.Group

	if err := db.WithContext(ctx).Where("name = ?", name).First(&group).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("group not found with name %s", name)
		}
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	return &group, nil
}

// Update updates a group
func (g *GroupPostgreSQL) Update(ctx context.Context, tx *gorm.DB, group *models.Group) error {
	db := g.getDB(tx)
	if err := db.WithContext(ctx).Save(group).Error; err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}
	return nil
}

// Delete soft deletes a group
func (g *GroupPostgreSQL) Delete(ctx context.Context, tx *gorm.DB, id uint) error {
	db := g.getDB(tx)
	if err := db.WithContext(ctx).Delete(&models.Group{}, id).Error; err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}
	return nil
}

// ===== LIST AND SEARCH OPERATIONS =====

// List retrieves a paginated list of groups with optional filters
func (g *GroupPostgreSQL) List(ctx context.Context, tx *gorm.DB, filters repositories.GroupFilters) ([]*models.Group, int64, error) {
	db := g.getDB(tx)

	// Set defaults
	if filters.Limit <= 0 {
		filters.Limit = 10
	}
	if filters.Limit > 100 {
		filters.Limit = 100
	}

	query := db.WithContext(ctx).Model(&models.Group{})

	// Apply filters
	if filters.Query != "" {
		searchPattern := "%" + filters.Query + "%"
		query = query.Where("name ILIKE ? OR display_name ILIKE ?", searchPattern, searchPattern)
	}

	if filters.Type != "" {
		query = query.Where("type = ?", filters.Type)
	}

	if filters.CreatedBy != "" {
		query = query.Where("created_by = ?", filters.CreatedBy)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to count groups: %w", err)
	}

	// Get paginated results
	var groups []*models.Group
	if err := query.
		Order("created_at DESC").
		Limit(filters.Limit).
		Offset(filters.Offset).
		Find(&groups).Error; err != nil {
		return nil, 0, fmt.Errorf("failed to list groups: %w", err)
	}

	return groups, total, nil
}

// GetByCreator retrieves all groups created by a specific user
func (g *GroupPostgreSQL) GetByCreator(ctx context.Context, tx *gorm.DB, creatorID string) ([]*models.Group, error) {
	db := g.getDB(tx)
	var groups []*models.Group

	if err := db.WithContext(ctx).
		Where("created_by = ?", creatorID).
		Order("created_at DESC").
		Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("failed to get groups by creator: %w", err)
	}

	return groups, nil
}

// ===== GROUP MEMBERSHIP OPERATIONS =====

// AddMember adds a user to a group
func (g *GroupPostgreSQL) AddMember(ctx context.Context, tx *gorm.DB, member *models.GroupMember) error {
	db := g.getDB(tx)

	// Check if member already exists
	exists, err := g.IsMember(ctx, tx, member.GroupID, member.UserID)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("user %s is already a member of group %d", member.UserID, member.GroupID)
	}

	if err := db.WithContext(ctx).Create(member).Error; err != nil {
		return fmt.Errorf("failed to add member to group: %w", err)
	}

	return nil
}

// UpdateMember updates an existing group member's information (e.g., role)
func (g *GroupPostgreSQL) UpdateMember(ctx context.Context, tx *gorm.DB, member *models.GroupMember) error {
	db := g.getDB(tx)

	// Check if member exists
	exists, err := g.IsMember(ctx, tx, member.GroupID, member.UserID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("user %s is not a member of group %d", member.UserID, member.GroupID)
	}

	// Update member - only update role field, preserve joined_at and other metadata
	result := db.WithContext(ctx).
		Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id = ?", member.GroupID, member.UserID).
		Update("role", member.Role)

	if result.Error != nil {
		return fmt.Errorf("failed to update member: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no member found to update for user %s in group %d", member.UserID, member.GroupID)
	}

	return nil
}

// RemoveMember removes a user from a group
func (g *GroupPostgreSQL) RemoveMember(ctx context.Context, tx *gorm.DB, groupID uint, userID string) error {
	db := g.getDB(tx)

	result := db.WithContext(ctx).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Delete(&models.GroupMember{})

	if result.Error != nil {
		return fmt.Errorf("failed to remove member from group: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("member not found in group")
	}

	return nil
}

// GetMembers retrieves all members of a group
func (g *GroupPostgreSQL) GetMembers(ctx context.Context, tx *gorm.DB, groupID uint) ([]*models.GroupMember, error) {
	db := g.getDB(tx)
	var members []*models.GroupMember

	if err := db.WithContext(ctx).
		Where("group_id = ?", groupID).
		Order("joined_at ASC").
		Find(&members).Error; err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}

	return members, nil
}

// GetMembersByUserID retrieves all group memberships for a user
func (g *GroupPostgreSQL) GetMembersByUserID(ctx context.Context, tx *gorm.DB, userID string) ([]*models.GroupMember, error) {
	db := g.getDB(tx)
	var members []*models.GroupMember

	if err := db.WithContext(ctx).
		Preload("Group").
		Where("user_id = ?", userID).
		Order("joined_at DESC").
		Find(&members).Error; err != nil {
		return nil, fmt.Errorf("failed to get user group memberships: %w", err)
	}

	return members, nil
}

// IsMember checks if a user is a member of a group
func (g *GroupPostgreSQL) IsMember(ctx context.Context, tx *gorm.DB, groupID uint, userID string) (bool, error) {
	db := g.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.GroupMember{}).
		Where("group_id = ? AND user_id = ?", groupID, userID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check group membership: %w", err)
	}

	return count > 0, nil
}

// ===== VALIDATION =====

// ExistsByID checks if a group exists by ID
func (g *GroupPostgreSQL) ExistsByID(ctx context.Context, tx *gorm.DB, id uint) (bool, error) {
	db := g.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.Group{}).
		Where("id = ?", id).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check group existence: %w", err)
	}

	return count > 0, nil
}

// ExistsByName checks if a group exists by name
func (g *GroupPostgreSQL) ExistsByName(ctx context.Context, tx *gorm.DB, name string) (bool, error) {
	db := g.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.Group{}).
		Where("name = ?", name).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check group name existence: %w", err)
	}

	return count > 0, nil
}
