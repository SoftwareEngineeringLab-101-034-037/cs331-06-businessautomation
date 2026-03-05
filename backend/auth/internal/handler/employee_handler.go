package handler

import (
	"errors"
	"log"
	"net/http"

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
	var body struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Name is required"})
		return
	}
	dept, err := h.Service.CreateDepartment(orgID, body.Name, body.Description)
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
	depts, err := h.Service.ListDepartments(orgID)
	if err != nil {
		log.Printf("ListDepartments error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	c.JSON(http.StatusOK, depts)
}

// POST /api/orgs/:orgId/employees/invite
func (h *EmployeeHandler) InviteSingle(c *gin.Context) {
	orgID := c.Param("orgId")
	userID := c.GetString("user_id")

	var body struct {
		Email          string `json:"email" binding:"required,email"`
		FirstName      string `json:"first_name" binding:"required"`
		LastName       string `json:"last_name" binding:"required"`
		DepartmentName string `json:"department" binding:"required"`
		RoleName       string `json:"role"`
		JobTitle       string `json:"job_title"`
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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
