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
export interface WorkflowStep {
  id: string;
  title: string;
  description: string;
  actionType: StepActionType;
  assignedRole: string; // custom org role
  slaDays: number;      // SLA in working days
  isRequired: boolean;
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
  tags: string[];
}

/** Generate a unique step ID */
export function generateStepId(): string {
  return `step_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`;
}

/** Create a blank step with defaults */
export function createBlankStep(order: number): WorkflowStep {
  return {
    id: generateStepId(),
    title: `Step ${order}`,
    description: "",
    actionType: "custom_task",
    assignedRole: "",
    slaDays: 1,
    isRequired: true,
  };
}
