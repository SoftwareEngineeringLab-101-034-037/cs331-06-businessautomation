"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useAuth, useOrganization, useUser } from "@clerk/nextjs";
import { useTheme } from "@/components/ThemeProvider";
import { useRole } from "@/components/dashboard/RoleProvider";
import { ROLE_LABELS, type UserRole } from "@/types/dashboard";
import CreateDepartmentDialog from "@/components/dashboard/CreateDepartmentDialog";
import CreateRoleDialog from "@/components/dashboard/CreateRoleDialog";

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";

type SettingsSection = "org" | "departments" | "roles" | "account" | "notifications" | "access" | "appearance";

const NOTIFICATION_STORAGE_KEY = "dashboard-notification-preferences";

interface ActorSummary {
  id: string;
  name: string;
  email?: string;
}

interface BackendDepartmentSummary {
  id: string;
  name: string;
  description?: string;
  created_by_user_id?: string;
  created_at: string;
  member_count: number;
  created_by?: ActorSummary;
}

interface BackendRoleMember {
  id: string;
  first_name: string;
  last_name: string;
  email: string;
  job_title?: string;
  department?: string;
}

interface BackendRoleSummary {
  id: string;
  name: string;
  description?: string;
  created_by_user_id?: string;
  created_at: string;
  member_count: number;
  created_by?: ActorSummary;
  members?: BackendRoleMember[];
}

interface BackendEmployee {
  id: string;
  first_name: string;
  last_name: string;
  email: string;
  job_title?: string;
  department?: {
    id: string;
    name: string;
  };
}

const SECTION_META: Record<SettingsSection, { label: string; description: string }> = {
  org: {
    label: "Org Settings",
    description: "Organisation identity, ownership, and access context.",
  },
  departments: {
    label: "Departments",
    description: "Department structure, ownership, and member distribution.",
  },
  roles: {
    label: "Roles",
    description: "Workflow role groups, members, and assignment structure.",
  },
  account: {
    label: "Account",
    description: "Identity, sign-in context, and user-facing profile details.",
  },
  notifications: {
    label: "Notifications",
    description: "Personal alert defaults for workflow and dashboard activity.",
  },
  access: {
    label: "Access",
    description: "Dashboard access roles and workflow-assignment model overview.",
  },
  appearance: {
    label: "Appearance",
    description: "Theme and interface look-and-feel preferences.",
  },
};

function canManageSettings(role: UserRole) {
  return role === "admin";
}

function formatDate(value?: string) {
  if (!value) return "—";
  return new Intl.DateTimeFormat("en", {
    year: "numeric",
    month: "short",
    day: "numeric",
  }).format(new Date(value));
}

export default function SettingsPage() {
  const { user } = useUser();
  const { organization } = useOrganization();
  const { getToken } = useAuth();
  const { theme, toggle } = useTheme();
  const { role } = useRole();

  const adminMode = canManageSettings(role);
  const [activeSection, setActiveSection] = useState<SettingsSection>(adminMode ? "org" : "appearance");
  const [departments, setDepartments] = useState<BackendDepartmentSummary[]>([]);
  const [roles, setRoles] = useState<BackendRoleSummary[]>([]);
  const [employees, setEmployees] = useState<BackendEmployee[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showDepartmentDialog, setShowDepartmentDialog] = useState(false);
  const [showRoleDialog, setShowRoleDialog] = useState(false);
  const [editingDepartment, setEditingDepartment] = useState<BackendDepartmentSummary | null>(null);
  const [editingRole, setEditingRole] = useState<BackendRoleSummary | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<{ type: "department" | "role"; id: string; name: string } | null>(null);
  const [notificationPrefs, setNotificationPrefs] = useState({
    email: true,
    browser: true,
    taskAssigned: true,
    taskCompleted: false,
    escalations: true,
  });

  useEffect(() => {
    setActiveSection(adminMode ? "org" : "appearance");
  }, [adminMode]);

  const sections = useMemo<SettingsSection[]>(() => (
    adminMode
      ? ["org", "departments", "roles", "account", "notifications", "access", "appearance"]
      : ["account", "notifications", "appearance"]
  ), [adminMode]);

  useEffect(() => {
    try {
      const saved = window.localStorage.getItem(NOTIFICATION_STORAGE_KEY);
      if (!saved) return;
      const parsed = JSON.parse(saved);
      setNotificationPrefs((current) => ({ ...current, ...parsed }));
    } catch {
      // Ignore malformed local settings.
    }
  }, []);

  useEffect(() => {
    window.localStorage.setItem(NOTIFICATION_STORAGE_KEY, JSON.stringify(notificationPrefs));
  }, [notificationPrefs]);

  const resolveCreatorName = useCallback((createdBy?: ActorSummary, createdByUserID?: string) => {
    if (createdBy?.name) return createdBy.name;
    if (createdByUserID && createdByUserID === user?.id) return "You";
    if (createdByUserID) return `User ${createdByUserID}`;
    return "Unknown creator";
  }, [user?.id]);

  const fetchManagementData = useCallback(async () => {
    if (!adminMode || !organization?.id) return;
    setLoading(true);
    setError(null);
    try {
      const token = await getToken();
      const [departmentRes, roleRes, employeeRes] = await Promise.all([
        fetch(`${AUTH_API}/api/orgs/${organization.id}/departments`, {
          headers: { Authorization: `Bearer ${token}` },
        }),
        fetch(`${AUTH_API}/api/orgs/${organization.id}/roles`, {
          headers: { Authorization: `Bearer ${token}` },
        }),
        fetch(`${AUTH_API}/api/orgs/${organization.id}/employees`, {
          headers: { Authorization: `Bearer ${token}` },
        }),
      ]);

      if (!departmentRes.ok || !roleRes.ok || !employeeRes.ok) {
        throw new Error("Failed to load settings data");
      }

      const [departmentData, roleData, employeeData] = await Promise.all([
        departmentRes.json(),
        roleRes.json(),
        employeeRes.json(),
      ]);

      setDepartments(Array.isArray(departmentData) ? departmentData : []);
      setRoles(Array.isArray(roleData) ? roleData : []);
      setEmployees(Array.isArray(employeeData) ? employeeData : []);
    } catch (fetchError: any) {
      setError(fetchError.message || "Could not load settings data");
    } finally {
      setLoading(false);
    }
  }, [adminMode, organization?.id, getToken]);

  const deleteEntity = useCallback(async () => {
    if (!confirmDelete || !organization?.id) return;
    try {
      const token = await getToken();
      const path = confirmDelete.type === "department"
        ? `${AUTH_API}/api/orgs/${organization.id}/departments/${confirmDelete.id}`
        : `${AUTH_API}/api/orgs/${organization.id}/roles/${confirmDelete.id}`;
      const res = await fetch(path, {
        method: "DELETE",
        headers: { Authorization: `Bearer ${token}` },
      });
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        throw new Error(data.error || `Failed to delete ${confirmDelete.type}`);
      }
      setConfirmDelete(null);
      fetchManagementData();
    } catch (deleteError: any) {
      setError(deleteError.message || "Delete failed");
    }
  }, [confirmDelete, organization?.id, getToken, fetchManagementData]);

  useEffect(() => {
    fetchManagementData();
  }, [fetchManagementData]);

  return (
    <div className="dashboard-page settings-workspace-page">
      <div className="page-header">
        <div>
          <h2 className="page-title">Settings</h2>
          <p className="page-subtitle">Manage organisation structure, workflow roles, and interface preferences</p>
        </div>
      </div>

      <div className="settings-workspace">
        <aside className="settings-sidebar-shell settings-sidebar-left">
          <div className="settings-sidebar-title">Workspace Settings</div>
          <div className="settings-sidebar-list">
            {sections.map((section) => {
              const meta = SECTION_META[section];
              return (
                <button
                  key={section}
                  className={`settings-sidebar-item ${activeSection === section ? "active" : ""}`}
                  onClick={() => setActiveSection(section)}
                >
                  <strong>{meta.label}</strong>
                  <span>{meta.description}</span>
                </button>
              );
            })}
          </div>
        </aside>

        <section className="settings-content-shell">
          {error && (
            <div className="invite-message invite-message-error" style={{ marginBottom: 16 }}>
              {error}
            </div>
          )}

          {activeSection === "org" && (
            <div className="settings-panel-grid">
              <div className="settings-card">
                <div className="settings-card-header">
                  <h4>Organisation Profile</h4>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Organisation Name</span>
                    <span className="settings-row-desc">{organization?.name || "—"}</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Organisation ID</span>
                    <span className="settings-row-desc">{organization?.id || "—"}</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Managed Access Role</span>
                    <span className="settings-row-desc">{ROLE_LABELS[role]}</span>
                  </div>
                  <span className="settings-badge settings-badge-muted">Current</span>
                </div>
              </div>

              <div className="settings-card">
                <div className="settings-card-header">
                  <h4>Admin Context</h4>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Signed In As</span>
                    <span className="settings-row-desc">{user?.fullName || user?.primaryEmailAddress?.emailAddress || "—"}</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Email</span>
                    <span className="settings-row-desc">{user?.primaryEmailAddress?.emailAddress || "—"}</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Structure Snapshot</span>
                    <span className="settings-row-desc">{departments.length} departments · {roles.length} workflow roles · {employees.length} employees</span>
                  </div>
                </div>
              </div>
            </div>
          )}

          {activeSection === "departments" && (
            <div className="settings-panel-stack">
              <div className="settings-panel-header-row">
                <div>
                  <h3>Departments</h3>
                  <p>Departments are organisational containers. Roles and employees can be filtered using them.</p>
                </div>
                <button className="action-btn action-btn-outline" onClick={() => { setEditingDepartment(null); setShowDepartmentDialog(true); }}>
                  New Department
                </button>
              </div>

              {loading ? (
                <div className="settings-empty-inline">Loading departments...</div>
              ) : departments.length > 0 ? (
                <div className="settings-list-shell">
                  <div className="settings-list-header settings-list-grid-department">
                    <span>Department</span>
                    <span>Description</span>
                    <span>People</span>
                    <span>Created</span>
                    <span>Actions</span>
                  </div>
                  {departments.map((department) => (
                    <div key={department.id} className="settings-list-row settings-list-grid-department">
                      <div className="settings-list-primary">
                        <strong>{department.name}</strong>
                        <span>Created by {resolveCreatorName(department.created_by, department.created_by_user_id)}</span>
                      </div>
                      <span>{department.description || "No description"}</span>
                      <span>{department.member_count}</span>
                      <span>{formatDate(department.created_at)}</span>
                      <div className="settings-row-actions">
                        <button className="action-btn action-btn-outline action-btn-sm" onClick={() => { setEditingDepartment(department); setShowDepartmentDialog(true); }}>
                          Edit
                        </button>
                        <button className="action-btn action-btn-outline action-btn-sm" onClick={() => setConfirmDelete({ type: "department", id: department.id, name: department.name })}>
                          Delete
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="settings-empty-inline">No departments created yet.</div>
              )}
            </div>
          )}

          {activeSection === "roles" && (
            <div className="settings-panel-stack">
              <div className="settings-panel-header-row">
                <div>
                  <h3>Workflow Roles</h3>
                  <p>Roles group people for workflow assignment. They are separate from job titles.</p>
                </div>
                <button className="action-btn action-btn-outline" onClick={() => { setEditingRole(null); setShowRoleDialog(true); }}>
                  New Role
                </button>
              </div>

              {loading ? (
                <div className="settings-empty-inline">Loading roles...</div>
              ) : roles.length > 0 ? (
                <div className="settings-list-shell">
                  <div className="settings-list-header settings-list-grid-role">
                    <span>Role</span>
                    <span>Description</span>
                    <span>Members</span>
                    <span>Created</span>
                    <span>Actions</span>
                  </div>
                  {roles.map((roleItem) => (
                    <div key={roleItem.id} className="settings-list-row settings-list-grid-role role-list-row-expand">
                      <div className="settings-list-primary">
                        <strong>{roleItem.name}</strong>
                        <span>Created by {resolveCreatorName(roleItem.created_by, roleItem.created_by_user_id)}</span>
                      </div>
                      <span>{roleItem.description || "No description"}</span>
                      <div>
                        <strong>{roleItem.member_count}</strong>
                        {roleItem.members && roleItem.members.length > 0 && (
                          <div className="settings-inline-members">
                            {roleItem.members.slice(0, 3).map((member) => `${member.first_name} ${member.last_name}`).join(", ")}
                            {roleItem.members.length > 3 ? ` +${roleItem.members.length - 3}` : ""}
                          </div>
                        )}
                      </div>
                      <span>{formatDate(roleItem.created_at)}</span>
                      <div className="settings-row-actions">
                        <button className="action-btn action-btn-outline action-btn-sm" onClick={() => { setEditingRole(roleItem); setShowRoleDialog(true); }}>
                          Edit
                        </button>
                        <button className="action-btn action-btn-outline action-btn-sm" onClick={() => setConfirmDelete({ type: "role", id: roleItem.id, name: roleItem.name })}>
                          Delete
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="settings-empty-inline">No workflow roles created yet.</div>
              )}
            </div>
          )}

          {activeSection === "account" && (
            <div className="settings-panel-grid">
              <div className="settings-card">
                <div className="settings-card-header">
                  <h4>Account</h4>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Signed In User</span>
                    <span className="settings-row-desc">{user?.fullName || "—"}</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Email</span>
                    <span className="settings-row-desc">{user?.primaryEmailAddress?.emailAddress || "—"}</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Access Role</span>
                    <span className="settings-row-desc">{ROLE_LABELS[role]}</span>
                  </div>
                </div>
              </div>

              <div className="settings-card">
                <div className="settings-card-header">
                  <h4>Workspace Notes</h4>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Workflow Roles</span>
                    <span className="settings-row-desc">Employees can now be tagged with multiple workflow roles independent of job title.</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Organisation Ownership</span>
                    <span className="settings-row-desc">Department and role management lives under Settings for admins only.</span>
                  </div>
                </div>
              </div>
            </div>
          )}

          {activeSection === "notifications" && (
            <div className="settings-panel-grid">
              <div className="settings-card">
                <div className="settings-card-header">
                  <h4>Delivery Channels</h4>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Email Notifications</span>
                    <span className="settings-row-desc">Receive workflow and invite updates by email.</span>
                  </div>
                  <label className="settings-switch">
                    <input
                      type="checkbox"
                      checked={notificationPrefs.email}
                      onChange={(e) => setNotificationPrefs((current) => ({ ...current, email: e.target.checked }))}
                    />
                    <span className="settings-switch-slider" />
                  </label>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Browser Notifications</span>
                    <span className="settings-row-desc">Enable browser-level alerts while the dashboard is open.</span>
                  </div>
                  <label className="settings-switch">
                    <input
                      type="checkbox"
                      checked={notificationPrefs.browser}
                      onChange={(e) => setNotificationPrefs((current) => ({ ...current, browser: e.target.checked }))}
                    />
                    <span className="settings-switch-slider" />
                  </label>
                </div>
              </div>

              <div className="settings-card">
                <div className="settings-card-header">
                  <h4>Workflow Events</h4>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Task Assigned</span>
                    <span className="settings-row-desc">Alert when a task is assigned to you or your team.</span>
                  </div>
                  <label className="settings-switch">
                    <input
                      type="checkbox"
                      checked={notificationPrefs.taskAssigned}
                      onChange={(e) => setNotificationPrefs((current) => ({ ...current, taskAssigned: e.target.checked }))}
                    />
                    <span className="settings-switch-slider" />
                  </label>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Task Completed</span>
                    <span className="settings-row-desc">Alert when a task you track reaches completion.</span>
                  </div>
                  <label className="settings-switch">
                    <input
                      type="checkbox"
                      checked={notificationPrefs.taskCompleted}
                      onChange={(e) => setNotificationPrefs((current) => ({ ...current, taskCompleted: e.target.checked }))}
                    />
                    <span className="settings-switch-slider" />
                  </label>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Escalations</span>
                    <span className="settings-row-desc">Alert when SLA breaches or escalations happen in workflows you oversee.</span>
                  </div>
                  <label className="settings-switch">
                    <input
                      type="checkbox"
                      checked={notificationPrefs.escalations}
                      onChange={(e) => setNotificationPrefs((current) => ({ ...current, escalations: e.target.checked }))}
                    />
                    <span className="settings-switch-slider" />
                  </label>
                </div>
                <div className="settings-card-footer">
                  <span className="settings-saved-msg settings-saved-static">Saved on this device</span>
                </div>
              </div>
            </div>
          )}

          {activeSection === "access" && adminMode && (
            <div className="settings-panel-grid">
              <div className="settings-card">
                <div className="settings-card-header">
                  <h4>Dashboard Access</h4>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Current Access Role</span>
                    <span className="settings-row-desc">{ROLE_LABELS[role]} controls what this user can manage in the dashboard.</span>
                  </div>
                  <span className="settings-badge settings-badge-muted">Access</span>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Admin Coverage</span>
                    <span className="settings-row-desc">Settings, role management, and structural controls remain admin-managed.</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Sign-In Provider</span>
                    <span className="settings-row-desc">Clerk is still the source of truth for identity and session management.</span>
                  </div>
                </div>
              </div>

              <div className="settings-card">
                <div className="settings-card-header">
                  <h4>Workflow Assignment Model</h4>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Departments</span>
                    <span className="settings-row-desc">{departments.length} organisational containers for filtering people and ownership.</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Workflow Roles</span>
                    <span className="settings-row-desc">{roles.length} reusable assignment groups separate from employee job titles.</span>
                  </div>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Employee Memberships</span>
                    <span className="settings-row-desc">Roles can now hold multiple employees, and employees can carry multiple workflow-role tags.</span>
                  </div>
                </div>
              </div>
            </div>
          )}

          {activeSection === "appearance" && (
            <div className="settings-panel-grid">
              <div className="settings-card">
                <div className="settings-card-header">
                  <h4>Appearance</h4>
                </div>
                <div className="settings-row">
                  <div className="settings-row-info">
                    <span className="settings-row-label">Theme</span>
                    <span className="settings-row-desc">Switch between light and dark mode</span>
                  </div>
                  <button className="settings-toggle-btn" onClick={toggle}>
                    {theme === "dark" ? "Dark Mode" : "Light Mode"}
                  </button>
                </div>
              </div>
            </div>
          )}
        </section>
      </div>

      <CreateDepartmentDialog
        isOpen={showDepartmentDialog}
        onClose={() => {
          setShowDepartmentDialog(false);
          setEditingDepartment(null);
        }}
        onCreated={() => {
          setShowDepartmentDialog(false);
          setEditingDepartment(null);
          fetchManagementData();
        }}
        initialDepartment={editingDepartment ? {
          id: editingDepartment.id,
          name: editingDepartment.name,
          description: editingDepartment.description,
        } : null}
      />
      <CreateRoleDialog
        isOpen={showRoleDialog}
        onClose={() => {
          setShowRoleDialog(false);
          setEditingRole(null);
        }}
        onCreated={() => {
          setShowRoleDialog(false);
          setEditingRole(null);
          fetchManagementData();
        }}
        employees={employees}
        initialRole={editingRole ? {
          id: editingRole.id,
          name: editingRole.name,
          description: editingRole.description,
          memberIds: editingRole.members?.map((member) => member.id) || [],
        } : null}
      />
      {confirmDelete && (
        <div className="modal-overlay" onClick={() => setConfirmDelete(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()}>
            <div className="modal-body modal-confirm-body">
              <h3 className="modal-title">Delete {confirmDelete.type === "department" ? "Department" : "Role"}</h3>
              <p className="modal-desc">
                This will remove <strong>{confirmDelete.name}</strong>. This action cannot be undone.
              </p>
            </div>
            <div className="modal-footer">
              <button className="action-btn action-btn-outline" onClick={() => setConfirmDelete(null)}>Cancel</button>
              <button className="action-btn action-btn-danger" onClick={deleteEntity}>Delete</button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
