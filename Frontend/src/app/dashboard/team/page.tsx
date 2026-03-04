"use client";

import { useState, useMemo } from "react";
import { RoleGate } from "@/components/dashboard/RoleProvider";
import { MOCK_TEAM } from "@/lib/mock-data";
import { ROLE_LABELS } from "@/types/dashboard";
import InviteDialog from "@/components/dashboard/InviteDialog";
import CreateDepartmentDialog from "@/components/dashboard/CreateDepartmentDialog";

export default function TeamPage() {
  const [search, setSearch] = useState("");
  const [deptFilter, setDeptFilter] = useState<string>("all");
  const [showInvite, setShowInvite] = useState(false);
  const [showDept, setShowDept] = useState(false);

  const departments = [...new Set(MOCK_TEAM.map((m) => m.department))];

  const filtered = MOCK_TEAM.filter((m) => {
    if (deptFilter !== "all" && m.department !== deptFilter) return false;
    if (search) {
      const q = search.toLowerCase();
      return m.name.toLowerCase().includes(q) || m.email.toLowerCase().includes(q);
    }
    return true;
  });

  // Stats
  const totalMembers = MOCK_TEAM.length;
  const activeMembers = MOCK_TEAM.filter((m) => m.isActive).length;
  const totalCompleted = MOCK_TEAM.reduce((a, m) => a + m.tasksCompleted, 0);
  const totalPending = MOCK_TEAM.reduce((a, m) => a + m.tasksPending, 0);

  // Dept breakdown
  const deptBreakdown = useMemo(() => {
    const map = new Map<string, { count: number; active: number; completed: number; pending: number }>();
    MOCK_TEAM.forEach((m) => {
      const entry = map.get(m.department) || { count: 0, active: 0, completed: 0, pending: 0 };
      entry.count++;
      if (m.isActive) entry.active++;
      entry.completed += m.tasksCompleted;
      entry.pending += m.tasksPending;
      map.set(m.department, entry);
    });
    return Array.from(map.entries()).map(([dept, data]) => ({ dept, ...data }));
  }, []);

  return (
    <RoleGate
      allowed={["org_admin", "admin"]}
      fallback={
        <div className="dashboard-page">
          <div className="empty-state">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor" width="64" height="64">
              <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" />
            </svg>
            <h3>Access Restricted</h3>
            <p>Team management is available to Admins and Organisation Admins only.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page observatory">
        <div className="page-header">
          <div>
            <h2 className="page-title">Team Observatory</h2>
            <p className="page-subtitle">Team performance, department breakdown, and member management</p>
          </div>
          <div style={{ display: "flex", gap: "8px" }}>
            <button className="action-btn action-btn-outline" onClick={() => setShowDept(true)}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 3h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Z" />
              </svg>
              New Department
            </button>
            <button className="action-btn action-btn-primary" onClick={() => setShowInvite(true)}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                <path strokeLinecap="round" strokeLinejoin="round" d="M18 7.5v3m0 0v3m0-3h3m-3 0h-3m-2.25-4.125a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0ZM3 19.235v-.11a6.375 6.375 0 0 1 12.75 0v.109A12.318 12.318 0 0 1 9.374 21c-2.331 0-4.512-.645-6.374-1.766Z" />
              </svg>
              Invite Employee
            </button>
          </div>
        </div>

        {/* Observatory — KPI Row */}
        <div className="obs-metrics-row">
          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(59,130,246,0.1)", color: "#3b82f6" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M15 19.128a9.38 9.38 0 0 0 2.625.372 9.337 9.337 0 0 0 4.121-.952 4.125 4.125 0 0 0-7.533-2.493M15 19.128v-.003c0-1.113-.285-2.16-.786-3.07M15 19.128v.106A12.318 12.318 0 0 1 8.624 21c-2.331 0-4.512-.645-6.374-1.766l-.001-.109a6.375 6.375 0 0 1 11.964-3.07M12 6.375a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0Zm8.25 2.25a2.625 2.625 0 1 1-5.25 0 2.625 2.625 0 0 1 5.25 0Z" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{totalMembers}</span>
              <span className="obs-metric-label">Total Members</span>
            </div>
          </div>
          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(34,197,94,0.1)", color: "#22c55e" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{activeMembers}</span>
              <span className="obs-metric-label">Active Now</span>
            </div>
          </div>
          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(139,92,246,0.1)", color: "#8b5cf6" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M11.35 3.836c-.065.21-.1.433-.1.664 0 .414.336.75.75.75h4.5a.75.75 0 0 0 .75-.75 2.25 2.25 0 0 0-.1-.664m-5.8 0A2.251 2.251 0 0 1 13.5 2.25H15a2.25 2.25 0 0 1 2.15 1.586m-5.8 0c-.376.023-.75.05-1.124.08C9.095 4.01 8.25 4.973 8.25 6.108V8.25m8.9-4.414c.376.023.75.05 1.124.08 1.131.094 1.976 1.057 1.976 2.192V16.5A2.25 2.25 0 0 1 18 18.75h-2.25m-7.5-10.5H4.875c-.621 0-1.125.504-1.125 1.125v11.25c0 .621.504 1.125 1.125 1.125h9.75c.621 0 1.125-.504 1.125-1.125V18.75m-7.5-10.5h6.375c.621 0 1.125.504 1.125 1.125v9.375m-8.25-3 1.5 1.5 3-3.75" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{totalCompleted}</span>
              <span className="obs-metric-label">Tasks Completed</span>
            </div>
          </div>
          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(245,158,11,0.1)", color: "#f59e0b" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{totalPending}</span>
              <span className="obs-metric-label">Tasks Pending</span>
            </div>
          </div>
        </div>

        {/* Department Breakdown */}
        <div className="obs-chart-grid" style={{ marginBottom: 28 }}>
          {deptBreakdown.map((d) => (
            <div key={d.dept} className="obs-chart-panel">
              <div className="obs-chart-panel-header">
                <h4>{d.dept}</h4>
                <span style={{ fontSize: "0.75rem", color: "var(--text-muted)" }}>{d.count} members</span>
              </div>
              <div className="obs-breakdown">
                <div className="obs-breakdown-item">
                  <span className="obs-breakdown-value">{d.active}</span>
                  <span className="obs-breakdown-label">Active</span>
                </div>
                <div className="obs-breakdown-item">
                  <span className="obs-breakdown-value">{d.completed}</span>
                  <span className="obs-breakdown-label">Completed</span>
                </div>
                <div className="obs-breakdown-item">
                  <span className="obs-breakdown-value">{d.pending}</span>
                  <span className="obs-breakdown-label">Pending</span>
                </div>
              </div>
            </div>
          ))}
        </div>

        {/* Search + Filters */}
        <div className="filters-bar" style={{ marginBottom: 16 }}>
          <div className="filter-search">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
              <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
            </svg>
            <input
              type="text"
              placeholder="Search by name or email..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="filter-search-input"
            />
          </div>
          <div className="filter-select-group">
            <label className="filter-label">Department:</label>
            <select
              value={deptFilter}
              onChange={(e) => setDeptFilter(e.target.value)}
              className="filter-select"
            >
              <option value="all">All Departments</option>
              {departments.map((d) => (
                <option key={d} value={d}>{d}</option>
              ))}
            </select>
          </div>
        </div>

        {/* Observatory-style Team Cards */}
        <div className="obs-team-grid">
          {filtered.map((member) => (
            <div key={member.id} className="obs-team-card">
              <div className="obs-team-card-top">
                <div className="obs-team-avatar">
                  {member.name.split(" ").map((n) => n[0]).join("").slice(0, 2)}
                </div>
                <div className="obs-team-info">
                  <h4 className="obs-team-name">{member.name}</h4>
                  <p className="obs-team-email">{member.email}</p>
                </div>
                <span className={`status-dot ${member.isActive ? "active" : "inactive"}`}>
                  {member.isActive ? "Active" : "Offline"}
                </span>
              </div>

              <div className="obs-team-details">
                <div className="obs-team-detail">
                  <span className="obs-team-detail-label">Department</span>
                  <span className="obs-team-detail-value">{member.department}</span>
                </div>
                <div className="obs-team-detail">
                  <span className="obs-team-detail-label">Role</span>
                  <span className="role-badge">{ROLE_LABELS[member.role]}</span>
                </div>
              </div>

              <div className="obs-team-stats">
                <div className="obs-team-stat">
                  <span className="obs-team-stat-value" style={{ color: "#22c55e" }}>{member.tasksCompleted}</span>
                  <span className="obs-team-stat-label">Done</span>
                </div>
                <div className="obs-team-stat">
                  <span className="obs-team-stat-value" style={{ color: "#f59e0b" }}>{member.tasksPending}</span>
                  <span className="obs-team-stat-label">Pending</span>
                </div>
                <div className="obs-team-stat">
                  <span className="obs-team-stat-value">{member.tasksCompleted + member.tasksPending}</span>
                  <span className="obs-team-stat-label">Total</span>
                </div>
              </div>

              <div className="obs-team-actions">
                <button className="action-btn action-btn-outline action-btn-sm">View Profile</button>
                <button className="action-btn action-btn-primary action-btn-sm">Assign Task</button>
              </div>
            </div>
          ))}
        </div>

        {filtered.length === 0 && (
          <div className="empty-state">
            <h3>No team members found</h3>
            <p>Try adjusting your search or department filter.</p>
          </div>
        )}
      </div>
      <InviteDialog isOpen={showInvite} onClose={() => setShowInvite(false)} />
      <CreateDepartmentDialog isOpen={showDept} onClose={() => setShowDept(false)} />
    </RoleGate>
  );
}
