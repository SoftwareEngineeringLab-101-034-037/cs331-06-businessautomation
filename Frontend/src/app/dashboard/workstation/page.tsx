"use client";

import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth, useOrganization } from "@clerk/nextjs";
import { RoleGate } from "@/components/dashboard/RoleProvider";
import { useToast, ToastContainer } from "@/components/Toast";

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";

/* Backend Workflow shape (matches Go struct) */
interface BackendWorkflow {
  id: string;
  name: string;
  description?: string;
  department?: string;
  version: number;
  status: "active" | "inactive" | "draft";
  trigger: { type: string; config?: Record<string, string> };
  nodes: unknown[];
  tags?: string[];
  raw_json?: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

interface BackendInstance {
  id: string;
  workflow_id: string;
  workflow_name?: string;
  status: "pending" | "running" | "waiting" | "completed" | "failed" | "cancelled";
  current_node?: string;
  node_states?: Record<string, { status: string; started_at?: string; completed_at?: string }>;
  audit_log?: Array<{ timestamp: string; node_id?: string; action: string; details?: Record<string, unknown> }>;
  started_at: string;
  completed_at?: string;
}

interface BackendTaskForInstance {
  id: string;
  node_id: string;
  title: string;
  assigned_user?: string;
  assigned_role?: string;
  status: string;
  decision?: string;
  comment?: string;
  completed_at?: string;
}

interface BackendEmployee {
  id: string;
  first_name: string;
  last_name: string;
  email?: string;
}

/* helper - pretty relative time */
function timeAgo(iso: string): string {
  if (!iso) return "unknown";
  const t = new Date(iso).getTime();
  if (isNaN(t)) return "unknown";
  const diff = Date.now() - t;
  if (diff < 0) return "just now";
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}

function formatTaskStatus(status: string): string {
  switch (status) {
    case "pending": return "Pending";
    case "in_progress": return "In Progress";
    case "approve": return "Approved";
    case "approved": return "Approved";
    case "reject": return "Rejected";
    case "rejected": return "Rejected";
    case "clarify": return "Needs Clarification";
    case "clarification_requested": return "Needs Clarification";
    case "complete": return "Completed";
    case "completed": return "Completed";
    default: return status;
  }
}

function formatTaskDecision(status: string, decision?: string): string {
  if (decision) {
    return formatTaskStatus(decision);
  }
  return formatTaskStatus(status);
}

function prettyActionName(action: string): string {
  switch (action) {
    case "instance_started": return "Workflow started";
    case "task_assigned": return "Task assigned";
    case "task_started": return "Task started";
    case "task_action": return "Task decision recorded";
    case "merge_completed": return "Merge completed";
    case "instance_completed": return "Workflow completed";
    case "instance_failed": return "Workflow failed";
    case "condition_evaluated": return "Condition evaluated";
    case "email_sent": return "Email sent";
    default: return action.replaceAll("_", " ");
  }
}

function formatInstanceLabel(instanceID: string): string {
  const trimmed = instanceID.trim();
  if (!trimmed) return "i-unknown";
  return `i-${trimmed.slice(0, 6)}`;
}

function formatEmployeeName(employee?: BackendEmployee): string {
  if (!employee) return "";
  const name = `${employee.first_name || ""} ${employee.last_name || ""}`.trim();
  return name || employee.email || employee.id;
}

const WF_API = process.env.NEXT_PUBLIC_WF_API || "http://localhost:8085";

export default function WorkstationPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { toasts, showToast, dismissToast } = useToast();
  const toastShownRef = useRef(false);
  const { getToken } = useAuth();
  const { organization } = useOrganization();

  const orgApiBase = `${WF_API}/api/orgs/${organization?.id}`;

  const authFetch = useCallback(async (input: string, init: RequestInit = {}): Promise<Response> => {
    const token = await getToken();
    return fetch(input, {
      ...init,
      headers: {
        ...(init.headers ?? {}),
        Authorization: `Bearer ${token}`,
      },
    });
  }, [getToken]);

  /* Show toast passed via URL query (from workflow-builder redirects) */
  useEffect(() => {
    if (toastShownRef.current) return;
    const msg = searchParams.get("toast");
    const rawType = searchParams.get("toastType") ?? "success";
    const validTypes = ["success", "error", "warning", "info"] as const;
    const type: typeof validTypes[number] = validTypes.includes(rawType as any) ? (rawType as any) : "success";
    if (msg) {
      toastShownRef.current = true;
      showToast(msg, type);
      // Clean params from URL without reloading
      const clean = new URL(window.location.href);
      clean.searchParams.delete("toast");
      clean.searchParams.delete("toastType");
      window.history.replaceState(null, "", clean.pathname + (clean.search || ""));
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  /* Workflow list from backend */
  const [workflows, setWorkflows] = useState<BackendWorkflow[]>([]);
  const [instances, setInstances] = useState<BackendInstance[]>([]);
  const [employees, setEmployees] = useState<BackendEmployee[]>([]);
  const [selectedInstance, setSelectedInstance] = useState<BackendInstance | null>(null);
  const [selectedInstanceTasks, setSelectedInstanceTasks] = useState<BackendTaskForInstance[]>([]);
  const [instanceDrawerLoading, setInstanceDrawerLoading] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const instanceRequestControllerRef = useRef<AbortController | null>(null);
  const instanceRequestIdRef = useRef(0);

  const closeInstanceDrawer = useCallback(() => {
    instanceRequestIdRef.current += 1;
    instanceRequestControllerRef.current?.abort();
    instanceRequestControllerRef.current = null;
    setSelectedInstance(null);
    setSelectedInstanceTasks([]);
    setInstanceDrawerLoading(false);
  }, []);

  const openInstanceDrawer = useCallback(async (instance: BackendInstance) => {
    if (!organization?.id) return;
    instanceRequestIdRef.current += 1;
    const requestID = instanceRequestIdRef.current;
    instanceRequestControllerRef.current?.abort();
    const controller = new AbortController();
    instanceRequestControllerRef.current = controller;
    setSelectedInstance(instance);
    setSelectedInstanceTasks([]);
    setInstanceDrawerLoading(true);
    try {
      const [instRes, taskRes] = await Promise.all([
        authFetch(`${orgApiBase}/instances/${instance.id}`, { signal: controller.signal }),
        authFetch(`${orgApiBase}/tasks?instance_id=${encodeURIComponent(instance.id)}`, { signal: controller.signal }),
      ]);
      if (!instRes.ok || !taskRes.ok) throw new Error(`HTTP ${instRes.status}/${taskRes.status}`);
      const [instDetail, taskData] = await Promise.all([
        instRes.json() as Promise<BackendInstance>,
        taskRes.json() as Promise<BackendTaskForInstance[]>,
      ]);
      if (instanceRequestIdRef.current !== requestID) return;
      setSelectedInstance(instDetail);
      setSelectedInstanceTasks(taskData ?? []);
    } catch (err) {
      if (controller.signal.aborted) {
        return;
      }
      console.error("Failed to load instance details", err);
      if (instanceRequestIdRef.current === requestID) {
        setSelectedInstanceTasks([]);
      }
    } finally {
      if (instanceRequestIdRef.current === requestID) {
        setInstanceDrawerLoading(false);
      }
    }
  }, [organization?.id, orgApiBase, authFetch]);

  useEffect(() => {
    return () => {
      instanceRequestControllerRef.current?.abort();
    };
  }, []);

  const employeeNameByID = useMemo(() => {
    const map = new Map<string, string>();
    for (const employee of employees) {
      map.set(employee.id, formatEmployeeName(employee));
    }
    return map;
  }, [employees]);

  const displayUserName = useCallback((userID?: string) => {
    const trimmed = (userID || "").trim();
    if (!trimmed) return "";
    return employeeNameByID.get(trimmed) || trimmed;
  }, [employeeNameByID]);

  const fetchWorkflows = useCallback(async () => {
    if (!organization?.id) return;
    setLoading(true);
    setError(null);
    try {
      const [wfRes, instRes, employeeRes] = await Promise.all([
        authFetch(`${orgApiBase}/workflows`),
        authFetch(`${orgApiBase}/instances`),
        authFetch(`${AUTH_API}/api/orgs/${organization.id}/employees`),
      ]);
      if (!wfRes.ok || !instRes.ok) throw new Error(`HTTP ${wfRes.status}/${instRes.status}`);

      const [wfData, instData, employeeData] = await Promise.all([
        wfRes.json() as Promise<BackendWorkflow[]>,
        instRes.json() as Promise<BackendInstance[]>,
        employeeRes.ok ? employeeRes.json() as Promise<BackendEmployee[]> : Promise.resolve([]),
      ]);

      setWorkflows(wfData ?? []);
      setInstances(instData ?? []);
      setEmployees(employeeData ?? []);
    } catch (err: any) {
      console.error("Failed to load workflows:", err);
      setError(err.message || "Could not reach workflow service");
    } finally {
      setLoading(false);
    }
  }, [organization?.id, orgApiBase, authFetch]);

  useEffect(() => { fetchWorkflows(); }, [fetchWorkflows]);

  /* 3-dot dropdown: open id + fixed position */
  const [openMenuId, setOpenMenuId] = useState<string | null>(null);
  const [menuPos, setMenuPos] = useState<{ top: number; left: number } | null>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const openMenu = useCallback((e: React.MouseEvent<HTMLButtonElement>, id: string) => {
    e.stopPropagation();
    if (openMenuId === id) { setOpenMenuId(null); return; }
    const rect = e.currentTarget.getBoundingClientRect();
    const wf = workflows.find((w) => w.id === id);
    const isDraft = wf?.status === "draft";
    const itemCount = [
      true,
      wf?.status === "active",
      !isDraft,
      true,
    ].filter(Boolean).length;
    const dropdownHeight = itemCount * 38 + 16;
    const spaceBelow = window.innerHeight - rect.bottom;
    const top = spaceBelow < dropdownHeight + 8 ? rect.top - dropdownHeight - 4 : rect.bottom + 4;
    setMenuPos({ top, left: rect.right - 160 });
    setOpenMenuId(id);
  }, [openMenuId, workflows]);

  useEffect(() => {
    if (!openMenuId) return;
    function close(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpenMenuId(null);
      }
    }
    function handleScroll() {
      setOpenMenuId(null);
    }
    document.addEventListener("mousedown", close);
    document.addEventListener("scroll", handleScroll, { once: true });
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("scroll", handleScroll);
    };
  }, [openMenuId]);

  /* Delete confirmation dialog */
  const [deleteTarget, setDeleteTarget] = useState<{ id: string; name: string } | null>(null);
  const [deleteInput, setDeleteInput] = useState("");
  const deleteInputRef = useRef<HTMLInputElement>(null);

  const openDeleteDialog = useCallback((wf: BackendWorkflow) => {
    setOpenMenuId(null);
    setDeleteInput("");
    setDeleteTarget({ id: wf.id, name: wf.name });
    setTimeout(() => deleteInputRef.current?.focus(), 80);
  }, []);

  const confirmDelete = useCallback(async () => {
    if (!deleteTarget || deleteInput.toLowerCase() !== "delete permanently") return;
    try {
      const res = await authFetch(`${orgApiBase}/workflows/${deleteTarget.id}`, { method: "DELETE" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setWorkflows((prev) => prev.filter((w) => w.id !== deleteTarget.id));
      showToast(`"${deleteTarget.name}" deleted successfully.`, "success");
    } catch (err: any) {
      showToast("Delete failed: " + (err.message || err), "error");
    }
    setDeleteTarget(null);
    setDeleteInput("");
  }, [deleteTarget, deleteInput, orgApiBase, authFetch, showToast]);

  /* Inactive confirmation dialog */
  const [inactiveTarget, setInactiveTarget] = useState<{ id: string; name: string } | null>(null);
  const [inactiveInput, setInactiveInput] = useState("");
  const inactiveInputRef = useRef<HTMLInputElement>(null);

  const openInactiveDialog = useCallback((wf: BackendWorkflow) => {
    setOpenMenuId(null);
    setInactiveInput("");
    setInactiveTarget({ id: wf.id, name: wf.name });
    setTimeout(() => inactiveInputRef.current?.focus(), 80);
  }, []);

  const confirmInactive = useCallback(async () => {
    if (!inactiveTarget || inactiveInput.toLowerCase() !== "inactive") return;
    try {
      const existing = workflows.find((w) => w.id === inactiveTarget.id);
      if (!existing) throw new Error("Workflow not found");
      const minimal = {
        id: existing.id, name: existing.name, description: existing.description,
        department: existing.department, version: existing.version,
        trigger: existing.trigger, nodes: existing.nodes,
        tags: existing.tags, raw_json: existing.raw_json, status: "inactive" as const,
      };
      const res = await authFetch(`${orgApiBase}/workflows/${inactiveTarget.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(minimal),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setWorkflows((prev) => prev.map((w) => w.id === inactiveTarget.id ? { ...w, status: "inactive" } : w));
      showToast(`"${inactiveTarget.name}" set to inactive.`, "success");
    } catch (err: any) {
      showToast("Failed to set inactive: " + (err.message || err), "error");
    }
    setInactiveTarget(null);
    setInactiveInput("");
  }, [inactiveTarget, inactiveInput, workflows, orgApiBase, authFetch, showToast]);

  const handleSetActive = useCallback(async (wf: BackendWorkflow) => {
    setOpenMenuId(null);
    try {
      const minimal = {
        id: wf.id, name: wf.name, description: wf.description,
        department: wf.department, version: wf.version,
        trigger: wf.trigger, nodes: wf.nodes,
        tags: wf.tags, raw_json: wf.raw_json, status: "active" as const,
      };
      const res = await authFetch(`${orgApiBase}/workflows/${wf.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(minimal),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setWorkflows((prev) => prev.map((w) => w.id === wf.id ? { ...w, status: "active" } : w));
      showToast(`"${wf.name}" set to active.`, "success");
    } catch (err: any) {
      showToast("Failed to set active: " + (err.message || err), "error");
    }
  }, [orgApiBase, authFetch, showToast]);

  /* Active confirmation dialog */
  const [activeTarget, setActiveTarget] = useState<BackendWorkflow | null>(null);
  const [activeInput, setActiveInput] = useState("");
  const activeInputRef = useRef<HTMLInputElement>(null);

  /* Trigger confirmation dialog */
  const [triggerTarget, setTriggerTarget] = useState<BackendWorkflow | null>(null);
  const [triggeringWorkflow, setTriggeringWorkflow] = useState(false);

  const openTriggerDialog = useCallback((wf: BackendWorkflow) => {
    setOpenMenuId(null);
    setTriggerTarget(wf);
  }, []);

  const confirmTrigger = useCallback(async () => {
    if (!triggerTarget) return;
    setTriggeringWorkflow(true);
    try {
      const res = await authFetch(`${orgApiBase}/instances`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ workflow_id: triggerTarget.id, data: {} }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data = await res.json();
      showToast(`"${triggerTarget.name}" triggered successfully. Instance: ${data.instance_id}`, "success");
      setTimeout(() => fetchWorkflows(), 500);
    } catch (err: any) {
      showToast("Failed to trigger workflow: " + (err.message || err), "error");
    } finally {
      setTriggerTarget(null);
      setTriggeringWorkflow(false);
    }
  }, [triggerTarget, orgApiBase, authFetch, showToast, fetchWorkflows]);

  const openActiveDialog = useCallback((wf: BackendWorkflow) => {
    setOpenMenuId(null);
    setActiveInput("");
    setActiveTarget(wf);
    setTimeout(() => activeInputRef.current?.focus(), 80);
  }, []);

  const confirmActive = useCallback(async () => {
    if (!activeTarget || activeInput.toLowerCase() !== "active") return;
    await handleSetActive(activeTarget);
    setActiveTarget(null);
    setActiveInput("");
  }, [activeTarget, activeInput, handleSetActive]);

  /* Filters */
  const [filterDept, setFilterDept] = useState("all");
  const [filterStatus, setFilterStatus] = useState("all");
  const [filterTag, setFilterTag] = useState("all");

  /* Unique filter options from live data */
  const deptOptions = useMemo(() => {
    const set = new Set(workflows.map((w) => w.department).filter(Boolean) as string[]);
    return Array.from(set).sort();
  }, [workflows]);

  const tagOptions = useMemo(() => {
    const set = new Set(workflows.flatMap((w) => w.tags ?? []));
    return Array.from(set).sort();
  }, [workflows]);

  /* Filtered list */
  const filtered = useMemo(() => {
    return workflows.filter((wf) => {
      if (filterDept !== "all" && wf.department !== filterDept) return false;
      if (filterStatus !== "all" && wf.status !== filterStatus) return false;
      if (filterTag !== "all" && !(wf.tags ?? []).includes(filterTag)) return false;
      return true;
    });
  }, [workflows, filterDept, filterStatus, filterTag]);

  return (
    <>
    <RoleGate
      allowed={["admin"]}
      fallback={
        <div className="dashboard-page">
          <div className="empty-state">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor" width="64" height="64">
              <path strokeLinecap="round" strokeLinejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z" />
            </svg>
            <h3>Access Restricted</h3>
            <p>The Workstation view is only available to Admins.</p>
          </div>
        </div>
      }
    >
      <div className="dashboard-page">
        {/* Workflows section */}
        <section className="dashboard-section">
          <div className="section-header" style={{ flexWrap: "wrap", gap: 10 }}>
            <h3 className="section-title">Workflows</h3>
            <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap", marginLeft: "auto" }}>
              <select className="filter-select" value={filterDept} onChange={(e) => setFilterDept(e.target.value)}>
                <option value="all">All Departments</option>
                {deptOptions.map((d) => (<option key={d} value={d}>{d}</option>))}
              </select>
              <select className="filter-select" value={filterStatus} onChange={(e) => setFilterStatus(e.target.value)}>
                <option value="all">All Statuses</option>
                <option value="active">Active</option>
                <option value="inactive">Inactive</option>
                <option value="draft">Draft</option>
              </select>
              {tagOptions.length > 0 && (
                <select className="filter-select" value={filterTag} onChange={(e) => setFilterTag(e.target.value)}>
                  <option value="all">All Tags</option>
                  {tagOptions.map((t) => (<option key={t} value={t}>{t}</option>))}
                </select>
              )}
              {(filterDept !== "all" || filterStatus !== "all" || filterTag !== "all") && (
                <button className="action-btn action-btn-outline action-btn-sm" onClick={() => { setFilterDept("all"); setFilterStatus("all"); setFilterTag("all"); }}>
                  Clear
                </button>
              )}
              <button className="action-btn action-btn-outline action-btn-sm" onClick={fetchWorkflows}>Refresh</button>
              <Link href="/workflow-builder" className="action-btn action-btn-primary action-btn-sm">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="18" height="18">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                </svg>
                New Workflow
              </Link>
            </div>
          </div>

          {loading && (
            <div className="empty-state" style={{ padding: "40px 0" }}>
              <p className="table-muted">Loading workflows&hellip;</p>
            </div>
          )}
          {error && !loading && (
            <div className="empty-state" style={{ padding: "40px 0" }}>
              <p style={{ color: "#ef4444" }}>&#9888; {error}</p>
              <button className="action-btn action-btn-outline action-btn-sm" style={{ marginTop: 12 }} onClick={fetchWorkflows}>Retry</button>
            </div>
          )}
          {!loading && !error && workflows.length === 0 && (
            <div className="empty-state" style={{ padding: "40px 0" }}>
              <p className="table-muted">No workflows yet. Create your first one!</p>
            </div>
          )}
          {!loading && !error && workflows.length > 0 && filtered.length === 0 && (
            <div className="empty-state" style={{ padding: "40px 0" }}>
              <p className="table-muted">No workflows match the selected filters.</p>
            </div>
          )}
          {!loading && !error && filtered.length > 0 && (
            <div className="table-container" style={{ maxHeight: `${5 * 48 + 72}px`, overflowY: "auto" }}>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Name</th><th>Department</th><th>Status</th><th>Trigger</th><th>Tags</th><th>Version</th><th>Updated</th><th style={{ width: 48 }}></th>
                  </tr>
                </thead>
                <tbody>
                  {filtered.map((wf) => (
                    <tr key={wf.id}>
                      <td className="font-medium">{wf.name}</td>
                      <td>{wf.department || <span className="table-muted">&mdash;</span>}</td>
                      <td>
                        <span className={`status-dot ${wf.status === "active" ? "active" : wf.status === "draft" ? "draft" : "inactive"}`}>{wf.status}</span>
                      </td>
                      <td><span className="role-badge">{wf.trigger?.type || "manual"}</span></td>
                      <td>
                        {wf.tags && wf.tags.length > 0
                          ? wf.tags.map((t) => (
                              <span key={t} className="role-badge" style={{ marginRight: 4, cursor: "pointer", opacity: filterTag === t ? 1 : 0.75 }} onClick={() => setFilterTag(filterTag === t ? "all" : t)}>{t}</span>
                            ))
                          : <span className="table-muted">&mdash;</span>}
                      </td>
                      <td className="table-center">v{wf.version}</td>
                      <td className="table-muted">{wf.updated_at ? timeAgo(wf.updated_at) : "\u2014"}</td>
                      <td>
                        <button className="wf-row-menu-btn" onClick={(e) => openMenu(e, wf.id)} aria-label="Actions">
                          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="18" height="18">
                            <circle cx="12" cy="5" r="1.5" fill="currentColor" />
                            <circle cx="12" cy="12" r="1.5" fill="currentColor" />
                            <circle cx="12" cy="19" r="1.5" fill="currentColor" />
                          </svg>
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>

        <section className="dashboard-section" style={{ marginTop: 20 }}>
          <div className="section-header" style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <h3 className="section-title">Workflow Instances</h3>
          </div>

          {!loading && !error && instances.length === 0 && (
            <div className="empty-state" style={{ padding: "28px 0" }}>
              <p className="table-muted">No workflow instances yet.</p>
            </div>
          )}

          {!loading && !error && instances.length > 0 && (
            <div className="table-container" style={{ maxHeight: `${10 * 48 + 72}px`, overflowY: "auto" }}>
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Instance</th><th>Workflow</th><th>Status</th><th>Current Node</th><th>Started</th><th>Completed</th>
                  </tr>
                </thead>
                <tbody>
                  {instances.map((inst) => (
                    <tr key={inst.id}>
                      <td className="font-medium">
                        <button
                          type="button"
                          onClick={() => openInstanceDrawer(inst)}
                          aria-label={`Open workflow instance ${formatInstanceLabel(inst.id)}`}
                          style={{ background: "none", border: "none", padding: 0, color: "var(--accent)", cursor: "pointer", font: "inherit", textDecoration: "underline" }}
                        >
                          {formatInstanceLabel(inst.id)}
                        </button>
                      </td>
                      <td>{inst.workflow_name || inst.workflow_id}</td>
                      <td><span className={`status-dot ${inst.status === "completed" ? "active" : inst.status === "failed" ? "inactive" : "draft"}`}>{inst.status}</span></td>
                      <td>{inst.current_node || <span className="table-muted">&mdash;</span>}</td>
                      <td className="table-muted">{timeAgo(inst.started_at)}</td>
                      <td className="table-muted">{inst.completed_at ? timeAgo(inst.completed_at) : "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </section>
      </div>

      {selectedInstance && (
        <div className="modal-overlay" onClick={closeInstanceDrawer}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 980, width: "95vw", maxHeight: "86vh", overflow: "hidden" }}>
            <div className="modal-header">
              <h3 className="modal-title">Instance Timeline - {formatInstanceLabel(selectedInstance.id)}</h3>
              <button className="modal-close" onClick={closeInstanceDrawer} aria-label="Close timeline">&times;</button>
            </div>
            <div className="modal-body" style={{ maxHeight: "68vh", overflowY: "auto" }}>
              {instanceDrawerLoading ? (
                <p className="table-muted">Loading instance timeline...</p>
              ) : (
                <>
                  <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: 12, marginBottom: 18 }}>
                    <div className="overview-stat-card"><span className="overview-stat-label">Status</span><span className="overview-stat-value" style={{ fontSize: "1rem" }}>{selectedInstance.status}</span></div>
                    <div className="overview-stat-card"><span className="overview-stat-label">Current Step</span><span className="overview-stat-value" style={{ fontSize: "1rem" }}>{selectedInstance.current_node || "-"}</span></div>
                    <div className="overview-stat-card"><span className="overview-stat-label">Started</span><span className="overview-stat-value" style={{ fontSize: "1rem" }}>{timeAgo(selectedInstance.started_at)}</span></div>
                    <div className="overview-stat-card"><span className="overview-stat-label">Completed</span><span className="overview-stat-value" style={{ fontSize: "1rem" }}>{selectedInstance.completed_at ? timeAgo(selectedInstance.completed_at) : "-"}</span></div>
                  </div>

                  <h4 className="section-title" style={{ marginBottom: 8 }}>Task Decisions</h4>
                  <div className="table-container" style={{ marginBottom: 16 }}>
                    <table className="data-table">
                      <thead><tr><th>Step</th><th>Assigned To</th><th>Handled By</th><th>Status</th><th>Comment</th><th>Updated</th></tr></thead>
                      <tbody>
                        {selectedInstanceTasks.length === 0 ? (
                          <tr><td colSpan={6} className="table-muted">No task records yet</td></tr>
                        ) : selectedInstanceTasks.map((t) => (
                          (() => {
                            const related = (selectedInstance.audit_log || []).filter((a) => {
                              const details = a.details as Record<string, unknown> | undefined;
                              return details && String(details.task_id || "") === t.id;
                            });
                            const latestAction = related
                              .slice()
                              .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())[0];
                            const latestDetails = (latestAction?.details || {}) as Record<string, unknown>;
                            return (
                              <tr key={t.id}>
                                <td>{t.title} <span className="table-muted">({t.node_id})</span></td>
                                <td>{t.assigned_user ? displayUserName(t.assigned_user) : (t.assigned_role || "Role Queue")}</td>
                                <td>{displayUserName(String(latestDetails.actor || "")) || "-"}</td>
                                <td>{formatTaskDecision(t.status, t.decision)}</td>
                                <td>{t.comment || String(latestDetails.comment || "-")}</td>
                                <td>{t.completed_at ? timeAgo(t.completed_at) : (latestAction?.timestamp ? timeAgo(latestAction.timestamp) : "-")}</td>
                              </tr>
                            );
                          })()
                        ))}
                      </tbody>
                    </table>
                  </div>

                  <h4 className="section-title" style={{ marginBottom: 8 }}>Execution Timeline</h4>
                  <div style={{ display: "grid", gap: 10 }}>
                    {selectedInstance.audit_log && selectedInstance.audit_log.length > 0 ? selectedInstance.audit_log
                      .slice()
                      .sort((a, b) => new Date(b.timestamp).getTime() - new Date(a.timestamp).getTime())
                      .map((entry, idx) => {
                        const details = (entry.details || {}) as Record<string, unknown>;
                        const pieces: string[] = [];
                        if (details.assigned_role) pieces.push(`Role: ${String(details.assigned_role)}`);
                        if (details.assigned_user) pieces.push(`Assigned user: ${displayUserName(String(details.assigned_user)) || String(details.assigned_user)}`);
                        if (details.actor) pieces.push(`Handled by: ${displayUserName(String(details.actor)) || String(details.actor)}`);
                        if (details.action) pieces.push(`Decision: ${String(details.action)}`);
                        if (details.comment) pieces.push(`Comment: ${String(details.comment)}`);
                        if (details.reason) pieces.push(`Reason: ${String(details.reason)}`);
                        return (
                          <div key={`${entry.timestamp}-${idx}`} style={{ border: "1px solid var(--border-color)", borderRadius: 10, padding: 10, background: "var(--surface-alt)" }}>
                            <div style={{ display: "flex", justifyContent: "space-between", gap: 8 }}>
                              <strong>{prettyActionName(entry.action)}</strong>
                              <span className="table-muted">{timeAgo(entry.timestamp)}</span>
                            </div>
                            <div className="table-muted" style={{ marginTop: 4 }}>Step: {entry.node_id || "-"}</div>
                            {pieces.length > 0 && (
                              <div style={{ marginTop: 8, display: "grid", gap: 4 }}>
                                {pieces.map((line, i) => (
                                  <div key={`${entry.timestamp}-${i}`} style={{ fontSize: "0.88rem" }}>{line}</div>
                                ))}
                              </div>
                            )}
                          </div>
                        );
                      }) : (
                      <p className="table-muted">No audit timeline yet.</p>
                    )}
                  </div>

                  {(() => {
                    const wf = workflows.find((w) => w.id === selectedInstance.workflow_id);
                    const nodes = Array.isArray(wf?.nodes) ? (wf?.nodes as Array<{ id?: string; title?: string }>) : [];
                    const done = new Set(Object.entries(selectedInstance.node_states || {}).filter(([, s]) => s?.status === "completed").map(([id]) => id));
                    const remaining = nodes.filter((n) => n.id && !done.has(n.id)).map((n) => n.title || n.id);
                    return (
                      <>
                        <h4 className="section-title" style={{ margin: "16px 0 8px" }}>Steps Left</h4>
                        {remaining.length === 0 ? <p className="table-muted">No remaining steps.</p> : (
                          <div style={{ display: "flex", flexWrap: "wrap", gap: 8 }}>
                            {remaining.map((name, i) => <span key={`${name}-${i}`} className="role-badge">{name}</span>)}
                          </div>
                        )}
                      </>
                    );
                  })()}
                </>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Fixed-position dropdown */}
      {openMenuId && menuPos && (() => {
        const openMenuWf = workflows.find((w) => w.id === openMenuId);
        const isDraft = openMenuWf?.status === "draft";
        const isInactive = openMenuWf?.status !== "active";
        return (
          <div ref={dropdownRef} className="wf-row-dropdown" style={{ position: "fixed", top: menuPos.top, left: menuPos.left, zIndex: 9999 }}>
            <button className="wf-row-dropdown-item" onClick={() => { setOpenMenuId(null); router.push(`/workflow-builder?id=${openMenuId}`); }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                <path strokeLinecap="round" strokeLinejoin="round" d="m16.862 4.487 1.687-1.688a1.875 1.875 0 1 1 2.652 2.652L10.582 16.07a4.5 4.5 0 0 1-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 0 1 1.13-1.897l8.932-8.931Zm0 0L19.5 7.125M18 14v4.75A2.25 2.25 0 0 1 15.75 21H5.25A2.25 2.25 0 0 1 3 18.75V8.25A2.25 2.25 0 0 1 5.25 6H10" />
              </svg>
              Modify
            </button>
            {openMenuWf?.status === "active" && (
              <button className="wf-row-dropdown-item" style={{ color: "#06b6d4" }} onClick={() => { if (openMenuWf) openTriggerDialog(openMenuWf); }}>
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3" />
                </svg>
                Trigger
              </button>
            )}
            {!isDraft && (isInactive ? (
              <button className="wf-row-dropdown-item" style={{ color: "#22c55e" }} onClick={() => { if (openMenuWf) openActiveDialog(openMenuWf); }}>
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
                </svg>
                Set Active
              </button>
            ) : (
              <button className="wf-row-dropdown-item wf-row-dropdown-danger" style={{ color: "#f59e0b" }} onClick={() => { if (openMenuWf) openInactiveDialog(openMenuWf); }}>
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M18.364 18.364A9 9 0 0 0 5.636 5.636m12.728 12.728A9 9 0 0 1 5.636 5.636m12.728 12.728L5.636 5.636" />
                </svg>
                Set Inactive
              </button>
            ))}
            <button className="wf-row-dropdown-item wf-row-dropdown-danger" onClick={() => { if (openMenuWf) openDeleteDialog(openMenuWf); }}>
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                <path strokeLinecap="round" strokeLinejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" />
              </svg>
              Delete
            </button>
          </div>
        );
      })()}

      {/* Delete confirmation dialog */}
      {deleteTarget && (
        <div className="modal-overlay" onClick={() => setDeleteTarget(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 420 }}>
            <div className="modal-header">
              <h3 className="modal-title" style={{ color: "#ef4444" }}>Delete Workflow</h3>
              <button className="modal-close" aria-label="Close delete modal" onClick={() => setDeleteTarget(null)}>&times;</button>
            </div>
            <div className="modal-body">
              <p style={{ fontSize: "0.9rem", marginBottom: 16, color: "var(--text-secondary)" }}>
                You are about to permanently delete <strong>&ldquo;{deleteTarget.name}&rdquo;</strong>. This cannot be undone.
              </p>
              <p style={{ fontSize: "0.85rem", marginBottom: 8, color: "var(--text-primary)", fontWeight: 500 }}>
                Type <span style={{ fontFamily: "monospace", background: "var(--surface-alt)", padding: "2px 6px", borderRadius: 4 }}>delete permanently</span> to confirm:
              </p>
              <input
                ref={deleteInputRef}
                className="wf-input"
                value={deleteInput}
                onChange={(e) => setDeleteInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") confirmDelete(); }}
                placeholder="delete permanently"
                autoComplete="off"
                aria-label='Type "delete permanently" to confirm deletion'
              />
            </div>
            <div className="modal-footer">
              <button className="action-btn action-btn-outline" onClick={() => setDeleteTarget(null)}>Cancel</button>
              <button className="action-btn action-btn-danger" disabled={deleteInput.toLowerCase() !== "delete permanently"} onClick={confirmDelete}>Delete Permanently</button>
            </div>
          </div>
        </div>
      )}

      {/* Active confirmation dialog */}
      {activeTarget && (
        <div className="modal-overlay" onClick={() => setActiveTarget(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 420 }}>
            <div className="modal-header">
              <h3 className="modal-title" style={{ color: "#22c55e" }}>Set Workflow Active</h3>
              <button className="modal-close" aria-label="Close set active modal" onClick={() => setActiveTarget(null)}>&times;</button>
            </div>
            <div className="modal-body">
              <p style={{ fontSize: "0.9rem", marginBottom: 16, color: "var(--text-secondary)" }}>
                <strong>&ldquo;{activeTarget.name}&rdquo;</strong> will be marked as active and can be triggered automatically.
              </p>
              <p style={{ fontSize: "0.85rem", marginBottom: 8, color: "var(--text-primary)", fontWeight: 500 }}>
                Type <span style={{ fontFamily: "monospace", background: "var(--surface-alt)", padding: "2px 6px", borderRadius: 4 }}>active</span> to confirm:
              </p>
              <input
                ref={activeInputRef}
                className="wf-input"
                value={activeInput}
                onChange={(e) => setActiveInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") confirmActive(); }}
                placeholder="active"
                autoComplete="off"
                aria-label='Type "active" to confirm setting workflow active'
              />
            </div>
            <div className="modal-footer">
              <button className="action-btn action-btn-outline" onClick={() => setActiveTarget(null)}>Cancel</button>
              <button
                className="action-btn"
                style={{ background: "#22c55e", color: "#fff", border: "none" }}
                disabled={activeInput.toLowerCase() !== "active"}
                onClick={confirmActive}
              >
                Set Active
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Trigger confirmation dialog */}
      {triggerTarget && (
        <div className="modal-overlay" onClick={() => setTriggerTarget(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 420 }}>
            <div className="modal-header">
              <h3 className="modal-title" style={{ color: "#06b6d4" }}>Trigger Workflow</h3>
              <button className="modal-close" aria-label="Close trigger modal" onClick={() => setTriggerTarget(null)}>&times;</button>
            </div>
            <div className="modal-body">
              <p style={{ fontSize: "0.9rem", marginBottom: 16, color: "var(--text-secondary)" }}>
                This will start a new instance of <strong>&ldquo;{triggerTarget.name}&rdquo;</strong> with empty input data.
              </p>
              <p style={{ fontSize: "0.85rem", color: "var(--text-muted)" }}>
                Note: This is a mock trigger for testing. In production, you would provide input data for the workflow.
              </p>
            </div>
            <div className="modal-footer">
              <button className="action-btn action-btn-outline" onClick={() => setTriggerTarget(null)} disabled={triggeringWorkflow}>Cancel</button>
              <button
                className="action-btn"
                style={{ background: "#06b6d4", color: "#fff", border: "none" }}
                disabled={triggeringWorkflow}
                onClick={confirmTrigger}
              >
                {triggeringWorkflow ? "Triggering..." : "Trigger Workflow"}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Inactive confirmation dialog */}
      {inactiveTarget && (
        <div className="modal-overlay" onClick={() => setInactiveTarget(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 420 }}>
            <div className="modal-header">
              <h3 className="modal-title" style={{ color: "#f59e0b" }}>Set Workflow Inactive</h3>
              <button className="modal-close" aria-label="Close set inactive modal" onClick={() => setInactiveTarget(null)}>&times;</button>
            </div>
            <div className="modal-body">
              <p style={{ fontSize: "0.9rem", marginBottom: 16, color: "var(--text-secondary)" }}>
                <strong>&ldquo;{inactiveTarget.name}&rdquo;</strong> will be marked as inactive and will no longer be triggered automatically.
              </p>
              <p style={{ fontSize: "0.85rem", marginBottom: 8, color: "var(--text-primary)", fontWeight: 500 }}>
                Type <span style={{ fontFamily: "monospace", background: "var(--surface-alt)", padding: "2px 6px", borderRadius: 4 }}>inactive</span> to confirm:
              </p>
              <input
                ref={inactiveInputRef}
                className="wf-input"
                value={inactiveInput}
                onChange={(e) => setInactiveInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === "Enter") confirmInactive(); }}
                placeholder="inactive"
                autoComplete="off"
                aria-label='Type "inactive" to confirm setting workflow inactive'
              />
            </div>
            <div className="modal-footer">
              <button className="action-btn action-btn-outline" onClick={() => setInactiveTarget(null)}>Cancel</button>
              <button
                className="action-btn"
                style={{ background: "#f59e0b", color: "#fff", border: "none" }}
                disabled={inactiveInput.toLowerCase() !== "inactive"}
                onClick={confirmInactive}
              >
                Set Inactive
              </button>
            </div>
          </div>
        </div>
      )}
    </RoleGate>
    <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </>
  );
}
