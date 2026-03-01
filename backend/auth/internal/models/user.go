package models

import (
	"time"

	"gorm.io/datatypes"
)

// User represents a user synced from Clerk with local extensions
type User struct {
	ID           string         `gorm:"primaryKey" json:"id"` // Clerk user ID
	Email        string         `gorm:"uniqueIndex;not null" json:"email"`
	FirstName    string         `json:"first_name"`
	LastName     string         `json:"last_name"`
	AvatarURL    string         `json:"avatar_url"`
	DepartmentID *string        `json:"department_id"`                 // Local extension
	RoleID       *string        `json:"role_id"`                       // FK to Role
	JobTitle     string         `json:"job_title"`                     // Employee's job title
	Preferences  datatypes.JSON `gorm:"type:jsonb" json:"preferences"` // Local extension
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	LastSignInAt *time.Time     `json:"last_sign_in_at"`

	// Relationships
	Role        *Role                    `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	Department  *Department              `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
	Memberships []OrganizationMembership `gorm:"foreignKey:UserID" json:"memberships,omitempty"`
}

// TableName specifies the table name for GORM
func (User) TableName() string {
	return "users"
}

// FullName returns the user's full name
func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName
}
