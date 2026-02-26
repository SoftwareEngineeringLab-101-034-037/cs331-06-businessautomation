"use client";

import { useState } from "react";
import type {
  WorkflowStep,
  WorkflowTrigger,
  TriggerType,
  StepActionType,
} from "@/types/workflow";
import {
  TRIGGER_CONFIG,
  STEP_ACTION_CONFIG,
  PRESET_ORG_ROLES,
} from "@/types/workflow";

/* ──────────────────────────────────────────────────────────────
   Trigger Editor
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
   Step Editor
   ────────────────────────────────────────────────────────────── */
interface StepEditorProps {
  step: WorkflowStep;
  stepIndex: number;
  onChange: (updated: WorkflowStep) => void;
  onClose: () => void;
}

export function StepEditor({ step, stepIndex, onChange, onClose }: StepEditorProps) {
  const [roleSearch, setRoleSearch] = useState("");
  const [showRoleDropdown, setShowRoleDropdown] = useState(false);

  const filteredRoles = PRESET_ORG_ROLES.filter((r) =>
    r.toLowerCase().includes(roleSearch.toLowerCase()),
  );

  function selectRole(role: string) {
    onChange({ ...step, assignedRole: role });
    setRoleSearch("");
    setShowRoleDropdown(false);
  }

  return (
    <div className="wf-editor">
      <div className="wf-editor-header">
        <h3 className="wf-editor-title">Step {stepIndex + 1} Configuration</h3>
        <button className="wf-editor-close" onClick={onClose}>
          <XIcon />
        </button>
      </div>

      <div className="wf-editor-body">
        {/* Step title */}
        <div className="wf-field">
          <label className="wf-field-label">Step Title</label>
          <input
            className="wf-input"
            placeholder="e.g. Manager Approval"
            value={step.title}
            onChange={(e) => onChange({ ...step, title: e.target.value })}
          />
        </div>

        {/* Description */}
        <div className="wf-field">
          <label className="wf-field-label">Description / Instructions</label>
          <textarea
            className="wf-textarea"
            placeholder="What should the assignee do at this step?"
            rows={3}
            value={step.description}
            onChange={(e) => onChange({ ...step, description: e.target.value })}
          />
        </div>

        {/* Action type */}
        <div className="wf-field">
          <label className="wf-field-label">Action Type</label>
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
                  <span
                    className="wf-action-dot"
                    style={{ background: cfg.color }}
                  />
                  {cfg.label}
                </button>
              );
            })}
          </div>
        </div>

        {/* Assigned role */}
        <div className="wf-field">
          <label className="wf-field-label">Assigned Role</label>
          <p className="wf-field-hint">
            Choose the organisational role responsible for this step
          </p>
          <div className="wf-role-picker">
            <input
              className="wf-input"
              placeholder="Search or type a custom role..."
              value={showRoleDropdown ? roleSearch : step.assignedRole}
              onFocus={() => {
                setShowRoleDropdown(true);
                setRoleSearch("");
              }}
              onChange={(e) => {
                setRoleSearch(e.target.value);
                setShowRoleDropdown(true);
              }}
              onBlur={() => {
                // Delay so click on option registers
                setTimeout(() => setShowRoleDropdown(false), 200);
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && roleSearch.trim()) {
                  selectRole(roleSearch.trim());
                }
              }}
            />
            {showRoleDropdown && (
              <div className="wf-role-dropdown">
                {filteredRoles.length > 0 ? (
                  filteredRoles.slice(0, 10).map((r) => (
                    <button
                      key={r}
                      className={`wf-role-option ${step.assignedRole === r ? "active" : ""}`}
                      onMouseDown={() => selectRole(r)}
                    >
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
              <button
                className="wf-role-clear"
                onClick={() => onChange({ ...step, assignedRole: "" })}
              >
                <XSmallIcon />
              </button>
            </div>
          )}
        </div>

        {/* SLA */}
        <div className="wf-field">
          <label className="wf-field-label">SLA (Working Days)</label>
          <div className="wf-sla-row">
            <input
              type="number"
              className="wf-input wf-input-sla"
              min={1}
              max={90}
              value={step.slaDays}
              onChange={(e) =>
                onChange({ ...step, slaDays: Math.max(1, Number(e.target.value)) })
              }
            />
            <span className="wf-sla-label">days to complete</span>
          </div>
        </div>

        {/* Required */}
        <div className="wf-field wf-field-row">
          <label className="wf-field-label" style={{ marginBottom: 0 }}>Required Step</label>
          <label className="settings-switch">
            <input
              type="checkbox"
              checked={step.isRequired}
              onChange={(e) => onChange({ ...step, isRequired: e.target.checked })}
            />
            <span className="settings-switch-slider" />
          </label>
        </div>
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
