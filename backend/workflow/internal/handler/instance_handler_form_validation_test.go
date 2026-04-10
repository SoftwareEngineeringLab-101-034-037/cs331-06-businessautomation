package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/executor"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/models"
)

func testIntegrationStartRequest(t *testing.T, r *gin.Engine, body []byte, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/integrations/google-forms/events", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Integration-Key", key)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestStartFromGoogleForms_TriggerFieldSchemaValidation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := newHandlerStore()
	wf := models.Workflow{
		ID:     "wf-form",
		OrgID:  "org-1",
		Name:   "Form Trigger",
		Status: models.WorkflowActive,
		Trigger: models.Trigger{
			Type: models.TriggerFormSubmit,
			Config: map[string]string{
				"form_id":       "form-1",
				"field_mapping": "q_amount:amount, q_urgent:is_urgent, q_date:requested_date",
				"field_schema": `[
					{"question_id":"q_amount","title":"Amount","required":true,"field_type":"text","variable":"amount","data_type":"number"},
					{"question_id":"q_urgent","title":"Urgent","required":false,"field_type":"text","variable":"is_urgent","data_type":"boolean"},
					{"question_id":"q_date","title":"Requested Date","required":false,"field_type":"date","variable":"requested_date","data_type":"date"}
				]`,
			},
		},
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "end"},
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}
	store.workflows[wf.ID] = wf

	exec := executor.NewExecutor(store, &noopEmail{}, nil)
	h := NewInstanceHandler(store, exec, "integration-secret")

	r := gin.New()
	r.POST("/integrations/google-forms/events", h.StartFromGoogleForms)

	t.Run("rejects non-numeric value for number field", func(t *testing.T) {
		rec := testIntegrationStartRequest(t, r, []byte(`{
			"org_id":"org-1",
			"workflow_id":"wf-form",
			"data":{
				"_form_id":"form-1",
				"_response_id":"resp-bad-number",
				"q_amount":"abc"
			}
		}`), "integration-secret")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "must be number") {
			t.Fatalf("expected number validation error, got %s", rec.Body.String())
		}
	})

	t.Run("rejects missing required mapped field", func(t *testing.T) {
		rec := testIntegrationStartRequest(t, r, []byte(`{
			"org_id":"org-1",
			"workflow_id":"wf-form",
			"data":{
				"_form_id":"form-1",
				"_response_id":"resp-missing-required"
			}
		}`), "integration-secret")
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
		}
		if !strings.Contains(strings.ToLower(rec.Body.String()), "required") {
			t.Fatalf("expected required field validation error, got %s", rec.Body.String())
		}
	})

	t.Run("coerces valid values and starts instance", func(t *testing.T) {
		rec := testIntegrationStartRequest(t, r, []byte(`{
			"org_id":"org-1",
			"workflow_id":"wf-form",
			"data":{
				"_form_id":"form-1",
				"_response_id":"resp-good",
				"q_amount":"42.5",
				"q_urgent":"yes",
				"q_date":"2026-04-10"
			}
		}`), "integration-secret")
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
		}

		var resp map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		instID := strings.TrimSpace(resp["instance_id"])
		if instID == "" {
			t.Fatalf("expected instance_id in response")
		}

		inst, ok := store.GetInstance(instID)
		if !ok {
			t.Fatalf("expected persisted instance %q", instID)
		}

		amount, ok := inst.Data["amount"].(float64)
		if !ok {
			t.Fatalf("expected amount to be float64, got %T (%v)", inst.Data["amount"], inst.Data["amount"])
		}
		if amount != 42.5 {
			t.Fatalf("expected amount=42.5, got %v", amount)
		}

		urgent, ok := inst.Data["is_urgent"].(bool)
		if !ok || !urgent {
			t.Fatalf("expected is_urgent=true bool, got %T (%v)", inst.Data["is_urgent"], inst.Data["is_urgent"])
		}

		requestedDate, ok := inst.Data["requested_date"].(string)
		if !ok || requestedDate != "2026-04-10" {
			t.Fatalf("expected requested_date=2026-04-10 string, got %T (%v)", inst.Data["requested_date"], inst.Data["requested_date"])
		}
	})
}
