package models

import (
	"time"

	"gorm.io/datatypes"
)

// EmployeeInvitation represents a pending invitation for an employee to join an organization
type EmployeeInvitation struct {
	ID             string     `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	OrganizationID string     `gorm:"not null;index" json:"organization_id"`
	DepartmentID   string     `gorm:"not null;index" json:"department_id"`
	Email          string     `gorm:"not null;index" json:"email"`
	FirstName      string     `json:"first_name"`
	LastName       string     `json:"last_name"`
	RoleName       string     `json:"role_name"`                                // Local role name to assign on accept
	RoleNames      datatypes.JSON `gorm:"type:jsonb" json:"role_names,omitempty"`  // Optional workflow roles to tag the invite with
	JobTitle       string     `json:"job_title"`                                // Optional job title
	Token          string     `gorm:"uniqueIndex;not null" json:"-"`            // SHA-256 hash of invite token (never exposed in JSON)
	Status         string     `gorm:"not null;default:'pending'" json:"status"` // pending, accepted, expired, revoked
	InvitedBy      string     `gorm:"not null" json:"invited_by"`               // User ID of the admin who created the invitation
	ExpiresAt      time.Time  `gorm:"not null" json:"expires_at"`
	AcceptedAt     *time.Time `json:"accepted_at"`      // When the invitation was accepted (nullable)
	AcceptedUserID *string    `json:"accepted_user_id"` // Clerk user ID of the employee who accepted (nullable)
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Relationships
	Organization Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Department   Department   `gorm:"foreignKey:DepartmentID" json:"department,omitempty"`
}

func (EmployeeInvitation) TableName() string {
	return "employee_invitations"
}

func (e *EmployeeInvitation) IsExpired() bool {
	return time.Now().After(e.ExpiresAt)
}

func (e *EmployeeInvitation) IsPending() bool {
	return e.Status == "pending" && !e.IsExpired()
}

func (e *EmployeeInvitation) CanAccept() bool {
	return e.Status == "pending" && !e.IsExpired()
}
