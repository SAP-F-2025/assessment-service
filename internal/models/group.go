package models

import (
	"time"

	"gorm.io/gorm"
)

// Group represents a group in the system
type Group struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	Name        string         `json:"name" gorm:"uniqueIndex;not null;size:100"`
	DisplayName string         `json:"display_name" gorm:"not null;size:255"`
	Description string         `json:"description" gorm:"type:text"`
	Type        string         `json:"type" gorm:"size:50;default:'standard'"`
	CreatedBy   string         `json:"created_by" gorm:"not null;size:255"`
	CreatedAt   time.Time      `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt   time.Time      `json:"updated_at" gorm:"autoUpdateTime"`
	DeletedAt   gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`

	// Relations
	Members             []GroupMember     `json:"members,omitempty" gorm:"foreignKey:GroupID;constraint:OnDelete:CASCADE"`
	AssignedAssessments []AssessmentGroup `json:"assigned_assessments,omitempty" gorm:"foreignKey:GroupID"`
}

// GroupMemberRole represents the role of a member in a group (class)
type GroupMemberRole string

const (
	GroupMemberRoleTeacher GroupMemberRole = "teacher" // Teacher in the class
	GroupMemberRoleStudent GroupMemberRole = "student" // Student in the class
)

// GroupMember represents a user's membership in a group (class)
// Teacher who created the group is the owner (not stored as member)
// Other teachers and students are members with their respective roles
type GroupMember struct {
	ID        uint            `json:"id" gorm:"primaryKey"`
	GroupID   uint            `json:"group_id" gorm:"not null;index"`
	UserID    string          `json:"user_id" gorm:"not null;size:255;index"`
	Role      GroupMemberRole `json:"role" gorm:"size:50;default:'student'"` // teacher, student
	JoinedAt  time.Time       `json:"joined_at" gorm:"autoCreateTime"`
	DeletedAt gorm.DeletedAt  `json:"deleted_at,omitempty" gorm:"index"`

	// Relations
	Group *Group `json:"group,omitempty" gorm:"foreignKey:GroupID"`
}

func (Group) TableName() string {
	return "groups"
}

func (GroupMember) TableName() string {
	return "group_members"
}
