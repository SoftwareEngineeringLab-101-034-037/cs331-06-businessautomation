"use client";

import { useState, useMemo } from "react";
import { MOCK_REQUESTS } from "@/lib/mock-data";
import type { RequestStatus, WorkflowRequest } from "@/types/dashboard";
import { REQUEST_STATUS_CONFIG } from "@/types/dashboard";

const REQUEST_COLUMNS: { key: RequestStatus; label: string }[] = [
  { key: "submitted", label: "Submitted" },
  { key: "in_progress", label: "In Progress" },
  { key: "approved", label: "Approved" },
  { key: "rejected", label: "Rejected" },
  { key: "completed", label: "Completed" },
];

export default function RequestsPage() {
  const [search, setSearch] = useState("");

  const filtered = useMemo(() => {
    if (!search) return MOCK_REQUESTS;
    const q = search.toLowerCase();
    return MOCK_REQUESTS.filter(
      (r) =>
        r.title.toLowerCase().includes(q) ||
        r.id.toLowerCase().includes(q) ||
        r.workflowName.toLowerCase().includes(q)
    );
  }, [search]);

  const columns = useMemo(() => {
    return REQUEST_COLUMNS.map((col) => ({
      ...col,
      requests: filtered.filter((r) => r.status === col.key),
    }));
  }, [filtered]);

  return (
    <div className="dashboard-page" style={{ maxWidth: "100%", padding: "0 16px" }}>
      <div className="page-header">
        <div>
          <h2 className="page-title">Workflow Requests</h2>
          <p className="page-subtitle">{filtered.length} request{filtered.length !== 1 ? "s" : ""} across {columns.filter((c) => c.requests.length > 0).length} stages</p>
        </div>
        <button className="action-btn action-btn-primary">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          New Request
        </button>
      </div>

      {/* Search */}
      <div className="filters-bar" style={{ marginBottom: 16 }}>
        <div className="filter-search">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
            <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
          </svg>
          <input
            type="text"
            placeholder="Search requests..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="filter-search-input"
          />
        </div>
      </div>

      {/* Kanban Board */}
      <div className="kanban-container">
        {columns.map((col) => {
          const cfg = REQUEST_STATUS_CONFIG[col.key];
          return (
            <div key={col.key} className="kanban-column">
              <div className="kanban-column-header">
                <span className="kanban-column-title" style={{ color: cfg.color }}>{col.label}</span>
                <span className="kanban-column-count">{col.requests.length}</span>
              </div>
              <div className="kanban-column-body">
                {col.requests.length > 0 ? (
                  col.requests.map((req) => (
                    <RequestKanbanCard key={req.id} request={req} />
                  ))
                ) : (
                  <div className="kanban-empty"><p>No requests</p></div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function RequestKanbanCard({ request }: { request: WorkflowRequest }) {
  const cfg = REQUEST_STATUS_CONFIG[request.status];
  const progress = request.totalSteps > 0 ? Math.round((request.completedSteps / request.totalSteps) * 100) : 0;

  return (
    <div className="kanban-card">
      <div className="kanban-card-header">
        <span className="kanban-card-id">{request.id}</span>
        <span
          className="kanban-card-priority"
          style={{ background: cfg.bg, color: cfg.color }}
        >
          {cfg.label}
        </span>
      </div>
      <h4 className="kanban-card-title">{request.title}</h4>
      <p className="kanban-card-workflow">{request.workflowName}</p>
      <div className="kanban-card-meta">
        <div className="kanban-card-meta-item">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="12" height="12">
            <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
          </svg>
          {request.submittedByName}
        </div>
        <div className="kanban-card-meta-item">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="12" height="12">
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
          </svg>
          {new Date(request.lastUpdated).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
        </div>
      </div>
      {/* Progress */}
      <div className="kanban-card-progress">
        <div className="kanban-card-progress-bar">
          <div className="kanban-card-progress-fill" style={{ width: `${progress}%` }} />
        </div>
        <span className="kanban-card-progress-text">
          {request.completedSteps}/{request.totalSteps}
        </span>
      </div>
      <div className="kanban-card-footer">
        <span style={{ fontSize: "0.72rem", color: "var(--text-muted)" }}>Current: {request.currentStep}</span>
      </div>
    </div>
  );
}
