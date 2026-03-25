"use client";

import { useState, useMemo, useCallback, useEffect } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";
import TaskDetailDrawer from "@/components/dashboard/TaskDetailDrawer";
import { MOCK_TASKS } from "@/lib/mock-data";
import type { Task, TaskStatus, TaskPriority } from "@/types/dashboard";
import { PRIORITY_CONFIG } from "@/types/dashboard";

const WF_API = process.env.NEXT_PUBLIC_WF_API || "http://localhost:8085";

type FilterPriority = "all" | TaskPriority;

type BackendTask = {
  id: string;
  org_id: string;
  instance_id: string;
  workflow_id: string;
  node_id: string;
  title: string;
  description?: string;
  assigned_role?: string;
  assigned_position?: string;
  assigned_user?: string;
  allowed_actions?: string[];
  sla_days?: number;
  status: string;
  comment?: string;
  created_at: string;
  completed_at?: string;
};

type BackendWorkflow = {
  id: string;
  name: string;
  department?: string;
};

const KANBAN_COLUMNS: { key: "pending" | "in_progress" | "completed"; label: string; statuses: TaskStatus[] }[] = [
  { key: "pending", label: "Pending", statuses: ["pending"] },
  { key: "in_progress", label: "In Progress", statuses: ["in_progress"] },
  { key: "completed", label: "Completed", statuses: ["completed", "escalated", "sent_back"] },
];

const PRIORITY_COLORS: Record<TaskPriority, string> = {
  critical: "#ef4444",
  high: "#f97316",
  medium: "#f59e0b",
  low: "#22c55e",
};

function mapBackendStatus(status: string): TaskStatus {
  switch (status) {
    case "pending":
      return "pending";
    case "in_progress":
      return "in_progress";
    case "escalated":
      return "escalated";
    case "clarification_requested":
      return "sent_back";
    case "approved":
    case "completed":
      return "completed";
    case "rejected":
      return "sent_back";
    default:
      return "in_progress";
  }
}

function priorityFromSLA(slaDays?: number): TaskPriority {
  if (!slaDays || slaDays <= 0) return "medium";
  if (slaDays <= 1) return "critical";
  if (slaDays <= 2) return "high";
  if (slaDays <= 5) return "medium";
  return "low";
}

function toUITask(task: BackendTask, workflow: BackendWorkflow | undefined): Task {
  const status = mapBackendStatus(task.status);
  const priority = priorityFromSLA(task.sla_days);
  const createdAt = task.created_at || new Date().toISOString();
  const dueDate = task.sla_days && task.sla_days > 0
    ? new Date(new Date(createdAt).getTime() + task.sla_days * 24 * 60 * 60 * 1000).toISOString()
    : new Date(new Date(createdAt).getTime() + 2 * 24 * 60 * 60 * 1000).toISOString();

  return {
    id: task.id,
    title: task.title,
    description: task.description || "No description provided.",
    comment: task.comment,
    status,
    priority,
    assignedTo: task.assigned_user || "",
    assignedToName: task.assigned_user ? "You" : (task.assigned_role || "Role Queue"),
    assignedBy: "workflow-engine",
    assignedByName: "Workflow Engine",
    workflowId: task.workflow_id,
    workflowName: workflow?.name || "Workflow",
    departmentOrigin: workflow?.department || "Operations",
    createdAt,
    dueDate,
    completedAt: task.completed_at,
    tags: [task.assigned_role || "workflow"].filter(Boolean),
    stepNumber: 1,
    totalSteps: 1,
    allowedActions: task.allowed_actions,
    nodeId: task.node_id,
    orgId: task.org_id,
    instanceId: task.instance_id,
  };
}

export default function TasksPage() {
  const { getToken, userId } = useAuth();
  const { organization } = useOrganization();

  const [search, setSearch] = useState("");
  const [priorityFilter, setPriorityFilter] = useState<FilterPriority>("all");
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const authFetch = useCallback(async (input: string, init: RequestInit = {}): Promise<Response> => {
    const token = await getToken();
    return fetch(input, {
      ...init,
      headers: {
        ...(init.headers ?? {}),
        Authorization: `Bearer ${token}`,
      },
    });
  }, [getToken]);

  const loadTasks = useCallback(async () => {
    if (!organization?.id || !userId) {
      setTasks(MOCK_TASKS);
      setLoading(false);
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const [taskRes, workflowRes] = await Promise.all([
        authFetch(`${WF_API}/api/orgs/${organization.id}/tasks?assigned_user=${encodeURIComponent(userId)}`),
        authFetch(`${WF_API}/api/orgs/${organization.id}/workflows`),
      ]);

      if (!taskRes.ok || !workflowRes.ok) {
        throw new Error(`Failed to load tasks (${taskRes.status}/${workflowRes.status})`);
      }

      const backendTasks = (await taskRes.json()) as BackendTask[];
      const workflows = (await workflowRes.json()) as BackendWorkflow[];
      const wfMap = new Map(workflows.map((w) => [w.id, w]));

      const mapped = (backendTasks || []).map((t) => toUITask(t, wfMap.get(t.workflow_id)));
      setTasks(mapped.length > 0 ? mapped : []);
    } catch (err: any) {
      setError(err?.message || "Could not load tasks");
      setTasks(MOCK_TASKS);
    } finally {
      setLoading(false);
    }
  }, [authFetch, organization?.id, userId]);

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  const handleSelectTask = useCallback((task: Task) => setSelectedTask(task), []);
  const handleCloseDrawer = useCallback(() => setSelectedTask(null), []);

  const handleAction = useCallback(async (task: Task, action: string, data?: Record<string, string>) => {
    if (!organization?.id || !task.id) return;
    try {
      const response = await authFetch(`${WF_API}/api/orgs/${organization.id}/tasks/${task.id}/${action}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ comment: data?.reason || "" }),
      });
      if (!response.ok) {
        throw new Error(`Action failed (${response.status})`);
      }
      await loadTasks();
      setSelectedTask(null);
    } catch (err) {
      console.error("Task action failed", err);
    }
  }, [authFetch, loadTasks, organization?.id]);

  const filtered = useMemo(() => {
    return tasks.filter((t) => {
      if (priorityFilter !== "all" && t.priority !== priorityFilter) return false;
      if (search) {
        const q = search.toLowerCase();
        return (
          t.title.toLowerCase().includes(q) ||
          t.id.toLowerCase().includes(q) ||
          t.workflowName.toLowerCase().includes(q) ||
          t.departmentOrigin.toLowerCase().includes(q)
        );
      }
      return true;
    });
  }, [priorityFilter, search, tasks]);

  const columns = useMemo(() => {
    return KANBAN_COLUMNS.map((col) => ({
      ...col,
      tasks: filtered
        .filter((t) => col.statuses.includes(t.status))
        .sort((a, b) => {
          return new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime();
        }),
    }));
  }, [filtered]);

  const totalFiltered = filtered.length;

  return (
    <div className="dashboard-page tasks-dashboard-page" style={{ maxWidth: "100%", padding: "0 16px" }}>
      <div className="page-header">
        <div>
          <h2 className="page-title">My Tasks</h2>
          <p className="page-subtitle">{totalFiltered} task{totalFiltered !== 1 ? "s" : ""} across {columns.filter((c) => c.tasks.length > 0).length} columns</p>
        </div>
      </div>

      {loading && <p className="table-muted">Loading tasks...</p>}
      {error && <p style={{ color: "#ef4444" }}>{error}</p>}

      <div className="filters-bar" style={{ marginBottom: 16 }}>
        <div className="filter-search">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
            <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
          </svg>
          <input
            type="text"
            placeholder="Search tasks..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="filter-search-input"
          />
        </div>
        <div className="filter-select-group">
          <label className="filter-label">Priority:</label>
          <select
            value={priorityFilter}
            onChange={(e) => setPriorityFilter(e.target.value as FilterPriority)}
            className="filter-select"
          >
            <option value="all">All</option>
            {(Object.keys(PRIORITY_CONFIG) as TaskPriority[]).map((p) => (
              <option key={p} value={p}>{PRIORITY_CONFIG[p].label}</option>
            ))}
          </select>
        </div>
      </div>

      <div className="kanban-container tasks-kanban-container">
        {columns.map((col) => (
          <div key={col.key} className="kanban-column">
            <div className="kanban-column-header">
              <span className="kanban-column-title">{col.label}</span>
              <span className="kanban-column-count">{col.tasks.length}</span>
            </div>
            <div className="kanban-column-body">
              {col.tasks.length > 0 ? (
                col.tasks.map((task) => (
                  (() => {
                    const overdue = (task.status === "pending" || task.status === "in_progress")
                      && new Date(task.dueDate).getTime() < Date.now();
                    const visualClass = overdue
                      ? "kanban-card-overdue"
                      : task.status === "escalated"
                        ? "kanban-card-escalated"
                        : task.status === "sent_back"
                          ? "kanban-card-sent-back"
                          : "";
                    return (
                  <div
                    key={task.id}
                    className={`kanban-card kanban-priority-${task.priority} ${visualClass}`}
                    onClick={() => handleSelectTask(task)}
                  >
                    <div className="kanban-card-header">
                      <span className="kanban-card-id">{task.id}</span>
                      <span
                        className="kanban-card-priority"
                        style={{ background: PRIORITY_COLORS[task.priority] }}
                      >
                        {PRIORITY_CONFIG[task.priority].label}
                      </span>
                    </div>
                    <h4 className="kanban-card-title">{task.title}</h4>
                    <p className="kanban-card-workflow">{task.workflowName}</p>
                    <div className="kanban-card-meta">
                      <div className="kanban-card-meta-item">{task.departmentOrigin}</div>
                      <div className="kanban-card-meta-item">
                        {new Date(task.dueDate).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
                      </div>
                    </div>
                    <div className="kanban-card-progress">
                      <div className="kanban-card-progress-bar">
                        <div
                          className="kanban-card-progress-fill"
                          style={{ width: `${(task.stepNumber / task.totalSteps) * 100}%` }}
                        />
                      </div>
                      <span className="kanban-card-progress-text">
                        {task.stepNumber}/{task.totalSteps}
                      </span>
                    </div>
                    <div className="kanban-card-footer">
                      <div className="kanban-card-avatar" title={task.assignedToName}>
                        {task.assignedToName.split(" ").map((n: string) => n[0]).join("").slice(0, 2)}
                      </div>
                      <span className="kanban-card-assignee">{task.assignedToName}</span>
                    </div>
                  </div>
                    );
                  })()
                ))
              ) : (
                <div className="kanban-empty">
                  <p>No tasks</p>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>

      <TaskDetailDrawer
        task={selectedTask}
        isOpen={selectedTask !== null}
        onClose={handleCloseDrawer}
        onAction={handleAction}
      />
    </div>
  );
}
