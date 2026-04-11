"use client";

import Link from "next/link";
import { useAuth, useOrganization } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams } from "next/navigation";
import { RoleGate } from "@/components/dashboard/RoleProvider";
import { authFetch as authFetchWithToken } from "@/lib/auth-fetch";
import TaskDetailDrawer from "@/components/dashboard/TaskDetailDrawer";
import { PRIORITY_CONFIG, TASK_STATUS_CONFIG } from "@/types/dashboard";
import type { Task, TaskPriority, TaskStatus } from "@/types/dashboard";

const WF_API = process.env.NEXT_PUBLIC_WF_API || "http://localhost:8085";
const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";
const REFRESH_INTERVAL_MS = 15000;

type AnalyticsSummary = {
  generated_at: string;
  tasks_total: number;
  tasks_open: number;
  tasks_overdue: number;
  instances_total: number;
  instances_failed: number;
  avg_lead_hours: number;
};

type TaskTypeRollup = {
  task_type: string;
  node_id: string;
  waiting_count: number;
  failed_count: number;
  total_count: number;
  action_counts: Record<string, number>;
};

type ProblemTask = {
  task_id: string;
  title?: string;
  workflow_id: string;
  workflow_name: string;
  status: TaskStatus;
  display_status: TaskStatus;
  priority: TaskPriority;
  created_at: string;
  due_at: string;
  instance_id: string;
  node_id: string;
  assigned_user?: string;
  assigned_role?: string;
  age_hours: number;
  is_overdue: boolean;
};

type BackendEmployee = {
  id: string;
  first_name: string;
  last_name: string;
  email: string;
  is_admin: boolean;
  is_active: boolean;
  job_title?: string;
  department?: { name?: string };
};

type FailedInstance = {
  instance_id: string;
  workflow_id: string;
  workflow_name: string;
  status: string;
  node_id?: string;
  error?: string;
  failed_at?: string;
};

type WorkflowRollup = {
  workflow_id: string;
  workflow_name: string;
};

type AnalyticsResponse = {
  summary: AnalyticsSummary;
  workflow_rollups: WorkflowRollup[];
  task_type_rollups: TaskTypeRollup[];
  problem_tasks: ProblemTask[];
  failed_instances: FailedInstance[];
};

type SideSection = "task-types" | "overdue-tasks" | "instance-health";

function formatHours(hours: number): string {
  if (!Number.isFinite(hours) || hours <= 0) return "0h";
  if (hours < 24) return `${hours.toFixed(1)}h`;
  return `${(hours / 24).toFixed(1)}d`;
}

function formatActionCounts(actionCounts: Record<string, number>): Array<[string, number]> {
  return Object.entries(actionCounts || {}).sort((a, b) => b[1] - a[1]);
}

export default function WorkflowAnalyticsDetailPage() {
  const params = useParams<{ workflowId: string }>();
  const { getToken, userId } = useAuth();
  const { organization } = useOrganization();

  const rawWorkflowId = params?.workflowId;
  const workflowID = useMemo(() => {
    if (!rawWorkflowId) return "";
    return decodeURIComponent(Array.isArray(rawWorkflowId) ? rawWorkflowId[0] : rawWorkflowId);
  }, [rawWorkflowId]);

  const [payload, setPayload] = useState<AnalyticsResponse | null>(null);
  const [employees, setEmployees] = useState<BackendEmployee[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeSection, setActiveSection] = useState<SideSection>("task-types");
  const [selectedEmployeeID, setSelectedEmployeeID] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);

  const requestVersionRef = useRef(0);
  const hasLoadedOnceRef = useRef(false);

  const authFetch = useCallback(async (
    input: string,
    init: RequestInit = {},
    timeoutMs = 10000,
  ): Promise<Response> => {
    return authFetchWithToken(getToken, input, init, timeoutMs);
  }, [getToken]);

  const loadWorkflowAnalytics = useCallback(async () => {
    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;

    const orgID = organization?.id;
    if (!orgID || !userId || !workflowID) {
      if (requestVersion === requestVersionRef.current) {
        setPayload(null);
        setError(null);
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
      const analyticsRes = await authFetch(
        `${WF_API}/api/orgs/${orgID}/analytics?workflow_id=${encodeURIComponent(workflowID)}&limit_problem_tasks=500&limit_failed_instances=200`,
      );
      if (!analyticsRes.ok) {
        throw new Error(`Failed to load workflow analytics (${analyticsRes.status})`);
      }

      const data = (await analyticsRes.json()) as AnalyticsResponse;

      // Employee lookup powers the team drawer and assignee labels in overdue table.
      let employeeData: BackendEmployee[] = [];
      try {
        const employeeRes = await authFetch(`${AUTH_API}/api/orgs/${orgID}/employees`);
        if (employeeRes.ok) {
          const parsed = (await employeeRes.json()) as BackendEmployee[];
          employeeData = Array.isArray(parsed) ? parsed : [];
        }
      } catch {
        employeeData = [];
      }

      if (requestVersion === requestVersionRef.current) {
        setPayload(data);
        setEmployees(employeeData);
      }
    } catch (err: any) {
      if (requestVersion === requestVersionRef.current) {
        setError(err?.message || "Could not load workflow analytics");
      }
    } finally {
      if (requestVersion === requestVersionRef.current) {
        setLoading(false);
        hasLoadedOnceRef.current = true;
      }
    }
  }, [authFetch, organization?.id, userId, workflowID]);

  useEffect(() => {
    void loadWorkflowAnalytics();
  }, [loadWorkflowAnalytics]);

  useEffect(() => {
    const intervalID = window.setInterval(() => {
      void loadWorkflowAnalytics();
    }, REFRESH_INTERVAL_MS);
    return () => {
      window.clearInterval(intervalID);
    };
  }, [loadWorkflowAnalytics]);

  const summary = payload?.summary;
  const taskTypeRollups = payload?.task_type_rollups || [];
  const overdueTasks = useMemo(() => {
    return (payload?.problem_tasks || []).filter((task) => task.display_status === "overdue" || task.display_status === "escalated");
  }, [payload?.problem_tasks]);
  const failedInstances = payload?.failed_instances || [];
  const employeeByID = useMemo(() => {
    const map = new Map<string, BackendEmployee>();
    for (const employee of employees) {
      map.set(employee.id, employee);
    }
    return map;
  }, [employees]);

  const selectedEmployee = useMemo(() => {
    if (!selectedEmployeeID) return null;
    return employeeByID.get(selectedEmployeeID) || null;
  }, [employeeByID, selectedEmployeeID]);

  const workflowName =
    payload?.workflow_rollups?.[0]?.workflow_name
    || payload?.problem_tasks?.[0]?.workflow_name
    || payload?.failed_instances?.[0]?.workflow_name
    || workflowID;

  const goToSection = useCallback((section: SideSection) => {
    setActiveSection(section);
  }, []);

  const employeeDisplayName = useCallback((employeeID?: string) => {
    if (!employeeID) return "-";
    const employee = employeeByID.get(employeeID);
    if (!employee) return employeeID;
    const fullName = `${employee.first_name || ""} ${employee.last_name || ""}`.trim();
    return fullName || employee.email || employee.id;
  }, [employeeByID]);

  const toDrawerTask = useCallback((task: ProblemTask): Task => {
    return {
      id: task.task_id,
      title: task.title || task.task_id,
      description: "Workflow task from deep-dive overdue analytics.",
      status: task.display_status,
      priority: task.priority,
      assignedTo: task.assigned_user || "",
      assignedToName: task.assigned_user ? employeeDisplayName(task.assigned_user) : (task.assigned_role || "Role Queue"),
      assignedBy: "workflow-engine",
      assignedByName: "Workflow Engine",
      workflowId: task.workflow_id,
      workflowName: task.workflow_name,
      departmentOrigin: "Operations",
      createdAt: task.created_at,
      dueDate: task.due_at,
      tags: [task.assigned_role || "workflow"],
      stepNumber: 1,
      totalSteps: 1,
      nodeId: task.node_id,
      instanceId: task.instance_id,
    };
  }, [employeeDisplayName]);

  const closeTaskDrawer = useCallback(() => setSelectedTask(null), []);
  const closeEmployeeDrawer = useCallback(() => setSelectedEmployeeID(null), []);

  return (
    <RoleGate
      allowed={["admin"]}
      fallback={
        <div className="dashboard-page">
          <div className="empty-state">
            <h3>Access Restricted</h3>
            <p>Workflow analytics deep dive is available to Admins only.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page workstation-page">
        <div className="page-header">
          <div>
            <h2 className="page-title">Workflow Analytics Deep Dive</h2>
            <p className="page-subtitle">{workflowName}</p>
            <p className="table-muted" style={{ marginTop: 8 }}>
              <Link href="/dashboard/analytics" style={{ color: "var(--accent)", textDecoration: "underline" }}>Back to Analytics Overview</Link>
            </p>
            {loading && <p className="table-muted" style={{ marginTop: 8 }}>Refreshing workflow analytics...</p>}
            {error && <p style={{ marginTop: 8, color: "#ef4444", fontSize: "0.85rem" }}>{error}</p>}
            {summary?.generated_at && (
              <p className="table-muted" style={{ marginTop: 8 }}>
                Generated {new Date(summary.generated_at).toLocaleString()}
              </p>
            )}
          </div>
        </div>

        <div className="obs-metrics-row" style={{ marginBottom: 18 }}>
          <div className="obs-metric-card">
            <span className="obs-metric-value">{summary?.tasks_total || 0}</span>
            <span className="obs-metric-label">Total Tasks</span>
            <span className="obs-metric-sub">open {summary?.tasks_open || 0}</span>
          </div>
          <div className="obs-metric-card">
            <span className="obs-metric-value">{summary?.tasks_overdue || 0}</span>
            <span className="obs-metric-label">Overdue Tasks</span>
            <span className="obs-metric-sub">need attention now</span>
          </div>
          <div className="obs-metric-card">
            <span className="obs-metric-value">{summary?.instances_failed || 0}</span>
            <span className="obs-metric-label">Failed Instances</span>
            <span className="obs-metric-sub">error-tracked</span>
          </div>
          <div className="obs-metric-card">
            <span className="obs-metric-value">{formatHours(summary?.avg_lead_hours || 0)}</span>
            <span className="obs-metric-label">Average Lead Time</span>
            <span className="obs-metric-sub">workflow-specific</span>
          </div>
        </div>

        <div className="settings-workspace">
          <aside className="settings-sidebar-shell">
            <p className="settings-sidebar-title">Deep Research Sections</p>
            <div className="settings-sidebar-list">
              <button type="button" className={`settings-sidebar-item ${activeSection === "task-types" ? "active" : ""}`} onClick={() => goToSection("task-types")}>
                <strong>Task Types</strong>
                <span>Waiting, failed and action counts</span>
              </button>
              <button type="button" className={`settings-sidebar-item ${activeSection === "overdue-tasks" ? "active" : ""}`} onClick={() => goToSection("overdue-tasks")}>
                <strong>Overdue Tasks</strong>
                <span>All overdue and escalated items</span>
              </button>
              <button type="button" className={`settings-sidebar-item ${activeSection === "instance-health" ? "active" : ""}`} onClick={() => goToSection("instance-health")}>
                <strong>Instance Health</strong>
                <span>Failed instances and error messages</span>
              </button>
            </div>
          </aside>

          <div className="settings-content-shell" style={{ display: "grid", gap: 18 }}>
            {activeSection === "task-types" && (
            <section id="task-types" className="dashboard-section">
              <div className="section-header">
                <h3 className="section-title">Task Type Breakdown</h3>
              </div>
              <div className="table-container" style={{ maxHeight: `${10 * 48 + 72}px`, overflowY: "auto" }}>
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Task Type</th>
                      <th>Node</th>
                      <th>Waiting</th>
                      <th>Failed</th>
                      <th>Total</th>
                      <th>Action Counts</th>
                    </tr>
                  </thead>
                  <tbody>
                    {taskTypeRollups.length === 0 && (
                      <tr>
                        <td colSpan={6} className="table-muted">No task-type records found for this workflow.</td>
                      </tr>
                    )}
                    {taskTypeRollups.map((row) => {
                      const actions = formatActionCounts(row.action_counts);
                      return (
                        <tr key={`${row.node_id}-${row.task_type}`}>
                          <td className="font-medium">{row.task_type}</td>
                          <td>{row.node_id || "-"}</td>
                          <td>{row.waiting_count}</td>
                          <td style={{ color: row.failed_count > 0 ? "#ef4444" : "var(--text-primary)", fontWeight: 700 }}>{row.failed_count}</td>
                          <td>{row.total_count}</td>
                          <td>
                            {actions.length === 0 ? (
                              <span className="table-muted">-</span>
                            ) : (
                              <div style={{ display: "flex", gap: 6, flexWrap: "wrap" }}>
                                {actions.map(([action, count]) => (
                                  <span key={action} className="role-badge">{action}: {count}</span>
                                ))}
                              </div>
                            )}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </section>
            )}

            {activeSection === "overdue-tasks" && (
            <section id="overdue-tasks" className="dashboard-section">
              <div className="section-header">
                <h3 className="section-title">Overdue Tasks</h3>
              </div>
              <div className="table-container" style={{ maxHeight: `${10 * 48 + 72}px`, overflowY: "auto" }}>
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Task</th>
                      <th>Assigned Employee</th>
                      <th>Node</th>
                      <th>Status</th>
                      <th>Priority</th>
                      <th>Age</th>
                      <th>Due</th>
                      <th>Instance</th>
                    </tr>
                  </thead>
                  <tbody>
                    {overdueTasks.length === 0 && (
                      <tr>
                        <td colSpan={8} className="table-muted">No overdue tasks in this workflow scope.</td>
                      </tr>
                    )}
                    {overdueTasks.map((task) => (
                      <tr
                        key={task.task_id}
                        onClick={() => setSelectedTask(toDrawerTask(task))}
                        style={{ cursor: "pointer" }}
                        title="Open task drawer"
                      >
                        <td className="font-medium">{task.title || task.task_id}</td>
                        <td>
                          {task.assigned_user ? (
                            <button
                              type="button"
                              className="role-badge"
                              onClick={(event) => {
                                event.stopPropagation();
                                setSelectedEmployeeID(task.assigned_user || null);
                              }}
                              title="Open team drawer"
                              style={{ cursor: "pointer" }}
                            >
                              {employeeDisplayName(task.assigned_user)}
                            </button>
                          ) : (
                            <span className="table-muted">{task.assigned_role || "-"}</span>
                          )}
                        </td>
                        <td>{task.node_id}</td>
                        <td>
                          <span className="status-badge" style={{ background: TASK_STATUS_CONFIG[task.display_status].bg, color: TASK_STATUS_CONFIG[task.display_status].color }}>
                            {TASK_STATUS_CONFIG[task.display_status].label}
                          </span>
                        </td>
                        <td style={{ color: PRIORITY_CONFIG[task.priority].color, fontWeight: 700 }}>{PRIORITY_CONFIG[task.priority].label}</td>
                        <td>{formatHours(task.age_hours)}</td>
                        <td>{new Date(task.due_at).toLocaleString()}</td>
                        <td>{task.instance_id}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
            )}

            {activeSection === "instance-health" && (
            <section id="instance-health" className="dashboard-section">
              <div className="section-header">
                <h3 className="section-title">Instance Health</h3>
              </div>
              <div className="table-container" style={{ maxHeight: `${10 * 48 + 72}px`, overflowY: "auto" }}>
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Instance</th>
                      <th>Status</th>
                      <th>Failed At</th>
                      <th>Failure Point</th>
                      <th>Error Message</th>
                    </tr>
                  </thead>
                  <tbody>
                    {failedInstances.length === 0 && (
                      <tr>
                        <td colSpan={5} className="table-muted">No failed instances for this workflow in current scope.</td>
                      </tr>
                    )}
                    {failedInstances.map((instance) => (
                      <tr key={instance.instance_id}>
                        <td className="font-medium">{instance.instance_id}</td>
                        <td>{instance.status}</td>
                        <td>{instance.failed_at ? new Date(instance.failed_at).toLocaleString() : "-"}</td>
                        <td>{instance.node_id || "-"}</td>
                        <td>{instance.error || "unknown failure"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
            )}
          </div>
        </div>
      </div>

      {selectedTask && (
        <TaskDetailDrawer
          task={selectedTask}
          isOpen={!!selectedTask}
          onClose={closeTaskDrawer}
        />
      )}

      {selectedEmployee && (
        <div className="drawer-overlay" onClick={closeEmployeeDrawer}>
          <aside className="drawer-panel employee-drawer-panel" onClick={(event) => event.stopPropagation()}>
            <div className="drawer-header employee-drawer-header">
              <div className="employee-drawer-identity">
                <div className="employee-drawer-avatar">
                  {`${selectedEmployee.first_name?.[0] || ""}${selectedEmployee.last_name?.[0] || ""}`.toUpperCase() || "?"}
                </div>
                <div className="employee-drawer-meta">
                  <div className="employee-drawer-title-row">
                    <h3 className="drawer-task-title">{`${selectedEmployee.first_name || ""} ${selectedEmployee.last_name || ""}`.trim() || selectedEmployee.email}</h3>
                    <span className={`status-dot ${selectedEmployee.is_active ? "active" : "inactive"}`}>
                      {selectedEmployee.is_active ? "Active" : "Inactive"}
                    </span>
                  </div>
                  <p className="employee-drawer-subtitle">{selectedEmployee.email}</p>
                  <div className="employee-drawer-badges">
                    <span className="employee-id-pill">ID {selectedEmployee.id}</span>
                    <span className="role-badge">Dashboard {selectedEmployee.is_admin ? "Admin" : "Employee"}</span>
                  </div>
                </div>
              </div>
              <button className="drawer-close-btn" onClick={closeEmployeeDrawer} aria-label="Close drawer">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="18" height="18">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                </svg>
              </button>
            </div>
            <div className="drawer-body" style={{ display: "grid", gap: 14 }}>
              <section className="detail-section">
                <h3 className="detail-section-title">Employee Details</h3>
                <div className="drawer-info-grid">
                  <div className="detail-info-item">
                    <dt>Name</dt>
                    <dd>{`${selectedEmployee.first_name || ""} ${selectedEmployee.last_name || ""}`.trim() || "-"}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Email</dt>
                    <dd>{selectedEmployee.email}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Department</dt>
                    <dd>{selectedEmployee.department?.name || "-"}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Job Title</dt>
                    <dd>{selectedEmployee.job_title || "-"}</dd>
                  </div>
                </div>
              </section>
            </div>
          </aside>
        </div>
      )}
    </RoleGate>
  );
}
