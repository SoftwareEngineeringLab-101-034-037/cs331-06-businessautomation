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
    : ((task.allowedActions && task.allowedActions.length > 0)
      ? task.allowedActions
      : ["complete"]);

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
                onClick={() => {
                  setShowCommentModalFor(action);
                }}
              >
                {meta.label}
              </button>
            );
          })}
        </div>
      </div>

      {showCommentModalFor && (
        <div className="modal-overlay" onClick={() => setShowCommentModalFor(null)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()}>
            <h3 className="modal-title">Add Comment</h3>
            <p className="modal-desc">Comment is mandatory for this action.</p>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Enter your comment..."
              className="modal-textarea"
              rows={3}
            />
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
