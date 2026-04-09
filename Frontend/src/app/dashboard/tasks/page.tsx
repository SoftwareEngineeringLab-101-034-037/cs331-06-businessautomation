"use client";

import { useState, useMemo, useCallback, useEffect, useRef } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";
import { ToastContainer, useToast } from "@/components/Toast";
import TaskDetailDrawer from "@/components/dashboard/TaskDetailDrawer";
import { authFetch as authFetchWithToken } from "@/lib/auth-fetch";
import { computeHeightBasedProgress, type WorkflowProgressNode } from "@/lib/workflow-progress";
import type { Task, TaskStatus, TaskPriority } from "@/types/dashboard";
import { PRIORITY_CONFIG } from "@/types/dashboard";

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";
const WF_API = process.env.NEXT_PUBLIC_WF_API || "http://localhost:8085";
const ROLE_CACHE_TTL_MS = 2 * 60 * 1000;
const WORKFLOW_CACHE_TTL_MS = 2 * 60 * 1000;

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
  output?: string;
};

type BackendAuditEntry = {
  action?: string;
  details?: Record<string, unknown>;
};

type BackendInstance = {
  id: string;
  workflow_id: string;
  node_states?: Record<string, BackendNodeState>;
  current_node?: string;
  status?: string;
  audit_log?: BackendAuditEntry[];
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
  switch (status.trim().toLowerCase()) {
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

function unknownToErrorString(value: unknown): string {
  if (typeof value === "string") {
    const raw = value.trim();
    if (!raw) return "";
    const bodyMarker = " body=";
    const markerIndex = raw.indexOf(bodyMarker);
    if (markerIndex >= 0) {
      const prefix = raw.slice(0, markerIndex).trim();
      const bodyRaw = raw.slice(markerIndex + bodyMarker.length).trim();
      try {
        const parsed = JSON.parse(bodyRaw) as Record<string, unknown>;
        const nested = typeof parsed.error === "string"
          ? parsed.error.trim()
          : typeof parsed.message === "string"
            ? parsed.message.trim()
            : "";
        if (nested) {
          return prefix ? `${prefix} - ${nested}` : nested;
        }
      } catch {
        // Keep original string when body is not JSON.
      }
    }
    return raw;
  }
  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }
  if (value && typeof value === "object") {
    const payload = value as Record<string, unknown>;
    if (typeof payload.error === "string" && payload.error.trim()) {
      return payload.error.trim();
    }
    if (typeof payload.message === "string" && payload.message.trim()) {
      return payload.message.trim();
    }
    if (typeof payload.reason === "string" && payload.reason.trim()) {
      return payload.reason.trim();
    }
    if (typeof payload.body === "string" && payload.body.trim()) {
      return unknownToErrorString(payload.body);
    }
    if (payload.body && typeof payload.body === "object") {
      return unknownToErrorString(payload.body);
    }
    try {
      return JSON.stringify(value);
    } catch {
      return "";
    }
  }
  return "";
}

function extractInstanceError(instance: BackendInstance | undefined): string | undefined {
  if (!instance) {
    return undefined;
  }

  const auditLog = Array.isArray(instance.audit_log) ? instance.audit_log : [];
  for (let idx = auditLog.length - 1; idx >= 0; idx -= 1) {
    const entry = auditLog[idx];
    if (!entry) {
      continue;
    }
    const details = entry.details || {};
    if (entry.action === "instance_failed") {
      const reason = unknownToErrorString(details.reason);
      if (reason) {
        return reason;
      }
    }
    if (entry.action === "action_failed") {
      const reason = unknownToErrorString(details.error) || unknownToErrorString(details.reason);
      if (reason) {
        return reason;
      }
    }
  }

  const nodeStates = Object.values(instance.node_states || {});
  for (let idx = nodeStates.length - 1; idx >= 0; idx -= 1) {
    const nodeState = nodeStates[idx];
    if (nodeState?.status !== "failed") {
      continue;
    }
    const output = unknownToErrorString(nodeState.output);
    if (output) {
      return output;
    }
  }

  return undefined;
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
    instanceStatus: instance?.status,
    instanceError: extractInstanceError(instance),
  };
}

function formatRelativeTime(iso?: string): string {
  if (!iso) return "just now";
  const time = new Date(iso).getTime();
  if (Number.isNaN(time)) return "just now";

  const diffMs = Date.now() - time;
  if (diffMs <= 0) return "just now";

  const minutes = Math.floor(diffMs / 60000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;

  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;

  const days = Math.floor(hours / 24);
  if (days < 7) return `${days}d ago`;

  const weeks = Math.floor(days / 7);
  if (weeks < 5) return `${weeks}w ago`;

  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;

  const years = Math.floor(days / 365);
  return `${years}y ago`;
}

function getTaskRecency(task: Task): { label: string; at: number } {
  const createdAt = new Date(task.createdAt).getTime();
  const completedAt = task.completedAt ? new Date(task.completedAt).getTime() : Number.NaN;

  const hasCompletedAt = Number.isFinite(completedAt);
  const hasCreatedAt = Number.isFinite(createdAt);

  if (task.status === "completed" && hasCompletedAt) {
    return { label: `Completed ${formatRelativeTime(task.completedAt)}`, at: completedAt };
  }

  if ((task.status === "rejected" || task.status === "escalated" || task.status === "sent_back") && hasCompletedAt) {
    return { label: `Updated ${formatRelativeTime(task.completedAt)}`, at: completedAt };
  }

  if (task.status === "in_progress") {
    return { label: `Started ${formatRelativeTime(task.createdAt)}`, at: hasCreatedAt ? createdAt : 0 };
  }

  return { label: `Created ${formatRelativeTime(task.createdAt)}`, at: hasCreatedAt ? createdAt : 0 };
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
  const recency = getTaskRecency(task);

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
        <div className="kanban-card-meta-item">{recency.label}</div>
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
  const hasLoadedOnceRef = useRef(false);
  const roleCacheRef = useRef<{ orgId: string; userId: string; roles: string[]; fetchedAt: number } | null>(null);
  const workflowCacheRef = useRef<{ orgId: string; workflows: BackendWorkflow[]; fetchedAt: number } | null>(null);

  const authFetch = useCallback(async (
    input: string,
    init: RequestInit = {},
    timeoutMs = 10000,
  ): Promise<Response> => {
    return authFetchWithToken(getToken, input, init, timeoutMs);
  }, [getToken]);

  const loadMyRoles = useCallback(async (orgId: string, currentUserId: string): Promise<string[]> => {
    const cached = roleCacheRef.current;
    const now = Date.now();
    if (
      cached
      && cached.orgId === orgId
      && cached.userId === currentUserId
      && (now - cached.fetchedAt) < ROLE_CACHE_TTL_MS
    ) {
      return cached.roles;
    }

    try {
      const roleRes = await authFetch(`${AUTH_API}/api/orgs/${orgId}/roles`);
      if (!roleRes.ok) {
        const body = await roleRes.text();
        throw new Error(`Failed to load roles (${roleRes.status}): ${body}`);
      }

      const roles = (await roleRes.json()) as BackendRoleSummary[];
      const myRoleNames = (roles || [])
        .filter((role) => (role.members || []).some((member) => member.id === currentUserId))
        .map((role) => role.name)
        .filter(Boolean);

      roleCacheRef.current = {
        orgId,
        userId: currentUserId,
        roles: myRoleNames,
        fetchedAt: now,
      };

      return myRoleNames;
    } catch (err) {
      throw err instanceof Error ? err : new Error("Failed to load roles");
    }
  }, [authFetch]);

  const loadWorkflowsCached = useCallback(async (orgId: string): Promise<BackendWorkflow[]> => {
    const cached = workflowCacheRef.current;
    const now = Date.now();
    if (cached && cached.orgId === orgId && (now - cached.fetchedAt) < WORKFLOW_CACHE_TTL_MS) {
      return cached.workflows;
    }

    const workflowRes = await authFetch(`${WF_API}/api/orgs/${orgId}/workflows`);
    if (!workflowRes.ok) {
      throw new Error(`Failed to load workflows (${workflowRes.status})`);
    }
    const workflows = (await workflowRes.json()) as BackendWorkflow[];
    const normalized = workflows || [];
    workflowCacheRef.current = {
      orgId,
      workflows: normalized,
      fetchedAt: now,
    };
    return normalized;
  }, [authFetch]);

  const loadTasks = useCallback(async () => {
    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;

    const orgId = organization?.id;
    if (!orgId || !userId) {
      if (requestVersion === requestVersionRef.current) {
        setError(null);
        setTasks([]);
        setLoading(false);
        hasLoadedOnceRef.current = false;
      }
      return;
    }

    if (requestVersion === requestVersionRef.current) {
      // Keep initial-load feedback, but avoid layout shifts during background auto-refresh.
      setLoading(!hasLoadedOnceRef.current);
      setError(null);
    }
    try {
      const [taskRes, instanceRes, workflows] = await Promise.all([
        authFetch(`${WF_API}/api/orgs/${orgId}/tasks?assigned_user=${encodeURIComponent(userId)}`),
        authFetch(`${WF_API}/api/orgs/${orgId}/instances?compact=true`),
        loadWorkflowsCached(orgId),
      ]);

      if (!taskRes.ok || !instanceRes.ok) {
        throw new Error(`Failed to load tasks (${taskRes.status}/${instanceRes.status})`);
      }

      const backendTasks = (await taskRes.json()) as BackendTask[];
      const instances = (await instanceRes.json()) as BackendInstance[];
      const wfMap = new Map(workflows.map((w) => [w.id, w]));
      const instanceMap = new Map(instances.map((inst) => [inst.id, inst]));
      const taskMap = new Map<string, BackendTask>();

      for (const task of backendTasks || []) {
        taskMap.set(task.id, task);
      }

      try {
        const myRoleNames = await loadMyRoles(orgId, userId);
        if (requestVersion !== requestVersionRef.current) {
          return;
        }

        if (myRoleNames.length > 0) {
          const roleTaskRes = await authFetch(`${WF_API}/api/orgs/${orgId}/tasks?roles=${encodeURIComponent(myRoleNames.join(","))}`);
          if (requestVersion !== requestVersionRef.current) {
            return;
          }
          if (roleTaskRes.ok) {
            const roleTasks = (await roleTaskRes.json()) as BackendTask[];
            for (const task of roleTasks || []) {
              if (task.assigned_user && task.assigned_user !== userId) {
                continue;
              }
              taskMap.set(task.id, task);
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
        setTasks((current) => (hasLoadedOnceRef.current && current.length > 0 ? current : []));
      }
    } finally {
      if (requestVersion === requestVersionRef.current) {
        setLoading(false);
        hasLoadedOnceRef.current = true;
      }
    }
  }, [authFetch, organization?.id, userId, loadMyRoles, loadWorkflowsCached]);

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
          return getTaskRecency(b).at - getTaskRecency(a).at;
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
