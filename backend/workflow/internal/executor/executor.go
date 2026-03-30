package executor

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/example/business-automation/backend/workflow/internal/connectors"
	"github.com/example/business-automation/backend/workflow/internal/models"
	"github.com/example/business-automation/backend/workflow/internal/storage"
)

// Executor handles workflow execution with one goroutine per instance.
type Executor struct {
	store            storage.Store
	email            connectors.EmailConnector
	assigneeSelector TaskAssigneeSelector

	mu           sync.Mutex
	mergeWaiters map[string]int
}

func NewExecutor(s storage.Store, e connectors.EmailConnector, selector TaskAssigneeSelector) *Executor {
	return &Executor{
		store:            s,
		email:            e,
		assigneeSelector: selector,
		mergeWaiters:     make(map[string]int),
	}
}

func (e *Executor) StartInstance(wf models.Workflow, data map[string]interface{}) (string, error) {
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
	go e.run(id, wf, data)
	return id, nil
}

func (e *Executor) run(instanceID string, wf models.Workflow, data map[string]interface{}) {
	log.Printf("executor: running instance=%s workflow=%s", instanceID, wf.ID)

	start := wf.StartNode()
	if start == nil {
		e.markFailed(instanceID, "no start node")
		return
	}

	e.walkNode(instanceID, start.ID, &wf, data)

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

func (e *Executor) walkNode(instanceID, nodeID string, wf *models.Workflow, data map[string]interface{}) {
	node := wf.FindNode(nodeID)
	if node == nil {
		log.Printf("executor: unknown node %s", nodeID)
		return
	}

	e.setNodeState(instanceID, nodeID, "running")

	switch node.Type {
	case models.NodeStart:
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, "", wf, data)
		return

	case models.NodeEnd:
		e.setNodeState(instanceID, nodeID, "completed")
		return

	case models.NodeTask:
		action, err := e.executeTask(instanceID, wf, node, data)
		if err != nil {
			e.setNodeState(instanceID, nodeID, "failed")
			e.markFailed(instanceID, fmt.Sprintf("task node %s: %v", nodeID, err))
			return
		}
		if action == "" {
			// waiting for human action
			return
		}
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, action, wf, data)
		return

	case models.NodeAction:
		e.executeAction(instanceID, node, data)
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, "", wf, data)
		return

	case models.NodeCondition:
		branch := e.evalCondition(node.Condition, data)
		e.audit(instanceID, nodeID, "condition_evaluated", map[string]interface{}{
			"expression": node.Condition,
			"branch":     branch,
		})
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, branch, wf, data)
		return

	case models.NodeParallel:
		e.setNodeState(instanceID, nodeID, "completed")
		for _, nextID := range node.NextBranches {
			e.walkNode(instanceID, nextID, wf, data)
		}
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
		e.walkNext(instanceID, node, "", wf, data)
		return
	}
}

func (e *Executor) walkNext(instanceID string, node *models.WorkflowNode, result string, wf *models.Workflow, data map[string]interface{}) {
	for _, nextID := range node.NextIDs(result) {
		e.walkNode(instanceID, nextID, wf, data)
	}
}

func (e *Executor) executeTask(instanceID string, wf *models.Workflow, node *models.WorkflowNode, data map[string]interface{}) (string, error) {
	assignedUser := node.AssignedUser
	if e.assigneeSelector != nil {
		resolvedUser, err := e.assigneeSelector.Select(wf.OrgID, node.AssignedRole, node.AssignedUser)
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
		Data:             data,
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

	inst, ok := e.store.GetInstance(instanceID)
	if ok {
		inst.Status = models.InstanceWaiting
		inst.CurrentNode = node.ID
		if _, err := e.store.SaveInstance(inst); err != nil {
			log.Printf("executor: failed to save waiting instance: %v", err)
		}
	}

	return "", nil
}

func (e *Executor) executeAction(instanceID string, node *models.WorkflowNode, data map[string]interface{}) {
	if node.Connector == nil {
		e.audit(instanceID, node.ID, "action_skipped", map[string]interface{}{"reason": "missing_connector"})
		return
	}
	if node.Connector.Type != models.ConnectorEmail || e.email == nil {
		e.audit(instanceID, node.ID, "action_skipped", map[string]interface{}{"reason": "unsupported_connector", "type": node.Connector.Type})
		return
	}
	to := resolveParam(node.Connector.Params, "to", data)
	subject := resolveParam(node.Connector.Params, "subject", data)
	body := resolveParam(node.Connector.Params, "body_template", data)
	if to == "" {
		return
	}
	if err := e.email.Send(to, subject, body); err != nil {
		log.Printf("executor: email send failed: %v", err)
		return
	}
	e.audit(instanceID, node.ID, "email_sent", map[string]interface{}{
		"to":      to,
		"subject": subject,
	})
}

func (e *Executor) ContinueTask(taskID, actorUserID, action, comment string) (models.TaskAssignment, error) {
	task, ok := e.store.GetTask(taskID)
	if !ok {
		return models.TaskAssignment{}, fmt.Errorf("task not found")
	}
	if isTerminalTaskStatus(task.Status) {
		return models.TaskAssignment{}, fmt.Errorf("task already completed")
	}
	if action != "start" && strings.TrimSpace(comment) == "" {
		return models.TaskAssignment{}, fmt.Errorf("comment is required for task actions")
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
			return models.TaskAssignment{}, fmt.Errorf("unknown action: %s", action)
		}
	}

	prevStatus := task.Status
	now := time.Now()
	task.Comment = strings.TrimSpace(comment)
	task.CompletedAt = &now
	switch action {
	case "start":
		if task.Status != models.TaskPending {
			return models.TaskAssignment{}, fmt.Errorf("task cannot be started from status %s", task.Status)
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
			return models.TaskAssignment{}, fmt.Errorf("task was updated concurrently")
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
		return models.TaskAssignment{}, fmt.Errorf("unknown action: %s", action)
	}
	swapped, err := e.store.CompareAndSwapTask(task, prevStatus)
	if err != nil {
		return models.TaskAssignment{}, fmt.Errorf("save task: %w", err)
	}
	if !swapped {
		return models.TaskAssignment{}, fmt.Errorf("task was updated concurrently")
	}

	inst, ok := e.store.GetInstance(task.InstanceID)
	if !ok {
		return models.TaskAssignment{}, fmt.Errorf("instance not found")
	}
	wf, ok := e.store.GetWorkflow(task.WorkflowID)
	if !ok {
		return models.TaskAssignment{}, fmt.Errorf("workflow not found")
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

	inst, ok = e.store.GetInstance(task.InstanceID)
	if !ok {
		return models.TaskAssignment{}, fmt.Errorf("instance not found")
	}
	inst.Status = models.InstanceRunning
	inst.CurrentNode = task.NodeID
	if _, err := e.store.SaveInstance(inst); err != nil {
		return models.TaskAssignment{}, fmt.Errorf("save instance: %w", err)
	}

	node := wf.FindNode(task.NodeID)
	if node == nil {
		return models.TaskAssignment{}, fmt.Errorf("task node not found in workflow")
	}
	e.setNodeState(task.InstanceID, task.NodeID, "completed")
	e.walkNext(task.InstanceID, node, action, &wf, inst.Data)

	hasActiveTasks, err := e.store.HasActiveTasks(task.InstanceID)
	if err != nil {
		return models.TaskAssignment{}, fmt.Errorf("check active tasks: %w", err)
	}
	finalInst, ok := e.store.GetInstance(task.InstanceID)
	if ok && finalInst.Status == models.InstanceRunning && !hasActiveTasks {
		nowDone := time.Now()
		finalInst.Status = models.InstanceCompleted
		finalInst.CompletedAt = &nowDone
		finalInst.AuditLog = append(finalInst.AuditLog, models.AuditEntry{Timestamp: nowDone, Action: "instance_completed"})
		_, _ = e.store.SaveInstance(finalInst)
	}

	return task, nil
}

func (e *Executor) setNodeState(instanceID, nodeID, status string) {
	inst, ok := e.store.GetInstance(instanceID)
	if !ok {
		return
	}
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
	_, _ = e.store.SaveInstance(inst)
}

func (e *Executor) audit(instanceID, nodeID, action string, details map[string]interface{}) {
	inst, ok := e.store.GetInstance(instanceID)
	if !ok {
		return
	}
	inst.AuditLog = append(inst.AuditLog, models.AuditEntry{
		Timestamp: time.Now(),
		NodeID:    nodeID,
		Action:    action,
		Details:   details,
	})
	_, _ = e.store.SaveInstance(inst)
}

func (e *Executor) markFailed(instanceID, reason string) {
	inst, ok := e.store.GetInstance(instanceID)
	if !ok {
		return
	}
	now := time.Now()
	inst.Status = models.InstanceFailed
	inst.CompletedAt = &now
	inst.AuditLog = append(inst.AuditLog, models.AuditEntry{
		Timestamp: now,
		Action:    "instance_failed",
		Details:   map[string]interface{}{"reason": reason},
	})
	_, _ = e.store.SaveInstance(inst)
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
			lf := toFloat(v)
			rf := toFloat(right)
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

	// invalid expression: default yes for backward compatibility
	return "yes"
}

func toFloat(v interface{}) float64 {
	switch x := v.(type) {
	case int:
		return float64(x)
	case int32:
		return float64(x)
	case int64:
		return float64(x)
	case float32:
		return float64(x)
	case float64:
		return x
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return 0
		}
		return f
	default:
		s, err := strconv.ParseFloat(fmt.Sprintf("%v", v), 64)
		if err != nil {
			return 0
		}
		return s
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
