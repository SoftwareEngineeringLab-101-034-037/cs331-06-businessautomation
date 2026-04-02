package handler

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/executor"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/middleware"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/models"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/storage"
)

// InstanceHandler handles starting and inspecting workflow instances.
type InstanceHandler struct {
	Store          storage.Store
	Exec           *executor.Executor
	IntegrationKey string
}

func NewInstanceHandler(store storage.Store, exec *executor.Executor, integrationKey ...string) *InstanceHandler {
	key := ""
	if len(integrationKey) > 0 {
		key = integrationKey[0]
	}
	return &InstanceHandler{Store: store, Exec: exec, IntegrationKey: key}
}

// POST /integrations/google-forms/events
func (h *InstanceHandler) StartFromGoogleForms(c *gin.Context) {
	if h.IntegrationKey != "" {
		if c.GetHeader("X-Integration-Key") != h.IntegrationKey {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid integration key"})
			return
		}
	}

	var req struct {
		OrgID      string                 `json:"org_id"`
		WorkflowID string                 `json:"workflow_id"`
		Data       map[string]interface{} `json:"data"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return
	}
	if req.OrgID == "" || req.WorkflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_id and workflow_id are required"})
		return
	}

	wf, ok := h.Store.GetWorkflow(req.WorkflowID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
		return
	}
	if wf.OrgID != req.OrgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if wf.Status != models.WorkflowActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow is not active"})
		return
	}

	normalizedData := normalizeGoogleFormsData(wf.Trigger.Config, req.Data)
	instID, err := h.Exec.StartInstance(wf, normalizedData, "")
	if err != nil {
		log.Printf("instance_handler.StartFromGoogleForms failed workflow_id=%q org_id=%q: %v", wf.ID, wf.OrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "start failed"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"instance_id": instID})
}

func normalizeGoogleFormsData(triggerConfig map[string]string, payload map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(payload)+6)
	for k, v := range payload {
		out[k] = v
	}

	fieldMapping := parseFieldMappingCSV(triggerConfig["field_mapping"])
	for source, target := range fieldMapping {
		if source == "" || target == "" {
			continue
		}
		if v, ok := payload[source]; ok {
			out[target] = v
		}
	}

	out["trigger_source"] = "google_forms"
	out["trigger_type"] = string(models.TriggerFormSubmit)
	out["form_submission"] = payload

	if formID := strings.TrimSpace(triggerConfig["form_id"]); formID != "" {
		out["form_id"] = formID
	}
	if v, ok := payload["_form_id"]; ok {
		out["form_id"] = v
	}
	if v, ok := payload["_submitted_at"]; ok {
		out["form_submitted_at"] = v
	}
	if v, ok := payload["_response_id"]; ok {
		out["form_response_id"] = v
	}

	return out
}

func parseFieldMappingCSV(raw string) map[string]string {
	out := make(map[string]string)
	for _, pair := range strings.Split(raw, ",") {
		parts := strings.SplitN(strings.TrimSpace(pair), ":", 2)
		if len(parts) != 2 {
			continue
		}
		from := strings.TrimSpace(parts[0])
		to := strings.TrimSpace(parts[1])
		if from == "" || to == "" {
			continue
		}
		out[from] = to
	}
	return out
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
	orgID := c.Param("orgId")
	if wf.OrgID != orgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if wf.Status != models.WorkflowActive {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow is not active"})
		return
	}

	instID, err := h.Exec.StartInstance(wf, req.Data, middleware.GetAuthorizationHeader(c))
	if err != nil {
		log.Printf("instance_handler.Start failed workflow_id=%q org_id=%q: %v", wf.ID, wf.OrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "start failed"})
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
		wf, ok := h.Store.GetWorkflow(workflowID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "workflow not found"})
			return
		}
		if wf.OrgID != orgID {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		instances, err = h.Store.ListInstancesByWorkflow(workflowID)
	} else {
		instances, err = h.Store.ListInstancesByOrg(orgID)
	}
	if err != nil {
		log.Printf("instance_handler.List org=%s workflow=%s list failed: %v", orgID, workflowID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	type enrichedInstance struct {
		models.Instance
		WorkflowName string `json:"workflow_name,omitempty"`
	}
	workflowIDs := make([]string, 0, len(instances))
	seenWorkflowIDs := make(map[string]struct{}, len(instances))
	for _, inst := range instances {
		if inst.OrgID != orgID || inst.WorkflowID == "" {
			continue
		}
		if _, seen := seenWorkflowIDs[inst.WorkflowID]; seen {
			continue
		}
		seenWorkflowIDs[inst.WorkflowID] = struct{}{}
		workflowIDs = append(workflowIDs, inst.WorkflowID)
	}
	workflowMap, err := h.Store.GetWorkflowsByIDs(workflowIDs)
	if err != nil {
		log.Printf("instance_handler.List org=%s workflow=%s batch workflow lookup failed: %v", orgID, workflowID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	out := make([]enrichedInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.OrgID != orgID {
			continue
		}
		wfName := ""
		if wf, ok := workflowMap[inst.WorkflowID]; ok {
			wfName = wf.Name
		}
		out = append(out, enrichedInstance{Instance: inst, WorkflowName: wfName})
	}

	c.JSON(http.StatusOK, out)
}
