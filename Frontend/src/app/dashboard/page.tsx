"use client";

import { useState, useCallback, useEffect, useRef, useMemo } from "react";
import Link from "next/link";
import { useAuth, useOrganization } from "@clerk/nextjs";
import TaskDetailDrawer from "@/components/dashboard/TaskDetailDrawer";
import ActivityFeed from "@/components/dashboard/ActivityFeed";
import { RoleGate, useRole } from "@/components/dashboard/RoleProvider";
import { authFetch as authFetchWithToken } from "@/lib/auth-fetch";
import { computeHeightBasedProgress, type WorkflowProgressNode } from "@/lib/workflow-progress";
import { ROLE_LABELS, TASK_STATUS_CONFIG, PRIORITY_CONFIG, normalizeTaskPriority } from "@/types/dashboard";
import type { ActivityItem, Task, TaskPriority, TaskStatus } from "@/types/dashboard";

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";
const WF_API = process.env.NEXT_PUBLIC_WF_API || "http://localhost:8085";
const ROLE_CACHE_TTL_MS = 2 * 60 * 1000;
const WORKFLOW_CACHE_TTL_MS = 2 * 60 * 1000;
const REFRESH_INTERVAL_MS = 15000;
const TIMELINE_START = new Date("2026-01-01T00:00:00");

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
  priority?: string;
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
  status?: string;
  created_by?: string;
  created_at?: string;
  updated_at?: string;
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
  workflow_name?: string;
  node_states?: Record<string, BackendNodeState>;
  current_node?: string;
  status?: string;
  audit_log?: BackendAuditEntry[];
  started_at?: string;
  completed_at?: string;
};

type BackendRoleMember = {
  id: string;
};

type BackendRoleSummary = {
  id: string;
  name: string;
  members?: BackendRoleMember[];
};

function startOfDay(date: Date): Date {
  return new Date(date.getFullYear(), date.getMonth(), date.getDate());
}

function addDays(date: Date, days: number): Date {
  const d = new Date(date);
  d.setDate(d.getDate() + days);
  return d;
}

function daysBetween(a: Date, b: Date): number {
  return (b.getTime() - a.getTime()) / (1000 * 60 * 60 * 24);
}

function formatShortDate(d: Date): string {
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

function formatWeekRange(start: Date): string {
  const end = addDays(start, 6);
  return `${formatShortDate(start)} - ${formatShortDate(end)}`;
}

function clampDate(date: Date, min: Date, max: Date): Date {
  if (date < min) return min;
  if (date > max) return max;
  return date;
}

function parseISO(value?: string): Date | null {
  if (!value) return null;
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return null;
  return parsed;
}

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
  if (!slaDays || slaDays <= 0) return "general";
  if (slaDays <= 1) return "critical";
  if (slaDays <= 2) return "high";
  if (slaDays <= 5) return "general";
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

function parseBodyError(raw: string): string | null {
  const bodyMarker = " body=";
  const markerIndex = raw.indexOf(bodyMarker);
  if (markerIndex < 0) return null;

  const prefix = raw.slice(0, markerIndex).trim();
  const bodyRaw = raw.slice(markerIndex + bodyMarker.length).trim();
  try {
    const parsed = JSON.parse(bodyRaw) as Record<string, unknown>;
    const nested = typeof parsed.error === "string"
      ? parsed.error.trim()
      : typeof parsed.message === "string"
        ? parsed.message.trim()
        : "";
    if (!nested) return null;
    return prefix ? `${prefix} - ${nested}` : nested;
  } catch {
    return null;
  }
}

function unknownToErrorString(value: unknown): string {
  if (typeof value === "string") {
    const raw = value.trim();
    if (!raw) return "";
    const parsedBodyError = parseBodyError(raw);
    if (parsedBodyError) return parsedBodyError;
    return raw;
  }

  if (typeof value === "number" || typeof value === "boolean") {
    return String(value);
  }

  if (value && typeof value === "object") {
    const payload = value as Record<string, unknown>;
    if (typeof payload.error === "string" && payload.error.trim()) return payload.error.trim();
    if (typeof payload.message === "string" && payload.message.trim()) return payload.message.trim();
    if (typeof payload.reason === "string" && payload.reason.trim()) return payload.reason.trim();
    if (typeof payload.body === "string" && payload.body.trim()) return unknownToErrorString(payload.body);
    if (payload.body && typeof payload.body === "object") return unknownToErrorString(payload.body);
    try {
      return JSON.stringify(value);
    } catch {
      return "";
    }
  }

  return "";
}

function extractInstanceError(instance: BackendInstance | undefined): string | undefined {
  if (!instance) return undefined;

  const auditLog = Array.isArray(instance.audit_log) ? instance.audit_log : [];
  for (let idx = auditLog.length - 1; idx >= 0; idx -= 1) {
    const entry = auditLog[idx];
    if (!entry) continue;

    const details = entry.details || {};
    if (entry.action === "instance_failed") {
      const reason = unknownToErrorString(details.reason);
      if (reason) return reason;
    }
    if (entry.action === "action_failed") {
      const reason = unknownToErrorString(details.error) || unknownToErrorString(details.reason);
      if (reason) return reason;
    }
  }

  const nodeStates = Object.values(instance.node_states || {});
  for (let idx = nodeStates.length - 1; idx >= 0; idx -= 1) {
    const nodeState = nodeStates[idx];
    if (nodeState?.status !== "failed") continue;
    const output = unknownToErrorString(nodeState.output);
    if (output) return output;
  }

  return undefined;
}

function toUITask(task: BackendTask, workflow: BackendWorkflow | undefined, instance: BackendInstance | undefined): Task {
  const status = mapBackendStatus(task.status);
  const priority = normalizeTaskPriority(task.priority || priorityFromSLA(task.sla_days));
  const createdAt = task.created_at || new Date().toISOString();
  const fallbackDue = new Date(new Date(createdAt).getTime() + 2 * 24 * 60 * 60 * 1000).toISOString();
  const dueDate = task.sla_days && task.sla_days > 0
    ? new Date(new Date(createdAt).getTime() + task.sla_days * 24 * 60 * 60 * 1000).toISOString()
    : fallbackDue;

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

function buildActivity(
  tasks: Task[],
  workflows: BackendWorkflow[],
  instances: BackendInstance[],
): ActivityItem[] {
  const workflowNameByID = new Map(workflows.map((wf) => [wf.id, wf.name || "Workflow"]));
  const items: ActivityItem[] = [];

  for (const task of tasks) {
    const completedAt = parseISO(task.completedAt);
    const createdAt = parseISO(task.createdAt);
    const timestamp = completedAt || createdAt;
    if (!timestamp) continue;

    if (task.status === "completed") {
      items.push({
        id: `task-completed-${task.id}-${timestamp.getTime()}`,
        type: "task_completed",
        message: `Task \"${task.title}\" completed in \"${task.workflowName}\"`,
        actor: task.assignedToName || "Workflow Engine",
        timestamp: timestamp.toISOString(),
        relatedId: task.id,
      });
      continue;
    }

    if (task.status === "escalated" || task.status === "overdue") {
      items.push({
        id: `task-escalated-${task.id}-${timestamp.getTime()}`,
        type: "task_escalated",
        message: `Task \"${task.title}\" needs urgent attention`,
        actor: "Workflow Engine",
        timestamp: timestamp.toISOString(),
        relatedId: task.id,
      });
      continue;
    }

    if (task.status === "pending" || task.status === "in_progress") {
      items.push({
        id: `task-assigned-${task.id}-${timestamp.getTime()}`,
        type: "task_assigned",
        message: `Task \"${task.title}\" is waiting in \"${task.workflowName}\"`,
        actor: "Workflow Engine",
        timestamp: timestamp.toISOString(),
        relatedId: task.id,
      });
      continue;
    }
  }

  for (const instance of instances) {
    const startedAt = parseISO(instance.started_at);
    const completedAt = parseISO(instance.completed_at);
    const timestamp = completedAt || startedAt;
    if (!timestamp) continue;

    const wfName = instance.workflow_name || workflowNameByID.get(instance.workflow_id) || "Workflow";
    if ((instance.status || "").toLowerCase() === "failed") {
      items.push({
        id: `instance-failed-${instance.id}-${timestamp.getTime()}`,
        type: "task_escalated",
        message: `Instance in \"${wfName}\" failed`,
        actor: "Workflow Engine",
        timestamp: timestamp.toISOString(),
        relatedId: instance.id,
      });
    }
  }

  for (const wf of workflows) {
    const ts = parseISO(wf.updated_at) || parseISO(wf.created_at);
    if (!ts) continue;

    items.push({
      id: `workflow-${wf.id}-${ts.getTime()}`,
      type: "workflow_published",
      message: `Workflow \"${wf.name}\" is ${wf.status || "active"}`,
      actor: wf.created_by || "Admin",
      timestamp: ts.toISOString(),
      relatedId: wf.id,
    });
  }

  items.sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime());

  const uniqueByID = new Map<string, ActivityItem>();
  for (const item of items) {
    if (!uniqueByID.has(item.id)) {
      uniqueByID.set(item.id, item);
    }
  }

  return Array.from(uniqueByID.values()).slice(0, 20);
}

export default function DashboardOverview() {
  const { role } = useRole();
  const { getToken, userId } = useAuth();
  const { organization } = useOrganization();

  const roleLabel = role ? ROLE_LABELS[role] : "Loading role...";

  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [showSearch, setShowSearch] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [tasks, setTasks] = useState<Task[]>([]);
  const [workflows, setWorkflows] = useState<BackendWorkflow[]>([]);
  const [instances, setInstances] = useState<BackendInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedWeekIndex, setSelectedWeekIndex] = useState(0);

  const searchRef = useRef<HTMLInputElement>(null);
  const requestVersionRef = useRef(0);
  const hasLoadedOnceRef = useRef(false);
  const initializedWeekRef = useRef(false);
  const roleCacheRef = useRef<{ orgId: string; userId: string; roles: string[]; fetchedAt: number } | null>(null);
  const workflowCacheRef = useRef<{ orgId: string; workflows: BackendWorkflow[]; fetchedAt: number } | null>(null);

  const handleSelectTask = useCallback((task: Task) => setSelectedTask(task), []);
  const handleCloseDrawer = useCallback(() => setSelectedTask(null), []);

  const authFetch = useCallback(async (
    input: string,
    init: RequestInit = {},
    timeoutMs = 10000,
  ): Promise<Response> => {
    return authFetchWithToken(getToken, input, init, timeoutMs);
  }, [getToken]);

  const loadMyRoles = useCallback(async (orgId: string, currentUserID: string): Promise<string[]> => {
    const cached = roleCacheRef.current;
    const now = Date.now();
    if (
      cached
      && cached.orgId === orgId
      && cached.userId === currentUserID
      && (now - cached.fetchedAt) < ROLE_CACHE_TTL_MS
    ) {
      return cached.roles;
    }

    const roleRes = await authFetch(`${AUTH_API}/api/orgs/${orgId}/roles`);
    if (!roleRes.ok) {
      throw new Error(`Failed to load roles (${roleRes.status})`);
    }

    const roles = (await roleRes.json()) as BackendRoleSummary[];
    const myRoleNames = (roles || [])
      .filter((entry) => (entry.members || []).some((member) => member.id === currentUserID))
      .map((entry) => entry.name)
      .filter(Boolean);

    roleCacheRef.current = {
      orgId,
      userId: currentUserID,
      roles: myRoleNames,
      fetchedAt: now,
    };

    return myRoleNames;
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

    const loaded = (await workflowRes.json()) as BackendWorkflow[];
    const normalized = loaded || [];
    workflowCacheRef.current = {
      orgId,
      workflows: normalized,
      fetchedAt: now,
    };

    return normalized;
  }, [authFetch]);

  const loadOverviewData = useCallback(async () => {
    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;

    const orgId = organization?.id;
    if (!orgId || !userId) {
      if (requestVersion === requestVersionRef.current) {
        setError(null);
        setTasks([]);
        setInstances([]);
        setWorkflows([]);
        setLoading(false);
        hasLoadedOnceRef.current = false;
      }
      return;
    }

    if (requestVersion === requestVersionRef.current) {
      setLoading(!hasLoadedOnceRef.current);
      setError(null);
    }

    try {
      const [taskRes, instanceRes, loadedWorkflows] = await Promise.all([
        authFetch(`${WF_API}/api/orgs/${orgId}/tasks?assigned_user=${encodeURIComponent(userId)}`),
        authFetch(`${WF_API}/api/orgs/${orgId}/instances`),
        loadWorkflowsCached(orgId),
      ]);

      if (!taskRes.ok || !instanceRes.ok) {
        throw new Error(`Failed to load overview data (${taskRes.status}/${instanceRes.status})`);
      }

      const directTasks = (await taskRes.json()) as BackendTask[];
      const loadedInstances = (await instanceRes.json()) as BackendInstance[];

      const workflowByID = new Map(loadedWorkflows.map((wf) => [wf.id, wf]));
      const instanceByID = new Map(loadedInstances.map((inst) => [inst.id, inst]));
      const taskMap = new Map<string, BackendTask>();

      for (const task of directTasks || []) {
        taskMap.set(task.id, task);
      }

      try {
        const myRoleNames = await loadMyRoles(orgId, userId);
        if (requestVersion !== requestVersionRef.current) return;

        if (myRoleNames.length > 0) {
          const roleTaskRes = await authFetch(`${WF_API}/api/orgs/${orgId}/tasks?roles=${encodeURIComponent(myRoleNames.join(","))}`);
          if (requestVersion !== requestVersionRef.current) return;

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
        console.warn("Overview role-task load failed", roleErr);
      }

      const mappedTasks = Array.from(taskMap.values())
        .map((task) => toUITask(task, workflowByID.get(task.workflow_id), instanceByID.get(task.instance_id)))
        .sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());

      if (requestVersion === requestVersionRef.current) {
        setTasks(mappedTasks);
        setInstances(loadedInstances || []);
        setWorkflows(loadedWorkflows || []);
      }
    } catch (err: unknown) {
      if (requestVersion === requestVersionRef.current) {
        const message = err instanceof Error ? err.message : "";
        setError(message || "Could not load overview data");
      }
    } finally {
      if (requestVersion === requestVersionRef.current) {
        setLoading(false);
        hasLoadedOnceRef.current = true;
      }
    }
  }, [authFetch, loadMyRoles, loadWorkflowsCached, organization?.id, userId]);

  useEffect(() => {
    void loadOverviewData();
  }, [loadOverviewData]);

  useEffect(() => {
    const intervalID = window.setInterval(() => {
      void loadOverviewData();
    }, REFRESH_INTERVAL_MS);

    return () => {
      window.clearInterval(intervalID);
    };
  }, [loadOverviewData]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setShowSearch(true);
      }
      if (e.key === "Escape") {
        setShowSearch(false);
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  useEffect(() => {
    if (showSearch && searchRef.current) {
      searchRef.current.focus();
    }
  }, [showSearch]);

  const pendingTasks = tasks.filter((task) => task.status === "pending").length;
  const inProgressTasks = tasks.filter((task) => task.status === "in_progress").length;
  const completedTasks = tasks.filter((task) => ["completed", "rejected", "sent_back", "cancelled"].includes(task.status)).length;
  const atRiskTasks = useMemo(() => tasks.filter((task) => {
    if (task.status === "escalated") return true;
    if (task.status !== "pending" && task.status !== "in_progress") return false;
    return new Date(task.dueDate).getTime() < Date.now();
  }), [tasks]);
  const atRiskTaskIDs = useMemo(() => new Set(atRiskTasks.map((task) => task.id)), [atRiskTasks]);
  const overdueTasks = atRiskTasks.length;

  const searchResults = searchQuery.trim()
    ? tasks.filter((task) => {
      const q = searchQuery.toLowerCase();
      return (
        task.title.toLowerCase().includes(q)
        || task.id.toLowerCase().includes(q)
        || task.workflowName.toLowerCase().includes(q)
      );
    }).slice(0, 5)
    : [];

  const activeTask = useMemo(() => {
    const priorityRank: Record<TaskPriority, number> = { critical: 0, high: 1, general: 2, low: 3 };
    return [...tasks]
      .filter((task) => !["completed", "cancelled", "rejected"].includes(task.status))
      .sort((a, b) => {
        const byPriority = priorityRank[a.priority] - priorityRank[b.priority];
        if (byPriority !== 0) return byPriority;
        return new Date(a.dueDate).getTime() - new Date(b.dueDate).getTime();
      })[0];
  }, [tasks]);

  const activityItems = useMemo(() => buildActivity(tasks, workflows, instances), [tasks, workflows, instances]);

  const timelineMinStart = useMemo(() => startOfDay(TIMELINE_START), []);

  const { weekStarts, currentWeekIndex } = useMemo(() => {
    const today = startOfDay(new Date());
    let latest = today;

    for (const task of tasks) {
      const created = parseISO(task.createdAt);
      const due = parseISO(task.dueDate);
      if (created && created > latest) latest = created;
      if (due && due > latest) latest = due;
    }

    const safeLatest = latest < timelineMinStart ? timelineMinStart : latest;
    const lastWeekIndex = Math.max(0, Math.floor(daysBetween(timelineMinStart, safeLatest) / 7));
    const starts: Date[] = [];
    for (let idx = 0; idx <= lastWeekIndex; idx += 1) {
      starts.push(addDays(timelineMinStart, idx * 7));
    }

    const rawCurrentWeekIndex = Math.max(0, Math.floor(daysBetween(timelineMinStart, today) / 7));
    const cappedCurrentWeekIndex = Math.min(rawCurrentWeekIndex, starts.length - 1);

    return {
      weekStarts: starts,
      currentWeekIndex: Math.max(0, cappedCurrentWeekIndex),
    };
  }, [tasks, timelineMinStart]);

  useEffect(() => {
    if (!initializedWeekRef.current) {
      setSelectedWeekIndex(currentWeekIndex);
      initializedWeekRef.current = true;
      return;
    }

    if (selectedWeekIndex > weekStarts.length - 1) {
      setSelectedWeekIndex(Math.max(0, weekStarts.length - 1));
    }
  }, [currentWeekIndex, selectedWeekIndex, weekStarts.length]);

  const activeWeekIndex = Math.min(Math.max(selectedWeekIndex, 0), Math.max(0, weekStarts.length - 1));
  const weekStart = weekStarts[activeWeekIndex] || timelineMinStart;
  const weekEnd = addDays(weekStart, 7);
  const weekDays = useMemo(() => Array.from({ length: 7 }, (_, idx) => addDays(weekStart, idx)), [weekStart]);

  const weekTasks = useMemo(() => {
    return tasks
      .filter((task) => {
        const created = parseISO(task.createdAt) || new Date();
        const dueRaw = parseISO(task.dueDate) || created;
        const finish = dueRaw > created ? dueRaw : new Date(created.getTime() + 60 * 60 * 1000);
        return created < weekEnd && finish > weekStart;
      })
      .sort((a, b) => {
        return new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
      });
  }, [tasks, weekEnd, weekStart]);

  const now = new Date();
  const showTodayMarker = now >= weekStart && now < weekEnd;
  const todayPct = (daysBetween(weekStart, now) / 7) * 100;

  const analysisSummary = useMemo(() => {
    const totalWorkflows = workflows.length;
    const activeWorkflows = workflows.filter((wf) => (wf.status || "").toLowerCase() === "active").length;
    const failedInstances = instances.filter((inst) => (inst.status || "").toLowerCase() === "failed").length;
    return { totalWorkflows, activeWorkflows, failedInstances };
  }, [instances, workflows]);

  return (
    <div className="dashboard-page">
      {showSearch && (
        <div className="cmd-search-overlay" onClick={() => setShowSearch(false)}>
          <div className="cmd-search-box" onClick={(e) => e.stopPropagation()}>
            <div className="cmd-search-input-row">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
              </svg>
              <input
                ref={searchRef}
                type="text"
                className="cmd-search-input"
                placeholder="Search tasks, workflows..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
              <span className="cmd-search-kbd">ESC</span>
            </div>
            {searchResults.length > 0 && (
              <div className="cmd-search-results">
                {searchResults.map((task) => (
                  <div
                    key={task.id}
                    className="cmd-search-item"
                    onClick={() => {
                      handleSelectTask(task);
                      setShowSearch(false);
                      setSearchQuery("");
                    }}
                  >
                    <div className="cmd-search-item-icon">
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15a2.25 2.25 0 0 1 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25Z" />
                      </svg>
                    </div>
                    <div className="cmd-search-item-text">
                      <div className="cmd-search-item-title">{task.title}</div>
                      <div className="cmd-search-item-desc">{task.id} · {task.workflowName}</div>
                    </div>
                  </div>
                ))}
              </div>
            )}
            <div className="cmd-search-hint">
              Type to search · <strong>Enter</strong> to select · <strong>Esc</strong> to close
            </div>
          </div>
        </div>
      )}

      <div className="overview-top">
        <div className="overview-stats">
          <div className="overview-stat-card">
            <span className="overview-stat-value">{pendingTasks}</span>
            <span className="overview-stat-label">Pending</span>
            <span className="overview-stat-hint">Awaiting action</span>
          </div>
          <div className="overview-stat-card">
            <span className="overview-stat-value">{inProgressTasks}</span>
            <span className="overview-stat-label">In Progress</span>
            <span className="overview-stat-hint">Live queue</span>
          </div>
          <div className="overview-stat-card">
            <span className="overview-stat-value">{completedTasks}</span>
            <span className="overview-stat-label">Completed</span>
            <span className="overview-stat-hint">From real instances</span>
          </div>
          <div className="overview-stat-card" data-alert={overdueTasks > 0 ? "true" : undefined}>
            <span className="overview-stat-value">{overdueTasks}</span>
            <span className="overview-stat-label">Overdue</span>
            {overdueTasks > 0 && <span className="overview-stat-hint overview-stat-danger">Needs attention</span>}
          </div>
        </div>
        <div className="overview-welcome">
          <div>
            <h2 className="overview-greeting">Good {getGreeting()}</h2>
            <p className="overview-role">Viewing as <strong>{roleLabel}</strong></p>
            {loading && <p className="overview-role" style={{ marginTop: 4 }}>Refreshing live data...</p>}
            {error && <p className="overview-role" style={{ marginTop: 4, color: "#ef4444" }}>{error}</p>}
          </div>
          <button className="cmd-trigger" onClick={() => setShowSearch(true)}>
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
              <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
            </svg>
            Search...
            <kbd>⌘K</kbd>
          </button>
        </div>
      </div>

      <section className="dashboard-section" style={{ marginTop: 16 }}>
        <div className="section-header timeline-header-wrap">
          <h3 className="section-title">Task Timeline (Weekly)</h3>
          <div className="timeline-header-actions">
            <div className="timeline-nav">
              {activeWeekIndex !== currentWeekIndex && (
                <button
                  type="button"
                  className="timeline-nav-btn timeline-nav-current"
                  onClick={() => setSelectedWeekIndex(currentWeekIndex)}
                  title="Jump to current week"
                >
                  Current Week
                </button>
              )}
              <button
                type="button"
                className="timeline-nav-btn"
                disabled={activeWeekIndex <= 0}
                onClick={() => setSelectedWeekIndex((idx) => Math.max(0, idx - 1))}
              >
                Prev Week
              </button>
              <span className="timeline-nav-label">{formatWeekRange(weekStart)}</span>
              <button
                type="button"
                className="timeline-nav-btn"
                disabled={activeWeekIndex >= weekStarts.length - 1}
                onClick={() => setSelectedWeekIndex((idx) => Math.min(weekStarts.length - 1, idx + 1))}
              >
                Next Week
              </button>
            </div>
          </div>
        </div>

        <div className="timeline-legend" style={{ borderTop: "none", padding: "0 0 12px" }}>
          {(Object.entries(TASK_STATUS_CONFIG) as [string, { label: string; color: string }][]).slice(0, 5).map(
            ([key, cfg]) => (
              <div key={key} className="timeline-legend-item">
                <div className="timeline-legend-color" style={{ background: cfg.color }} />
                <span>{cfg.label}</span>
              </div>
            )
          )}
        </div>

        <div className="timeline-week-shell">
          <div className="timeline-container">
            <div className="timeline-header">
              <div className="timeline-label-col">Task</div>
              <div className="timeline-dates">
                {weekDays.map((date, idx) => {
                  const isWeekend = date.getDay() === 0 || date.getDay() === 6;
                  const isToday = startOfDay(date).getTime() === startOfDay(new Date()).getTime();
                  return (
                    <div
                      key={idx}
                      className={`timeline-date-col ${isWeekend ? "timeline-weekend" : ""} ${isToday ? "today" : ""}`}
                    >
                      <span className="timeline-date-label">{formatShortDate(date)}</span>
                    </div>
                  );
                })}
              </div>
            </div>

            <div className="timeline-body">
              {showTodayMarker && (
                <div className="timeline-today-marker" style={{ left: `calc(220px + (100% - 220px) * ${todayPct / 100})` }}>
                  <div className="timeline-today-label">Today</div>
                </div>
              )}

              {weekTasks.length === 0 && (
                <div className="timeline-empty">No tasks in this week.</div>
              )}

              {weekTasks.map((task) => {
                const created = parseISO(task.createdAt) || new Date();
                const dueRaw = parseISO(task.dueDate) || created;
                const finish = dueRaw > created ? dueRaw : new Date(created.getTime() + 60 * 60 * 1000);
                const isOverdueForTimeline =
                  (task.status === "pending" || task.status === "in_progress")
                  && dueRaw.getTime() < Date.now();
                const timelineStatus: TaskStatus = isOverdueForTimeline ? "overdue" : task.status;

                const clippedStart = clampDate(created, weekStart, weekEnd);
                const clippedEnd = clampDate(finish, weekStart, weekEnd);

                const leftPct = (daysBetween(weekStart, clippedStart) / 7) * 100;
                const widthPct = Math.max((daysBetween(clippedStart, clippedEnd) / 7) * 100, 2);

                const statusCfg = TASK_STATUS_CONFIG[timelineStatus];
                const priorityCfg = PRIORITY_CONFIG[task.priority];

                return (
                  <div key={task.id} className="timeline-row" onClick={() => handleSelectTask(task)}>
                    <div className="timeline-row-label">
                      <span className="timeline-row-title">{task.title}</span>
                      <span className="timeline-row-meta">
                        <span className="timeline-row-id">{task.id}</span>
                        <span className="timeline-row-priority" style={{ color: priorityCfg.color }}>
                          {priorityCfg.label}
                        </span>
                      </span>
                    </div>
                    <div className="timeline-row-track">
                      <div
                        className={`timeline-bar ${timelineStatus}`}
                        style={{
                          left: `${leftPct}%`,
                          width: `${widthPct}%`,
                          background: `linear-gradient(135deg, ${statusCfg.color}, ${statusCfg.color}dd)`,
                        }}
                        title={`${task.title} - ${formatShortDate(created)} to ${formatShortDate(finish)}`}
                      >
                        <span className="timeline-bar-label">
                          {task.title.length > 20 ? `${task.title.slice(0, 20)}...` : task.title}
                        </span>
                        <div
                          className="timeline-bar-progress"
                          style={{ width: `${(task.stepNumber / task.totalSteps) * 100}%` }}
                        />
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>

        <div className="timeline-summary">
          <div className="timeline-summary-item">
            <span className="timeline-summary-value">{weekTasks.length}</span>
            <span className="timeline-summary-label">Tasks In Week</span>
          </div>
          <div className="timeline-summary-item">
            <span className="timeline-summary-value">{weekTasks.filter((task) => atRiskTaskIDs.has(task.id)).length}</span>
            <span className="timeline-summary-label">At Risk</span>
          </div>
          <div className="timeline-summary-item">
            <span className="timeline-summary-value">W{activeWeekIndex + 1}</span>
            <span className="timeline-summary-label">Week Index</span>
          </div>
          <div className="timeline-summary-item">
            <span className="timeline-summary-value">{formatWeekRange(weekStart)}</span>
            <span className="timeline-summary-label">Visible Range</span>
          </div>
        </div>
      </section>

      <div className="cc-layout" style={{ marginTop: 20 }}>
        <div className="cc-main">
          <section className="dashboard-section">
            <div className="section-header">
              <h3 className="section-title">Recent Activity</h3>
            </div>
            <ActivityFeed items={activityItems} limit={8} />
          </section>

          <RoleGate allowed={["admin"]}>
            <section className="dashboard-section">
              <div className="section-header">
                <h3 className="section-title">Workflow Analysis</h3>
                <Link href="/dashboard/analytics" className="section-link">Open analytics page -&gt;</Link>
              </div>
              <div className="settings-empty-inline" style={{ marginTop: 6 }}>
                Analysis view is temporarily paused. Core live counters are still running.
              </div>
              <div className="timeline-summary" style={{ marginTop: 12 }}>
                <div className="timeline-summary-item">
                  <span className="timeline-summary-value">{analysisSummary.totalWorkflows}</span>
                  <span className="timeline-summary-label">Total Workflows</span>
                </div>
                <div className="timeline-summary-item">
                  <span className="timeline-summary-value">{analysisSummary.activeWorkflows}</span>
                  <span className="timeline-summary-label">Active Workflows</span>
                </div>
                <div className="timeline-summary-item">
                  <span className="timeline-summary-value">{analysisSummary.failedInstances}</span>
                  <span className="timeline-summary-label">Failed Instances</span>
                </div>
              </div>
            </section>
          </RoleGate>
        </div>

        <div className="cc-aside">
          {activeTask && (
            <div className="cc-active-detail" style={{ cursor: "pointer" }} onClick={() => handleSelectTask(activeTask)}>
              <p className="cc-detail-label">Most Urgent Task</p>
              <h3 className="cc-detail-title">{activeTask.title}</h3>
              <div className="cc-detail-meta">
                <div className="cc-detail-meta-item">
                  <span>Priority</span>
                  <span style={{ color: activeTask.priority === "critical" ? "var(--danger)" : activeTask.priority === "high" ? "var(--warning)" : "var(--text-primary)" }}>
                    {activeTask.priority.charAt(0).toUpperCase() + activeTask.priority.slice(1)}
                  </span>
                </div>
                <div className="cc-detail-meta-item">
                  <span>Workflow</span>
                  <span>{activeTask.workflowName}</span>
                </div>
                <div className="cc-detail-meta-item">
                  <span>Due</span>
                  <span>{new Date(activeTask.dueDate).toLocaleDateString()}</span>
                </div>
              </div>
              <div className="cc-progress-bar">
                <div className="cc-progress-fill" style={{ width: `${(activeTask.stepNumber / activeTask.totalSteps) * 100}%` }} />
              </div>
              <span style={{ fontSize: "0.72rem", color: "var(--text-muted)" }}>
                Step {activeTask.stepNumber} of {activeTask.totalSteps}
              </span>
            </div>
          )}

          <div className="cc-panel">
            <div className="cc-panel-header">
              <h4 className="cc-panel-title">Quick Actions</h4>
            </div>
            <div className="cc-panel-body">
              <div className="cc-quick-grid">
                <RoleGate allowed={["admin", "employee"]}>
                  <Link href="/dashboard/tasks" className="cc-quick-card">
                    <div className="cc-quick-icon" style={{ background: "var(--accent-subtle)", color: "var(--accent)" }}>
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="18" height="18">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15a2.25 2.25 0 0 1 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25Z" />
                      </svg>
                    </div>
                    View Tasks
                  </Link>
                </RoleGate>
                <RoleGate allowed={["admin"]}>
                  <Link href="/workflow-builder" className="cc-quick-card">
                    <div className="cc-quick-icon" style={{ background: "var(--success-subtle)", color: "var(--success)" }}>
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="18" height="18">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9.594 3.94c.09-.542.56-.94 1.11-.94h2.593c.55 0 1.02.398 1.11.94l.213 1.281c.063.374.313.686.645.87.074.04.147.083.22.127.325.196.72.257 1.075.124l1.217-.456a1.125 1.125 0 0 1 1.37.49l1.296 2.247a1.125 1.125 0 0 1-.26 1.431l-1.003.827c-.293.241-.438.613-.43.992a7.723 7.723 0 0 1 0 .255c-.008.378.137.75.43.991l1.004.827c.424.35.534.955.26 1.43l-1.298 2.247a1.125 1.125 0 0 1-1.369.491l-1.217-.456c-.355-.133-.75-.072-1.076.124a6.47 6.47 0 0 1-.22.128c-.331.183-.581.495-.644.869l-.213 1.281c-.09.543-.56.94-1.11.94h-2.594c-.55 0-1.019-.398-1.11-.94l-.213-1.281c-.062-.374-.312-.686-.644-.87a6.52 6.52 0 0 1-.22-.127c-.325-.196-.72-.257-1.076-.124l-1.217.456a1.125 1.125 0 0 1-1.369-.49l-1.297-2.247a1.125 1.125 0 0 1 .26-1.431l1.004-.827c.292-.24.437-.613.43-.991a6.932 6.932 0 0 1 0-.255c.007-.38-.138-.751-.43-.992l-1.004-.827a1.125 1.125 0 0 1-.26-1.43l1.297-2.247a1.125 1.125 0 0 1 1.37-.491l1.216.456c.356.133.751.072 1.076-.124.072-.044.146-.086.22-.128.332-.183.582-.495.644-.869l.214-1.28Z" />
                        <path strokeLinecap="round" strokeLinejoin="round" d="M15 12a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z" />
                      </svg>
                    </div>
                    Create Workflow
                  </Link>
                </RoleGate>
                <RoleGate allowed={["admin"]}>
                  <Link href="/dashboard/team" className="cc-quick-card">
                    <div className="cc-quick-icon" style={{ background: "var(--warning-subtle)", color: "var(--warning)" }}>
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="18" height="18">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />
                      </svg>
                    </div>
                    Team
                  </Link>
                </RoleGate>
              </div>
            </div>
          </div>
        </div>
      </div>

      <TaskDetailDrawer
        task={selectedTask}
        isOpen={selectedTask !== null}
        onClose={handleCloseDrawer}
      />
    </div>
  );
}

function getGreeting(): string {
  const h = new Date().getHours();
  if (h < 12) return "morning";
  if (h < 17) return "afternoon";
  return "evening";
}
