"use client";

import { RoleGate } from "@/components/dashboard/RoleProvider";
import { MOCK_METRICS, MOCK_TASKS } from "@/lib/mock-data";
import { TASK_STATUS_CONFIG } from "@/types/dashboard";
import type { TaskStatus } from "@/types/dashboard";

export default function AnalyticsPage() {
  const totalTasks = MOCK_TASKS.length;
  const completed = MOCK_TASKS.filter((t) => t.status === "completed").length;
  const avgSuccessRate = Math.round(MOCK_METRICS.reduce((a, m) => a + m.successRate, 0) / MOCK_METRICS.length);
  const bottlenecks = MOCK_METRICS.filter((m) => m.bottleneckStep).length;
  const completionRate = Math.round((completed / totalTasks) * 100);

  // Task distribution
  const statusDist = MOCK_TASKS.reduce<Record<string, number>>((acc, t) => {
    acc[t.status] = (acc[t.status] || 0) + 1;
    return acc;
  }, {});

  return (
    <RoleGate
      allowed={["org_admin", "admin", "analyst"]}
      fallback={
        <div className="dashboard-page">
          <div className="empty-state">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor" width="64" height="64">
              <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" />
            </svg>
            <h3>Access Restricted</h3>
            <p>Analytics is available to Admins, Org Admins, and Analysts only.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page observatory">
        <div className="page-header">
          <div>
            <h2 className="page-title">Data Observatory</h2>
            <p className="page-subtitle">Workflow performance, bottleneck analysis, and operational metrics</p>
          </div>
        </div>

        {/* Observatory — Top KPI Metric Row */}
        <div className="obs-metrics-row">
          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(79,70,229,0.1)", color: "#6366f1" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 12h16.5m-16.5 3.75h16.5M3.75 19.5h16.5M5.625 4.5h12.75a1.875 1.875 0 0 1 0 3.75H5.625a1.875 1.875 0 0 1 0-3.75Z" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{totalTasks}</span>
              <span className="obs-metric-label">Total Tasks</span>
            </div>
          </div>

          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(34,197,94,0.1)", color: "#22c55e" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{completionRate}%</span>
              <span className="obs-metric-label">Completion Rate</span>
            </div>
            <span className="obs-metric-sub">{completed}/{totalTasks} tasks</span>
          </div>

          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(59,130,246,0.1)", color: "#3b82f6" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 18 9 11.25l4.306 4.306a11.95 11.95 0 0 1 5.814-5.518l2.74-1.22m0 0-5.94-2.281m5.94 2.28-2.28 5.941" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{avgSuccessRate}%</span>
              <span className="obs-metric-label">Avg Success Rate</span>
            </div>
            <span className="obs-metric-sub obs-metric-trend-up">↑ 3% from last month</span>
          </div>

          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(249,115,22,0.1)", color: "#f97316" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{bottlenecks}</span>
              <span className="obs-metric-label">Bottlenecks</span>
            </div>
            <span className="obs-metric-sub">across {MOCK_METRICS.length} workflows</span>
          </div>
        </div>

        {/* Observatory — Chart Grid (Workflow Performance) */}
        <div className="obs-chart-grid">
          {MOCK_METRICS.map((m) => (
            <div key={m.workflowName} className="obs-chart-panel">
              <div className="obs-chart-panel-header">
                <h4>{m.workflowName}</h4>
                {m.bottleneckStep && (
                  <span className="bottleneck-badge">⚠ {m.bottleneckStep}</span>
                )}
              </div>

              {/* Success rate bar */}
              <div className="obs-rate-bar-container">
                <div className="obs-rate-bar-header">
                  <span>Success Rate</span>
                  <span style={{ color: m.successRate >= 90 ? "#22c55e" : m.successRate >= 80 ? "#f59e0b" : "#ef4444", fontWeight: 700 }}>
                    {m.successRate}%
                  </span>
                </div>
                <div className="obs-rate-bar">
                  <div
                    className="obs-rate-bar-fill"
                    style={{
                      width: `${m.successRate}%`,
                      background: m.successRate >= 90
                        ? "linear-gradient(90deg, #22c55e, #16a34a)"
                        : m.successRate >= 80
                        ? "linear-gradient(90deg, #f59e0b, #d97706)"
                        : "linear-gradient(90deg, #ef4444, #dc2626)",
                    }}
                  />
                </div>
              </div>

              {/* Breakdown row */}
              <div className="obs-breakdown">
                <div className="obs-breakdown-item">
                  <span className="obs-breakdown-value">{m.totalRuns}</span>
                  <span className="obs-breakdown-label">Total Runs</span>
                </div>
                <div className="obs-breakdown-item">
                  <span className="obs-breakdown-value">{m.avgCompletionTime}</span>
                  <span className="obs-breakdown-label">Avg Time</span>
                </div>
                <div className="obs-breakdown-item">
                  <span className="obs-breakdown-value">{Math.round(m.totalRuns * m.successRate / 100)}</span>
                  <span className="obs-breakdown-label">Successful</span>
                </div>
                <div className="obs-breakdown-item">
                  <span className="obs-breakdown-value" style={{ color: "#ef4444" }}>{Math.round(m.totalRuns * (100 - m.successRate) / 100)}</span>
                  <span className="obs-breakdown-label">Failed</span>
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* Observatory — Task Distribution */}
        <section className="obs-distribution-section">
          <h3 className="section-title">Task Distribution by Status</h3>
          <div className="obs-distribution">
            {Object.entries(statusDist).map(([status, count]) => {
              const cfg = TASK_STATUS_CONFIG[status as TaskStatus] || { label: status, color: "#6b7280" };
              const pct = Math.round((count / totalTasks) * 100);
              return (
                <div key={status} className="obs-dist-row">
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
    </RoleGate>
  );
}
