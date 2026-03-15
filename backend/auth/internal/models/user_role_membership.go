package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserRoleMembership stores membership of a user in a workflow role.
type UserRoleMembership struct {
	ID             string    `gorm:"primaryKey;type:uuid" json:"id"`
	OrganizationID string    `gorm:"not null;index;uniqueIndex:idx_org_user_role" json:"organization_id"`
	UserID         string    `gorm:"not null;index;uniqueIndex:idx_org_user_role" json:"user_id"`
	RoleID         string    `gorm:"not null;index;uniqueIndex:idx_org_user_role" json:"role_id"`
	AssignedBy     *string   `gorm:"index" json:"assigned_by,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	User User `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Role Role `gorm:"foreignKey:RoleID" json:"role,omitempty"`
}

func (UserRoleMembership) TableName() string {
	return "user_role_memberships"
}

func (m *UserRoleMembership) BeforeCreate(tx *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	return nil
}
