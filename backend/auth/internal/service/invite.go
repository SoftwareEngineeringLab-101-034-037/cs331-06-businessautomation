package service

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"gorm.io/gorm"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
)

const clerkBaseURL = "https://api.clerk.com/v1"

// InviteInput holds the parameters for creating a single employee invitation.
type InviteInput struct {
	OrgID        string
	Email        string
	FirstName    string
	LastName     string
	DepartmentID string // department name or ID — resolved to department
	Role         string
	JobTitle     string
	InvitedBy    string
}

// InviteResult is the outcome of a successful invitation.
type InviteResult struct {
	Invitation models.EmployeeInvitation `json:"invitation"`
}

// InviteAndNotify creates an employee invitation in our DB and sends the
// invitation email via Clerk's organization invitation API.
func (s *EmployeeService) InviteAndNotify(input InviteInput) (*InviteResult, error) {
	// Check for an existing pending invitation with the same email + org
	var existing models.EmployeeInvitation
	err := s.db.Where(
		"email = ? AND organization_id = ? AND status = ?",
		input.Email, input.OrgID, "pending",
	).First(&existing).Error
	if err == nil {
		return nil, fmt.Errorf("a pending invitation already exists for %s", input.Email)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("database lookup failed for %s in org %s: %w", input.Email, input.OrgID, err)
	}

	// Resolve department — look up by name within the org, or use as ID directly
	deptID, err := s.resolveDepartmentID(input.OrgID, input.DepartmentID)
	if err != nil {
		return nil, fmt.Errorf("department lookup failed: %w", err)
	}

	// Generate a secure invite token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate invite token: %w", err)
	}
	tokenHash := sha256.Sum256(tokenBytes)
	hashedToken := hex.EncodeToString(tokenHash[:])

	invitation := models.EmployeeInvitation{
		OrganizationID: input.OrgID,
		DepartmentID:   deptID,
		Email:          input.Email,
		FirstName:      input.FirstName,
		LastName:       input.LastName,
		RoleName:       input.Role,
		JobTitle:       input.JobTitle,
		Token:          hashedToken,
		Status:         "pending",
		InvitedBy:      input.InvitedBy,
		ExpiresAt:      time.Now().Add(7 * 24 * time.Hour), // 7-day expiry
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := s.db.Create(&invitation).Error; err != nil {
		return nil, fmt.Errorf("failed to create invitation: %w", err)
	}

	// Send the invitation email via Clerk's organization invitation API
	if err := s.sendClerkOrgInvitation(input.OrgID, input.Email); err != nil {
		log.Printf("Warning: Clerk invitation email failed for %s: %v (local invitation still created)", input.Email, err)
	} else {
		log.Printf("Clerk invitation email sent to %s for org %s", input.Email, input.OrgID)
	}

	return &InviteResult{Invitation: invitation}, nil
}

// sendClerkOrgInvitation calls Clerk's API to create an organization invitation,
// which triggers Clerk to send the invitation email automatically.
func (s *EmployeeService) sendClerkOrgInvitation(orgID, email string) error {
	if s.clerkSecretKey == "" {
		return fmt.Errorf("clerk secret key not configured, skipping email")
	}

	url := fmt.Sprintf("%s/organizations/%s/invitations", clerkBaseURL, orgID)

	body, _ := json.Marshal(map[string]string{
		"email_address": email,
		"role":          "org:member",
	})

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.clerkSecretKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("clerk API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("clerk API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// AcceptInvitationByEmail finds a pending invitation matching the email and
// org, marks it as accepted, and assigns the department/role to the user.
func (s *EmployeeService) AcceptInvitationByEmail(email, orgID, userID string) error {
	var invitation models.EmployeeInvitation
	err := s.db.Where(
		"email = ? AND organization_id = ? AND status = ?",
		email, orgID, "pending",
	).First(&invitation).Error
	if err != nil {
		return fmt.Errorf("no pending invitation found for %s in org %s", email, orgID)
	}

	if invitation.IsExpired() {
		if err := s.db.Model(&invitation).Update("status", "expired").Error; err != nil {
			return fmt.Errorf("failed to mark invitation as expired: %w", err)
		}
		return fmt.Errorf("invitation for %s has expired", email)
	}

	now := time.Now()

	return s.db.Transaction(func(tx *gorm.DB) error {
		// Mark invitation as accepted
		if err := tx.Model(&invitation).Updates(map[string]interface{}{
			"status":           "accepted",
			"accepted_at":      now,
			"accepted_user_id": userID,
			"updated_at":       now,
		}).Error; err != nil {
			return fmt.Errorf("failed to accept invitation: %w", err)
		}

		// Build user updates
		userUpdates := map[string]interface{}{
			"department_id": invitation.DepartmentID,
			"updated_at":    now,
		}
		if invitation.JobTitle != "" {
			userUpdates["job_title"] = invitation.JobTitle
		}

		// Resolve and assign role if specified
		if invitation.RoleName != "" {
			var role models.Role
			if err := tx.Where("name = ? AND organization_id = ?", invitation.RoleName, orgID).First(&role).Error; err == nil {
				userUpdates["role_id"] = role.ID
			} else {
				log.Printf("Role %q not found for org %s, skipping role assignment", invitation.RoleName, orgID)
			}
		}

		result := tx.Model(&models.User{}).Where("id = ?", userID).Updates(userUpdates)
		if result.Error != nil {
			return fmt.Errorf("failed to update user after invitation acceptance: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("user %s not found, rolling back invitation acceptance", userID)
		}

		log.Printf("Invitation accepted: user %s joined org %s, assigned to department %s",
			userID, orgID, invitation.DepartmentID)
		return nil
	})
}

// resolveDepartmentID tries to find a department by name within the org first,
// then falls back to treating the input as a direct department ID.
func (s *EmployeeService) resolveDepartmentID(orgID, nameOrID string) (string, error) {
	// Try by name first
	var dept models.Department
	err := s.db.Where("name = ? AND organization_id = ?", nameOrID, orgID).First(&dept).Error
	if err == nil {
		return dept.ID, nil
	}

	// Fall back to treating as a direct ID
	err = s.db.Where("id = ? AND organization_id = ?", nameOrID, orgID).First(&dept).Error
	if err == nil {
		return dept.ID, nil
	}

	return "", fmt.Errorf("department %q not found in organization %s", nameOrID, orgID)
}
