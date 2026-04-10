"use client";

import { useMemo, useState } from "react";
import type {
  WorkflowStep,
  WorkflowTrigger,
  TriggerType,
  StepActionType,
  NodeType,
  ConnectorType,
  TaskAction,
  TaskDataVisibilityMode,
  ConditionConfig,
  ConditionRule,
  ConditionJoin,
  ConditionDataType,
  ConditionOperator,
  WorkflowDataField,
} from "@/types/workflow";
import {
  TRIGGER_CONFIG,
  STEP_ACTION_CONFIG,
  NODE_TYPE_CONFIG,
  PRESET_ORG_ROLES,
  CONNECTOR_CONFIG,
  TASK_ACTION_OPTIONS,
} from "@/types/workflow";
import {
  parseFieldMapping,
  serializeFieldMapping,
  parseFieldSchema,
  normalizeConditionDataType,
  inferConditionDataTypeFromFormFieldType,
  buildFieldSchemaJSON,
} from "@/lib/workflow-mapping";

const GOOGLE_FORMS_CREATE_URL = "https://docs.google.com/forms/u/0/create";

type GoogleFormField = {
  question_id: string;
  item_id?: string;
  title: string;
  required?: boolean;
  field_type?: string;
};

const CONDITION_DATA_TYPE_OPTIONS: Array<{ value: ConditionDataType; label: string }> = [
  { value: "text", label: "Text" },
  { value: "number", label: "Number" },
  { value: "boolean", label: "Yes / No" },
  { value: "date", label: "Date" },
  { value: "datetime", label: "Date & Time" },
  { value: "time", label: "Time" },
];

const CONDITION_OPERATORS_BY_TYPE: Record<ConditionDataType, Array<{ value: ConditionOperator; label: string; requiresValue?: boolean }>> = {
  text: [
    { value: "eq", label: "is" },
    { value: "neq", label: "is not" },
    { value: "contains", label: "contains" },
    { value: "not_contains", label: "does not contain" },
    { value: "starts_with", label: "starts with" },
    { value: "ends_with", label: "ends with" },
    { value: "is_empty", label: "is empty", requiresValue: false },
    { value: "is_not_empty", label: "is not empty", requiresValue: false },
  ],
  number: [
    { value: "eq", label: "=" },
    { value: "neq", label: "!=" },
    { value: "gt", label: ">" },
    { value: "gte", label: ">=" },
    { value: "lt", label: "<" },
    { value: "lte", label: "<=" },
  ],
  boolean: [
    { value: "eq", label: "is" },
    { value: "neq", label: "is not" },
  ],
  date: [
    { value: "eq", label: "=" },
    { value: "neq", label: "!=" },
    { value: "gt", label: ">" },
    { value: "gte", label: ">=" },
    { value: "lt", label: "<" },
    { value: "lte", label: "<=" },
  ],
  datetime: [
    { value: "eq", label: "=" },
    { value: "neq", label: "!=" },
    { value: "gt", label: ">" },
    { value: "gte", label: ">=" },
    { value: "lt", label: "<" },
    { value: "lte", label: "<=" },
  ],
  time: [
    { value: "eq", label: "=" },
    { value: "neq", label: "!=" },
    { value: "gt", label: ">" },
    { value: "gte", label: ">=" },
    { value: "lt", label: "<" },
    { value: "lte", label: "<=" },
  ],
};

function inferConditionDataTypeForField(field?: WorkflowDataField): ConditionDataType {
  if (!field) return "text";
  return normalizeConditionDataType(field.dataType);
}

function operatorRequiresValue(dataType: ConditionDataType, operator: ConditionOperator): boolean {
  const meta = CONDITION_OPERATORS_BY_TYPE[dataType].find((item) => item.value === operator);
  if (!meta) return true;
  return meta.requiresValue !== false;
}

function formatConditionRuleSummary(rule: ConditionRule): string {
  if (!rule.field || !rule.operator) return "";
  const opLabel = CONDITION_OPERATORS_BY_TYPE[rule.dataType]
    .find((item) => item.value === rule.operator)?.label || rule.operator;
  if (!operatorRequiresValue(rule.dataType, rule.operator)) {
    return `${rule.field} ${opLabel}`;
  }
  const value = String(rule.value || "").trim();
  if (!value) return `${rule.field} ${opLabel}`;
  return `${rule.field} ${opLabel} ${value}`;
}

function formatConditionSummary(config: ConditionConfig): string {
  const explicitLogic = String(config.logic || "").trim();
  if (explicitLogic) return explicitLogic;

  const parts = config.rules
    .map((rule) => formatConditionRuleSummary(rule))
    .filter((part) => part.length > 0);
  if (parts.length === 0) return "";
  const joinLabel = (config.join || "and").toUpperCase();
  return parts.join(` ${joinLabel} `);
}

function buildDefaultConditionLogic(ruleCount: number, join: ConditionJoin): string {
  if (ruleCount <= 0) return "";
  if (ruleCount === 1) return "1";
  const op = join === "or" ? " OR " : " AND ";
  return Array.from({ length: ruleCount }, (_, idx) => String(idx + 1)).join(op);
}

function truncateConditionFieldLabel(label: string, maxLength = 28): string {
  const text = String(label || "").trim();
  if (text.length <= maxLength) return text;
  return `${text.slice(0, maxLength - 1)}…`;
}

function tokenizeConditionLogicInput(raw: string): string[] {
  const matches = String(raw || "").toUpperCase().match(/\d+|AND|OR|\(|\)/g);
  return matches || [];
}

function formatConditionLogicTokens(tokens: string[]): string {
  let out = "";
  for (const token of tokens) {
    if (token === "(") {
      out = out ? `${out} (` : "(";
      continue;
    }
    if (token === ")") {
      out = `${out})`;
      continue;
    }
    if (token === "AND" || token === "OR") {
      out = out ? `${out} ${token}` : token;
      continue;
    }
    if (!out || out.endsWith("(")) {
      out = `${out}${token}`;
    } else {
      out = `${out} ${token}`;
    }
  }
  return out.trim();
}

function appendConditionLogicToken(current: string, token: string): string {
  const normalizedToken = String(token || "").trim().toUpperCase();
  if (!normalizedToken) return String(current || "").trim();

  const tokens = tokenizeConditionLogicInput(current);
  const last = tokens[tokens.length - 1] || "";

  if ((normalizedToken === "AND" || normalizedToken === "OR") && (tokens.length === 0 || last === "(" || last === "AND" || last === "OR")) {
    return formatConditionLogicTokens(tokens);
  }
  if (normalizedToken === ")" && (tokens.length === 0 || last === "(" || last === "AND" || last === "OR")) {
    return formatConditionLogicTokens(tokens);
  }

  tokens.push(normalizedToken);
  return formatConditionLogicTokens(tokens);
}

function popConditionLogicToken(current: string): string {
  const tokens = tokenizeConditionLogicInput(current);
  if (tokens.length === 0) return "";
  tokens.pop();
  return formatConditionLogicTokens(tokens);
}

function normalizeAssignmentToken(raw: string): string {
  const text = String(raw || "").trim();
  if (!text) return "";
  if (!text.startsWith("#") && !text.startsWith("@")) return "";
  const prefix = text[0];
  const body = text.slice(1).trim().replace(/\s+/g, " ");
  if (!body) return "";
  return `${prefix}${body}`;
}

function parseTaskAssignmentTargets(rawTargets: string[]): { targets: string[]; roles: string[]; users: string[] } {
  const targets: string[] = [];
  const roles: string[] = [];
  const users: string[] = [];
  const seenTargets = new Set<string>();
  const seenRoles = new Set<string>();
  const seenUsers = new Set<string>();

  for (const rawTarget of rawTargets) {
    const token = normalizeAssignmentToken(rawTarget);
    if (!token) continue;
    const lowered = token.toLowerCase();
    if (seenTargets.has(lowered)) continue;
    seenTargets.add(lowered);
    targets.push(token);

    if (token.startsWith("#")) {
      const role = token.slice(1).trim();
      const roleKey = role.toLowerCase();
      if (role && !seenRoles.has(roleKey)) {
        seenRoles.add(roleKey);
        roles.push(role);
      }
      continue;
    }

    if (token.startsWith("@")) {
      const userID = token.slice(1).trim();
      const userKey = userID.toLowerCase();
      if (userID && !seenUsers.has(userKey)) {
        seenUsers.add(userKey);
        users.push(userID);
      }
    }
  }

  return { targets, roles, users };
}

function buildTaskAssignmentTargets(rawTargets: string[] | undefined, fallbackRole?: string, fallbackUser?: string): string[] {
  if (Array.isArray(rawTargets) && rawTargets.length > 0) {
    return rawTargets;
  }

  const fallbackTargets: string[] = [];
  const role = String(fallbackRole || "").trim();
  const user = String(fallbackUser || "").trim();
  if (role) fallbackTargets.push(`#${role}`);
  if (user) fallbackTargets.push(`@${user}`);
  return fallbackTargets;
}

function tokenizeConditionLogicExpression(raw: string): { tokens: string[]; error?: string } {
  const input = String(raw || "").trim();
  if (!input) {
    return { tokens: [], error: "logic expression is empty" };
  }

  const tokens: string[] = [];
  let index = 0;

  while (index < input.length) {
    const ch = input[index];

    if (/\s/.test(ch)) {
      index += 1;
      continue;
    }

    if (ch === "(" || ch === ")") {
      tokens.push(ch);
      index += 1;
      continue;
    }

    if (/\d/.test(ch)) {
      let end = index + 1;
      while (end < input.length && /\d/.test(input[end])) end += 1;
      tokens.push(input.slice(index, end));
      index = end;
      continue;
    }

    if (/[a-z]/i.test(ch)) {
      let end = index + 1;
      while (end < input.length && /[a-z]/i.test(input[end])) end += 1;
      const word = input.slice(index, end).toUpperCase();
      if (word !== "AND" && word !== "OR") {
        return { tokens: [], error: `unsupported token "${word}"` };
      }
      tokens.push(word);
      index = end;
      continue;
    }

    return { tokens: [], error: `invalid character "${ch}"` };
  }

  return { tokens };
}

function validateConditionLogicExpression(raw: string, ruleCount: number): string | null {
  const tokenized = tokenizeConditionLogicExpression(raw);
  if (tokenized.error) return tokenized.error;

  const tokens = tokenized.tokens;
  if (tokens.length === 0) return "logic expression is empty";

  let position = 0;

  const parseExpr = (): string | null => {
    const termErr = parseTerm();
    if (termErr) return termErr;
    while (position < tokens.length && tokens[position] === "OR") {
      position += 1;
      const nextErr = parseTerm();
      if (nextErr) return nextErr;
    }
    return null;
  };

  const parseTerm = (): string | null => {
    const factorErr = parseFactor();
    if (factorErr) return factorErr;
    while (position < tokens.length && tokens[position] === "AND") {
      position += 1;
      const nextErr = parseFactor();
      if (nextErr) return nextErr;
    }
    return null;
  };

  const parseFactor = (): string | null => {
    if (position >= tokens.length) return "unexpected end of logic expression";

    const token = tokens[position];
    if (token === "(") {
      position += 1;
      const nestedErr = parseExpr();
      if (nestedErr) return nestedErr;
      if (position >= tokens.length || tokens[position] !== ")") {
        return "missing closing parenthesis";
      }
      position += 1;
      return null;
    }

    const ref = Number(token);
    if (!Number.isInteger(ref) || ref < 1) {
      return `invalid rule reference "${token}"`;
    }
    if (ref > ruleCount) {
      return `rule reference ${ref} is out of range (max ${ruleCount})`;
    }

    position += 1;
    return null;
  };

  const parseErr = parseExpr();
  if (parseErr) return parseErr;
  if (position !== tokens.length) {
    return `unexpected token "${tokens[position]}"`;
  }
  return null;
}

function parseCommaSeparatedList(raw: string): string[] {
  const out: string[] = [];
  const seen = new Set<string>();
  for (const item of raw.split(",")) {
    const key = item.trim();
    if (!key || seen.has(key)) continue;
    seen.add(key);
    out.push(key);
  }
  return out;
}

function extractGoogleFormID(formURL: string): string {
  const match = formURL.match(/\/forms\/d\/(?:e\/)?([^/]+)/i);
  return match?.[1] || "";
}

/* ──────────────────────────────────────────────────────────────
   Trigger Editor  (used when user clicks the Start node)
   ────────────────────────────────────────────────────────────── */
interface TriggerEditorProps {
  trigger: WorkflowTrigger;
  onChange: (t: WorkflowTrigger) => void;
  availableForms?: Array<{
    form_id: string;
    title: string;
    responder_uri?: string;
  }>;
  formsLoading?: boolean;
  formsError?: string | null;
  googleAuthConfigured?: boolean;
  googleConnected?: boolean;
  googleConnectURL?: string;
  onGoogleConnect?: () => void;
  onRefreshForms?: () => void;
  formFields?: GoogleFormField[];
  formFieldsLoading?: boolean;
  formFieldsError?: string | null;
  onRefreshFormFields?: () => void;
  onApplySuggestedMapping?: () => void;
  onClose: () => void;
}

export function TriggerEditor({
  trigger,
  onChange,
  availableForms = [],
  formsLoading = false,
  formsError = null,
  googleAuthConfigured = true,
  googleConnected = false,
  googleConnectURL,
  onGoogleConnect,
  onRefreshForms,
  formFields = [],
  formFieldsLoading = false,
  formFieldsError = null,
  onRefreshFormFields,
  onApplySuggestedMapping,
  onClose,
}: TriggerEditorProps) {
  const mapping = parseFieldMapping(trigger.config.field_mapping || "");
  const parsedSchema = parseFieldSchema(trigger.config.field_schema || "");
  const schemaByQuestionID = new Map(parsedSchema.map((item) => [item.question_id, item]));
  const schemaTypeOverrides: Record<string, ConditionDataType> = {};
  for (const item of parsedSchema) {
    if (item.data_type) {
      schemaTypeOverrides[item.question_id] = normalizeConditionDataType(item.data_type);
    }
  }

  const buildFallbackFieldSchema = (
    nextMapping: Record<string, string>,
    overrides: Record<string, ConditionDataType>,
  ) => {
    const rows = new Map<string, {
      question_id: string;
      title: string;
      required: boolean;
      field_type: string;
      variable: string;
      data_type: ConditionDataType;
    }>();

    for (const [questionIDRaw, variableRaw] of Object.entries(nextMapping)) {
      const questionID = String(questionIDRaw || "").trim();
      const variable = String(variableRaw || "").trim();
      if (!questionID || !variable) continue;

      const existing = schemaByQuestionID.get(questionID);
      const resolvedType = normalizeConditionDataType(
        overrides[questionID]
        || existing?.data_type
        || inferConditionDataTypeFromFormFieldType(existing?.field_type),
      );

      rows.set(questionID, {
        question_id: questionID,
        title: String(existing?.title || questionID).trim() || questionID,
        required: Boolean(existing?.required),
        field_type: String(existing?.field_type || "text").trim() || "text",
        variable,
        data_type: resolvedType,
      });
    }

    for (const item of parsedSchema) {
      const questionID = String(item.question_id || "").trim();
      if (!questionID || rows.has(questionID)) continue;

      const variable = String(nextMapping[questionID] || item.variable || "").trim();
      if (!variable) continue;

      const resolvedType = normalizeConditionDataType(
        overrides[questionID]
        || item.data_type
        || inferConditionDataTypeFromFormFieldType(item.field_type),
      );

      rows.set(questionID, {
        question_id: questionID,
        title: String(item.title || questionID).trim() || questionID,
        required: Boolean(item.required),
        field_type: String(item.field_type || "text").trim() || "text",
        variable,
        data_type: resolvedType,
      });
    }

    return JSON.stringify(Array.from(rows.values()));
  };

  const manualMappingRows = (() => {
    const rows = new Map<string, {
      questionID: string;
      title: string;
      required: boolean;
      variable: string;
      dataType: ConditionDataType;
    }>();

    for (const [questionIDRaw, variableRaw] of Object.entries(mapping)) {
      const questionID = String(questionIDRaw || "").trim();
      const variable = String(variableRaw || "").trim();
      if (!questionID || !variable) continue;

      const existing = schemaByQuestionID.get(questionID);
      const dataType = normalizeConditionDataType(
        schemaTypeOverrides[questionID]
        || existing?.data_type
        || inferConditionDataTypeFromFormFieldType(existing?.field_type),
      );

      rows.set(questionID, {
        questionID,
        title: String(existing?.title || questionID).trim() || questionID,
        required: Boolean(existing?.required),
        variable,
        dataType,
      });
    }

    for (const item of parsedSchema) {
      const questionID = String(item.question_id || "").trim();
      const variable = String(item.variable || "").trim();
      if (!questionID || !variable || rows.has(questionID)) continue;

      const dataType = normalizeConditionDataType(
        schemaTypeOverrides[questionID]
        || item.data_type
        || inferConditionDataTypeFromFormFieldType(item.field_type),
      );

      rows.set(questionID, {
        questionID,
        title: String(item.title || questionID).trim() || questionID,
        required: Boolean(item.required),
        variable,
        dataType,
      });
    }

    return Array.from(rows.values()).sort((a, b) => a.questionID.localeCompare(b.questionID));
  })();

  const rebuildSchema = (
    nextMapping: Record<string, string>,
    overrides: Record<string, ConditionDataType> = schemaTypeOverrides,
  ) => {
    if (formFields.length === 0) {
      return buildFallbackFieldSchema(nextMapping, overrides);
    }
    return buildFieldSchemaJSON(formFields, nextMapping, {
      dataTypeOverrides: overrides,
      existingSchemaRaw: trigger.config.field_schema || "",
    });
  };

  const updateFieldAlias = (questionID: string, alias: string) => {
    const next = { ...mapping };
    const normalized = alias.trim();
    if (!normalized) {
      delete next[questionID];
    } else {
      next[questionID] = normalized;
    }
    onChange({
      ...trigger,
      config: {
        ...trigger.config,
        field_mapping: serializeFieldMapping(next),
        field_schema: rebuildSchema(next),
      },
    });
  };

  const updateFieldDataType = (questionID: string, dataType: ConditionDataType) => {
    const nextOverrides = { ...schemaTypeOverrides, [questionID]: normalizeConditionDataType(dataType) };
    onChange({
      ...trigger,
      config: {
        ...trigger.config,
        field_mapping: serializeFieldMapping(mapping),
        field_schema: rebuildSchema(mapping, nextOverrides),
      },
    });
  };

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
          <>
            <div className="wf-field">
              <label className="wf-field-label">Google Form</label>
              <div className="wf-field-row">
                <select
                  className="wf-select"
                  value={trigger.config.form_id || ""}
                  onChange={(e) => {
                    const selectedFormID = e.target.value;
                    const selected = availableForms.find((f) => f.form_id === selectedFormID);
                    onChange({
                      ...trigger,
                      config: {
                        ...trigger.config,
                        form_id: selectedFormID,
                        form_url: selectedFormID ? (selected?.responder_uri || "") : "",
                        formName: selectedFormID ? (selected?.title || "") : "",
                        field_mapping: "",
                        field_schema: "",
                      },
                    });
                  }}
                >
                  <option value="">Select an existing form...</option>
                  {availableForms.map((form) => (
                    <option key={form.form_id} value={form.form_id}>
                      {form.title}
                    </option>
                  ))}
                </select>
                <button
                  className="action-btn action-btn-outline"
                  type="button"
                  onClick={() => onRefreshForms?.()}
                  disabled={formsLoading}
                  style={{ marginTop: 8 }}
                >
                  {formsLoading ? "Refreshing..." : "Refresh"}
                </button>
                <button
                  className="action-btn action-btn-outline"
                  type="button"
                  onClick={() => window.open(GOOGLE_FORMS_CREATE_URL, "_blank", "noopener,noreferrer")}
                  style={{ marginTop: 8 }}
                >
                  Create New Form
                </button>
              </div>
              {formsError && <span className="wf-field-hint" style={{ color: "#b45309" }}>{formsError}</span>}
              <span className="wf-field-hint">
                If you create a new form, click Refresh to load it in this list.
              </span>
              {!googleAuthConfigured && (
                <span className="wf-field-hint">
                  Google Forms integration is not configured yet. A platform admin needs to set OAuth credentials in the Integrations service.
                </span>
              )}
              {googleAuthConfigured && !googleConnected && (googleConnectURL || onGoogleConnect) && (
                <span className="wf-field-hint">
                  Google account not connected.{" "}
                  {onGoogleConnect ? (
                    <button
                      type="button"
                      onClick={onGoogleConnect}
                      style={{ background: "none", border: "none", padding: 0, color: "var(--accent)", textDecoration: "underline", cursor: "pointer", font: "inherit" }}
                    >
                      Connect Google Forms
                    </button>
                  ) : (
                    <a href={googleConnectURL} target="_blank" rel="noreferrer">Connect Google Forms</a>
                  )}
                </span>
              )}
            </div>

            <div className="wf-field">
              <label className="wf-field-label">Form ID</label>
              <input
                className="wf-input"
                placeholder="google-form-id"
                value={trigger.config.form_id || ""}
                onChange={(e) => {
                  const formID = e.target.value;
                  onChange({
                    ...trigger,
                    config: {
                      ...trigger.config,
                      form_id: formID,
                      form_url: "",
                      field_mapping: "",
                      field_schema: "",
                    },
                  });
                }}
              />
            </div>

            <div className="wf-field">
              <label className="wf-field-label">Form URL (optional)</label>
              <input
                className="wf-input"
                placeholder="https://docs.google.com/forms/d/.../viewform"
                value={trigger.config.form_url || ""}
                onChange={(e) => {
                  const formURL = e.target.value;
                  const extractedFormID = extractGoogleFormID(formURL);
                  onChange({
                    ...trigger,
                    config: {
                      ...trigger.config,
                      form_url: formURL,
                      form_id: extractedFormID,
                      field_mapping: "",
                      field_schema: "",
                    },
                  });
                }}
              />
            </div>

            <div className="wf-field">
              <label className="wf-field-label">Field Mapping</label>
              <div className="integration-actions" style={{ marginBottom: 8 }}>
                <button
                  className="action-btn action-btn-outline"
                  type="button"
                  onClick={() => onRefreshFormFields?.()}
                  disabled={formFieldsLoading || !(trigger.config.form_id || trigger.config.form_url)}
                >
                  {formFieldsLoading ? "Loading fields..." : "Load Form Fields"}
                </button>
                <button
                  className="action-btn action-btn-outline"
                  type="button"
                  onClick={onApplySuggestedMapping}
                  disabled={formFieldsLoading || formFields.length === 0}
                >
                  Use Suggested Mapping
                </button>
              </div>
              {formFieldsError && (
                <span className="wf-field-hint" style={{ color: "#b45309" }}>{formFieldsError}</span>
              )}
              {formFields.length > 0 ? (
                <div style={{ display: "grid", gap: 8 }}>
                  {formFields.map((field) => {
                    const resolvedDataType =
                      schemaTypeOverrides[field.question_id] ||
                      inferConditionDataTypeFromFormFieldType(field.field_type);
                    return (
                      <div key={field.question_id} className="wf-field-row" style={{ alignItems: "center", gap: 8 }}>
                        <div style={{ flex: 1, minWidth: 180 }}>
                          <div style={{ fontSize: "0.82rem", fontWeight: 600 }}>{field.title}</div>
                          <div className="wf-field-hint">{field.question_id}{field.required ? " • required" : ""}</div>
                        </div>
                        <input
                          className="wf-input"
                          style={{ flex: 1 }}
                          placeholder="workflow_variable_name"
                          value={mapping[field.question_id] || ""}
                          onChange={(e) => updateFieldAlias(field.question_id, e.target.value)}
                        />
                        <select
                          className="wf-select"
                          style={{ width: 150 }}
                          value={resolvedDataType}
                          onChange={(e) => updateFieldDataType(field.question_id, e.target.value as ConditionDataType)}
                        >
                          {CONDITION_DATA_TYPE_OPTIONS.map((option) => (
                            <option key={option.value} value={option.value}>{option.label}</option>
                          ))}
                        </select>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div style={{ display: "grid", gap: 8 }}>
                  <input
                    className="wf-input"
                    placeholder="questionId:name, amountQuestion:amount"
                    value={trigger.config.field_mapping || ""}
                    onChange={(e) =>
                      onChange({ ...trigger, config: { ...trigger.config, field_mapping: e.target.value } })
                    }
                  />

                  {manualMappingRows.length > 0 && (
                    <>
                      <span className="wf-field-hint">Edit datatype for mapped global keys:</span>
                      {manualMappingRows.map((row) => (
                        <div key={row.questionID} className="wf-field-row" style={{ alignItems: "center", gap: 8 }}>
                          <div style={{ flex: 1, minWidth: 180 }}>
                            <div style={{ fontSize: "0.82rem", fontWeight: 600 }}>{row.title}</div>
                            <div className="wf-field-hint">{row.questionID}{row.required ? " • required" : ""}</div>
                          </div>
                          <input
                            className="wf-input"
                            style={{ flex: 1 }}
                            placeholder="workflow_variable_name"
                            value={row.variable}
                            onChange={(e) => updateFieldAlias(row.questionID, e.target.value)}
                          />
                          <select
                            className="wf-select"
                            style={{ width: 150 }}
                            value={row.dataType}
                            onChange={(e) => updateFieldDataType(row.questionID, e.target.value as ConditionDataType)}
                          >
                            {CONDITION_DATA_TYPE_OPTIONS.map((option) => (
                              <option key={option.value} value={option.value}>{option.label}</option>
                            ))}
                          </select>
                        </div>
                      ))}
                    </>
                  )}
                </div>
              )}
              <span className="wf-field-hint">
                Mapped values become global workflow data keys and are available as template tokens like {"{{data.your_field}}"}.
              </span>
              <span className="wf-field-hint">
                If your Google Form collects emails, map the "Respondent Email" field to expose it as a workflow variable.
              </span>
            </div>
          </>
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
  availableUsers?: Array<{
    id: string;
    name?: string;
    email?: string;
  }>;
  suggestedDataKeys?: string[];
  availableConditionFields?: WorkflowDataField[];
  availableForms?: Array<{
    form_id: string;
    title: string;
    responder_uri?: string;
  }>;
  formsLoading?: boolean;
  onRefreshForms?: () => void;
  availableGmailAccounts?: Array<{
    account_id: string;
    account_email: string;
    account_name?: string;
    is_primary?: boolean;
  }>;
  gmailAccountsLoading?: boolean;
  gmailAccountsError?: string | null;
  onRefreshGmailAccounts?: () => void;
}

export function StepEditor({
  step,
  stepIndex,
  onChange,
  onClose,
  availableRoles = PRESET_ORG_ROLES,
  availableUsers = [],
  suggestedDataKeys = [],
  availableConditionFields = [],
  availableForms = [],
  formsLoading = false,
  onRefreshForms,
  availableGmailAccounts = [],
  gmailAccountsLoading = false,
  gmailAccountsError = null,
  onRefreshGmailAccounts,
}: StepEditorProps) {
  const [assignmentSearch, setAssignmentSearch] = useState("");
  const [showAssignDropdown, setShowAssignDropdown] = useState(false);

  const normalizedTaskAssignments = useMemo(() => {
    return parseTaskAssignmentTargets(
      buildTaskAssignmentTargets(step.assignmentTargets, step.assignedRole, step.assignedUser),
    );
  }, [step.assignmentTargets, step.assignedRole, step.assignedUser]);

  const assignmentTargets = normalizedTaskAssignments.targets;

  const assignmentUsersByID = useMemo(() => {
    const lookup = new Map<string, { id: string; name?: string; email?: string }>();
    for (const user of availableUsers) {
      const id = String(user.id || "").trim();
      if (!id) continue;
      lookup.set(id, user);
    }
    return lookup;
  }, [availableUsers]);

  const assignmentSuggestions = useMemo<Array<{ token: string; label: string; subtitle?: string }>>(() => {
    const query = assignmentSearch.trim();
    if (!query) return [] as Array<{ token: string; label: string; subtitle?: string }>;

    if (query.startsWith("#")) {
      const roleQuery = query.slice(1).trim().toLowerCase();
      return availableRoles
        .filter((role) => roleQuery.length === 0 || role.toLowerCase().includes(roleQuery))
        .slice(0, 10)
        .map((role) => ({
          token: `#${role}`,
          label: role,
        }));
    }

    if (query.startsWith("@")) {
      const userQuery = query.slice(1).trim().toLowerCase();
      return availableUsers
        .filter((user) => {
          if (!userQuery) return true;
          const id = String(user.id || "").toLowerCase();
          const name = String(user.name || "").toLowerCase();
          const email = String(user.email || "").toLowerCase();
          return id.includes(userQuery) || name.includes(userQuery) || email.includes(userQuery);
        })
        .slice(0, 10)
        .map((user) => {
          const id = String(user.id || "").trim();
          const label = String(user.name || user.email || id);
          const email = String(user.email || "").trim();
          return {
            token: `@${id}`,
            label,
            subtitle: email && email !== label ? email : undefined,
          };
        });
    }

    return [] as Array<{ token: string; label: string; subtitle?: string }>;
  }, [assignmentSearch, availableRoles, availableUsers]);
  const visibilityMode: TaskDataVisibilityMode = step.taskDataVisibility || "all";
  const visibleDataKeys = step.visibleDataKeys || [];
  const conditionFieldLookup = new Map(availableConditionFields.map((field) => [field.key, field]));

  const normalizeRule = (rule?: Partial<ConditionRule>): ConditionRule => {
    const field = typeof rule?.field === "string" ? rule.field : "";
    const inferredDataType = inferConditionDataTypeForField(conditionFieldLookup.get(field));
    const dataType = field
      ? inferredDataType
      : normalizeConditionDataType(rule?.dataType || inferredDataType);
    const allowedOperators = CONDITION_OPERATORS_BY_TYPE[dataType] || CONDITION_OPERATORS_BY_TYPE.text;
    const operator = allowedOperators.some((item) => item.value === rule?.operator)
      ? (rule!.operator as ConditionOperator)
      : allowedOperators[0].value;
    const value = typeof rule?.value === "string" ? rule.value : "";
    return {
      field,
      dataType,
      operator,
      value: operatorRequiresValue(dataType, operator) ? value : "",
    };
  };

  const hasExplicitConditionLogic = typeof step.conditionConfig?.logic === "string";
  const explicitConditionLogic = hasExplicitConditionLogic
    ? String(step.conditionConfig?.logic || "").trim()
    : undefined;

  const baseConditionConfig: ConditionConfig = {
    join: step.conditionConfig?.join === "or" ? "or" : "and",
    logic: explicitConditionLogic || "",
    rules: Array.isArray(step.conditionConfig?.rules) && step.conditionConfig.rules.length > 0
      ? step.conditionConfig.rules.map((rule) => normalizeRule(rule))
      : [normalizeRule()],
  };

  const defaultConditionLogic = buildDefaultConditionLogic(baseConditionConfig.rules.length, baseConditionConfig.join);
  const activeConditionLogic = hasExplicitConditionLogic
    ? String(explicitConditionLogic || "").trim()
    : defaultConditionLogic;
  const logicRuleTokens = baseConditionConfig.rules.map((_, index) => String(index + 1));
  const hasMultipleConditionRules = baseConditionConfig.rules.length > 1;
  const conditionLogicTouched = hasMultipleConditionRules && hasExplicitConditionLogic;
  const conditionLogicError = hasMultipleConditionRules
    ? validateConditionLogicExpression(activeConditionLogic, baseConditionConfig.rules.length)
    : null;
  const conditionLogicBorderColor = !conditionLogicTouched
    ? "var(--border)"
    : conditionLogicError
      ? "#ef4444"
      : "#22c55e";

  function updateAssignmentTargets(nextTargets: string[]) {
    const normalized = parseTaskAssignmentTargets(nextTargets);
    onChange({
      ...step,
      assignmentTargets: normalized.targets,
      assignedRole: normalized.roles[0] || "",
      assignedPosition: "",
      assignedUser: "",
    });
  }

  function addAssignmentToken(raw: string) {
    const token = normalizeAssignmentToken(raw);
    if (!token) return;
    updateAssignmentTargets([...assignmentTargets, token]);
    setAssignmentSearch("");
    setShowAssignDropdown(false);
  }

  function removeAssignmentToken(token: string) {
    updateAssignmentTargets(assignmentTargets.filter((item) => item !== token));
  }

  function assignmentTokenLabel(token: string): string {
    if (token.startsWith("#")) {
      return token;
    }
    if (token.startsWith("@")) {
      const userID = token.slice(1).trim();
      const user = assignmentUsersByID.get(userID);
      const label = String(user?.name || user?.email || userID);
      return `@${label}`;
    }
    return token;
  }

  function updateConditionConfig(next: ConditionConfig & { logic?: string }) {
    const normalizedJoin: ConditionJoin = next.join === "or" ? "or" : "and";
    const normalizedRules = next.rules.map((rule) => normalizeRule(rule));
    const hasLogic = typeof next.logic === "string";
    const normalizedLogic = hasLogic ? String(next.logic || "").trim() : undefined;
    const nextConditionConfig: ConditionConfig = {
      join: normalizedJoin,
      rules: normalizedRules,
    };
    if (hasLogic) {
      nextConditionConfig.logic = normalizedLogic;
    }

    onChange({
      ...step,
      conditionConfig: nextConditionConfig,
      condition: formatConditionSummary({
        join: normalizedJoin,
        logic: normalizedLogic,
        rules: normalizedRules,
      }),
    });
  }

  function updateConditionRule(index: number, partial: Partial<ConditionRule>) {
    const nextRules = baseConditionConfig.rules.map((rule, i) => (i === index ? normalizeRule({ ...rule, ...partial }) : rule));
    updateConditionConfig({ ...baseConditionConfig, logic: explicitConditionLogic, rules: nextRules });
  }

  function addConditionRule() {
    const nextRules = [...baseConditionConfig.rules, normalizeRule()];
    const nextLogic = hasExplicitConditionLogic
      ? String(explicitConditionLogic || "")
      : undefined;
    updateConditionConfig({
      ...baseConditionConfig,
      logic: nextLogic,
      rules: nextRules,
    });
  }

  function removeConditionRule(index: number) {
    const kept = baseConditionConfig.rules.filter((_, i) => i !== index);
    const nextRules = kept.length > 0 ? kept : [normalizeRule()];
    const nextLogic = hasExplicitConditionLogic
      ? buildDefaultConditionLogic(nextRules.length, baseConditionConfig.join)
      : undefined;
    updateConditionConfig({
      ...baseConditionConfig,
      logic: nextLogic,
      rules: nextRules,
    });
  }

  function updateConditionLogic(logic: string) {
    updateConditionConfig({
      ...baseConditionConfig,
      logic,
    });
  }

  function insertConditionLogicToken(token: string) {
    updateConditionLogic(appendConditionLogicToken(activeConditionLogic, token));
  }

  function removeConditionLogicToken() {
    updateConditionLogic(popConditionLogicToken(activeConditionLogic));
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

            <div className="wf-section">
              <div className="wf-section-label">Assignment</div>
              <div className="wf-field">
                <label className="wf-field-label">Assign to</label>
                <span className="wf-field-hint">
                  Use <strong>#</strong> for workflow roles and <strong>@</strong> for specific users. You can add multiple.
                </span>
                <div className="wf-role-picker">
                  <input
                    className="wf-input"
                    placeholder="e.g. #Finance Reviewer or @user_123"
                    value={assignmentSearch}
                    onFocus={() => setShowAssignDropdown(true)}
                    onChange={(e) => {
                      setAssignmentSearch(e.target.value);
                      setShowAssignDropdown(true);
                    }}
                    onBlur={() => setTimeout(() => setShowAssignDropdown(false), 200)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault();
                        if (assignmentSearch.trim()) addAssignmentToken(assignmentSearch);
                      }
                    }}
                  />
                  {showAssignDropdown && (
                    <div className="wf-role-dropdown">
                      {assignmentSuggestions.length > 0 ? (
                        assignmentSuggestions.map((item) => (
                          <button
                            key={item.token}
                            className="wf-role-option"
                            onMouseDown={() => addAssignmentToken(item.token)}
                          >
                            <span>{item.label}</span>
                            {item.subtitle ? <span style={{ marginLeft: "auto", opacity: 0.72 }}>{item.subtitle}</span> : null}
                          </button>
                        ))
                      ) : assignmentSearch.trim().startsWith("#") || assignmentSearch.trim().startsWith("@") ? (
                        <div className="wf-role-empty">
                          Press <kbd>Enter</kbd> to add <strong>{assignmentSearch.trim()}</strong>
                        </div>
                      ) : (
                        <div className="wf-role-empty">
                          Start with <kbd>#</kbd> for role or <kbd>@</kbd> for user.
                        </div>
                      )}
                    </div>
                  )}
                </div>
                {assignmentTargets.length > 0 ? (
                  <div style={{ display: "flex", flexWrap: "wrap", gap: 8, marginTop: 6 }}>
                    {assignmentTargets.map((token) => (
                      <div key={token} className="wf-selected-role" style={{ marginTop: 0 }}>
                        <PersonIcon /> {assignmentTokenLabel(token)}
                        <button className="wf-role-clear" onClick={() => removeAssignmentToken(token)}>
                          <XSmallIcon />
                        </button>
                      </div>
                    ))}
                  </div>
                ) : null}
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

            {/* ── Assignee data visibility ── */}
            <div className="wf-section">
              <div className="wf-section-label">Assignee Data Visibility</div>
              <span className="wf-field-hint">
                Control which instance values are shown in the assignee task drawer.
              </span>

              <div className="wf-task-actions-grid" style={{ marginTop: 8 }}>
                {([
                  { value: "all", label: "Show All Data" },
                  { value: "selected", label: "Select Keys" },
                  { value: "none", label: "Hide All Data" },
                ] as Array<{ value: TaskDataVisibilityMode; label: string }>).map((opt) => {
                  const selected = visibilityMode === opt.value;
                  return (
                    <button
                      key={opt.value}
                      className={`wf-task-action-btn ${selected ? "active" : ""}`}
                      onClick={() => onChange({ ...step, taskDataVisibility: opt.value })}
                      style={selected ? { borderColor: "#3b82f6", background: "rgba(59,130,246,0.16)", color: "#1d4ed8" } : {}}
                    >
                      {opt.label}
                    </button>
                  );
                })}
              </div>

              {visibilityMode === "selected" && (
                <div className="wf-field" style={{ marginTop: 10 }}>
                  <label className="wf-field-label">Visible Keys</label>
                  <input
                    className="wf-input"
                    placeholder="amount, employee_name, form_response_id"
                    value={visibleDataKeys.join(", ")}
                    onChange={(e) => onChange({ ...step, visibleDataKeys: parseCommaSeparatedList(e.target.value) })}
                  />
                  <span className="wf-field-hint">Comma-separated top-level instance keys.</span>
                  {suggestedDataKeys.length > 0 && (
                    <div className="wf-task-actions-grid" style={{ marginTop: 8 }}>
                      {suggestedDataKeys.map((key) => {
                        const selected = visibleDataKeys.includes(key);
                        return (
                          <button
                            key={key}
                            className={`wf-task-action-btn ${selected ? "active" : ""}`}
                            style={selected ? { borderColor: "#22c55e", background: "rgba(34,197,94,0.16)", color: "#15803d" } : {}}
                            onClick={() => {
                              const next = selected
                                ? visibleDataKeys.filter((k) => k !== key)
                                : [...visibleDataKeys, key];
                              onChange({ ...step, visibleDataKeys: next });
                            }}
                          >
                            {key}
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              )}

              <div className="wf-task-actions-grid" style={{ marginTop: 10 }}>
                <button
                  className={`wf-task-action-btn ${step.includeFullFormResponse ? "active" : ""}`}
                  style={step.includeFullFormResponse ? { borderColor: "#8b5cf6", background: "rgba(139,92,246,0.16)", color: "#6d28d9" } : {}}
                  onClick={() => onChange({ ...step, includeFullFormResponse: !step.includeFullFormResponse })}
                >
                  Include Full Form Response
                </button>
                <button
                  className={`wf-task-action-btn ${step.includeFormFiles ? "active" : ""}`}
                  style={step.includeFormFiles ? { borderColor: "#0ea5e9", background: "rgba(14,165,233,0.16)", color: "#0369a1" } : {}}
                  onClick={() => onChange({ ...step, includeFormFiles: !step.includeFormFiles })}
                >
                  Include Form File Links
                </button>
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
                {step.connector.type === "email" && (
                  <span className="wf-field-hint">
                    This action sends via the Gmail integration. Use Send From Account to target a specific connected Gmail account (email or account id), or leave it blank to use primary.
                  </span>
                )}
                {step.connector.type === "form_submit" && (
                  <div className="wf-field">
                    <label className="wf-field-label">Use Existing Form</label>
                    <select
                      className="wf-select"
                      value={step.connector?.params.form_id || ""}
                      onChange={(e) => {
                        const selectedFormID = e.target.value;
                        const selected = availableForms.find((f) => f.form_id === selectedFormID);
                        onChange({
                          ...step,
                          connector: {
                            ...step.connector!,
                            params: {
                              ...step.connector!.params,
                              form_id: selectedFormID,
                              form_url: selectedFormID ? (selected?.responder_uri || "") : "",
                            },
                          },
                        });
                      }}
                    >
                      <option value="">Select an existing form...</option>
                      {availableForms.map((form) => (
                        <option key={form.form_id} value={form.form_id}>
                          {form.title}
                        </option>
                      ))}
                    </select>
                    <button
                      className="action-btn action-btn-outline"
                      type="button"
                      style={{ marginTop: 8 }}
                      onClick={() => onRefreshForms?.()}
                      disabled={formsLoading}
                    >
                      {formsLoading ? "Refreshing..." : "Refresh list"}
                    </button>
                  </div>
                )}
                {CONNECTOR_CONFIG[step.connector.type].paramFields.map((field) => (
                  <div key={field.key} className="wf-field">
                    <label className="wf-field-label">
                      {field.label}
                      {field.required && <span className="wf-required-star">*</span>}
                    </label>
                    {step.connector?.type === "email" && field.key === "from_account_id" ? (
                      <>
                        {(() => {
                          const currentValue = step.connector?.params[field.key] || "";
                          const hasCurrent = currentValue
                            ? availableGmailAccounts.some((account) => (account.account_id || account.account_email) === currentValue)
                            : true;
                          return (
                        <select
                          className="wf-select"
                          value={currentValue}
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
                          <option value="">Primary connected account</option>
                          {!hasCurrent && currentValue && (
                            <option key={`disconnected-${currentValue}`} value={currentValue}>
                              Disconnected account: {currentValue}
                            </option>
                          )}
                          {availableGmailAccounts.map((account) => {
                            const fallback = account.account_email || account.account_id;
                            const label = account.account_name
                              ? `${account.account_name} (${fallback})`
                              : fallback;
                            return (
                              <option key={account.account_id || account.account_email} value={account.account_id || account.account_email}>
                                {account.is_primary ? `Primary - ${label}` : label}
                              </option>
                            );
                          })}
                        </select>
                          );
                        })()}
                        <button
                          className="action-btn action-btn-outline"
                          type="button"
                          style={{ marginTop: 8 }}
                          onClick={() => onRefreshGmailAccounts?.()}
                          disabled={gmailAccountsLoading}
                        >
                          {gmailAccountsLoading ? "Refreshing accounts..." : "Refresh sender accounts"}
                        </button>
                        {gmailAccountsError && (
                          <span className="wf-field-hint" style={{ color: "#b45309", display: "block", marginTop: 6 }}>
                            {gmailAccountsError}
                          </span>
                        )}
                      </>
                    ) : field.options ? (
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
            {baseConditionConfig.rules.length > 1 && (
              <div className="wf-field" style={{ marginBottom: 8 }}>
                <label className="wf-field-label">Rule Logic</label>
                <input
                  className="wf-input"
                  placeholder="1 AND 2 AND (3 OR 4)"
                  value={activeConditionLogic}
                  readOnly
                  style={{ borderColor: conditionLogicBorderColor }}
                />
                <div className="wf-task-actions-grid" style={{ marginTop: 6, gap: 6 }}>
                  {logicRuleTokens.map((token) => (
                    <button
                      key={`logic-token-rule-${token}`}
                      type="button"
                      className="wf-task-action-btn"
                      onClick={() => insertConditionLogicToken(token)}
                    >
                      {token}
                    </button>
                  ))}
                  {[
                    { label: "AND", value: "AND" },
                    { label: "OR", value: "OR" },
                    { label: "(", value: "(" },
                    { label: ")", value: ")" },
                  ].map((token) => (
                    <button
                      key={`logic-token-${token.label}`}
                      type="button"
                      className="wf-task-action-btn"
                      onClick={() => insertConditionLogicToken(token.value)}
                    >
                      {token.label}
                    </button>
                  ))}
                  <button
                    type="button"
                    className="wf-task-action-btn"
                    onClick={removeConditionLogicToken}
                  >
                    Backspace
                  </button>
                  <button
                    type="button"
                    className="wf-task-action-btn"
                    onClick={() => updateConditionLogic("")}
                  >
                    Clear
                  </button>
                </div>
                <span className="wf-field-hint">Use rule numbers with AND/OR and parentheses.</span>
                {conditionLogicTouched && conditionLogicError && (
                  <span className="wf-field-hint" style={{ color: "#ef4444", marginTop: 4 }}>
                    Invalid logic: {conditionLogicError}
                  </span>
                )}
              </div>
            )}

            <div style={{ display: "grid", gap: 8 }}>
              {baseConditionConfig.rules.map((rule, index) => {
                const showRuleLabel = baseConditionConfig.rules.length > 1;
                const inferredDataType = inferConditionDataTypeForField(conditionFieldLookup.get(rule.field));
                const selectedDataType = rule.field
                  ? inferredDataType
                  : normalizeConditionDataType(rule.dataType || inferredDataType);
                const operators = CONDITION_OPERATORS_BY_TYPE[selectedDataType] || CONDITION_OPERATORS_BY_TYPE.text;
                const selectedOperator = operators.some((item) => item.value === rule.operator)
                  ? rule.operator
                  : operators[0].value;
                const needsValue = operatorRequiresValue(selectedDataType, selectedOperator);
                const selectedFieldLabel = rule.field
                  ? (availableConditionFields.find((field) => field.key === rule.field)?.label || rule.field)
                  : "";
                return (
                  <div key={`condition-rule-${index}`} style={{ display: "grid", gap: 4 }}>
                    {showRuleLabel && (
                      <span className="wf-field-hint" style={{ margin: 0, fontWeight: 700 }}>{`Cond ${index + 1}`}</span>
                    )}
                    <div className="wf-field-row" style={{ gap: 6, alignItems: "center", flexWrap: "nowrap" }}>
                      <select
                        className="wf-select"
                        style={{ width: 220, maxWidth: 220 }}
                        title={selectedFieldLabel}
                        value={rule.field}
                        onChange={(e) => {
                          const field = e.target.value;
                          const nextType = inferConditionDataTypeForField(conditionFieldLookup.get(field));
                          const nextOps = CONDITION_OPERATORS_BY_TYPE[nextType] || CONDITION_OPERATORS_BY_TYPE.text;
                          updateConditionRule(index, {
                            field,
                            dataType: nextType,
                            operator: nextOps[0].value,
                            value: "",
                          });
                        }}
                      >
                        <option value="">Select field</option>
                        {availableConditionFields.map((field) => (
                          <option key={field.key} value={field.key} title={field.label || field.key}>
                            {truncateConditionFieldLabel(field.label || field.key)}
                          </option>
                        ))}
                      </select>

                      <select
                        className="wf-select"
                        style={{ width: 128 }}
                        value={selectedOperator}
                        onChange={(e) => updateConditionRule(index, { operator: e.target.value as ConditionOperator })}
                      >
                        <option value="">Select operator</option>
                        {operators.map((operator) => (
                          <option key={operator.value} value={operator.value}>{operator.label}</option>
                        ))}
                      </select>

                      {needsValue ? (
                        selectedDataType === "boolean" ? (
                          <select
                            className="wf-select"
                            style={{ width: 112 }}
                            value={String(rule.value || "")}
                            onChange={(e) => updateConditionRule(index, { value: e.target.value })}
                          >
                            <option value="">Select</option>
                            <option value="true">True</option>
                            <option value="false">False</option>
                          </select>
                        ) : selectedDataType === "number" ? (
                          <input
                            type="number"
                            className="wf-input"
                            style={{ width: 120 }}
                            value={rule.value || ""}
                            onChange={(e) => updateConditionRule(index, { value: e.target.value })}
                          />
                        ) : selectedDataType === "date" ? (
                          <input
                            type="date"
                            className="wf-input"
                            style={{ width: 140 }}
                            value={rule.value || ""}
                            onChange={(e) => updateConditionRule(index, { value: e.target.value })}
                          />
                        ) : selectedDataType === "time" ? (
                          <input
                            type="time"
                            className="wf-input"
                            style={{ width: 112 }}
                            value={rule.value || ""}
                            onChange={(e) => updateConditionRule(index, { value: e.target.value })}
                          />
                        ) : selectedDataType === "datetime" ? (
                          <input
                            type="datetime-local"
                            className="wf-input"
                            style={{ width: 168 }}
                            value={rule.value || ""}
                            onChange={(e) => updateConditionRule(index, { value: e.target.value })}
                          />
                        ) : (
                          <input
                            className="wf-input"
                            style={{ width: 132 }}
                            value={rule.value || ""}
                            onChange={(e) => updateConditionRule(index, { value: e.target.value })}
                          />
                        )
                      ) : (
                        <input className="wf-input" style={{ width: 132 }} value="" readOnly placeholder="(blank)" />
                      )}

                      <button
                        type="button"
                        className="action-btn action-btn-outline"
                        onClick={() => removeConditionRule(index)}
                        disabled={baseConditionConfig.rules.length <= 1}
                        style={{ padding: "6px 10px", minWidth: 40 }}
                      >
                        ×
                      </button>
                    </div>
                  </div>
                );
              })}
            </div>

            <div style={{ marginTop: 8 }}>
              <button type="button" className="action-btn action-btn-outline" onClick={addConditionRule}>
                + Add condition
              </button>
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
