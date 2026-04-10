"use client";

import { useState, useMemo, useEffect, useCallback } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";
import { RoleGate } from "@/components/dashboard/RoleProvider";
import InviteDialog from "@/components/dashboard/InviteDialog";
import { useToast, ToastContainer } from "@/components/Toast";

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";
const WF_API = process.env.NEXT_PUBLIC_WF_API || "http://localhost:8085";

interface BackendDepartment {
  id: string;
  name: string;
  description?: string;
}

interface BackendRole {
  id: string;
  name: string;
  display_name?: string;
  description?: string;
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
  display_name?: string;
  description?: string;
  members?: BackendRoleMember[];
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
  is_admin: boolean;
  is_active: boolean;
  created_at: string;
  updated_at: string;
  last_sign_in_at?: string;
  department?: BackendDepartment;
  role?: BackendRole;
}

interface WorkflowTask {
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
  form_template_id?: string;
  sla_days?: number;
  status: string;
  comment?: string;
  created_at: string;
  completed_at?: string;
}

interface BackendInvitation {
  id: string;
  organization_id: string;
  department_id?: string;
  email: string;
  first_name: string;
  last_name: string;
  role_name?: string;
  role_names?: unknown;
  job_title?: string;
  status: string;
  invited_by?: string;
  expires_at: string;
  created_at: string;
  updated_at: string;
  department?: BackendDepartment;
}

type TeamView = "employees" | "invites";

function initials(first: string, last: string): string {
  return ((first?.[0] || "") + (last?.[0] || "")).toUpperCase() || "?";
}

function dashboardAccessLabel(user?: Pick<BackendUser, "is_admin"> | null): string {
  if (!user) return "Employee";
  return user.is_admin ? "Admin" : "Employee";
}

function workflowRoleLabel(role: BackendRoleSummary): string {
  return role.display_name || role.name;
}

function formatDateTime(value?: string): string {
  if (!value) return "—";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "—";
  return new Intl.DateTimeFormat("en", {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  }).format(parsed);
}

function formatTaskStatus(status: string): string {
  return status.replaceAll("_", " ").replace(/\b\w/g, (char) => char.toUpperCase());
}

function normalizeError(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message) return error.message;
  return fallback;
}

function arraysEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  const sortedA = [...a].sort();
  const sortedB = [...b].sort();
  return sortedA.every((value, index) => value === sortedB[index]);
}

function uniqueStrings(values: string[]): string[] {
  return [...new Set(values.filter(Boolean))];
}

function invitationRoleNames(invitation: BackendInvitation): string[] {
  const raw = invitation.role_names;
  if (Array.isArray(raw)) {
    return uniqueStrings(raw.filter((value): value is string => typeof value === "string").map((value) => value.trim()));
  }
  if (typeof raw === "string") {
    try {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return uniqueStrings(parsed.filter((value): value is string => typeof value === "string").map((value) => value.trim()));
      }
    } catch {
      const split = raw.split(",").map((value) => value.trim()).filter(Boolean);
      if (split.length > 0) return uniqueStrings(split);
    }
  }
  if (invitation.role_name) {
    return [invitation.role_name];
  }
  return [];
}

export default function TeamPage() {
  const [view, setView] = useState<TeamView>("employees");
  const [search, setSearch] = useState("");
  const [deptFilter, setDeptFilter] = useState<string>("all");
  const [showInvite, setShowInvite] = useState(false);

  const [employees, setEmployees] = useState<BackendUser[]>([]);
  const [invitations, setInvitations] = useState<BackendInvitation[]>([]);
  const [roles, setRoles] = useState<BackendRoleSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [selectedEmployeeID, setSelectedEmployeeID] = useState<string | null>(null);
  const [selectedRoleIDs, setSelectedRoleIDs] = useState<string[]>([]);
  const [roleSaving, setRoleSaving] = useState(false);
  const [roleSaveError, setRoleSaveError] = useState<string | null>(null);
  const [roleSaveSuccess, setRoleSaveSuccess] = useState<string | null>(null);

  const [tasks, setTasks] = useState<WorkflowTask[]>([]);
  const [taskLoading, setTaskLoading] = useState(false);
  const [taskError, setTaskError] = useState<string | null>(null);
  const [memberMenuOpen, setMemberMenuOpen] = useState(false);
  const [showRemoveConfirm, setShowRemoveConfirm] = useState(false);
  const [removeConfirmText, setRemoveConfirmText] = useState("");
  const [removeLoading, setRemoveLoading] = useState(false);
  const [selectedInviteID, setSelectedInviteID] = useState<string | null>(null);
  const [inviteActionLoading, setInviteActionLoading] = useState(false);
  const [inviteActionError, setInviteActionError] = useState<string | null>(null);

  const { getToken, userId } = useAuth();
  const { organization } = useOrganization();
  const { toasts, showToast, dismissToast } = useToast();

  const authorizedFetch = useCallback(async (input: string, init: RequestInit = {}): Promise<Response> => {
    const token = await getToken();
    const headers = new Headers(init.headers);
    if (token) {
      headers.set("Authorization", `Bearer ${token}`);
    }
    return fetch(input, {
      ...init,
      headers,
    });
  }, [getToken]);

  const fetchTeamData = useCallback(async () => {
    if (!organization?.id) return;
    setLoading(true);
    setError(null);

    try {
      const [employeeRes, roleRes, invitationRes] = await Promise.all([
        authorizedFetch(`${AUTH_API}/api/orgs/${organization.id}/employees`),
        authorizedFetch(`${AUTH_API}/api/orgs/${organization.id}/roles`),
        authorizedFetch(`${AUTH_API}/api/orgs/${organization.id}/invitations`),
      ]);

      if (!employeeRes.ok || !roleRes.ok || !invitationRes.ok) {
        throw new Error(`Failed to load team data (${employeeRes.status}/${roleRes.status}/${invitationRes.status})`);
      }

      const [employeeData, roleData, invitationData] = await Promise.all([
        employeeRes.json() as Promise<BackendUser[]>,
        roleRes.json() as Promise<BackendRoleSummary[]>,
        invitationRes.json() as Promise<BackendInvitation[]>,
      ]);

      setEmployees(Array.isArray(employeeData) ? employeeData : []);
      setRoles(Array.isArray(roleData) ? roleData : []);
      setInvitations(Array.isArray(invitationData) ? invitationData : []);
    } catch (fetchError) {
      console.error("Failed to load team data:", fetchError);
      setError(normalizeError(fetchError, "Could not reach auth service"));
    } finally {
      setLoading(false);
    }
  }, [organization?.id, authorizedFetch]);

  useEffect(() => {
    fetchTeamData();
  }, [fetchTeamData]);

  const roleAssignmentsByUser = useMemo(() => {
    const assignments = new Map<string, BackendRoleSummary[]>();
    roles.forEach((role) => {
      (role.members || []).forEach((member) => {
        const current = assignments.get(member.id) || [];
        current.push(role);
        current.sort((left, right) => workflowRoleLabel(left).localeCompare(workflowRoleLabel(right)));
        assignments.set(member.id, current);
      });
    });
    return assignments;
  }, [roles]);

  const departments = useMemo(() => {
    const values = [
      ...employees.map((employee) => employee.department?.name || "").filter(Boolean),
      ...invitations.map((invitation) => invitation.department?.name || "").filter(Boolean),
    ];
    return [...new Set(values)].sort();
  }, [employees, invitations]);

  const filteredEmployees = useMemo(() => {
    return employees.filter((employee) => {
      const deptName = employee.department?.name || "";
      if (deptFilter !== "all" && deptName !== deptFilter) return false;
      if (!search) return true;
      const query = search.toLowerCase();
      const fullName = `${employee.first_name} ${employee.last_name}`.toLowerCase();
      return fullName.includes(query) || employee.email.toLowerCase().includes(query);
    });
  }, [employees, deptFilter, search]);

  const orderedMembers = useMemo(() => {
    return [...filteredEmployees].sort((left, right) => {
      const leftIsYou = left.id === userId;
      const rightIsYou = right.id === userId;
      if (leftIsYou && !rightIsYou) return -1;
      if (!leftIsYou && rightIsYou) return 1;
      return `${left.first_name} ${left.last_name}`.localeCompare(`${right.first_name} ${right.last_name}`);
    });
  }, [filteredEmployees, userId]);

  const filteredInvitations = useMemo(() => {
    return invitations.filter((invitation) => {
      const deptName = invitation.department?.name || "";
      if (deptFilter !== "all" && deptName !== deptFilter) return false;
      if (!search) return true;
      const query = search.toLowerCase();
      const fullName = `${invitation.first_name} ${invitation.last_name}`.toLowerCase();
      return fullName.includes(query) || invitation.email.toLowerCase().includes(query);
    });
  }, [invitations, deptFilter, search]);

  const orderedInvitations = useMemo(() => {
    return [...filteredInvitations].sort((left, right) => {
      return new Date(right.created_at).getTime() - new Date(left.created_at).getTime();
    });
  }, [filteredInvitations]);

  const totalMembers = employees.length;
  const activeMembers = employees.filter((employee) => employee.is_active).length;
  const unassignedMembers = employees.filter((employee) => !employee.department?.name).length;

  const selectedEmployee = useMemo(
    () => employees.find((employee) => employee.id === selectedEmployeeID) || null,
    [employees, selectedEmployeeID]
  );

  const selectedInvitation = useMemo(
    () => invitations.find((invitation) => invitation.id === selectedInviteID) || null,
    [invitations, selectedInviteID]
  );

  const selectedEmployeeRoles = useMemo(
    () => (selectedEmployeeID ? roleAssignmentsByUser.get(selectedEmployeeID) || [] : []),
    [roleAssignmentsByUser, selectedEmployeeID]
  );

  const availableRoles = useMemo(
    () => roles.filter((role) => !selectedRoleIDs.includes(role.id)).sort((left, right) => workflowRoleLabel(left).localeCompare(workflowRoleLabel(right))),
    [roles, selectedRoleIDs]
  );

  const hasRoleChanges = useMemo(
    () => !arraysEqual(selectedRoleIDs, selectedEmployeeRoles.map((role) => role.id)),
    [selectedRoleIDs, selectedEmployeeRoles]
  );

  const taskSummary = useMemo(() => {
    const summary = {
      total: tasks.length,
      pending: 0,
      completed: 0,
      rejected: 0,
      other: 0,
    };
    tasks.forEach((task) => {
      if (task.status === "pending") summary.pending += 1;
      else if (task.status === "completed" || task.status === "approved") summary.completed += 1;
      else if (task.status === "rejected") summary.rejected += 1;
      else summary.other += 1;
    });
    return summary;
  }, [tasks]);

  useEffect(() => {
    if (!selectedEmployeeID) {
      setSelectedRoleIDs([]);
      setRoleSaveError(null);
      setRoleSaveSuccess(null);
      return;
    }
    setSelectedRoleIDs(selectedEmployeeRoles.map((role) => role.id));
    setRoleSaveError(null);
    setRoleSaveSuccess(null);
  }, [selectedEmployeeID, selectedEmployeeRoles]);

  useEffect(() => {
    if (selectedEmployeeID && !selectedEmployee) {
      setSelectedEmployeeID(null);
    }
  }, [selectedEmployee, selectedEmployeeID]);

  useEffect(() => {
    if (selectedInviteID && !selectedInvitation) {
      setSelectedInviteID(null);
      setInviteActionError(null);
      setInviteActionLoading(false);
    }
  }, [selectedInvitation, selectedInviteID]);

  useEffect(() => {
    setSearch("");
    setDeptFilter("all");
    setInviteActionError(null);
    setMemberMenuOpen(false);
    setShowRemoveConfirm(false);
    setRemoveConfirmText("");
    if (view === "employees") {
      setSelectedInviteID(null);
    } else {
      setSelectedEmployeeID(null);
      setSelectedRoleIDs([]);
      setRoleSaveError(null);
      setRoleSaveSuccess(null);
      setTasks([]);
      setTaskError(null);
      setTaskLoading(false);
    }
  }, [view]);

  useEffect(() => {
    setMemberMenuOpen(false);
    setShowRemoveConfirm(false);
    setRemoveConfirmText("");
    setRemoveLoading(false);
  }, [selectedEmployeeID]);

  const closeRemoveConfirm = useCallback(() => {
    if (removeLoading) return;
    setRemoveConfirmText("");
    setShowRemoveConfirm(false);
  }, [removeLoading]);

  useEffect(() => {
    let active = true;

    const loadTasks = async () => {
      if (!selectedEmployeeID || !organization?.id) {
        setTasks([]);
        setTaskError(null);
        setTaskLoading(false);
        return;
      }

      const roleNames = uniqueStrings(selectedEmployeeRoles.map((role) => role.name));
      if (roleNames.length === 0) {
        setTasks([]);
        setTaskError(null);
        setTaskLoading(false);
        return;
      }

      setTaskLoading(true);
      setTaskError(null);

      try {
        const responses = await Promise.allSettled(
          roleNames.map(async (roleName) => {
            const response = await authorizedFetch(`${WF_API}/api/orgs/${organization.id}/tasks?role=${encodeURIComponent(roleName)}`);
            if (!response.ok) {
              throw new Error(`Workflow task lookup failed for ${roleName}`);
            }
            const data = await response.json() as WorkflowTask[];
            return Array.isArray(data) ? data : [];
          })
        );

        if (!active) return;

        const taskMap = new Map<string, WorkflowTask>();
        let failedLookups = 0;

        responses.forEach((result) => {
          if (result.status === "fulfilled") {
            result.value.forEach((task) => {
              if (task.assigned_user && task.assigned_user !== selectedEmployeeID) {
                return;
              }
              taskMap.set(task.id, task);
            });
          } else {
            failedLookups += 1;
          }
        });

        const aggregated = Array.from(taskMap.values()).sort((left, right) => {
          return new Date(right.created_at).getTime() - new Date(left.created_at).getTime();
        });

        setTasks(aggregated);
        setTaskError(failedLookups > 0 ? "Some role-based task feeds could not be loaded." : null);
      } catch (taskLoadError) {
        if (!active) return;
        setTasks([]);
        setTaskError(normalizeError(taskLoadError, "Could not load workflow tasks"));
      } finally {
        if (active) {
          setTaskLoading(false);
        }
      }
    };

    loadTasks();

    return () => {
      active = false;
    };
  }, [selectedEmployeeID, selectedEmployeeRoles, organization?.id, authorizedFetch]);

  const openEmployeeDrawer = useCallback((employeeID: string) => {
    setSelectedInviteID(null);
    setSelectedEmployeeID(employeeID);
  }, []);

  const closeEmployeeDrawer = useCallback(() => {
    closeRemoveConfirm();
    setSelectedEmployeeID(null);
    setRoleSaveError(null);
    setRoleSaveSuccess(null);
  }, [closeRemoveConfirm]);

  const openInviteDrawer = useCallback((invitationID: string) => {
    setSelectedEmployeeID(null);
    setSelectedInviteID(invitationID);
    setInviteActionError(null);
  }, []);

  const closeInviteDrawer = useCallback(() => {
    if (inviteActionLoading) return;
    setSelectedInviteID(null);
    setInviteActionError(null);
  }, [inviteActionLoading]);

  const addRoleToDraft = useCallback((roleID: string) => {
    if (!roleID) return;
    setSelectedRoleIDs((current) => (current.includes(roleID) ? current : [...current, roleID]));
    setRoleSaveError(null);
    setRoleSaveSuccess(null);
  }, []);

  const removeRoleFromDraft = useCallback((roleID: string) => {
    setSelectedRoleIDs((current) => current.filter((value) => value !== roleID));
    setRoleSaveError(null);
    setRoleSaveSuccess(null);
  }, []);

  const saveRoleAssignments = useCallback(async () => {
    if (!organization?.id || !selectedEmployeeID) return;

    const desiredRoleIDs = new Set(selectedRoleIDs);
    const changedRoles = roles.filter((role) => {
      const currentlyAssigned = (role.members || []).some((member) => member.id === selectedEmployeeID);
      const shouldBeAssigned = desiredRoleIDs.has(role.id);
      return currentlyAssigned !== shouldBeAssigned;
    });

    if (changedRoles.length === 0) {
      setRoleSaveSuccess("No role changes to save.");
      setRoleSaveError(null);
      return;
    }

    setRoleSaving(true);
    setRoleSaveError(null);
    setRoleSaveSuccess(null);

    try {
      await Promise.all(changedRoles.map(async (role) => {
        const currentMemberIDs = uniqueStrings((role.members || []).map((member) => member.id));
        const nextMemberIDs = desiredRoleIDs.has(role.id)
          ? uniqueStrings([...currentMemberIDs, selectedEmployeeID])
          : currentMemberIDs.filter((memberID) => memberID !== selectedEmployeeID);

        const response = await authorizedFetch(`${AUTH_API}/api/orgs/${organization.id}/roles/${role.id}`, {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
          },
          body: JSON.stringify({
            name: role.name,
            description: role.description || "",
            member_ids: nextMemberIDs,
          }),
        });

        if (!response.ok) {
          const payload = await response.json().catch(() => ({} as { error?: string }));
          throw new Error(payload.error || `Failed to update role ${workflowRoleLabel(role)}`);
        }
      }));

      await fetchTeamData();
      setRoleSaveSuccess("Workflow roles updated for this employee.");
    } catch (saveError) {
      setRoleSaveError(normalizeError(saveError, "Failed to update employee roles"));
    } finally {
      setRoleSaving(false);
    }
  }, [organization?.id, selectedEmployeeID, selectedRoleIDs, roles, authorizedFetch, fetchTeamData]);

  const handleInviteResult = useCallback((message: string, type: "success" | "error") => {
    showToast(message, type);
    if (type === "success") {
      void fetchTeamData();
    }
  }, [showToast, fetchTeamData]);

  const canRemoveSelectedMember = !!selectedEmployee && !selectedEmployee.is_admin && selectedEmployee.id !== userId;

  const removeSelectedMember = useCallback(async () => {
    if (!organization?.id || !selectedEmployee || !canRemoveSelectedMember) return;
    if (removeConfirmText.trim().toLowerCase() !== "remove") return;

    setRemoveLoading(true);
    try {
      const res = await authorizedFetch(`${AUTH_API}/api/orgs/${organization.id}/employees/${selectedEmployee.id}`, {
        method: "DELETE",
      });
      const payload = await res.json().catch(() => ({} as { error?: string; message?: string }));
      if (!res.ok) {
        throw new Error(payload.error || `Failed to remove member (${res.status})`);
      }

      showToast(payload.message || "Member removed successfully.", "success");
      closeRemoveConfirm();
      setMemberMenuOpen(false);
      closeEmployeeDrawer();
      await fetchTeamData();
    } catch (removeError) {
      showToast(normalizeError(removeError, "Failed to remove member"), "error");
    } finally {
      setRemoveLoading(false);
    }
  }, [organization?.id, selectedEmployee, canRemoveSelectedMember, removeConfirmText, authorizedFetch, showToast, closeRemoveConfirm, closeEmployeeDrawer, fetchTeamData]);

  const revokeSelectedInvitation = useCallback(async () => {
    if (!organization?.id || !selectedInvitation) return;
    setInviteActionLoading(true);
    setInviteActionError(null);
    try {
      const res = await authorizedFetch(`${AUTH_API}/api/orgs/${organization.id}/invitations/${selectedInvitation.id}`, {
        method: "DELETE",
      });
      const payload = await res.json().catch(() => ({} as { error?: string; message?: string }));
      if (!res.ok) {
        throw new Error(payload.error || `Failed to revoke invitation (${res.status})`);
      }
      showToast(payload.message || "Invitation revoked.", "success");
      closeInviteDrawer();
      await fetchTeamData();
    } catch (revokeError) {
      setInviteActionError(normalizeError(revokeError, "Failed to revoke invitation"));
    } finally {
      setInviteActionLoading(false);
    }
  }, [organization?.id, selectedInvitation, authorizedFetch, showToast, closeInviteDrawer, fetchTeamData]);

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
            <p>Team management is available to Admins only.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page observatory team-observatory">
        <div className="page-header">
          <div>
            <h2 className="page-title">Team Observatory</h2>
            <p className="page-subtitle">Click a member to inspect their profile, workflow roles, and current task queue</p>
          </div>
          <div style={{ display: "flex", gap: "8px" }}>
            <button className="action-btn action-btn-primary" onClick={() => setShowInvite(true)}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                <path strokeLinecap="round" strokeLinejoin="round" d="M18 7.5v3m0 0v3m0-3h3m-3 0h-3m-2.25-4.125a3.375 3.375 0 1 1-6.75 0 3.375 3.375 0 0 1 6.75 0ZM3 19.235v-.11a6.375 6.375 0 0 1 12.75 0v.109A12.318 12.318 0 0 1 9.374 21c-2.331 0-4.512-.645-6.374-1.766Z" />
              </svg>
              Invite Employee
            </button>
          </div>
        </div>

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
          <div className="obs-metric-card">
            <div className="obs-metric-icon" style={{ background: "rgba(148,163,184,0.14)", color: "#64748b" }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9.568 3.192A.75.75 0 0 1 10.25 3h3.5a.75.75 0 0 1 .682.442l.902 2.015 2.186.316a.75.75 0 0 1 .416 1.279l-1.597 1.556.377 2.197a.75.75 0 0 1-1.088.79L12 10.61l-1.958 1.03a.75.75 0 0 1-1.088-.79l.377-2.197-1.597-1.556a.75.75 0 0 1 .416-1.28l2.186-.315.902-2.015Z" />
              </svg>
            </div>
            <div className="obs-metric-data">
              <span className="obs-metric-value">{unassignedMembers}</span>
              <span className="obs-metric-label">Unassigned</span>
            </div>
          </div>
        </div>

        <div className="team-view-tabs" role="tablist" aria-label="Team views">
          <button
            type="button"
            role="tab"
            aria-selected={view === "employees"}
            className={`team-view-tab ${view === "employees" ? "active" : ""}`}
            onClick={() => setView("employees")}
          >
            Employees ({employees.length})
          </button>
          <button
            type="button"
            role="tab"
            aria-selected={view === "invites"}
            className={`team-view-tab ${view === "invites" ? "active" : ""}`}
            onClick={() => setView("invites")}
          >
            Invites ({invitations.length})
          </button>
        </div>

        <div className="filters-bar" style={{ marginBottom: 16 }}>
          <div className="filter-search">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
              <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
            </svg>
            <input
              type="text"
              placeholder="Search by name or email..."
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              className="filter-search-input"
            />
          </div>
          <div className="filter-select-group">
            <label className="filter-label">Department:</label>
            <select
              value={deptFilter}
              onChange={(event) => setDeptFilter(event.target.value)}
              className="filter-select"
            >
              <option value="all">All Departments</option>
              {departments.map((department) => (
                <option key={department} value={department}>{department}</option>
              ))}
            </select>
          </div>
          <button className="action-btn action-btn-outline action-btn-sm" onClick={fetchTeamData} style={{ marginLeft: "auto" }}>
            Refresh
          </button>
        </div>

        {loading && (
          <div className="empty-state" style={{ padding: "40px 0" }}>
            <p className="table-muted">{view === "employees" ? "Loading team members…" : "Loading invitations…"}</p>
          </div>
        )}

        {error && !loading && (
          <div className="empty-state" style={{ padding: "40px 0" }}>
            <p style={{ color: "#ef4444" }}>⚠ {error}</p>
            <button className="action-btn action-btn-outline action-btn-sm" style={{ marginTop: 12 }} onClick={fetchTeamData}>Retry</button>
          </div>
        )}

        {!loading && !error && view === "employees" && (
          <div className="obs-team-grid">
            {orderedMembers.map((member) => {
              const memberRoles = roleAssignmentsByUser.get(member.id) || [];
              const isYou = member.id === userId;
              return (
                <div
                  key={member.id}
                  className={`obs-team-card obs-team-card-interactive ${isYou ? "obs-team-card-self" : ""}`}
                  onClick={() => openEmployeeDrawer(member.id)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      openEmployeeDrawer(member.id);
                    }
                  }}
                  role="button"
                  tabIndex={0}
                >
                  {isYou && <span className="obs-you-badge">You</span>}
                  <div className="obs-team-card-top">
                    <div className="obs-team-avatar">{initials(member.first_name, member.last_name)}</div>
                    <div className="obs-team-info">
                      <h4 className="obs-team-name">{member.first_name} {member.last_name}</h4>
                      <p className="obs-team-email">{member.email}</p>
                    </div>
                    <span className={`status-dot ${member.is_active ? "active" : "inactive"}`}>{member.is_active ? "Active" : "Inactive"}</span>
                  </div>

                  <div className="obs-team-details">
                    <div className="obs-team-detail">
                      <span className="obs-team-detail-label">Department</span>
                      <span className="obs-team-detail-value">{member.department?.name || "—"}</span>
                    </div>
                    <div className="obs-team-detail">
                      <span className="obs-team-detail-label">Dashboard Access</span>
                      <span className="role-badge">{dashboardAccessLabel(member)}</span>
                    </div>
                  </div>

                  <div className="obs-team-details">
                    <div className="obs-team-detail">
                      <span className="obs-team-detail-label">Job Title</span>
                      <span className="obs-team-detail-value">{member.job_title || "—"}</span>
                    </div>
                    <div className="obs-team-detail">
                      <span className="obs-team-detail-label">Workflow Roles</span>
                      <span className="obs-team-detail-value">{memberRoles.length}</span>
                    </div>
                  </div>

                  <div className="obs-team-role-list">
                    {memberRoles.length > 0 ? (
                      <>
                        {memberRoles.slice(0, 2).map((role) => (
                          <span key={role.id} className="obs-role-chip">{workflowRoleLabel(role)}</span>
                        ))}
                        {memberRoles.length > 2 && <span className="obs-role-chip obs-role-chip-count">+{memberRoles.length - 2} more</span>}
                      </>
                    ) : (
                      <span className="obs-role-chip obs-role-chip-muted">No workflow roles</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {!loading && !error && view === "invites" && (
          <div className="obs-team-grid">
            {orderedInvitations.map((invitation) => {
              const inviteRoles = invitationRoleNames(invitation);
              return (
                <div
                  key={invitation.id}
                  className="obs-team-card obs-team-card-interactive"
                  onClick={() => openInviteDrawer(invitation.id)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      openInviteDrawer(invitation.id);
                    }
                  }}
                  role="button"
                  tabIndex={0}
                >
                  <div className="obs-team-card-top">
                    <div className="obs-team-avatar">{initials(invitation.first_name, invitation.last_name)}</div>
                    <div className="obs-team-info">
                      <h4 className="obs-team-name">{invitation.first_name} {invitation.last_name}</h4>
                      <p className="obs-team-email">{invitation.email}</p>
                    </div>
                    <span className="status-dot pending">Pending</span>
                  </div>

                  <div className="obs-team-details">
                    <div className="obs-team-detail">
                      <span className="obs-team-detail-label">Department</span>
                      <span className="obs-team-detail-value">{invitation.department?.name || "—"}</span>
                    </div>
                    <div className="obs-team-detail">
                      <span className="obs-team-detail-label">Invited</span>
                      <span className="obs-team-detail-value">{formatDateTime(invitation.created_at)}</span>
                    </div>
                  </div>

                  <div className="obs-team-details">
                    <div className="obs-team-detail">
                      <span className="obs-team-detail-label">Expires</span>
                      <span className="obs-team-detail-value">{formatDateTime(invitation.expires_at)}</span>
                    </div>
                    <div className="obs-team-detail">
                      <span className="obs-team-detail-label">Job Title</span>
                      <span className="obs-team-detail-value">{invitation.job_title || "—"}</span>
                    </div>
                  </div>

                  <div className="obs-team-role-list">
                    {inviteRoles.length > 0 ? (
                      <>
                        {inviteRoles.slice(0, 2).map((roleName) => (
                          <span key={`${invitation.id}-${roleName}`} className="obs-role-chip">{roleName}</span>
                        ))}
                        {inviteRoles.length > 2 && <span className="obs-role-chip obs-role-chip-count">+{inviteRoles.length - 2} more</span>}
                      </>
                    ) : (
                      <span className="obs-role-chip obs-role-chip-muted">No workflow roles</span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        )}

        {!loading && !error && view === "employees" && filteredEmployees.length === 0 && (
          <div className="empty-state">
            <h3>No team members found</h3>
            <p>{employees.length === 0 ? "Invite employees to get started." : "Try adjusting your search or department filter."}</p>
          </div>
        )}

        {!loading && !error && view === "invites" && filteredInvitations.length === 0 && (
          <div className="empty-state">
            <h3>No invitations found</h3>
            <p>{invitations.length === 0 ? "No pending invites right now." : "Try adjusting your search or department filter."}</p>
          </div>
        )}
      </div>

      {selectedEmployee && (
        <div className="drawer-overlay" onClick={closeEmployeeDrawer}>
          <aside className="drawer-panel employee-drawer-panel" onClick={(event) => event.stopPropagation()}>
            <div className="drawer-header employee-drawer-header">
              <div className="employee-drawer-identity">
                <div className="employee-drawer-avatar">{initials(selectedEmployee.first_name, selectedEmployee.last_name)}</div>
                <div className="employee-drawer-meta">
                  <div className="employee-drawer-title-row">
                    <h3 className="drawer-task-title">{selectedEmployee.first_name} {selectedEmployee.last_name}</h3>
                    <span className={`status-dot ${selectedEmployee.is_active ? "active" : "inactive"}`}>
                      {selectedEmployee.is_active ? "Active" : "Inactive"}
                    </span>
                  </div>
                  <p className="employee-drawer-subtitle">{selectedEmployee.email}</p>
                  <div className="employee-drawer-badges">
                    <span className="employee-id-pill">ID {selectedEmployee.id}</span>
                    <span className="role-badge">Dashboard {dashboardAccessLabel(selectedEmployee)}</span>
                  </div>
                </div>
              </div>
              {canRemoveSelectedMember && (
                <div className="employee-drawer-menu-wrap">
                  <button
                    type="button"
                    className="wf-row-menu-btn"
                    aria-label="Member options"
                    onClick={() => setMemberMenuOpen((open) => !open)}
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="16" height="16">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M12 6.75a.75.75 0 1 0 0-1.5.75.75 0 0 0 0 1.5Zm0 6a.75.75 0 1 0 0-1.5.75.75 0 0 0 0 1.5Zm0 6a.75.75 0 1 0 0-1.5.75.75 0 0 0 0 1.5Z" />
                    </svg>
                  </button>
                  {memberMenuOpen && (
                    <div className="employee-member-menu" role="menu">
                      <button
                        type="button"
                        className="employee-member-menu-item danger"
                        onClick={() => {
                          setMemberMenuOpen(false);
                          setShowRemoveConfirm(true);
                        }}
                      >
                        Remove Member
                      </button>
                    </div>
                  )}
                </div>
              )}
              <button className="drawer-close-btn" onClick={closeEmployeeDrawer} aria-label="Close employee details">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="18" height="18">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="drawer-body">
              <section className="drawer-section">
                <h4 className="drawer-section-title">Employee Overview</h4>
                <dl className="drawer-info-grid">
                  <div className="detail-info-item">
                    <dt>Email</dt>
                    <dd>{selectedEmployee.email}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Department</dt>
                    <dd>{selectedEmployee.department?.name || "Unassigned"}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Job Title</dt>
                    <dd>{selectedEmployee.job_title || "—"}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Dashboard Access</dt>
                    <dd>{dashboardAccessLabel(selectedEmployee)}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Joined</dt>
                    <dd>{formatDateTime(selectedEmployee.created_at)}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Last Sign-In</dt>
                    <dd>{formatDateTime(selectedEmployee.last_sign_in_at)}</dd>
                  </div>
                </dl>
              </section>

              <section className="drawer-section">
                <h4 className="drawer-section-title">Workflow Role Tags</h4>
                <p className="employee-inline-note">Add or remove workflow roles for this employee the same way you would manage tags.</p>

                {roleSaveError && <div className="employee-inline-feedback error">{roleSaveError}</div>}
                {roleSaveSuccess && <div className="employee-inline-feedback success">{roleSaveSuccess}</div>}

                <div className="employee-role-editor">
                  <div className="employee-role-chip-row">
                    {selectedRoleIDs.length > 0 ? (
                      selectedRoleIDs.map((roleID) => {
                        const role = roles.find((candidate) => candidate.id === roleID);
                        if (!role) return null;
                        return (
                          <button
                            key={roleID}
                            type="button"
                            className="employee-role-chip"
                            onClick={() => removeRoleFromDraft(roleID)}
                            disabled={roleSaving}
                            title={`Remove ${workflowRoleLabel(role)}`}
                          >
                            {workflowRoleLabel(role)}
                            <span aria-hidden="true">×</span>
                          </button>
                        );
                      })
                    ) : (
                      <span className="employee-role-empty">No workflow roles assigned yet.</span>
                    )}
                  </div>

                  <div className="employee-role-add-row">
                    <select
                      className="filter-select employee-role-select"
                      defaultValue=""
                      onChange={(event) => {
                        addRoleToDraft(event.target.value);
                        event.target.value = "";
                      }}
                      disabled={availableRoles.length === 0 || roleSaving}
                    >
                      <option value="">Add workflow role…</option>
                      {availableRoles.map((role) => (
                        <option key={role.id} value={role.id}>{workflowRoleLabel(role)}</option>
                      ))}
                    </select>
                    <button
                      type="button"
                      className="action-btn action-btn-primary"
                      onClick={saveRoleAssignments}
                      disabled={!hasRoleChanges || roleSaving}
                    >
                      {roleSaving ? "Saving…" : "Save Roles"}
                    </button>
                  </div>
                </div>
              </section>

              <section className="drawer-section">
                <h4 className="drawer-section-title">Task Queue</h4>
                <p className="employee-inline-note">Tasks are aggregated from the workflow roles currently assigned to this employee.</p>

                {taskLoading && <p className="employee-inline-note">Loading tasks…</p>}
                {taskError && <div className="employee-inline-feedback error">{taskError}</div>}

                {!taskLoading && !taskError && (
                  <div className="employee-task-summary">
                    <div className="employee-task-stat">
                      <strong>{taskSummary.total}</strong>
                      <span>Total</span>
                    </div>
                    <div className="employee-task-stat">
                      <strong>{taskSummary.pending}</strong>
                      <span>Pending</span>
                    </div>
                    <div className="employee-task-stat">
                      <strong>{taskSummary.completed}</strong>
                      <span>Completed</span>
                    </div>
                    <div className="employee-task-stat">
                      <strong>{taskSummary.rejected + taskSummary.other}</strong>
                      <span>Other</span>
                    </div>
                  </div>
                )}

                {!taskLoading && tasks.length === 0 && !taskError && (
                  <div className="employee-task-empty">No workflow tasks were found for this employee’s current role tags.</div>
                )}

                {!taskLoading && tasks.length > 0 && (
                  <div className="employee-task-list">
                    {tasks.map((task) => (
                      <article key={task.id} className="employee-task-item">
                        <div className="employee-task-item-top">
                          <div>
                            <h5 className="employee-task-title">{task.title}</h5>
                            <p className="employee-task-meta">
                              {task.assigned_role || "Unscoped role"} · Created {formatDateTime(task.created_at)}
                            </p>
                          </div>
                          <span className={`employee-task-status status-${task.status}`}>{formatTaskStatus(task.status)}</span>
                        </div>
                        {task.description && <p className="employee-task-description">{task.description}</p>}
                        <div className="employee-task-footer">
                          <span className="task-card-id">Task {task.id}</span>
                          {task.instance_id && <span className="employee-task-meta">Instance {task.instance_id}</span>}
                        </div>
                      </article>
                    ))}
                  </div>
                )}
              </section>
            </div>
          </aside>
        </div>
      )}

      {selectedInvitation && (
        <div className="drawer-overlay" onClick={closeInviteDrawer}>
          <aside className="drawer-panel employee-drawer-panel" onClick={(event) => event.stopPropagation()}>
            <div className="drawer-header employee-drawer-header">
              <div className="employee-drawer-identity">
                <div className="employee-drawer-avatar">{initials(selectedInvitation.first_name, selectedInvitation.last_name)}</div>
                <div className="employee-drawer-meta">
                  <div className="employee-drawer-title-row">
                    <h3 className="drawer-task-title">{selectedInvitation.first_name} {selectedInvitation.last_name}</h3>
                    <span className="status-dot pending">Pending</span>
                  </div>
                  <p className="employee-drawer-subtitle">{selectedInvitation.email}</p>
                  <div className="employee-drawer-badges">
                    <span className="employee-id-pill">Invite {selectedInvitation.id}</span>
                    <span className="role-badge">Status pending</span>
                  </div>
                </div>
              </div>
              <button className="drawer-close-btn" onClick={closeInviteDrawer} aria-label="Close invitation details">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="18" height="18">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="drawer-body">
              <section className="drawer-section">
                <h4 className="drawer-section-title">Invitation Overview</h4>
                <dl className="drawer-info-grid">
                  <div className="detail-info-item">
                    <dt>Email</dt>
                    <dd>{selectedInvitation.email}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Department</dt>
                    <dd>{selectedInvitation.department?.name || "Unassigned"}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Job Title</dt>
                    <dd>{selectedInvitation.job_title || "—"}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Invited By</dt>
                    <dd>{selectedInvitation.invited_by || "—"}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Created</dt>
                    <dd>{formatDateTime(selectedInvitation.created_at)}</dd>
                  </div>
                  <div className="detail-info-item">
                    <dt>Expires</dt>
                    <dd>{formatDateTime(selectedInvitation.expires_at)}</dd>
                  </div>
                </dl>
              </section>

              <section className="drawer-section">
                <h4 className="drawer-section-title">Workflow Role Tags</h4>
                <div className="obs-team-role-list">
                  {invitationRoleNames(selectedInvitation).length > 0 ? (
                    invitationRoleNames(selectedInvitation).map((roleName) => (
                      <span key={`${selectedInvitation.id}-drawer-${roleName}`} className="obs-role-chip">{roleName}</span>
                    ))
                  ) : (
                    <span className="obs-role-chip obs-role-chip-muted">No workflow roles</span>
                  )}
                </div>
              </section>

              <section className="drawer-section">
                {inviteActionError && <div className="employee-inline-feedback error">{inviteActionError}</div>}
                <div className="employee-drawer-actions">
                  <button
                    type="button"
                    className="action-btn action-btn-danger"
                    onClick={revokeSelectedInvitation}
                    disabled={inviteActionLoading}
                  >
                    {inviteActionLoading ? "Revoking..." : "Revoke Invitation"}
                  </button>
                </div>
              </section>
            </div>
          </aside>
        </div>
      )}

      <InviteDialog
        isOpen={showInvite}
        onClose={() => setShowInvite(false)}
        onResult={handleInviteResult}
      />

      {showRemoveConfirm && selectedEmployee && (
        <div className="invite-overlay" onClick={closeRemoveConfirm}>
          <div className="invite-dialog" onClick={(event) => event.stopPropagation()} style={{ maxWidth: 520 }}>
            <div className="invite-header">
              <div className="invite-header-text">
                <h3 className="invite-title">Confirm Member Removal</h3>
                <p className="invite-subtitle">
                  This permanently deletes this member's auth records, memberships, role tags, and invitations for this organization.
                </p>
              </div>
              <button
                className="invite-close"
                onClick={closeRemoveConfirm}
                aria-label="Close"
              >
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="18" height="18">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                </svg>
              </button>
            </div>

            <div className="invite-form" style={{ paddingTop: 8 }}>
              <div className="employee-inline-feedback error" style={{ marginBottom: 8 }}>
                Type <strong>remove</strong> to confirm deleting {selectedEmployee.first_name} {selectedEmployee.last_name}.
              </div>
              <div className="invite-field">
                <label className="invite-label">Confirmation</label>
                <input
                  type="text"
                  className="invite-input"
                  value={removeConfirmText}
                  onChange={(event) => setRemoveConfirmText(event.target.value)}
                  placeholder='Type "remove"'
                  disabled={removeLoading}
                />
              </div>
              <div className="invite-actions">
                <button
                  type="button"
                  className="action-btn action-btn-outline"
                  onClick={closeRemoveConfirm}
                  disabled={removeLoading}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className="action-btn action-btn-danger"
                  onClick={removeSelectedMember}
                  disabled={removeLoading || removeConfirmText.trim().toLowerCase() !== "remove"}
                >
                  {removeLoading ? "Removing..." : "Delete Member"}
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </RoleGate>
  );
}