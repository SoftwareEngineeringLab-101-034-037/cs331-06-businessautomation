package models

import (
	"time"

	"gorm.io/datatypes"
)

// User represents a user synced from Clerk with local extensions
type User struct {
	ID             string         `gorm:"primaryKey" json:"id"` // Clerk user ID
	Email          string         `gorm:"uniqueIndex;not null" json:"email"`
	FirstName      string         `json:"first_name"`
	LastName       string         `json:"last_name"`
	AvatarURL      string         `json:"avatar_url"`
	OrganizationID *string        `gorm:"type:text" json:"organization_id"` // FK to organizations table
	DepartmentID   *string        `gorm:"type:text" json:"department_id"`
	JobTitle       string         `json:"job_title"`
	IsAdmin        bool           `gorm:"default:false" json:"is_admin"`    // Whether user is an admin of their org
	Preferences    datatypes.JSON `gorm:"type:jsonb" json:"preferences"`    // Local extension
	IsActive       bool           `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	LastSignInAt   *time.Time     `json:"last_sign_in_at"`

	// Relationships
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Department   *Department   `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
}

// TableName specifies the table name for GORM
func (User) TableName() string {
	return "users"
}

// FullName returns the user's full name
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}
