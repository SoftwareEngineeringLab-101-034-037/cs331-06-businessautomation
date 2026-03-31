"use client";

import { useState } from "react";
import type { Task } from "@/types/dashboard";

interface TaskActionsProps {
  task: Task;
  onAction: (action: string, data?: Record<string, string>) => void;
}

export default function TaskActions({ task, onAction }: TaskActionsProps) {
  const [showCommentModalFor, setShowCommentModalFor] = useState<string | null>(null);
  const [reason, setReason] = useState("");

  const actionMeta: Record<string, { label: string; className: string }> = {
    start: { label: "Start", className: "action-btn action-btn-primary" },
    approve: { label: "Approve", className: "action-btn action-btn-success" },
    reject: { label: "Reject", className: "action-btn action-btn-danger" },
    clarify: { label: "Clarify", className: "action-btn action-btn-warning" },
    complete: { label: "Complete", className: "action-btn action-btn-primary" },
  };

  const allowed = task.status === "pending"
    ? ["start"]
    : (task.allowedActions && task.allowedActions.length > 0
      ? task.allowedActions
      : ["complete"]);

  function handleClick(action: string) {
    if (action === "start") {
      onAction(action);
      return;
    }
    setShowCommentModalFor(action);
  }

  return (
    <>
      <div className="task-actions">
        <h4 className="task-actions-title">Actions</h4>
        <div className="task-actions-grid">
          {allowed.map((action) => {
            const meta = actionMeta[action];
            if (!meta) return null;
            return (
              <button
                key={action}
                className={meta.className}
                onClick={() => handleClick(action)}
              >
                {meta.label}
              </button>
            );
          })}
        </div>
      </div>

      {showCommentModalFor && (
        <div className="modal-overlay" onClick={() => { setShowCommentModalFor(null); setReason(""); }}>
          <div className="modal-content task-action-modal" onClick={(e) => e.stopPropagation()}>
            <h3 className="modal-title">Add Comment</h3>
            <p className="modal-desc">Required. Briefly explain this decision for the audit trail.</p>
            <div className="wf-field">
              <label className="wf-field-label">
                Comment
                <span className="wf-required-star" style={{ marginLeft: 4 }}>*</span>
              </label>
              <span className="wf-field-hint">Use the same kind of short, clear note you would leave in a workflow update.</span>
              <textarea
                value={reason}
                onChange={(e) => setReason(e.target.value)}
                placeholder="e.g. Approved after reviewing the submitted invoice"
                className="wf-textarea"
                rows={3}
                style={!reason.trim() ? { borderColor: "#ef4444" } : {}}
              />
              {!reason.trim() && (
                <span className="task-action-error">
                  A comment is required to continue.
                </span>
              )}
            </div>
            <div className="modal-actions">
              <button className="action-btn action-btn-outline" onClick={() => { setShowCommentModalFor(null); setReason(""); }}>
                Cancel
              </button>
              <button
                className="action-btn action-btn-primary"
                onClick={() => {
                  onAction(showCommentModalFor, { reason });
                  setShowCommentModalFor(null);
                  setReason("");
                }}
                disabled={!reason.trim()}
              >
                Confirm
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
