"use client";

import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";
import { ToastContainer, useToast } from "@/components/Toast";
import TaskDetailDrawer from "@/components/dashboard/TaskDetailDrawer";
import { MOCK_TASKS } from "@/lib/mock-data";
import { computeHeightBasedProgress, type WorkflowProgressNode } from "@/lib/workflow-progress";
import type { Task, TaskStatus, TaskPriority } from "@/types/dashboard";
import { PRIORITY_CONFIG } from "@/types/dashboard";

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";
const WF_API = process.env.NEXT_PUBLIC_WF_API || "http://localhost:8085";
const DEMO_TASKS_ENABLED = process.env.NEXT_PUBLIC_ENABLE_DEMO_TASKS === "true";

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
  action_committed?: string;
  sla_days?: number;
  status: string;
  comment?: string;
  visible_data?: Record<string, unknown>;
  created_at: string;
  completed_at?: string;
};

type BackendWorkflow = {
  id: string;
  name: string;
  department?: string;
  nodes?: WorkflowProgressNode[];
};

type BackendNodeState = {
  status?: string;
};

type BackendInstance = {
  id: string;
  workflow_id: string;
  node_states?: Record<string, BackendNodeState>;
  current_node?: string;
  status?: string;
};

type BackendRoleMember = {
  id: string;
};

type BackendRoleSummary = {
  id: string;
  name: string;
  members?: BackendRoleMember[];
};

const KANBAN_COLUMNS: { key: "pending" | "in_progress" | "completed"; label: string; statuses: TaskStatus[] }[] = [
  { key: "pending", label: "Pending", statuses: ["pending"] },
  { key: "in_progress", label: "In Progress", statuses: ["in_progress"] },
  { key: "completed", label: "Completed", statuses: ["completed", "rejected", "escalated", "sent_back"] },
];

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
      return "rejected";
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

function computeInstanceProgress(instance: BackendInstance | undefined, workflow: BackendWorkflow | undefined): { stepNumber: number; totalSteps: number } {
  const progress = computeHeightBasedProgress(
    workflow?.nodes,
    instance?.node_states,
    instance?.current_node,
    instance?.status,
  );
  return {
    stepNumber: progress.checkpointNumber,
    totalSteps: progress.totalCheckpoints,
  };
}

function toUITask(task: BackendTask, workflow: BackendWorkflow | undefined, instance: BackendInstance | undefined): Task {
  const status = mapBackendStatus(task.status);
  const priority = priorityFromSLA(task.sla_days);
  const createdAt = task.created_at || new Date().toISOString();
  const dueDate = task.sla_days && task.sla_days > 0
    ? new Date(new Date(createdAt).getTime() + task.sla_days * 24 * 60 * 60 * 1000).toISOString()
    : new Date(new Date(createdAt).getTime() + 2 * 24 * 60 * 60 * 1000).toISOString();
  const progress = computeInstanceProgress(instance, workflow);

  return {
    id: task.id,
    title: task.title,
    description: task.description || "No description provided.",
    comment: task.comment,
    actionCommitted: task.action_committed,
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
    stepNumber: progress.stepNumber,
    totalSteps: progress.totalSteps,
    allowedActions: task.allowed_actions,
    visibleData: task.visible_data,
    nodeId: task.node_id,
    orgId: task.org_id,
    instanceId: task.instance_id,
  };
}

function TaskCard({
  task,
  onSelect,
  onStart,
}: {
  task: Task;
  onSelect: (task: Task) => void;
  onStart: (task: Task) => void;
}) {
  const overdue = (task.status === "pending" || task.status === "in_progress")
    && new Date(task.dueDate).getTime() < Date.now();
  const visualClass = overdue
    ? "kanban-card-overdue"
    : task.status === "rejected"
      ? "kanban-card-rejected"
      : task.status === "escalated"
        ? "kanban-card-escalated"
        : task.status === "sent_back"
          ? "kanban-card-sent-back"
          : "";

  return (
    <div
      className={`kanban-card ${visualClass}`}
      onClick={() => onSelect(task)}
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onSelect(task);
        }
      }}
      role="button"
      tabIndex={0}
      aria-label={`Open task ${task.title}`}
    >
      <div className="kanban-card-header">
        <span className="kanban-card-id">{task.id}</span>
        <span className={`kanban-card-priority ${task.priority}`}>
          {PRIORITY_CONFIG[task.priority].label}
        </span>
      </div>
      <h4 className="kanban-card-title">{task.title}</h4>
      <p className="kanban-card-workflow">{task.workflowName}</p>
      {task.status === "completed" && task.actionCommitted && (
        <p className="kanban-card-workflow">
          Outcome: {task.actionCommitted.replaceAll("_", " ").replace(/\b\w/g, (char) => char.toUpperCase())}
        </p>
      )}
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
          {task.stepNumber}/{task.totalSteps} ({Math.round((task.stepNumber / task.totalSteps) * 100)}%)
        </span>
      </div>
      <div className="kanban-card-footer">
        <div className="kanban-card-footer-main">
          <div className="kanban-card-avatar" title={task.assignedToName}>
            {task.assignedToName.split(" ").map((n: string) => n[0]).join("").slice(0, 2)}
          </div>
          <span className="kanban-card-assignee">{task.assignedToName}</span>
        </div>
        {task.status === "pending" && (
          <button
            type="button"
            className="kanban-card-start-btn"
            onClick={(event) => {
              event.stopPropagation();
              onStart(task);
            }}
          >
            Start
          </button>
        )}
      </div>
    </div>
  );
}

function mergeSignals(signals: Array<AbortSignal | null | undefined>): AbortSignal | undefined {
  const activeSignals = signals.filter(Boolean) as AbortSignal[];
  if (activeSignals.length === 0) {
    return undefined;
  }
  if (activeSignals.length === 1) {
    return activeSignals[0];
  }

  const controller = new AbortController();
  const abort = () => controller.abort();
  for (const signal of activeSignals) {
    if (signal.aborted) {
      controller.abort();
      break;
    }
    signal.addEventListener("abort", abort, { once: true });
  }
  return controller.signal;
}

export default function TasksPage() {
  const { getToken, userId } = useAuth();
  const { organization } = useOrganization();
  const { toasts, showToast, dismissToast } = useToast();

  const [search, setSearch] = useState("");
  const [priorityFilter, setPriorityFilter] = useState<FilterPriority>("all");
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const requestVersionRef = useRef(0);

  const authFetch = useCallback(async (
    input: string,
    init: RequestInit = {},
    timeoutMs = 10000,
  ): Promise<Response> => {
    const token = await getToken();
    const controller = new AbortController();
    const timeoutID = setTimeout(() => controller.abort(), timeoutMs);
    try {
      return await fetch(input, {
        ...init,
        signal: mergeSignals([init.signal, controller.signal]),
        headers: {
          ...(init.headers ?? {}),
          Authorization: `Bearer ${token}`,
        },
      });
    } finally {
      clearTimeout(timeoutID);
    }
  }, [getToken]);

  const loadTasks = useCallback(async () => {
    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;

    if (!organization?.id || !userId) {
      if (requestVersion === requestVersionRef.current) {
        setTasks(DEMO_TASKS_ENABLED ? MOCK_TASKS : []);
        setLoading(false);
      }
      return;
    }

    if (requestVersion === requestVersionRef.current) {
      setLoading(true);
      setError(null);
    }
    try {
      const [taskRes, workflowRes] = await Promise.all([
        authFetch(`${WF_API}/api/orgs/${organization.id}/tasks?assigned_user=${encodeURIComponent(userId)}`),
        authFetch(`${WF_API}/api/orgs/${organization.id}/workflows`),
      ]);

      const instanceRes = await authFetch(`${WF_API}/api/orgs/${organization.id}/instances`);

      if (!taskRes.ok || !workflowRes.ok || !instanceRes.ok) {
        throw new Error(`Failed to load tasks (${taskRes.status}/${workflowRes.status}/${instanceRes.status})`);
      }

      const backendTasks = (await taskRes.json()) as BackendTask[];
      const workflows = (await workflowRes.json()) as BackendWorkflow[];
      const instances = (await instanceRes.json()) as BackendInstance[];
      const wfMap = new Map(workflows.map((w) => [w.id, w]));
      const instanceMap = new Map(instances.map((inst) => [inst.id, inst]));
      const taskMap = new Map<string, BackendTask>();

      for (const task of backendTasks || []) {
        taskMap.set(task.id, task);
      }

      try {
        const roleRes = await authFetch(`${AUTH_API}/api/orgs/${organization.id}/roles`);
        if (requestVersion !== requestVersionRef.current) {
          return;
        }
        if (roleRes.ok) {
          const roles = (await roleRes.json()) as BackendRoleSummary[];
          const myRoleNames = (roles || [])
            .filter((role) => (role.members || []).some((member) => member.id === userId))
            .map((role) => role.name)
            .filter(Boolean);

          if (myRoleNames.length > 0) {
            const roleTaskResponses = await Promise.allSettled(
              myRoleNames.map(async (roleName) => {
                const response = await authFetch(`${WF_API}/api/orgs/${organization.id}/tasks?role=${encodeURIComponent(roleName)}`);
                if (!response.ok) {
                  throw new Error(`Failed to load role tasks for ${roleName}`);
                }
                return await response.json() as BackendTask[];
              }),
            );

            if (requestVersion !== requestVersionRef.current) {
              return;
            }

            for (const result of roleTaskResponses) {
              if (result.status !== "fulfilled") {
                continue;
              }
              for (const task of result.value || []) {
                if (task.assigned_user && task.assigned_user !== userId) {
                  continue;
                }
                taskMap.set(task.id, task);
              }
            }
          }
        }
      } catch (roleErr) {
        console.warn("Failed to load role-based tasks", roleErr);
      }

      const mapped = Array.from(taskMap.values()).map((t) => toUITask(t, wfMap.get(t.workflow_id), instanceMap.get(t.instance_id)));
      if (requestVersion === requestVersionRef.current) {
        setTasks(mapped.length > 0 ? mapped : []);
      }
    } catch (err: any) {
      if (requestVersion === requestVersionRef.current) {
        setError(err?.message || "Could not load tasks");
        setTasks(DEMO_TASKS_ENABLED ? MOCK_TASKS : []);
      }
    } finally {
      if (requestVersion === requestVersionRef.current) {
        setLoading(false);
      }
    }
  }, [authFetch, organization?.id, userId]);

  useEffect(() => {
    loadTasks();
  }, [loadTasks]);

  useEffect(() => {
    const intervalID = window.setInterval(() => {
      void loadTasks();
    }, 15000);
    return () => {
      window.clearInterval(intervalID);
    };
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
        let message = `Action failed (${response.status})`;
        try {
          const payload = (await response.json()) as { error?: string };
          if (payload?.error) {
            message = payload.error;
          }
        } catch {
          // Keep the generic status-based message when the body is not JSON.
        }
        throw new Error(message);
      }
      await loadTasks();
      setSelectedTask(null);
    } catch (err: any) {
      console.error("Task action failed", err);
      showToast(
        err?.name === "AbortError"
          ? "Task action timed out. Please try again."
          : (err?.message || "Task action failed."),
        "error",
      );
    }
  }, [authFetch, loadTasks, organization?.id, showToast]);

  const handleQuickStart = useCallback((task: Task) => {
    void handleAction(task, "start");
  }, [handleAction]);

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

  return (
    <div className="dashboard-page tasks-dashboard-page" style={{ maxWidth: "100%", padding: "28px 16px 0" }}>
      {loading && <p className="table-muted">Loading tasks...</p>}
      {error && <p style={{ color: "#ef4444" }}>{error}</p>}

      <div className="filters-bar">
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
                  <TaskCard key={task.id} task={task} onSelect={handleSelectTask} onStart={handleQuickStart} />
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
      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </div>
  );
}
