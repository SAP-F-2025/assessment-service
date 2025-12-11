package models

import (
	"time"

	"gorm.io/gorm"
)

// InviteType represents the type of invite
type InviteType string

const (
	// InviteTypeLink is for URL-based invites
	InviteTypeLink InviteType = "link"
	// InviteTypeCode is for short code-based invites
	InviteTypeCode InviteType = "code"
)

// DefaultInviteExpirationDays is the default number of days before an invite expires
const DefaultInviteExpirationDays = 7

// GroupInvite represents an invitation to join a group
type GroupInvite struct {
	ID      uint `json:"id" gorm:"primaryKey"`
	GroupID uint `json:"group_id" gorm:"not null;index"`

	// Invite identifiers
	Token string  `json:"token" gorm:"uniqueIndex;not null;size:64"` // UUID-based token for links
	Code  *string `json:"code,omitempty" gorm:"uniqueIndex;size:8"`  // Short alphanumeric code

	// Settings
	Type        InviteType      `json:"type" gorm:"size:20;default:'link'"` // 'link' or 'code'
	MaxUses     *int            `json:"max_uses,omitempty"`                 // NULL = unlimited
	UsesCount   int             `json:"uses_count" gorm:"default:0"`
	ExpiresAt   *time.Time      `json:"expires_at,omitempty"` // NULL = never expires
	DefaultRole GroupMemberRole `json:"default_role" gorm:"size:50;default:'member'"`

	// Metadata
	CreatedBy string         `json:"created_by" gorm:"not null;size:255"`
	CreatedAt time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`

	// Relations
	Group *Group `json:"group,omitempty" gorm:"foreignKey:GroupID"`
}

// TableName returns the table name for GroupInvite
func (GroupInvite) TableName() string {
	return "group_invites"
}

// IsExpired checks if the invite has expired
func (gi *GroupInvite) IsExpired() bool {
	if gi.ExpiresAt == nil {
		return false
	}
	return time.Now().After(*gi.ExpiresAt)
}

// IsExhausted checks if the invite has reached its maximum uses
func (gi *GroupInvite) IsExhausted() bool {
	if gi.MaxUses == nil {
		return false // Unlimited uses
	}
	return gi.UsesCount >= *gi.MaxUses
}

// CanUse checks if the invite can still be used
func (gi *GroupInvite) CanUse() bool {
	return !gi.IsExpired() && !gi.IsExhausted()
}

// RemainingUses returns the number of remaining uses, or -1 if unlimited
func (gi *GroupInvite) RemainingUses() int {
	if gi.MaxUses == nil {
		return -1 // Unlimited
	}
	remaining := *gi.MaxUses - gi.UsesCount
	if remaining < 0 {
		return 0
	}
	return remaining
}
