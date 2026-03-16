package handler

import (
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"encoding/json"

	"github.com/gin-gonic/gin"
	svix "github.com/svix/svix-webhooks/go"
	"gorm.io/gorm"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/database"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/models"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/service"
)

type WebhookHandler struct {
	webhookSecret   string
	employeeService *service.EmployeeService
}

func NewWebhookHandler(secret string, empSvc *service.EmployeeService) *WebhookHandler {
	return &WebhookHandler{webhookSecret: secret, employeeService: empSvc}
}

// ClerkWebhookEvent represents the structure of a Clerk webhook payload
type ClerkWebhookEvent struct {
	Type   string          `json:"type"`
	Data   json.RawMessage `json:"data"`
	Object string          `json:"object"`
}

// ClerkUser represents user data from Clerk webhook
type ClerkUser struct {
	ID             string `json:"id"`
	EmailAddresses []struct {
		EmailAddress string `json:"email_address"`
	} `json:"email_addresses"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	ImageURL     string `json:"image_url"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	LastSignInAt *int64 `json:"last_sign_in_at"`
}

// ClerkOrganization represents organization data from Clerk webhook
type ClerkOrganization struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	ImageURL  string `json:"image_url"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

type ClerkMembershipData struct {
	ID           string `json:"id"`
	Role         string `json:"role"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	Organization struct {
		ID string `json:"id"`
	} `json:"organization"`
	PublicUserData struct {
		UserID string `json:"user_id"`
	} `json:"public_user_data"`
}

const defaultAdminDepartmentName = "Admin"

// IsAdmin checks if the Clerk membership role is an admin role.
func (m *ClerkMembershipData) IsAdmin() bool {
	return m.Role == "org:admin" || m.Role == "admin"
}

// Handle processes incoming Clerk webhooks
func (h *WebhookHandler) Handle(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		log.Printf("Error reading webhook body: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error reading request body"})
		return
	}

	// Verify the webhook signature using Svix. We do this because anyone on the web can send a webhook request to our backendURL. clerk signs its webhooks with a secret signature here we verify the webhook is indeed from clerk.
	wh, err := svix.NewWebhook(h.webhookSecret)
	if err != nil {
		log.Printf("Error creating webhook verifier: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	headers := http.Header{}
	headers.Set("svix-id", c.GetHeader("svix-id"))
	headers.Set("svix-timestamp", c.GetHeader("svix-timestamp"))
	headers.Set("svix-signature", c.GetHeader("svix-signature"))

	err = wh.Verify(body, headers)
	if err != nil {
		log.Printf("Invalid webhook signature: %v", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid signature"})
		return
	}

	// Parse the webhook event
	var event ClerkWebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("Error parsing webhook event: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Error parsing event"})
		return
	}

	log.Printf("Received Clerk webhook: %s", event.Type)

	switch event.Type {
	case "user.created":
		if err := h.handleUserCreated(event.Data); err != nil {
			log.Printf("Error handling user created event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling user created event"})
			return
		}
	case "user.updated":
		if err := h.handleUserUpdated(event.Data); err != nil {
			log.Printf("Error handling user updated event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling user updated event"})
			return
		}
	case "user.deleted":
		if err := h.handleUserDeleted(event.Data); err != nil {
			log.Printf("Error handling user deleted event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling user deleted event"})
			return
		}
	case "organization.created":
		if err := h.handleOrganizationCreated(event.Data); err != nil {
			log.Printf("Error handling organization created event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling organization created event"})
			return
		}
	case "organization.deleted":
		if err := h.handleOrganizationDeleted(event.Data); err != nil {
			log.Printf("Error handling organization deleted event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling organization deleted event"})
			return
		}
	case "organizationMembership.created":
		if err := h.handleMembershipCreated(event.Data); err != nil {
			log.Printf("Error handling membership created event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling membership created event"})
			return
		}
	case "organizationInvitation.accepted":
		if err := h.handleInvitationAccepted(event.Data); err != nil {
			log.Printf("Error handling invitation accepted event: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error handling invitation accepted event"})
			return
		}
	default:
		log.Printf("Unhandled webhook event type: %s", event.Type)
	}

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// This function handles the backend when clerk sends a webhook saying a new user is created
func (h *WebhookHandler) handleUserCreated(data json.RawMessage) error {
	var clerkUser ClerkUser
	if err := json.Unmarshal(data, &clerkUser); err != nil {
		log.Printf("Error parsing user data: %v", err)
		return err
	}

	email := ""
	if len(clerkUser.EmailAddresses) > 0 {
		email = clerkUser.EmailAddresses[0].EmailAddress
	}

	user := models.User{
		ID:        clerkUser.ID,
		Email:     email,
		FirstName: clerkUser.FirstName,
		LastName:  clerkUser.LastName,
		AvatarURL: clerkUser.ImageURL,
		IsActive:  true,
		CreatedAt: time.UnixMilli(clerkUser.CreatedAt),
		UpdatedAt: time.UnixMilli(clerkUser.UpdatedAt),
	}

	err := database.DB.Create(&user).Error
	if err != nil {
		log.Printf("Error creating user: %v", err)
		return err
	}

	log.Printf("User created: %s (%s)", user.ID, user.Email)
	return nil
}

// This function handles the backend when clerk sends a webhook saying a user is updated
func (h *WebhookHandler) handleUserUpdated(data json.RawMessage) error {
	var clerkUser ClerkUser
	if err := json.Unmarshal(data, &clerkUser); err != nil {
		log.Printf("Error parsing user data: %v", err)
		return err
	}

	email := ""
	if len(clerkUser.EmailAddresses) > 0 {
		email = clerkUser.EmailAddresses[0].EmailAddress
	}

	updates := map[string]interface{}{
		"email":      email,
		"first_name": clerkUser.FirstName,
		"last_name":  clerkUser.LastName,
		"avatar_url": clerkUser.ImageURL,
		"updated_at": time.UnixMilli(clerkUser.UpdatedAt),
	}

	if err := database.DB.Model(&models.User{}).Where("id = ?", clerkUser.ID).Updates(updates).Error; err != nil {
		log.Printf("Error updating user: %v", err)
		return err
	}

	log.Printf("User updated: %s", clerkUser.ID)
	return nil
}

// This function handles the backend when clerk sends a webhook saying a user is deleted
func (h *WebhookHandler) handleUserDeleted(data json.RawMessage) error {
	var clerkUser struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &clerkUser); err != nil {
		log.Printf("Error parsing user data: %v", err)
		return err
	}

	// Soft delete - set isActive to false
	if err := database.DB.Model(&models.User{}).Where("id = ?", clerkUser.ID).Update("is_active", false).Error; err != nil {
		log.Printf("Error deleting user: %v", err)
		return err
	}

	log.Printf("User deleted (soft): %s", clerkUser.ID)
	return nil
}

// This function handles the backend when clerk sends a webhook saying a new organization is created
func (h *WebhookHandler) handleOrganizationCreated(data json.RawMessage) error {
	var clerkOrg ClerkOrganization
	if err := json.Unmarshal(data, &clerkOrg); err != nil {
		log.Printf("Error parsing organization data: %v", err)
		return err
	}

	// Extract the creator's user ID from Clerk's webhook payload
	var rawData map[string]interface{}
	if err := json.Unmarshal(data, &rawData); err != nil {
		log.Printf("Error parsing raw organization data: %v", err)
		return err
	}
	creatorID := getString(rawData, "created_by")

	org := models.Organization{
		ID:        clerkOrg.ID,
		Name:      clerkOrg.Name,
		Slug:      clerkOrg.Slug,
		ImageURL:  clerkOrg.ImageURL,
		IsActive:  true,
		CreatedAt: time.UnixMilli(clerkOrg.CreatedAt),
		UpdatedAt: time.UnixMilli(clerkOrg.UpdatedAt),
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&org).Error; err != nil {
			return err
		}

		settings := models.OrganizationSettings{
			OrganizationID: clerkOrg.ID,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}

		if err := tx.Create(&settings).Error; err != nil {
			return err
		}

		adminDeptID, err := ensureAdminDepartment(tx, clerkOrg.ID, creatorID)
		if err != nil {
			return err
		}

		// Set the creator as an admin of the new org
		if creatorID != "" {
			result := tx.Model(&models.User{}).Where("id = ?", creatorID).Updates(map[string]interface{}{
				"organization_id": clerkOrg.ID,
				"department_id":   adminDeptID,
				"is_admin":        true,
				"updated_at":      time.Now(),
			})
			if result.Error != nil {
				log.Printf("Warning: failed to set org creator %s as admin: %v", creatorID, result.Error)
			} else if result.RowsAffected == 0 {
				log.Printf("Warning: creator %s not yet in DB for org %s (will be set via membership webhook)", creatorID, clerkOrg.ID)
			}
		}

		return nil
	})
	if err != nil {
		log.Printf("Error creating organization and settings: %v", err)
		return err
	}

	log.Printf("Organization created: %s (%s) by %s", clerkOrg.ID, clerkOrg.Name, creatorID)
	return nil
}

func (h *WebhookHandler) handleOrganizationDeleted(data json.RawMessage) error {
	var clerkOrg struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &clerkOrg); err != nil {
		log.Printf("Error parsing organization data: %v", err)
		return err
	}

	// Soft delete
	if err := database.DB.Model(&models.Organization{}).Where("id = ?", clerkOrg.ID).Update("is_active", false).Error; err != nil {
		log.Printf("Error deleting organization: %v", err)
		return err
	}

	log.Printf("Organization deleted (soft): %s", clerkOrg.ID)
	return nil
}

func (h *WebhookHandler) handleMembershipCreated(data json.RawMessage) error {
	var memberData ClerkMembershipData
	if err := json.Unmarshal(data, &memberData); err != nil {
		log.Printf("Error parsing membership data: %v", err)
		return err
	}

	userID := memberData.PublicUserData.UserID
	orgID := memberData.Organization.ID
	isAdmin := memberData.IsAdmin()

	updates := map[string]interface{}{
		"organization_id": orgID,
		"is_admin":        isAdmin,
		"updated_at":      time.Now(),
	}

	if isAdmin {
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			adminDeptID, err := ensureAdminDepartment(tx, orgID, userID)
			if err != nil {
				return err
			}
			updates["department_id"] = adminDeptID
			result := tx.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				log.Printf("User %s not found for membership linking (user.created webhook may not have arrived yet)", userID)
			}
			return nil
		})
		if err != nil {
			log.Printf("Error updating admin membership for user %s: %v", userID, err)
			return err
		}
	} else {
		result := database.DB.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
		if result.Error != nil {
			log.Printf("Error updating user %s with org membership: %v", userID, result.Error)
			return result.Error
		}
		if result.RowsAffected == 0 {
			log.Printf("User %s not found for membership linking (user.created webhook may not have arrived yet)", userID)
			return nil
		}
	}

	// 2. Look up user's email to match against pending invitations
	var user models.User
	if err := database.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		log.Printf("User %s not found for invitation matching: %v", userID, err)
		return nil
	}

	// 3. Try to auto-accept a matching invitation (assigns department + role)
	if h.employeeService != nil {
		if err := h.employeeService.AcceptInvitationByEmail(user.Email, orgID, userID); err != nil {
			log.Printf("Invitation auto-accept note: %v", err)
		}
	}

	log.Printf("Membership created: user %s joined org %s (is_admin=%v)", userID, orgID, isAdmin)
	return nil
}

// ClerkInvitationAcceptedData represents data from an organizationInvitation.accepted webhook
type ClerkInvitationAcceptedData struct {
	ID             string `json:"id"`
	EmailAddress   string `json:"email_address"`
	OrganizationID string `json:"organization_id"`
	Role           string `json:"role"`
	Status         string `json:"status"`
}

func (h *WebhookHandler) handleInvitationAccepted(data json.RawMessage) error {
	var invData ClerkInvitationAcceptedData
	if err := json.Unmarshal(data, &invData); err != nil {
		log.Printf("Error parsing invitation accepted data: %v", err)
		return err
	}

	email := invData.EmailAddress
	orgID := invData.OrganizationID

	log.Printf("Invitation accepted: email=%s org=%s role=%s", email, orgID, invData.Role)

	// Look up user by email
	var user models.User
	if err := database.DB.Where("email = ?", email).First(&user).Error; err != nil {
		log.Printf("User with email %s not found for invitation acceptance: %v", email, err)
		return nil
	}

	// Set the user's organization and admin status
	isAdmin := invData.Role == "org:admin" || invData.Role == "admin"
	if isAdmin {
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			adminDeptID, err := ensureAdminDepartment(tx, orgID, user.ID)
			if err != nil {
				return err
			}
			updates := map[string]interface{}{
				"organization_id": orgID,
				"department_id":   adminDeptID,
				"is_admin":        true,
				"updated_at":      time.Now(),
			}
			return tx.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error
		})
		if err != nil {
			log.Printf("Error updating admin invitation acceptance for user %s: %v", user.ID, err)
			return err
		}
	} else {
		updates := map[string]interface{}{
			"organization_id": orgID,
			"is_admin":        false,
			"updated_at":      time.Now(),
		}
		if err := database.DB.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
			log.Printf("Error updating user %s with org: %v", user.ID, err)
			return err
		}
	}

	// Auto-accept matching local employee invitation
	if h.employeeService != nil {
		if err := h.employeeService.AcceptInvitationByEmail(email, orgID, user.ID); err != nil {
			log.Printf("Local invitation auto-accept note: %v", err)
		}
	}

	log.Printf("Invitation accepted: user %s (%s) joined org %s", user.ID, email, orgID)
	return nil
}

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func ensureAdminDepartment(tx *gorm.DB, orgID, createdBy string) (string, error) {
	var department models.Department
	err := tx.Where("organization_id = ? AND name = ?", orgID, defaultAdminDepartmentName).First(&department).Error
	if err == nil {
		return department.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	department = models.Department{
		OrganizationID: orgID,
		Name:           defaultAdminDepartmentName,
		Description:    "Special executive/admin bucket for members not assigned to a functional department.",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if createdBy != "" {
		department.CreatedByUserID = &createdBy
	}
	if err := tx.Create(&department).Error; err != nil {
		return "", err
	}
	return department.ID, nil
}
