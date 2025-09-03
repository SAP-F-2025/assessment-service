package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type UserRole string
type Role = UserRole // Alias for compatibility

const (
	RoleStudent UserRole = "student"
	RoleTeacher UserRole = "teacher"
	RoleProctor UserRole = "proctor"
	RoleAdmin   UserRole = "admin"
)

type User struct {
	ID       uint     `json:"id" gorm:"primaryKey"`
	FullName string   `json:"full_name" gorm:"not null;size:100"`
	Email    string   `json:"email" gorm:"uniqueIndex;not null;size:255"`
	Role     UserRole `json:"role" gorm:"not null;index"`

	// Profile info
	AvatarURL    *string `json:"avatar_url" gorm:"size:500"`
	PhoneNumber  *string `json:"phone_number" gorm:"size:20"`
	Organization *string `json:"organization" gorm:"size:100"`
	Department   *string `json:"department" gorm:"size:100"`

	// Settings
	Timezone    string         `json:"timezone" gorm:"default:UTC;size:50"`
	Language    string         `json:"language" gorm:"default:en;size:10"`
	Preferences datatypes.JSON `json:"preferences" gorm:"type:jsonb"`

	// Status
	IsActive      bool       `json:"is_active" gorm:"default:true"`
	EmailVerified bool       `json:"email_verified" gorm:"default:false"`
	LastLoginAt   *time.Time `json:"last_login_at"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (User) TableName() string {
	return "users"
}
