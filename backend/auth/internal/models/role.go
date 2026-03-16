package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Role represents a role with permissions for local RBAC
type Role struct {
	ID              string         `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name            string         `gorm:"not null;uniqueIndex:idx_role_org" json:"name"`
	Description     string         `json:"description"`
	OrganizationID  string         `gorm:"not null;index;uniqueIndex:idx_role_org" json:"organization_id"` // Scoped to org
	CreatedByUserID *string        `gorm:"index" json:"created_by_user_id,omitempty"`
	Permissions     datatypes.JSON `gorm:"type:jsonb" json:"permissions"`
	IsSystemRole    bool           `gorm:"default:false" json:"is_system_role"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`

	CreatedBy *User `gorm:"foreignKey:CreatedByUserID" json:"created_by,omitempty"`
}

// TableName specifies the table name for GORM
func (Role) TableName() string {
	return "roles"
}

// BeforeCreate ensures Role.ID is always present even if DB default is missing.
func (r *Role) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}

// Permission represents a permission entry
type Permission struct {
	ID       string `gorm:"primaryKey" json:"id"`
	Name     string `gorm:"not null" json:"name"`
	Resource string `gorm:"not null" json:"resource"` // e.g., "workflow", "task"
	Action   string `gorm:"not null" json:"action"`   // e.g., "read", "write", "delete"
}

// TableName specifies the table name for GORM
func (Permission) TableName() string {
	return "permissions"
}

// HasPermission checks if role has a specific permission
func (r *Role) HasPermission(resource, action string) bool {
	// This is a simplified check - in production, parse the JSON properly
	// For now, return false as placeholder
	return false
}
