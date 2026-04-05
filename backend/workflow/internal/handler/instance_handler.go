package handler

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

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

type compactInstanceLister interface {
	ListInstancesByOrgCompact(orgID string) ([]models.Instance, error)
	ListInstancesByWorkflowCompact(workflowID string) ([]models.Instance, error)
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
	if strings.TrimSpace(h.IntegrationKey) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "integration key not configured"})
		return
	}

	header := c.GetHeader("X-Integration-Key")
	if subtle.ConstantTimeCompare([]byte(header), []byte(h.IntegrationKey)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid integration key"})
		return
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
	if wf.Trigger.Type != models.TriggerFormSubmit {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow trigger is not form_submit"})
		return
	}
	if wf.Trigger.Config == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow trigger config is required"})
		return
	}
	if configuredFormID := strings.TrimSpace(wf.Trigger.Config["form_id"]); configuredFormID != "" {
		rawIncomingFormID := ""
		if formIDVal, ok := req.Data["_form_id"]; ok && formIDVal != nil {
			rawIncomingFormID = strings.TrimSpace(fmt.Sprint(formIDVal))
		}
		if rawIncomingFormID == "" || rawIncomingFormID != configuredFormID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "form submission does not match configured form_id"})
			return
		}
	}

	normalizedData := normalizeGoogleFormsData(wf.Trigger.Config, req.Data)
	responseID := ""
	if responseVal, ok := normalizedData["_response_id"]; ok && responseVal != nil {
		responseID = strings.TrimSpace(fmt.Sprint(responseVal))
	}
	if responseID == "" {
		if responseVal, ok := normalizedData["form_response_id"]; ok && responseVal != nil {
			responseID = strings.TrimSpace(fmt.Sprint(responseVal))
		}
	}

	instID, deduped, err := h.Exec.FindOrStartInstanceByFormResponse(wf, normalizedData, responseID, middleware.GetAuthorizationHeader(c))
	if err != nil {
		log.Printf("instance_handler.StartFromGoogleForms failed workflow_id=%q org_id=%q: %v", wf.ID, wf.OrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "start failed"})
		return
	}
	if deduped {
		c.JSON(http.StatusOK, gin.H{"instance_id": instID})
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
		if strings.TrimSpace(fmt.Sprint(v)) != "" {
			out["form_response_id"] = v
		}
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

// POST /integrations/gmail/events
func (h *InstanceHandler) StartFromGmail(c *gin.Context) {
	if strings.TrimSpace(h.IntegrationKey) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "integration key not configured"})
		return
	}

	header := c.GetHeader("X-Integration-Key")
	if subtle.ConstantTimeCompare([]byte(header), []byte(h.IntegrationKey)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid integration key"})
		return
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
	if wf.Trigger.Type != models.TriggerEmail {
		c.JSON(http.StatusBadRequest, gin.H{"error": "workflow trigger is not email_received"})
		return
	}

	if wf.Trigger.Config != nil {
		incomingFrom := strings.ToLower(strings.TrimSpace(payloadStringValue(req.Data, "_from")))
		incomingSubject := strings.ToLower(strings.TrimSpace(payloadStringValue(req.Data, "_subject")))
		incomingSnippet := strings.ToLower(strings.TrimSpace(payloadStringValue(req.Data, "_snippet")))

		if expectedFrom := strings.ToLower(strings.TrimSpace(wf.Trigger.Config["from_contains"])); expectedFrom != "" {
			if !strings.Contains(incomingFrom, expectedFrom) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "email sender does not match trigger config"})
				return
			}
		}
		if expectedSubject := strings.ToLower(strings.TrimSpace(wf.Trigger.Config["subject_contains"])); expectedSubject != "" {
			if !strings.Contains(incomingSubject, expectedSubject) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "email subject does not match trigger config"})
				return
			}
		}
		if expectedSnippet := strings.ToLower(strings.TrimSpace(wf.Trigger.Config["snippet_contains"])); expectedSnippet != "" {
			if !strings.Contains(incomingSnippet, expectedSnippet) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "email content does not match trigger config"})
				return
			}
		}
	}

	normalizedData := normalizeGmailData(wf.Trigger.Config, req.Data)
	emailMessageID := extractEmailMessageID(normalizedData)
	if emailMessageID != "" {
		existingInstanceID, lookupErr := h.findExistingInstanceByEmailMessageID(req.WorkflowID, emailMessageID)
		if lookupErr != nil {
			log.Printf("instance_handler.StartFromGmail dedupe lookup failed workflow_id=%q org_id=%q message_id=%q: %v", wf.ID, wf.OrgID, emailMessageID, lookupErr)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "start failed"})
			return
		}
		if existingInstanceID != "" {
			c.JSON(http.StatusOK, gin.H{"instance_id": existingInstanceID})
			return
		}
	}

	instID, err := h.Exec.StartInstance(wf, normalizedData, middleware.GetAuthorizationHeader(c))
	if err != nil {
		log.Printf("instance_handler.StartFromGmail failed workflow_id=%q org_id=%q: %v", wf.ID, wf.OrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "start failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"instance_id": instID})
}

func extractEmailMessageID(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	raw, ok := data["email_message_id"]
	if !ok || raw == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}

func (h *InstanceHandler) findExistingInstanceByEmailMessageID(workflowID, emailMessageID string) (string, error) {
	instances, err := h.Store.ListInstancesByWorkflow(workflowID)
	if err != nil {
		return "", err
	}
	for _, instance := range instances {
		if extractEmailMessageID(instance.Data) == emailMessageID {
			return instance.ID, nil
		}
	}
	return "", nil
}

func payloadStringValue(payload map[string]interface{}, key string) string {
	if payload == nil {
		return ""
	}
	raw, ok := payload[key]
	if !ok || raw == nil {
		return ""
	}
	if value, ok := raw.(string); ok {
		return value
	}
	return fmt.Sprintf("%v", raw)
}

func normalizeGmailData(triggerConfig map[string]string, payload map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(payload)+8)
	for k, v := range payload {
		out[k] = v
	}

	out["trigger_source"] = "gmail"
	out["trigger_type"] = string(models.TriggerEmail)
	out["email_event"] = payload

	if v, ok := payload["_message_id"]; ok {
		out["email_message_id"] = v
	}
	if v, ok := payload["_thread_id"]; ok {
		out["email_thread_id"] = v
	}
	if v, ok := payload["_subject"]; ok {
		out["email_subject"] = v
	}
	if v, ok := payload["_from"]; ok {
		out["email_from"] = v
	}
	if v, ok := payload["_to"]; ok {
		out["email_to"] = v
	}
	if v, ok := payload["_snippet"]; ok {
		out["email_snippet"] = v
	}

	if triggerConfig != nil {
		if value := strings.TrimSpace(triggerConfig["query"]); value != "" {
			out["email_trigger_query"] = value
		}
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
	compact := strings.EqualFold(strings.TrimSpace(c.Query("compact")), "true")

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
		if compact {
			if compactStore, ok := h.Store.(compactInstanceLister); ok {
				instances, err = compactStore.ListInstancesByWorkflowCompact(workflowID)
			} else {
				instances, err = h.Store.ListInstancesByWorkflow(workflowID)
			}
		} else {
			instances, err = h.Store.ListInstancesByWorkflow(workflowID)
		}
	} else {
		if compact {
			if compactStore, ok := h.Store.(compactInstanceLister); ok {
				instances, err = compactStore.ListInstancesByOrgCompact(orgID)
			} else {
				instances, err = h.Store.ListInstancesByOrg(orgID)
			}
		} else {
			instances, err = h.Store.ListInstancesByOrg(orgID)
		}
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
	type compactInstance struct {
		ID           string                      `json:"id"`
		OrgID        string                      `json:"org_id"`
		WorkflowID   string                      `json:"workflow_id"`
		WorkflowName string                      `json:"workflow_name,omitempty"`
		Status       models.InstanceStatus       `json:"status"`
		CurrentNode  string                      `json:"current_node,omitempty"`
		NodeStates   map[string]models.NodeState `json:"node_states,omitempty"`
		StartedAt    time.Time                   `json:"started_at"`
		CompletedAt  *time.Time                  `json:"completed_at,omitempty"`
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
	compactOut := make([]compactInstance, 0, len(instances))
	for _, inst := range instances {
		if inst.OrgID != orgID {
			continue
		}
		wfName := ""
		if wf, ok := workflowMap[inst.WorkflowID]; ok {
			wfName = wf.Name
		}
		if compact {
			compactOut = append(compactOut, compactInstance{
				ID:           inst.ID,
				OrgID:        inst.OrgID,
				WorkflowID:   inst.WorkflowID,
				WorkflowName: wfName,
				Status:       inst.Status,
				CurrentNode:  inst.CurrentNode,
				NodeStates:   inst.NodeStates,
				StartedAt:    inst.StartedAt,
				CompletedAt:  inst.CompletedAt,
			})
			continue
		}

		out = append(out, enrichedInstance{Instance: inst, WorkflowName: wfName})
	}

	if compact {
		c.JSON(http.StatusOK, compactOut)
		return
	}

	c.JSON(http.StatusOK, out)
}
