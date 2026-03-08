package models

import (
	"time"
)

// Organization represents an organization synced from Clerk
type Organization struct {
	ID        string    `gorm:"primaryKey" json:"id"` // Clerk organization ID
	Name      string    `gorm:"not null" json:"name"`
	Slug      string    `gorm:"uniqueIndex" json:"slug"` // URL friendly shortform for the org
	ImageURL  string    `json:"image_url"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relationships
	Settings *OrganizationSettings `gorm:"foreignKey:OrganizationID" json:"settings,omitempty"`
}

// TableName specifies the table name for GORM
func (Organization) TableName() string {
	return "organizations"
}
