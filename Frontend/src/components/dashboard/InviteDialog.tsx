"use client";

import { useState, useRef, useEffect, useCallback } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";

type InviteTab = "single" | "bulk";

interface InviteDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onResult: (message: string, type: "success" | "error") => void;
}

interface SingleFormData {
  email: string;
  first_name: string;
  last_name: string;
  department: string;
  roles: string[];
  job_title: string;
}

interface BulkResult {
  total_rows: number;
  successful: number;
  failed: number;
  errors: { row: number; email: string; message: string }[];
}

interface Department {
  id: string;
  name: string;
  description?: string;
}

interface Role {
  id: string;
  name: string;
  description?: string;
}

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";

const INITIAL_FORM: SingleFormData = {
  email: "",
  first_name: "",
  last_name: "",
  department: "",
  roles: [],
  job_title: "",
};

export default function InviteDialog({ isOpen, onClose, onResult }: InviteDialogProps) {
  const [tab, setTab] = useState<InviteTab>("single");
  const [form, setForm] = useState<SingleFormData>(INITIAL_FORM);
  const [file, setFile] = useState<File | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const [loading, setLoading] = useState(false);
  const [bulkResult, setBulkResult] = useState<BulkResult | null>(null);
  const [departments, setDepartments] = useState<Department[]>([]);
  const [deptsLoading, setDeptsLoading] = useState(false);
  const [roles, setRoles] = useState<Role[]>([]);
  const [rolesLoading, setRolesLoading] = useState(false);
  const [roleToAdd, setRoleToAdd] = useState("");

  const { getToken } = useAuth();
  const { organization } = useOrganization();

  const dialogRef = useRef<HTMLDivElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Fetch departments and roles when dialog opens
  useEffect(() => {
    if (!isOpen || !organization?.id) return;

    setForm(INITIAL_FORM);
    setFile(null);
    setBulkResult(null);
    setLoading(false);
    setDragOver(false);
    setRoleToAdd("");

    const fetchDeptsAndRoles = async () => {
      setDeptsLoading(true);
      setRolesLoading(true);
      setDepartments([]);
      setRoles([]);
      try {
        const token = await getToken();
        const [deptsRes, rolesRes] = await Promise.all([
          fetch(`${AUTH_API}/api/orgs/${organization.id}/departments`, {
            headers: { Authorization: `Bearer ${token}` },
          }),
          fetch(`${AUTH_API}/api/orgs/${organization.id}/roles`, {
            headers: { Authorization: `Bearer ${token}` },
          }),
        ]);

        if (deptsRes.ok) {
          const data = await deptsRes.json();
          setDepartments(Array.isArray(data) ? data : []);
        } else {
          const bodyText = await deptsRes.text();
          console.error("InviteDialog: failed to fetch departments", {
            status: deptsRes.status,
            body: bodyText,
          });
          setDepartments([]);
        }

        if (rolesRes.ok) {
          const data = await rolesRes.json();
          setRoles(Array.isArray(data) ? data : []);
        } else {
          const bodyText = await rolesRes.text();
          console.error("InviteDialog: failed to fetch roles", {
            status: rolesRes.status,
            body: bodyText,
          });
          setRoles([]);
        }
      } catch (err) {
        console.warn('InviteDialog: failed to load departments/roles', err)
        setDepartments([]);
        setRoles([]);
      } finally {
        setDeptsLoading(false);
        setRolesLoading(false);
      }
    };
    fetchDeptsAndRoles();
  }, [isOpen, organization?.id, getToken]);

  // Escape key handler
  useEffect(() => {
    if (!isOpen) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [isOpen, onClose]);

  // Click outside handler
  const handleOverlayClick = useCallback(
    (e: React.MouseEvent) => {
      if (dialogRef.current && !dialogRef.current.contains(e.target as Node)) {
        onClose();
      }
    },
    [onClose]
  );

  // Form field handler
  const handleField = (field: keyof SingleFormData, value: string) => {
    setForm((prev) => ({ ...prev, [field]: value }));
  };

  // Single invite submit
  const handleSingleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (!form.email || !form.first_name || !form.last_name || !form.department) {
      onResult("Please fill in all required fields.", "error");
      return;
    }
    const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
    if (!emailRegex.test(form.email)) {
      onResult("Please enter a valid email address.", "error");
      return;
    }
    if (!organization?.id) {
      onResult("No organisation selected.", "error");
      return;
    }

    setLoading(true);
    try {
      const token = await getToken();
      const res = await fetch(`${AUTH_API}/api/orgs/${organization.id}/employees/invite`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify(form),
      });
      const data = await res.json();
      if (!res.ok) {
        onResult(data.error || "Failed to send invitation.", "error");
      } else {
        onResult(`Invitation sent to ${form.email}!`, "success");
        setForm(INITIAL_FORM);
        onClose();
      }
    } catch {
      onResult("Network error. Please try again.", "error");
    } finally {
      setLoading(false);
    }
  };

  // Bulk invite submit
  const handleBulkSubmit = async () => {
    if (!file) {
      onResult("Please select an Excel file first.", "error");
      return;
    }
    if (!organization?.id) {
      onResult("No organisation selected.", "error");
      return;
    }
    setBulkResult(null);
    setLoading(true);

    try {
      const token = await getToken();
      const formData = new FormData();
      formData.append("file", file);
      const res = await fetch(`${AUTH_API}/api/orgs/${organization.id}/employees/invite/bulk`, {
        method: "POST",
        headers: { Authorization: `Bearer ${token}` },
        body: formData,
      });
      const data = await res.json();
      if (!res.ok) {
        const fallbackMessage = data?.error || "Bulk upload failed.";
        const normalized: BulkResult = {
          total_rows: Number(data?.total_rows || 0),
          successful: Number(data?.successful || 0),
          failed: Number(data?.failed || 1),
          errors: Array.isArray(data?.errors) && data.errors.length > 0
            ? data.errors.map((entry: any, index: number) => ({
              row: Number(entry?.row || index + 1),
              email: String(entry?.email || ""),
              message: String(entry?.message || fallbackMessage),
            }))
            : [{ row: 0, email: "", message: fallbackMessage }],
        };
        setBulkResult(normalized);
        onResult(fallbackMessage, "error");
      } else {
        const successCount = Number(data.successful || 0);
        const failCount = Number(data.failed || 0);
        const totalRows = Number(data.total_rows || successCount + failCount);
        const toastType: "success" | "error" = successCount > 0 ? "success" : "error";
        onResult(`Bulk invite finished: ${successCount} successful, ${failCount} failed, ${totalRows} total.`, toastType);
        setBulkResult({
          total_rows: totalRows,
          successful: successCount,
          failed: failCount,
          errors: Array.isArray(data?.errors) ? data.errors : [],
        });
        if (failCount === 0) {
          onClose();
        }
      }
    } catch {
      onResult("Network error. Please try again.", "error");
    } finally {
      setLoading(false);
    }
  };

  // Drag and drop handlers
  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(true);
  };
  const handleDragLeave = () => setDragOver(false);
  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDragOver(false);
    const droppedFile = e.dataTransfer.files[0];
    if (droppedFile && (droppedFile.name.endsWith(".xlsx") || droppedFile.name.endsWith(".xls"))) {
      setFile(droppedFile);
    } else {
      onResult("Please upload a valid Excel file (.xlsx or .xls).", "error");
    }
  };
  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = e.target.files?.[0];
    if (selected) {
      setFile(selected);
    }
  };

  if (!isOpen) return null;

  return (
    <div className="invite-overlay" onClick={handleOverlayClick}>
      <div className="invite-dialog" ref={dialogRef}>
        {/* Header */}
        <div className="invite-header">
          <div className="invite-header-text">
            <h3 className="invite-title">Invite Employees</h3>
            <p className="invite-subtitle">Add new team members to your organisation</p>
          </div>
          <button className="invite-close" onClick={onClose} aria-label="Close">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="18" height="18">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Tab Switcher */}
        <div className="invite-tabs">
          <button
            className={`invite-tab ${tab === "single" ? "active" : ""}`}
            onClick={() => { setTab("single"); }}
          >
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
            </svg>
            Single Invite
          </button>
          <button
            className={`invite-tab ${tab === "bulk" ? "active" : ""}`}
            onClick={() => { setTab("bulk"); }}
          >
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
              <path strokeLinecap="round" strokeLinejoin="round" d="M18 18.72a9.094 9.094 0 0 0 3.741-.479 3 3 0 0 0-4.682-2.72m.94 3.198.001.031c0 .225-.012.447-.037.666A11.944 11.944 0 0 1 12 21c-2.17 0-4.207-.576-5.963-1.584A6.062 6.062 0 0 1 6 18.719m12 0a5.971 5.971 0 0 0-.941-3.197m0 0A5.995 5.995 0 0 0 12 12.75a5.995 5.995 0 0 0-5.058 2.772m0 0a3 3 0 0 0-4.681 2.72 8.986 8.986 0 0 0 3.74.477m.94-3.197a5.971 5.971 0 0 0-.94 3.197M15 6.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Zm6 3a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Zm-13.5 0a2.25 2.25 0 1 1-4.5 0 2.25 2.25 0 0 1 4.5 0Z" />
            </svg>
            Bulk Invite
          </button>
        </div>
        {/* Single Invite Tab */}
        {tab === "single" && (
          <form className="invite-form" onSubmit={handleSingleSubmit}>
            <div className="invite-form-row">
              <div className="invite-field">
                <label className="invite-label">
                  First Name <span className="invite-required">*</span>
                </label>
                <input
                  type="text"
                  className="invite-input"
                  placeholder="John"
                  value={form.first_name}
                  onChange={(e) => handleField("first_name", e.target.value)}
                  required
                />
              </div>
              <div className="invite-field">
                <label className="invite-label">
                  Last Name <span className="invite-required">*</span>
                </label>
                <input
                  type="text"
                  className="invite-input"
                  placeholder="Doe"
                  value={form.last_name}
                  onChange={(e) => handleField("last_name", e.target.value)}
                  required
                />
              </div>
            </div>

            <div className="invite-field">
              <label className="invite-label">
                Email Address <span className="invite-required">*</span>
              </label>
              <input
                type="email"
                className="invite-input"
                placeholder="john.doe@company.com"
                value={form.email}
                onChange={(e) => handleField("email", e.target.value)}
                required
              />
            </div>

            <div className="invite-field">
              <label className="invite-label">
                Department <span className="invite-required">*</span>
              </label>
              {deptsLoading ? (
                <div className="invite-input" style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--text-muted)" }}>
                  <span className="invite-spinner" style={{ borderColor: "var(--border)", borderTopColor: "var(--accent)", width: 12, height: 12 }} />
                  Loading departments…
                </div>
              ) : departments.length > 0 ? (
                <select
                  className="invite-input"
                  value={form.department}
                  onChange={(e) => handleField("department", e.target.value)}
                  required
                >
                  <option value="">Select a department</option>
                  {departments.map((dept) => (
                    <option key={dept.id} value={dept.name}>
                      {dept.name}
                    </option>
                  ))}
                </select>
              ) : (
                <input
                  type="text"
                  className="invite-input"
                  placeholder="e.g. Engineering, Marketing"
                  value={form.department}
                  onChange={(e) => handleField("department", e.target.value)}
                  required
                />
              )}
            </div>

            <div className="invite-form-row">
              <div className="invite-field">
                <label className="invite-label">Workflow Roles</label>
                {rolesLoading ? (
                  <div className="invite-input" style={{ display: "flex", alignItems: "center", gap: 8, color: "var(--text-muted)" }}>
                    <span className="invite-spinner" style={{ borderColor: "var(--border)", borderTopColor: "var(--accent)", width: 12, height: 12 }} />
                    Loading roles...
                  </div>
                ) : roles.length > 0 ? (
                  <>
                    <div className="invite-tag-picker-row">
                      <select
                        className="invite-input"
                        value={roleToAdd}
                        onChange={(e) => setRoleToAdd(e.target.value)}
                      >
                        <option value="">Select a workflow role</option>
                        {roles.map((role) => (
                          <option key={role.id} value={role.name}>
                            {role.name}
                          </option>
                        ))}
                      </select>
                      <button
                        type="button"
                        className="action-btn action-btn-outline"
                        onClick={() => {
                          if (!roleToAdd || form.roles.includes(roleToAdd)) return;
                          setForm((prev) => ({ ...prev, roles: [...prev.roles, roleToAdd] }));
                          setRoleToAdd("");
                        }}
                        disabled={!roleToAdd || form.roles.includes(roleToAdd)}
                      >
                        Add
                      </button>
                    </div>
                    {form.roles.length > 0 && (
                      <div className="invite-tag-list">
                        {form.roles.map((roleName) => (
                          <span key={roleName} className="invite-tag-chip">
                            {roleName}
                            <button
                              type="button"
                              aria-label={`Remove ${roleName} role`}
                              onClick={() => setForm((prev) => ({ ...prev, roles: prev.roles.filter((item) => item !== roleName) }))}
                            >
                              ×
                            </button>
                          </span>
                        ))}
                      </div>
                    )}
                  </>
                ) : (
                  <div className="settings-empty-inline">Create workflow roles from Settings before tagging invitees.</div>
                )}
              </div>
              <div className="invite-field">
                <label className="invite-label">Job Title</label>
                <input
                  type="text"
                  className="invite-input"
                  placeholder="e.g. Software Engineer"
                  value={form.job_title}
                  onChange={(e) => handleField("job_title", e.target.value)}
                />
              </div>
            </div>

            <div className="invite-actions">
              <button type="button" className="action-btn action-btn-outline" onClick={onClose}>
                Cancel
              </button>
              <button type="submit" className="action-btn action-btn-primary invite-submit" disabled={loading}>
                {loading ? (
                  <>
                    <span className="invite-spinner" />
                    Sending…
                  </>
                ) : (
                  <>
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M6 12 3.269 3.125A59.769 59.769 0 0 1 21.485 12 59.768 59.768 0 0 1 3.27 20.875L5.999 12Zm0 0h7.5" />
                    </svg>
                    Send Invitation
                  </>
                )}
              </button>
            </div>
          </form>
        )}

        {/* Bulk Invite Tab */}
        {tab === "bulk" && (
          <div className="invite-bulk">
            <div
              className={`invite-dropzone ${dragOver ? "dragover" : ""} ${file ? "has-file" : ""}`}
              onDragOver={handleDragOver}
              onDragLeave={handleDragLeave}
              onDrop={handleDrop}
              onClick={() => fileInputRef.current?.click()}
            >
              <input
                ref={fileInputRef}
                type="file"
                accept=".xlsx,.xls"
                onChange={handleFileChange}
                style={{ display: "none" }}
              />
              {file ? (
                <div className="invite-file-info">
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="32" height="32">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
                  </svg>
                  <div>
                    <p className="invite-file-name">{file.name}</p>
                    <p className="invite-file-size">{(file.size / 1024).toFixed(1)} KB</p>
                  </div>
                  <button
                    className="invite-file-remove"
                    onClick={(e) => { e.stopPropagation(); setFile(null); setBulkResult(null); }}
                    aria-label="Remove file"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="14" height="14">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              ) : (
                <div className="invite-drop-content">
                  <div className="invite-drop-icon">
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="36" height="36">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
                    </svg>
                  </div>
                  <p className="invite-drop-text">
                    Drag & drop your Excel file here
                  </p>
                  <p className="invite-drop-hint">
                    or click to browse · Accepts <strong>.xlsx</strong> and <strong>.xls</strong>
                  </p>
                </div>
              )}
            </div>

            <div className="invite-bulk-info">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="14" height="14">
                <path strokeLinecap="round" strokeLinejoin="round" d="m11.25 11.25.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z" />
              </svg>
              <span>
                Columns required: <strong>email</strong>, <strong>first_name</strong>, <strong>last_name</strong>, <strong>department</strong>.
                Optional: <strong>role</strong>, <strong>job_title</strong>.
              </span>
            </div>

            {/* Bulk Results */}
            {bulkResult && (
              <div className="invite-bulk-results">
                <div className="invite-bulk-stat invite-bulk-stat-success">
                  <span className="invite-bulk-stat-value">{bulkResult.successful}</span>
                  <span className="invite-bulk-stat-label">Successful</span>
                </div>
                <div className="invite-bulk-stat invite-bulk-stat-fail">
                  <span className="invite-bulk-stat-value">{bulkResult.failed}</span>
                  <span className="invite-bulk-stat-label">Failed</span>
                </div>
                <div className="invite-bulk-stat">
                  <span className="invite-bulk-stat-value">{bulkResult.total_rows}</span>
                  <span className="invite-bulk-stat-label">Total Rows</span>
                </div>
              </div>
            )}

            {bulkResult && bulkResult.errors.length > 0 && (
              <div className="invite-bulk-errors">
                <p className="invite-bulk-errors-title">Errors</p>
                <div className="invite-bulk-errors-list">
                  {bulkResult.errors.slice(0, 10).map((err, i) => (
                    <div key={i} className="invite-bulk-error-row">
                      <span className="invite-bulk-error-row-num">Row {err.row}</span>
                      <span className="invite-bulk-error-email">{err.email || "—"}</span>
                      <span className="invite-bulk-error-msg">{err.message}</span>
                    </div>
                  ))}
                  {bulkResult.errors.length > 10 && (
                    <p className="invite-bulk-errors-more">
                      +{bulkResult.errors.length - 10} more errors
                    </p>
                  )}
                </div>
              </div>
            )}

            <div className="invite-actions">
              <button type="button" className="action-btn action-btn-outline" onClick={onClose}>
                Cancel
              </button>
              <button
                className="action-btn action-btn-primary invite-submit"
                onClick={handleBulkSubmit}
                disabled={!file || loading}
              >
                {loading ? (
                  <>
                    <span className="invite-spinner" />
                    Uploading…
                  </>
                ) : (
                  <>
                    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                      <path strokeLinecap="round" strokeLinejoin="round" d="M3 16.5v2.25A2.25 2.25 0 0 0 5.25 21h13.5A2.25 2.25 0 0 0 21 18.75V16.5m-13.5-9L12 3m0 0 4.5 4.5M12 3v13.5" />
                    </svg>
                    Upload & Invite
                  </>
                )}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
