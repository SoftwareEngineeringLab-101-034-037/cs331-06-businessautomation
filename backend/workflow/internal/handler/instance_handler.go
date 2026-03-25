package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/example/business-automation/backend/workflow/internal/executor"
	"github.com/example/business-automation/backend/workflow/internal/models"
	"github.com/example/business-automation/backend/workflow/internal/storage"
)

// InstanceHandler handles starting and inspecting workflow instances.
type InstanceHandler struct {
	Store storage.Store
	Exec  *executor.Executor
}

func NewInstanceHandler(store storage.Store, exec *executor.Executor) *InstanceHandler {
	return &InstanceHandler{Store: store, Exec: exec}
}

// POST /api/orgs/:orgId/instances
func (h *InstanceHandler) Start(c *gin.Context) {
	var req struct {
		WorkflowID string                 `json:"workflow_id"`
		Data       map[string]interface{} `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}

	wf, ok := h.Store.GetWorkflow(req.WorkflowID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	if wf.Status != models.WorkflowActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow is not active"})
		return
	}

	instID, err := h.Exec.StartInstance(wf, req.Data)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "start failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"instance_id": instID})
}

// GET /api/orgs/:orgId/instances/:id
func (h *InstanceHandler) Get(c *gin.Context) {
	id := c.Param("id")
	inst, ok := h.Store.GetInstance(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if inst.OrgID != c.Param("orgId") {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusOK, inst)
}

// GET /api/orgs/:orgId/instances?workflow_id=...
func (h *InstanceHandler) List(c *gin.Context) {
	orgID := c.Param("orgId")
	workflowID := c.Query("workflow_id")

	var (
		instances []models.Instance
		err       error
	)
	if workflowID != "" {
		instances, err = h.Store.ListInstancesByWorkflow(workflowID)
	} else {
		instances, err = h.Store.ListInstancesByOrg(orgID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type enrichedInstance struct {
		models.Instance
		WorkflowName string `json:"workflow_name,omitempty"`
	}
	out := make([]enrichedInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.OrgID != orgID {
			continue
		}
		wfName := ""
		if wf, ok := h.Store.GetWorkflow(inst.WorkflowID); ok {
			wfName = wf.Name
		}
		out = append(out, enrichedInstance{Instance: inst, WorkflowName: wfName})
	}

	c.JSON(http.StatusOK, out)
}
