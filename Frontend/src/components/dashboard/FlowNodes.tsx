"use client";

import { memo, Fragment } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import type { WorkflowStep } from "@/types/workflow";
import { NODE_TYPE_CONFIG, STEP_ACTION_CONFIG, TASK_ACTION_OPTIONS } from "@/types/workflow";

export type FlowNodeData = WorkflowStep & {
  selected?: boolean;
  onDelete?: () => void;
  /** Source handle IDs that already have an outgoing edge — used to hide handle labels */
  connectedHandles?: string[];
};

/* ─── Reusable node delete button ─── */
function NodeDeleteBtn({ onDelete }: { onDelete: () => void }) {
  return (
    <button
      className="rf-node-delete-btn"
      onClick={(e) => { e.stopPropagation(); onDelete(); }}
      title="Delete node (or press Backspace)"
    >
      &times;
    </button>
  );
}

/* ───────────────────────────────────────────────────────
   Start Node  (circle, single output)
   ─────────────────────────────────────────────────────── */
export const StartNode = memo(function StartNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  return (
    <div className="rf-node rf-node-start">
      <div className="rf-node-icon">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="#fff" width="20" height="20">
          <path strokeLinecap="round" strokeLinejoin="round" d="M5.25 5.653c0-.856.917-1.398 1.667-.986l11.54 6.347a1.125 1.125 0 0 1 0 1.972l-11.54 6.347a1.125 1.125 0 0 1-1.667-.986V5.653Z" />
        </svg>
      </div>
      <span className="rf-node-label">{d.title || "Start"}</span>
      <Handle type="source" position={Position.Bottom} id="source" className="rf-handle rf-handle-source" />
    </div>
  );
});

/* ───────────────────────────────────────────────────────
   End Node  (circle, single input)
   ─────────────────────────────────────────────────────── */
export const EndNode = memo(function EndNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  return (
    <div className="rf-node rf-node-end">
      <Handle type="target" position={Position.Top} id="target" className="rf-handle rf-handle-target" />
      <div className="rf-node-icon rf-node-icon-end">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="#fff" width="20" height="20">
          <path strokeLinecap="round" strokeLinejoin="round" d="M3 3v1.5M3 21v-6m0 0 2.77-.693a9 9 0 0 1 6.208.682l.108.054a9 9 0 0 0 6.086.71l3.114-.732a48.524 48.524 0 0 1-.005-10.499l-3.11.732a9 9 0 0 1-6.085-.711l-.108-.054a9 9 0 0 0-6.208-.682L3 4.5M3 15V4.5" />
        </svg>
      </div>
      <span className="rf-node-label">{d.title || "End"}</span>
    </div>
  );
});

/* ───────────────────────────────────────────────────────
   Task Node  (rounded rect, 1 input + N action outputs)
   When the task has ≥ 2 enabled actions, each action gets its
   own output handle so the user can branch the workflow.
   With 0 or 1 actions a single "source" handle is shown.
   ─────────────────────────────────────────────────────── */
export const TaskNode = memo(function TaskNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  const actionCfg = STEP_ACTION_CONFIG[d.actionType] ?? STEP_ACTION_CONFIG.custom_task;

  // Determine if we show per-action output handles
  const actions = d.taskActions ?? [];
  const useBranching = actions.length >= 2;
  const connected = new Set<string>(d.connectedHandles ?? []);

  // Build action handle metadata
  const actionHandles = useBranching
    ? actions.map((a) => {
        const opt = TASK_ACTION_OPTIONS.find((o) => o.value === a);
        return { id: a, label: opt?.label ?? a, color: opt?.color ?? "#6b7280" };
      })
    : [];

  return (
    <div className="rf-node rf-node-task" style={{ borderLeftColor: actionCfg.color }}>
      {d.onDelete && <NodeDeleteBtn onDelete={d.onDelete} />}
      <Handle type="target" position={Position.Top} id="target" className="rf-handle rf-handle-target" />
      <div className="rf-node-header">
        <span className="rf-node-badge" style={{ background: actionCfg.color }}>{actionCfg.label}</span>
        {d.assignedRole && <span className="rf-node-role">{d.assignedRole}</span>}
      </div>
      <span className="rf-node-title">{d.title || "Untitled Task"}</span>
      {d.description && <span className="rf-node-desc">{d.description}</span>}
      <div className="rf-node-footer">
        <span className="rf-node-sla">{d.slaDays}d SLA</span>
      </div>

      {/* Single fallback handle when no branching */}
      {!useBranching && (
        <Handle type="source" position={Position.Bottom} id="source" className="rf-handle rf-handle-source" />
      )}

      {/* Per-action output handles when branching */}
      {useBranching && actionHandles.map((h, idx) => {
        const pct = actionHandles.length === 1
          ? 50
          : 15 + (idx / (actionHandles.length - 1)) * 70;
        const isConnected = connected.has(h.id);
        return (
          <Fragment key={h.id}>
            <Handle
              type="source"
              position={Position.Bottom}
              id={h.id}
              className={`rf-handle rf-handle-source rf-handle-action rf-handle-action-${h.id}`}
              style={{ left: `${pct}%` }}
            />
            {!isConnected && (
              <span
                className="rf-task-action-handle-label"
                style={{ left: `${pct}%`, color: h.color, fontWeight: 700 }}
              >
                {h.label}
              </span>
            )}
          </Fragment>
        );
      })}
    </div>
  );
});

/* ───────────────────────────────────────────────────────
   Action Node  (rounded rect, 1 input + 1 output)
   ─────────────────────────────────────────────────────── */
export const ActionNode = memo(function ActionNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  const actionCfg = STEP_ACTION_CONFIG[d.actionType] ?? STEP_ACTION_CONFIG.custom_task;
  const ntCfg = NODE_TYPE_CONFIG.action;
  return (
    <div className="rf-node rf-node-action" style={{ borderLeftColor: ntCfg.color }}>
      {d.onDelete && <NodeDeleteBtn onDelete={d.onDelete} />}
      <Handle type="target" position={Position.Top} id="target" className="rf-handle rf-handle-target" />
      <div className="rf-node-header">
        <span className="rf-node-badge" style={{ background: ntCfg.color }}>{actionCfg.label}</span>
      </div>
      <span className="rf-node-title">{d.title || "Untitled Action"}</span>
      {d.description && <span className="rf-node-desc">{d.description}</span>}
      <Handle type="source" position={Position.Bottom} id="source" className="rf-handle rf-handle-source" />
    </div>
  );
});

/* ───────────────────────────────────────────────────────
   Condition Node  (diamond, 1 input, 2 outputs: yes / no)
   ─────────────────────────────────────────────────────── */
export const ConditionNode = memo(function ConditionNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  const ntCfg = NODE_TYPE_CONFIG.condition;
  return (
    <div className="rf-node rf-node-condition">
      {d.onDelete && <NodeDeleteBtn onDelete={d.onDelete} />}
      <Handle type="target" position={Position.Top} id="target" className="rf-handle rf-handle-target" />

      {/* Rotated square — gives proper bordered diamond */}
      <div className="rf-condition-diamond" style={{ borderColor: ntCfg.color }}>
        {/* Counter-rotate content so text stays upright */}
        <div className="rf-condition-content">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke={ntCfg.color} width="18" height="18">
            <path strokeLinecap="round" strokeLinejoin="round" d="M9.879 7.519c1.171-1.025 3.071-1.025 4.242 0 1.172 1.025 1.172 2.687 0 3.712-.203.179-.43.326-.67.442-.745.361-1.45.999-1.45 1.827v.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 5.25h.008v.008H12v-.008Z" />
          </svg>
          <span className="rf-node-title" style={{ textAlign: "center", fontSize: "0.75rem" }}>{d.title || "Condition"}</span>
          {d.condition && <span className="rf-condition-expr">{d.condition}</span>}
        </div>
      </div>

      {/* Yes — left tip */}
      <Handle
        type="source"
        position={Position.Left}
        id="yes"
        className="rf-handle rf-handle-source rf-handle-yes"
      />
      {!(d.connectedHandles ?? []).includes("yes") && (
        <div className="rf-handle-label rf-handle-label-left" style={{ color: "#166534", fontWeight: 700 }}>Yes</div>
      )}

      {/* No — right tip */}
      <Handle
        type="source"
        position={Position.Right}
        id="no"
        className="rf-handle rf-handle-source rf-handle-no"
      />
      {!(d.connectedHandles ?? []).includes("no") && (
        <div className="rf-handle-label rf-handle-label-right" style={{ color: "#991b1b", fontWeight: 700 }}>No</div>
      )}
    </div>
  );
});

/* ───────────────────────────────────────────────────────
   Parallel Node  (bar, 1 input, N dynamic outputs)
   ─────────────────────────────────────────────────────── */
export const ParallelNode = memo(function ParallelNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  const ntCfg = NODE_TYPE_CONFIG.parallel;
  const branchCount = d.branches ?? 2;

  // Generate evenly-spaced output handles
  const handles: { id: string; leftPercent: number }[] = [];
  for (let i = 0; i < branchCount; i++) {
    const pct = branchCount === 1 ? 50 : (15 + (i / (branchCount - 1)) * 70);
    handles.push({ id: `branch-${i}`, leftPercent: pct });
  }

  return (
    <div className="rf-node rf-node-parallel" style={{ borderColor: ntCfg.color }}>
      {d.onDelete && <NodeDeleteBtn onDelete={d.onDelete} />}
      <Handle type="target" position={Position.Top} id="target" className="rf-handle rf-handle-target" />
      <div className="rf-parallel-bar" style={{ background: ntCfg.color }}>
        <span className="rf-parallel-label">{d.title || "Parallel"}</span>
        <span className="rf-parallel-count">{branchCount} branches</span>
      </div>
      {handles.map((h) => (
        <Handle
          key={h.id}
          type="source"
          position={Position.Bottom}
          id={h.id}
          className="rf-handle rf-handle-source"
          style={{ left: `${h.leftPercent}%` }}
        />
      ))}
    </div>
  );
});

/* ───────────────────────────────────────────────────────
   Merge Node  (bar, N dynamic inputs, 1 output)
   Inverse of Parallel — waits for all incoming branches.
   ─────────────────────────────────────────────────────── */
export const MergeNode = memo(function MergeNode({ data }: NodeProps) {
  const d = data as unknown as FlowNodeData;
  const ntCfg = NODE_TYPE_CONFIG.merge;
  const inputCount = d.mergeInputs ?? 2;

  // Generate evenly-spaced input handles across the top
  const handles: { id: string; leftPercent: number }[] = [];
  for (let i = 0; i < inputCount; i++) {
    const pct = inputCount === 1 ? 50 : (15 + (i / (inputCount - 1)) * 70);
    handles.push({ id: `input-${i}`, leftPercent: pct });
  }

  // Determine which inputs are required.
  // undefined means all required by default; only explicitly listed ones are required once set.
  const allHandleIds = handles.map((h) => h.id);
  const requiredSet = new Set(d.requiredInputs === undefined ? allHandleIds : d.requiredInputs);

  return (
    <div className="rf-node rf-node-merge" style={{ borderColor: ntCfg.color }}>
      {d.onDelete && <NodeDeleteBtn onDelete={d.onDelete} />}

      {/* Per-handle labels floating above the top edge */}
      {handles.map((h) => {
        const req = requiredSet.has(h.id);
        return (
          <span
            key={`lbl-${h.id}`}
            className={`rf-merge-input-label ${req ? "rf-merge-input-label-req" : "rf-merge-input-label-opt"}`}
            style={{ left: `${h.leftPercent}%` }}
          >
            {req ? "R" : "O"}
          </span>
        );
      })}

      {/* Multiple target handles at the top */}
      {handles.map((h) => (
        <Handle
          key={h.id}
          type="target"
          position={Position.Top}
          id={h.id}
          className={`rf-handle rf-handle-target ${requiredSet.has(h.id) ? "rf-handle-required" : "rf-handle-optional"}`}
          style={{ left: `${h.leftPercent}%` }}
        />
      ))}

      <div className="rf-merge-bar" style={{ background: ntCfg.color }}>
        <span className="rf-merge-label">{d.title || "Merge"}</span>
        <span className="rf-merge-count">{inputCount} inputs</span>
      </div>

      {/* Single source handle at the bottom */}
      <Handle type="source" position={Position.Bottom} id="source" className="rf-handle rf-handle-source" />
    </div>
  );
});

/* ─── Node type map for React Flow registration ─── */
export const nodeTypes = {
  start: StartNode,
  end: EndNode,
  task: TaskNode,
  action: ActionNode,
  condition: ConditionNode,
  parallel: ParallelNode,
  merge: MergeNode,
};
