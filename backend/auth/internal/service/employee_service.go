package service

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
)

var ErrDuplicateDepartment = errors.New("duplicate department")

// EmployeeService handles department, employee, and invitation operations.
type EmployeeService struct {
	db             *gorm.DB
	clerkSecretKey string
}

// NewEmployeeService creates a new EmployeeService.
func NewEmployeeService(db *gorm.DB, clerkSecretKey string) *EmployeeService {
	return &EmployeeService{db: db, clerkSecretKey: clerkSecretKey}
}

func (s *EmployeeService) CreateDepartment(orgID, name, description string) (*models.Department, error) {
	var existing models.Department
	err := s.db.Where("name = ? AND organization_id = ?", name, orgID).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: department %q already exists in this organization", ErrDuplicateDepartment, name)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing department: %w", err)
	}

	dept := models.Department{
		OrganizationID: orgID,
		Name:           name,
		Description:    description,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if err := s.db.Create(&dept).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: department %q already exists in this organization", ErrDuplicateDepartment, name)
		}
		return nil, fmt.Errorf("failed to create department: %w", err)
	}
	log.Printf("Department created: %s in org %s", dept.ID, orgID)
	return &dept, nil
}

// ListDepartments returns all departments for an organization.
func (s *EmployeeService) ListDepartments(orgID string) ([]models.Department, error) {
	var departments []models.Department
	if err := s.db.Where("organization_id = ?", orgID).Find(&departments).Error; err != nil {
		return nil, fmt.Errorf("failed to list departments: %w", err)
	}
	return departments, nil
}

func (s *EmployeeService) ListEmployees(orgID string) ([]models.User, error) {
	var users []models.User
	err := s.db.
		Joins("JOIN organization_memberships ON organization_memberships.user_id = users.id").
		Where("organization_memberships.organization_id = ?", orgID).
		Preload("Department").
		Preload("Role").
		Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list employees: %w", err)
	}
	return users, nil
}

// ---------------------------------------------------------------------------
// Invitations
// ---------------------------------------------------------------------------

// ListInvitations returns all pending invitations for an organization.
func (s *EmployeeService) ListInvitations(orgID string) ([]models.EmployeeInvitation, error) {
	var invitations []models.EmployeeInvitation
	err := s.db.
		Where("organization_id = ? AND status = ?", orgID, "pending").
		Preload("Department").
		Find(&invitations).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list invitations: %w", err)
	}
	return invitations, nil
}

// RevokeInvitation marks a pending invitation as revoked.
func (s *EmployeeService) RevokeInvitation(invitationID, orgID string) error {
	result := s.db.Model(&models.EmployeeInvitation{}).
		Where("id = ? AND organization_id = ? AND status = ?", invitationID, orgID, "pending").
		Updates(map[string]interface{}{
			"status":     "revoked",
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return fmt.Errorf("failed to revoke invitation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%w: invitation %s not found or already processed", ErrNotFound, invitationID)
	}
	log.Printf("Invitation %s revoked", invitationID)
	return nil
}

func (s *EmployeeService) GetDepartmentDetails(orgID, deptID string) (*models.Department, error) {
	var dept models.Department
	err := s.db.Where("organization_id = ? AND id = ?", orgID, deptID).First(&dept).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: department %s in org %s", ErrNotFound, deptID, orgID)
		}
		return nil, fmt.Errorf("failed to get department details: %w", err)
	}
	return &dept, nil
}
