package handler

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/auth/internal/service"
	"github.com/gin-gonic/gin"
)

type EmployeeHandler struct {
	Service *service.EmployeeService
}

func NewEmployeeHandler(svc *service.EmployeeService) *EmployeeHandler {
	return &EmployeeHandler{Service: svc}
}

// POST /api/orgs/:orgId/departments
func (h *EmployeeHandler) CreateDepartment(c *gin.Context) {
	orgID := c.Param("orgId")
	userID := c.GetString("user_id")
	var body struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	dept, err := h.Service.CreateDepartment(orgID, body.Name, body.Description, userID)
	if err != nil {
		if errors.Is(err, service.ErrDuplicateDepartment) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			log.Printf("CreateDepartment error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusCreated, dept)
}

// GET /api/orgs/:orgId/departments
func (h *EmployeeHandler) ListDepartments(c *gin.Context) {
	orgID := c.Param("orgId")
	depts, err := h.Service.ListDepartmentSummaries(orgID)
	if err != nil {
		log.Printf("ListDepartments error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, depts)
}

// PUT /api/orgs/:orgId/departments/:deptID
func (h *EmployeeHandler) UpdateDepartment(c *gin.Context) {
	orgID := c.Param("orgId")
	deptID := c.Param("deptID")
	var body struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}
	dept, err := h.Service.UpdateDepartment(orgID, deptID, body.Name, body.Description)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDuplicateDepartment):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			log.Printf("UpdateDepartment error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, dept)
}

// DELETE /api/orgs/:orgId/departments/:deptID
func (h *EmployeeHandler) DeleteDepartment(c *gin.Context) {
	orgID := c.Param("orgId")
	deptID := c.Param("deptID")
	if err := h.Service.DeleteDepartment(orgID, deptID); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case strings.Contains(err.Error(), "still has"):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			log.Printf("DeleteDepartment error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Department deleted"})
}

// POST /api/orgs/:orgId/roles
func (h *EmployeeHandler) CreateRole(c *gin.Context) {
	orgID := c.Param("orgId")
	userID := c.GetString("user_id")
	var body struct {
		Name        string   `json:"name" binding:"required"`
		Description string   `json:"description"`
		MemberIDs   []string `json:"member_ids"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}

	role, err := h.Service.CreateRole(orgID, body.Name, body.Description, userID, body.MemberIDs)
	if err != nil {
		if errors.Is(err, service.ErrDuplicateRole) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		} else {
			log.Printf("CreateRole error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, role)
}

// GET /api/orgs/:orgId/roles
func (h *EmployeeHandler) ListRoles(c *gin.Context) {
	// This handler is intentionally user-context only via OrgMemberOnly().
	// Workflow initiations that need role resolution must propagate the original
	// user Authorization header, or add a separate service-token endpoint for
	// non-user-initiated execution paths.
	orgID := c.Param("orgId")
	roles, err := h.Service.ListRoleSummaries(orgID)
	if err != nil {
		log.Printf("ListRoles error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// PUT /api/orgs/:orgId/roles/:roleID
func (h *EmployeeHandler) UpdateRole(c *gin.Context) {
	orgID := c.Param("orgId")
	roleID := c.Param("roleID")
	userID := c.GetString("user_id")
	var body struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		MemberIDs   []string `json:"member_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	role, err := h.Service.UpdateRole(orgID, roleID, body.Name, body.Description, userID, body.MemberIDs)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDuplicateRole):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			log.Printf("UpdateRole error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, role)
}

// DELETE /api/orgs/:orgId/roles/:roleID
func (h *EmployeeHandler) DeleteRole(c *gin.Context) {
	orgID := c.Param("orgId")
	roleID := c.Param("roleID")
	if err := h.Service.DeleteRole(orgID, roleID); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			log.Printf("DeleteRole error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Role deleted"})
}

// POST /api/orgs/:orgId/employees/invite
func (h *EmployeeHandler) InviteSingle(c *gin.Context) {
	orgID := c.Param("orgId")
	userID := c.GetString("user_id")

	var body struct {
		Email          string   `json:"email" binding:"required,email"`
		FirstName      string   `json:"first_name" binding:"required"`
		LastName       string   `json:"last_name" binding:"required"`
		DepartmentName string   `json:"department" binding:"required"`
		RoleName       string   `json:"role"`
		Roles          []string `json:"roles"`
		JobTitle       string   `json:"job_title"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.Service.InviteAndNotify(service.InviteInput{
		OrgID:        orgID,
		Email:        body.Email,
		FirstName:    body.FirstName,
		LastName:     body.LastName,
		DepartmentID: body.DepartmentName,
		Role:         body.RoleName,
		Roles:        body.Roles,
		JobTitle:     body.JobTitle,
		InvitedBy:    userID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrDuplicateInvite):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			log.Printf("InviteSingle error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"invitation": result.Invitation,
		"message":    "Invitation created and email sent",
	})
}

// GET /api/orgs/:orgId/invitations
func (h *EmployeeHandler) ListInvitations(c *gin.Context) {
	orgID := c.Param("orgId")
	invitations, err := h.Service.ListInvitations(orgID)
	if err != nil {
		log.Printf("ListInvitations error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, invitations)
}

// DELETE /api/orgs/:orgId/invitations/:invitationId
func (h *EmployeeHandler) RevokeInvitation(c *gin.Context) {
	orgID := c.Param("orgId")
	invID := c.Param("invitationId")
	if err := h.Service.RevokeInvitation(invID, orgID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Printf("RevokeInvitation error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Invitation revoked"})
}

// POST /api/orgs/:orgId/employees/invite/bulk
func (h *EmployeeHandler) InviteBulk(c *gin.Context) {
	orgID := c.Param("orgId")
	userID := c.GetString("user_id")

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Excel file is required (form field: 'file')"})
		return
	}
	defer file.Close()

	parseResult, err := service.ParseEmployeeExcel(file)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	successful := 0
	var inviteErrors []service.ParseError

	for _, row := range parseResult.Rows {
		_, err := h.Service.InviteAndNotify(service.InviteInput{
			OrgID:        orgID,
			Email:        row.Email,
			FirstName:    row.FirstName,
			LastName:     row.LastName,
			DepartmentID: row.Department,
			Role:         row.Role,
			Roles:        []string{row.Role},
			JobTitle:     row.JobTitle,
			InvitedBy:    userID,
		})
		if err != nil {
			inviteErrors = append(inviteErrors, service.ParseError{
				Row:     row.RowNumber,
				Email:   row.Email,
				Message: err.Error(),
			})
		} else {
			successful++
		}
	}

	allErrors := append(parseResult.Errors, inviteErrors...)

	c.JSON(http.StatusOK, gin.H{
		"total_rows": len(parseResult.Rows) + len(parseResult.Errors),
		"successful": successful,
		"failed":     len(allErrors),
		"errors":     allErrors,
	})
}

// GET /api/orgs/:orgId/employees
func (h *EmployeeHandler) ListEmployees(c *gin.Context) {
	orgID := c.Param("orgId")
	employees, err := h.Service.ListEmployees(orgID)
	if err != nil {
		log.Printf("ListEmployees error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, employees)
}

// DELETE /api/orgs/:orgId/employees/:employeeId
func (h *EmployeeHandler) DeleteEmployee(c *gin.Context) {
	orgID := c.Param("orgId")
	employeeID := c.Param("employeeId")
	actorUserID := c.GetString("user_id")

	if err := h.Service.RemoveEmployee(orgID, employeeID, actorUserID); err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, service.ErrCannotRemoveAdmin), errors.Is(err, service.ErrCannotRemoveSelf):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			log.Printf("DeleteEmployee error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Employee removed"})
}

// GET /api/orgs/:orgId/departments/:deptID
func (h *EmployeeHandler) GetDepartment(c *gin.Context) {
	orgID := c.Param("orgId")
	deptID := c.Param("deptID")
	deptDetails, err := h.Service.GetDepartmentDetails(orgID, deptID)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		} else {
			log.Printf("GetDepartment error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(http.StatusOK, deptDetails)
}

// POST /api/orgs/:orgId/invitations/:invitationId/accept
func (h *EmployeeHandler) AcceptInvitation(c *gin.Context) {
	orgID := c.Param("orgId")
	invitationID := c.Param("invitationId")
	userID := c.GetString("user_id")

	err := h.Service.AcceptInvitationByID(invitationID, orgID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case err.Error() == "invitation email does not match user email":
			c.JSON(http.StatusForbidden, gin.H{"error": "This invitation is not for your account"})
		default:
			log.Printf("AcceptInvitation error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Invitation accepted successfully"})
}

// GET /api/orgs/:orgId/me/profile
func (h *EmployeeHandler) GetMyProfile(c *gin.Context) {
	orgID := c.Param("orgId")
	userID := c.GetString("user_id")

	profile, err := h.Service.GetMemberProfile(orgID, userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrNotFound):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			log.Printf("GetMyProfile error: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusOK, profile)
}
