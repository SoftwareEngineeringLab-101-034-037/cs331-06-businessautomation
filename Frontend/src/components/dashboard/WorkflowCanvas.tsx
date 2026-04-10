"use client";

import { useState, useCallback, useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  BaseEdge,
  EdgeLabelRenderer,
  getSmoothStepPath,
  type Node,
  type Edge,
  type EdgeProps,
  type OnNodesChange,
  type OnEdgesChange,
  type OnConnect,
  type Connection,
  BackgroundVariant,
  MarkerType,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";

import type { WorkflowStep, WorkflowEdge, WorkflowTrigger, NodeType } from "@/types/workflow";
import { NODE_TYPE_CONFIG } from "@/types/workflow";
import { nodeTypes, type FlowNodeData } from "./FlowNodes";

/* ─── Deletable Edge ───
   Receives `selected` and `onDelete` via edge.data
   to avoid relying on RF's internal selection state
   being wiped on every controlled re-render. */
// Map label text to a colour pill (opaque backgrounds)
const LABEL_COLOR_MAP: Record<string, { text: string; bg: string; border: string }> = {
  Yes:      { text: "#166534", bg: "#dcfce7", border: "#86efac" },
  No:       { text: "#991b1b", bg: "#fee2e2", border: "#fca5a5" },
  Approve:  { text: "#166534", bg: "#dcfce7", border: "#86efac" },
  Reject:   { text: "#991b1b", bg: "#fee2e2", border: "#fca5a5" },
  Clarify:  { text: "#92400e", bg: "#fef3c7", border: "#fde68a" },
  Complete: { text: "#1e40af", bg: "#dbeafe", border: "#93c5fd" },
};

function DeletableEdge({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  data,
  label,
  markerEnd,
  style,
}: EdgeProps) {
  const d = data as { selected?: boolean; onDelete?: () => void } | undefined;
  const isSelected = d?.selected ?? false;

  const [edgePath, labelX, labelY] = getSmoothStepPath({
    sourceX, sourceY, sourcePosition,
    targetX, targetY, targetPosition,
  });

  const midX = (sourceX + targetX) / 2;
  const midY = (sourceY + targetY) / 2;

  return (
    <>
      <BaseEdge
        path={edgePath}
        markerEnd={markerEnd}
        style={{
          ...style,
          stroke: isSelected ? "var(--accent, #6366f1)" : "var(--border)",
          strokeWidth: isSelected ? 2.5 : 2,
        }}
      />
      <EdgeLabelRenderer>
        {/* Edge label (Yes / No / action name) */}
        {label && (() => {
          const lbl = label as string;
          const palette = LABEL_COLOR_MAP[lbl];
          return (
            <div
              style={{
                position: "absolute",
                transform: `translate(-50%,-50%) translate(${labelX}px,${labelY}px)`,
                pointerEvents: "none",
                color: palette?.text ?? "var(--text-secondary)",
                background: palette?.bg ?? "#f3f4f6",
                border: `1px solid ${palette?.border ?? "var(--border)"}`,
                borderRadius: 99,
                padding: "1px 8px",
                fontSize: "0.68rem",
                fontWeight: 700,
                letterSpacing: "0.03em",
                lineHeight: 1.8,
                whiteSpace: "nowrap",
              }}
            >
              {lbl}
            </div>
          );
        })()}
        {/* Delete button — visible when edge is selected */}
        {isSelected && d?.onDelete && (
          <button
            style={{
              position: "absolute",
              transform: `translate(-50%,-50%) translate(${midX}px,${midY}px)`,
              pointerEvents: "all",
            }}
            className="rf-edge-delete-btn"
            onClick={(e) => { e.stopPropagation(); d.onDelete!(); }}
            title="Delete edge (or press Backspace)"
          >
            &times;
          </button>
        )}
      </EdgeLabelRenderer>
    </>
  );
}

const edgeTypes = { deletable: DeletableEdge };

/* ─── Props ─── */
export interface WorkflowCanvasProps {
  steps: WorkflowStep[];
  edges: WorkflowEdge[];
  selectedStepId: string | null;
  onSelectStep: (id: string | null) => void;
  onNodesChange: OnNodesChange;
  onEdgesChange: OnEdgesChange;
  onConnect: (connection: Connection) => void;
  onDeleteStep: (id: string) => void;
  onDeleteEdge: (id: string) => void;
  trigger?: WorkflowTrigger;
  onConfigureTrigger?: () => void;
}

/* ─── Helpers ─── */
function stepsToFlowNodes(
  steps: WorkflowStep[],
  edges: WorkflowEdge[],
  selectedId: string | null,
  onDeleteStep: (id: string) => void,
  trigger?: WorkflowTrigger,
  onConfigureTrigger?: () => void,
): Node[] {
  // Build per-node set of source handles that already have an outgoing edge
  const connectedSrc = new Map<string, Set<string>>();
  for (const e of edges) {
    if (!connectedSrc.has(e.source)) connectedSrc.set(e.source, new Set());
    connectedSrc.get(e.source)!.add(e.sourceHandle ?? "source");
  }

  return steps.map((s) => {
    const deletable = s.type !== "start" && s.type !== "end";
    const nt = s.type || "task";
    return {
      id: s.id,
      type: nt,
      position: s.position ?? { x: 250, y: 0 },
      data: {
        ...s,
        selected: s.id === selectedId,
        onDelete: deletable ? () => onDeleteStep(s.id) : undefined,
        connectedHandles: Array.from(connectedSrc.get(s.id) ?? []),
        ...(s.type === "start" && { trigger, onConfigureTrigger }),
      } as unknown as Record<string, unknown>,
      selected: s.id === selectedId,
    };
  });
}

function edgesToFlowEdges(
  edges: WorkflowEdge[],
  selectedEdgeId: string | null,
  onDeleteEdge: (id: string) => void,
): Edge[] {
  return edges.map((e) => ({
    id: e.id,
    source: e.source,
    target: e.target,
    sourceHandle: e.sourceHandle,
    targetHandle: e.targetHandle,
    label: e.label,
    type: "deletable",
    animated: false,
    markerEnd: { type: MarkerType.ArrowClosed, width: 16, height: 16 },
    style: { strokeWidth: 2, stroke: "var(--border)" },
    /* Pass selection and delete callback via data so the edge
       component doesn’t depend on RF’s internal selection state */
    data: {
      selected: e.id === selectedEdgeId,
      onDelete: () => onDeleteEdge(e.id),
    },
  }));
}

/* ─── Canvas Component ─── */
export default function WorkflowCanvas({
  steps,
  edges,
  selectedStepId,
  onSelectStep,
  onNodesChange,
  onEdgesChange,
  onConnect,
  onDeleteStep,
  onDeleteEdge,
  trigger,
  onConfigureTrigger,
}: WorkflowCanvasProps) {
  /* Track which edge is "selected" locally — avoids the problem
     of the controlled-rerender resetting RF internal selection. */
  const [selectedEdgeId, setSelectedEdgeId] = useState<string | null>(null);

  const flowNodes = useMemo(
    () => stepsToFlowNodes(steps, edges, selectedStepId, onDeleteStep, trigger, onConfigureTrigger),
    [steps, edges, selectedStepId, onDeleteStep, trigger, onConfigureTrigger],
  );
  const flowEdges = useMemo(
    () => edgesToFlowEdges(edges, selectedEdgeId, onDeleteEdge),
    [edges, selectedEdgeId, onDeleteEdge],
  );

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      const nodeData = node.data as unknown as FlowNodeData | undefined;
      if (node.type === "start") {
        nodeData?.onConfigureTrigger?.();
        onSelectStep(null);
        setSelectedEdgeId(null);
        return;
      }
      onSelectStep(node.id);
      setSelectedEdgeId(null);
    },
    [onSelectStep],
  );

  const handleEdgeClick = useCallback((_: React.MouseEvent, edge: Edge) => {
    setSelectedEdgeId((cur) => (cur === edge.id ? null : edge.id));
  }, []);

  const handlePaneClick = useCallback(() => {
    onSelectStep(null);
    setSelectedEdgeId(null);
  }, [onSelectStep]);

  /* Keyboard delete for selected edge */
  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if ((e.key === "Backspace" || e.key === "Delete") && selectedEdgeId) {
        onDeleteEdge(selectedEdgeId);
        setSelectedEdgeId(null);
      }
    },
    [selectedEdgeId, onDeleteEdge],
  );

  const handleConnect = useCallback(
    (connection: Connection) => onConnect(connection),
    [onConnect],
  );

  const minimapColor = useCallback((node: Node) => {
    const nt = (node.type || "task") as NodeType;
    return NODE_TYPE_CONFIG[nt]?.color ?? "#888";
  }, []);

  return (
    <div
      style={{ position: "absolute", inset: 0 }}
      tabIndex={0}
      onKeyDown={handleKeyDown}
    >
      <ReactFlow
        nodes={flowNodes}
        edges={flowEdges}
        nodeTypes={nodeTypes}
        edgeTypes={edgeTypes}
        onNodesChange={onNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={handleConnect}
        onNodeClick={handleNodeClick}
        onEdgeClick={handleEdgeClick}
        onPaneClick={handlePaneClick}
        fitView
        snapToGrid
        snapGrid={[20, 20]}
        /* Keyboard delete: Backspace/Delete on selected node —
         onNodesChange already handles "remove" changes */
      deleteKeyCode={["Backspace", "Delete"]}
        connectionLineStyle={{ strokeWidth: 2, stroke: "var(--accent)" }}
        defaultEdgeOptions={{
          type: "deletable",
          markerEnd: { type: MarkerType.ArrowClosed, width: 16, height: 16 },
          style: { strokeWidth: 2, stroke: "var(--border)" },
        }}
      >
        <Background variant={BackgroundVariant.Dots} gap={20} size={1} color="var(--border)" />
        <Controls position="bottom-left" />
        <MiniMap
          nodeColor={minimapColor}
          nodeStrokeColor={minimapColor}
          nodeStrokeWidth={3}
          nodeBorderRadius={4}
          maskColor="rgba(0,0,0,0.08)"
          bgColor="#f8fafc"
          position="bottom-right"
          pannable
          zoomable
          style={{
            border: "1px solid var(--border)",
            borderRadius: 10,
            boxShadow: "0 2px 10px rgba(0,0,0,0.10)",
            overflow: "hidden",
            width: 180,
            height: 120,
          }}
        />
      </ReactFlow>
    </div>
  );
}
