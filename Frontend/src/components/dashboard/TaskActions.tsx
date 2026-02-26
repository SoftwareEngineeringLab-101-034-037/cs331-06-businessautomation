"use client";

import { useState } from "react";
import type { Task, TaskStatus } from "@/types/dashboard";
import { useRole } from "./RoleProvider";

interface TaskActionsProps {
  task: Task;
  onAction: (action: string, data?: Record<string, string>) => void;
}

export default function TaskActions({ task, onAction }: TaskActionsProps) {
  const { role } = useRole();
  const [showEscalateModal, setShowEscalateModal] = useState(false);
  const [showSendBackModal, setShowSendBackModal] = useState(false);
  const [reason, setReason] = useState("");

  const canComplete = ["pending", "in_progress", "sent_back"].includes(task.status);
  const canEscalate = ["pending", "in_progress", "overdue"].includes(task.status);
  const canSendBack = ["pending", "in_progress"].includes(task.status);
  const canCancel = ["pending", "in_progress", "sent_back"].includes(task.status);
  const canReopen = ["cancelled", "completed"].includes(task.status);
  const canReassign = ["org_admin", "admin"].includes(role) && !["completed", "cancelled"].includes(task.status);
  const canStartProgress = task.status === "pending";

  return (
    <>
      <div className="task-actions">
        <h4 className="task-actions-title">Actions</h4>
        <div className="task-actions-grid">
          {canStartProgress && (
            <button
              className="action-btn action-btn-primary"
              onClick={() => onAction("start_progress")}
            >
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 0 1 0 1.972l-11.54 6.347a1.125 1.125 0 0 1-1.667-.986V5.653Z" />
              </svg>
              Start Working
            </button>
          )}

          {canComplete && (
            <button
              className="action-btn action-btn-success"
              onClick={() => onAction("complete")}
            >
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
              Mark Complete
            </button>
          )}

          {canEscalate && (
            <button
              className="action-btn action-btn-warning"
              onClick={() => setShowEscalateModal(true)}
            >
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
              </svg>
              Escalate Task
            </button>
          )}

          {canSendBack && (
            <button
              className="action-btn action-btn-secondary"
              onClick={() => setShowSendBackModal(true)}
            >
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 15 3 9m0 0 6-6M3 9h12a6 6 0 0 1 0 12h-3" />
              </svg>
              Send Back
            </button>
          )}

          {canReassign && (
            <button
              className="action-btn action-btn-outline"
              onClick={() => onAction("reassign")}
            >
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21 3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
              </svg>
              Reassign
            </button>
          )}

          {canCancel && (
            <button
              className="action-btn action-btn-danger"
              onClick={() => onAction("cancel")}
            >
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                <path strokeLinecap="round" strokeLinejoin="round" d="m9.75 9.75 4.5 4.5m0-4.5-4.5 4.5M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
              Cancel Task
            </button>
          )}

          {canReopen && (
            <button
              className="action-btn action-btn-outline"
              onClick={() => onAction("reopen")}
            >
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-4 h-4">
                <path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182" />
              </svg>
              Reopen
            </button>
          )}
        </div>
      </div>

      {/* Escalate Modal */}
      {showEscalateModal && (
        <div className="modal-overlay" onClick={() => setShowEscalateModal(false)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()}>
            <h3 className="modal-title">Escalate Task</h3>
            <p className="modal-desc">Provide a reason for escalating this task. It will be routed to the next authority level.</p>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Reason for escalation..."
              className="modal-textarea"
              rows={3}
            />
            <div className="modal-actions">
              <button className="action-btn action-btn-outline" onClick={() => { setShowEscalateModal(false); setReason(""); }}>
                Cancel
              </button>
              <button
                className="action-btn action-btn-warning"
                onClick={() => { onAction("escalate", { reason }); setShowEscalateModal(false); setReason(""); }}
                disabled={!reason.trim()}
              >
                Escalate
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Send Back Modal */}
      {showSendBackModal && (
        <div className="modal-overlay" onClick={() => setShowSendBackModal(false)}>
          <div className="modal-content" onClick={(e) => e.stopPropagation()}>
            <h3 className="modal-title">Send Task Back</h3>
            <p className="modal-desc">This will return the task to the previous step. Please provide your feedback.</p>
            <textarea
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              placeholder="Reason for sending back..."
              className="modal-textarea"
              rows={3}
            />
            <div className="modal-actions">
              <button className="action-btn action-btn-outline" onClick={() => { setShowSendBackModal(false); setReason(""); }}>
                Cancel
              </button>
              <button
                className="action-btn action-btn-secondary"
                onClick={() => { onAction("send_back", { reason }); setShowSendBackModal(false); setReason(""); }}
                disabled={!reason.trim()}
              >
                Send Back
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}
