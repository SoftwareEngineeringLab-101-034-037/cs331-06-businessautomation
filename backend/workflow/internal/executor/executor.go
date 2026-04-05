package executor

import (
	"errors"
	"fmt"
	"log"
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
	ErrWorkflowNotFound     = errors.New("workflow not found")
	ErrTaskNodeNotFound     = errors.New("task node not found in workflow")
	ErrForbiddenTaskAction  = errors.New("forbidden task action")
	ErrTaskClaimNotAllowed  = errors.New("task claim not allowed")
	ErrPendingTaskStartOnly = errors.New("pending tasks can only be started")
)

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
		branch := e.evalCondition(node.Condition, data)
		e.audit(instanceID, nodeID, "condition_evaluated", map[string]interface{}{
			"expression": node.Condition,
			"branch":     branch,
		})
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
	assignedUser := node.AssignedUser
	if e.assigneeSelector != nil {
		resolvedUser, err := e.selectAssignee(wf.OrgID, node.AssignedRole, node.AssignedUser, authHeader)
		if err != nil {
			// Do not fail workflow execution on assignee lookup issues.
			// Create the task anyway so it remains visible for manual pickup.
			log.Printf("executor: assignee lookup failed for role=%q org=%q: %v; creating unassigned task", node.AssignedRole, wf.OrgID, err)
			e.audit(instanceID, node.ID, "assignee_lookup_failed", map[string]interface{}{
				"role":  node.AssignedRole,
				"error": err.Error(),
			})
			resolvedUser = ""
		}
		assignedUser = resolvedUser
	}

	task := models.TaskAssignment{
		InstanceID:       instanceID,
		OrgID:            wf.OrgID,
		WorkflowID:       wf.ID,
		NodeID:           node.ID,
		Title:            node.Title,
		Description:      node.Description,
		AssignedRole:     node.AssignedRole,
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
		"task_id":         taskID,
		"assigned_role":   node.AssignedRole,
		"assigned_user":   assignedUser,
		"allowed_actions": node.TaskActions,
	})

	if _, err := e.saveInstanceMutation(instanceID, func(inst *models.Instance) {
		inst.Status = models.InstanceWaiting
		inst.CurrentNode = node.ID
	}); err != nil && !errors.Is(err, ErrInstanceNotFound) {
		log.Printf("executor: failed to save waiting instance instance_id=%s node_id=%s: %v", instanceID, node.ID, err)
	}

	return "", nil
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
	if task.Status == models.TaskPending && action != "start" {
		return models.TaskAssignment{}, ErrPendingTaskStartOnly
	}
	if action != "start" && comment == "" {
		return models.TaskAssignment{}, ErrCommentRequired
	}
	if err := e.CanActOnTask(actorUserID, task, action, authHeader); err != nil {
		return models.TaskAssignment{}, err
	}

	if action != "start" && len(task.AllowedActions) > 0 {
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

func (e *Executor) evalCondition(condition string, data map[string]interface{}) string {
	condition = strings.TrimSpace(condition)
	if condition == "" {
		return "yes"
	}

	ops := []string{"==", "!=", ">=", "<=", ">", "<"}
	for _, op := range ops {
		parts := strings.Split(condition, op)
		if len(parts) != 2 {
			continue
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		v, ok := data[left]
		if !ok {
			return "no"
		}
		leftStr := fmt.Sprintf("%v", v)

		switch op {
		case "==":
			if leftStr == right {
				return "yes"
			}
			return "no"
		case "!=":
			if leftStr != right {
				return "yes"
			}
			return "no"
		case ">", "<", ">=", "<=":
			lf, leftOK := toFloat(v)
			rf, rightOK := toFloat(right)
			if !leftOK || !rightOK {
				log.Printf("executor: evalCondition numeric parse failed condition=%q left=%v right=%q", condition, v, right)
				return "no"
			}
			switch op {
			case ">":
				if lf > rf {
					return "yes"
				}
			case "<":
				if lf < rf {
					return "yes"
				}
			case ">=":
				if lf >= rf {
					return "yes"
				}
			case "<=":
				if lf <= rf {
					return "yes"
				}
			}
			return "no"
		}
	}

	log.Printf("executor: evalCondition malformed expression condition=%q", condition)
	return "no"
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

func (e *Executor) CanActOnTask(actorUserID string, task models.TaskAssignment, action, authHeader string) error {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return ErrForbiddenTaskAction
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

func (e *Executor) canClaimTask(actorUserID string, task models.TaskAssignment, authHeader string) (bool, error) {
	roleName := strings.TrimSpace(task.AssignedRole)
	if roleName == "" {
		return true, nil
	}

	directory := e.roleDirectory()
	if directory == nil {
		log.Printf("executor: roleDirectory returned nil for role-restricted task claim role=%q task_id=%q; allowing claim to avoid blocking workflow", roleName, task.ID)
		return true, nil
	}

	memberIDs, err := e.listRoleMemberIDs(directory, task.OrgID, roleName, authHeader)
	if err != nil {
		if errors.Is(err, ErrRoleNotFound) || errors.Is(err, ErrNoMembers) {
			return false, nil
		}
		return false, err
	}
	for _, memberID := range memberIDs {
		if strings.TrimSpace(memberID) == actorUserID {
			return true, nil
		}
	}
	return false, nil
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
				if !isLikelyFileURL(candidate) {
					continue
				}
				if _, exists := seen[candidate]; exists {
					continue
				}
				seen[candidate] = struct{}{}
				out = append(out, candidate)
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
