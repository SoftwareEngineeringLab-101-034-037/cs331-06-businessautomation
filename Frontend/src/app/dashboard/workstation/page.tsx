"use client";

import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { MOCK_DEPARTMENTS } from "@/lib/mock-data";
import { RoleGate } from "@/components/dashboard/RoleProvider";
import { useToast, ToastContainer } from "@/components/Toast";

/* Backend Workflow shape (matches Go struct) */
interface BackendWorkflow {
  id: string;
  name: string;
  description?: string;
  department?: string;
  version: number;
  status: string;
  trigger: { type: string; config?: Record<string, string> };
  nodes: unknown[];
  tags?: string[];
  raw_json?: string;
  created_by?: string;
  created_at: string;
  updated_at: string;
}

/* helper - pretty relative time */
function timeAgo(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return "just now";
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}

const WF_API = "http://localhost:8085";

export default function WorkstationPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { toasts, showToast, dismissToast } = useToast();
  const toastShownRef = useRef(false);

  /* Show toast passed via URL query (from workflow-builder redirects) */
  useEffect(() => {
    if (toastShownRef.current) return;
    const msg = searchParams.get("toast");
    const type = (searchParams.get("toastType") ?? "success") as "success" | "error" | "warning" | "info";
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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchWorkflows = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await fetch(`${WF_API}/workflows`);
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      const data: BackendWorkflow[] = await res.json();
      setWorkflows(data);
    } catch (err: any) {
      console.error("Failed to load workflows:", err);
      setError(err.message || "Could not reach workflow service");
    } finally {
      setLoading(false);
    }
  }, []);

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
    // 2 items (draft) or 3 items (non-draft), each ~38px + 16px padding
    const itemCount = isDraft ? 2 : 3;
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
    document.addEventListener("mousedown", close);
    document.addEventListener("scroll", () => setOpenMenuId(null), { once: true });
    return () => document.removeEventListener("mousedown", close);
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
      const res = await fetch(`${WF_API}/workflows/${deleteTarget.id}`, { method: "DELETE" });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setWorkflows((prev) => prev.filter((w) => w.id !== deleteTarget.id));
      showToast(`"${deleteTarget.name}" deleted successfully.`, "success");
    } catch (err: any) {
      showToast("Delete failed: " + (err.message || err), "error");
    }
    setDeleteTarget(null);
    setDeleteInput("");
  }, [deleteTarget, deleteInput, showToast]);

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
      const res = await fetch(`${WF_API}/workflows/${inactiveTarget.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...existing, status: "inactive" }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setWorkflows((prev) => prev.map((w) => w.id === inactiveTarget.id ? { ...w, status: "inactive" } : w));
      showToast(`"${inactiveTarget.name}" set to inactive.`, "success");
    } catch (err: any) {
      showToast("Failed to set inactive: " + (err.message || err), "error");
    }
    setInactiveTarget(null);
    setInactiveInput("");
  }, [inactiveTarget, inactiveInput, workflows, showToast]);

  const handleSetActive = useCallback(async (wf: BackendWorkflow) => {
    setOpenMenuId(null);
    try {
      const res = await fetch(`${WF_API}/workflows/${wf.id}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ ...wf, status: "active" }),
      });
      if (!res.ok) throw new Error(`HTTP ${res.status}`);
      setWorkflows((prev) => prev.map((w) => w.id === wf.id ? { ...w, status: "active" } : w));
      showToast(`"${wf.name}" set to active.`, "success");
    } catch (err: any) {
      showToast("Failed to set active: " + (err.message || err), "error");
    }
  }, [showToast]);

  /* Active confirmation dialog */
  const [activeTarget, setActiveTarget] = useState<BackendWorkflow | null>(null);
  const [activeInput, setActiveInput] = useState("");
  const activeInputRef = useRef<HTMLInputElement>(null);

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

  /* Derived KPIs */
  const totalWorkflows = workflows.length;
  const activeWorkflows = workflows.filter((w) => w.status === "active").length;
  const inactiveWorkflows = workflows.filter((w) => w.status === "inactive").length;
  const draftWorkflows = workflows.filter((w) => w.status === "draft").length;
  const totalDepartments = MOCK_DEPARTMENTS.length;

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

        {/* Page header */}
        <div className="page-header">
          <div>
            <h2 className="page-title">Organisation Workstation</h2>
            <p className="page-subtitle">Manage and monitor all workflows across your organisation</p>
          </div>
          <Link href="/workflow-builder" className="action-btn action-btn-primary">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="18" height="18">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
            </svg>
            Create Workflow
          </Link>
        </div>

        {/* Compact stat row */}
        <div className="overview-stats" style={{ marginBottom: 28, flexWrap: "wrap" }}>
          <div className="overview-stat-card">
            <span className="overview-stat-value">{totalDepartments}</span>
            <span className="overview-stat-label">Departments</span>
          </div>
          <div className="overview-stat-card">
            <span className="overview-stat-value">{totalWorkflows}</span>
            <span className="overview-stat-label">Total Workflows</span>
          </div>
          <div className="overview-stat-card">
            <span className="overview-stat-value" style={{ color: "#22c55e" }}>{activeWorkflows}</span>
            <span className="overview-stat-label">Active</span>
          </div>
          <div className="overview-stat-card">
            <span className="overview-stat-value" style={{ color: "var(--text-muted)" }}>{inactiveWorkflows}</span>
            <span className="overview-stat-label">Inactive</span>
          </div>
          <div className="overview-stat-card">
            <span className="overview-stat-value" style={{ color: "#f59e0b" }}>{draftWorkflows}</span>
            <span className="overview-stat-label">Drafts</span>
          </div>
        </div>

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
            <div className="table-container">
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
                        <span className={`status-dot ${wf.status === "active" ? "active" : "inactive"}`}>{wf.status}</span>
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
      </div>

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
              <button className="modal-close" onClick={() => setDeleteTarget(null)}>&times;</button>
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
              <button className="modal-close" onClick={() => setActiveTarget(null)}>&times;</button>
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

      {/* Inactive confirmation dialog */}
      {inactiveTarget && (
        <div className="modal-overlay" onClick={() => setInactiveTarget(null)}>
          <div className="modal-panel" onClick={(e) => e.stopPropagation()} style={{ maxWidth: 420 }}>
            <div className="modal-header">
              <h3 className="modal-title" style={{ color: "#f59e0b" }}>Set Workflow Inactive</h3>
              <button className="modal-close" onClick={() => setInactiveTarget(null)}>&times;</button>
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
