// ─── Role System ───────────────────────────────────────────────
export type UserRole = "admin" | "employee";

export const ROLE_LABELS: Record<UserRole, string> = {
  admin: "Admin",
  employee: "Employee",
};

// ─── Task Types ────────────────────────────────────────────────
export type TaskStatus =
  | "pending"
  | "in_progress"
  | "completed"
  | "overdue"
  | "escalated"
  | "cancelled"
  | "sent_back";

export type TaskPriority = "low" | "medium" | "high" | "critical";

export interface Task {
  id: string;
  title: string;
  description: string;
  status: TaskStatus;
  priority: TaskPriority;
  assignedTo: string;
  assignedToName: string;
  assignedBy: string;
  assignedByName: string;
  workflowId: string;
  workflowName: string;
  departmentOrigin: string;
  createdAt: string;
  dueDate: string;
  completedAt?: string;
  escalatedAt?: string;
  sentBackAt?: string;
  sentBackReason?: string;
  tags: string[];
  stepNumber: number;
  totalSteps: number;
}

export const TASK_STATUS_CONFIG: Record<
  TaskStatus,
  { label: string; color: string; bg: string }
> = {
  pending: { label: "Pending", color: "#f59e0b", bg: "rgba(245,158,11,0.12)" },
  in_progress: { label: "In Progress", color: "#3b82f6", bg: "rgba(59,130,246,0.12)" },
  completed: { label: "Completed", color: "#22c55e", bg: "rgba(34,197,94,0.12)" },
  overdue: { label: "Overdue", color: "#ef4444", bg: "rgba(239,68,68,0.12)" },
  escalated: { label: "Escalated", color: "#f97316", bg: "rgba(249,115,22,0.12)" },
  cancelled: { label: "Cancelled", color: "#6b7280", bg: "rgba(107,114,128,0.12)" },
  sent_back: { label: "Sent Back", color: "#8b5cf6", bg: "rgba(139,92,246,0.12)" },
};

export const PRIORITY_CONFIG: Record<
  TaskPriority,
  { label: string; color: string; bg: string }
> = {
  low: { label: "Low", color: "#22c55e", bg: "rgba(34,197,94,0.12)" },
  medium: { label: "Medium", color: "#f59e0b", bg: "rgba(245,158,11,0.12)" },
  high: { label: "High", color: "#f97316", bg: "rgba(249,115,22,0.12)" },
  critical: { label: "Critical", color: "#ef4444", bg: "rgba(239,68,68,0.12)" },
};

// ─── Workflow Request Types ────────────────────────────────────
export type RequestStatus =
  | "submitted"
  | "in_progress"
  | "approved"
  | "rejected"
  | "cancelled"
  | "completed";

export interface WorkflowRequest {
  id: string;
  title: string;
  workflowName: string;
  status: RequestStatus;
  submittedBy: string;
  submittedByName: string;
  submittedAt: string;
  currentStep: string;
  totalSteps: number;
  completedSteps: number;
  lastUpdated: string;
  approver?: string;
}

export const REQUEST_STATUS_CONFIG: Record<
  RequestStatus,
  { label: string; color: string; bg: string }
> = {
  submitted: { label: "Submitted", color: "#3b82f6", bg: "rgba(59,130,246,0.12)" },
  in_progress: { label: "In Progress", color: "#f59e0b", bg: "rgba(245,158,11,0.12)" },
  approved: { label: "Approved", color: "#22c55e", bg: "rgba(34,197,94,0.12)" },
  rejected: { label: "Rejected", color: "#ef4444", bg: "rgba(239,68,68,0.12)" },
  cancelled: { label: "Cancelled", color: "#6b7280", bg: "rgba(107,114,128,0.12)" },
  completed: { label: "Completed", color: "#06b6d4", bg: "rgba(6,182,212,0.12)" },
};

// ─── Activity Feed ─────────────────────────────────────────────
export type ActivityType =
  | "task_assigned"
  | "task_completed"
  | "task_escalated"
  | "request_submitted"
  | "request_approved"
  | "request_rejected"
  | "member_joined"
  | "workflow_published";

export interface ActivityItem {
  id: string;
  type: ActivityType;
  message: string;
  actor: string;
  timestamp: string;
  relatedId?: string;
}

// ─── Workstation Types ─────────────────────────────────────────
export interface Department {
  id: string;
  name: string;
  head: string;
  memberCount: number;
  activeWorkflows: number;
}

export interface TeamMember {
  id: string;
  name: string;
  email: string;
  role: UserRole;
  department: string;
  avatarUrl?: string;
  isActive: boolean;
  tasksCompleted: number;
  tasksPending: number;
  lastActive: string;
}

// ─── Analytics Types ───────────────────────────────────────────
export interface WorkflowMetric {
  workflowName: string;
  totalRuns: number;
  avgCompletionTime: string;
  successRate: number;
  bottleneckStep?: string;
}

// ─── Sidebar Navigation ───────────────────────────────────────
export interface NavItem {
  label: string;
  href: string;
  icon: string; // icon identifier
  roles: UserRole[]; // which roles can see this
  badge?: number;
}

export const DASHBOARD_NAV: NavItem[] = [
  {
    label: "Overview",
    href: "/dashboard",
    icon: "home",
    roles: ["admin", "employee"],
  },
  {
    label: "My Tasks",
    href: "/dashboard/tasks",
    icon: "tasks",
    roles: ["admin", "employee"],
  },
  {
    label: "Requests",
    href: "/dashboard/requests",
    icon: "requests",
    roles: ["admin", "employee"],
  },
  {
    label: "Workstation",
    href: "/dashboard/workstation",
    icon: "workstation",
    roles: ["admin"],
  },
  {
    label: "Analytics",
    href: "/dashboard/analytics",
    icon: "analytics",
    roles: ["admin"],
  },
  {
    label: "Team",
    href: "/dashboard/team",
    icon: "team",
    roles: ["admin"],
  },
];
