package executor

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/models"
)

type emailCall struct {
	to      string
	subject string
	body    string
}

type mockEmail struct {
	mu      sync.Mutex
	calls   []emailCall
	sendErr error
}

type mockOrgEmail struct {
	mockEmail
	sendForOrgErr error
}

type fixedRoleMemberSelectionStrategy struct {
	selectedUserID string
}

func (s fixedRoleMemberSelectionStrategy) Select(memberIDs []string) string {
	_ = memberIDs
	return s.selectedUserID
}

type flipFlopRoleMemberSelectionStrategy struct {
	selectedUserID string
	callCount      int
}

func (s *flipFlopRoleMemberSelectionStrategy) Select(memberIDs []string) string {
	_ = memberIDs
	s.callCount++
	if s.callCount == 1 {
		return ""
	}
	return s.selectedUserID
}

func (m *mockEmail) Send(to, subject, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, emailCall{to: to, subject: subject, body: body})
	if m.sendErr != nil {
		return m.sendErr
	}
	return nil
}

func (m *mockEmail) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockEmail) first() emailCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return emailCall{}
	}
	return m.calls[0]
}

func (m *mockOrgEmail) SendForOrg(orgID, to, cc, subject, body, fromName, fromAccountID string) error {
	_ = orgID
	_ = cc
	_ = fromName
	_ = fromAccountID
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, emailCall{to: to, subject: subject, body: body})
	if m.sendForOrgErr != nil {
		return m.sendForOrgErr
	}
	return nil
}

type mockStore struct {
	mu sync.RWMutex

	workflows map[string]models.Workflow
	instances map[string]models.Instance
	tasks     map[string]models.TaskAssignment

	nextWorkflowID int
	nextInstanceID int
	nextTaskID     int

	saveWorkflowErr error
	saveInstanceErr error
	saveTaskErr     error
	listWorkflowErr error
	listTaskErr     error
}

func newMockStore() *mockStore {
	return &mockStore{
		workflows:      make(map[string]models.Workflow),
		instances:      make(map[string]models.Instance),
		tasks:          make(map[string]models.TaskAssignment),
		nextWorkflowID: 1,
		nextInstanceID: 1,
		nextTaskID:     1,
	}
}

func (m *mockStore) SaveWorkflow(w models.Workflow) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveWorkflowErr != nil {
		return "", m.saveWorkflowErr
	}
	if w.ID == "" {
		w.ID = fmt.Sprintf("wf-%d", m.nextWorkflowID)
		m.nextWorkflowID++
	}
	m.workflows[w.ID] = w
	return w.ID, nil
}

func (m *mockStore) GetWorkflow(id string) (models.Workflow, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	w, ok := m.workflows[id]
	return w, ok
}

func (m *mockStore) GetWorkflowsByIDs(ids []string) (map[string]models.Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]models.Workflow, len(ids))
	for _, id := range ids {
		if wf, ok := m.workflows[id]; ok {
			out[id] = wf
		}
	}
	return out, nil
}

func (m *mockStore) ListWorkflows(orgID string) ([]models.Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listWorkflowErr != nil {
		return nil, m.listWorkflowErr
	}
	var out []models.Workflow
	for _, wf := range m.workflows {
		if wf.OrgID == orgID {
			out = append(out, wf)
		}
	}
	return out, nil
}

func (m *mockStore) DeleteWorkflow(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.workflows[id]; !ok {
		return errors.New("not found")
	}
	delete(m.workflows, id)
	return nil
}

func (m *mockStore) SaveInstance(inst models.Instance) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveInstanceErr != nil {
		return "", m.saveInstanceErr
	}
	if inst.ID == "" {
		inst.ID = fmt.Sprintf("inst-%d", m.nextInstanceID)
		m.nextInstanceID++
	}
	m.instances[inst.ID] = inst
	return inst.ID, nil
}

func (m *mockStore) GetInstance(id string) (models.Instance, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inst, ok := m.instances[id]
	return inst, ok
}

func (m *mockStore) FindInstanceByWorkflowAndFormResponse(workflowID, formResponseID string) (models.Instance, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, inst := range m.instances {
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

func (m *mockStore) FindInstanceByWorkflowAndEmailMessageID(workflowID, emailMessageID string) (models.Instance, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, inst := range m.instances {
		if inst.WorkflowID != workflowID || inst.Data == nil {
			continue
		}
		value, ok := inst.Data["email_message_id"]
		if !ok || value == nil {
			continue
		}
		if fmt.Sprint(value) == emailMessageID {
			return inst, true, nil
		}
	}
	return models.Instance{}, false, nil
}

func (m *mockStore) ListInstancesByWorkflow(workflowID string) ([]models.Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []models.Instance
	for _, inst := range m.instances {
		if inst.WorkflowID == workflowID {
			out = append(out, inst)
		}
	}
	return out, nil
}

func (m *mockStore) ListInstancesByOrg(orgID string) ([]models.Instance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []models.Instance
	for _, inst := range m.instances {
		if inst.OrgID == orgID {
			out = append(out, inst)
		}
	}
	return out, nil
}

func (m *mockStore) SaveTask(t models.TaskAssignment) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveTaskErr != nil {
		return "", m.saveTaskErr
	}
	if t.ID == "" {
		t.ID = fmt.Sprintf("task-%d", m.nextTaskID)
		m.nextTaskID++
	}
	m.tasks[t.ID] = t
	return t.ID, nil
}

func (m *mockStore) GetTask(id string) (models.TaskAssignment, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[id]
	return task, ok
}

func (m *mockStore) CompareAndSwapTask(task models.TaskAssignment, expectedStatus models.TaskStatus) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.saveTaskErr != nil {
		return false, m.saveTaskErr
	}
	current, ok := m.tasks[task.ID]
	if !ok || current.Status != expectedStatus {
		return false, nil
	}
	m.tasks[task.ID] = task
	return true, nil
}

func (m *mockStore) HasActiveTasks(instanceID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, task := range m.tasks {
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

func (m *mockStore) ListTasksByAssignee(orgID, userID string) ([]models.TaskAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listTaskErr != nil {
		return nil, m.listTaskErr
	}
	var out []models.TaskAssignment
	for _, task := range m.tasks {
		if task.OrgID != orgID {
			continue
		}
		if task.AssignedUser == userID {
			out = append(out, task)
			continue
		}
		if task.Status == models.TaskPending {
			for _, allowedUser := range task.AssignedUsers {
				if allowedUser == userID {
					out = append(out, task)
					break
				}
			}
		}
	}
	return out, nil
}

func (m *mockStore) ListTasksByRole(orgID, role string) ([]models.TaskAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listTaskErr != nil {
		return nil, m.listTaskErr
	}
	var out []models.TaskAssignment
	for _, task := range m.tasks {
		if task.OrgID != orgID {
			continue
		}
		if task.AssignedRole == role {
			out = append(out, task)
			continue
		}
		for _, assignedRole := range task.AssignedRoles {
			if assignedRole == role {
				out = append(out, task)
				break
			}
		}
	}
	return out, nil
}

func (m *mockStore) ListTasksByRoles(orgID string, roles []string) ([]models.TaskAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listTaskErr != nil {
		return nil, m.listTaskErr
	}
	if len(roles) == 0 {
		return []models.TaskAssignment{}, nil
	}
	allowed := make(map[string]struct{}, len(roles))
	for _, role := range roles {
		allowed[role] = struct{}{}
	}
	var out []models.TaskAssignment
	for _, task := range m.tasks {
		if task.OrgID != orgID {
			continue
		}
		if task.Status != models.TaskPending {
			continue
		}
		if _, ok := allowed[task.AssignedRole]; ok {
			out = append(out, task)
			continue
		}
		for _, assignedRole := range task.AssignedRoles {
			if _, ok := allowed[assignedRole]; ok {
				out = append(out, task)
				break
			}
		}
	}
	return out, nil
}

func (m *mockStore) ListTasksByInstance(instanceID string) ([]models.TaskAssignment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.listTaskErr != nil {
		return nil, m.listTaskErr
	}
	var out []models.TaskAssignment
	for _, task := range m.tasks {
		if task.InstanceID == instanceID {
			out = append(out, task)
		}
	}
	return out, nil
}

func waitForInstanceStatus(t *testing.T, store *mockStore, instanceID string, want models.InstanceStatus) models.Instance {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		inst, ok := store.GetInstance(instanceID)
		if ok && inst.Status == want {
			return inst
		}
		time.Sleep(10 * time.Millisecond)
	}
	inst, _ := store.GetInstance(instanceID)
	t.Fatalf("timed out waiting for status %s, last=%s", want, inst.Status)
	return models.Instance{}
}

func countAuditAction(inst models.Instance, action string) int {
	count := 0
	for _, entry := range inst.AuditLog {
		if entry.Action == action {
			count++
		}
	}
	return count
}

func seedInstance(t *testing.T, store *mockStore, wf models.Workflow, data map[string]interface{}) string {
	t.Helper()
	id, err := store.SaveInstance(models.Instance{
		ID:         "seed-instance",
		WorkflowID: wf.ID,
		OrgID:      wf.OrgID,
		Status:     models.InstanceRunning,
		Data:       data,
		NodeStates: map[string]models.NodeState{},
		AuditLog:   []models.AuditEntry{},
		StartedAt:  time.Now(),
	})
	if err != nil {
		t.Fatalf("seed instance failed: %v", err)
	}
	return id
}

func TestResolveParam(t *testing.T) {
	data := map[string]interface{}{
		"name":  "Alice",
		"count": 3,
	}
	params := map[string]interface{}{
		"subject": "Hello {{data.name}}",
		"body":    "Count={{data.count}}",
	}

	if got := resolveParam(params, "subject", data); got != "Hello Alice" {
		t.Fatalf("unexpected subject: %q", got)
	}
	if got := resolveParam(params, "body", data); got != "Count=3" {
		t.Fatalf("unexpected body: %q", got)
	}
	if got := resolveParam(params, "missing", data); got != "" {
		t.Fatalf("expected empty string for missing key, got %q", got)
	}
	if got := resolveParam(nil, "x", data); got != "" {
		t.Fatalf("expected empty string for nil params, got %q", got)
	}
}

func TestEvalConditionForNodeStructured(t *testing.T) {
	e := NewExecutor(newMockStore(), &mockEmail{}, nil)
	wf := &models.Workflow{
		Trigger: models.Trigger{
			Type: models.TriggerFormSubmit,
			Config: map[string]string{
				"field_schema": `[
					{"question_id":"q_amount","title":"Amount","field_type":"scale","variable":"amount","data_type":"number"},
					{"question_id":"q_dept","title":"Department","field_type":"text","variable":"department","data_type":"text"}
				]`,
			},
		},
	}

	tests := []struct {
		name        string
		node        *models.WorkflowNode
		data        map[string]interface{}
		wantBranch  string
		wantJoin    string
		wantLogic   string
		wantReason  string
		wantMatched int
	}{
		{
			name: "single number rule",
			node: &models.WorkflowNode{ConditionConfig: &models.ConditionConfig{
				Join: models.ConditionJoinAnd,
				Rules: []models.ConditionRule{{
					Field:    "amount",
					DataType: models.ConditionDataTypeNumber,
					Operator: models.ConditionOperatorGreaterThan,
					Value:    "100",
				}},
			}},
			data:        map[string]interface{}{"amount": 120},
			wantBranch:  "yes",
			wantJoin:    "and",
			wantMatched: 1,
		},
		{
			name: "and combination with two matching rules",
			node: &models.WorkflowNode{ConditionConfig: &models.ConditionConfig{
				Join: models.ConditionJoinAnd,
				Rules: []models.ConditionRule{
					{Field: "amount", DataType: models.ConditionDataTypeNumber, Operator: models.ConditionOperatorGreaterEq, Value: "100"},
					{Field: "department", DataType: models.ConditionDataTypeText, Operator: models.ConditionOperatorContains, Value: "fin"},
				},
			}},
			data:        map[string]interface{}{"amount": 120, "department": "Finance"},
			wantBranch:  "yes",
			wantJoin:    "and",
			wantMatched: 2,
		},
		{
			name: "or combination with one matching rule",
			node: &models.WorkflowNode{ConditionConfig: &models.ConditionConfig{
				Join: models.ConditionJoinOr,
				Rules: []models.ConditionRule{
					{Field: "amount", DataType: models.ConditionDataTypeNumber, Operator: models.ConditionOperatorGreaterThan, Value: "1000"},
					{Field: "department", DataType: models.ConditionDataTypeText, Operator: models.ConditionOperatorContains, Value: "fin"},
				},
			}},
			data:        map[string]interface{}{"amount": 120, "department": "Finance"},
			wantBranch:  "yes",
			wantJoin:    "or",
			wantMatched: 1,
		},
		{
			name: "type mismatch routes no",
			node: &models.WorkflowNode{ConditionConfig: &models.ConditionConfig{
				Join: models.ConditionJoinAnd,
				Rules: []models.ConditionRule{{
					Field:    "amount",
					DataType: models.ConditionDataTypeNumber,
					Operator: models.ConditionOperatorGreaterThan,
					Value:    "100",
				}},
			}},
			data:       map[string]interface{}{"amount": "not-a-number"},
			wantBranch: "no",
			wantJoin:   "and",
			wantReason: "type_mismatch",
		},
		{
			name: "nested logic expression supports parentheses",
			node: &models.WorkflowNode{ConditionConfig: &models.ConditionConfig{
				Join:  models.ConditionJoinAnd,
				Logic: "1 AND 2 AND (3 OR 4)",
				Rules: []models.ConditionRule{
					{Field: "amount", DataType: models.ConditionDataTypeNumber, Operator: models.ConditionOperatorGreaterEq, Value: "100"},
					{Field: "department", DataType: models.ConditionDataTypeText, Operator: models.ConditionOperatorContains, Value: "fin"},
					{Field: "amount", DataType: models.ConditionDataTypeNumber, Operator: models.ConditionOperatorLessThan, Value: "100"},
					{Field: "department", DataType: models.ConditionDataTypeText, Operator: models.ConditionOperatorContains, Value: "fin"},
				},
			}},
			data:        map[string]interface{}{"amount": 120, "department": "Finance"},
			wantBranch:  "yes",
			wantJoin:    "and",
			wantLogic:   "1 AND 2 AND (3 OR 4)",
			wantMatched: 3,
		},
		{
			name: "invalid logic expression routes no",
			node: &models.WorkflowNode{ConditionConfig: &models.ConditionConfig{
				Join:  models.ConditionJoinAnd,
				Logic: "1 AND (2 OR 9)",
				Rules: []models.ConditionRule{
					{Field: "amount", DataType: models.ConditionDataTypeNumber, Operator: models.ConditionOperatorGreaterEq, Value: "100"},
					{Field: "department", DataType: models.ConditionDataTypeText, Operator: models.ConditionOperatorContains, Value: "fin"},
				},
			}},
			data:       map[string]interface{}{"amount": 120, "department": "Finance"},
			wantBranch: "no",
			wantJoin:   "and",
			wantLogic:  "1 AND (2 OR 9)",
			wantReason: "invalid_logic_expression",
		},
		{
			name:       "missing condition config routes no",
			node:       &models.WorkflowNode{},
			data:       map[string]interface{}{"amount": 100},
			wantBranch: "no",
			wantReason: "missing_condition_config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			branch, details := e.evalConditionForNode(tt.node, wf, tt.data)
			if branch != tt.wantBranch {
				t.Fatalf("branch=%q want=%q details=%v", branch, tt.wantBranch, details)
			}
			if mode := fmt.Sprintf("%v", details["mode"]); mode != "structured" {
				t.Fatalf("mode=%q want=%q details=%v", mode, "structured", details)
			}
			if tt.wantJoin != "" {
				if join := fmt.Sprintf("%v", details["join"]); join != tt.wantJoin {
					t.Fatalf("join=%q want=%q details=%v", join, tt.wantJoin, details)
				}
			}
			if tt.wantLogic != "" {
				if logic := fmt.Sprintf("%v", details["logic"]); logic != tt.wantLogic {
					t.Fatalf("logic=%q want=%q details=%v", logic, tt.wantLogic, details)
				}
			}
			if tt.wantReason != "" {
				if reason := fmt.Sprintf("%v", details["reason"]); reason != tt.wantReason {
					t.Fatalf("reason=%q want=%q details=%v", reason, tt.wantReason, details)
				}
			}
			if tt.wantMatched > 0 {
				if matched := fmt.Sprintf("%v", details["matched_rules"]); matched != fmt.Sprintf("%d", tt.wantMatched) {
					t.Fatalf("matched_rules=%q want=%d details=%v", matched, tt.wantMatched, details)
				}
			}
		})
	}
}

func TestToFloat(t *testing.T) {
	if got, ok := toFloat("12.5"); !ok || got != 12.5 {
		t.Fatalf("unexpected float: %v", got)
	}
	if got, ok := toFloat("bad"); ok || got != 0 {
		t.Fatalf("expected invalid float parse, got value=%v ok=%v", got, ok)
	}
}

func TestStartInstanceNoStartNodeMarksFailed(t *testing.T) {
	store := newMockStore()
	email := &mockEmail{}
	exec := NewExecutor(store, email, nil)

	wf := models.Workflow{
		ID:    "wf-no-start",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}

	instanceID, err := exec.StartInstance(wf, map[string]interface{}{}, "")
	if err != nil {
		t.Fatalf("StartInstance failed: %v", err)
	}

	inst := waitForInstanceStatus(t, store, instanceID, models.InstanceFailed)
	if countAuditAction(inst, "instance_failed") != 1 {
		t.Fatalf("expected one instance_failed audit entry, got %d", countAuditAction(inst, "instance_failed"))
	}
}

func TestRunLinearActionFlowCompletesAndSendsEmail(t *testing.T) {
	store := newMockStore()
	email := &mockEmail{}
	exec := NewExecutor(store, email, nil)

	wf := models.Workflow{
		ID:    "wf-linear",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "email"},
			{
				ID:    "email",
				Type:  models.NodeAction,
				Title: "Send Email",
				Connector: &models.ConnectorConfig{
					Type: models.ConnectorEmail,
					Params: map[string]interface{}{
						"to":            "{{data.recipient}}",
						"subject":       "Hi {{data.name}}",
						"body_template": "Welcome {{data.name}}",
					},
				},
				Next: "end",
			},
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}
	data := map[string]interface{}{
		"recipient": "alice@example.com",
		"name":      "Alice",
	}
	instanceID := seedInstance(t, store, wf, data)

	exec.run(instanceID, wf, data, "")

	inst, ok := store.GetInstance(instanceID)
	if !ok {
		t.Fatalf("instance not found")
	}
	if inst.Status != models.InstanceCompleted {
		t.Fatalf("expected completed status, got %s", inst.Status)
	}
	if inst.CompletedAt == nil {
		t.Fatalf("expected CompletedAt to be set")
	}
	if inst.NodeStates["start"].Status != "completed" || inst.NodeStates["email"].Status != "completed" || inst.NodeStates["end"].Status != "completed" {
		t.Fatalf("unexpected node states: %+v", inst.NodeStates)
	}
	if email.count() != 1 {
		t.Fatalf("expected 1 email call, got %d", email.count())
	}
	call := email.first()
	if call.to != "alice@example.com" || call.subject != "Hi Alice" || call.body != "Welcome Alice" {
		t.Fatalf("unexpected email payload: %+v", call)
	}
	if countAuditAction(inst, "email_sent") != 1 || countAuditAction(inst, "instance_completed") != 1 {
		t.Fatalf("missing expected audit events: %+v", inst.AuditLog)
	}
}

func TestRunConditionRoutesToNoBranch(t *testing.T) {
	store := newMockStore()
	exec := NewExecutor(store, &mockEmail{}, NewRandomRoleAssigneeSelector(NewStaticRoleMemberDirectory(map[string]map[string][]string{
		"org-1": {
			"manager": {"user-1"},
		},
	})))

	wf := models.Workflow{
		ID:    "wf-condition",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "cond"},
			{ID: "cond", Type: models.NodeCondition, Title: "Condition", Condition: "amount > 100", NextYes: "yes-end", NextNo: "no-end"},
			{ID: "yes-end", Type: models.NodeEnd, Title: "Yes"},
			{ID: "no-end", Type: models.NodeEnd, Title: "No"},
		},
	}
	data := map[string]interface{}{"amount": 20}
	instanceID := seedInstance(t, store, wf, data)

	exec.run(instanceID, wf, data, "")

	inst, _ := store.GetInstance(instanceID)
	if _, ok := inst.NodeStates["no-end"]; !ok {
		t.Fatalf("expected no-end to be visited")
	}
	if _, ok := inst.NodeStates["yes-end"]; ok {
		t.Fatalf("did not expect yes-end to be visited")
	}

	foundNoBranch := false
	for _, entry := range inst.AuditLog {
		if entry.Action == "condition_evaluated" && entry.Details["branch"] == "no" {
			foundNoBranch = true
			break
		}
	}
	if !foundNoBranch {
		t.Fatalf("expected condition_evaluated audit with no branch: %+v", inst.AuditLog)
	}
}

func TestRunTaskCreatesAssignmentAndBranchesByAction(t *testing.T) {
	store := newMockStore()
	exec := NewExecutor(store, &mockEmail{}, NewRandomRoleAssigneeSelector(NewStaticRoleMemberDirectory(map[string]map[string][]string{
		"org-1": {
			"manager": {"user-1"},
		},
	})))

	wf := models.Workflow{
		ID:    "wf-task",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "task"},
			{
				ID:           "task",
				Type:         models.NodeTask,
				Title:        "Manager Approval",
				AssignedRole: "manager",
				TaskActions:  []string{"approve", "reject"},
				NextActions: map[string]string{
					"approve": "approved-end",
					"reject":  "rejected-end",
				},
				Next: "fallback-end",
			},
			{ID: "approved-end", Type: models.NodeEnd, Title: "Approved"},
			{ID: "rejected-end", Type: models.NodeEnd, Title: "Rejected"},
			{ID: "fallback-end", Type: models.NodeEnd, Title: "Fallback"},
		},
	}
	store.workflows[wf.ID] = wf
	data := map[string]interface{}{"request_id": "r-1"}
	instanceID := seedInstance(t, store, wf, data)

	exec.run(instanceID, wf, data, "")

	inst, _ := store.GetInstance(instanceID)
	if inst.Status != models.InstanceWaiting {
		t.Fatalf("expected waiting status after task creation, got %s", inst.Status)
	}
	if _, ok := inst.NodeStates["approved-end"]; ok {
		t.Fatalf("did not expect approved-end before human action")
	}
	if _, ok := inst.NodeStates["rejected-end"]; ok {
		t.Fatalf("did not expect rejected-end to be visited")
	}
	if _, ok := inst.NodeStates["fallback-end"]; ok {
		t.Fatalf("did not expect fallback-end to be visited")
	}
	if len(store.tasks) != 1 {
		t.Fatalf("expected one task assignment, got %d", len(store.tasks))
	}
	var savedTask models.TaskAssignment
	for _, task := range store.tasks {
		savedTask = task
		break
	}
	if savedTask.NodeID != "task" || savedTask.AssignedRole != "manager" {
		t.Fatalf("unexpected saved task: %+v", savedTask)
	}
	if savedTask.VisibleData == nil {
		t.Fatalf("expected visible data to be populated by default")
	}
	if got, ok := savedTask.VisibleData["request_id"]; !ok || got != "r-1" {
		t.Fatalf("expected request_id in visible data, got %#v", savedTask.VisibleData)
	}
	if countAuditAction(inst, "task_assigned") != 1 {
		t.Fatalf("expected task_assigned audit entry")
	}

	if _, err := exec.ContinueTask(savedTask.ID, "user-1", "start", "", ""); err != nil {
		t.Fatalf("ContinueTask start failed: %v", err)
	}
	if _, err := exec.ContinueTask(savedTask.ID, "user-1", "approve", "looks good", ""); err != nil {
		t.Fatalf("ContinueTask failed: %v", err)
	}
	persistedTask, ok := store.GetTask(savedTask.ID)
	if !ok {
		t.Fatalf("expected persisted task after ContinueTask")
	}
	if persistedTask.Status != models.TaskCompleted {
		t.Fatalf("expected persisted completed status, got %s", persistedTask.Status)
	}
	if persistedTask.ActionCommitted != "approve" {
		t.Fatalf("expected committed action approve, got %q", persistedTask.ActionCommitted)
	}
	if persistedTask.Comment != "looks good" {
		t.Fatalf("expected persisted comment, got %q", persistedTask.Comment)
	}
	if persistedTask.CompletedAt == nil {
		t.Fatalf("expected persisted completed timestamp")
	}

	inst, _ = store.GetInstance(instanceID)
	if inst.Status != models.InstanceCompleted {
		t.Fatalf("expected completed status after continue, got %s", inst.Status)
	}
	if _, ok := inst.NodeStates["approved-end"]; !ok {
		t.Fatalf("expected approved-end to be visited")
	}
}

func TestExecuteTaskAppliesVisibilitySelection(t *testing.T) {
	store := newMockStore()
	exec := NewExecutor(store, &mockEmail{}, nil)

	wf := models.Workflow{
		ID:    "wf-visibility-selected",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{{
			ID:                    "task",
			Type:                  models.NodeTask,
			Title:                 "Review",
			TaskDataVisibility:    "selected",
			VisibleDataKeys:       []string{"employee_name", "amount", "missing_key"},
			IncludeFormSubmission: true,
			IncludeFormFiles:      true,
		}},
	}

	data := map[string]interface{}{
		"employee_name": "Alice",
		"amount":        "1200",
		"internal_note": "sensitive",
		"form_submission": map[string]interface{}{
			"receipt": "https://drive.google.com/file/d/abc123/view",
		},
	}

	if _, err := exec.executeTask("inst-1", &wf, &wf.Nodes[0], data, ""); err != nil {
		t.Fatalf("executeTask failed: %v", err)
	}
	if len(store.tasks) != 1 {
		t.Fatalf("expected one saved task, got %d", len(store.tasks))
	}

	var savedTask models.TaskAssignment
	for _, task := range store.tasks {
		savedTask = task
		break
	}

	if savedTask.Data == nil || savedTask.Data["internal_note"] != "sensitive" {
		t.Fatalf("expected full internal data to be retained, got %#v", savedTask.Data)
	}
	if _, ok := savedTask.VisibleData["internal_note"]; ok {
		t.Fatalf("did not expect internal_note in assignee visible data: %#v", savedTask.VisibleData)
	}
	if savedTask.VisibleData["employee_name"] != "Alice" || savedTask.VisibleData["amount"] != "1200" {
		t.Fatalf("expected selected fields in visible data, got %#v", savedTask.VisibleData)
	}
	if _, ok := savedTask.VisibleData["form_submission"]; !ok {
		t.Fatalf("expected form_submission to be included when requested")
	}
	rawFiles, ok := savedTask.VisibleData["form_submission_files"]
	if !ok {
		t.Fatalf("expected extracted file refs in visible data")
	}
	files, ok := rawFiles.([]string)
	if !ok || len(files) != 1 {
		t.Fatalf("expected one extracted file ref, got %#v", rawFiles)
	}
}

func TestExecuteTaskVisibilityNoneWithoutOverrides(t *testing.T) {
	store := newMockStore()
	exec := NewExecutor(store, &mockEmail{}, nil)

	wf := models.Workflow{
		ID:    "wf-visibility-none",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{{
			ID:                 "task",
			Type:               models.NodeTask,
			Title:              "Review",
			TaskDataVisibility: "none",
		}},
	}

	if _, err := exec.executeTask("inst-2", &wf, &wf.Nodes[0], map[string]interface{}{"secret": "x"}, ""); err != nil {
		t.Fatalf("executeTask failed: %v", err)
	}

	var savedTask models.TaskAssignment
	for _, task := range store.tasks {
		savedTask = task
		break
	}
	if savedTask.VisibleData != nil {
		t.Fatalf("expected no visible data for none mode, got %#v", savedTask.VisibleData)
	}
}

func TestExecuteTaskVisibilityUnknownModeDefaultsToNone(t *testing.T) {
	store := newMockStore()
	exec := NewExecutor(store, &mockEmail{}, nil)

	wf := models.Workflow{
		ID:    "wf-visibility-unknown",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{{
			ID:                 "task",
			Type:               models.NodeTask,
			Title:              "Review",
			TaskDataVisibility: "unexpected_mode",
		}},
	}

	if _, err := exec.executeTask("inst-3", &wf, &wf.Nodes[0], map[string]interface{}{"secret": "x"}, ""); err != nil {
		t.Fatalf("executeTask failed: %v", err)
	}

	var savedTask models.TaskAssignment
	for _, task := range store.tasks {
		savedTask = task
		break
	}
	if savedTask.VisibleData != nil {
		t.Fatalf("expected no visible data for unknown mode, got %#v", savedTask.VisibleData)
	}
}

func TestContinueTaskStartClaimsTaskWithoutComment(t *testing.T) {
	store := newMockStore()
	exec := NewExecutor(store, &mockEmail{}, nil)

	wf := models.Workflow{
		ID:    "wf-start-claim",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{
				ID:          "task",
				Type:        models.NodeTask,
				Title:       "Review",
				TaskActions: []string{"approve", "reject"},
			},
		},
	}
	store.workflows[wf.ID] = wf
	instanceID := seedInstance(t, store, wf, map[string]interface{}{})
	taskID, err := store.SaveTask(models.TaskAssignment{
		OrgID:          "org-1",
		InstanceID:     instanceID,
		WorkflowID:     wf.ID,
		NodeID:         "task",
		Title:          "Review",
		AllowedActions: []string{"approve", "reject"},
		Status:         models.TaskPending,
		CreatedAt:      time.Now(),
	})
	if err != nil {
		t.Fatalf("save task failed: %v", err)
	}

	updated, err := exec.ContinueTask(taskID, "user-1", "start", "", "")
	if err != nil {
		t.Fatalf("ContinueTask start failed: %v", err)
	}
	if updated.Status != models.TaskInProgress {
		t.Fatalf("expected in_progress after start, got %s", updated.Status)
	}
	if updated.AssignedUser != "user-1" {
		t.Fatalf("expected task to be claimed by actor, got %q", updated.AssignedUser)
	}
	if updated.Comment != "" {
		t.Fatalf("expected no comment on start, got %q", updated.Comment)
	}
	if updated.CompletedAt != nil {
		t.Fatalf("expected no completed time after start")
	}

	persisted, ok := store.GetTask(taskID)
	if !ok {
		t.Fatalf("expected persisted task")
	}
	if persisted.Status != models.TaskInProgress || persisted.AssignedUser != "user-1" {
		t.Fatalf("unexpected persisted task after start: %+v", persisted)
	}
	if got := countAuditAction(mustGetInstance(t, store, instanceID), "task_started"); got != 1 {
		t.Fatalf("expected one task_started audit entry, got %d", got)
	}
}

func mustGetInstance(t *testing.T, store *mockStore, instanceID string) models.Instance {
	t.Helper()
	inst, ok := store.GetInstance(instanceID)
	if !ok {
		t.Fatalf("instance %s not found", instanceID)
	}
	return inst
}

func TestRunParallelMergeCompletesAfterBothBranches(t *testing.T) {
	store := newMockStore()
	email := &mockEmail{}
	exec := NewExecutor(store, email, nil)

	wf := models.Workflow{
		ID:    "wf-parallel",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "parallel"},
			{ID: "parallel", Type: models.NodeParallel, Title: "Parallel", NextBranches: []string{"a", "b"}},
			{ID: "a", Type: models.NodeAction, Title: "A", Connector: &models.ConnectorConfig{Type: models.ConnectorEmail, Params: map[string]interface{}{"to": "a@example.com", "subject": "A", "body_template": "A"}}, Next: "merge"},
			{ID: "b", Type: models.NodeAction, Title: "B", Connector: &models.ConnectorConfig{Type: models.ConnectorEmail, Params: map[string]interface{}{"to": "b@example.com", "subject": "B", "body_template": "B"}}, Next: "merge"},
			{ID: "merge", Type: models.NodeMerge, Title: "Merge", RequiredInputs: []string{"a", "b"}, Next: "end"},
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}
	instanceID := seedInstance(t, store, wf, map[string]interface{}{})

	exec.run(instanceID, wf, map[string]interface{}{}, "")

	inst, _ := store.GetInstance(instanceID)
	if inst.Status != models.InstanceCompleted {
		t.Fatalf("expected completed status, got %s", inst.Status)
	}
	if inst.NodeStates["merge"].Status != "completed" || inst.NodeStates["end"].Status != "completed" {
		t.Fatalf("expected merge/end completion, got %+v", inst.NodeStates)
	}
	if got := countAuditAction(inst, "merge_completed"); got != 1 {
		t.Fatalf("expected one merge_completed audit event, got %d", got)
	}
	if email.count() != 2 {
		t.Fatalf("expected two email calls, got %d", email.count())
	}
}

func TestRunActionFailureMarksInstanceFailedAndStopsProgress(t *testing.T) {
	store := newMockStore()
	exec := NewExecutor(store, &mockEmail{}, nil)

	wf := models.Workflow{
		ID:    "wf-action-skips",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "unknown"},
			{
				ID:    "unknown",
				Type:  models.NodeAction,
				Title: "Unknown",
				Connector: &models.ConnectorConfig{
					Type: models.ConnectorType("mystery"),
				},
				Next: "missing",
			},
			{ID: "missing", Type: models.NodeAction, Title: "Missing", Next: "end"},
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}
	instanceID := seedInstance(t, store, wf, map[string]interface{}{})

	exec.run(instanceID, wf, map[string]interface{}{}, "")

	inst, _ := store.GetInstance(instanceID)
	if inst.Status != models.InstanceFailed {
		t.Fatalf("expected failed status, got %s", inst.Status)
	}
	if got := countAuditAction(inst, "instance_failed"); got != 1 {
		t.Fatalf("expected one instance_failed audit event, got %d", got)
	}
	if state := inst.NodeStates["unknown"]; state.Status != "failed" {
		t.Fatalf("expected unknown node failed state, got %+v", state)
	}
	if _, ok := inst.NodeStates["missing"]; ok {
		t.Fatalf("expected execution to stop before missing node")
	}
	if _, ok := inst.NodeStates["end"]; ok {
		t.Fatalf("expected execution to stop before end node")
	}
}

func TestRunActionSendErrorMarksFailedAndDoesNotAdvance(t *testing.T) {
	store := newMockStore()
	email := &mockEmail{sendErr: errors.New("gmail send denied")}
	exec := NewExecutor(store, email, nil)

	wf := models.Workflow{
		ID:    "wf-action-send-fail",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "email"},
			{
				ID:    "email",
				Type:  models.NodeAction,
				Title: "Send Email",
				Connector: &models.ConnectorConfig{
					Type: models.ConnectorEmail,
					Params: map[string]interface{}{
						"to":            "alice@example.com",
						"subject":       "Subject",
						"body_template": "Body",
					},
				},
				Next: "end",
			},
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}

	instanceID := seedInstance(t, store, wf, map[string]interface{}{})

	exec.run(instanceID, wf, map[string]interface{}{}, "")

	inst, _ := store.GetInstance(instanceID)
	if inst.Status != models.InstanceFailed {
		t.Fatalf("expected failed status, got %s", inst.Status)
	}
	if got := countAuditAction(inst, "action_failed"); got != 1 {
		t.Fatalf("expected one action_failed audit event, got %d", got)
	}
	if got := countAuditAction(inst, "instance_failed"); got != 1 {
		t.Fatalf("expected one instance_failed audit event, got %d", got)
	}
	if _, ok := inst.NodeStates["end"]; ok {
		t.Fatalf("expected execution not to reach end node")
	}
}

func TestRunActionSendForOrgErrorMarksFailedAndDoesNotAdvance(t *testing.T) {
	store := newMockStore()
	email := &mockOrgEmail{sendForOrgErr: errors.New("gmail org send denied")}
	exec := NewExecutor(store, email, nil)

	wf := models.Workflow{
		ID:    "wf-action-sendfororg-fail",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "email"},
			{
				ID:    "email",
				Type:  models.NodeAction,
				Title: "Send Email",
				Connector: &models.ConnectorConfig{
					Type: models.ConnectorEmail,
					Params: map[string]interface{}{
						"to":            "alice@example.com",
						"subject":       "Subject",
						"body_template": "Body",
						"from_name":     "Workflow Bot",
					},
				},
				Next: "end",
			},
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}

	instanceID := seedInstance(t, store, wf, map[string]interface{}{})

	exec.run(instanceID, wf, map[string]interface{}{}, "")

	inst, _ := store.GetInstance(instanceID)
	if inst.Status != models.InstanceFailed {
		t.Fatalf("expected failed status, got %s", inst.Status)
	}
	if got := countAuditAction(inst, "action_failed"); got != 1 {
		t.Fatalf("expected one action_failed audit event, got %d", got)
	}
	if got := countAuditAction(inst, "instance_failed"); got != 1 {
		t.Fatalf("expected one instance_failed audit event, got %d", got)
	}
	if _, ok := inst.NodeStates["end"]; ok {
		t.Fatalf("expected execution not to reach end node")
	}
}

func TestRunTaskFailsWhenRoleAssigneeCannotBeResolved(t *testing.T) {
	store := newMockStore()
	exec := NewExecutor(store, &mockEmail{}, NewRandomRoleAssigneeSelector(NewStaticRoleMemberDirectory(map[string]map[string][]string{})))

	wf := models.Workflow{
		ID:    "wf-task-no-assignee",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "task"},
			{ID: "task", Type: models.NodeTask, Title: "Manager Task", AssignedRole: "manager", TaskActions: []string{"approve"}, Next: "end"},
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}
	instanceID := seedInstance(t, store, wf, map[string]interface{}{"request_id": "r-404"})

	exec.run(instanceID, wf, map[string]interface{}{"request_id": "r-404"}, "")

	inst, ok := store.GetInstance(instanceID)
	if !ok {
		t.Fatalf("instance not found")
	}
	if inst.Status != models.InstanceFailed {
		t.Fatalf("expected failed status when assignee cannot be resolved, got %s", inst.Status)
	}
	if len(store.tasks) != 0 {
		t.Fatalf("expected no task to be persisted when role assignee is unresolved, got %d", len(store.tasks))
	}
	if got := countAuditAction(inst, "assignee_unresolved"); got != 1 {
		t.Fatalf("expected assignee_unresolved audit entry, got %d", got)
	}
}

func TestExecuteTaskUsesInjectedRoleSelectionStrategy(t *testing.T) {
	store := newMockStore()
	directory := NewStaticRoleMemberDirectory(map[string]map[string][]string{
		"org-1": {
			"manager": {"user-a", "user-b"},
		},
	})
	selector := NewRandomRoleAssigneeSelectorWithStrategy(directory, fixedRoleMemberSelectionStrategy{selectedUserID: "user-b"})
	exec := NewExecutor(store, &mockEmail{}, selector)

	wf := models.Workflow{ID: "wf-fixed-selector", OrgID: "org-1"}
	node := models.WorkflowNode{ID: "task", Type: models.NodeTask, Title: "Task", AssignedRole: "manager", TaskActions: []string{"approve"}}

	if _, err := exec.executeTask("inst-fixed", &wf, &node, map[string]interface{}{"x": 1}, ""); err != nil {
		t.Fatalf("executeTask failed: %v", err)
	}
	if len(store.tasks) != 1 {
		t.Fatalf("expected one saved task, got %d", len(store.tasks))
	}
	for _, task := range store.tasks {
		if task.AssignedUser != "user-b" {
			t.Fatalf("expected injected strategy to pick user-b, got %q", task.AssignedUser)
		}
	}
}

func TestExecuteTaskRoleSelectionOverridesPreferredUser(t *testing.T) {
	store := newMockStore()
	directory := NewStaticRoleMemberDirectory(map[string]map[string][]string{
		"org-1": {
			"manager": {"user-a", "user-b"},
		},
	})
	selector := NewRandomRoleAssigneeSelectorWithStrategy(directory, fixedRoleMemberSelectionStrategy{selectedUserID: "user-b"})
	exec := NewExecutor(store, &mockEmail{}, selector)

	wf := models.Workflow{ID: "wf-role-preferred", OrgID: "org-1"}
	node := models.WorkflowNode{
		ID:           "task",
		Type:         models.NodeTask,
		Title:        "Task",
		AssignedRole: "manager",
		AssignedUser: "user-a",
		TaskActions:  []string{"approve"},
	}

	if _, err := exec.executeTask("inst-role-preferred", &wf, &node, map[string]interface{}{"x": 1}, ""); err != nil {
		t.Fatalf("executeTask failed: %v", err)
	}
	if len(store.tasks) != 1 {
		t.Fatalf("expected one saved task, got %d", len(store.tasks))
	}
	for _, task := range store.tasks {
		if task.AssignedUser != "user-b" {
			t.Fatalf("expected role-based strategy to override preferred user and pick user-b, got %q", task.AssignedUser)
		}
	}
}

func TestSelectorUsesPreferredUserWhenRoleIsEmpty(t *testing.T) {
	selector := NewRandomRoleAssigneeSelectorWithStrategy(
		NewStaticRoleMemberDirectory(map[string]map[string][]string{}),
		fixedRoleMemberSelectionStrategy{selectedUserID: "user-b"},
	)

	assignee, err := selector.Select("org-1", "", "user-a")
	if err != nil {
		t.Fatalf("selector returned error: %v", err)
	}
	if assignee != "user-a" {
		t.Fatalf("expected preferred user fallback when role is empty, got %q", assignee)
	}
}

func TestBalancedSelectorPrefersLeastAssignedMember(t *testing.T) {
	store := newMockStore()
	seedTask := func(id, user string, createdAt time.Time) {
		store.tasks[id] = models.TaskAssignment{
			ID:           id,
			OrgID:        "org-1",
			AssignedRole: "manager",
			AssignedUser: user,
			Status:       models.TaskCompleted,
			CreatedAt:    createdAt,
		}
	}
	now := time.Now()
	seedTask("t-1", "user-a", now.Add(-5*time.Minute))
	seedTask("t-2", "user-a", now.Add(-4*time.Minute))
	seedTask("t-3", "user-a", now.Add(-3*time.Minute))
	seedTask("t-4", "user-b", now.Add(-2*time.Minute))

	directory := NewStaticRoleMemberDirectory(map[string]map[string][]string{
		"org-1": {"manager": {"user-a", "user-b"}},
	})
	selector := NewBalancedRoleAssigneeSelector(directory, store)
	selector.strategy = fixedRoleMemberSelectionStrategy{selectedUserID: "user-b"}

	assignee, err := selector.Select("org-1", "manager", "")
	if err != nil {
		t.Fatalf("selector returned error: %v", err)
	}
	if assignee != "user-b" {
		t.Fatalf("expected least-assigned member user-b, got %q", assignee)
	}
}

func TestBalancedSelectorTieBreakUsesStrategy(t *testing.T) {
	store := newMockStore()
	seedTask := func(id, user string, createdAt time.Time) {
		store.tasks[id] = models.TaskAssignment{
			ID:           id,
			OrgID:        "org-1",
			AssignedRole: "manager",
			AssignedUser: user,
			Status:       models.TaskCompleted,
			CreatedAt:    createdAt,
		}
	}
	now := time.Now()
	seedTask("t-1", "user-a", now.Add(-2*time.Minute))
	seedTask("t-2", "user-b", now.Add(-1*time.Minute))

	directory := NewStaticRoleMemberDirectory(map[string]map[string][]string{
		"org-1": {"manager": {"user-a", "user-b"}},
	})
	selector := NewBalancedRoleAssigneeSelector(directory, store)
	selector.strategy = fixedRoleMemberSelectionStrategy{selectedUserID: "user-b"}

	assignee, err := selector.Select("org-1", "manager", "")
	if err != nil {
		t.Fatalf("selector returned error: %v", err)
	}
	if assignee != "user-b" {
		t.Fatalf("expected tie-break strategy to select user-b, got %q", assignee)
	}
}

func TestRestartFailedInstanceResumesFromFailedTaskNode(t *testing.T) {
	store := newMockStore()
	strategy := &flipFlopRoleMemberSelectionStrategy{selectedUserID: "user-b"}
	exec := NewExecutor(store, &mockEmail{}, NewRandomRoleAssigneeSelectorWithStrategy(NewStaticRoleMemberDirectory(map[string]map[string][]string{
		"org-1": {
			"manager": {"user-b"},
		},
	}), strategy))

	wf := models.Workflow{
		ID:    "wf-restart",
		OrgID: "org-1",
		Nodes: []models.WorkflowNode{
			{ID: "start", Type: models.NodeStart, Title: "Start", Next: "task"},
			{ID: "task", Type: models.NodeTask, Title: "Review", AssignedRole: "manager", TaskActions: []string{"approve"}, Next: "end"},
			{ID: "end", Type: models.NodeEnd, Title: "End"},
		},
	}
	store.workflows[wf.ID] = wf
	instanceID := seedInstance(t, store, wf, map[string]interface{}{"request_id": "r-1"})

	exec.run(instanceID, wf, map[string]interface{}{"request_id": "r-1"}, "")

	inst, ok := store.GetInstance(instanceID)
	if !ok {
		t.Fatalf("instance not found")
	}
	if inst.Status != models.InstanceFailed {
		t.Fatalf("expected failed status before restart, got %s", inst.Status)
	}

	restarted, err := exec.RestartFailedInstance(instanceID, "Bearer user-token")
	if err != nil {
		t.Fatalf("RestartFailedInstance returned error: %v", err)
	}
	if restarted.Status != models.InstanceRunning {
		t.Fatalf("expected running status immediately after restart, got %s", restarted.Status)
	}

	inst = waitForInstanceStatus(t, store, instanceID, models.InstanceWaiting)
	if inst.Status != models.InstanceWaiting {
		t.Fatalf("expected waiting status after restart resumed task node, got %s", inst.Status)
	}
	if inst.CurrentNode != "task" {
		t.Fatalf("expected current node to remain task, got %q", inst.CurrentNode)
	}
	if countAuditAction(inst, "instance_restarted") != 1 {
		t.Fatalf("expected instance_restarted audit entry, got %+v", inst.AuditLog)
	}
	if len(store.tasks) != 1 {
		t.Fatalf("expected one task after restart, got %d", len(store.tasks))
	}
	for _, task := range store.tasks {
		if task.AssignedUser != "user-b" {
			t.Fatalf("expected restarted task to be assigned to user-b, got %q", task.AssignedUser)
		}
	}
}
