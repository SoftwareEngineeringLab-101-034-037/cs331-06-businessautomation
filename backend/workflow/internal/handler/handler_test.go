package handler

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/executor"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/middleware"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/models"
)

type handlerStore struct {
	mu sync.RWMutex

	workflows map[string]models.Workflow
	instances map[string]models.Instance
	tasks     map[string]models.TaskAssignment

	nextWorkflowID int
	nextInstanceID int
	nextTaskID     int

	saveWorkflowErr        error
	listWorkflowsErr       error
	deleteWorkflowErr      error
	saveInstanceErr        error
	listTasksByRoleErr     error
	listTasksByInstanceErr error
}

func newHandlerStore() *handlerStore {
	return &handlerStore{
		workflows:      make(map[string]models.Workflow),
		instances:      make(map[string]models.Instance),
		tasks:          make(map[string]models.TaskAssignment),
		nextWorkflowID: 1,
		nextInstanceID: 1,
		nextTaskID:     1,
	}
}

func (s *handlerStore) SaveWorkflow(w models.Workflow) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveWorkflowErr != nil {
		return "", s.saveWorkflowErr
	}
	if w.ID == "" {
		w.ID = fmt.Sprintf("wf-%d", s.nextWorkflowID)
		s.nextWorkflowID++
	}
	s.workflows[w.ID] = w
	return w.ID, nil
}

func (s *handlerStore) GetWorkflow(id string) (models.Workflow, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wf, ok := s.workflows[id]
	return wf, ok
}

func (s *handlerStore) GetWorkflowsByIDs(ids []string) (map[string]models.Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]models.Workflow, len(ids))
	for _, id := range ids {
		if wf, ok := s.workflows[id]; ok {
			out[id] = wf
		}
	}
	return out, nil
}

func (s *handlerStore) ListWorkflows(orgID string) ([]models.Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listWorkflowsErr != nil {
		return nil, s.listWorkflowsErr
	}
	out := make([]models.Workflow, 0)
	for _, wf := range s.workflows {
		if wf.OrgID == orgID {
			out = append(out, wf)
		}
	}
	return out, nil
}

func (s *handlerStore) DeleteWorkflow(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deleteWorkflowErr != nil {
		return s.deleteWorkflowErr
	}
	if _, ok := s.workflows[id]; !ok {
		return errors.New("not found")
	}
	delete(s.workflows, id)
	return nil
}

func (s *handlerStore) SaveInstance(inst models.Instance) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.saveInstanceErr != nil {
		return "", s.saveInstanceErr
	}
	if inst.ID == "" {
		inst.ID = fmt.Sprintf("inst-%d", s.nextInstanceID)
		s.nextInstanceID++
	}
	s.instances[inst.ID] = inst
	return inst.ID, nil
}

func (s *handlerStore) GetInstance(id string) (models.Instance, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	inst, ok := s.instances[id]
	return inst, ok
}

func (s *handlerStore) FindInstanceByWorkflowAndFormResponse(workflowID, formResponseID string) (models.Instance, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, inst := range s.instances {
		if inst.WorkflowID != workflowID || inst.Data == nil {
			continue
		}
		value, ok := inst.Data["form_response_id"]
		if !ok || value == nil {
			continue
		}
		if fmt.Sprint(value) == formResponseID {
			return inst, true, nil
		}
	}
	return models.Instance{}, false, nil
}

func (s *handlerStore) ListInstancesByWorkflow(workflowID string) ([]models.Instance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Instance, 0)
	for _, inst := range s.instances {
		if inst.WorkflowID == workflowID {
			out = append(out, inst)
		}
	}
	return out, nil
}

func (s *handlerStore) ListInstancesByOrg(orgID string) ([]models.Instance, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Instance, 0)
	for _, inst := range s.instances {
		if inst.OrgID == orgID {
			out = append(out, inst)
		}
	}
	return out, nil
}

func (s *handlerStore) SaveTask(task models.TaskAssignment) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if task.ID == "" {
		task.ID = fmt.Sprintf("task-%d", s.nextTaskID)
		s.nextTaskID++
	}
	s.tasks[task.ID] = task
	return task.ID, nil
}

func (s *handlerStore) GetTask(id string) (models.TaskAssignment, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[id]
	return task, ok
}

func (s *handlerStore) CompareAndSwapTask(task models.TaskAssignment, expectedStatus models.TaskStatus) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.tasks[task.ID]
	if !ok || current.Status != expectedStatus {
		return false, nil
	}
	s.tasks[task.ID] = task
	return true, nil
}

func (s *handlerStore) HasActiveTasks(instanceID string) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, task := range s.tasks {
		if task.InstanceID != instanceID {
			continue
		}
		switch task.Status {
		case models.TaskPending, models.TaskInProgress, models.TaskClarify:
			return true, nil
		}
	}
	return false, nil
}

type mockTaskExecutor struct {
	mu            sync.Mutex
	calls         []taskContinueCall
	responseTask  models.TaskAssignment
	responseError error
	authError     error
}

type taskContinueCall struct {
	taskID      string
	actorUserID string
	action      string
	comment     string
	authHeader  string
}

func (m *mockTaskExecutor) ContinueTask(taskID, actorUserID, action, comment, authHeader string) (models.TaskAssignment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, taskContinueCall{
		taskID:      taskID,
		actorUserID: actorUserID,
		action:      action,
		comment:     comment,
		authHeader:  authHeader,
	})
	if m.responseError != nil {
		return models.TaskAssignment{}, m.responseError
	}
	return m.responseTask, nil
}

func (m *mockTaskExecutor) CanActOnTask(actorUserID string, task models.TaskAssignment, action, authHeader string) error {
	if m.authError != nil {
		return m.authError
	}
	return nil
}

func (s *handlerStore) ListTasksByAssignee(orgID, userID string) ([]models.TaskAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.TaskAssignment, 0)
	for _, task := range s.tasks {
		if task.OrgID == orgID && task.AssignedUser == userID {
			out = append(out, task)
		}
	}
	return out, nil
}

func (s *handlerStore) ListTasksByRole(orgID, role string) ([]models.TaskAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listTasksByRoleErr != nil {
		return nil, s.listTasksByRoleErr
	}
	out := make([]models.TaskAssignment, 0)
	for _, task := range s.tasks {
		if task.OrgID == orgID && task.AssignedRole == role {
			out = append(out, task)
		}
	}
	return out, nil
}

func (s *handlerStore) ListTasksByInstance(instanceID string) ([]models.TaskAssignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.listTasksByInstanceErr != nil {
		return nil, s.listTasksByInstanceErr
	}
	out := make([]models.TaskAssignment, 0)
	for _, task := range s.tasks {
		if task.InstanceID == instanceID {
			out = append(out, task)
		}
	}
	return out, nil
}

type noopEmail struct{}

func (n *noopEmail) Send(to, subject, body string) error { return nil }

func testRequest(t *testing.T, r *gin.Engine, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer test-token")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestWorkflowHandlerCreate(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := newHandlerStore()
	h := NewWorkflowHandler(store)

	r := gin.New()
	r.POST("/api/orgs/:orgId/workflows", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, "user-1")
		h.Create(c)
	})

	rec := testRequest(t, r, http.MethodPost, "/api/orgs/org-1/workflows", []byte("{"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid JSON, got %d", rec.Code)
	}

	rec = testRequest(t, r, http.MethodPost, "/api/orgs/org-1/workflows", []byte(`{"trigger":{"type":"manual"}}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing name, got %d", rec.Code)
	}

	rec = testRequest(t, r, http.MethodPost, "/api/orgs/org-1/workflows", []byte(`{"name":"WF","trigger":{"type":"manual"}}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp struct {
		ID       string          `json:"id"`
		Workflow models.Workflow `json:"workflow"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.ID == "" {
		t.Fatalf("expected workflow id")
	}
	if resp.Workflow.OrgID != "org-1" || resp.Workflow.CreatedBy != "user-1" {
		t.Fatalf("unexpected workflow metadata: %+v", resp.Workflow)
	}
	if resp.Workflow.Status != models.WorkflowActive {
		t.Fatalf("expected default status active, got %s", resp.Workflow.Status)
	}
	if resp.Workflow.Version != 1 {
		t.Fatalf("expected default version 1, got %d", resp.Workflow.Version)
	}
}

func TestWorkflowHandlerListGetUpdateDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := newHandlerStore()
	existing := models.Workflow{
		ID:        "wf-1",
		OrgID:     "org-1",
		Name:      "Old",
		Status:    models.WorkflowActive,
		Version:   2,
		CreatedBy: "creator-1",
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now().Add(-time.Minute),
		Trigger:   models.Trigger{Type: models.TriggerManual},
	}
	store.workflows[existing.ID] = existing

	// workflow owned by a different org — used for cross-org isolation checks below
	crossOrg := models.Workflow{
		ID:      "wf-x",
		OrgID:   "org-2",
		Name:    "Foreign",
		Status:  models.WorkflowActive,
		Version: 5,
		Trigger: models.Trigger{Type: models.TriggerManual},
	}
	store.workflows[crossOrg.ID] = crossOrg

	h := NewWorkflowHandler(store)
	r := gin.New()
	r.GET("/api/orgs/:orgId/workflows", h.List)
	r.GET("/api/orgs/:orgId/workflows/:id", h.Get)
	r.PUT("/api/orgs/:orgId/workflows/:id", h.Update)
	r.DELETE("/api/orgs/:orgId/workflows/:id", h.Delete)

	rec := testRequest(t, r, http.MethodGet, "/api/orgs/org-1/workflows", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for list, got %d", rec.Code)
	}
	var listed []models.Workflow
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	for _, wf := range listed {
		if wf.ID == crossOrg.ID {
			t.Fatalf("org-2 workflow %q must not appear in org-1 list", crossOrg.ID)
		}
	}

	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/workflows/wf-1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get, got %d", rec.Code)
	}

	updateBody := []byte(`{
		"name":"Updated",
		"status":"active",
		"version":1,
		"trigger":{"type":"manual"},
		"commit_message":"update"
	}`)
	rec = testRequest(t, r, http.MethodPut, "/api/orgs/org-1/workflows/wf-1", updateBody)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for update, got %d body=%s", rec.Code, rec.Body.String())
	}
	updated, ok := store.GetWorkflow("wf-1")
	if !ok {
		t.Fatalf("updated workflow not found")
	}
	if updated.Version != 3 {
		t.Fatalf("expected version increment to 3, got %d", updated.Version)
	}
	if updated.CreatedBy != "creator-1" || !updated.CreatedAt.Equal(existing.CreatedAt) {
		t.Fatalf("expected created metadata preserved, got %+v", updated)
	}

	rec = testRequest(t, r, http.MethodDelete, "/api/orgs/org-1/workflows/wf-1", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for delete, got %d", rec.Code)
	}

	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/workflows/wf-1", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted workflow, got %d", rec.Code)
	}

	// ── cross-org isolation ────────────────────────────────────────────────────
	// Every operation under /api/orgs/org-1/... must be denied for data owned
	// by org-2 and must never mutate that data.

	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/workflows/wf-x", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-org Get: expected 404, got %d", rec.Code)
	}

	crossUpdateBody := []byte(`{"name":"Hijacked","status":"active","version":1,"trigger":{"type":"manual"}}`)
	rec = testRequest(t, r, http.MethodPut, "/api/orgs/org-1/workflows/wf-x", crossUpdateBody)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-org Update: expected 404, got %d", rec.Code)
	}
	if after, ok := store.GetWorkflow("wf-x"); !ok || after.Name != crossOrg.Name || after.Version != crossOrg.Version {
		t.Fatalf("cross-org Update must not mutate wf-x: %+v", after)
	}

	rec = testRequest(t, r, http.MethodDelete, "/api/orgs/org-1/workflows/wf-x", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-org Delete: expected 404, got %d", rec.Code)
	}
	if _, ok := store.GetWorkflow("wf-x"); !ok {
		t.Fatalf("cross-org Delete must not remove wf-x")
	}
}

func TestWorkflowHandlerErrorsAndDraftVersion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := newHandlerStore()
	h := NewWorkflowHandler(store)
	r := gin.New()
	r.GET("/api/orgs/:orgId/workflows", h.List)
	r.POST("/api/orgs/:orgId/workflows", h.Create)
	r.PUT("/api/orgs/:orgId/workflows/:id", h.Update)
	r.DELETE("/api/orgs/:orgId/workflows/:id", h.Delete)

	store.listWorkflowsErr = errors.New("list failed")
	rec := testRequest(t, r, http.MethodGet, "/api/orgs/org-1/workflows", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for list error, got %d", rec.Code)
	}
	store.listWorkflowsErr = nil

	store.saveWorkflowErr = errors.New("save failed")
	rec = testRequest(t, r, http.MethodPost, "/api/orgs/org-1/workflows", []byte(`{"name":"WF","trigger":{"type":"manual"}}`))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for create save error, got %d", rec.Code)
	}
	store.saveWorkflowErr = nil

	store.workflows["wf-new"] = models.Workflow{
		ID:        "wf-new",
		OrgID:     "org-1",
		Name:      "Seed",
		Status:    models.WorkflowActive,
		Version:   1,
		CreatedAt: time.Now().Add(-time.Hour),
		UpdatedAt: time.Now().Add(-time.Minute),
		Trigger:   models.Trigger{Type: models.TriggerManual},
	}
	rec = testRequest(t, r, http.MethodPut, "/api/orgs/org-1/workflows/wf-new", []byte("{"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid update json, got %d body=%s", rec.Code, rec.Body.String())
	}

	store.workflows["wf-draft"] = models.Workflow{
		ID:        "wf-draft",
		OrgID:     "org-1",
		Name:      "Seed Draft",
		Status:    models.WorkflowActive,
		Version:   2,
		CreatedAt: time.Now().Add(-2 * time.Hour),
		UpdatedAt: time.Now().Add(-time.Hour),
		Trigger:   models.Trigger{Type: models.TriggerManual},
	}
	rec = testRequest(t, r, http.MethodPut, "/api/orgs/org-1/workflows/wf-draft", []byte(`{
		"name":"Draft",
		"status":"draft",
		"version":100,
		"trigger":{"type":"manual"}
	}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for draft update, got %d", rec.Code)
	}
	draft, ok := store.GetWorkflow("wf-draft")
	if !ok {
		t.Fatalf("draft workflow not saved")
	}
	if draft.Version != 0 {
		t.Fatalf("expected draft version 0, got %d", draft.Version)
	}

	rec = testRequest(t, r, http.MethodDelete, "/api/orgs/org-1/workflows/not-found", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing delete, got %d", rec.Code)
	}

	// cross-org: Update and Delete under org-1 on an org-2 workflow must both fail
	// and must leave the record untouched.
	store.workflows["wf-cross"] = models.Workflow{
		ID:      "wf-cross",
		OrgID:   "org-2",
		Name:    "CrossOrgWF",
		Status:  models.WorkflowActive,
		Version: 3,
		Trigger: models.Trigger{Type: models.TriggerManual},
	}

	rec = testRequest(t, r, http.MethodPut, "/api/orgs/org-1/workflows/wf-cross",
		[]byte(`{"name":"Hijacked","status":"active","trigger":{"type":"manual"}}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-org Update (errors test): expected 404, got %d", rec.Code)
	}
	crossWF, _ := store.GetWorkflow("wf-cross")
	if crossWF.Name != "CrossOrgWF" || crossWF.Version != 3 {
		t.Fatalf("cross-org Update must not mutate wf-cross: %+v", crossWF)
	}

	rec = testRequest(t, r, http.MethodDelete, "/api/orgs/org-1/workflows/wf-cross", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-org Delete (errors test): expected 404, got %d", rec.Code)
	}
	if _, ok := store.GetWorkflow("wf-cross"); !ok {
		t.Fatalf("cross-org Delete must not remove wf-cross")
	}
}

func TestInstanceHandlerStartAndGet(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := newHandlerStore()
	activeWF := models.Workflow{
		ID:      "wf-active",
		OrgID:   "org-1",
		Name:    "Active",
		Status:  models.WorkflowActive,
		Trigger: models.Trigger{Type: models.TriggerManual},
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "end"},
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}
	inactiveWF := activeWF
	inactiveWF.ID = "wf-inactive"
	inactiveWF.Status = models.WorkflowDraft
	store.workflows[activeWF.ID] = activeWF
	store.workflows[inactiveWF.ID] = inactiveWF

	exec := executor.NewExecutor(store, &noopEmail{}, nil)
	h := NewInstanceHandler(store, exec)

	r := gin.New()
	r.POST("/api/orgs/:orgId/instances", h.Start)
	r.GET("/api/orgs/:orgId/instances/:id", h.Get)

	rec := testRequest(t, r, http.MethodPost, "/api/orgs/org-1/instances", []byte("{"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid json, got %d", rec.Code)
	}

	rec = testRequest(t, r, http.MethodPost, "/api/orgs/org-1/instances", []byte(`{"workflow_id":"missing"}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing workflow, got %d", rec.Code)
	}

	rec = testRequest(t, r, http.MethodPost, "/api/orgs/org-1/instances", []byte(`{"workflow_id":"wf-inactive"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for inactive workflow, got %d", rec.Code)
	}

	store.saveInstanceErr = errors.New("db down")
	rec = testRequest(t, r, http.MethodPost, "/api/orgs/org-1/instances", []byte(`{"workflow_id":"wf-active","data":{"x":1}}`))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for start error, got %d", rec.Code)
	}
	store.saveInstanceErr = nil

	rec = testRequest(t, r, http.MethodPost, "/api/orgs/org-1/instances", []byte(`{"workflow_id":"wf-active","data":{"x":1}}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 for start success, got %d body=%s", rec.Code, rec.Body.String())
	}
	var startResp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &startResp); err != nil {
		t.Fatalf("decode start response failed: %v", err)
	}
	instanceID := startResp["instance_id"]
	if instanceID == "" {
		t.Fatalf("expected instance_id in response")
	}

	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/instances/missing", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing instance, got %d", rec.Code)
	}

	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/instances/"+instanceID, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for get instance, got %d", rec.Code)
	}
}

func TestTaskHandlerListAndAction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	store := newHandlerStore()
	exec := &mockTaskExecutor{
		responseTask: models.TaskAssignment{
			ID:              "task-1",
			OrgID:           "org-1",
			Status:          models.TaskCompleted,
			ActionCommitted: "approve",
			Comment:         "approved",
			CreatedAt:       time.Now(),
		},
	}
	store.tasks["task-1"] = models.TaskAssignment{
		ID:           "task-1",
		OrgID:        "org-1",
		InstanceID:   "inst-1",
		AssignedRole: "manager",
		AssignedUser: "user-1",
		Status:       models.TaskPending,
		CreatedAt:    time.Now(),
	}
	// task owned by a different org — used for cross-org isolation checks below
	store.tasks["task-x"] = models.TaskAssignment{
		ID:           "task-x",
		OrgID:        "org-2",
		InstanceID:   "inst-cross",
		AssignedRole: "manager",
		Status:       models.TaskPending,
		CreatedAt:    time.Now(),
	}

	h := NewTaskHandler(store, exec)
	legacyHandler := NewTaskHandler(store, nil)
	r := gin.New()
	r.GET("/api/orgs/:orgId/tasks", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, "user-1")
		h.List(c)
	})
	r.PUT("/api/orgs/:orgId/tasks/:id/:action", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, "user-1")
		h.Action(c)
	})
	legacyRouter := gin.New()
	legacyRouter.PUT("/api/orgs/:orgId/tasks/:id/:action", func(c *gin.Context) {
		c.Set(middleware.UserIDKey, "user-1")
		legacyHandler.Action(c)
	})

	rec := testRequest(t, r, http.MethodGet, "/api/orgs/org-1/tasks", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing query, got %d", rec.Code)
	}

	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/tasks?assigned_user=someone-else", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for assigned user list, got %d", rec.Code)
	}
	var myTasks []models.TaskAssignment
	if err := json.Unmarshal(rec.Body.Bytes(), &myTasks); err != nil {
		t.Fatalf("decode assigned user list failed: %v", err)
	}
	if len(myTasks) != 1 || myTasks[0].ID != "task-1" {
		t.Fatalf("expected only current user's task, got %+v", myTasks)
	}

	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/tasks?role=manager", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for role list, got %d", rec.Code)
	}

	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/tasks?role=auditor", nil)
	if rec.Code != http.StatusOK || rec.Body.String() != "[]" {
		t.Fatalf("expected empty array for no tasks, got code=%d body=%s", rec.Code, rec.Body.String())
	}

	store.listTasksByRoleErr = errors.New("list role failed")
	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/tasks?role=manager", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for role list error, got %d", rec.Code)
	}
	store.listTasksByRoleErr = nil

	store.listTasksByInstanceErr = errors.New("list instance failed")
	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/tasks?instance_id=inst-1", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 for instance list error, got %d", rec.Code)
	}
	store.listTasksByInstanceErr = nil

	rec = testRequest(t, r, http.MethodPut, "/api/orgs/org-1/tasks/missing/approve", []byte(`{"comment":"ok"}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing task action, got %d", rec.Code)
	}

	rec = testRequest(t, r, http.MethodPut, "/api/orgs/org-1/tasks/task-1/approve", []byte(`{`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed json, got %d", rec.Code)
	}

	rec = testRequest(t, legacyRouter, http.MethodPut, "/api/orgs/org-1/tasks/task-1/start", []byte(`{"comment":""}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for start without comment, got %d body=%s", rec.Code, rec.Body.String())
	}
	startedTask, ok := store.GetTask("task-1")
	if !ok {
		t.Fatalf("expected started task to exist")
	}
	if startedTask.Status != models.TaskInProgress {
		t.Fatalf("expected in_progress after start, got %s", startedTask.Status)
	}
	if startedTask.AssignedUser != "user-1" {
		t.Fatalf("expected task to be claimed by current user, got %q", startedTask.AssignedUser)
	}

	rec = testRequest(t, legacyRouter, http.MethodPut, "/api/orgs/org-1/tasks/task-1/unknown", []byte(`{"comment":"x"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown action, got %d", rec.Code)
	}

	// ── cross-org isolation ────────────────────────────────────────────────────
	// Action on a task owned by org-2 must return 404 and leave it unchanged.
	rec = testRequest(t, r, http.MethodPut, "/api/orgs/org-1/tasks/task-x/approve", []byte(`{"comment":"hijack"}`))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("cross-org task Action: expected 404, got %d", rec.Code)
	}
	if crossTask, ok := store.GetTask("task-x"); !ok || crossTask.Status != models.TaskPending {
		t.Fatalf("cross-org task Action must not mutate task-x: %+v", crossTask)
	}

	// ListTasksByInstance for an org-2 instance queried under org-1 must return empty.
	rec = testRequest(t, r, http.MethodGet, "/api/orgs/org-1/tasks?instance_id=inst-cross", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("cross-org instance list: expected 200, got %d", rec.Code)
	}
	var crossTasks []models.TaskAssignment
	if err := json.Unmarshal(rec.Body.Bytes(), &crossTasks); err != nil {
		t.Fatalf("decode cross-org instance list: %v", err)
	}
	for _, task := range crossTasks {
		if task.ID == "task-x" {
			t.Fatalf("org-2 task task-x must not appear in org-1 instance list")
		}
	}

	tests := []struct {
		action string
		want   models.TaskStatus
	}{
		{action: "approve", want: models.TaskCompleted},
		{action: "reject", want: models.TaskCompleted},
		{action: "clarify", want: models.TaskCompleted},
		{action: "complete", want: models.TaskCompleted},
	}
	for i, tt := range tests {
		taskID := fmt.Sprintf("task-action-%d", i)
		store.tasks[taskID] = models.TaskAssignment{
			ID:           taskID,
			OrgID:        "org-1",
			AssignedUser: "user-1",
			Status:       models.TaskInProgress,
			CreatedAt:    time.Now(),
		}

		rec = testRequest(t, legacyRouter, http.MethodPut, "/api/orgs/org-1/tasks/"+taskID+"/"+tt.action, []byte(`{"comment":"done"}`))
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 for action %s, got %d", tt.action, rec.Code)
		}

		updated, ok := store.GetTask(taskID)
		if !ok {
			t.Fatalf("task %s not found after action", taskID)
		}
		if updated.Status != tt.want {
			t.Fatalf("unexpected status for action %s: got %s want %s", tt.action, updated.Status, tt.want)
		}
		if updated.ActionCommitted != tt.action {
			t.Fatalf("unexpected action committed for action %s: got %q want %q", tt.action, updated.ActionCommitted, tt.action)
		}
		if updated.Comment != "done" || updated.CompletedAt == nil {
			t.Fatalf("expected comment/completion timestamp to be set: %+v", updated)
		}
	}

	rec = testRequest(t, r, http.MethodPut, "/api/orgs/org-1/tasks/task-1/approve", []byte(`{"comment":"approved"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for executor-backed action, got %d body=%s", rec.Code, rec.Body.String())
	}
	if len(exec.calls) != 1 {
		t.Fatalf("expected executor ContinueTask to be called once, got %d", len(exec.calls))
	}
	if exec.calls[0].taskID != "task-1" || exec.calls[0].actorUserID != "user-1" || exec.calls[0].action != "approve" || exec.calls[0].comment != "approved" || exec.calls[0].authHeader == "" {
		t.Fatalf("unexpected executor call: %+v", exec.calls[0])
	}
}
