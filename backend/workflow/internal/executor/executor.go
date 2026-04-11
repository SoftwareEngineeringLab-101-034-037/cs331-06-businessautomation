package executor

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/connectors"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/models"
	"github.com/SoftwareEngineeringLab-101-034-037/CS331-06-BusinessAutomation/backend/workflow/internal/storage"
)

// Executor handles workflow execution with one goroutine per instance.
type Executor struct {
	store            storage.Store
	email            connectors.EmailConnector
	assigneeSelector TaskAssigneeSelector

	mu            sync.Mutex
	mergeWaiters  map[string]int
	instanceLocks map[string]*sync.Mutex
}

var (
	ErrTaskNotFound         = errors.New("task not found")
	ErrTaskAlreadyCompleted = errors.New("task already completed")
	ErrCommentRequired      = errors.New("comment is required for task actions")
	ErrUnknownAction        = errors.New("unknown action")
	ErrTaskConflict         = errors.New("task was updated concurrently")
	ErrInstanceNotFound     = errors.New("instance not found")
	ErrInstanceNotFailed    = errors.New("instance is not failed")
	ErrWorkflowNotFound     = errors.New("workflow not found")
	ErrTaskNodeNotFound     = errors.New("task node not found in workflow")
	ErrFailedNodeNotFound   = errors.New("failed node not found in workflow")
	ErrForbiddenTaskAction  = errors.New("forbidden task action")
	ErrTaskClaimNotAllowed  = errors.New("task claim not allowed")
	ErrPendingTaskStartOnly = errors.New("pending tasks can only be started")
	ErrNoEligibleAssignee   = errors.New("no eligible assignee found for task")
)

const numberComparisonEpsilon = 1e-9

func NewExecutor(s storage.Store, e connectors.EmailConnector, selector TaskAssigneeSelector) *Executor {
	return &Executor{
		store:            s,
		email:            e,
		assigneeSelector: selector,
		mergeWaiters:     make(map[string]int),
		instanceLocks:    make(map[string]*sync.Mutex),
	}
}

func (e *Executor) StartInstance(wf models.Workflow, data map[string]interface{}, authHeader string) (string, error) {
	now := time.Now()
	inst := models.Instance{
		WorkflowID: wf.ID,
		OrgID:      wf.OrgID,
		Status:     models.InstanceRunning,
		Data:       data,
		NodeStates: make(map[string]models.NodeState),
		AuditLog: []models.AuditEntry{{
			Timestamp: now,
			Action:    "instance_started",
			Details:   map[string]interface{}{"workflow_id": wf.ID},
		}},
		StartedAt: now,
	}
	id, err := e.store.SaveInstance(inst)
	if err != nil {
		return "", err
	}
	go e.run(id, wf, data, authHeader)
	return id, nil
}

func (e *Executor) FindOrStartInstanceByFormResponse(wf models.Workflow, data map[string]interface{}, responseID, authHeader string) (string, bool, error) {
	trimmedResponseID := strings.TrimSpace(responseID)
	if trimmedResponseID == "" {
		id, err := e.StartInstance(wf, data, authHeader)
		return id, false, err
	}

	// Serialize create/check flow by workflow+response key to prevent duplicate starts in-process.
	lock := e.instanceLock("form_response:" + wf.ID + ":" + trimmedResponseID)
	lock.Lock()
	defer lock.Unlock()

	existingID, err := e.findInstanceIDByFormResponse(wf.ID, trimmedResponseID)
	if err != nil {
		return "", false, err
	}
	if existingID != "" {
		return existingID, true, nil
	}

	instanceID, err := e.StartInstance(wf, data, authHeader)
	if err != nil {
		if isDuplicateKeyError(err) {
			existingID, lookupErr := e.findInstanceIDByFormResponse(wf.ID, trimmedResponseID)
			if lookupErr == nil && existingID != "" {
				return existingID, true, nil
			}
		}
		return "", false, err
	}
	return instanceID, false, nil
}

func (e *Executor) FindOrStartInstanceByEmailMessage(wf models.Workflow, data map[string]interface{}, emailMessageID, authHeader string) (string, bool, error) {
	trimmedMessageID := strings.TrimSpace(emailMessageID)
	if trimmedMessageID == "" {
		id, err := e.StartInstance(wf, data, authHeader)
		return id, false, err
	}

	lock := e.instanceLock("email_message:" + wf.ID + ":" + trimmedMessageID)
	lock.Lock()
	defer lock.Unlock()

	existingID, err := e.findInstanceIDByEmailMessage(wf.ID, trimmedMessageID)
	if err != nil {
		return "", false, err
	}
	if existingID != "" {
		return existingID, true, nil
	}

	instanceID, err := e.StartInstance(wf, data, authHeader)
	if err != nil {
		if isDuplicateKeyError(err) {
			existingID, lookupErr := e.findInstanceIDByEmailMessage(wf.ID, trimmedMessageID)
			if lookupErr == nil && existingID != "" {
				return existingID, true, nil
			}
		}
		return "", false, err
	}
	return instanceID, false, nil
}

func (e *Executor) RestartFailedInstance(instanceID, authHeader string) (models.Instance, error) {
	lock := e.instanceLock(instanceID)
	lock.Lock()
	defer lock.Unlock()

	inst, ok := e.store.GetInstance(instanceID)
	if !ok {
		return models.Instance{}, ErrInstanceNotFound
	}
	if inst.Status != models.InstanceFailed {
		return models.Instance{}, ErrInstanceNotFailed
	}

	wf, ok := e.store.GetWorkflow(inst.WorkflowID)
	if !ok {
		return models.Instance{}, ErrWorkflowNotFound
	}

	nodeID := restartNodeID(inst)
	if nodeID == "" || wf.FindNode(nodeID) == nil {
		return models.Instance{}, ErrFailedNodeNotFound
	}

	now := time.Now()
	if inst.NodeStates == nil {
		inst.NodeStates = make(map[string]models.NodeState)
	}
	ns := inst.NodeStates[nodeID]
	ns.Status = "running"
	ns.StartedAt = &now
	ns.CompletedAt = nil
	ns.Output = ""
	inst.NodeStates[nodeID] = ns
	inst.Status = models.InstanceRunning
	inst.CurrentNode = nodeID
	inst.CompletedAt = nil
	inst.AuditLog = append(inst.AuditLog, models.AuditEntry{
		Timestamp: now,
		NodeID:    nodeID,
		Action:    "instance_restarted",
		Details: map[string]interface{}{
			"failed_node": nodeID,
			"workflow_id": wf.ID,
		},
	})
	if _, err := e.store.SaveInstance(inst); err != nil {
		return models.Instance{}, err
	}

	go e.walkNode(instanceID, nodeID, &wf, inst.Data, authHeader)
	return inst, nil
}

func restartNodeID(inst models.Instance) string {
	if trimmed := strings.TrimSpace(inst.CurrentNode); trimmed != "" {
		return trimmed
	}
	var latestNodeID string
	var latestTime time.Time
	for nodeID, state := range inst.NodeStates {
		if state.Status != "failed" || state.CompletedAt == nil {
			continue
		}
		if latestNodeID == "" || state.CompletedAt.After(latestTime) {
			latestNodeID = nodeID
			latestTime = *state.CompletedAt
		}
	}
	return latestNodeID
}

func (e *Executor) findInstanceIDByFormResponse(workflowID, responseID string) (string, error) {
	inst, ok, err := e.store.FindInstanceByWorkflowAndFormResponse(workflowID, responseID)
	if err != nil {
		return "", err
	}
	if ok {
		return inst.ID, nil
	}
	return "", nil
}

func (e *Executor) findInstanceIDByEmailMessage(workflowID, emailMessageID string) (string, error) {
	inst, ok, err := e.store.FindInstanceByWorkflowAndEmailMessageID(workflowID, emailMessageID)
	if err != nil {
		return "", err
	}
	if ok {
		return inst.ID, nil
	}
	return "", nil
}

func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}

func (e *Executor) run(instanceID string, wf models.Workflow, data map[string]interface{}, authHeader string) {
	log.Printf("executor: running instance=%s workflow=%s", instanceID, wf.ID)

	start := wf.StartNode()
	if start == nil {
		e.markFailed(instanceID, "no start node")
		return
	}

	e.walkNode(instanceID, start.ID, &wf, data, authHeader)

	inst, ok := e.store.GetInstance(instanceID)
	if !ok {
		return
	}
	if inst.Status == models.InstanceWaiting || inst.Status == models.InstanceFailed || inst.Status == models.InstanceCancelled || inst.Status == models.InstanceCompleted {
		return
	}

	now := time.Now()
	inst.Status = models.InstanceCompleted
	inst.CompletedAt = &now
	inst.AuditLog = append(inst.AuditLog, models.AuditEntry{Timestamp: now, Action: "instance_completed"})
	if _, err := e.store.SaveInstance(inst); err != nil {
		log.Printf("executor: save completed instance failed: %v", err)
	}
}

func (e *Executor) walkNode(instanceID, nodeID string, wf *models.Workflow, data map[string]interface{}, authHeader string) {
	if e.isInstanceTerminal(instanceID) {
		return
	}

	node := wf.FindNode(nodeID)
	if node == nil {
		errMsg := fmt.Sprintf("node %s not found", nodeID)
		log.Printf("executor: %s", errMsg)
		e.markNodeFailed(instanceID, nodeID, errMsg)
		e.markFailed(instanceID, errMsg)
		return
	}

	e.setNodeState(instanceID, nodeID, "running")

	switch node.Type {
	case models.NodeStart:
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, "", wf, data, authHeader)
		return

	case models.NodeEnd:
		e.setNodeState(instanceID, nodeID, "completed")
		return

	case models.NodeTask:
		action, err := e.executeTask(instanceID, wf, node, data, authHeader)
		if err != nil {
			errMsg := fmt.Sprintf("task node %s failed: %v", nodeID, err)
			e.markNodeFailed(instanceID, nodeID, errMsg)
			e.markFailed(instanceID, errMsg)
			return
		}
		if action == "" {
			// waiting for human action
			return
		}
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, action, wf, data, authHeader)
		return

	case models.NodeAction:
		err := e.executeAction(instanceID, wf.OrgID, node, data)
		if err != nil {
			errMsg := fmt.Sprintf("action node %s failed: %v", nodeID, err)
			e.markNodeFailed(instanceID, nodeID, errMsg)
			e.markFailed(instanceID, errMsg)
			return
		}
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, "", wf, data, authHeader)
		return

	case models.NodeCondition:
		branch, details := e.evalConditionForNode(node, wf, data)
		details["branch"] = branch
		e.audit(instanceID, nodeID, "condition_evaluated", details)
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, branch, wf, data, authHeader)
		return

	case models.NodeParallel:
		e.setNodeState(instanceID, nodeID, "completed")
		var wg sync.WaitGroup
		for _, nextID := range node.NextBranches {
			branchNextID := nextID
			branchData := cloneWorkflowData(data)
			wg.Add(1)
			go func() {
				defer wg.Done()
				e.walkNode(instanceID, branchNextID, wf, branchData, authHeader)
			}()
		}
		wg.Wait()
		return

	case models.NodeMerge:
		needed := len(node.RequiredInputs)
		if needed == 0 {
			needed = 1
		}
		key := instanceID + ":" + node.ID

		e.mu.Lock()
		e.mergeWaiters[key]++
		arrived := e.mergeWaiters[key]
		e.mu.Unlock()

		if arrived < needed {
			return
		}

		e.mu.Lock()
		delete(e.mergeWaiters, key)
		e.mu.Unlock()

		e.audit(instanceID, nodeID, "merge_completed", map[string]interface{}{
			"required_inputs": node.RequiredInputs,
		})
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, "", wf, data, authHeader)
		return
	}
}

func (e *Executor) walkNext(instanceID string, node *models.WorkflowNode, result string, wf *models.Workflow, data map[string]interface{}, authHeader string) {
	isTerminal := e.isInstanceTerminal(instanceID)
	if isTerminal {
		return
	}
	for _, nextID := range node.NextIDs(result) {
		if isTerminal {
			return
		}
		e.walkNode(instanceID, nextID, wf, data, authHeader)
		isTerminal = e.isInstanceTerminal(instanceID)
	}
}

func (e *Executor) executeTask(instanceID string, wf *models.Workflow, node *models.WorkflowNode, data map[string]interface{}, authHeader string) (string, error) {
	assignmentTargets, assignedRoles, assignedUsers := extractNodeAssignmentTargets(node)
	hasExplicitTargetConfig := len(node.AssignedTargets) > 0 || len(node.AssignedRoles) > 0 || len(node.AssignedUsers) > 0

	assignedRole := strings.TrimSpace(node.AssignedRole)
	if len(assignedRoles) > 0 {
		assignedRole = assignedRoles[0]
	}
	assignedUser := strings.TrimSpace(node.AssignedUser)

	if hasExplicitTargetConfig {
		if len(assignedRoles) == 0 && len(assignedUsers) == 0 {
			e.audit(instanceID, node.ID, "assignee_unresolved", map[string]interface{}{
				"targets": assignmentTargets,
			})
			return "", ErrNoEligibleAssignee
		}
		assignedUser = ""
	} else {
		if e.assigneeSelector != nil {
			resolvedUser, err := e.selectAssignee(wf.OrgID, node.AssignedRole, node.AssignedUser, authHeader)
			if err != nil {
				log.Printf("executor: assignee lookup failed for role=%q org=%q: %v", node.AssignedRole, wf.OrgID, err)
				e.audit(instanceID, node.ID, "assignee_lookup_failed", map[string]interface{}{
					"role":  node.AssignedRole,
					"error": err.Error(),
				})
				return "", fmt.Errorf("resolve assignee: %w", err)
			}
			assignedUser = strings.TrimSpace(resolvedUser)
		}

		if assignedRole != "" && assignedUser == "" {
			e.audit(instanceID, node.ID, "assignee_unresolved", map[string]interface{}{
				"role": assignedRole,
			})
			return "", ErrNoEligibleAssignee
		}
	}

	if len(assignedRoles) == 0 && assignedRole != "" {
		assignedRoles = []string{assignedRole}
	}
	if len(assignedUsers) == 0 && strings.TrimSpace(node.AssignedUser) != "" {
		assignedUsers = []string{strings.TrimSpace(node.AssignedUser)}
	}

	task := models.TaskAssignment{
		InstanceID:       instanceID,
		OrgID:            wf.OrgID,
		WorkflowID:       wf.ID,
		NodeID:           node.ID,
		Title:            node.Title,
		Description:      node.Description,
		AssignedRole:     assignedRole,
		AssignedRoles:    assignedRoles,
		AssignedUsers:    assignedUsers,
		AssignedTargets:  assignmentTargets,
		AssignedPosition: node.AssignedPosition,
		AssignedUser:     assignedUser,
		AllowedActions:   node.TaskActions,
		FormTemplateID:   node.FormTemplateID,
		SLADays:          node.SLADays,
		Status:           models.TaskPending,
		Data:             cloneWorkflowData(data),
		VisibleData:      buildTaskVisibleData(node, data),
		CreatedAt:        time.Now(),
	}
	taskID, err := e.store.SaveTask(task)
	if err != nil {
		return "", fmt.Errorf("save task: %w", err)
	}

	e.audit(instanceID, node.ID, "task_assigned", map[string]interface{}{
		"task_id":          taskID,
		"assigned_role":    task.AssignedRole,
		"assigned_roles":   task.AssignedRoles,
		"assigned_users":   task.AssignedUsers,
		"assigned_targets": task.AssignedTargets,
		"assigned_user":    task.AssignedUser,
		"allowed_actions":  node.TaskActions,
	})

	if _, err := e.saveInstanceMutation(instanceID, func(inst *models.Instance) {
		inst.Status = models.InstanceWaiting
		inst.CurrentNode = node.ID
	}); err != nil && !errors.Is(err, ErrInstanceNotFound) {
		log.Printf("executor: failed to save waiting instance instance_id=%s node_id=%s: %v", instanceID, node.ID, err)
	}

	return "", nil
}

func extractNodeAssignmentTargets(node *models.WorkflowNode) ([]string, []string, []string) {
	rawTargets := make([]string, 0, len(node.AssignedTargets))
	rawTargets = append(rawTargets, node.AssignedTargets...)
	if len(rawTargets) == 0 {
		for _, role := range node.AssignedRoles {
			role = strings.TrimSpace(role)
			if role == "" {
				continue
			}
			rawTargets = append(rawTargets, "#"+role)
		}
		for _, user := range node.AssignedUsers {
			user = strings.TrimSpace(user)
			if user == "" {
				continue
			}
			rawTargets = append(rawTargets, "@"+user)
		}
	}
	if len(rawTargets) == 0 {
		if role := strings.TrimSpace(node.AssignedRole); role != "" {
			rawTargets = append(rawTargets, "#"+role)
		}
		if user := strings.TrimSpace(node.AssignedUser); user != "" {
			rawTargets = append(rawTargets, "@"+user)
		}
	}

	targets := make([]string, 0, len(rawTargets))
	roles := make([]string, 0, len(rawTargets))
	users := make([]string, 0, len(rawTargets))
	seenTargets := map[string]struct{}{}
	seenRoles := map[string]struct{}{}
	seenUsers := map[string]struct{}{}

	for _, raw := range rawTargets {
		token := normalizeAssignmentToken(raw)
		if token == "" {
			continue
		}
		targetKey := strings.ToLower(token)
		if _, ok := seenTargets[targetKey]; ok {
			continue
		}
		seenTargets[targetKey] = struct{}{}
		targets = append(targets, token)

		if strings.HasPrefix(token, "#") {
			role := strings.TrimSpace(strings.TrimPrefix(token, "#"))
			roleKey := strings.ToLower(role)
			if role != "" {
				if _, ok := seenRoles[roleKey]; !ok {
					seenRoles[roleKey] = struct{}{}
					roles = append(roles, role)
				}
			}
			continue
		}

		if strings.HasPrefix(token, "@") {
			user := strings.TrimSpace(strings.TrimPrefix(token, "@"))
			userKey := strings.ToLower(user)
			if user != "" {
				if _, ok := seenUsers[userKey]; !ok {
					seenUsers[userKey] = struct{}{}
					users = append(users, user)
				}
			}
		}
	}

	return targets, roles, users
}

func normalizeAssignmentToken(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if !strings.HasPrefix(text, "#") && !strings.HasPrefix(text, "@") {
		return ""
	}
	prefix := text[:1]
	body := strings.TrimSpace(text[1:])
	body = strings.Join(strings.Fields(body), " ")
	if body == "" {
		return ""
	}
	return prefix + body
}

func (e *Executor) executeAction(instanceID, orgID string, node *models.WorkflowNode, data map[string]interface{}) error {
	if node.Connector == nil {
		return fmt.Errorf("missing connector configuration")
	}
	if node.Connector.Type != models.ConnectorEmail || e.email == nil {
		return fmt.Errorf("unsupported connector type: %s", node.Connector.Type)
	}
	to := resolveParam(node.Connector.Params, "to", data)
	cc := resolveParam(node.Connector.Params, "cc", data)
	subject := resolveParam(node.Connector.Params, "subject", data)
	body := resolveParam(node.Connector.Params, "body_template", data)
	fromName := resolveParam(node.Connector.Params, "from_name", data)
	fromAccountID := resolveParam(node.Connector.Params, "from_account_id", data)
	if to == "" {
		return fmt.Errorf("missing recipient (to)")
	}

	var err error
	if sender, ok := e.email.(connectors.OrgEmailConnector); ok {
		err = sender.SendForOrg(orgID, to, cc, subject, body, fromName, fromAccountID)
	} else {
		err = e.email.Send(to, subject, body)
	}
	if err != nil {
		log.Printf("executor: email send failed: %v", err)
		e.audit(instanceID, node.ID, "action_failed", map[string]interface{}{"connector": "email", "error": err.Error()})
		return err
	}
	e.audit(instanceID, node.ID, "email_sent", map[string]interface{}{
		"to":      to,
		"subject": subject,
	})
	return nil
}

func (e *Executor) ContinueTask(taskID, actorUserID, action, comment, authHeader string) (models.TaskAssignment, error) {
	task, ok := e.store.GetTask(taskID)
	if !ok {
		return models.TaskAssignment{}, ErrTaskNotFound
	}
	if isTerminalTaskStatus(task.Status) {
		return models.TaskAssignment{}, ErrTaskAlreadyCompleted
	}
	comment = strings.TrimSpace(comment)
	actorUserID = strings.TrimSpace(actorUserID)
	if task.Status == models.TaskPending && action != "start" && !isEscalationAction(action) {
		return models.TaskAssignment{}, ErrPendingTaskStartOnly
	}
	if action != "start" && comment == "" {
		return models.TaskAssignment{}, ErrCommentRequired
	}
	if err := e.CanActOnTask(actorUserID, task, action, authHeader); err != nil {
		return models.TaskAssignment{}, err
	}

	if action != "start" && !isEscalationAction(action) && len(task.AllowedActions) > 0 {
		allowed := false
		for _, a := range task.AllowedActions {
			if a == action {
				allowed = true
				break
			}
		}
		if !allowed {
			return models.TaskAssignment{}, fmt.Errorf("%w: %s", ErrUnknownAction, action)
		}
	}

	inst, ok := e.store.GetInstance(task.InstanceID)
	if !ok {
		return models.TaskAssignment{}, ErrInstanceNotFound
	}
	wf, ok := e.store.GetWorkflow(task.WorkflowID)
	if !ok {
		return models.TaskAssignment{}, ErrWorkflowNotFound
	}
	node := wf.FindNode(task.NodeID)
	if node == nil {
		return models.TaskAssignment{}, ErrTaskNodeNotFound
	}

	prevStatus := task.Status
	now := time.Now()
	task.Comment = comment
	task.CompletedAt = &now
	switch action {
	case "start":
		if task.Status != models.TaskPending {
			return models.TaskAssignment{}, fmt.Errorf("%w: cannot start from status %s", ErrForbiddenTaskAction, task.Status)
		}
		if strings.TrimSpace(task.AssignedUser) == "" {
			task.AssignedUser = actorUserID
		}
		task.Comment = ""
		task.Status = models.TaskInProgress
		task.CompletedAt = nil
		swapped, err := e.store.CompareAndSwapTask(task, prevStatus)
		if err != nil {
			return models.TaskAssignment{}, fmt.Errorf("save task: %w", err)
		}
		if !swapped {
			return models.TaskAssignment{}, ErrTaskConflict
		}
		e.audit(task.InstanceID, task.NodeID, "task_started", map[string]interface{}{
			"task_id":       task.ID,
			"actor":         actorUserID,
			"assigned_user": task.AssignedUser,
		})
		return task, nil
	case "escalate_notify":
		if task.Status != models.TaskPending {
			return models.TaskAssignment{}, fmt.Errorf("%w: cannot escalate from status %s", ErrForbiddenTaskAction, task.Status)
		}
		task.Status = models.TaskPending
		task.ActionCommitted = action
		task.CompletedAt = nil
		if task.SLADays > 1 {
			task.SLADays--
		}
		swapped, err := e.store.CompareAndSwapTask(task, prevStatus)
		if err != nil {
			return models.TaskAssignment{}, fmt.Errorf("save task: %w", err)
		}
		if !swapped {
			return models.TaskAssignment{}, ErrTaskConflict
		}
		e.audit(task.InstanceID, task.NodeID, "task_escalated_notify", map[string]interface{}{
			"task_id":       task.ID,
			"actor":         actorUserID,
			"assigned_user": task.AssignedUser,
			"assigned_role": task.AssignedRole,
			"comment":       comment,
			"sla_days":      task.SLADays,
		})
		return task, nil
	case "escalate_reassign":
		if task.Status != models.TaskPending {
			return models.TaskAssignment{}, fmt.Errorf("%w: cannot escalate from status %s", ErrForbiddenTaskAction, task.Status)
		}
		candidates, err := e.ListEscalationCandidates(task, authHeader)
		if err != nil {
			return models.TaskAssignment{}, err
		}
		if len(candidates) == 0 {
			return models.TaskAssignment{}, ErrNoEligibleAssignee
		}
		previousAssignee := strings.TrimSpace(task.AssignedUser)
		task.AssignedUser = candidates[0]
		task.Status = models.TaskPending
		task.ActionCommitted = action
		task.CompletedAt = nil
		if task.SLADays > 1 {
			task.SLADays--
		}
		swapped, err := e.store.CompareAndSwapTask(task, prevStatus)
		if err != nil {
			return models.TaskAssignment{}, fmt.Errorf("save task: %w", err)
		}
		if !swapped {
			return models.TaskAssignment{}, ErrTaskConflict
		}
		e.audit(task.InstanceID, task.NodeID, "task_escalated_reassigned", map[string]interface{}{
			"task_id":            task.ID,
			"actor":              actorUserID,
			"from_assigned_user": previousAssignee,
			"to_assigned_user":   task.AssignedUser,
			"assigned_role":      task.AssignedRole,
			"comment":            comment,
			"sla_days":           task.SLADays,
		})
		return task, nil
	case "approve":
		task.Status = models.TaskCompleted
		task.ActionCommitted = action
	case "reject":
		task.Status = models.TaskCompleted
		task.ActionCommitted = action
	case "clarify":
		task.Status = models.TaskCompleted
		task.ActionCommitted = action
	case "complete":
		task.Status = models.TaskCompleted
		task.ActionCommitted = action
	default:
		return models.TaskAssignment{}, fmt.Errorf("%w: %s", ErrUnknownAction, action)
	}
	swapped, err := e.store.CompareAndSwapTask(task, prevStatus)
	if err != nil {
		return models.TaskAssignment{}, fmt.Errorf("save task: %w", err)
	}
	if !swapped {
		return models.TaskAssignment{}, ErrTaskConflict
	}

	e.audit(task.InstanceID, task.NodeID, "task_action", map[string]interface{}{
		"task_id":       task.ID,
		"action":        action,
		"actor":         actorUserID,
		"assigned_user": task.AssignedUser,
		"assigned_role": task.AssignedRole,
		"comment":       comment,
		"decision_at":   now.Format(time.RFC3339),
	})

	inst, err = e.saveInstanceMutation(task.InstanceID, func(inst *models.Instance) {
		inst.Status = models.InstanceRunning
		inst.CurrentNode = task.NodeID
	})
	if err != nil {
		log.Printf("executor: failed to save running instance after task CAS instance_id=%s task_id=%s action=%s: %v", task.InstanceID, task.ID, action, err)
		inst, ok = e.store.GetInstance(task.InstanceID)
		if !ok {
			inst = models.Instance{Data: task.Data}
		}
	}

	e.setNodeState(task.InstanceID, task.NodeID, "completed")
	e.walkNext(task.InstanceID, node, action, &wf, inst.Data, authHeader)

	hasActiveTasks, err := e.store.HasActiveTasks(task.InstanceID)
	if err != nil {
		return models.TaskAssignment{}, fmt.Errorf("check active tasks: %w", err)
	}
	finalInst, ok := e.store.GetInstance(task.InstanceID)
	if ok && finalInst.Status == models.InstanceRunning && !hasActiveTasks {
		if _, err := e.saveInstanceMutation(task.InstanceID, func(inst *models.Instance) {
			nowDone := time.Now()
			inst.Status = models.InstanceCompleted
			inst.CompletedAt = &nowDone
			inst.AuditLog = append(inst.AuditLog, models.AuditEntry{Timestamp: nowDone, Action: "instance_completed"})
		}); err != nil {
			log.Printf("executor: failed to save completed instance instance_id=%s status=%s action=%s: %v", task.InstanceID, models.InstanceCompleted, "instance_completed", err)
		}
	}

	return task, nil
}

func (e *Executor) setNodeState(instanceID, nodeID, status string) {
	if _, err := e.saveInstanceMutation(instanceID, func(inst *models.Instance) {
		if inst.NodeStates == nil {
			inst.NodeStates = make(map[string]models.NodeState)
		}
		ns := inst.NodeStates[nodeID]
		now := time.Now()
		ns.Status = status
		if status == "running" {
			ns.StartedAt = &now
		}
		if status == "completed" || status == "failed" {
			ns.CompletedAt = &now
		}
		inst.NodeStates[nodeID] = ns
		inst.CurrentNode = nodeID
	}); err != nil {
		log.Printf("executor: failed to persist node state instance_id=%s node_id=%s status=%s: %v", instanceID, nodeID, status, err)
	}
}

func (e *Executor) audit(instanceID, nodeID, action string, details map[string]interface{}) {
	if _, err := e.saveInstanceMutation(instanceID, func(inst *models.Instance) {
		inst.AuditLog = append(inst.AuditLog, models.AuditEntry{
			Timestamp: time.Now(),
			NodeID:    nodeID,
			Action:    action,
			Details:   details,
		})
	}); err != nil {
		log.Printf("executor: failed to append audit entry instance_id=%s node_id=%s action=%s: %v", instanceID, nodeID, action, err)
	}
}

func (e *Executor) markFailed(instanceID, reason string) {
	if _, err := e.saveInstanceMutation(instanceID, func(inst *models.Instance) {
		now := time.Now()
		inst.Status = models.InstanceFailed
		inst.CompletedAt = &now
		inst.AuditLog = append(inst.AuditLog, models.AuditEntry{
			Timestamp: now,
			Action:    "instance_failed",
			Details:   map[string]interface{}{"reason": reason},
		})
	}); err != nil {
		log.Printf("executor: failed to mark instance failed instance_id=%s: %v", instanceID, err)
	}
}

func (e *Executor) isInstanceTerminal(instanceID string) bool {
	inst, ok := e.store.GetInstance(instanceID)
	if !ok {
		return true
	}
	switch inst.Status {
	case models.InstanceFailed, models.InstanceCancelled, models.InstanceCompleted:
		return true
	default:
		return false
	}
}

func (e *Executor) markNodeFailed(instanceID, nodeID, output string) {
	if _, err := e.saveInstanceMutation(instanceID, func(inst *models.Instance) {
		if inst.NodeStates == nil {
			inst.NodeStates = make(map[string]models.NodeState)
		}
		ns := inst.NodeStates[nodeID]
		now := time.Now()
		ns.Status = "failed"
		ns.Output = strings.TrimSpace(output)
		ns.CompletedAt = &now
		if ns.StartedAt == nil {
			ns.StartedAt = &now
		}
		inst.NodeStates[nodeID] = ns
		inst.CurrentNode = nodeID
	}); err != nil {
		log.Printf("executor: failed to persist failed node state instance_id=%s node_id=%s: %v", instanceID, nodeID, err)
	}
}

func (e *Executor) evalConditionForNode(node *models.WorkflowNode, wf *models.Workflow, data map[string]interface{}) (string, map[string]interface{}) {
	details := map[string]interface{}{
		"mode": "structured",
	}
	if node == nil || node.ConditionConfig == nil {
		details["reason"] = "missing_condition_config"
		return "no", details
	}

	cfg := node.ConditionConfig
	join := strings.ToLower(strings.TrimSpace(string(cfg.Join)))
	if join != string(models.ConditionJoinOr) {
		join = string(models.ConditionJoinAnd)
	}
	details["join"] = join

	rules := make([]models.ConditionRule, 0, len(cfg.Rules))
	for _, rule := range cfg.Rules {
		if strings.TrimSpace(rule.Field) == "" {
			continue
		}
		if strings.TrimSpace(string(rule.Operator)) == "" {
			continue
		}
		rules = append(rules, rule)
	}
	details["rules_count"] = len(rules)
	if len(rules) == 0 {
		details["reason"] = "no_rules"
		return "no", details
	}

	ruleResults := make([]bool, len(rules))
	matchedCount := 0
	firstFailedIndex := -1
	firstFailedReason := ""
	firstFailedType := models.ConditionDataType("")

	for idx, rule := range rules {
		ruleMatched, ruleReason, dataType := e.evalConditionRule(rule, wf, data)
		ruleResults[idx] = ruleMatched
		if ruleMatched {
			matchedCount++
			continue
		}
		if firstFailedIndex == -1 {
			firstFailedIndex = idx
			firstFailedReason = ruleReason
			firstFailedType = dataType
		}
	}

	details["matched_rules"] = matchedCount
	if firstFailedIndex >= 0 {
		details["failed_rule_index"] = firstFailedIndex
		details["data_type"] = string(firstFailedType)
	}

	logic := strings.TrimSpace(cfg.Logic)
	if logic != "" {
		details["logic"] = logic
		logicMatched, err := evaluateConditionLogicExpression(logic, ruleResults)
		if err != nil {
			details["reason"] = "invalid_logic_expression"
			details["logic_error"] = err.Error()
			return "no", details
		}
		if logicMatched {
			return "yes", details
		}
		details["reason"] = "logic_expression_false"
		if firstFailedReason != "" {
			details["rule_reason"] = firstFailedReason
		}
		return "no", details
	}

	if join == string(models.ConditionJoinAnd) {
		if matchedCount == len(rules) {
			return "yes", details
		}
		if firstFailedReason != "" {
			details["reason"] = firstFailedReason
		}
		return "no", details
	}

	if matchedCount > 0 {
		for idx, matched := range ruleResults {
			if matched {
				details["matched_rule_index"] = idx
				break
			}
		}
		return "yes", details
	}

	if firstFailedReason != "" {
		details["reason"] = firstFailedReason
	} else {
		details["reason"] = "no_rule_matched"
	}
	return "no", details
}

func evaluateConditionLogicExpression(raw string, ruleResults []bool) (bool, error) {
	tokens, err := tokenizeConditionLogicExpression(raw)
	if err != nil {
		return false, err
	}
	if len(tokens) == 0 {
		return false, errors.New("logic expression is empty")
	}

	parser := &conditionLogicParser{
		tokens:      tokens,
		ruleResults: ruleResults,
	}
	value, parseErr := parser.parseExpr()
	if parseErr != nil {
		return false, parseErr
	}
	if parser.pos != len(tokens) {
		return false, fmt.Errorf("unexpected token %q", tokens[parser.pos])
	}
	return value, nil
}

func tokenizeConditionLogicExpression(raw string) ([]string, error) {
	input := strings.TrimSpace(raw)
	if input == "" {
		return nil, nil
	}

	tokens := make([]string, 0, len(input)/2)
	for i := 0; i < len(input); {
		ch := input[i]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			i++
			continue
		}
		if ch == '(' || ch == ')' {
			tokens = append(tokens, string(ch))
			i++
			continue
		}
		if ch >= '0' && ch <= '9' {
			j := i + 1
			for j < len(input) && input[j] >= '0' && input[j] <= '9' {
				j++
			}
			tokens = append(tokens, input[i:j])
			i = j
			continue
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			j := i + 1
			for j < len(input) {
				next := input[j]
				if !((next >= 'A' && next <= 'Z') || (next >= 'a' && next <= 'z')) {
					break
				}
				j++
			}
			word := strings.ToUpper(input[i:j])
			if word != "AND" && word != "OR" {
				return nil, fmt.Errorf("unsupported token %q", word)
			}
			tokens = append(tokens, word)
			i = j
			continue
		}
		return nil, fmt.Errorf("invalid character %q", string(ch))
	}

	return tokens, nil
}

type conditionLogicParser struct {
	tokens      []string
	pos         int
	ruleResults []bool
}

func (p *conditionLogicParser) parseExpr() (bool, error) {
	left, err := p.parseTerm()
	if err != nil {
		return false, err
	}
	for p.pos < len(p.tokens) && p.tokens[p.pos] == "OR" {
		p.pos++
		right, rightErr := p.parseTerm()
		if rightErr != nil {
			return false, rightErr
		}
		left = left || right
	}
	return left, nil
}

func (p *conditionLogicParser) parseTerm() (bool, error) {
	left, err := p.parseFactor()
	if err != nil {
		return false, err
	}
	for p.pos < len(p.tokens) && p.tokens[p.pos] == "AND" {
		p.pos++
		right, rightErr := p.parseFactor()
		if rightErr != nil {
			return false, rightErr
		}
		left = left && right
	}
	return left, nil
}

func (p *conditionLogicParser) parseFactor() (bool, error) {
	if p.pos >= len(p.tokens) {
		return false, errors.New("unexpected end of logic expression")
	}

	token := p.tokens[p.pos]
	if token == "(" {
		p.pos++
		value, err := p.parseExpr()
		if err != nil {
			return false, err
		}
		if p.pos >= len(p.tokens) || p.tokens[p.pos] != ")" {
			return false, errors.New("missing closing parenthesis")
		}
		p.pos++
		return value, nil
	}

	ruleIndex, err := strconv.Atoi(token)
	if err != nil || ruleIndex <= 0 {
		return false, fmt.Errorf("invalid rule reference %q", token)
	}
	if ruleIndex > len(p.ruleResults) {
		return false, fmt.Errorf("rule reference %d is out of range (max %d)", ruleIndex, len(p.ruleResults))
	}

	p.pos++
	return p.ruleResults[ruleIndex-1], nil
}

func (e *Executor) evalConditionRule(rule models.ConditionRule, wf *models.Workflow, data map[string]interface{}) (bool, string, models.ConditionDataType) {
	field := strings.TrimSpace(rule.Field)
	if field == "" {
		return false, "field_missing", ""
	}
	rawValue, found := data[field]
	if !found {
		return false, "field_missing", ""
	}

	dataType := normalizeConditionDataType(string(rule.DataType))
	if dataType == "" {
		dataType = inferConditionDataTypeFromTriggerField(wf, field)
	}
	if dataType == "" {
		dataType = inferConditionDataTypeFromValue(rawValue)
	}
	if dataType == "" {
		dataType = models.ConditionDataTypeText
	}

	if !isOperatorSupportedForDataType(dataType, rule.Operator) {
		return false, "operator_not_supported_for_data_type", dataType
	}

	matched, reason := evaluateTypedCondition(dataType, rule.Operator, rawValue, rule.Value)
	if reason != "" {
		return false, reason, dataType
	}
	if matched {
		return true, "", dataType
	}
	return false, "rule_not_matched", dataType
}

func normalizeConditionDataType(raw string) models.ConditionDataType {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "number", "numeric", "int", "float", "decimal":
		return models.ConditionDataTypeNumber
	case "boolean", "bool":
		return models.ConditionDataTypeBoolean
	case "date":
		return models.ConditionDataTypeDate
	case "datetime", "date_time", "timestamp":
		return models.ConditionDataTypeDateTime
	case "time":
		return models.ConditionDataTypeTime
	case "text", "string":
		return models.ConditionDataTypeText
	case "":
		return ""
	default:
		return ""
	}
}

func inferConditionDataTypeFromTriggerField(wf *models.Workflow, field string) models.ConditionDataType {
	if wf == nil || wf.Trigger.Config == nil {
		return ""
	}
	raw := strings.TrimSpace(wf.Trigger.Config["field_schema"])
	if raw == "" {
		return ""
	}
	var schema []struct {
		QuestionID string `json:"question_id"`
		FieldType  string `json:"field_type"`
		Variable   string `json:"variable"`
		DataType   string `json:"data_type"`
	}
	if err := json.Unmarshal([]byte(raw), &schema); err != nil {
		return ""
	}
	for _, item := range schema {
		if strings.TrimSpace(item.Variable) != field && strings.TrimSpace(item.QuestionID) != field {
			continue
		}
		if dataType := normalizeConditionDataType(item.DataType); dataType != "" {
			return dataType
		}
		switch strings.ToLower(strings.TrimSpace(item.FieldType)) {
		case "scale":
			return models.ConditionDataTypeNumber
		case "boolean", "bool", "yes_no":
			return models.ConditionDataTypeBoolean
		case "date":
			return models.ConditionDataTypeDate
		case "datetime", "date_time", "timestamp", "iso8601":
			return models.ConditionDataTypeDateTime
		case "time":
			return models.ConditionDataTypeTime
		default:
			return models.ConditionDataTypeText
		}
	}
	return ""
}

func inferConditionDataTypeFromValue(value interface{}) models.ConditionDataType {
	switch v := value.(type) {
	case bool:
		return models.ConditionDataTypeBoolean
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return models.ConditionDataTypeNumber
	case time.Time:
		return models.ConditionDataTypeDateTime
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return models.ConditionDataTypeText
		}
		if _, ok := toBool(s); ok {
			return models.ConditionDataTypeBoolean
		}
		if _, ok := toFloat(s); ok {
			return models.ConditionDataTypeNumber
		}
		if _, ok := parseDateValue(s); ok {
			return models.ConditionDataTypeDate
		}
		if _, ok := parseDateTimeValue(s); ok {
			return models.ConditionDataTypeDateTime
		}
		if _, ok := parseTimeValue(s); ok {
			return models.ConditionDataTypeTime
		}
		return models.ConditionDataTypeText
	default:
		return models.ConditionDataTypeText
	}
}

func isOperatorSupportedForDataType(dataType models.ConditionDataType, operator models.ConditionOperator) bool {
	switch dataType {
	case models.ConditionDataTypeText:
		switch operator {
		case models.ConditionOperatorEquals,
			models.ConditionOperatorNotEquals,
			models.ConditionOperatorContains,
			models.ConditionOperatorNotContains,
			models.ConditionOperatorStartsWith,
			models.ConditionOperatorEndsWith,
			models.ConditionOperatorIsEmpty,
			models.ConditionOperatorIsNotEmpty:
			return true
		default:
			return false
		}
	case models.ConditionDataTypeNumber, models.ConditionDataTypeDate, models.ConditionDataTypeDateTime, models.ConditionDataTypeTime:
		switch operator {
		case models.ConditionOperatorEquals,
			models.ConditionOperatorNotEquals,
			models.ConditionOperatorGreaterThan,
			models.ConditionOperatorGreaterEq,
			models.ConditionOperatorLessThan,
			models.ConditionOperatorLessEq,
			models.ConditionOperatorIsEmpty,
			models.ConditionOperatorIsNotEmpty:
			return true
		default:
			return false
		}
	case models.ConditionDataTypeBoolean:
		switch operator {
		case models.ConditionOperatorEquals,
			models.ConditionOperatorNotEquals,
			models.ConditionOperatorIsEmpty,
			models.ConditionOperatorIsNotEmpty:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func evaluateTypedCondition(dataType models.ConditionDataType, operator models.ConditionOperator, left interface{}, right interface{}) (bool, string) {
	if operator == models.ConditionOperatorIsEmpty {
		return isEmptyValue(left), ""
	}
	if operator == models.ConditionOperatorIsNotEmpty {
		return !isEmptyValue(left), ""
	}

	switch dataType {
	case models.ConditionDataTypeText:
		leftValue := strings.TrimSpace(fmt.Sprintf("%v", left))
		rightValue := strings.TrimSpace(fmt.Sprintf("%v", right))
		switch operator {
		case models.ConditionOperatorEquals:
			return leftValue == rightValue, ""
		case models.ConditionOperatorNotEquals:
			return leftValue != rightValue, ""
		case models.ConditionOperatorContains:
			return strings.Contains(strings.ToLower(leftValue), strings.ToLower(rightValue)), ""
		case models.ConditionOperatorNotContains:
			return !strings.Contains(strings.ToLower(leftValue), strings.ToLower(rightValue)), ""
		case models.ConditionOperatorStartsWith:
			return strings.HasPrefix(strings.ToLower(leftValue), strings.ToLower(rightValue)), ""
		case models.ConditionOperatorEndsWith:
			return strings.HasSuffix(strings.ToLower(leftValue), strings.ToLower(rightValue)), ""
		default:
			return false, "unsupported_operator"
		}

	case models.ConditionDataTypeNumber:
		leftNum, leftOK := toFloat(left)
		rightNum, rightOK := toFloat(right)
		if !leftOK || !rightOK {
			return false, "type_mismatch"
		}
		switch operator {
		case models.ConditionOperatorEquals:
			return nearlyEqualFloat(leftNum, rightNum), ""
		case models.ConditionOperatorNotEquals:
			return !nearlyEqualFloat(leftNum, rightNum), ""
		case models.ConditionOperatorGreaterThan:
			return leftNum > rightNum, ""
		case models.ConditionOperatorGreaterEq:
			return leftNum >= rightNum, ""
		case models.ConditionOperatorLessThan:
			return leftNum < rightNum, ""
		case models.ConditionOperatorLessEq:
			return leftNum <= rightNum, ""
		default:
			return false, "unsupported_operator"
		}

	case models.ConditionDataTypeBoolean:
		leftBool, leftOK := toBool(left)
		rightBool, rightOK := toBool(right)
		if !leftOK || !rightOK {
			return false, "type_mismatch"
		}
		switch operator {
		case models.ConditionOperatorEquals:
			return leftBool == rightBool, ""
		case models.ConditionOperatorNotEquals:
			return leftBool != rightBool, ""
		default:
			return false, "unsupported_operator"
		}

	case models.ConditionDataTypeDate:
		leftTime, leftOK := coerceDateValue(left)
		rightTime, rightOK := coerceDateValue(right)
		if !leftOK || !rightOK {
			return false, "type_mismatch"
		}
		return compareTimes(leftTime, rightTime, operator)

	case models.ConditionDataTypeDateTime:
		leftTime, leftOK := coerceDateTimeValue(left)
		rightTime, rightOK := coerceDateTimeValue(right)
		if !leftOK || !rightOK {
			return false, "type_mismatch"
		}
		return compareTimes(leftTime, rightTime, operator)

	case models.ConditionDataTypeTime:
		leftTime, leftOK := coerceTimeValue(left)
		rightTime, rightOK := coerceTimeValue(right)
		if !leftOK || !rightOK {
			return false, "type_mismatch"
		}
		return compareTimes(leftTime, rightTime, operator)
	}

	return false, "unsupported_data_type"
}

func nearlyEqualFloat(left, right float64) bool {
	return math.Abs(left-right) <= numberComparisonEpsilon
}

func compareTimes(left, right time.Time, operator models.ConditionOperator) (bool, string) {
	switch operator {
	case models.ConditionOperatorEquals:
		return left.Equal(right), ""
	case models.ConditionOperatorNotEquals:
		return !left.Equal(right), ""
	case models.ConditionOperatorGreaterThan:
		return left.After(right), ""
	case models.ConditionOperatorGreaterEq:
		return left.After(right) || left.Equal(right), ""
	case models.ConditionOperatorLessThan:
		return left.Before(right), ""
	case models.ConditionOperatorLessEq:
		return left.Before(right) || left.Equal(right), ""
	default:
		return false, "unsupported_operator"
	}
}

func coerceDateValue(value interface{}) (time.Time, bool) {
	if t, ok := value.(time.Time); ok {
		y, m, d := t.Date()
		return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), true
	}
	return parseDateValue(fmt.Sprintf("%v", value))
}

func coerceDateTimeValue(value interface{}) (time.Time, bool) {
	if t, ok := value.(time.Time); ok {
		return t, true
	}
	return parseDateTimeValue(fmt.Sprintf("%v", value))
}

func coerceTimeValue(value interface{}) (time.Time, bool) {
	if t, ok := value.(time.Time); ok {
		return t, true
	}
	return parseTimeValue(fmt.Sprintf("%v", value))
}

func parseDateValue(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{"2006-01-02", time.RFC3339, "2006-01-02 15:04:05", "2006-01-02T15:04", "2006-01-02T15:04:05"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			y, m, d := parsed.Date()
			return time.Date(y, m, d, 0, 0, 0, 0, time.UTC), true
		}
	}
	return time.Time{}, false
}

func parseDateTimeValue(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{time.RFC3339, "2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04:05"}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func parseTimeValue(raw string) (time.Time, bool) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{"15:04", "15:04:05", time.Kitchen}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, true
		}
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func isEmptyValue(value interface{}) bool {
	if value == nil {
		return true
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == ""
	case []string:
		return len(v) == 0
	case []interface{}:
		return len(v) == 0
	case map[string]interface{}:
		return len(v) == 0
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value)) == ""
	}
}

func toBool(v interface{}) (bool, bool) {
	switch value := v.(type) {
	case bool:
		return value, true
	case string:
		normalized := strings.TrimSpace(strings.ToLower(value))
		switch normalized {
		case "true", "1", "yes", "y":
			return true, true
		case "false", "0", "no", "n":
			return false, true
		default:
			return false, false
		}
	default:
		if num, ok := toFloat(v); ok {
			return num != 0, true
		}
		return false, false
	}
}

func toFloat(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		s, err := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
		if err != nil {
			return 0, false
		}
		return s, true
	}
}

func resolveParam(params map[string]interface{}, key string, data map[string]interface{}) string {
	if params == nil {
		return ""
	}
	raw, ok := params[key]
	if !ok {
		return ""
	}
	s := fmt.Sprintf("%v", raw)
	for k, v := range data {
		token := "{{data." + k + "}}"
		s = strings.ReplaceAll(s, token, fmt.Sprintf("%v", v))
	}
	return s
}

func (e *Executor) GetTasksByAssignee(orgID, userID string) ([]models.TaskAssignment, error) {
	return e.store.ListTasksByAssignee(orgID, userID)
}

func isTerminalTaskStatus(status models.TaskStatus) bool {
	switch status {
	case models.TaskApproved, models.TaskRejected, models.TaskCompleted:
		return true
	default:
		return false
	}
}

func isEscalationAction(action string) bool {
	switch strings.TrimSpace(action) {
	case "escalate_notify", "escalate_reassign":
		return true
	default:
		return false
	}
}

func (e *Executor) CanActOnTask(actorUserID string, task models.TaskAssignment, action, authHeader string) error {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return ErrForbiddenTaskAction
	}
	if task.Status == models.TaskPending && isEscalationAction(action) {
		return nil
	}
	if task.Status == models.TaskPending && action != "start" {
		return ErrPendingTaskStartOnly
	}

	assignedUser := strings.TrimSpace(task.AssignedUser)
	if assignedUser != "" {
		if actorUserID != assignedUser {
			return ErrForbiddenTaskAction
		}
		return nil
	}

	if action != "start" {
		return ErrForbiddenTaskAction
	}

	canClaim, err := e.canClaimTask(actorUserID, task, authHeader)
	if err != nil {
		return err
	}
	if !canClaim {
		return ErrTaskClaimNotAllowed
	}
	return nil
}

func (e *Executor) ListEscalationCandidates(task models.TaskAssignment, authHeader string) ([]string, error) {
	candidates, err := e.collectEligibleAssigneeIDs(task, authHeader)
	if err != nil {
		return nil, err
	}
	currentAssignee := strings.TrimSpace(task.AssignedUser)
	if currentAssignee == "" {
		sort.Strings(candidates)
		return candidates, nil
	}
	filtered := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.EqualFold(strings.TrimSpace(candidate), currentAssignee) {
			continue
		}
		filtered = append(filtered, candidate)
	}
	sort.Strings(filtered)
	return filtered, nil
}

func (e *Executor) collectEligibleAssigneeIDs(task models.TaskAssignment, authHeader string) ([]string, error) {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(task.AssignedUsers)+4)
	add := func(userID string) {
		trimmed := strings.TrimSpace(userID)
		if trimmed == "" {
			return
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}

	for _, userID := range task.AssignedUsers {
		add(userID)
	}
	if strings.TrimSpace(task.AssignedUser) != "" {
		add(task.AssignedUser)
	}

	roleNames := uniqueNormalizedStrings(task.AssignedRoles)
	if len(roleNames) == 0 {
		legacyRole := strings.TrimSpace(task.AssignedRole)
		if legacyRole != "" {
			roleNames = append(roleNames, legacyRole)
		}
	}

	if len(roleNames) > 0 {
		directory := e.roleDirectory()
		if directory == nil {
			return nil, fmt.Errorf("role directory unavailable for escalation")
		}
		for _, roleName := range roleNames {
			memberIDs, err := e.listRoleMemberIDs(directory, task.OrgID, roleName, authHeader)
			if err != nil {
				if errors.Is(err, ErrRoleNotFound) || errors.Is(err, ErrNoMembers) {
					continue
				}
				return nil, err
			}
			for _, memberID := range memberIDs {
				add(memberID)
			}
		}
	}

	return out, nil
}

func (e *Executor) canClaimTask(actorUserID string, task models.TaskAssignment, authHeader string) (bool, error) {
	allowedUsers := uniqueNormalizedStrings(task.AssignedUsers)
	allowedRoles := uniqueNormalizedStrings(task.AssignedRoles)
	if len(allowedRoles) == 0 {
		legacyRole := strings.TrimSpace(task.AssignedRole)
		if legacyRole != "" {
			allowedRoles = append(allowedRoles, legacyRole)
		}
	}

	restricted := len(allowedUsers) > 0 || len(allowedRoles) > 0

	for _, allowedUser := range allowedUsers {
		if strings.EqualFold(strings.TrimSpace(allowedUser), actorUserID) {
			return true, nil
		}
	}

	if len(allowedRoles) == 0 {
		if restricted {
			return false, nil
		}
		return true, nil
	}

	directory := e.roleDirectory()
	if directory == nil {
		log.Printf("executor: roleDirectory returned nil for task claim roles=%v task_id=%q restricted=%v", allowedRoles, task.ID, restricted)
		if restricted {
			return false, fmt.Errorf("role directory unavailable for restricted task claim")
		}
		return true, nil
	}

	for _, roleName := range allowedRoles {
		memberIDs, err := e.listRoleMemberIDs(directory, task.OrgID, roleName, authHeader)
		if err != nil {
			if errors.Is(err, ErrRoleNotFound) || errors.Is(err, ErrNoMembers) {
				continue
			}
			return false, err
		}
		for _, memberID := range memberIDs {
			if strings.TrimSpace(memberID) == actorUserID {
				return true, nil
			}
		}
	}

	if restricted {
		return false, nil
	}
	return true, nil
}

func uniqueNormalizedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		key := strings.ToLower(trimmed)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

func (e *Executor) listRoleMemberIDs(directory RoleMemberDirectory, orgID, roleName, authHeader string) ([]string, error) {
	type authAwareRoleMemberDirectory interface {
		ListMemberIDsWithAuth(orgID, roleName, authHeader string) ([]string, error)
	}

	if authAware, ok := directory.(authAwareRoleMemberDirectory); ok {
		return authAware.ListMemberIDsWithAuth(orgID, roleName, authHeader)
	}
	if strings.TrimSpace(authHeader) != "" {
		log.Printf("executor: role directory %T does not support auth-aware member lookup; falling back to ListMemberIDs without auth header", directory)
	}
	return directory.ListMemberIDs(orgID, roleName)
}

func cloneWorkflowData(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}
	cloned := make(map[string]interface{}, len(data))
	for key, value := range data {
		cloned[key] = value
	}
	return cloned
}

func buildTaskVisibleData(node *models.WorkflowNode, data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	mode := strings.ToLower(strings.TrimSpace(node.TaskDataVisibility))
	if mode == "" {
		mode = "all"
	}

	visible := make(map[string]interface{})
	switch mode {
	case "all":
		for key, value := range data {
			visible[key] = value
		}
	case "none":
		// Intentionally leave empty unless explicit include toggles are enabled.
	case "selected":
		for _, rawKey := range node.VisibleDataKeys {
			key := strings.TrimSpace(rawKey)
			if key == "" {
				continue
			}
			if value, ok := data[key]; ok {
				visible[key] = value
			}
		}
	default:
		// Unknown modes are treated as none to avoid unintentionally exposing all task data.
	}

	if node.IncludeFormSubmission {
		if value, ok := data["form_submission"]; ok {
			visible["form_submission"] = value
		}
	}

	if node.IncludeFormFiles {
		files := extractFormFileRefs(data["form_submission"])
		if len(files) > 0 {
			visible["form_submission_files"] = files
		}
	}

	if len(visible) == 0 {
		return nil
	}
	return visible
}

func extractFormFileRefs(value interface{}) []string {
	out := make([]string, 0)
	seen := make(map[string]struct{})

	var visit func(v interface{})
	visit = func(v interface{}) {
		switch x := v.(type) {
		case map[string]interface{}:
			for _, child := range x {
				visit(child)
			}
		case []interface{}:
			for _, child := range x {
				visit(child)
			}
		case []string:
			for _, child := range x {
				visit(child)
			}
		case string:
			for _, part := range strings.Split(x, ",") {
				candidate := strings.TrimSpace(part)
				if candidate == "" {
					continue
				}
				resolvedURL := candidate
				if strings.Contains(candidate, "|") {
					parts := strings.SplitN(candidate, "|", 2)
					resolvedURL = strings.TrimSpace(parts[1])
				}
				if !isLikelyFileURL(resolvedURL) {
					continue
				}
				if _, exists := seen[resolvedURL]; exists {
					continue
				}
				seen[resolvedURL] = struct{}{}
				out = append(out, resolvedURL)
			}
		}
	}

	visit(value)
	return out
}

func isLikelyFileURL(value string) bool {
	if value == "" {
		return false
	}
	lower := strings.ToLower(value)
	if !strings.HasPrefix(lower, "https://") && !strings.HasPrefix(lower, "http://") {
		return false
	}
	return strings.Contains(lower, "drive.google.com") ||
		strings.Contains(lower, "googleusercontent.com") ||
		strings.Contains(lower, "/file/") ||
		strings.Contains(lower, "upload")
}

func (e *Executor) saveInstanceMutation(instanceID string, mutate func(inst *models.Instance)) (models.Instance, error) {
	lock := e.instanceLock(instanceID)
	lock.Lock()
	defer lock.Unlock()

	inst, ok := e.store.GetInstance(instanceID)
	if !ok {
		return models.Instance{}, ErrInstanceNotFound
	}
	mutate(&inst)
	_, err := e.store.SaveInstance(inst)
	return inst, err
}

func (e *Executor) instanceLock(instanceID string) *sync.Mutex {
	e.mu.Lock()
	defer e.mu.Unlock()

	lock, ok := e.instanceLocks[instanceID]
	if !ok {
		lock = &sync.Mutex{}
		e.instanceLocks[instanceID] = lock
	}
	return lock
}

func (e *Executor) roleDirectory() RoleMemberDirectory {
	type directoryProvider interface {
		Directory() RoleMemberDirectory
	}

	provider, ok := e.assigneeSelector.(directoryProvider)
	if !ok {
		return nil
	}
	return provider.Directory()
}

func (e *Executor) selectAssignee(orgID, roleName, preferredUserID, authHeader string) (string, error) {
	type authAwareTaskAssigneeSelector interface {
		SelectWithAuth(orgID, roleName, preferredUserID, authHeader string) (string, error)
	}

	if authAware, ok := e.assigneeSelector.(authAwareTaskAssigneeSelector); ok {
		return authAware.SelectWithAuth(orgID, roleName, preferredUserID, authHeader)
	}
	return e.assigneeSelector.Select(orgID, roleName, preferredUserID)
}
