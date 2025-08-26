package repositories

import (
	"context"
	"time"

	"github.com/SAP-F-2025/assessment-service/internal/models"
)

// UserRepository interface for user operations (minimal for assessment service)
type UserRepository interface {
	// Basic read operations (assessment service is not owner of user data)
	GetByID(ctx context.Context, id uint) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByIDs(ctx context.Context, ids []uint) ([]*models.User, error)

	// Role-based queries
	GetByRole(ctx context.Context, role models.UserRole, limit, offset int) ([]*models.User, error)
	GetTeachers(ctx context.Context, limit, offset int) ([]*models.User, error)
	GetStudents(ctx context.Context, limit, offset int) ([]*models.User, error)

	// Validation and checks
	ExistsByID(ctx context.Context, id uint) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	IsActive(ctx context.Context, id uint) (bool, error)
	HasRole(ctx context.Context, id uint, role models.UserRole) (bool, error)

	// Search operations
	Search(ctx context.Context, query string, role *models.UserRole, limit, offset int) ([]*models.User, error)
	GetByOrganization(ctx context.Context, organization string, limit, offset int) ([]*models.User, error)

	// Activity tracking (read-only)
	UpdateLastLogin(ctx context.Context, id uint, loginTime time.Time) error
	GetActiveUsers(ctx context.Context, since time.Time) ([]*models.User, error)

	// Statistics (for assessment service analytics)
	GetUserStats(ctx context.Context, id uint) (*UserStats, error)
	GetRoleDistribution(ctx context.Context) (map[models.UserRole]int, error)
}

// ===== USER STATISTICS STRUCT =====

type UserStats struct {
	UserID      uint            `json:"user_id"`
	Role        models.UserRole `json:"role"`
	IsActive    bool            `json:"is_active"`
	LastLoginAt *time.Time      `json:"last_login_at"`

	// Teacher-specific stats
	AssessmentsCreated *int `json:"assessments_created,omitempty"`
	QuestionsCreated   *int `json:"questions_created,omitempty"`
	TotalAttempts      *int `json:"total_attempts,omitempty"`

	// Student-specific stats
	AssessmentsTaken  *int     `json:"assessments_taken,omitempty"`
	AttemptsCompleted *int     `json:"attempts_completed,omitempty"`
	AverageScore      *float64 `json:"average_score,omitempty"`
	TotalTimeSpent    *int     `json:"total_time_spent,omitempty"`

	// Common stats
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
