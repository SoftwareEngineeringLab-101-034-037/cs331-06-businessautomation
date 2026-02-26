"use client";

import { useState, useCallback, useEffect, useRef, useMemo } from "react";
import Link from "next/link";
import TaskDetailDrawer from "@/components/dashboard/TaskDetailDrawer";
import ActivityFeed from "@/components/dashboard/ActivityFeed";
import { RoleGate, useRole } from "@/components/dashboard/RoleProvider";
import { MOCK_TASKS, MOCK_ACTIVITY, MOCK_METRICS } from "@/lib/mock-data";
import { ROLE_LABELS, TASK_STATUS_CONFIG, PRIORITY_CONFIG } from "@/types/dashboard";
import type { Task } from "@/types/dashboard";

// Gantt timeline helpers
function addDays(date: Date, days: number): Date {
  const d = new Date(date);
  d.setDate(d.getDate() + days);
  return d;
}
function daysBetween(a: Date, b: Date): number {
  return Math.round((b.getTime() - a.getTime()) / (1000 * 60 * 60 * 24));
}
function formatShortDate(d: Date): string {
  return d.toLocaleDateString("en-US", { month: "short", day: "numeric" });
}

export default function DashboardOverview() {
  const { role } = useRole();
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [showSearch, setShowSearch] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const searchRef = useRef<HTMLInputElement>(null);
  const handleSelectTask = useCallback((task: Task) => setSelectedTask(task), []);
  const handleCloseDrawer = useCallback(() => setSelectedTask(null), []);

  const pendingTasks = MOCK_TASKS.filter((t) => t.status === "pending").length;
  const inProgressTasks = MOCK_TASKS.filter((t) => t.status === "in_progress").length;
  const completedTasks = MOCK_TASKS.filter((t) => t.status === "completed").length;
  const overdueTasks = MOCK_TASKS.filter((t) => ["overdue", "escalated"].includes(t.status)).length;


  // Cmd+K listener
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault();
        setShowSearch(true);
      }
      if (e.key === "Escape") setShowSearch(false);
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, []);

  useEffect(() => {
    if (showSearch && searchRef.current) searchRef.current.focus();
  }, [showSearch]);

  // Filtered search results
  const searchResults = searchQuery.trim()
    ? MOCK_TASKS.filter(
        (t) =>
          t.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
          t.id.toLowerCase().includes(searchQuery.toLowerCase())
      ).slice(0, 5)
    : [];

  // Most urgent active task for detail panel
  const activeTask = MOCK_TASKS.filter(
    (t) => !["completed", "cancelled"].includes(t.status)
  ).sort((a, b) => {
    const p = { critical: 0, high: 1, medium: 2, low: 3 };
    return p[a.priority] - p[b.priority];
  })[0];

  // Gantt timeline computation
  const { tlStart, tlEnd, tlTotalDays, tlDateCols, tlTasks } = useMemo(() => {
    const sorted = [...MOCK_TASKS].sort(
      (a, b) => new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime()
    );
    let minDate = new Date(sorted[0].createdAt);
    let maxDate = new Date(sorted[0].dueDate);
    sorted.forEach((t) => {
      const c = new Date(t.createdAt);
      const d = new Date(t.dueDate);
      if (c < minDate) minDate = c;
      if (d > maxDate) maxDate = d;
    });
    const start = addDays(minDate, -1);
    const end = addDays(maxDate, 1);
    const total = daysBetween(start, end);
    const step = total > 20 ? 2 : 1;
    const cols: Date[] = [];
    for (let i = 0; i <= total; i += step) cols.push(addDays(start, i));
    return { tlStart: start, tlEnd: end, tlTotalDays: total, tlDateCols: cols, tlTasks: sorted };
  }, []);

  const today = new Date();
  const todayOffset = daysBetween(tlStart, today);
  const todayPct = (todayOffset / tlTotalDays) * 100;
  const showTodayMarker = todayPct >= 0 && todayPct <= 100;

  return (
    <div className="dashboard-page">
      {/* Command Center Search Overlay */}
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
                placeholder="Search tasks, requests, workflows..."
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
              <span className="cmd-search-kbd">ESC</span>
            </div>
            {searchResults.length > 0 && (
              <div className="cmd-search-results">
                {searchResults.map((t) => (
                  <div
                    key={t.id}
                    className="cmd-search-item"
                    onClick={() => {
                      handleSelectTask(t);
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
                      <div className="cmd-search-item-title">{t.title}</div>
                      <div className="cmd-search-item-desc">{t.id} · {t.workflowName}</div>
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

      {/* Top row: compact stats (left) + welcome/search (right) */}
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
            <span className="overview-stat-hint">+2 from yesterday</span>
          </div>
          <div className="overview-stat-card">
            <span className="overview-stat-value">{completedTasks}</span>
            <span className="overview-stat-label">Completed</span>
            <span className="overview-stat-hint">12% this week</span>
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
            <p className="overview-role">Viewing as <strong>{ROLE_LABELS[role]}</strong></p>
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

      {/* ── Gantt Timeline (prominent, near top) ── */}
      <section className="dashboard-section" style={{ marginTop: 16 }}>
        <div className="section-header">
          <h3 className="section-title">Task Timeline</h3>
          <div className="timeline-legend" style={{ borderTop: "none", padding: 0 }}>
            {(Object.entries(TASK_STATUS_CONFIG) as [string, { label: string; color: string }][]).slice(0, 4).map(
              ([key, cfg]) => (
                <div key={key} className="timeline-legend-item">
                  <div className="timeline-legend-color" style={{ background: cfg.color }} />
                  <span>{cfg.label}</span>
                </div>
              )
            )}
          </div>
        </div>

        <div className="timeline-container">
          <div className="timeline-header">
            <div className="timeline-label-col">Task</div>
            <div className="timeline-dates">
              {tlDateCols.map((d, i) => {
                const isWeekend = d.getDay() === 0 || d.getDay() === 6;
                return (
                  <div
                    key={i}
                    className={`timeline-date-col ${isWeekend ? "timeline-weekend" : ""}`}
                    style={{ width: `${100 / tlDateCols.length}%` }}
                  >
                    <span className="timeline-date-label">{formatShortDate(d)}</span>
                  </div>
                );
              })}
            </div>
          </div>

          <div className="timeline-body">
            {showTodayMarker && (
              <div className="timeline-today-marker" style={{ left: `calc(200px + ${todayPct}% * (100% - 200px) / 100%)` }}>
                <div className="timeline-today-label">Today</div>
              </div>
            )}

            {tlTasks.map((task) => {
              const created = new Date(task.createdAt);
              const due = new Date(task.dueDate);
              const startOff = daysBetween(tlStart, created);
              const dur = daysBetween(created, due);
              const leftPct = (startOff / tlTotalDays) * 100;
              const widthPct = Math.max((dur / tlTotalDays) * 100, 2);
              const statusCfg = TASK_STATUS_CONFIG[task.status];
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
                      className={`timeline-bar ${task.status}`}
                      style={{
                        left: `${leftPct}%`,
                        width: `${widthPct}%`,
                        background: `linear-gradient(135deg, ${statusCfg.color}, ${statusCfg.color}dd)`,
                      }}
                      title={`${task.title} — ${formatShortDate(created)} to ${formatShortDate(due)}`}
                    >
                      <span className="timeline-bar-label">
                        {task.title.length > 20 ? task.title.slice(0, 20) + "…" : task.title}
                      </span>
                      <div
                        className="timeline-bar-progress"
                        style={{ width: `${(task.stepNumber / task.totalSteps) * 100}%` }}
                      />
                    </div>
                    <div
                      className="timeline-sla-marker"
                      style={{ left: `${leftPct + widthPct}%` }}
                      title={`Due: ${formatShortDate(due)}`}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        <div className="timeline-summary">
          <div className="timeline-summary-item">
            <span className="timeline-summary-value">{tlTasks.length}</span>
            <span className="timeline-summary-label">Total Tasks</span>
          </div>
          <div className="timeline-summary-item">
            <span className="timeline-summary-value">{tlTasks.filter((t) => ["overdue", "escalated"].includes(t.status)).length}</span>
            <span className="timeline-summary-label">At Risk</span>
          </div>
          <div className="timeline-summary-item">
            <span className="timeline-summary-value">{tlTotalDays}d</span>
            <span className="timeline-summary-label">Span</span>
          </div>
          <div className="timeline-summary-item">
            <span className="timeline-summary-value">{formatShortDate(tlStart)} — {formatShortDate(tlEnd)}</span>
            <span className="timeline-summary-label">Range</span>
          </div>
        </div>
      </section>

      {/* Bottom row: Activity + Workflow Performance | Quick Actions + Most Urgent */}
      <div className="cc-layout" style={{ marginTop: 20 }}>
        <div className="cc-main">
          {/* Recent Activity */}
          <section className="dashboard-section">
            <div className="section-header">
              <h3 className="section-title">Recent Activity</h3>
            </div>
            <ActivityFeed items={MOCK_ACTIVITY} limit={6} />
          </section>

          {/* Workflow Performance (role-gated) */}
          <RoleGate allowed={["org_admin", "admin", "analyst"]}>
            <section className="dashboard-section">
              <div className="section-header">
                <h3 className="section-title">Workflow Performance</h3>
                <Link href="/dashboard/analytics" className="section-link">Full analytics →</Link>
              </div>
              <div className="metrics-grid">
                {MOCK_METRICS.slice(0, 4).map((m) => (
                  <div key={m.workflowName} className="metric-card">
                    <h4 className="metric-name">{m.workflowName}</h4>
                    <div className="metric-stats">
                      <div className="metric-stat">
                        <span className="metric-value">{m.totalRuns}</span>
                        <span className="metric-label">Total Runs</span>
                      </div>
                      <div className="metric-stat">
                        <span className="metric-value">{m.avgCompletionTime}</span>
                        <span className="metric-label">Avg Time</span>
                      </div>
                      <div className="metric-stat">
                        <span className="metric-value" style={{ color: m.successRate >= 90 ? "var(--success)" : "var(--warning)" }}>
                          {m.successRate}%
                        </span>
                        <span className="metric-label">Success</span>
                      </div>
                    </div>
                    {m.bottleneckStep && (
                      <div className="metric-bottleneck">
                        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="14" height="14">
                          <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
                        </svg>
                        Bottleneck: {m.bottleneckStep}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </section>
          </RoleGate>
        </div>

        {/* Right aside */}
        <div className="cc-aside">
          {/* Most Urgent Task */}
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

          {/* Quick Actions */}
          <div className="cc-panel">
            <div className="cc-panel-header">
              <h4 className="cc-panel-title">Quick Actions</h4>
            </div>
            <div className="cc-panel-body">
              <div className="cc-quick-grid">
                <RoleGate allowed={["org_admin", "admin", "employee"]}>
                  <Link href="/dashboard/tasks" className="cc-quick-card">
                    <div className="cc-quick-icon" style={{ background: "var(--accent-subtle)", color: "var(--accent)" }}>
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="18" height="18">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12h3.75M9 15h3.75M9 18h3.75m3 .75H18a2.25 2.25 0 0 0 2.25-2.25V6.108c0-1.135-.845-2.098-1.976-2.192a48.424 48.424 0 0 0-1.123-.08m-5.801 0c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15a2.25 2.25 0 0 1 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m0 0H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V9.375c0-.621-.504-1.125-1.125-1.125H8.25Z" />
                      </svg>
                    </div>
                    View Tasks
                  </Link>
                </RoleGate>
                <RoleGate allowed={["org_admin", "admin", "employee"]}>
                  <Link href="/dashboard/requests" className="cc-quick-card">
                    <div className="cc-quick-icon" style={{ background: "var(--info-subtle)", color: "var(--info)" }}>
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="18" height="18">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                      </svg>
                    </div>
                    New Request
                  </Link>
                </RoleGate>
                <RoleGate allowed={["org_admin", "admin"]}>
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
                <RoleGate allowed={["org_admin", "admin"]}>
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

      {/* Task detail drawer */}
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
