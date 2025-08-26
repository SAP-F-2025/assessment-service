package casdoor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/casdoor/casdoor-go-sdk/casdoorsdk"
	"github.com/redis/go-redis/v9"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
)

// CasdoorConfig holds the configuration for Casdoor connection
type CasdoorConfig struct {
	Endpoint         string
	ClientID         string
	ClientSecret     string
	Certificate      string
	OrganizationName string
	ApplicationName  string
}

type UserCasdoor struct {
	client *casdoorsdk.Client
	redis  *redis.Client
	config CasdoorConfig

	// Cache settings
	cachePrefix string
	cacheTTL    time.Duration
}

func NewUserCasdoor(config CasdoorConfig, redisClient *redis.Client) repositories.UserRepository {
	// Initialize Casdoor client
	client := casdoorsdk.NewClient(
		config.Endpoint,
		config.ClientID,
		config.ClientSecret,
		config.Certificate,
		config.OrganizationName,
		config.ApplicationName,
	)

	return &UserCasdoor{
		client:      client,
		redis:       redisClient,
		config:      config,
		cachePrefix: "user:",
		cacheTTL:    15 * time.Minute, // Cache for 15 minutes
	}
}

// ===== CACHE METHODS =====

// getCacheKey generates cache key for user data
func (u *UserCasdoor) getCacheKey(key string) string {
	return fmt.Sprintf("%s%s", u.cachePrefix, key)
}

// getUserFromCache retrieves user from cache
func (u *UserCasdoor) getUserFromCache(ctx context.Context, key string) (*models.User, error) {
	if u.redis == nil {
		return nil, nil // Cache not available
	}

	cacheKey := u.getCacheKey(key)
	data, err := u.redis.Get(ctx, cacheKey).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil // Not found in cache
		}
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	var user models.User
	if err := json.Unmarshal([]byte(data), &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached user: %w", err)
	}

	return &user, nil
}

// setUserCache stores user in cache
func (u *UserCasdoor) setUserCache(ctx context.Context, key string, user *models.User) error {
	if u.redis == nil {
		return nil // Cache not available
	}

	data, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user for cache: %w", err)
	}

	cacheKey := u.getCacheKey(key)
	return u.redis.Set(ctx, cacheKey, data, u.cacheTTL).Err()
}

// invalidateUserCache removes user from cache
func (u *UserCasdoor) invalidateUserCache(ctx context.Context, keys ...string) error {
	if u.redis == nil {
		return nil
	}

	cacheKeys := make([]string, len(keys))
	for i, key := range keys {
		cacheKeys[i] = u.getCacheKey(key)
	}

	return u.redis.Del(ctx, cacheKeys...).Err()
}

// ===== CONVERSION METHODS =====

// convertCasdoorUserToModel converts Casdoor user to internal model
func (u *UserCasdoor) convertCasdoorUserToModel(casdoorUser *casdoorsdk.User) *models.User {
	if casdoorUser == nil {
		return nil
	}

	// Parse ID from string to uint
	id, _ := strconv.ParseUint(casdoorUser.Id, 10, 32)

	// Determine role from Casdoor user type or custom field
	role := u.mapCasdoorRoleToUserRole(casdoorUser.Type)

	// Parse timestamps
	var createdAt, updatedAt time.Time
	if casdoorUser.CreatedTime != "" {
		createdAt, _ = time.Parse(time.RFC3339, casdoorUser.CreatedTime)
	}
	if casdoorUser.UpdatedTime != "" {
		updatedAt, _ = time.Parse(time.RFC3339, casdoorUser.UpdatedTime)
	}

	// Parse last login
	var lastLoginAt *time.Time
	if casdoorUser.LastSigninTime != "" {
		if parsed, err := time.Parse(time.RFC3339, casdoorUser.LastSigninTime); err == nil {
			lastLoginAt = &parsed
		}
	}

	// Convert preferences from map to JSON
	var preferences []byte
	if len(casdoorUser.Properties) > 0 {
		preferences, _ = json.Marshal(casdoorUser.Properties)
	}

	return &models.User{
		ID:            uint(id),
		FullName:      casdoorUser.DisplayName,
		Email:         casdoorUser.Email,
		Role:          role,
		AvatarURL:     &casdoorUser.Avatar,
		PhoneNumber:   &casdoorUser.Phone,
		Organization:  &casdoorUser.Affiliation,
		Department:    nil, // Map from properties if available
		Timezone:      u.getPropertyOrDefault(casdoorUser.Properties, "timezone", "UTC"),
		Language:      u.getPropertyOrDefault(casdoorUser.Properties, "language", "en"),
		Preferences:   preferences,
		IsActive:      !casdoorUser.IsForbidden,
		EmailVerified: casdoorUser.EmailVerified,
		LastLoginAt:   lastLoginAt,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
}

// mapCasdoorRoleToUserRole maps Casdoor user type to internal role
func (u *UserCasdoor) mapCasdoorRoleToUserRole(casdoorType string) models.UserRole {
	switch strings.ToLower(casdoorType) {
	case "student":
		return models.RoleStudent
	case "teacher", "instructor":
		return models.RoleTeacher
	case "proctor":
		return models.RoleProctor
	case "admin", "administrator":
		return models.RoleAdmin
	default:
		return models.RoleStudent // Default role
	}
}

// getPropertyOrDefault gets property value or returns default
func (u *UserCasdoor) getPropertyOrDefault(properties map[string]string, key, defaultValue string) string {
	if value, exists := properties[key]; exists {
		return value
	}
	return defaultValue
}

// ===== BASIC READ OPERATIONS =====

// GetByID retrieves a user by ID
func (u *UserCasdoor) GetByID(ctx context.Context, id uint) (*models.User, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("id:%d", id)
	if cachedUser, err := u.getUserFromCache(ctx, cacheKey); err == nil && cachedUser != nil {
		return cachedUser, nil
	}

	// Get from Casdoor
	casdoorUser, err := u.client.GetUserByUserId(strconv.FormatUint(uint64(id), 10))
	if err != nil {
		return nil, fmt.Errorf("failed to get user from Casdoor: %w", err)
	}

	if casdoorUser == nil {
		return nil, fmt.Errorf("user not found with ID %d", id)
	}

	user := u.convertCasdoorUserToModel(casdoorUser)
	if user == nil {
		return nil, fmt.Errorf("failed to convert Casdoor user")
	}

	// Cache the result
	u.setUserCache(ctx, cacheKey, user)
	u.setUserCache(ctx, fmt.Sprintf("email:%s", user.Email), user)

	return user, nil
}

// GetByEmail retrieves a user by email
func (u *UserCasdoor) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	// Try cache first
	cacheKey := fmt.Sprintf("email:%s", email)
	if cachedUser, err := u.getUserFromCache(ctx, cacheKey); err == nil && cachedUser != nil {
		return cachedUser, nil
	}

	// Get from Casdoor by email
	casdoorUser, err := u.client.GetUserByEmail(email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email from Casdoor: %w", err)
	}

	if casdoorUser == nil {
		return nil, fmt.Errorf("user not found with email %s", email)
	}

	user := u.convertCasdoorUserToModel(casdoorUser)
	if user == nil {
		return nil, fmt.Errorf("failed to convert Casdoor user")
	}

	// Cache the result
	u.setUserCache(ctx, cacheKey, user)
	u.setUserCache(ctx, fmt.Sprintf("id:%d", user.ID), user)

	return user, nil
}

// GetByIDs retrieves multiple users by their IDs
func (u *UserCasdoor) GetByIDs(ctx context.Context, ids []uint) ([]*models.User, error) {
	if len(ids) == 0 {
		return []*models.User{}, nil
	}

	users := make([]*models.User, 0, len(ids))
	uncachedIDs := make([]uint, 0)

	// Check cache first
	for _, id := range ids {
		cacheKey := fmt.Sprintf("id:%d", id)
		if cachedUser, err := u.getUserFromCache(ctx, cacheKey); err == nil && cachedUser != nil {
			users = append(users, cachedUser)
		} else {
			uncachedIDs = append(uncachedIDs, id)
		}
	}

	// Fetch uncached users from Casdoor
	for _, id := range uncachedIDs {
		user, err := u.GetByID(ctx, id)
		if err == nil && user != nil {
			users = append(users, user)
		}
		// Continue even if individual user fetch fails
	}

	return users, nil
}

// ===== VALIDATION AND CHECKS =====

// ExistsByID checks if a user exists by ID
func (u *UserCasdoor) ExistsByID(ctx context.Context, id uint) (bool, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("exists:id:%d", id)
	if u.redis != nil {
		exists, err := u.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			return exists == "true", nil
		}
	}

	// Check with Casdoor
	user, err := u.client.GetUser(strconv.FormatUint(uint64(id), 10))
	if err != nil {
		return false, fmt.Errorf("failed to check user existence: %w", err)
	}

	exists := user != nil

	// Cache the result for a shorter time
	if u.redis != nil {
		u.redis.Set(ctx, cacheKey, fmt.Sprintf("%t", exists), 5*time.Minute)
	}

	return exists, nil
}

// ExistsByEmail checks if a user exists by email
func (u *UserCasdoor) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("exists:email:%s", email)
	if u.redis != nil {
		exists, err := u.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			return exists == "true", nil
		}
	}

	// Check with Casdoor
	user, err := u.client.GetUserByEmail(email)
	if err != nil {
		return false, fmt.Errorf("failed to check user existence by email: %w", err)
	}

	exists := user != nil

	// Cache the result
	if u.redis != nil {
		u.redis.Set(ctx, cacheKey, fmt.Sprintf("%t", exists), 5*time.Minute)
	}

	return exists, nil
}

// IsActive checks if a user is active
func (u *UserCasdoor) IsActive(ctx context.Context, id uint) (bool, error) {
	user, err := u.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	return user.IsActive, nil
}

// HasRole checks if a user has a specific role
func (u *UserCasdoor) HasRole(ctx context.Context, id uint, role models.UserRole) (bool, error) {
	user, err := u.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	return user.Role == role, nil
}
