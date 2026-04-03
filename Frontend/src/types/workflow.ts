// ─── Workflow Builder Types ────────────────────────────────────

/** Trigger types — what kicks off a workflow */
export type TriggerType =
  | "form_submission"
  | "email_received"
  | "scheduled"
  | "manual"
  | "webhook"
  | "condition";

export const TRIGGER_CONFIG: Record<
  TriggerType,
  { label: string; icon: string; description: string }
> = {
  form_submission: {
    label: "Form Submission",
    icon: "form",
    description: "Starts when someone submits a form",
  },
  email_received: {
    label: "Email Received",
    icon: "email",
    description: "Starts when an email arrives matching criteria",
  },
  scheduled: {
    label: "Scheduled",
    icon: "clock",
    description: "Runs on a recurring schedule",
  },
  manual: {
    label: "Manual Trigger",
    icon: "hand",
    description: "Started manually by an admin or user",
  },
  webhook: {
    label: "Webhook",
    icon: "webhook",
    description: "Triggered via an external API call",
  },
  condition: {
    label: "Condition Met",
    icon: "condition",
    description: "Fires when a data condition is met",
  },
};

/** Predefined custom org roles that admin can assign to steps */
export const PRESET_ORG_ROLES: string[] = [
  "CEO",
  "CTO",
  "CFO",
  "COO",
  "VP of Engineering",
  "VP of Sales",
  "VP of Marketing",
  "Director of HR",
  "Engineering Manager",
  "Senior Developer",
  "Junior Developer",
  "IT Administrator",
  "IT Support Specialist",
  "Department Head",
  "Team Lead",
  "Project Manager",
  "Product Manager",
  "QA Engineer",
  "DevOps Engineer",
  "Finance Manager",
  "Accountant",
  "HR Specialist",
  "Recruiter",
  "Sales Manager",
  "Sales Representative",
  "Marketing Specialist",
  "Customer Support Agent",
  "Legal Counsel",
  "Operations Manager",
  "Office Administrator",
  "Intern",
];

/** Predefined position titles for task assignment specificity */
export const PRESET_POSITIONS: string[] = [
  "Department Head",
  "Team Lead",
  "Senior Staff",
  "Junior Staff",
  "Direct Supervisor",
  "CFO",
  "CTO",
  "CEO",
  "COO",
  "VP",
  "Director",
  "Manager",
  "Coordinator",
  "Specialist",
  "Analyst",
  "Clerk",
];

/** Connector types — external services an action node can invoke */
export type ConnectorType =
  | "email"
  | "webhook"
  | "form_submit"
  | "payment"
  | "notification";

export const CONNECTOR_CONFIG: Record<
  ConnectorType,
  { label: string; icon: string; color: string; paramFields: ConnectorParamField[] }
> = {
  email: {
    label: "Gmail Send Email",
    icon: "email",
    color: "#3b82f6",
    paramFields: [
      { key: "to", label: "To", placeholder: "recipient@company.com or {{data.email}}", required: true },
      { key: "cc", label: "CC", placeholder: "cc@company.com (optional)" },
      { key: "subject", label: "Subject", placeholder: "Expense Request: {{data.title}}", required: true },
      { key: "body_template", label: "Body", placeholder: "Please review the expense of {{data.amount}}...", multiline: true, required: true },
      { key: "from_account_id", label: "Send From Account", placeholder: "primary or connected Gmail account email" },
      { key: "from_name", label: "From Name", placeholder: "Workflow System" },
    ],
  },
  webhook: {
    label: "Webhook / API Call",
    icon: "webhook",
    color: "#8b5cf6",
    paramFields: [
      { key: "url", label: "URL", placeholder: "https://hooks.example.com/endpoint", required: true },
      { key: "method", label: "Method", placeholder: "POST", options: ["GET", "POST", "PUT", "PATCH", "DELETE"] },
      { key: "headers", label: "Headers (JSON)", placeholder: '{"Authorization": "Bearer {{data.token}}"}', multiline: true },
      { key: "body_template", label: "Body Template", placeholder: '{"amount": {{data.amount}}}', multiline: true },
      { key: "timeout_seconds", label: "Timeout (seconds)", placeholder: "10" },
    ],
  },
  form_submit: {
    label: "Form / Survey",
    icon: "form",
    color: "#22c55e",
    paramFields: [
      { key: "form_id", label: "Form ID", placeholder: "google-form-id or internal form ID", required: true },
      { key: "form_url", label: "Form URL", placeholder: "https://forms.google.com/..." },
      { key: "collect_fields", label: "Fields to Collect", placeholder: "name, email, amount (comma-separated)" },
    ],
  },
  payment: {
    label: "Payment / Reimbursement",
    icon: "payment",
    color: "#f59e0b",
    paramFields: [
      { key: "provider", label: "Provider", placeholder: "stripe", options: ["stripe", "paypal", "bank_transfer", "manual"] },
      { key: "amount_field", label: "Amount Field", placeholder: "data.amount", required: true },
      { key: "currency", label: "Currency", placeholder: "USD", options: ["USD", "EUR", "GBP", "INR", "BDT"] },
      { key: "description_template", label: "Description", placeholder: "Reimbursement for {{data.submitted_by}}" },
    ],
  },
  notification: {
    label: "Notification",
    icon: "bell",
    color: "#06b6d4",
    paramFields: [
      { key: "channel", label: "Channel", placeholder: "slack", options: ["slack", "teams", "sms", "in_app"], required: true },
      { key: "recipient", label: "Recipient", placeholder: "#finance-approvals or {{data.manager_email}}", required: true },
      { key: "message_template", label: "Message", placeholder: "New expense from {{data.submitted_by}} for ${{data.amount}}", multiline: true, required: true },
    ],
  },
};

/** Shape of a param field declaration for a connector */
export interface ConnectorParamField {
  key: string;
  label: string;
  placeholder?: string;
  required?: boolean;
  multiline?: boolean;
  options?: string[]; // if provided, render as <select>
}

/** Connector config stored on an action node */
export interface ConnectorConfigData {
  type: ConnectorType;
  params: Record<string, string>;
}

/** Allowed task actions the assignee can take */
export type TaskAction = "approve" | "reject" | "clarify" | "complete";

export type TaskDataVisibilityMode = "all" | "selected" | "none";

export const TASK_ACTION_OPTIONS: { value: TaskAction; label: string; color: string }[] = [
  { value: "approve",  label: "Approve",  color: "#22c55e" },
  { value: "reject",   label: "Reject",   color: "#ef4444" },
  { value: "clarify",  label: "Clarify",  color: "#f59e0b" },
  { value: "complete", label: "Complete", color: "#3b82f6" },
];

/** Step action types — what kind of work this step performs */
export type StepActionType =
  | "review_approve"
  | "fill_form"
  | "upload_document"
  | "sign_document"
  | "send_notification"
  | "data_entry"
  | "custom_task";

export const STEP_ACTION_CONFIG: Record<
  StepActionType,
  { label: string; icon: string; color: string }
> = {
  review_approve: {
    label: "Review & Approve",
    icon: "check",
    color: "#22c55e",
  },
  fill_form: {
    label: "Fill Out Form",
    icon: "form",
    color: "#3b82f6",
  },
  upload_document: {
    label: "Upload Document",
    icon: "upload",
    color: "#8b5cf6",
  },
  sign_document: {
    label: "Sign Document",
    icon: "sign",
    color: "#06b6d4",
  },
  send_notification: {
    label: "Send Notification",
    icon: "bell",
    color: "#f59e0b",
  },
  data_entry: {
    label: "Data Entry",
    icon: "data",
    color: "#f97316",
  },
  custom_task: {
    label: "Custom Task",
    icon: "task",
    color: "#ec4899",
  },
};

/** A single step in the workflow */
export type NodeType =
  | "start"
  | "task"
  | "action"
  | "condition"
  | "parallel"
  | "merge"
  | "end";

export const NODE_TYPE_CONFIG: Record<
  NodeType,
  { label: string; color: string; shape: "circle" | "rect" | "diamond" | "bar" }
> = {
  start:     { label: "Start",     color: "#22c55e", shape: "circle"  },
  task:      { label: "Task",      color: "#3b82f6", shape: "rect"    },
  action:    { label: "Action",    color: "#8b5cf6", shape: "rect"    },
  condition: { label: "Condition", color: "#f59e0b", shape: "diamond" },
  parallel:  { label: "Parallel",  color: "#06b6d4", shape: "bar"     },
  merge:     { label: "Merge",     color: "#ec4899", shape: "bar"     },
  end:       { label: "End",       color: "#ef4444", shape: "circle"  },
};

export interface WorkflowStep {
  id: string;
  type?: NodeType;
  title: string;
  description: string;

  // ── Task assignment fields (type == "task") ───────────────
  assignedRole: string;
  assignedPosition?: string;
  assignedUser?: string;
  taskActions?: TaskAction[];
  /** Maps each task action to the downstream node ID (derived from canvas edges on publish). */
  nextActions?: Record<string, string>;
  formTemplateId?: string;
  taskDataVisibility?: TaskDataVisibilityMode;
  visibleDataKeys?: string[];
  includeFullFormResponse?: boolean;
  includeFormFiles?: boolean;
  slaDays: number;
  isRequired: boolean;
  actionType: StepActionType;

  // ── Connector (type == "action") ──────────────────────────
  connector?: ConnectorConfigData;

  // ── Condition (type == "condition") ────────────────────────
  condition?: string;

  // ── Parallel (type == "parallel") ─────────────────────────
  branches?: number;

  // ── Merge (type == "merge") ───────────────────────────────
  mergeInputs?: number;
  requiredInputs?: string[];

  // ── Canvas position ───────────────────────────────────────
  position?: { x: number; y: number };
}

/** Edge connecting two nodes on the canvas */
export interface WorkflowEdge {
  id: string;
  source: string;
  target: string;
  sourceHandle?: string; // e.g. "yes", "no", "branch-0"
  targetHandle?: string;
  label?: string;
}

/** Trigger configuration */
export interface WorkflowTrigger {
  type: TriggerType;
  config: Record<string, string>; // flexible config per trigger type
}

/** Full workflow definition being built */
export interface WorkflowDraft {
  name: string;
  description: string;
  department: string;
  trigger: WorkflowTrigger;
  steps: WorkflowStep[];
  edges: WorkflowEdge[];
  tags: string[];
}

let stepIdSequence = 0;

/** Generate a sequential step ID with a unique suffix to avoid collisions */
export function generateStepId(order: number): string {
  const safeOrder = Number.isFinite(order) && order > 0 ? order : 1;
  stepIdSequence += 1;
  return `s_${safeOrder.toString().padStart(3, "0")}_${stepIdSequence.toString().padStart(4, "0")}`;
}

/** Create a blank step with defaults */
export function createBlankStep(
  order: number,
  position?: { x: number; y: number },
): WorkflowStep {
  let rawOrder = Number(order);
  if (!Number.isFinite(rawOrder)) {
    rawOrder = 1;
  }
  const normalizedOrder = Math.max(1, Math.floor(rawOrder));
  return {
    id: generateStepId(normalizedOrder),
    type: "task",
    title: `Step ${normalizedOrder}`,
    description: "",
    actionType: "custom_task",
    assignedRole: "",
    taskDataVisibility: "all",
    visibleDataKeys: [],
    includeFullFormResponse: false,
    includeFormFiles: false,
    slaDays: 1,
    isRequired: true,
    branches: 2,
    position: position ?? { x: 250, y: 80 + normalizedOrder * 150 },
  };
}
