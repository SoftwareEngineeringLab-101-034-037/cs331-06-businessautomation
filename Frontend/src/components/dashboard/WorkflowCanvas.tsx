"use client";

import { useCallback, useRef, useState } from "react";
import type { WorkflowStep, WorkflowTrigger } from "@/types/workflow";
import { TRIGGER_CONFIG, STEP_ACTION_CONFIG } from "@/types/workflow";

interface WorkflowCanvasProps {
  trigger: WorkflowTrigger;
  steps: WorkflowStep[];
  selectedStepId: string | null;
  onSelectTrigger: () => void;
  onSelectStep: (id: string) => void;
  onAddStep: (afterIndex: number) => void;
  onReorder: (steps: WorkflowStep[]) => void;
  onDeleteStep: (id: string) => void;
}

export default function WorkflowCanvas({
  trigger,
  steps,
  selectedStepId,
  onSelectTrigger,
  onSelectStep,
  onAddStep,
  onReorder,
  onDeleteStep,
}: WorkflowCanvasProps) {
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [dropTarget, setDropTarget] = useState<number | null>(null);
  const dragRef = useRef<number | null>(null);

  const triggerCfg = TRIGGER_CONFIG[trigger.type];

  /* ── Drag handlers ── */
  const handleDragStart = useCallback((idx: number) => {
    dragRef.current = idx;
    setDragIndex(idx);
  }, []);

  const handleDragOver = useCallback(
    (e: React.DragEvent, idx: number) => {
      e.preventDefault();
      if (dragRef.current !== null && dragRef.current !== idx) {
        setDropTarget(idx);
      }
    },
    [],
  );

  const handleDrop = useCallback(
    (idx: number) => {
      const from = dragRef.current;
      if (from === null || from === idx) return;
      const updated = [...steps];
      const [moved] = updated.splice(from, 1);
      updated.splice(idx, 0, moved);
      onReorder(updated);
      setDragIndex(null);
      setDropTarget(null);
      dragRef.current = null;
    },
    [steps, onReorder],
  );

  const handleDragEnd = useCallback(() => {
    setDragIndex(null);
    setDropTarget(null);
    dragRef.current = null;
  }, []);

  return (
    <div className="wf-canvas">
      {/* ── Trigger node ── */}
      <button
        className={`wf-node wf-node-trigger ${selectedStepId === "__trigger__" ? "wf-node-selected" : ""}`}
        onClick={onSelectTrigger}
      >
        <div className="wf-node-icon wf-node-icon-trigger">
          <TriggerIcon type={trigger.type} />
        </div>
        <div className="wf-node-content">
          <span className="wf-node-label">Trigger</span>
          <span className="wf-node-title">{triggerCfg.label}</span>
        </div>
        <ChevronRight />
      </button>

      {/* Connector */}
      <div className="wf-connector">
        <div className="wf-connector-line" />
        <button
          className="wf-connector-add"
          onClick={() => onAddStep(0)}
          title="Add step here"
        >
          <PlusIcon />
        </button>
        <div className="wf-connector-line" />
      </div>

      {/* ── Step nodes ── */}
      {steps.map((step, idx) => {
        const actionCfg = STEP_ACTION_CONFIG[step.actionType];
        const isDragging = dragIndex === idx;
        const isDropTarget = dropTarget === idx;

        return (
          <div key={step.id}>
            <div
              className={`wf-node wf-node-step
                ${selectedStepId === step.id ? "wf-node-selected" : ""}
                ${isDragging ? "wf-node-dragging" : ""}
                ${isDropTarget ? "wf-node-drop-target" : ""}`}
              draggable
              onDragStart={() => handleDragStart(idx)}
              onDragOver={(e) => handleDragOver(e, idx)}
              onDrop={() => handleDrop(idx)}
              onDragEnd={handleDragEnd}
              onClick={() => onSelectStep(step.id)}
              role="button"
              tabIndex={0}
              onKeyDown={(e) => {
                if (e.key === "Enter") onSelectStep(step.id);
              }}
            >
              {/* Drag handle */}
              <div className="wf-node-drag" title="Drag to reorder">
                <GripIcon />
              </div>

              {/* Step number */}
              <div
                className="wf-node-number"
                style={{ background: actionCfg.color }}
              >
                {idx + 1}
              </div>

              <div className="wf-node-content">
                <span className="wf-node-label" style={{ color: actionCfg.color }}>
                  {actionCfg.label}
                </span>
                <span className="wf-node-title">
                  {step.title || "Untitled step"}
                </span>
                {step.assignedRole && (
                  <span className="wf-node-role">
                    <PersonIcon /> {step.assignedRole}
                  </span>
                )}
              </div>

              {/* SLA badge */}
              <span className="wf-node-sla">
                {step.slaDays}d SLA
              </span>

              {/* Delete */}
              <button
                className="wf-node-delete"
                onClick={(e) => {
                  e.stopPropagation();
                  onDeleteStep(step.id);
                }}
                title="Remove step"
              >
                <TrashIcon />
              </button>
            </div>

            {/* Connector after step */}
            <div className="wf-connector">
              <div className="wf-connector-line" />
              <button
                className="wf-connector-add"
                onClick={() => onAddStep(idx + 1)}
                title="Add step here"
              >
                <PlusIcon />
              </button>
              <div className="wf-connector-line" />
            </div>
          </div>
        );
      })}

      {/* End node */}
      <div className="wf-node wf-node-end">
        <div className="wf-node-icon wf-node-icon-end">
          <FlagIcon />
        </div>
        <div className="wf-node-content">
          <span className="wf-node-label">End</span>
          <span className="wf-node-title">Workflow Complete</span>
        </div>
      </div>
    </div>
  );
}

/* ── Inline SVG icons ── */
function TriggerIcon({ type }: { type: string }) {
  switch (type) {
    case "form_submission":
      return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="22" height="22"><path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m0 12.75h7.5m-7.5 3H12M10.5 2.25H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" /></svg>;
    case "email_received":
      return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="22" height="22"><path strokeLinecap="round" strokeLinejoin="round" d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15a2.25 2.25 0 0 1-2.25-2.25V6.75m19.5 0A2.25 2.25 0 0 0 19.5 4.5h-15a2.25 2.25 0 0 0-2.25 2.25m19.5 0v.243a2.25 2.25 0 0 1-1.07 1.916l-7.5 4.615a2.25 2.25 0 0 1-2.36 0L3.32 8.91a2.25 2.25 0 0 1-1.07-1.916V6.75" /></svg>;
    case "scheduled":
      return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="22" height="22"><path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" /></svg>;
    case "webhook":
      return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="22" height="22"><path strokeLinecap="round" strokeLinejoin="round" d="M13.19 8.688a4.5 4.5 0 0 1 1.242 7.244l-4.5 4.5a4.5 4.5 0 0 1-6.364-6.364l1.757-1.757m13.35-.622 1.757-1.757a4.5 4.5 0 0 0-6.364-6.364l-4.5 4.5a4.5 4.5 0 0 0 1.242 7.244" /></svg>;
    case "condition":
      return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="22" height="22"><path strokeLinecap="round" strokeLinejoin="round" d="M9.75 3.104v5.714a2.25 2.25 0 0 1-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 0 1 4.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 14.5M14.25 3.104c.251.023.501.05.75.082M19.8 14.5l-2.846 2.046a.75.75 0 0 1-.882 0L12 13.685l-4.072 2.86a.75.75 0 0 1-.882 0L4.2 14.5" /></svg>;
    default: // manual
      return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="22" height="22"><path strokeLinecap="round" strokeLinejoin="round" d="M10.05 4.575a1.575 1.575 0 1 0-3.15 0v3m3.15-3v-1.5a1.575 1.575 0 0 1 3.15 0v1.5m-3.15 0 .075 5.925m3.075-5.925v2.468m0 0a1.575 1.575 0 0 1 3.15 0V15a6 6 0 0 1-6 6h-.5a6 6 0 0 1-6-6v-2.422" /></svg>;
  }
}

function PlusIcon() {
  return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="16" height="16"><path strokeLinecap="round" strokeLinejoin="round" d="M12 4.5v15m7.5-7.5h-15" /></svg>;
}

function GripIcon() {
  return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16"><path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" /></svg>;
}

function TrashIcon() {
  return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16"><path strokeLinecap="round" strokeLinejoin="round" d="m14.74 9-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 0 1-2.244 2.077H8.084a2.25 2.25 0 0 1-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 0 0-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 0 1 3.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 0 0-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 0 0-7.5 0" /></svg>;
}

function PersonIcon() {
  return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="13" height="13"><path strokeLinecap="round" strokeLinejoin="round" d="M15.75 6a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0ZM4.501 20.118a7.5 7.5 0 0 1 14.998 0A17.933 17.933 0 0 1 12 21.75c-2.676 0-5.216-.584-7.499-1.632Z" /></svg>;
}

function FlagIcon() {
  return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="22" height="22"><path strokeLinecap="round" strokeLinejoin="round" d="M3 3v1.5M3 21v-6m0 0 2.77-.693a9 9 0 0 1 6.208.682l.108.054a9 9 0 0 0 6.086.71l3.114-.732a48.524 48.524 0 0 1-.005-10.499l-3.11.732a9 9 0 0 1-6.085-.711l-.108-.054a9 9 0 0 0-6.208-.682L3 4.5M3 15V4.5" /></svg>;
}

function ChevronRight() {
  return <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={2} stroke="currentColor" width="16" height="16" className="wf-node-chevron"><path strokeLinecap="round" strokeLinejoin="round" d="m8.25 4.5 7.5 7.5-7.5 7.5" /></svg>;
}
