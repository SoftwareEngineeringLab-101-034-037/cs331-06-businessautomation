package models

import "time"

// ─────────────────────────────────────────────────────────────
// Node Types
// ─────────────────────────────────────────────────────────────

type NodeType string

const (
	NodeStart     NodeType = "start"
	NodeTask      NodeType = "task"
	NodeAction    NodeType = "action"
	NodeCondition NodeType = "condition"
	NodeParallel  NodeType = "parallel"
	NodeMerge     NodeType = "merge"
	NodeEnd       NodeType = "end"
)

// ─────────────────────────────────────────────────────────────
// Connector Types — which external service an action node calls
// ─────────────────────────────────────────────────────────────

type ConnectorType string

const (
	ConnectorEmail        ConnectorType = "email"
	ConnectorWebhook      ConnectorType = "webhook"
	ConnectorFormSubmit   ConnectorType = "form_submit"
	ConnectorPayment      ConnectorType = "payment"
	ConnectorNotification ConnectorType = "notification"
)

// ConnectorConfig holds the connector type and all its parameters.
// Param values may use {{data.fieldname}} template syntax to reference
// instance data collected during execution.
//
// Email params:    to, cc, subject, body_template, from_name
// Webhook params:  url, method, headers (JSON string), body_template, timeout_seconds
// Form params:     form_id, form_url, collect_fields (comma-separated)
// Payment params:  provider, amount_field, currency, description_template
// Notification:    channel (slack|teams|sms), recipient, message_template
type ConnectorConfig struct {
	Type   ConnectorType          `json:"type" bson:"type"`
	Params map[string]interface{} `json:"params" bson:"params"`
}

type ConditionDataType string

const (
	ConditionDataTypeText     ConditionDataType = "text"
	ConditionDataTypeNumber   ConditionDataType = "number"
	ConditionDataTypeBoolean  ConditionDataType = "boolean"
	ConditionDataTypeDate     ConditionDataType = "date"
	ConditionDataTypeDateTime ConditionDataType = "datetime"
	ConditionDataTypeTime     ConditionDataType = "time"
)

type ConditionOperator string

const (
	ConditionOperatorEquals      ConditionOperator = "eq"
	ConditionOperatorNotEquals   ConditionOperator = "neq"
	ConditionOperatorGreaterThan ConditionOperator = "gt"
	ConditionOperatorGreaterEq   ConditionOperator = "gte"
	ConditionOperatorLessThan    ConditionOperator = "lt"
	ConditionOperatorLessEq      ConditionOperator = "lte"
	ConditionOperatorContains    ConditionOperator = "contains"
	ConditionOperatorNotContains ConditionOperator = "not_contains"
	ConditionOperatorStartsWith  ConditionOperator = "starts_with"
	ConditionOperatorEndsWith    ConditionOperator = "ends_with"
	ConditionOperatorIsEmpty     ConditionOperator = "is_empty"
	ConditionOperatorIsNotEmpty  ConditionOperator = "is_not_empty"
)

type ConditionJoin string

const (
	ConditionJoinAnd ConditionJoin = "and"
	ConditionJoinOr  ConditionJoin = "or"
)

type ConditionRule struct {
	Field    string            `json:"field" bson:"field"`
	DataType ConditionDataType `json:"data_type,omitempty" bson:"data_type,omitempty"`
	Operator ConditionOperator `json:"operator" bson:"operator"`
	Value    interface{}       `json:"value,omitempty" bson:"value,omitempty"`
}

type ConditionConfig struct {
	Join  ConditionJoin   `json:"join,omitempty" bson:"join,omitempty"`
	Logic string          `json:"logic,omitempty" bson:"logic,omitempty"`
	Rules []ConditionRule `json:"rules,omitempty" bson:"rules,omitempty"`
}

// ─────────────────────────────────────────────────────────────
// Trigger Types
// ─────────────────────────────────────────────────────────────

type TriggerType string

const (
	TriggerManual     TriggerType = "manual"
	TriggerScheduled  TriggerType = "scheduled"
	TriggerWebhook    TriggerType = "webhook"
	TriggerFormSubmit TriggerType = "form_submit"
	TriggerEmail      TriggerType = "email_received"
)

// Trigger defines what kicks off a workflow instance.
type Trigger struct {
	Type   TriggerType       `json:"type" bson:"type"`
	Config map[string]string `json:"config,omitempty" bson:"config,omitempty"`
}

// ─────────────────────────────────────────────────────────────
// Workflow Status
// ─────────────────────────────────────────────────────────────

type WorkflowStatus string

const (
	WorkflowDraft    WorkflowStatus = "draft"
	WorkflowActive   WorkflowStatus = "active"
	WorkflowInactive WorkflowStatus = "inactive"
	WorkflowArchived WorkflowStatus = "archived"
)

// ─────────────────────────────────────────────────────────────
// Workflow Definition — stored when the user publishes
// Routing is embedded inside each node; there is no separate
// edges array.  The full original JSON the frontend sent is kept
// in RawJSON for audit / re-import purposes.
// ─────────────────────────────────────────────────────────────

type Workflow struct {
	ID          string         `json:"id" bson:"id"`
	OrgID       string         `json:"org_id" bson:"org_id"`
	Version     int            `json:"version" bson:"version"`
	Name        string         `json:"name" bson:"name"`
	Description string         `json:"description,omitempty" bson:"description,omitempty"`
	Department  string         `json:"department,omitempty" bson:"department,omitempty"`
	Status      WorkflowStatus `json:"status" bson:"status"`
	Trigger     Trigger        `json:"trigger" bson:"trigger"`
	Nodes       []WorkflowNode `json:"nodes" bson:"nodes"`
	Tags        []string       `json:"tags,omitempty" bson:"tags,omitempty"`
	RawJSON     string         `json:"raw_json,omitempty" bson:"raw_json,omitempty"`
	CreatedBy   string         `json:"created_by,omitempty" bson:"created_by,omitempty"`
	CreatedAt   time.Time      `json:"created_at" bson:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at" bson:"updated_at"`
}

// ─────────────────────────────────────────────────────────────
// WorkflowNode — single step; routing is embedded, not in edges
//
// Routing fields by node type:
//   start      → Next
//   task       → Next  (resumes after human completes the task)
//   action     → Next
//   condition  → Condition + NextYes + NextNo
//   parallel   → NextBranches  (fan-out; all branches run concurrently)
//   merge      → Next + RequiredInputs + OptionalInputs
//   end        → (none)
//
// Connector field: only used on action nodes.
// Task fields:     only used on task nodes.
// ─────────────────────────────────────────────────────────────

type WorkflowNode struct {
	ID          string   `json:"id" bson:"id"`
	Type        NodeType `json:"type" bson:"type"`
	Title       string   `json:"title" bson:"title"`
	Description string   `json:"description,omitempty" bson:"description,omitempty"`

	// ── Routing ────────────────────────────────────────────────────────────

	// Next is the single successor for start / task / action / merge / end.
	Next string `json:"next,omitempty" bson:"next,omitempty"`

	// Condition nodes: expression to evaluate + yes/no successors.
	Condition       string           `json:"condition,omitempty" bson:"condition,omitempty"`
	ConditionConfig *ConditionConfig `json:"condition_config,omitempty" bson:"condition_config,omitempty"`
	NextYes         string           `json:"next_yes,omitempty" bson:"next_yes,omitempty"`
	NextNo          string           `json:"next_no,omitempty" bson:"next_no,omitempty"`

	// Parallel nodes: list of node IDs to fan out to simultaneously.
	NextBranches []string `json:"next_branches,omitempty" bson:"next_branches,omitempty"`

	// Merge nodes: node IDs whose branches MUST arrive before continuing.
	// OptionalInputs arrive early but don't block the merge.
	RequiredInputs []string `json:"required_inputs,omitempty" bson:"required_inputs,omitempty"`
	OptionalInputs []string `json:"optional_inputs,omitempty" bson:"optional_inputs,omitempty"`

	// ── Task Assignment (type == task) ─────────────────────────────────────

	// AssignedRole is the role group that receives this task (e.g. "manager").
	AssignedRole string `json:"assigned_role,omitempty" bson:"assigned_role,omitempty"`
	// AssignedRoles stores all role targets for this task when multiple role tags are configured.
	AssignedRoles []string `json:"assigned_roles,omitempty" bson:"assigned_roles,omitempty"`
	// AssignedUsers stores explicit user targets for this task when @user tags are configured.
	AssignedUsers []string `json:"assigned_users,omitempty" bson:"assigned_users,omitempty"`
	// AssignedTargets stores the original assignment tokens (#role/@user) configured in builder UI.
	AssignedTargets []string `json:"assigned_targets,omitempty" bson:"assigned_targets,omitempty"`
	// AssignedPosition narrows within the role (e.g. "CFO", "Department Head").
	AssignedPosition string `json:"assigned_position,omitempty" bson:"assigned_position,omitempty"`
	// AssignedUser is a direct user fallback used only when AssignedRole is empty.
	AssignedUser string `json:"assigned_user,omitempty" bson:"assigned_user,omitempty"`
	// TaskActions lists what the assignee can do: approve, reject, clarify, complete.
	TaskActions []string `json:"task_actions,omitempty" bson:"task_actions,omitempty"`
	// NextActions maps each task action (e.g. "approve", "reject") to the next
	// node ID that should be executed when the assignee picks that action.
	// Only used on task nodes with multiple possible outcomes.  When empty the
	// executor falls back to the single Next field.
	NextActions map[string]string `json:"next_actions,omitempty" bson:"next_actions,omitempty"`
	// FormTemplateID is an optional form the assignee must fill in.
	FormTemplateID string `json:"form_template_id,omitempty" bson:"form_template_id,omitempty"`
	// SLADays is the deadline in business days; 0 means no SLA.
	SLADays int `json:"sla_days,omitempty" bson:"sla_days,omitempty"`
	// TaskDataVisibility controls what instance data is shown to assignees.
	// Allowed values: all | selected | none
	TaskDataVisibility string `json:"task_data_visibility,omitempty" bson:"task_data_visibility,omitempty"`
	// VisibleDataKeys is used when TaskDataVisibility == selected.
	VisibleDataKeys []string `json:"visible_data_keys,omitempty" bson:"visible_data_keys,omitempty"`
	// IncludeFormSubmission exposes the full normalized form payload.
	IncludeFormSubmission bool `json:"include_form_submission,omitempty" bson:"include_form_submission,omitempty"`
	// IncludeFormFiles exposes extracted file URLs from form_submission.
	IncludeFormFiles bool `json:"include_form_files,omitempty" bson:"include_form_files,omitempty"`

	// ── Connector (type == action) ─────────────────────────────────────────

	Connector *ConnectorConfig `json:"connector,omitempty" bson:"connector,omitempty"`

	// ── Canvas metadata (ignored by executor) ──────────────────────────────

	Position *Position `json:"position,omitempty" bson:"position,omitempty"`
}

// Position stores React Flow canvas x/y coordinates.
type Position struct {
	X float64 `json:"x" bson:"x"`
	Y float64 `json:"y" bson:"y"`
}

// ─────────────────────────────────────────────────────────────
// Workflow Instance — a running or completed execution
// ─────────────────────────────────────────────────────────────

type InstanceStatus string

const (
	InstancePending   InstanceStatus = "pending"
	InstanceRunning   InstanceStatus = "running"
	InstanceWaiting   InstanceStatus = "waiting"
	InstanceCompleted InstanceStatus = "completed"
	InstanceFailed    InstanceStatus = "failed"
	InstanceCancelled InstanceStatus = "cancelled"
)

type Instance struct {
	ID          string                 `json:"id" bson:"id"`
	OrgID       string                 `json:"org_id" bson:"org_id"`
	WorkflowID  string                 `json:"workflow_id" bson:"workflow_id"`
	Status      InstanceStatus         `json:"status" bson:"status"`
	CurrentNode string                 `json:"current_node,omitempty" bson:"current_node,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty" bson:"data,omitempty"`
	NodeStates  map[string]NodeState   `json:"node_states,omitempty" bson:"node_states,omitempty"`
	AuditLog    []AuditEntry           `json:"audit_log,omitempty" bson:"audit_log,omitempty"`
	StartedAt   time.Time              `json:"started_at" bson:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}

// NodeState tracks per-node execution status within an instance.
type NodeState struct {
	Status      string     `json:"status" bson:"status"`
	StartedAt   *time.Time `json:"started_at,omitempty" bson:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
	Output      string     `json:"output,omitempty" bson:"output,omitempty"`
}

// ─────────────────────────────────────────────────────────────
// Task Assignment — human tasks waiting for action
// ─────────────────────────────────────────────────────────────

type TaskStatus string

const (
	TaskPending    TaskStatus = "pending"
	TaskInProgress TaskStatus = "in_progress"
	TaskApproved   TaskStatus = "approved"
	TaskRejected   TaskStatus = "rejected"
	TaskClarify    TaskStatus = "clarification_requested"
	TaskCompleted  TaskStatus = "completed"
)

type TaskAssignment struct {
	ID               string                 `json:"id" bson:"id"`
	OrgID            string                 `json:"org_id" bson:"org_id"`
	InstanceID       string                 `json:"instance_id" bson:"instance_id"`
	WorkflowID       string                 `json:"workflow_id" bson:"workflow_id"`
	NodeID           string                 `json:"node_id" bson:"node_id"`
	Title            string                 `json:"title" bson:"title"`
	Description      string                 `json:"description,omitempty" bson:"description,omitempty"`
	AssignedRole     string                 `json:"assigned_role,omitempty" bson:"assigned_role,omitempty"`
	AssignedRoles    []string               `json:"assigned_roles,omitempty" bson:"assigned_roles,omitempty"`
	AssignedUsers    []string               `json:"assigned_users,omitempty" bson:"assigned_users,omitempty"`
	AssignedTargets  []string               `json:"assigned_targets,omitempty" bson:"assigned_targets,omitempty"`
	AssignedPosition string                 `json:"assigned_position,omitempty" bson:"assigned_position,omitempty"`
	AssignedUser     string                 `json:"assigned_user,omitempty" bson:"assigned_user,omitempty"`
	AllowedActions   []string               `json:"allowed_actions,omitempty" bson:"allowed_actions,omitempty"`
	FormTemplateID   string                 `json:"form_template_id,omitempty" bson:"form_template_id,omitempty"`
	SLADays          int                    `json:"sla_days,omitempty" bson:"sla_days,omitempty"`
	Status           TaskStatus             `json:"status" bson:"status"`
	ActionCommitted  string                 `json:"action_committed,omitempty" bson:"action_committed,omitempty"`
	Data             map[string]interface{} `json:"-" bson:"data,omitempty"`
	VisibleData      map[string]interface{} `json:"visible_data,omitempty" bson:"visible_data,omitempty"`
	Comment          string                 `json:"comment,omitempty" bson:"comment,omitempty"`
	CreatedAt        time.Time              `json:"created_at" bson:"created_at"`
	CompletedAt      *time.Time             `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}

// ─────────────────────────────────────────────────────────────
// Audit Log
// ─────────────────────────────────────────────────────────────

type AuditEntry struct {
	Timestamp time.Time              `json:"timestamp" bson:"timestamp"`
	NodeID    string                 `json:"node_id,omitempty" bson:"node_id,omitempty"`
	Action    string                 `json:"action" bson:"action"`
	Actor     string                 `json:"actor,omitempty" bson:"actor,omitempty"`
	Details   map[string]interface{} `json:"details,omitempty" bson:"details,omitempty"`
}

// ─────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────

// FindNode returns the node with the given ID, or nil.
func (w *Workflow) FindNode(id string) *WorkflowNode {
	for i := range w.Nodes {
		if w.Nodes[i].ID == id {
			return &w.Nodes[i]
		}
	}
	return nil
}

// StartNode returns the first start-typed node, or nil.
func (w *Workflow) StartNode() *WorkflowNode {
	for i := range w.Nodes {
		if w.Nodes[i].Type == NodeStart {
			return &w.Nodes[i]
		}
	}
	return nil
}

// NextIDs returns the list of node IDs that follow the given node.
// For condition nodes pass the branch result ("yes" or "no").
// For task nodes pass the action taken (e.g. "approve", "reject"); if the
// node has a NextActions map the result selects the branch, otherwise it
// falls back to the single Next field.
// For all other nodes the result parameter is ignored.
func (n *WorkflowNode) NextIDs(result string) []string {
	switch n.Type {
	case NodeCondition:
		if result == "yes" && n.NextYes != "" {
			return []string{n.NextYes}
		}
		if result == "no" && n.NextNo != "" {
			return []string{n.NextNo}
		}
		return nil
	case NodeTask:
		// Action-based branching: if NextActions is populated and the result
		// matches an entry, route to that specific branch.
		if len(n.NextActions) > 0 && result != "" {
			if target, ok := n.NextActions[result]; ok && target != "" {
				return []string{target}
			}
		}
		// Fallback to single successor
		if n.Next != "" {
			return []string{n.Next}
		}
		return nil
	case NodeParallel:
		return n.NextBranches
	case NodeEnd:
		return nil
	default:
		if n.Next != "" {
			return []string{n.Next}
		}
		return nil
	}
}
