package service

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
)

var ErrDuplicateDepartment = errors.New("duplicate department")
var ErrDuplicateRole = errors.New("duplicate role")
var ErrCannotRemoveAdmin = errors.New("cannot remove admin member")
var ErrCannotRemoveSelf = errors.New("cannot remove your own account")

var ClerkDeleteMembershipFunc = func(clerkSecretKey, organizationID, userID string) error {
	endpoint := fmt.Sprintf("%s/organizations/%s/memberships/%s", clerkBaseURL, url.PathEscape(organizationID), url.PathEscape(userID))
	req, err := http.NewRequest(http.MethodDelete, endpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create Clerk delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+clerkSecretKey)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("clerk user delete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Clerk membership delete failed for org %s user %s: status=%d body=%s", organizationID, userID, resp.StatusCode, strings.TrimSpace(string(body)))
		return fmt.Errorf("clerk user delete failed status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// EmployeeService handles department, employee, and invitation operations.
type EmployeeService struct {
	db             *gorm.DB
	clerkSecretKey string
}

// NewEmployeeService creates a new EmployeeService.
func NewEmployeeService(db *gorm.DB, clerkSecretKey string) *EmployeeService {
	return &EmployeeService{db: db, clerkSecretKey: clerkSecretKey}
}

func (s *EmployeeService) CreateDepartment(orgID, name, description, createdBy string) (*models.Department, error) {
	trimmedName := normalizeName(name)
	if trimmedName == "" {
		return nil, fmt.Errorf("department name is required")
	}
	normalizedNameKey := normalizeNameKey(trimmedName)

	var existing models.Department
	err := s.db.Where("organization_id = ? AND lower(trim(name)) = ?", orgID, normalizedNameKey).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("%w: department %q already exists in this organization", ErrDuplicateDepartment, trimmedName)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check existing department: %w", err)
	}

	dept := models.Department{
		OrganizationID: orgID,
		Name:           trimmedName,
		Description:    description,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if createdBy != "" {
		dept.CreatedByUserID = &createdBy
	}
	if err := s.db.Create(&dept).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return nil, fmt.Errorf("%w: department %q already exists in this organization", ErrDuplicateDepartment, trimmedName)
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
		Where("organization_id = ?", orgID).
		Preload("Department").
		Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list employees: %w", err)
	}
	return users, nil
}

type MemberProfile struct {
	ID            string             `json:"id"`
	Email         string             `json:"email"`
	FirstName     string             `json:"first_name"`
	LastName      string             `json:"last_name"`
	JobTitle      string             `json:"job_title"`
	IsAdmin       bool               `json:"is_admin"`
	IsActive      bool               `json:"is_active"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	LastSignInAt  *time.Time         `json:"last_sign_in_at,omitempty"`
	Department    *models.Department `json:"department,omitempty"`
	WorkflowRoles []string           `json:"workflow_roles"`
}

func (s *EmployeeService) GetMemberProfile(orgID, userID string) (*MemberProfile, error) {
	var user models.User
	err := s.db.
		Where("id = ? AND organization_id = ?", userID, orgID).
		Preload("Department").
		First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("%w: user %s in org %s", ErrNotFound, userID, orgID)
		}
		return nil, fmt.Errorf("failed to load member profile: %w", err)
	}

	var roleRows []struct {
		Name string `gorm:"column:name"`
	}
	if err := s.db.Table("user_role_memberships AS urm").
		Select("r.name AS name").
		Joins("JOIN roles AS r ON r.id = urm.role_id AND r.organization_id = urm.organization_id").
		Where("urm.organization_id = ? AND urm.user_id = ?", orgID, userID).
		Order("LOWER(TRIM(r.name)) ASC").
		Scan(&roleRows).Error; err != nil {
		return nil, fmt.Errorf("failed to load workflow roles for member: %w", err)
	}

	workflowRoles := make([]string, 0, len(roleRows))
	for _, row := range roleRows {
		if strings.TrimSpace(row.Name) == "" {
			continue
		}
		workflowRoles = append(workflowRoles, row.Name)
	}

	profile := &MemberProfile{
		ID:            user.ID,
		Email:         user.Email,
		FirstName:     user.FirstName,
		LastName:      user.LastName,
		JobTitle:      user.JobTitle,
		IsAdmin:       user.IsAdmin,
		IsActive:      user.IsActive,
		CreatedAt:     user.CreatedAt,
		UpdatedAt:     user.UpdatedAt,
		LastSignInAt:  user.LastSignInAt,
		Department:    user.Department,
		WorkflowRoles: workflowRoles,
	}

	return profile, nil
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
	var invitation models.EmployeeInvitation
	if err := s.db.Where("id = ? AND organization_id = ? AND status = ?", invitationID, orgID, "pending").First(&invitation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: invitation %s not found or already processed", ErrNotFound, invitationID)
		}
		return fmt.Errorf("failed to load invitation for revoke: %w", err)
	}

	if err := ClerkRevokeOrgInvitationsByEmailFunc(s.clerkSecretKey, orgID, invitation.Email); err != nil {
		return fmt.Errorf("failed to revoke Clerk invite link: %w", err)
	}

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

// RemoveEmployee permanently removes a non-admin member from the auth service DB,
// including role memberships, org membership linkage, and invitations tied to the user.
func (s *EmployeeService) RemoveEmployee(orgID, employeeID, actorUserID string) error {
	if strings.TrimSpace(employeeID) == "" {
		return fmt.Errorf("employee id is required")
	}
	if actorUserID != "" && actorUserID == employeeID {
		return ErrCannotRemoveSelf
	}

	var user models.User
	if err := s.db.Where("id = ? AND organization_id = ?", employeeID, orgID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: employee %s in org %s", ErrNotFound, employeeID, orgID)
		}
		return fmt.Errorf("failed to load employee: %w", err)
	}

	if user.IsAdmin {
		return ErrCannotRemoveAdmin
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("organization_id = ? AND user_id = ?", orgID, employeeID).
			Delete(&models.UserRoleMembership{}).Error; err != nil {
			return fmt.Errorf("failed to remove user role memberships: %w", err)
		}

		if err := tx.Where("organization_id = ? AND accepted_user_id = ?", orgID, employeeID).
			Delete(&models.EmployeeInvitation{}).Error; err != nil {
			return fmt.Errorf("failed to remove accepted invitations: %w", err)
		}
		if user.Email != "" {
			if err := tx.Where("organization_id = ? AND email = ?", orgID, user.Email).
				Delete(&models.EmployeeInvitation{}).Error; err != nil {
				return fmt.Errorf("failed to remove invitations by email: %w", err)
			}
		}

		result := tx.Where("id = ? AND organization_id = ?", employeeID, orgID).Delete(&models.User{})
		if result.Error != nil {
			return fmt.Errorf("failed to remove user record: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("%w: employee %s in org %s", ErrNotFound, employeeID, orgID)
		}

		return nil
	}); err != nil {
		return err
	}

	if err := s.deleteFromClerk(orgID, user.ID); err != nil {
		log.Printf("Warning: employee removed from database but failed to delete from Clerk org=%s user=%s: %v", orgID, user.ID, err)
		// TODO: enqueue async retry/reconciliation for Clerk membership deletion.
	}

	log.Printf("Employee removed: user=%s org=%s", employeeID, orgID)
	return nil
}

func (s *EmployeeService) deleteFromClerk(orgID, userID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("invalid clerk user id")
	}
	if strings.TrimSpace(orgID) == "" {
		return fmt.Errorf("invalid clerk organization id")
	}
	if strings.TrimSpace(s.clerkSecretKey) == "" {
		return fmt.Errorf("clerk secret key not configured for strict user deletion")
	}
	return ClerkDeleteMembershipFunc(s.clerkSecretKey, orgID, userID)
}
