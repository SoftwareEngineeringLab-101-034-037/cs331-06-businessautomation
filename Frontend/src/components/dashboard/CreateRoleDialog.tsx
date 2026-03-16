"use client";

import { useMemo, useState, useRef, useEffect, useCallback } from "react";
import { useAuth, useOrganization } from "@clerk/nextjs";

interface CreateRoleDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated?: () => void;
  employees?: EmployeeOption[];
  initialRole?: {
    id: string;
    name: string;
    description?: string;
    memberIds?: string[];
  } | null;
}

interface EmployeeOption {
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

const AUTH_API = process.env.NEXT_PUBLIC_AUTH_API || "http://localhost:8080";

export default function CreateRoleDialog({ isOpen, onClose, onCreated, employees = [], initialRole = null }: CreateRoleDialogProps) {
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [search, setSearch] = useState("");
  const [departmentFilter, setDepartmentFilter] = useState("all");
  const [jobTitleFilter, setJobTitleFilter] = useState("all");
  const [selectedMemberIDs, setSelectedMemberIDs] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const { getToken } = useAuth();
  const { organization } = useOrganization();

  const dialogRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (isOpen) {
      setName(initialRole?.name || "");
      setDescription(initialRole?.description || "");
      setSearch("");
      setDepartmentFilter("all");
      setJobTitleFilter("all");
      setSelectedMemberIDs(initialRole?.memberIds || []);
      setError(null);
      setSuccess(null);
      setLoading(false);
    }
  }, [isOpen, initialRole]);

  useEffect(() => {
    if (!isOpen) return;
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [isOpen, onClose]);

  const handleOverlayClick = useCallback(
    (e: React.MouseEvent) => {
      if (dialogRef.current && !dialogRef.current.contains(e.target as Node)) {
        onClose();
      }
    },
    [onClose]
  );

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setSuccess(null);

    if (!name.trim()) {
      setError("Role name is required.");
      return;
    }

    if (!organization?.id) {
      setError("No organisation selected. Please select an organisation first.");
      return;
    }

    setLoading(true);
    try {
      const token = await getToken();
      const endpoint = initialRole
        ? `${AUTH_API}/api/orgs/${organization.id}/roles/${initialRole.id}`
        : `${AUTH_API}/api/orgs/${organization.id}/roles`;
      const res = await fetch(endpoint, {
        method: initialRole ? "PUT" : "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          name: name.trim(),
          description: description.trim(),
          member_ids: selectedMemberIDs,
        }),
      });

      const data = await res.json();

      if (!res.ok) {
        if (res.status === 409) {
          setError(data.error || "A role with this name already exists.");
        } else {
          setError(data.error || `Failed to ${initialRole ? "update" : "create"} role.`);
        }
      } else {
        setSuccess(`Role "${data.name || name}" ${initialRole ? "updated" : "created"} successfully!`);
        setName("");
        setDescription("");
        setSelectedMemberIDs([]);
        onCreated?.();
      }
    } catch {
      setError("Network error. Is the auth server running?");
    } finally {
      setLoading(false);
    }
  };

  const departments = useMemo(
    () => [...new Set(employees.map((employee) => employee.department?.name).filter(Boolean) as string[])].sort(),
    [employees]
  );
  const jobTitles = useMemo(
    () => [...new Set(employees.map((employee) => employee.job_title).filter(Boolean) as string[])].sort(),
    [employees]
  );
  const filteredEmployees = useMemo(() => {
    return employees.filter((employee) => {
      const fullName = `${employee.first_name} ${employee.last_name}`.toLowerCase();
      const matchesSearch =
        !search ||
        fullName.includes(search.toLowerCase()) ||
        employee.email.toLowerCase().includes(search.toLowerCase());
      const matchesDepartment = departmentFilter === "all" || employee.department?.name === departmentFilter;
      const matchesJobTitle = jobTitleFilter === "all" || employee.job_title === jobTitleFilter;
      return matchesSearch && matchesDepartment && matchesJobTitle;
    });
  }, [employees, search, departmentFilter, jobTitleFilter]);

  const toggleMember = (userID: string) => {
    setSelectedMemberIDs((current) =>
      current.includes(userID) ? current.filter((id) => id !== userID) : [...current, userID]
    );
  };

  if (!isOpen) return null;

  return (
    <div className="invite-overlay" onClick={handleOverlayClick}>
      <div className="invite-dialog dept-dialog" ref={dialogRef}>
        <div className="invite-header">
          <div className="invite-header-text">
            <h3 className="invite-title">Create Role</h3>
            <p className="invite-subtitle">{initialRole ? "Edit this workflow role and its members" : "Add a new workflow role to your organisation"}</p>
          </div>
          <button className="invite-close" onClick={onClose} aria-label="Close">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="18" height="18">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="dept-icon-banner">
          <div className="dept-icon-circle">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="28" height="28">
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 9V5.25m0 0A2.25 2.25 0 0 0 13.5 3h-3A2.25 2.25 0 0 0 8.25 5.25m7.5 0V9m0 0h3m-3 0h-3m0 0v9A2.25 2.25 0 0 0 10.5 20.25h3A2.25 2.25 0 0 0 15.75 18V9Z" />
            </svg>
          </div>
        </div>

        {error && (
          <div className="invite-message invite-message-error">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
            </svg>
            {error}
          </div>
        )}
        {success && (
          <div className="invite-message invite-message-success">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
              <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
            </svg>
            {success}
          </div>
        )}

        <form className="invite-form" onSubmit={handleSubmit}>
          <div className="invite-field">
            <label className="invite-label">
              Role Name <span className="invite-required">*</span>
            </label>
            <input
              type="text"
              className="invite-input"
              placeholder="e.g. Department Head, QA Lead"
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setError(null);
              }}
              autoFocus
              required
            />
          </div>

          <div className="invite-field">
            <label className="invite-label">Description</label>
            <textarea
              className="invite-input invite-textarea"
              placeholder="Briefly describe responsibilities for this role..."
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              rows={3}
            />
          </div>

          <div className="invite-field">
            <label className="invite-label">Role Members</label>
            <p className="settings-row-desc" style={{ marginBottom: 10 }}>
              Assign employees to this workflow role now. Filter by department or job title first.
            </p>

            <div className="settings-role-filter-row" style={{ marginBottom: 12 }}>
              <input
                type="text"
                className="invite-input"
                placeholder="Search employee by name or email"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
              <select className="invite-input" value={departmentFilter} onChange={(e) => setDepartmentFilter(e.target.value)}>
                <option value="all">All Departments</option>
                {departments.map((department) => (
                  <option key={department} value={department}>{department}</option>
                ))}
              </select>
              <select className="invite-input" value={jobTitleFilter} onChange={(e) => setJobTitleFilter(e.target.value)}>
                <option value="all">All Job Titles</option>
                {jobTitles.map((jobTitle) => (
                  <option key={jobTitle} value={jobTitle}>{jobTitle}</option>
                ))}
              </select>
            </div>

            <div className="settings-member-picker">
              {filteredEmployees.length > 0 ? filteredEmployees.map((employee) => {
                const checked = selectedMemberIDs.includes(employee.id);
                return (
                  <label key={employee.id} className={`settings-member-option ${checked ? "active" : ""}`}>
                    <input type="checkbox" checked={checked} onChange={() => toggleMember(employee.id)} />
                    <div className="settings-member-option-body">
                      <strong>{employee.first_name} {employee.last_name}</strong>
                      <span>{employee.email}</span>
                      <span>{employee.job_title || "No job title"}{employee.department?.name ? ` · ${employee.department.name}` : ""}</span>
                    </div>
                  </label>
                );
              }) : (
                <div className="settings-empty-inline">No employees match the current filters.</div>
              )}
            </div>

            {selectedMemberIDs.length > 0 && (
              <div className="settings-selection-meta">{selectedMemberIDs.length} employee(s) selected for this role.</div>
            )}
          </div>

          <div className="invite-actions">
            <button type="button" className="action-btn action-btn-outline" onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="action-btn action-btn-primary invite-submit" disabled={loading}>
              {loading ? (
                <>
                  <span className="invite-spinner" />
                  {initialRole ? "Saving..." : "Creating..."}
                </>
              ) : (
                <>
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
                  </svg>
                  {initialRole ? "Save Role" : "Create Role"}
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
