"use client";

import { useState, useMemo, useEffect, useCallback } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";
import { RoleGate } from "@/components/dashboard/RoleProvider";
import InviteDialog from "@/components/dashboard/InviteDialog";
import CreateDepartmentDialog from "@/components/dashboard/CreateDepartmentDialog";

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";

/* ── Backend response shapes ─────────────────────────── */

interface BackendDepartment {
  id: string;
  name: string;
  description?: string;
}

interface BackendRole {
  id: string;
  name: string;
  display_name?: string;
}

interface BackendUser {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  avatar_url?: string;
  department_id?: string;
  role_id?: string;
  job_title?: string;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  last_sign_in_at?: string;
  department?: BackendDepartment;
  role?: BackendRole;
}

/* ── Helpers ─────────────────────────────────────────── */

function initials(first: string, last: string): string {
  return ((first?.[0] || "") + (last?.[0] || "")).toUpperCase() || "?";
}

function roleLabel(role?: BackendRole): string {
  if (!role) return "Employee";
  return role.display_name || role.name || "Employee";
}

export default function TeamPage() {
  const [search, setSearch] = useState("");
  const [deptFilter, setDeptFilter] = useState<string>("all");
  const [showInvite, setShowInvite] = useState(false);
  const [showDept, setShowDept] = useState(false);

  const [employees, setEmployees] = useState<BackendUser[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const { getToken } = useAuth();
  const { organization } = useOrganization();

  /* ── Fetch employees from backend ───────────────────── */
  const fetchEmployees = useCallback(async () => {
    if (!organization?.id) return;
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      const res = await fetch(`${AUTH_API}/api/orgs/${organization.id}/employees`, {
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: BackendUser[] = await res.json();
      setEmployees(data ?? []);
    } catch (err: any) {
      console.error("Failed to load employees:", err);
      setError(err.message || "Could not reach auth service");
    } finally {
      setLoading(false);
    }
  }, [organization?.id, getToken]);

  useEffect(() => {
    fetchEmployees();
  }, [fetchEmployees]);

  /* ── Derived data ───────────────────────────────────── */
  const departments = useMemo(
    () => [...new Set(employees.map((e) => e.department?.name).filter(Boolean) as string[])].sort(),
    [employees]
  );

  const filtered = useMemo(() => {
    return employees.filter((e) => {
      const deptName = e.department?.name || "";
      if (deptFilter !== "all" && deptName !== deptFilter) return false;
      if (search) {
        const q = search.toLowerCase();
        const name = `${e.first_name} ${e.last_name}`.toLowerCase();
        return name.includes(q) || e.email.toLowerCase().includes(q);
      }
      return true;
    });
  }, [employees, deptFilter, search]);

  const totalMembers = employees.length;
  const activeMembers = employees.filter((e) => e.is_active).length;

  const deptBreakdown = useMemo(() => {
    const map = new Map<string, { count: number; active: number }>();
    employees.forEach((e) => {
      const dept = e.department?.name || "Unassigned";
      const entry = map.get(dept) || { count: 0, active: 0 };
      entry.count++;
      if (e.is_active) entry.active++;
      map.set(dept, entry);
    });
    return Array.from(map.entries()).map(([dept, data]) => ({ dept, ...data }));
  }, [employees]);

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
              <span className="obs-metric-label">Active</span>
            </div>
          </div>
          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(139,92,246,0.1)", color: "#8b5cf6" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 21h19.5m-18-18v18m10.5-18v18m6-13.5V21M6.75 6.75h.75m-.75 3h.75m-.75 3h.75m3-6h.75m-.75 3h.75m-.75 3h.75M6.75 21v-3.375c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125V21M3 3h12m-.75 4.5H21m-3.75 3h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Zm0 3h.008v.008h-.008v-.008Z" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{departments.length}</span>
              <span className="obs-metric-label">Departments</span>
            </div>
          </div>
          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(245,158,11,0.1)", color: "#f59e0b" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{totalMembers - activeMembers}</span>
              <span className="obs-metric-label">Inactive</span>
            </div>
          </div>
        </div>

        {/* Department Breakdown */}
        {deptBreakdown.length > 0 && (
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
                    <span className="obs-breakdown-value">{d.count - d.active}</span>
                    <span className="obs-breakdown-label">Inactive</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}

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
          <button className="action-btn action-btn-outline action-btn-sm" onClick={fetchEmployees} style={{ marginLeft: "auto" }}>
            Refresh
          </button>
        </div>

        {/* Loading state */}
        {loading && (
          <div className="empty-state" style={{ padding: "40px 0" }}>
            <p className="table-muted">Loading team members…</p>
          </div>
        )}

        {/* Error state */}
        {error && !loading && (
          <div className="empty-state" style={{ padding: "40px 0" }}>
            <p style={{ color: "#ef4444" }}>⚠ {error}</p>
            <button className="action-btn action-btn-outline action-btn-sm" style={{ marginTop: 12 }} onClick={fetchEmployees}>Retry</button>
          </div>
        )}

        {/* Team Cards */}
        {!loading && !error && (
          <div className="obs-team-grid">
            {filtered.map((member) => (
              <div key={member.id} className="obs-team-card">
                <div className="obs-team-card-top">
                  <div className="obs-team-avatar">
                    {initials(member.first_name, member.last_name)}
                  </div>
                  <div className="obs-team-info">
                    <h4 className="obs-team-name">{member.first_name} {member.last_name}</h4>
                    <p className="obs-team-email">{member.email}</p>
                  </div>
                  <span className={`status-dot ${member.is_active ? "active" : "inactive"}`}>
                    {member.is_active ? "Active" : "Inactive"}
                  </span>
                </div>

                <div className="obs-team-details">
                  <div className="obs-team-detail">
                    <span className="obs-team-detail-label">Department</span>
                    <span className="obs-team-detail-value">{member.department?.name || "—"}</span>
                  </div>
                  <div className="obs-team-detail">
                    <span className="obs-team-detail-label">Role</span>
                    <span className="role-badge">{roleLabel(member.role)}</span>
                  </div>
                </div>

                {member.job_title && (
                  <div className="obs-team-details">
                    <div className="obs-team-detail">
                      <span className="obs-team-detail-label">Job Title</span>
                      <span className="obs-team-detail-value">{member.job_title}</span>
                    </div>
                  </div>
                )}

                <div className="obs-team-actions">
                  <button className="action-btn action-btn-outline action-btn-sm">View Profile</button>
                  <button className="action-btn action-btn-primary action-btn-sm">Assign Task</button>
                </div>
              </div>
            ))}
          </div>
        )}

        {!loading && !error && filtered.length === 0 && (
          <div className="empty-state">
            <h3>No team members found</h3>
            <p>{employees.length === 0 ? "Invite employees to get started." : "Try adjusting your search or department filter."}</p>
          </div>
        )}
      </div>
      <InviteDialog isOpen={showInvite} onClose={() => setShowInvite(false)} />
      <CreateDepartmentDialog isOpen={showDept} onClose={() => setShowDept(false)} />
    </RoleGate>
  );
}
