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

// Executor handles running workflow instances by walking the node graph.
// Routing is embedded in each node (next, next_yes, next_no, next_branches,
// required_inputs) â€” there is no separate edges array.
type Executor struct {
	store storage.Store
	email connectors.EmailConnector

	// mergeWaiters counts how many required-input branches have arrived at
	// each merge node per instance.  Key = "instanceID:nodeID".
	mu           sync.Mutex
	mergeWaiters map[string]int
}

func NewExecutor(s storage.Store, e connectors.EmailConnector) *Executor {
	return &Executor{
		store:        s,
		email:        e,
		mergeWaiters: make(map[string]int),
	}
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// StartInstance
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

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
		log.Printf("executor: no start node in workflow=%s", wf.ID)
		e.markFailed(instanceID, "no start node")
		return
	}

	e.walkNode(instanceID, start.ID, &wf, data)

	if inst, ok := e.store.GetInstance(instanceID); ok {
		now := time.Now()
		inst.Status = models.InstanceCompleted
		inst.CompletedAt = &now
		inst.AuditLog = append(inst.AuditLog, models.AuditEntry{
			Timestamp: now,
			Action:    "instance_completed",
		})
		e.store.SaveInstance(inst)
	}
	log.Printf("executor: instance=%s completed", instanceID)
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// walkNode â€” execute one node then follow its successors
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (e *Executor) walkNode(instanceID, nodeID string, wf *models.Workflow, data map[string]interface{}) {
	node := wf.FindNode(nodeID)
	if node == nil {
		log.Printf("executor: unknown node %s", nodeID)
		return
	}

	e.setNodeState(instanceID, nodeID, "running")
	log.Printf("executor: instance=%s node=%s type=%s", instanceID, nodeID, node.Type)

	switch node.Type {

	case models.NodeStart:
		// pass-through; just follow next
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, "", wf, data)

	case models.NodeEnd:
		e.setNodeState(instanceID, nodeID, "completed")
		return // terminal

	case models.NodeTask:
		action := e.executeTask(instanceID, wf, node, data)
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, action, wf, data)

	case models.NodeAction:
		e.executeAction(instanceID, node, data)
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, "", wf, data)

	case models.NodeCondition:
		branch := e.evalCondition(node.Condition, data) // "yes" or "no"
		e.audit(instanceID, nodeID, "condition_evaluated", map[string]interface{}{
			"expression": node.Condition,
			"branch":     branch,
		})
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, branch, wf, data)

	case models.NodeParallel:
		e.executeParallel(instanceID, node, wf, data)
		// parallel fans out; walkNext is called inside executeParallel

	case models.NodeMerge:
		// Merge sync: only the last required branch proceeds.
		needed := len(node.RequiredInputs)
		if needed == 0 {
			needed = 1 // safety: treat as single-input if not configured
		}

		key := instanceID + ":" + node.ID
		e.mu.Lock()
		e.mergeWaiters[key]++
		arrived := e.mergeWaiters[key]
		e.mu.Unlock()

		log.Printf("executor: merge %s arrived=%d needed=%d", node.ID, arrived, needed)

		if arrived < needed {
			return // not all required branches here yet
		}

		e.mu.Lock()
		delete(e.mergeWaiters, key)
		e.mu.Unlock()

		e.audit(instanceID, nodeID, "merge_completed", map[string]interface{}{
			"required_inputs": node.RequiredInputs,
		})
		e.setNodeState(instanceID, nodeID, "completed")
		e.walkNext(instanceID, node, "", wf, data)
	}
}

// walkNext follows all successors of a node based on embedded routing.
// result is only used for condition nodes ("yes"/"no"); pass "" otherwise.
func (e *Executor) walkNext(instanceID string, node *models.WorkflowNode, result string, wf *models.Workflow, data map[string]interface{}) {
	for _, nextID := range node.NextIDs(result) {
		e.walkNode(instanceID, nextID, wf, data)
	}
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// Node executors
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

// executeTask creates a TaskAssignment for a human to act on.
// Returns the chosen action string (used for action-based branching).
// In production this would pause and wait for the assignee to complete
// the task via the API; for now it simulates by picking the first allowed action.
func (e *Executor) executeTask(instanceID string, wf *models.Workflow, node *models.WorkflowNode, data map[string]interface{}) string {
	task := models.TaskAssignment{
		InstanceID:       instanceID,
		OrgID:            wf.OrgID,
		WorkflowID:       wf.ID,
		NodeID:           node.ID,
		Title:            node.Title,
		Description:      node.Description,
		AssignedRole:     node.AssignedRole,
		AssignedPosition: node.AssignedPosition,
		AssignedUser:     node.AssignedUser,
		AllowedActions:   node.TaskActions,
		FormTemplateID:   node.FormTemplateID,
		SLADays:          node.SLADays,
		Status:           models.TaskPending,
		Data:             data,
		CreatedAt:        time.Now(),
	}
	taskID, _ := e.store.SaveTask(task)
	e.audit(instanceID, node.ID, "task_assigned", map[string]interface{}{
		"task_id":           taskID,
		"assigned_role":     node.AssignedRole,
		"assigned_position": node.AssignedPosition,
		"assigned_user":     node.AssignedUser,
		"sla_days":          node.SLADays,
		"allowed_actions":   node.TaskActions,
		"form_template_id":  node.FormTemplateID,
	})
	log.Printf("executor: task created id=%s role=%s position=%s",
		taskID, node.AssignedRole, node.AssignedPosition)

	// Real system would pause here and wait for a human action via the API.
	// For now, simulate with a short delay so the flow doesn't block tests.
	time.Sleep(200 * time.Millisecond)

	// Pick the first allowed action for simulation purposes; in a real
	// system the action would come from the API when the assignee acts.
	chosenAction := ""
	if len(node.TaskActions) > 0 {
		chosenAction = node.TaskActions[0]
	}
	log.Printf("executor: task %s simulated action=%q", taskID, chosenAction)
	return chosenAction
}

// executeAction calls the configured connector.
func (e *Executor) executeAction(instanceID string, node *models.WorkflowNode, data map[string]interface{}) {
	if node.Connector == nil {
		log.Printf("executor: action node %s has no connector configured", node.ID)
		e.audit(instanceID, node.ID, "action_skipped", map[string]interface{}{"reason": "no connector"})
		return
	}

	params := node.Connector.Params
	connType := node.Connector.Type

	switch connType {
	case models.ConnectorEmail:
		to := resolveParam(params, "to", data)
		subject := resolveParam(params, "subject", data)
		body := resolveParam(params, "body_template", data)
		if to != "" {
			_ = e.email.Send(to, subject, body)
			e.audit(instanceID, node.ID, "email_sent", map[string]interface{}{
				"to": to, "subject": subject,
			})
			log.Printf("executor: [email] â†’ %s | %s", to, subject)
		}

	case models.ConnectorWebhook:
		url := resolveParam(params, "url", data)
		method := resolveParam(params, "method", data)
		if method == "" {
			method = "POST"
		}
		log.Printf("executor: [webhook] %s %s", method, url)
		e.audit(instanceID, node.ID, "webhook_called", map[string]interface{}{
			"url": url, "method": method,
		})

	case models.ConnectorPayment:
		provider := resolveParam(params, "provider", data)
		amountField := resolveParam(params, "amount_field", data)
		currency := resolveParam(params, "currency", data)
		log.Printf("executor: [payment] provider=%s amount_field=%s currency=%s", provider, amountField, currency)
		e.audit(instanceID, node.ID, "payment_triggered", map[string]interface{}{
			"provider": provider, "currency": currency,
		})

	case models.ConnectorFormSubmit:
		formID := resolveParam(params, "form_id", data)
		formURL := resolveParam(params, "form_url", data)
		log.Printf("executor: [form] form_id=%s url=%s", formID, formURL)
		e.audit(instanceID, node.ID, "form_triggered", map[string]interface{}{
			"form_id": formID, "form_url": formURL,
		})

	case models.ConnectorNotification:
		channel := resolveParam(params, "channel", data)
		recipient := resolveParam(params, "recipient", data)
		message := resolveParam(params, "message_template", data)
		log.Printf("executor: [notification] channel=%s recipient=%s msg=%s", channel, recipient, message)
		e.audit(instanceID, node.ID, "notification_sent", map[string]interface{}{
			"channel": channel, "recipient": recipient,
		})

	default:
		log.Printf("executor: unknown connector type=%s on node=%s", connType, node.ID)
		e.audit(instanceID, node.ID, "action_skipped", map[string]interface{}{
			"reason": "unknown connector type", "type": connType,
		})
	}
}

// executeParallel fans out to all NextBranches concurrently.
func (e *Executor) executeParallel(instanceID string, node *models.WorkflowNode, wf *models.Workflow, data map[string]interface{}) {
	branches := node.NextBranches
	e.audit(instanceID, node.ID, "parallel_started", map[string]interface{}{
		"branches": len(branches),
	})
	e.setNodeState(instanceID, node.ID, "completed")

	var wg sync.WaitGroup
	for _, nextID := range branches {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			e.walkNode(instanceID, id, wf, data)
		}(nextID)
	}
	wg.Wait()
}

// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€
// Helpers
// â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€â”€

func (e *Executor) setNodeState(instanceID, nodeID, status string) {
	inst, ok := e.store.GetInstance(instanceID)
	if !ok {
		return
	}
	if inst.NodeStates == nil {
		inst.NodeStates = make(map[string]models.NodeState)
	}
	now := time.Now()
	ns := inst.NodeStates[nodeID]
	ns.Status = status
	if status == "running" {
		ns.StartedAt = &now
	}
	if status == "completed" || status == "failed" || status == "skipped" {
		ns.CompletedAt = &now
	}
	inst.NodeStates[nodeID] = ns
	inst.CurrentNode = nodeID
	e.store.SaveInstance(inst)
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
	e.store.SaveInstance(inst)
}

func (e *Executor) markFailed(instanceID, reason string) {
	inst, ok := e.store.GetInstance(instanceID)
	if !ok {
		return
	}
	inst.Status = models.InstanceFailed
	inst.AuditLog = append(inst.AuditLog, models.AuditEntry{
		Timestamp: time.Now(),
		Action:    "instance_failed",
		Details:   map[string]interface{}{"reason": reason},
	})
	e.store.SaveInstance(inst)
}

// resolveParam gets a connector param value and interpolates {{data.field}} references.
func resolveParam(params map[string]interface{}, key string, data map[string]interface{}) string {
	if params == nil {
		return ""
	}
	raw, ok := params[key]
	if !ok {
		return ""
	}
	s := fmt.Sprintf("%v", raw)
	// Interpolate {{data.fieldname}}
	for k, v := range data {
		placeholder := "{{data." + k + "}}"
		s = strings.ReplaceAll(s, placeholder, fmt.Sprintf("%v", v))
	}
	return s
}

// evalCondition evaluates a simple expression against instance data.
// Returns "yes" if true, "no" if false.
func (e *Executor) evalCondition(cond string, data map[string]interface{}) string {
	cond = strings.TrimSpace(cond)
	if cond == "" {
		return "yes"
	}
	for _, op := range []string{">=", "<=", "!=", "==", ">", "<"} {
		parts := strings.SplitN(cond, op, 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			expected := strings.TrimSpace(parts[1])
			actual := ""
			if v, ok := data[key]; ok {
				actual = strings.TrimSpace(fmt.Sprintf("%v", v))
			}
			match := false
			switch op {
			case "==":
				match = actual == expected
			case "!=":
				match = actual != expected
			case ">", ">=", "<", "<=":
				af := toFloat(actual)
				ef := toFloat(expected)
				switch op {
				case ">":
					match = af > ef
				case ">=":
					match = af >= ef
				case "<":
					match = af < ef
				case "<=":
					match = af <= ef
				}
			}
			if match {
				return "yes"
			}
			return "no"
		}
	}
	log.Printf("executor: could not evaluate condition: %s", cond)
	return "yes"
}

func toFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}
