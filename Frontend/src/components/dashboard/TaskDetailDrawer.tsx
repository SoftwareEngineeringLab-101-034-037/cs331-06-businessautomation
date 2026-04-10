"use client";

import { Fragment, useEffect } from "react";
import type { Task } from "@/types/dashboard";
import { TASK_STATUS_CONFIG, PRIORITY_CONFIG } from "@/types/dashboard";
import { formatInstanceLabel } from "@/lib/instance-label";
import TaskActions from "./TaskActions";

interface TaskDetailDrawerProps {
  task: Task | null;
  isOpen: boolean;
  onClose: () => void;
  onAction?: (task: Task, action: string, data?: Record<string, string>) => void;
}

export default function TaskDetailDrawer({ task, isOpen, onClose, onAction }: TaskDetailDrawerProps) {
  // Close on Escape key
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onClose();
    }
    if (isOpen) {
      document.addEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "hidden";
    }
    return () => {
      document.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = "";
    };
  }, [isOpen, onClose]);

  useEffect(() => {
    if (isOpen && task?.instanceError) {
      console.warn(`Workflow instance error occurred for task ${task.id}.`);
    }
  }, [isOpen, task?.id, task?.instanceError]);

  if (!isOpen || !task) return null;

  const statusCfg = TASK_STATUS_CONFIG[task.status];
  const priorityCfg = PRIORITY_CONFIG[task.priority];
  const progress = (task.stepNumber / task.totalSteps) * 100;
  const completedSteps = Math.min(task.totalSteps, Math.max(0, task.stepNumber));
  const remainingSteps = Math.max(0, task.totalSteps - completedSteps);
  const uploadedFiles = extractUploadedFiles(task.visibleData);

  function handleAction(action: string, data?: Record<string, string>) {
    if (onAction) {
      onAction(task!, action, data);
      return;
    }
    alert(`Action: ${action}${data ? `\nData: ${JSON.stringify(data)}` : ""}`);
  }

  return (
    <>
      {/* Overlay backdrop */}
      <div className="drawer-overlay" onClick={onClose} />

      {/* Slide-in panel */}
      <aside className="drawer-panel" role="dialog" aria-label="Task details">
        {/* Drawer header */}
        <div className="drawer-header">
          <div className="drawer-header-left">
            <div style={{ display: "flex", gap: 8, alignItems: "center", flexWrap: "wrap" }}>
              <span className="task-card-id" style={{ fontSize: "0.85rem" }}>
                {task.id}
              </span>
              {task.instanceId && (
                <span className="role-badge" style={{ fontSize: "0.75rem" }}>
                  Instance: {formatInstanceLabel(task.instanceId)}
                </span>
              )}
            </div>
            <div className="task-card-badges">
              <span
                className="task-badge"
                style={{
                  background: priorityCfg.bg,
                  color: priorityCfg.color,
                  fontSize: "0.8rem",
                  padding: "3px 12px",
                }}
              >
                {priorityCfg.label}
              </span>
              <span
                className="task-badge"
                style={{
                  background: statusCfg.bg,
                  color: statusCfg.color,
                  fontSize: "0.8rem",
                  padding: "3px 12px",
                }}
              >
                {statusCfg.label}
              </span>
            </div>
          </div>
          <button className="drawer-close-btn" onClick={onClose} aria-label="Close drawer">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="20" height="20">
              <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Drawer body — scrollable */}
        <div className="drawer-body">
          {/* Title */}
          <h2 className="drawer-task-title">{task.title}</h2>

          {/* Description */}
          <section className="detail-section">
            <h3 className="detail-section-title">Description</h3>
            <p className="detail-description">{task.description}</p>
          </section>

          {/* Sent back notice */}
          {task.status === "sent_back" && task.sentBackReason && (
            <div className="detail-notice warning">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 15 3 9m0 0 6-6M3 9h12a6 6 0 0 1 0 12h-3" />
              </svg>
              <div>
                <strong>Sent Back</strong>
                <p>{task.sentBackReason}</p>
              </div>
            </div>
          )}

          {/* Escalation notice */}
          {task.status === "escalated" && (
            <div className="detail-notice danger">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
              </svg>
              <div>
                <strong>Escalated</strong>
                <p>This task has been escalated due to SLA breach. Immediate attention required.</p>
              </div>
            </div>
          )}

          {task.instanceError && (
          <div className="detail-notice danger" role="alert">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="20" height="20">
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
              </svg>
              <div>
                <strong>Workflow Instance Error</strong>
                <p>{toUserSafeInstanceError(task.instanceError)}</p>
              </div>
            </div>
          )}

          {/* Workflow Progress */}
          <section className="detail-section">
            <h3 className="detail-section-title">Workflow Progress</h3>
            <div className="detail-progress">
              <div className="detail-progress-bar">
                <div className="task-progress-fill" style={{ width: `${progress}%`, background: statusCfg.color }} />
              </div>
              <div className="detail-progress-info">
                <span>Step {task.stepNumber} of {task.totalSteps}</span>
                <span>{completedSteps}/{task.totalSteps} = {progress.toFixed(1)}% complete</span>
                <span>{remainingSteps} step{remainingSteps === 1 ? "" : "s"} remaining</span>
              </div>
            </div>
            <div className="step-indicators">
              {Array.from({ length: task.totalSteps }).map((_, i) => (
                <div
                  key={i}
                  className={`step-dot ${i < task.stepNumber ? "completed" : ""} ${i === task.stepNumber - 1 ? "current" : ""}`}
                >
                  {i + 1}
                </div>
              ))}
            </div>
          </section>

          {/* Details card (inline in drawer — no sidebar layout) */}
          <section className="detail-section">
            <h3 className="detail-section-title">Details</h3>
            <div className="drawer-info-grid">
              <div className="detail-info-item">
                <dt>Workflow</dt>
                <dd>{task.workflowName}</dd>
              </div>
              {task.instanceId && (
                <div className="detail-info-item">
                  <dt>Instance</dt>
                  <dd>{formatInstanceLabel(task.instanceId)}</dd>
                </div>
              )}
              {task.instanceStatus && (
                <div className="detail-info-item">
                  <dt>Instance Status</dt>
                  <dd>{formatActionLabel(task.instanceStatus)}</dd>
                </div>
              )}
              <div className="detail-info-item">
                <dt>Department</dt>
                <dd>{task.departmentOrigin}</dd>
              </div>
              <div className="detail-info-item">
                <dt>Created</dt>
                <dd>{formatDate(task.createdAt)}</dd>
              </div>
              <div className="detail-info-item">
                <dt>Due Date</dt>
                <dd className={isOverdue(task.dueDate) ? "text-red" : ""}>
                  {formatDate(task.dueDate)}
                </dd>
              </div>
              {task.actionCommitted && (
                <div className="detail-info-item">
                  <dt>Action Committed</dt>
                  <dd>{formatActionLabel(task.actionCommitted)}</dd>
                </div>
              )}
              {task.completedAt && (
                <div className="detail-info-item">
                  <dt>Completed</dt>
                  <dd>{formatDate(task.completedAt)}</dd>
                </div>
              )}
              {task.escalatedAt && (
                <div className="detail-info-item">
                  <dt>Escalated</dt>
                  <dd className="text-red">{formatDate(task.escalatedAt)}</dd>
                </div>
              )}
            </div>
          </section>

          {task.comment && (
            <section className="detail-section">
              <h3 className="detail-section-title">
                {task.status === "completed" ? "Completion Comment" : "Latest Comment"}
              </h3>
              <div className="detail-comment-block">
                <p className="detail-comment-text">{task.comment}</p>
              </div>
            </section>
          )}

          {task.status === "in_progress" && task.visibleData && Object.keys(task.visibleData).length > 0 && (
            <section className="detail-section">
              <h3 className="detail-section-title">Visible Workflow Data (Live)</h3>
              <div className="workflow-data-panel">
                <div className="workflow-data-header">
                  <span>Fields available to assignee</span>
                  <span className="workflow-data-count">{Object.keys(task.visibleData).length}</span>
                </div>
                <div className="workflow-data-grid">
                  {Object.entries(task.visibleData)
                    .sort(([a], [b]) => a.localeCompare(b))
                    .map(([key, value]) => (
                      <div key={key} className="workflow-data-item">
                        <div className="workflow-data-keyline">
                          <span className="workflow-data-label">{prettifyDataKey(key)}</span>
                          <span className="workflow-data-codekey">{key}</span>
                        </div>
                        <div className="workflow-data-value">
                          <ValueRenderer value={value} />
                        </div>
                      </div>
                    ))}
                </div>
                {uploadedFiles.length > 0 && (
                  <div className="workflow-uploaded-files">
                    <h4 className="workflow-uploaded-files-title">Uploaded Files</h4>
                    <div className="workflow-uploaded-files-list">
                      {uploadedFiles.map((file, idx) => (
                        <a
                          key={`${file.url}-${idx}`}
                          className="workflow-data-chip workflow-data-chip-link"
                          href={file.url}
                          target="_blank"
                          rel="noreferrer"
                          title={file.url}
                        >
                          {file.name}
                        </a>
                      ))}
                    </div>
                  </div>
                )}
              </div>
            </section>
          )}

          {/* Tags */}
          <section className="detail-section">
            <h3 className="detail-section-title">Tags</h3>
            <div className="detail-tags">
              {task.tags.map((tag) => (
                <span key={tag} className="detail-tag">{tag}</span>
              ))}
            </div>
          </section>

          {/* Actions */}
          {task.status !== "completed" && (
            <TaskActions task={task} onAction={handleAction} />
          )}
        </div>
      </aside>
    </>
  );
}

function formatDate(dateStr: string): string {
  return new Date(dateStr).toLocaleDateString("en-US", {
    month: "short",
    day: "numeric",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function isOverdue(dateStr: string): boolean {
  return new Date(dateStr) < new Date();
}

function formatActionLabel(value: string): string {
  return value
    .replaceAll("_", " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function prettifyDataKey(key: string): string {
  return key
    .replaceAll("_", " ")
    .replace(/\b\w/g, (char) => char.toUpperCase());
}

function toUserSafeInstanceError(rawError: string): string {
  const fallback = "Workflow execution failed.";
  let raw = String(rawError || "").trim();
  let matchedFriendly = false;
  if (!raw) return fallback;

  // Handle JSON-string payloads before free-form parsing.
  if ((raw.startsWith("{") && raw.endsWith("}")) || (raw.startsWith("[") && raw.endsWith("]"))) {
    try {
      const parsed = JSON.parse(raw) as Record<string, unknown>;
      if (typeof parsed.error === "string" && parsed.error.trim()) {
        raw = parsed.error.trim();
      } else if (typeof parsed.message === "string" && parsed.message.trim()) {
        raw = parsed.message.trim();
      }
    } catch {
      // Keep raw text when payload is not valid JSON.
    }
  }

  const gmailErr = raw.match(/^integrations gmail send failed status=(\d+) error=(.+)$/i);
  if (gmailErr) {
    matchedFriendly = true;
    return `Gmail send failed (HTTP ${gmailErr[1]}): ${gmailErr[2].trim()}`;
  }

  const gmailBodyErr = raw.match(/^integrations gmail send failed status=(\d+) body=(.+)$/i);
  if (gmailBodyErr) {
    matchedFriendly = true;
    return `Gmail send failed (HTTP ${gmailBodyErr[1]}): ${gmailBodyErr[2].trim()}`;
  }

  const bodyMarker = " body=";
  const markerIndex = raw.indexOf(bodyMarker);
  if (markerIndex >= 0) {
    const prefix = raw.slice(0, markerIndex).trim();
    const bodyRaw = raw.slice(markerIndex + bodyMarker.length).trim();
    try {
      const parsedBody = JSON.parse(bodyRaw) as Record<string, unknown>;
      const nested = typeof parsedBody.error === "string"
        ? parsedBody.error.trim()
        : typeof parsedBody.message === "string"
          ? parsedBody.message.trim()
          : "";
      if (nested) {
        raw = prefix ? `${prefix}: ${nested}` : nested;
      }
    } catch {
      // Keep raw when body is not parsable JSON.
    }
  }

  const taskNodeErr = raw.match(/^task node\s+([^\s]+)\s+failed:\s*(.+)$/i);
  if (taskNodeErr) {
    matchedFriendly = true;
    raw = `Task step ${taskNodeErr[1]} failed: ${taskNodeErr[2].trim()}`;
  }

  const actionNodeErr = raw.match(/^action node\s+([^\s]+)\s+failed:\s*(.+)$/i);
  if (actionNodeErr) {
    matchedFriendly = true;
    raw = `Action step ${actionNodeErr[1]} failed: ${actionNodeErr[2].trim()}`;
  }

  raw = raw.replace(/\s+/g, " ").trim();
  if (!raw || !matchedFriendly) return fallback;
  if (raw.length > 260) {
    return `${raw.slice(0, 257)}...`;
  }
  return raw;
}

function ValueRenderer({ value }: { value: unknown }) {
  if (value == null) {
    return <span className="workflow-data-empty">No value</span>;
  }

  if (typeof value === "boolean") {
    return (
      <span className={`workflow-data-boolean ${value ? "true" : "false"}`}>
        {value ? "True" : "False"}
      </span>
    );
  }

  if (typeof value === "number" || typeof value === "string") {
    if (typeof value === "string") {
      const parsedLinks = parseNamedLinks(value);
      if (parsedLinks.length > 0) {
        return (
          <div className="workflow-data-chip-list">
            {parsedLinks.map((entry, idx) => (
              <a
                key={`${entry.url}-${idx}`}
                className="workflow-data-chip workflow-data-chip-link"
                href={entry.url}
                target="_blank"
                rel="noreferrer"
                title={entry.url}
              >
                {entry.name}
              </a>
            ))}
          </div>
        );
      }
    }
    if (typeof value === "string" && isLikelyUrl(value)) {
      return (
        <a className="workflow-data-link" href={value} target="_blank" rel="noreferrer" title={value}>
          {value}
        </a>
      );
    }
    return <span className="workflow-data-text">{String(value)}</span>;
  }

  if (Array.isArray(value)) {
    if (value.length === 0) {
      return <span className="workflow-data-empty">Empty list</span>;
    }

    const allPrimitive = value.every((entry) =>
      entry == null || typeof entry === "string" || typeof entry === "number" || typeof entry === "boolean",
    );

    if (allPrimitive) {
      return (
        <div className="workflow-data-chip-list">
          {value.map((entry, index) => {
            if (typeof entry === "string") {
              const parsedLinks = parseNamedLinks(entry);
              if (parsedLinks.length > 0) {
                return parsedLinks.map((parsed, parsedIndex) => (
                  <a
                    key={`${parsed.url}-${index}-${parsedIndex}`}
                    className="workflow-data-chip workflow-data-chip-link"
                    href={parsed.url}
                    target="_blank"
                    rel="noreferrer"
                    title={parsed.url}
                  >
                    {parsed.name}
                  </a>
                ));
              }
              if (isLikelyUrl(entry)) {
                return (
                  <a key={`${entry}-${index}`} className="workflow-data-chip workflow-data-chip-link" href={entry} target="_blank" rel="noreferrer" title={entry}>
                    {entry}
                  </a>
                );
              }
            }
            return (
              <span key={`${String(entry)}-${index}`} className="workflow-data-chip">
                {entry == null ? "null" : String(entry)}
              </span>
            );
          })}
        </div>
      );
    }

    return (
      <details className="workflow-data-details">
        <summary>Show list ({value.length})</summary>
        <pre className="workflow-data-json">{JSON.stringify(value, null, 2)}</pre>
      </details>
    );
  }

  if (typeof value === "object") {
    const record = value as Record<string, unknown>;
    const entries = Object.entries(record);
    if (entries.length === 0) {
      return <span className="workflow-data-empty">Empty object</span>;
    }

    const primitiveEntries = entries.filter(([, nestedValue]) =>
      nestedValue == null || typeof nestedValue === "string" || typeof nestedValue === "number" || typeof nestedValue === "boolean",
    );
    const complexEntries = entries.filter(([, nestedValue]) =>
      !(nestedValue == null || typeof nestedValue === "string" || typeof nestedValue === "number" || typeof nestedValue === "boolean"),
    );

    return (
      <Fragment>
        {primitiveEntries.length > 0 && (
          <div className="workflow-data-subgrid">
            {primitiveEntries.map(([nestedKey, nestedValue]) => (
              <div key={nestedKey} className="workflow-data-subitem">
                <span className="workflow-data-subkey">{prettifyDataKey(nestedKey)}</span>
                <span className="workflow-data-subvalue">
                  {nestedValue == null ? "null" : String(nestedValue)}
                </span>
              </div>
            ))}
          </div>
        )}

        {complexEntries.length > 0 && (
          <details className="workflow-data-details">
            <summary>Show nested data ({complexEntries.length})</summary>
            <pre className="workflow-data-json">{JSON.stringify(value, null, 2)}</pre>
          </details>
        )}
      </Fragment>
    );
  }

  return <span className="workflow-data-text">{String(value)}</span>;
}

function isLikelyUrl(value: string): boolean {
  const trimmed = value.trim();
  if (!trimmed) {
    return false;
  }
  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

function parseNamedLinks(value: string): Array<{ name: string; url: string }> {
  const out: Array<{ name: string; url: string }> = [];
  const seen = new Set<string>();
  for (const rawPart of value.split(",")) {
    const part = rawPart.trim();
    if (!part) {
      continue;
    }

    let name = "";
    let url = "";
    if (part.includes("|")) {
      const [left, right] = part.split("|", 2);
      name = left.trim();
      url = right.trim();
    } else if (isLikelyUrl(part)) {
      url = part;
    } else {
      continue;
    }

    if (!isLikelyUrl(url) || seen.has(url)) {
      continue;
    }
    seen.add(url);
    out.push({ name: name || deriveLinkLabel(url), url });
  }
  return out;
}

function deriveLinkLabel(url: string): string {
  const fileIDMatch = url.match(/\/file\/d\/([^/]+)/);
  if (fileIDMatch?.[1]) {
    return `Drive file ${fileIDMatch[1]}`;
  }
  try {
    const parsed = new URL(url);
    const leaf = parsed.pathname.split("/").filter(Boolean).pop();
    return leaf || parsed.hostname;
  } catch {
    return url;
  }
}

function extractUploadedFiles(visibleData?: Record<string, unknown>): Array<{ name: string; url: string }> {
  if (!visibleData) {
    return [];
  }

  const raw = visibleData["form_submission_files"];
  const aggregate: Array<{ name: string; url: string }> = [];
  const seen = new Set<string>();
  const append = (entries: Array<{ name: string; url: string }>) => {
    for (const entry of entries) {
      if (seen.has(entry.url)) {
        continue;
      }
      seen.add(entry.url);
      aggregate.push(entry);
    }
  };

  if (typeof raw === "string") {
    append(parseNamedLinks(raw));
  } else if (Array.isArray(raw)) {
    for (const item of raw) {
      if (typeof item === "string") {
        append(parseNamedLinks(item));
      }
    }
  }

  return aggregate;
}
