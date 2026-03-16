"use client";

import { useState } from "react";
import type {
  WorkflowStep,
  WorkflowTrigger,
  TriggerType,
  StepActionType,
  NodeType,
  ConnectorType,
  TaskAction,
} from "@/types/workflow";
import {
  TRIGGER_CONFIG,
  STEP_ACTION_CONFIG,
  NODE_TYPE_CONFIG,
  PRESET_ORG_ROLES,
  PRESET_POSITIONS,
  CONNECTOR_CONFIG,
  TASK_ACTION_OPTIONS,
} from "@/types/workflow";

/* ──────────────────────────────────────────────────────────────
   Trigger Editor  (used when user clicks the Start node)
   ────────────────────────────────────────────────────────────── */
interface TriggerEditorProps {
  trigger: WorkflowTrigger;
  onChange: (t: WorkflowTrigger) => void;
  onClose: () => void;
}

export function TriggerEditor({ trigger, onChange, onClose }: TriggerEditorProps) {
  return (
    <div className="wf-editor">
      <div className="wf-editor-header">
        <h3 className="wf-editor-title">Configure Trigger</h3>
        <button className="wf-editor-close" onClick={onClose}>
          <XIcon />
        </button>
      </div>

      <div className="wf-editor-body">
        <label className="wf-field-label">Trigger Type</label>
        <div className="wf-trigger-grid">
          {(Object.keys(TRIGGER_CONFIG) as TriggerType[]).map((type) => {
            const cfg = TRIGGER_CONFIG[type];
            const active = trigger.type === type;
            return (
              <button
                key={type}
                className={`wf-trigger-option ${active ? "active" : ""}`}
                onClick={() => onChange({ ...trigger, type })}
              >
                <span className="wf-trigger-option-label">{cfg.label}</span>
                <span className="wf-trigger-option-desc">{cfg.description}</span>
              </button>
            );
          })}
        </div>

        {/* Trigger-specific config */}
        {trigger.type === "scheduled" && (
          <div className="wf-field">
            <label className="wf-field-label">Schedule (cron or description)</label>
            <input
              className="wf-input"
              placeholder="e.g. Every Monday at 9:00 AM"
              value={trigger.config.schedule || ""}
              onChange={(e) =>
                onChange({ ...trigger, config: { ...trigger.config, schedule: e.target.value } })
              }
            />
          </div>
        )}
        {trigger.type === "email_received" && (
          <div className="wf-field">
            <label className="wf-field-label">Match Subject Contains</label>
            <input
              className="wf-input"
              placeholder="e.g. Purchase Order"
              value={trigger.config.subject || ""}
              onChange={(e) =>
                onChange({ ...trigger, config: { ...trigger.config, subject: e.target.value } })
              }
            />
          </div>
        )}
        {trigger.type === "webhook" && (
          <div className="wf-field">
            <label className="wf-field-label">Webhook Endpoint</label>
            <input
              className="wf-input"
              placeholder="Auto-generated when published"
              value={trigger.config.endpoint || ""}
              readOnly
            />
            <span className="wf-field-hint">
              A unique URL will be generated when you publish this workflow.
            </span>
          </div>
        )}
        {trigger.type === "form_submission" && (
          <div className="wf-field">
            <label className="wf-field-label">Form Name</label>
            <input
              className="wf-input"
              placeholder="e.g. Leave Request Form"
              value={trigger.config.formName || ""}
              onChange={(e) =>
                onChange({ ...trigger, config: { ...trigger.config, formName: e.target.value } })
              }
            />
          </div>
        )}
      </div>
    </div>
  );
}

/* ──────────────────────────────────────────────────────────────
   Step Editor — right panel for any selected node
   ────────────────────────────────────────────────────────────── */
interface StepEditorProps {
  step: WorkflowStep;
  stepIndex: number;
  onChange: (updated: WorkflowStep) => void;
  onClose: () => void;
  availableRoles?: string[];
}

export function StepEditor({ step, stepIndex, onChange, onClose, availableRoles = PRESET_ORG_ROLES }: StepEditorProps) {
  const [roleSearch, setRoleSearch] = useState("");
  const [showRoleDropdown, setShowRoleDropdown] = useState(false);
  const [posSearch, setPosSearch] = useState("");
  const [showPosDropdown, setShowPosDropdown] = useState(false);

  const filteredRoles = availableRoles.filter((r) =>
    r.toLowerCase().includes(roleSearch.toLowerCase()),
  );
  const filteredPositions = PRESET_POSITIONS.filter((p) =>
    p.toLowerCase().includes(posSearch.toLowerCase()),
  );

  function selectRole(role: string) {
    onChange({ ...step, assignedRole: role });
    setRoleSearch("");
    setShowRoleDropdown(false);
  }
  function selectPosition(pos: string) {
    onChange({ ...step, assignedPosition: pos });
    setPosSearch("");
    setShowPosDropdown(false);
  }

  const nodeType: NodeType = step.type || "task";
  const nodeTypeCfg = NODE_TYPE_CONFIG[nodeType];

  return (
    <div className="wf-editor">
      {/* ── Header ───────────────────────────────── */}
      <div className="wf-editor-header">
        <h3 className="wf-editor-title">
          Step {stepIndex + 1} &mdash;{" "}
          <span style={{ color: nodeTypeCfg?.color || "#888" }}>
            {nodeTypeCfg?.label || nodeType}
          </span>
        </h3>
        <button className="wf-editor-close" onClick={onClose}>
          <XIcon />
        </button>
      </div>

      <div className="wf-editor-body">

        {/* ════════════════════════════════════════════
            COMMON: Title & Description (all node types)
           ════════════════════════════════════════════ */}
        <div className="wf-section">
          <div className="wf-section-label">General</div>
          <div className="wf-field">
            <label className="wf-field-label">Title</label>
            <input
              className="wf-input"
              placeholder="e.g. Manager Approval"
              value={step.title}
              onChange={(e) => onChange({ ...step, title: e.target.value })}
            />
          </div>
          <div className="wf-field">
            <label className="wf-field-label">Description</label>
            <textarea
              className="wf-textarea"
              placeholder="What happens at this step?"
              rows={3}
              value={step.description}
              onChange={(e) => onChange({ ...step, description: e.target.value })}
            />
          </div>
        </div>

        {/* ════════════════════════════════════════════
            START NODE — Trigger overview (read-only)
           ════════════════════════════════════════════ */}
        {nodeType === "start" && (
          <div className="wf-section">
            <div className="wf-section-label">Trigger</div>
            <span className="wf-field-hint">
              Configure the trigger from the left sidebar panel.
              The start node will fire when the selected trigger activates.
            </span>
          </div>
        )}

        {/* ════════════════════════════════════════════
            TASK NODE — Full assignment configuration
           ════════════════════════════════════════════ */}
        {nodeType === "task" && (
          <>
            {/* ── Action type ── */}
            <div className="wf-section">
              <div className="wf-section-label">Task Type</div>
              <div className="wf-action-grid">
                {(Object.keys(STEP_ACTION_CONFIG) as StepActionType[]).map((type) => {
                  const cfg = STEP_ACTION_CONFIG[type];
                  const active = step.actionType === type;
                  return (
                    <button
                      key={type}
                      className={`wf-action-option ${active ? "active" : ""}`}
                      style={active ? { borderColor: cfg.color, background: `${cfg.color}10` } : {}}
                      onClick={() => onChange({ ...step, actionType: type })}
                    >
                      <span className="wf-action-dot" style={{ background: cfg.color }} />
                      {cfg.label}
                    </button>
                  );
                })}
              </div>
            </div>

            {/* ── Assigned Role ── */}
            <div className="wf-section">
              <div className="wf-section-label">Assignment</div>
              <div className="wf-field">
                <label className="wf-field-label">Assigned Role</label>
                <span className="wf-field-hint">
                  Which role group should handle this task?
                </span>
                <div className="wf-role-picker">
                  <input
                    className="wf-input"
                    placeholder="Search or type a custom role..."
                    value={showRoleDropdown ? roleSearch : step.assignedRole}
                    onFocus={() => { setShowRoleDropdown(true); setRoleSearch(""); }}
                    onChange={(e) => { setRoleSearch(e.target.value); setShowRoleDropdown(true); }}
                    onBlur={() => setTimeout(() => setShowRoleDropdown(false), 200)}
                    onKeyDown={(e) => { if (e.key === "Enter" && roleSearch.trim()) selectRole(roleSearch.trim()); }}
                  />
                  {showRoleDropdown && (
                    <div className="wf-role-dropdown">
                      {filteredRoles.length > 0 ? (
                        filteredRoles.slice(0, 10).map((r) => (
                          <button key={r} className={`wf-role-option ${step.assignedRole === r ? "active" : ""}`} onMouseDown={() => selectRole(r)}>
                            {r}
                          </button>
                        ))
                      ) : (
                        <div className="wf-role-empty">
                          Press <kbd>Enter</kbd> to add &ldquo;{roleSearch}&rdquo; as custom role
                        </div>
                      )}
                    </div>
                  )}
                </div>
                {step.assignedRole && (
                  <div className="wf-selected-role">
                    <PersonIcon /> {step.assignedRole}
                    <button className="wf-role-clear" onClick={() => onChange({ ...step, assignedRole: "" })}>
                      <XSmallIcon />
                    </button>
                  </div>
                )}
              </div>

              {/* ── Assigned Position ── */}
              <div className="wf-field">
                <label className="wf-field-label">Position (optional)</label>
                <span className="wf-field-hint">
                  Narrows within the role, e.g. &ldquo;Department Head&rdquo;
                </span>
                <div className="wf-role-picker">
                  <input
                    className="wf-input"
                    placeholder="Search or type position..."
                    value={showPosDropdown ? posSearch : (step.assignedPosition || "")}
                    onFocus={() => { setShowPosDropdown(true); setPosSearch(""); }}
                    onChange={(e) => { setPosSearch(e.target.value); setShowPosDropdown(true); }}
                    onBlur={() => setTimeout(() => setShowPosDropdown(false), 200)}
                    onKeyDown={(e) => { if (e.key === "Enter" && posSearch.trim()) selectPosition(posSearch.trim()); }}
                  />
                  {showPosDropdown && (
                    <div className="wf-role-dropdown">
                      {filteredPositions.length > 0 ? (
                        filteredPositions.slice(0, 10).map((p) => (
                          <button key={p} className={`wf-role-option ${step.assignedPosition === p ? "active" : ""}`} onMouseDown={() => selectPosition(p)}>
                            {p}
                          </button>
                        ))
                      ) : (
                        <div className="wf-role-empty">
                          Press <kbd>Enter</kbd> to add &ldquo;{posSearch}&rdquo;
                        </div>
                      )}
                    </div>
                  )}
                </div>
                {step.assignedPosition && (
                  <div className="wf-selected-role">
                    <PersonIcon /> {step.assignedPosition}
                    <button className="wf-role-clear" onClick={() => onChange({ ...step, assignedPosition: "" })}>
                      <XSmallIcon />
                    </button>
                  </div>
                )}
              </div>

              {/* ── Specific User Override ── */}
              <div className="wf-field">
                <label className="wf-field-label">Specific User (optional)</label>
                <span className="wf-field-hint">
                  Pin the task to a specific person. Overrides role/position.
                </span>
                <input
                  className="wf-input"
                  placeholder="user ID or email"
                  value={step.assignedUser || ""}
                  onChange={(e) => onChange({ ...step, assignedUser: e.target.value })}
                />
              </div>
            </div>

            {/* ── Allowed Task Actions ── */}
            <div className="wf-section">
              <div className="wf-section-label">Allowed Actions</div>
              <span className="wf-field-hint">
                What can the assignee do when they receive this task?
              </span>
              <div className="wf-task-actions-grid">
                {TASK_ACTION_OPTIONS.map((opt) => {
                  const selected = (step.taskActions || []).includes(opt.value);
                  return (
                    <button
                      key={opt.value}
                      className={`wf-task-action-btn ${selected ? "active" : ""}`}
                      style={selected ? { borderColor: opt.color, background: `${opt.color}18`, color: opt.color } : {}}
                      onClick={() => {
                        const prev = step.taskActions || [];
                        const next = selected
                          ? prev.filter((a) => a !== opt.value)
                          : [...prev, opt.value];
                        onChange({ ...step, taskActions: next });
                      }}
                    >
                      {opt.label}
                    </button>
                  );
                })}
              </div>
              {(step.taskActions || []).length >= 2 && (
                <span className="wf-field-hint wf-field-hint-branch">
                  <strong>Branching active:</strong> Each action above creates a separate output handle on the canvas.
                  Connect each handle to a different downstream node to branch the workflow based on the assignee&apos;s choice.
                </span>
              )}
            </div>

            {/* ── Form Template ── */}
            <div className="wf-section">
              <div className="wf-section-label">Form</div>
              <div className="wf-field">
                <label className="wf-field-label">Form Template ID (optional)</label>
                <span className="wf-field-hint">
                  If the assignee must fill a form before completing the task.
                </span>
                <input
                  className="wf-input"
                  placeholder="e.g. expense-report-form"
                  value={step.formTemplateId || ""}
                  onChange={(e) => onChange({ ...step, formTemplateId: e.target.value })}
                />
              </div>
            </div>

            {/* ── SLA ── */}
            <div className="wf-section">
              <div className="wf-section-label">SLA</div>
              <div className="wf-field">
                <label className="wf-field-label">Deadline (Working Days)</label>
                <div className="wf-sla-row">
                  <input
                    type="number"
                    className="wf-input wf-input-sla"
                    min={0}
                    max={90}
                    value={step.slaDays}
                    onChange={(e) =>
                      onChange({ ...step, slaDays: Math.max(0, Number(e.target.value)) })
                    }
                  />
                  <span className="wf-sla-label">days to complete (0 = no SLA)</span>
                </div>
              </div>
            </div>
          </>
        )}

        {/* ════════════════════════════════════════════
            ACTION NODE — Connector configuration
           ════════════════════════════════════════════ */}
        {nodeType === "action" && (
          <>
            <div className="wf-section">
              <div className="wf-section-label">Connector</div>
              <span className="wf-field-hint">
                Which external service should this action invoke?
              </span>
              <div className="wf-connector-type-grid">
                {(Object.keys(CONNECTOR_CONFIG) as ConnectorType[]).map((ctype) => {
                  const cfg = CONNECTOR_CONFIG[ctype];
                  const active = step.connector?.type === ctype;
                  return (
                    <button
                      key={ctype}
                      className={`wf-connector-type-btn ${active ? "active" : ""}`}
                      style={active ? { borderColor: cfg.color, background: `${cfg.color}14` } : {}}
                      onClick={() =>
                        onChange({
                          ...step,
                          connector: {
                            type: ctype,
                            params: step.connector?.type === ctype ? step.connector.params : {},
                          },
                        })
                      }
                    >
                      <span className="wf-connector-type-dot" style={{ background: cfg.color }} />
                      {cfg.label}
                    </button>
                  );
                })}
              </div>
            </div>

            {/* ── Connector params ── */}
            {step.connector?.type && (
              <div className="wf-section">
                <div className="wf-section-label">
                  {CONNECTOR_CONFIG[step.connector.type].label} Parameters
                </div>
                {CONNECTOR_CONFIG[step.connector.type].paramFields.map((field) => (
                  <div key={field.key} className="wf-field">
                    <label className="wf-field-label">
                      {field.label}
                      {field.required && <span className="wf-required-star">*</span>}
                    </label>
                    {field.options ? (
                      <select
                        className="wf-select"
                        value={step.connector?.params[field.key] || ""}
                        onChange={(e) =>
                          onChange({
                            ...step,
                            connector: {
                              ...step.connector!,
                              params: { ...step.connector!.params, [field.key]: e.target.value },
                            },
                          })
                        }
                      >
                        <option value="">Select...</option>
                        {field.options.map((opt) => (
                          <option key={opt} value={opt}>{opt}</option>
                        ))}
                      </select>
                    ) : field.multiline ? (
                      <textarea
                        className="wf-textarea"
                        rows={3}
                        placeholder={field.placeholder}
                        value={step.connector?.params[field.key] || ""}
                        onChange={(e) =>
                          onChange({
                            ...step,
                            connector: {
                              ...step.connector!,
                              params: { ...step.connector!.params, [field.key]: e.target.value },
                            },
                          })
                        }
                      />
                    ) : (
                      <input
                        className="wf-input"
                        placeholder={field.placeholder}
                        value={step.connector?.params[field.key] || ""}
                        onChange={(e) =>
                          onChange({
                            ...step,
                            connector: {
                              ...step.connector!,
                              params: { ...step.connector!.params, [field.key]: e.target.value },
                            },
                          })
                        }
                      />
                    )}
                  </div>
                ))}
                <span className="wf-field-hint">
                  Use <code>{"{{data.fieldname}}"}</code> to reference instance data dynamically.
                </span>
              </div>
            )}
          </>
        )}

        {/* ════════════════════════════════════════════
            CONDITION NODE
           ════════════════════════════════════════════ */}
        {nodeType === "condition" && (
          <div className="wf-section">
            <div className="wf-section-label">Condition</div>
            <div className="wf-field">
              <label className="wf-field-label">Expression</label>
              <input
                className="wf-input"
                placeholder='e.g. amount > 5000'
                value={step.condition || ""}
                onChange={(e) => onChange({ ...step, condition: e.target.value })}
              />
              <span className="wf-field-hint">
                Supports: <code>==</code> <code>!=</code> <code>&gt;</code> <code>&lt;</code> <code>&gt;=</code> <code>&lt;=</code>.
                References instance data fields by name.
              </span>
            </div>
            <div className="wf-condition-branches">
              <div className="wf-condition-branch yes">
                <span className="wf-condition-branch-dot" style={{ background: "#22c55e" }} />
                <span>Yes → follows left output</span>
              </div>
              <div className="wf-condition-branch no">
                <span className="wf-condition-branch-dot" style={{ background: "#ef4444" }} />
                <span>No → follows right output</span>
              </div>
            </div>
          </div>
        )}

        {/* ════════════════════════════════════════════
            PARALLEL NODE
           ════════════════════════════════════════════ */}
        {nodeType === "parallel" && (
          <div className="wf-section">
            <div className="wf-section-label">Parallel Branches</div>
            <div className="wf-field">
              <label className="wf-field-label">Number of Branches</label>
              <input
                type="number"
                className="wf-input"
                min={2}
                max={10}
                value={step.branches ?? 2}
                onChange={(e) =>
                  onChange({ ...step, branches: Math.max(2, Math.min(10, Number(e.target.value))) })
                }
              />
              <span className="wf-field-hint">
                All branches execute simultaneously. Connect each output handle to the next step.
                Use a Merge node downstream to synchronize them.
              </span>
            </div>
          </div>
        )}

        {/* ════════════════════════════════════════════
            MERGE NODE
           ════════════════════════════════════════════ */}
        {nodeType === "merge" && (
          <div className="wf-section">
            <div className="wf-section-label">Merge Configuration</div>
            <div className="wf-field">
              <label className="wf-field-label">Number of Inputs</label>
              <input
                type="number"
                className="wf-input"
                min={2}
                max={10}
                value={step.mergeInputs ?? 2}
                onChange={(e) =>
                  onChange({ ...step, mergeInputs: Math.max(2, Math.min(10, Number(e.target.value))) })
                }
              />
            </div>
            <div className="wf-field">
              <label className="wf-field-label">Input Requirements</label>
              <div className="wf-merge-inputs-list">
                {Array.from({ length: step.mergeInputs ?? 2 }, (_, i) => {
                  const handleId = `input-${i}`;
                  // undefined means all required by default
                  const allHandles = Array.from({ length: step.mergeInputs ?? 2 }, (_, j) => `input-${j}`);
                  const isRequired = step.requiredInputs === undefined || step.requiredInputs.includes(handleId);
                  return (
                    <div key={handleId} className="wf-merge-input-row">
                      <span className="wf-merge-input-name">Input {i + 1}</span>
                      <div className="wf-merge-toggle-group">
                        <button
                          className={`wf-merge-toggle-btn ${isRequired ? "active" : ""}`}
                          style={isRequired ? { background: "#ec4899", borderColor: "#ec4899", color: "#fff" } : {}}
                          onClick={() => {
                            if (!isRequired) {
                              const prev = step.requiredInputs ?? [];
                              onChange({ ...step, requiredInputs: [...prev, handleId] });
                            }
                          }}
                        >
                          Required
                        </button>
                        <button
                          className={`wf-merge-toggle-btn ${!isRequired ? "active" : ""}`}
                          style={!isRequired ? { background: "#64748b", borderColor: "#64748b", color: "#fff" } : {}}
                          onClick={() => {
                            if (isRequired) {
                              // start from all handles if never explicitly set
                              const prev = step.requiredInputs ?? allHandles;
                              onChange({ ...step, requiredInputs: prev.filter((id) => id !== handleId) });
                            }
                          }}
                        >
                          Optional
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
              <span className="wf-field-hint">
                Required inputs must all arrive before the merge continues.
                Optional inputs are accepted but won&apos;t block.
              </span>
            </div>
          </div>
        )}

        {/* ════════════════════════════════════════════
            END NODE — minimal
           ════════════════════════════════════════════ */}
        {nodeType === "end" && (
          <div className="wf-section">
            <div className="wf-section-label">End</div>
            <span className="wf-field-hint">
              This is a terminal node. The workflow instance completes when execution reaches here.
            </span>
          </div>
        )}

      </div>
    </div>
  );
}

/* ── Icons ── */
function XIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="20" height="20">
      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
    </svg>
  );
}

function XSmallIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="14" height="14">
      <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
    </svg>
  );
}

function PersonIcon() {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="14" height="14">
      <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" />
    </svg>
  );
}
