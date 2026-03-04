package handler

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/example/business-automation/backend/workflow/internal/middleware"
	"github.com/example/business-automation/backend/workflow/internal/models"
	"github.com/example/business-automation/backend/workflow/internal/storage"
)

// WorkflowHandler handles CRUD operations on workflow definitions.
type WorkflowHandler struct {
	Store storage.Store
}

func NewWorkflowHandler(store storage.Store) *WorkflowHandler {
	return &WorkflowHandler{Store: store}
}

// GET /api/orgs/:orgId/workflows
func (h *WorkflowHandler) List(c *gin.Context) {
	orgId := c.Param("orgId")
	wfs, err := h.Store.ListWorkflows(orgId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, wfs)
}

// POST /api/orgs/:orgId/workflows
func (h *WorkflowHandler) Create(c *gin.Context) {
	orgId := c.Param("orgId")
	var wf models.Workflow
	if err := c.ShouldBindJSON(&wf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON: " + err.Error()})
		return
	}
	if wf.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	now := time.Now()
	if wf.Status == "" {
		wf.Status = models.WorkflowActive
	}
	wf.OrgID = orgId
	wf.CreatedAt = now
	wf.UpdatedAt = now
	if wf.Version == 0 && wf.Status != "draft" {
		wf.Version = 1
	}

	// Attribute the workflow to the authenticated user
	if userID := middleware.GetUserID(c); userID != "" {
		wf.CreatedBy = userID
	}

	id, err := h.Store.SaveWorkflow(wf)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed: " + err.Error()})
		return
	}
	wf.ID = id
	c.JSON(http.StatusCreated, gin.H{"id": id, "workflow": wf})
}

// GET /api/orgs/:orgId/workflows/:id
func (h *WorkflowHandler) Get(c *gin.Context) {
	id := c.Param("id")
	wf, ok := h.Store.GetWorkflow(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, wf)
}

// PUT /api/orgs/:orgId/workflows/:id
func (h *WorkflowHandler) Update(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		models.Workflow
		CommitMessage string `json:"commit_message"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	wf := req.Workflow
	wf.ID = id
	wf.UpdatedAt = time.Now()

	orgId := c.Param("orgId")
	wf.OrgID = orgId

	if existing, ok := h.Store.GetWorkflow(id); ok {
		wf.CreatedAt = existing.CreatedAt
		wf.CreatedBy = existing.CreatedBy
		if wf.Status == "draft" {
			wf.Version = 0
		} else if wf.Version <= existing.Version {
			wf.Version = existing.Version + 1
		}
	} else {
		wf.CreatedAt = wf.UpdatedAt
		if wf.Status == "draft" {
			wf.Version = 0
		} else if wf.Version == 0 {
			wf.Version = 1
		}
	}

	if req.CommitMessage != "" {
		log.Printf("[AUDIT] workflow %s (v%d) updated by %s — %s",
			wf.ID, wf.Version, middleware.GetUserID(c), req.CommitMessage)
	}

	if _, err := h.Store.SaveWorkflow(wf); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, wf)
}

// DELETE /api/orgs/:orgId/workflows/:id
func (h *WorkflowHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.Store.DeleteWorkflow(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": id})
}
