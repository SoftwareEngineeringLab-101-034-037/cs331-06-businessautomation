"use client";

import Link from "next/link";
import { useAuth, useOrganization } from "@clerk/nextjs";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { RoleGate } from "@/components/dashboard/RoleProvider";
import { authFetch as authFetchWithToken } from "@/lib/auth-fetch";
import { PRIORITY_CONFIG, TASK_STATUS_CONFIG } from "@/types/dashboard";
import type { TaskPriority, TaskStatus } from "@/types/dashboard";

const WF_API = process.env.NEXT_PUBLIC_WF_API || "http://localhost:8085";
const REFRESH_INTERVAL_MS = 15000;

type AnalyticsSummary = {
  generated_at: string;
  workflows_total: number;
  workflows_active: number;
  tasks_total: number;
  tasks_open: number;
  tasks_pending: number;
  tasks_in_progress: number;
  tasks_resolved: number;
  tasks_overdue: number;
  tasks_overdue_pending: number;
  tasks_overdue_in_progress: number;
  tasks_escalated: number;
  instances_total: number;
  instances_active: number;
  instances_completed: number;
  instances_failed: number;
  task_resolution_rate: number;
  instance_success_rate: number;
  avg_lead_hours: number;
  backlog_delta_24h: number;
};

type StatusSliceItem = {
  status: TaskStatus;
  label: string;
  count: number;
  pct: number;
};

type PrioritySliceItem = {
  priority: TaskPriority;
  count: number;
  pct: number;
};

type QueueAging = {
  lt_half_sla: number;
  lt_sla: number;
  gt_1_5_sla: number;
  gt_2_5_sla: number;
  overdue_open: number;
};

type ThroughputDay = {
  key: string;
  label: string;
  tasks_resolved: number;
  instances_started: number;
};

type WorkflowRollup = {
  workflow_id: string;
  workflow_name: string;
  total_tasks: number;
  open_tasks: number;
  closed_tasks: number;
  resolved_tasks: number;
  overdue_tasks: number;
  failed_instances: number;
  resolution_rate: number;
  avg_lead_hours: number;
};

type FailedInstance = {
  instance_id: string;
  workflow_id: string;
  workflow_name: string;
  status: string;
  started_at: string;
  node_id?: string;
  error?: string;
  failed_at?: string;
};

type AnalyticsResponse = {
  org_id: string;
  summary: AnalyticsSummary;
  status_distribution: StatusSliceItem[];
  priority_distribution_open: PrioritySliceItem[];
  queue_aging: QueueAging;
  throughput_7d: ThroughputDay[];
  workflow_rollups: WorkflowRollup[];
  failed_instances: FailedInstance[];
};

function toPercent(value: number, total: number): number {
  if (total <= 0) return 0;
  return Math.round((value / total) * 100);
}

function formatHours(hours: number): string {
  if (!Number.isFinite(hours) || hours <= 0) return "0h";
  if (hours < 24) return `${hours.toFixed(1)}h`;
  return `${(hours / 24).toFixed(1)}d`;
}

export default function AnalyticsPage() {
  const { getToken, userId } = useAuth();
  const { organization } = useOrganization();

  const [payload, setPayload] = useState<AnalyticsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const requestVersionRef = useRef(0);
  const hasLoadedOnceRef = useRef(false);

  const authFetch = useCallback(async (
    input: string,
    init: RequestInit = {},
    timeoutMs = 10000,
  ): Promise<Response> => {
    return authFetchWithToken(getToken, input, init, timeoutMs);
  }, [getToken]);

  const loadAnalyticsData = useCallback(async () => {
    const requestVersion = requestVersionRef.current + 1;
    requestVersionRef.current = requestVersion;

    const orgID = organization?.id;
    if (!orgID || !userId) {
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
      const params = new URLSearchParams();
      params.set("limit_problem_tasks", "120");
      params.set("limit_failed_instances", "80");

      const query = params.toString();
      const analyticsURL = `${WF_API}/api/orgs/${orgID}/analytics${query ? `?${query}` : ""}`;

      const analyticsRes = await authFetch(analyticsURL);
      if (!analyticsRes.ok) {
        throw new Error(`Failed to load analytics data (${analyticsRes.status})`);
      }

      const data = (await analyticsRes.json()) as AnalyticsResponse;

      if (requestVersion === requestVersionRef.current) {
        setPayload(data);
      }
    } catch (err: any) {
      if (requestVersion === requestVersionRef.current) {
        setError(err?.message || "Could not load analytics");
      }
    } finally {
      if (requestVersion === requestVersionRef.current) {
        setLoading(false);
        hasLoadedOnceRef.current = true;
      }
    }
  }, [authFetch, organization?.id, userId]);

  useEffect(() => {
    void loadAnalyticsData();
  }, [loadAnalyticsData]);

  useEffect(() => {
    const intervalID = window.setInterval(() => {
      void loadAnalyticsData();
    }, REFRESH_INTERVAL_MS);
    return () => {
      window.clearInterval(intervalID);
    };
  }, [loadAnalyticsData]);

  const summary = payload?.summary;
  const statusDist = payload?.status_distribution || [];
  const priorityDist = payload?.priority_distribution_open || [];
  const queueAging = payload?.queue_aging;
  const workflowRollups = payload?.workflow_rollups || [];
  const throughput7d = payload?.throughput_7d || [];

  const maxThroughput = useMemo(() => {
    return throughput7d.reduce((max, row) => Math.max(max, row.tasks_resolved, row.instances_started), 1);
  }, [throughput7d]);

  const statusCountByKey = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const row of statusDist) {
      counts[row.status] = row.count;
    }
    return counts;
  }, [statusDist]);

  return (
    <RoleGate
      allowed={["admin"]}
      fallback={
        <div className="dashboard-page">
          <div className="empty-state">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor" width="64" height="64">
              <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" />
            </svg>
            <h3>Access Restricted</h3>
            <p>Analytics is available to Admins only.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page observatory workstation-page">
        <div className="page-header" style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: 14, flexWrap: "wrap" }}>
          <div style={{ flex: "1 1 320px", minWidth: 260 }}>
            <h2 className="page-title">Data Observatory</h2>
            <p className="page-subtitle">Overall analytics across all workflows</p>
            {loading && <p className="table-muted" style={{ marginTop: 8 }}>Refreshing live analytics...</p>}
            {error && <p style={{ marginTop: 8, color: "#ef4444", fontSize: "0.85rem" }}>{error}</p>}
            {summary?.generated_at && (
              <p className="table-muted" style={{ marginTop: 8 }}>
                Generated {new Date(summary.generated_at).toLocaleString()}
              </p>
            )}
          </div>

          <div
            className="obs-metrics-row"
            style={{
              flex: "1 1 640px",
              minWidth: 0,
              margin: 0,
              gap: 6,
              display: "flex",
              flexWrap: "nowrap",
              justifyContent: "flex-end",
              overflowX: "auto",
              paddingBottom: 2,
            }}
          >
          <div className="obs-metric-card" style={{ padding: "6px 8px", minHeight: "unset", borderRadius: 9, gap: 0, display: "flex", flexDirection: "row", alignItems: "center", justifyContent: "space-between", flex: "0 0 156px", minWidth: 156 }}>
            <div style={{ display: "grid", gap: 1, minWidth: 0 }}>
              <span className="obs-metric-value" style={{ fontSize: "0.9rem", lineHeight: 1.05 }}>{summary?.tasks_total || 0}</span>
              <span className="obs-metric-label" style={{ fontSize: "0.54rem" }}>Total Tasks</span>
              <span className="obs-metric-sub" style={{ fontSize: "0.56rem", lineHeight: 1.1, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>open {summary?.tasks_open || 0} · resolved {summary?.tasks_resolved || 0}</span>
            </div>
            <div className="obs-metric-icon" style={{ width: 20, height: 20, borderRadius: 6, marginLeft: 6, background: "rgba(79,70,229,0.1)", color: "#6366f1" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="11" height="11">
                <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 0 1 0 3.75H5.625a1.875 1.875 0 0 1 0-3.75Z" />
              </svg>
            </div>
          </div>

          <div className="obs-metric-card" style={{ padding: "6px 8px", minHeight: "unset", borderRadius: 9, gap: 0, display: "flex", flexDirection: "row", alignItems: "center", justifyContent: "space-between", flex: "0 0 156px", minWidth: 156 }}>
            <div style={{ display: "grid", gap: 1, minWidth: 0 }}>
              <span className="obs-metric-value" style={{ fontSize: "0.9rem", lineHeight: 1.05 }}>{summary?.task_resolution_rate || 0}%</span>
              <span className="obs-metric-label" style={{ fontSize: "0.54rem" }}>Task Resolution</span>
              <span className="obs-metric-sub" style={{ fontSize: "0.56rem", lineHeight: 1.1, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>avg lead {formatHours(summary?.avg_lead_hours || 0)}</span>
            </div>
            <div className="obs-metric-icon" style={{ width: 20, height: 20, borderRadius: 6, marginLeft: 6, background: "rgba(34,197,94,0.1)", color: "#22c55e" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="11" height="11">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
            </div>
          </div>

          <div className="obs-metric-card" style={{ padding: "6px 8px", minHeight: "unset", borderRadius: 9, gap: 0, display: "flex", flexDirection: "row", alignItems: "center", justifyContent: "space-between", flex: "0 0 156px", minWidth: 156 }}>
            <div style={{ display: "grid", gap: 1, minWidth: 0 }}>
              <span className="obs-metric-value" style={{ fontSize: "0.9rem", lineHeight: 1.05 }}>{summary?.instance_success_rate || 0}%</span>
              <span className="obs-metric-label" style={{ fontSize: "0.54rem" }}>Instance Success</span>
              <span className="obs-metric-sub" style={{ fontSize: "0.56rem", lineHeight: 1.1, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{summary?.instances_completed || 0} completed · {summary?.instances_failed || 0} failed</span>
            </div>
            <div className="obs-metric-icon" style={{ width: 20, height: 20, borderRadius: 6, marginLeft: 6, background: "rgba(59,130,246,0.1)", color: "#3b82f6" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="11" height="11">
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18 9 11.25l4.306 4.306a11.95 11.95 0 0 1 5.814-5.518l2.74-1.22m0 0-5.94-2.281m5.94 2.28-2.28 5.941" />
              </svg>
            </div>
          </div>

          <div className="obs-metric-card" style={{ padding: "6px 8px", minHeight: "unset", borderRadius: 9, gap: 0, display: "flex", flexDirection: "row", alignItems: "center", justifyContent: "space-between", flex: "0 0 156px", minWidth: 156 }}>
            <div style={{ display: "grid", gap: 1, minWidth: 0 }}>
              <span className="obs-metric-value" style={{ fontSize: "0.9rem", lineHeight: 1.05 }}>{summary?.tasks_overdue || 0}</span>
              <span className="obs-metric-label" style={{ fontSize: "0.54rem" }}>SLA Risk</span>
              <span className="obs-metric-sub" style={{ fontSize: "0.56rem", lineHeight: 1.1, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>active wf {summary?.workflows_active || 0} · live inst {summary?.instances_active || 0}</span>
            </div>
            <div className="obs-metric-icon" style={{ width: 20, height: 20, borderRadius: 6, marginLeft: 6, background: "rgba(249,115,22,0.1)", color: "#f97316" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="11" height="11">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
              </svg>
            </div>
          </div>
        </div>
        </div>

        <section className="obs-distribution-section">
          <div className="section-header" style={{ marginBottom: 10 }}>
            <h3 className="section-title">Workflow Analytics Overview</h3>
          </div>
          <div className="table-container" style={{ maxHeight: `${8 * 48 + 72}px`, overflowY: "auto" }}>
            <table className="data-table">
              <thead>
                <tr>
                  <th>Workflow</th>
                  <th>Resolution</th>
                  <th>Total</th>
                  <th>Open</th>
                  <th>Closed</th>
                  <th>Overdue</th>
                  <th>Failed Inst</th>
                  <th>Avg Lead</th>
                </tr>
              </thead>
              <tbody>
                {workflowRollups.length === 0 ? (
                  <tr>
                    <td colSpan={8} className="table-muted">No workflow analytics rows available.</td>
                  </tr>
                ) : (
                  workflowRollups.map((row) => (
                    <tr key={row.workflow_id}>
                      <td className="font-medium">
                        <Link href={`/dashboard/analytics/${encodeURIComponent(row.workflow_id)}`} style={{ color: "var(--accent)", textDecoration: "underline" }}>
                          {row.workflow_name}
                        </Link>
                      </td>
                      <td>
                        <div style={{ display: "grid", gap: 4 }}>
                          <span style={{ color: row.resolution_rate >= 85 ? "#22c55e" : row.resolution_rate >= 65 ? "#f59e0b" : "#ef4444", fontWeight: 700 }}>
                            {row.resolution_rate}%
                          </span>
                          <div className="obs-rate-bar" style={{ marginTop: 0 }}>
                            <div
                              className="obs-rate-bar-fill"
                              style={{
                                width: `${row.resolution_rate}%`,
                                background: row.resolution_rate >= 85
                                  ? "linear-gradient(90deg, #22c55e, #16a34a)"
                                  : row.resolution_rate >= 65
                                    ? "linear-gradient(90deg, #f59e0b, #d97706)"
                                    : "linear-gradient(90deg, #ef4444, #dc2626)",
                              }}
                            />
                          </div>
                        </div>
                      </td>
                      <td>{row.total_tasks}</td>
                      <td>{row.open_tasks}</td>
                      <td>{row.closed_tasks}</td>
                      <td style={{ color: row.overdue_tasks > 0 ? "#ef4444" : "var(--text-primary)", fontWeight: 700 }}>{row.overdue_tasks}</td>
                      <td>{row.failed_instances}</td>
                      <td>{formatHours(row.avg_lead_hours)}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </section>

        <section className="obs-distribution-section">
          <h3 className="section-title">Task Distribution by Status (Live)</h3>
          <div className="obs-distribution">
            {[
              { key: "pending", label: "Pending", color: TASK_STATUS_CONFIG.pending.color, count: statusCountByKey.pending || 0 },
              { key: "in_progress", label: "In Progress", color: TASK_STATUS_CONFIG.in_progress.color, count: statusCountByKey.in_progress || 0 },
              { key: "overdue", label: "Overdue", color: "#ef4444", count: summary?.tasks_overdue || 0 },
              { key: "completed", label: "Completed", color: TASK_STATUS_CONFIG.completed.color, count: statusCountByKey.completed || 0 },
              { key: "escalated", label: "Escalated", color: TASK_STATUS_CONFIG.escalated.color, count: statusCountByKey.escalated || 0 },
            ].map((row) => {
              const pct = toPercent(row.count, summary?.tasks_total || 1);
              return (
                <div key={row.key} style={{ display: "grid", gap: row.key === "overdue" ? 6 : 0 }}>
                  <div className="obs-dist-row">
                  <div className="obs-dist-color" style={{ background: row.color }} />
                  <span className="obs-dist-label">{row.label}</span>
                  <div className="obs-dist-bar">
                    <div className="obs-dist-bar-fill" style={{ width: `${pct}%`, background: row.color }} />
                  </div>
                  <span className="obs-dist-count">{row.count}</span>
                  <span className="obs-dist-pct">{pct}%</span>
                  </div>
                  {row.key === "overdue" && (
                    <>
                      <div className="obs-dist-row" style={{ marginLeft: 20, gridTemplateColumns: "8px 170px 1fr 72px 52px", opacity: 0.9 }}>
                        <div className="obs-dist-color" style={{ background: TASK_STATUS_CONFIG.pending.color }} />
                        <span className="obs-dist-label">pending overdue</span>
                        <div className="obs-dist-bar">
                          <div
                            className="obs-dist-bar-fill"
                            style={{
                              width: `${toPercent(summary?.tasks_overdue_pending || 0, summary?.tasks_overdue || 1)}%`,
                              background: TASK_STATUS_CONFIG.pending.color,
                            }}
                          />
                        </div>
                        <span className="obs-dist-count">{summary?.tasks_overdue_pending || 0}</span>
                        <span className="obs-dist-pct">{toPercent(summary?.tasks_overdue_pending || 0, summary?.tasks_overdue || 1)}%</span>
                      </div>
                      <div className="obs-dist-row" style={{ marginLeft: 20, gridTemplateColumns: "8px 170px 1fr 72px 52px", opacity: 0.9 }}>
                        <div className="obs-dist-color" style={{ background: TASK_STATUS_CONFIG.in_progress.color }} />
                        <span className="obs-dist-label">in progress overdue</span>
                        <div className="obs-dist-bar">
                          <div
                            className="obs-dist-bar-fill"
                            style={{
                              width: `${toPercent(summary?.tasks_overdue_in_progress || 0, summary?.tasks_overdue || 1)}%`,
                              background: TASK_STATUS_CONFIG.in_progress.color,
                            }}
                          />
                        </div>
                        <span className="obs-dist-count">{summary?.tasks_overdue_in_progress || 0}</span>
                        <span className="obs-dist-pct">{toPercent(summary?.tasks_overdue_in_progress || 0, summary?.tasks_overdue || 1)}%</span>
                      </div>
                    </>
                  )}
                </div>
              );
            })}
          </div>
        </section>

        <div className="obs-chart-grid">
          <section className="obs-chart-panel">
            <div className="obs-chart-panel-header">
              <h4>Queue Aging by SLA Ratio (Open Tasks)</h4>
              <span className="obs-metric-sub">overdue open: {queueAging?.overdue_open || 0}</span>
            </div>
            <div className="obs-distribution">
              {[
                { key: "lt-half", label: "< 0.5 SLA", value: queueAging?.lt_half_sla || 0, color: "#22c55e" },
                { key: "lt-sla", label: "< SLA", value: queueAging?.lt_sla || 0, color: "#84cc16" },
                { key: "gt-1-5", label: "> 1.5 SLA", value: queueAging?.gt_1_5_sla || 0, color: "#f59e0b" },
                { key: "gt-2-5", label: "> 2.5 SLA", value: queueAging?.gt_2_5_sla || 0, color: "#ef4444" },
              ].map((bucket) => {
                const pct = toPercent(bucket.value, summary?.tasks_open || 1);
                return (
                  <div key={bucket.key} className="obs-dist-row">
                    <div className="obs-dist-color" style={{ background: bucket.color }} />
                    <span className="obs-dist-label">{bucket.label}</span>
                    <div className="obs-dist-bar">
                      <div className="obs-dist-bar-fill" style={{ width: `${pct}%`, background: bucket.color }} />
                    </div>
                    <span className="obs-dist-count">{bucket.value}</span>
                    <span className="obs-dist-pct">{pct}%</span>
                  </div>
                );
              })}
            </div>
          </section>

          <section className="obs-chart-panel">
            <div className="obs-chart-panel-header">
              <h4>Open Queue by Priority</h4>
            </div>
            <div className="obs-distribution">
              {(Object.keys(PRIORITY_CONFIG) as TaskPriority[]).map((priority) => {
                const matched = priorityDist.find((entry) => entry.priority === priority);
                const count = matched?.count || 0;
                const cfg = PRIORITY_CONFIG[priority];
                const pct = matched?.pct ?? toPercent(count, summary?.tasks_open || 1);
                return (
                  <div key={priority} className="obs-dist-row">
                    <div className="obs-dist-color" style={{ background: cfg.color }} />
                    <span className="obs-dist-label">{cfg.label}</span>
                    <div className="obs-dist-bar">
                      <div className="obs-dist-bar-fill" style={{ width: `${pct}%`, background: cfg.color }} />
                    </div>
                    <span className="obs-dist-count">{count}</span>
                    <span className="obs-dist-pct">{pct}%</span>
                  </div>
                );
              })}
            </div>
          </section>
        </div>

        <div className="obs-chart-grid">
          <section className="obs-chart-panel">
            <div className="obs-chart-panel-header">
              <h4>Throughput Trend (Last 7 Days)</h4>
              <span className="obs-metric-sub">tasks resolved vs instances started</span>
            </div>
            <div className="obs-distribution">
              {throughput7d.map((row) => (
                <div key={row.key} style={{ display: "grid", gridTemplateColumns: "72px 1fr 44px 44px", gap: 10, alignItems: "center" }}>
                  <span className="obs-dist-label">{row.label}</span>
                  <div style={{ display: "grid", gap: 4 }}>
                    <div className="obs-dist-bar">
                      <div className="obs-dist-bar-fill" style={{ width: `${toPercent(row.tasks_resolved, maxThroughput)}%`, background: "#22c55e" }} />
                    </div>
                    <div className="obs-dist-bar">
                      <div className="obs-dist-bar-fill" style={{ width: `${toPercent(row.instances_started, maxThroughput)}%`, background: "#3b82f6" }} />
                    </div>
                  </div>
                  <span className="obs-dist-count" title="tasks resolved">{row.tasks_resolved}</span>
                  <span className="obs-dist-count" title="instances started" style={{ color: "#3b82f6" }}>{row.instances_started}</span>
                </div>
              ))}
            </div>
          </section>
        </div>

      </div>
    </RoleGate>
  );
}
