package handler

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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

type integrationInstanceRequest struct {
	OrgID      string                 `json:"org_id"`
	WorkflowID string                 `json:"workflow_id"`
	Data       map[string]interface{} `json:"data"`
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
	req, ok := h.requireIntegrationRequest(c)
	if !ok {
		return
	}

	wf, found := h.Store.GetWorkflow(req.WorkflowID)
	if !found {
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
	validatedData, validationErr := validateAndCoerceGoogleFormsData(wf.Trigger.Config, normalizedData)
	if validationErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": validationErr.Error()})
		return
	}
	normalizedData = validatedData

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

type triggerFieldSchemaItem struct {
	QuestionID string `json:"question_id"`
	Title      string `json:"title"`
	Required   bool   `json:"required"`
	FieldType  string `json:"field_type"`
	Variable   string `json:"variable"`
	DataType   string `json:"data_type"`
}

func validateAndCoerceGoogleFormsData(triggerConfig map[string]string, data map[string]interface{}) (map[string]interface{}, error) {
	if triggerConfig == nil {
		return data, nil
	}
	rawSchema := strings.TrimSpace(triggerConfig["field_schema"])
	if rawSchema == "" {
		return data, nil
	}

	var schema []triggerFieldSchemaItem
	if err := json.Unmarshal([]byte(rawSchema), &schema); err != nil {
		return nil, fmt.Errorf("invalid trigger field_schema")
	}

	out := make(map[string]interface{}, len(data))
	for key, value := range data {
		out[key] = value
	}

	for _, item := range schema {
		fieldKey := strings.TrimSpace(item.Variable)
		if fieldKey == "" {
			fieldKey = strings.TrimSpace(item.QuestionID)
		}
		if fieldKey == "" {
			continue
		}

		rawValue, exists := out[fieldKey]
		if !exists || isEmptyTriggerValue(rawValue) {
			if item.Required {
				return nil, fmt.Errorf("required trigger field %q is missing", triggerFieldLabel(item, fieldKey))
			}
			continue
		}

		dataType := resolveTriggerFieldDataType(item)
		coerced, ok := coerceTriggerValueByDataType(rawValue, dataType)
		if !ok {
			return nil, fmt.Errorf("trigger field %q must be %s", triggerFieldLabel(item, fieldKey), dataType)
		}
		out[fieldKey] = coerced
	}

	return out, nil
}

func triggerFieldLabel(item triggerFieldSchemaItem, fallback string) string {
	if title := strings.TrimSpace(item.Title); title != "" {
		return title
	}
	if variable := strings.TrimSpace(item.Variable); variable != "" {
		return variable
	}
	if questionID := strings.TrimSpace(item.QuestionID); questionID != "" {
		return questionID
	}
	return fallback
}

func resolveTriggerFieldDataType(item triggerFieldSchemaItem) string {
	if explicit := normalizeTriggerDataType(item.DataType); explicit != "" {
		return explicit
	}
	switch strings.ToLower(strings.TrimSpace(item.FieldType)) {
	case "scale", "number", "numeric":
		return "number"
	case "date":
		return "date"
	case "time":
		return "time"
	case "datetime", "date_time", "timestamp":
		return "datetime"
	case "bool", "boolean", "yes_no":
		return "boolean"
	default:
		return "text"
	}
}

func normalizeTriggerDataType(raw string) string {
	value := strings.ToLower(strings.TrimSpace(raw))
	switch value {
	case "number", "numeric", "int", "float", "decimal":
		return "number"
	case "boolean", "bool":
		return "boolean"
	case "date":
		return "date"
	case "time":
		return "time"
	case "datetime", "date_time", "timestamp":
		return "datetime"
	case "text", "string", "":
		return "text"
	default:
		return "text"
	}
}

func coerceTriggerValueByDataType(value interface{}, dataType string) (interface{}, bool) {
	switch dataType {
	case "number":
		return coerceTriggerNumber(value)
	case "boolean":
		return coerceTriggerBoolean(value)
	case "date":
		return coerceTriggerDate(value)
	case "time":
		return coerceTriggerTime(value)
	case "datetime":
		return coerceTriggerDateTime(value)
	case "text":
		return strings.TrimSpace(fmt.Sprint(value)), true
	default:
		return strings.TrimSpace(fmt.Sprint(value)), true
	}
}

func coerceTriggerNumber(value interface{}) (interface{}, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int8:
		return float64(typed), true
	case int16:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case uint:
		return float64(typed), true
	case uint8:
		return float64(typed), true
	case uint16:
		return float64(typed), true
	case uint32:
		return float64(typed), true
	case uint64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		if err != nil {
			return nil, false
		}
		return parsed, true
	case string:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		if err != nil {
			return nil, false
		}
		return parsed, true
	default:
		parsed, err := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(value)), 64)
		if err != nil {
			return nil, false
		}
		return parsed, true
	}
}

func coerceTriggerBoolean(value interface{}) (interface{}, bool) {
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		default:
			return nil, false
		}
	case float64:
		if typed == 1 {
			return true, true
		}
		if typed == 0 {
			return false, true
		}
		return nil, false
	default:
		text := strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
		switch text {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		default:
			return nil, false
		}
	}
}

func coerceTriggerDate(value interface{}) (interface{}, bool) {
	parsed, ok := parseTriggerDateValue(value)
	if !ok {
		return nil, false
	}
	return parsed.Format("2006-01-02"), true
}

func coerceTriggerTime(value interface{}) (interface{}, bool) {
	parsed, ok := parseTriggerClockValue(value)
	if !ok {
		return nil, false
	}
	return parsed.Format("15:04:05"), true
}

func coerceTriggerDateTime(value interface{}) (interface{}, bool) {
	parsed, ok := parseTriggerDateTimeValue(value)
	if !ok {
		return nil, false
	}
	return parsed.Format(time.RFC3339), true
}

func parseTriggerDateValue(value interface{}) (time.Time, bool) {
	if value == nil {
		return time.Time{}, false
	}
	if typed, ok := value.(time.Time); ok {
		y, m, d := typed.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), true
	}
	raw := strings.TrimSpace(fmt.Sprint(value))
	if raw == "" {
		return time.Time{}, false
	}

	layouts := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		time.RFC3339,
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			y, m, d := parsed.Date()
			return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

func parseTriggerDateTimeValue(value interface{}) (time.Time, bool) {
	if value == nil {
		return time.Time{}, false
	}
	if typed, ok := value.(time.Time); ok {
		return typed, true
	}
	raw := strings.TrimSpace(fmt.Sprint(value))
	if raw == "" {
		return time.Time{}, false
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func parseTriggerClockValue(value interface{}) (time.Time, bool) {
	if value == nil {
		return time.Time{}, false
	}
	raw := strings.TrimSpace(fmt.Sprint(value))
	if raw == "" {
		return time.Time{}, false
	}

	layouts := []string{
		"15:04",
		"15:04:05",
		time.Kitchen,
	}
	for _, layout := range layouts {
		parsed, err := time.Parse(layout, raw)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func isEmptyTriggerValue(value interface{}) bool {
	if value == nil {
		return true
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed) == ""
	case []interface{}:
		return len(typed) == 0
	case []string:
		return len(typed) == 0
	case map[string]interface{}:
		return len(typed) == 0
	default:
		return false
	}
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
	req, ok := h.requireIntegrationRequest(c)
	if !ok {
		return
	}

	wf, found := h.Store.GetWorkflow(req.WorkflowID)
	if !found {
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
	instID, deduped, err := h.Exec.FindOrStartInstanceByEmailMessage(wf, normalizedData, extractEmailMessageID(normalizedData), middleware.GetAuthorizationHeader(c))
	if err != nil {
		log.Printf("instance_handler.StartFromGmail failed workflow_id=%q org_id=%q: %v", wf.ID, wf.OrgID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "start failed"})
		return
	}
	if deduped {
		c.JSON(http.StatusOK, gin.H{"instance_id": instID})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"instance_id": instID})
}

func (h *InstanceHandler) requireIntegrationRequest(c *gin.Context) (integrationInstanceRequest, bool) {
	if strings.TrimSpace(h.IntegrationKey) == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "integration key not configured"})
		return integrationInstanceRequest{}, false
	}

	header := c.GetHeader("X-Integration-Key")
	if subtle.ConstantTimeCompare([]byte(header), []byte(h.IntegrationKey)) != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid integration key"})
		return integrationInstanceRequest{}, false
	}

	var req integrationInstanceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid JSON"})
		return integrationInstanceRequest{}, false
	}
	req.OrgID = strings.TrimSpace(req.OrgID)
	req.WorkflowID = strings.TrimSpace(req.WorkflowID)
	if req.OrgID == "" || req.WorkflowID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org_id and workflow_id are required"})
		return integrationInstanceRequest{}, false
	}
	if req.Data == nil {
		req.Data = map[string]interface{}{}
	}

	return req, true
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

// POST /api/orgs/:orgId/instances/:id/restart
func (h *InstanceHandler) Restart(c *gin.Context) {
	orgID := c.Param("orgId")
	instanceID := c.Param("id")

	inst, ok := h.Store.GetInstance(instanceID)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if inst.OrgID != orgID {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	if inst.Status != models.InstanceFailed {
		c.JSON(http.StatusConflict, gin.H{"error": "instance is not failed"})
		return
	}
	if h.Exec == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "executor not configured"})
		return
	}

	restarted, err := h.Exec.RestartFailedInstance(instanceID, middleware.GetAuthorizationHeader(c))
	if err != nil {
		switch err {
		case executor.ErrInstanceNotFound, executor.ErrWorkflowNotFound, executor.ErrFailedNodeNotFound:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "restart failed"})
		case executor.ErrInstanceNotFailed:
			c.JSON(http.StatusConflict, gin.H{"error": "instance is not failed"})
		default:
			log.Printf("instance_handler.Restart failed instance_id=%q org_id=%q: %v", instanceID, orgID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "restart failed"})
		}
		return
	}

	c.JSON(http.StatusOK, restarted)
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
	includeNodeState := strings.EqualFold(strings.TrimSpace(c.Query("include_node_state")), "true")

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
			var nodeStates map[string]models.NodeState
			if includeNodeState {
				nodeStates = inst.NodeStates
			}
			compactOut = append(compactOut, compactInstance{
				ID:           inst.ID,
				OrgID:        inst.OrgID,
				WorkflowID:   inst.WorkflowID,
				WorkflowName: wfName,
				Status:       inst.Status,
				CurrentNode:  inst.CurrentNode,
				NodeStates:   nodeStates,
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
