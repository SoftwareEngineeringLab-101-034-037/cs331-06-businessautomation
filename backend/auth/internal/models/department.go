package models

import "time"

type Department struct {
	ID             string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name           string    `gorm:"not null;uniqueIndex:idx_dept_org" json:"name"`
	OrganizationID string    `gorm:"not null;index;uniqueIndex:idx_dept_org" json:"organization_id"`
	Description    string    `json:"description"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (Department) TableName() string {
	return "departments"
}
