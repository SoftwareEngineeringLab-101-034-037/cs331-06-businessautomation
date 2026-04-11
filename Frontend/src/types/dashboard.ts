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
  | "rejected"
  | "overdue"
  | "escalated"
  | "cancelled"
  | "sent_back";

export type TaskPriority = "low" | "medium" | "high" | "critical";

export interface Task {
  id: string;
  title: string;
  description: string;
  comment?: string;
  actionCommitted?: string;
  status: TaskStatus;
  baseStatus?: TaskStatus;
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
  allowedActions?: string[];
  overridePendingActions?: boolean;
  visibleData?: Record<string, unknown>;
  nodeId?: string;
  orgId?: string;
  instanceId?: string;
  instanceStatus?: string;
  instanceError?: string;
}

export const TASK_STATUS_CONFIG: Record<
  TaskStatus,
  { label: string; color: string; bg: string }
> = {
  pending: { label: "Pending", color: "#f59e0b", bg: "rgba(245,158,11,0.12)" },
  in_progress: { label: "In Progress", color: "#3b82f6", bg: "rgba(59,130,246,0.12)" },
  completed: { label: "Completed", color: "#22c55e", bg: "rgba(34,197,94,0.12)" },
  rejected: { label: "Rejected", color: "#ef4444", bg: "rgba(239,68,68,0.12)" },
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

// ─── Activity Feed ─────────────────────────────────────────────
export type ActivityType =
  | "task_assigned"
  | "task_completed"
  | "task_escalated"
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
  {
    label: "Integrations",
    href: "/dashboard/integrations",
    icon: "integrations",
    roles: ["admin"],
  },
];
