"use client";

import Link from "next/link";
import { MOCK_DEPARTMENTS, MOCK_TEAM, MOCK_METRICS } from "@/lib/mock-data";
import { RoleGate } from "@/components/dashboard/RoleProvider";

export default function WorkstationPage() {
  const totalEmployees = MOCK_TEAM.length;
  const activeEmployees = MOCK_TEAM.filter((m) => m.isActive).length;
  const totalWorkflows = MOCK_METRICS.length;
  const totalDepartments = MOCK_DEPARTMENTS.length;
  const avgSuccess = Math.round(MOCK_METRICS.reduce((a, m) => a + m.successRate, 0) / MOCK_METRICS.length);

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
            <p>The Workstation view is only available to Admins and Organisation Admins.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page">
        <div className="page-header">
          <div>
            <h2 className="page-title">Organisation Workstation</h2>
            <p className="page-subtitle">Full overview of your organisation&apos;s structure, departments, and operations</p>
          </div>
          <Link href="/workflow-builder" className="action-btn action-btn-primary">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="18" height="18">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
            </svg>
            Create Workflow
          </Link>
        </div>

        {/* Bento Grid — Org KPIs */}
        <div className="bento-grid" style={{ marginBottom: 28 }}>
          <div className="bento-tile">
            <div>
              <div className="bento-value">{totalEmployees}</div>
              <div className="bento-label">Total Employees</div>
            </div>
            <div className="bento-trend up">{activeEmployees} active now</div>
          </div>
          <div className="bento-tile">
            <div>
              <div className="bento-value">{totalDepartments}</div>
              <div className="bento-label">Departments</div>
            </div>
          </div>
          <div className="bento-tile">
            <div>
              <div className="bento-value">{totalWorkflows}</div>
              <div className="bento-label">Active Workflows</div>
            </div>
          </div>
          <div className="bento-tile">
            <div>
              <div className="bento-value" style={{ color: avgSuccess >= 90 ? "#22c55e" : "#f59e0b" }}>{avgSuccess}%</div>
              <div className="bento-label">Avg Success Rate</div>
            </div>
          </div>
        </div>

        {/* Departments */}
        <section className="dashboard-section">
          <div className="section-header">
            <h3 className="section-title">Departments</h3>
          </div>
          <div className="departments-grid">
            {MOCK_DEPARTMENTS.map((dept) => (
              <div key={dept.id} className="department-card">
                <div className="department-card-header">
                  <h4 className="department-name">{dept.name}</h4>
                  <span className="department-head">Head: {dept.head}</span>
                </div>
                <div className="department-stats">
                  <div className="department-stat">
                    <span className="department-stat-value">{dept.memberCount}</span>
                    <span className="department-stat-label">Members</span>
                  </div>
                  <div className="department-stat">
                    <span className="department-stat-value">{dept.activeWorkflows}</span>
                    <span className="department-stat-label">Workflows</span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* Employees Table */}
        <section className="dashboard-section">
          <div className="section-header">
            <h3 className="section-title">Employees</h3>
          </div>
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Email</th>
                  <th>Department</th>
                  <th>Role</th>
                  <th>Tasks Done</th>
                  <th>Pending</th>
                  <th>Status</th>
                </tr>
              </thead>
              <tbody>
                {MOCK_TEAM.map((member) => (
                  <tr key={member.id}>
                    <td className="table-name-cell">
                      <div className="table-avatar">
                        {member.name.split(" ").map((n) => n[0]).join("").slice(0, 2)}
                      </div>
                      {member.name}
                    </td>
                    <td className="table-muted">{member.email}</td>
                    <td>{member.department}</td>
                    <td>
                      <span className="role-badge">{member.role}</span>
                    </td>
                    <td className="table-center">{member.tasksCompleted}</td>
                    <td className="table-center">{member.tasksPending}</td>
                    <td>
                      <span className={`status-dot ${member.isActive ? "active" : "inactive"}`}>
                        {member.isActive ? "Active" : "Offline"}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>

        {/* Workflow performance */}
        <section className="dashboard-section">
          <div className="section-header">
            <h3 className="section-title">Workflow Performance Overview</h3>
          </div>
          <div className="table-container">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Workflow</th>
                  <th>Total Runs</th>
                  <th>Avg Completion</th>
                  <th>Success Rate</th>
                  <th>Bottleneck</th>
                </tr>
              </thead>
              <tbody>
                {MOCK_METRICS.map((m) => (
                  <tr key={m.workflowName}>
                    <td className="font-medium">{m.workflowName}</td>
                    <td className="table-center">{m.totalRuns}</td>
                    <td className="table-center">{m.avgCompletionTime}</td>
                    <td className="table-center">
                      <span style={{ color: m.successRate >= 90 ? "#22c55e" : m.successRate >= 80 ? "#f59e0b" : "#ef4444", fontWeight: 600 }}>
                        {m.successRate}%
                      </span>
                    </td>
                    <td>
                      {m.bottleneckStep ? (
                        <span className="bottleneck-badge">⚠ {m.bottleneckStep}</span>
                      ) : (
                        <span className="table-muted">None detected</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      </div>
    </RoleGate>
  );
}
