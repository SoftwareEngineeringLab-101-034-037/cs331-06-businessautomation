"use client";

import { useState, useCallback, useEffect, useRef } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  applyEdgeChanges,
  type OnNodesChange,
  type OnEdgesChange,
  type Connection,
} from "@xyflow/react";
import { RoleGate } from "@/components/dashboard/RoleProvider";
import { useTheme } from "@/components/ThemeProvider";
import WorkflowCanvas from "@/components/dashboard/WorkflowCanvas";
import { TriggerEditor, StepEditor } from "@/components/dashboard/StepEditor";
import { useToast, ToastContainer } from "@/components/Toast";
import type {
  WorkflowDraft,
  WorkflowStep,
  WorkflowEdge,
  WorkflowTrigger,
  NodeType,
} from "@/types/workflow";
import { createBlankStep, generateStepId, NODE_TYPE_CONFIG } from "@/types/workflow";
import { MOCK_DEPARTMENTS } from "@/lib/mock-data";

/* ── Initial draft ── */
const INITIAL_STEPS: WorkflowStep[] = [
  {
    id: "node_start",
    type: "start",
    title: "Start",
    description: "",
    actionType: "custom_task",
    assignedRole: "",
    slaDays: 1,
    isRequired: true,
    position: { x: 250, y: 0 },
  },
  {
    id: "node_end",
    type: "end",
    title: "End",
    description: "",
    actionType: "custom_task",
    assignedRole: "",
    slaDays: 1,
    isRequired: true,
    position: { x: 250, y: 300 },
  },
];

const INITIAL_EDGES: WorkflowEdge[] = [];

const INITIAL_DRAFT: WorkflowDraft = {
  name: "",
  description: "",
  department: "",
  trigger: { type: "manual", config: {} },
  steps: INITIAL_STEPS,
  edges: INITIAL_EDGES,
  tags: [],
};

export default function WorkflowBuilderPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { theme, toggle: toggleTheme } = useTheme();

  /* ── State ── */
  const [draft, setDraft] = useState<WorkflowDraft>(INITIAL_DRAFT);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [tagInput, setTagInput] = useState("");
  const [showPublishModal, setShowPublishModal] = useState(false);
  const [publishErrors, setPublishErrors] = useState<string[]>([]);
  const [showDetailsDialog, setShowDetailsDialog] = useState(true);
  const [detailsSidebarOpen, setDetailsSidebarOpen] = useState(false);

  /* ── Editing existing workflow (via ?id=) ── */
  const [editingId, setEditingId] = useState<string | null>(null);
  const [commitMessage, setCommitMessage] = useState("");
  const loadedRef = useRef(false);

  /* ── Discard confirmation dialog ── */
  const [showDiscardModal, setShowDiscardModal] = useState(false);
  // Snapshot of the draft at load time — used to detect actual changes
  const originalDraftRef = useRef<string | null>(null);

  /* ── Toast notifications ── */
  const { toasts, showToast, dismissToast } = useToast();

  useEffect(() => {
    const wfId = searchParams.get("id");
    if (!wfId || loadedRef.current) return;
    loadedRef.current = true;

    (async () => {
      try {
        const res = await fetch(`http://localhost:8085/workflows/${wfId}`);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const wf = await res.json();

        // Restore canvas from raw_json if available
        let steps: WorkflowStep[] = INITIAL_STEPS;
        let edges: WorkflowEdge[] = INITIAL_EDGES;
        if (wf.raw_json) {
          try {
            const raw = JSON.parse(wf.raw_json);
            if (Array.isArray(raw.steps) && raw.steps.length > 0) steps = raw.steps;
            if (Array.isArray(raw.edges)) edges = raw.edges;
          } catch { /* ignore bad JSON */ }
        }

        const loaded: WorkflowDraft = {
          name: wf.name || "",
          description: wf.description || "",
          department: wf.department || "",
          trigger: {
            type: wf.trigger?.type || "manual",
            config: wf.trigger?.config || {},
          },
          steps,
          edges,
          tags: wf.tags || [],
        };

        setDraft(loaded);
        setEditingId(wfId);
        // Snapshot for change-detection
        originalDraftRef.current = JSON.stringify(loaded);
        // Skip the "New Workflow" details dialog — go straight to canvas
        setShowDetailsDialog(false);
      } catch (err) {
        console.error("Failed to load workflow for editing:", err);
        showToast("Could not load workflow. Starting fresh.", "error");
      }
    })();
  }, [searchParams]);

  /* Dialog form local state */
  const [dlgName, setDlgName] = useState("");
  const [dlgDesc, setDlgDesc] = useState("");
  const [dlgDept, setDlgDept] = useState("");
  const [dlgTags, setDlgTags] = useState<string[]>([]);
  const [dlgTagInput, setDlgTagInput] = useState("");

  /* ── Navigation ── */
  const handleBack = useCallback(() => {
    router.push("/dashboard/workstation");
  }, [router]);

  useEffect(() => {
    const handlePop = () => router.push("/dashboard/workstation");
    window.addEventListener("popstate", handlePop);
    return () => window.removeEventListener("popstate", handlePop);
  }, [router]);

  /* ── Dialog handlers ── */
  const addDlgTag = useCallback(() => {
    const t = dlgTagInput.trim();
    if (t && !dlgTags.includes(t)) setDlgTags((p) => [...p, t]);
    setDlgTagInput("");
  }, [dlgTagInput, dlgTags]);

  const removeDlgTag = useCallback((tag: string) => {
    setDlgTags((p) => p.filter((t) => t !== tag));
  }, []);

  const handleDialogContinue = useCallback(() => {
    setDraft((d) => ({ ...d, name: dlgName, description: dlgDesc, department: dlgDept, tags: dlgTags }));
    setShowDetailsDialog(false);
  }, [dlgName, dlgDesc, dlgDept, dlgTags]);

  /* ── Trigger ── */
  const handleTriggerChange = useCallback(
    (t: WorkflowTrigger) => setDraft((d) => ({ ...d, trigger: t })),
    [],
  );

  /* ── React Flow change handlers ── */
  const onNodesChange: OnNodesChange = useCallback(
    (changes) => {
      // Only process position drags and removals.
      // Ignore dimension/selection changes to avoid infinite re-render loops
      // (our flowNodes are re-built from steps each render without `measured`).
      const posChanges = changes.filter(
        (c) => c.type === "position" && c.dragging && c.position,
      );
      const removeChanges = changes.filter((c) => c.type === "remove");

      if (posChanges.length === 0 && removeChanges.length === 0) return;

      setDraft((d) => {
        let steps = d.steps;
        let edges = d.edges;

        // Apply position updates
        if (posChanges.length > 0) {
          const posMap = new Map<string, { x: number; y: number }>();
          for (const c of posChanges) {
            if (c.type === "position" && c.position) {
              posMap.set(c.id, c.position);
            }
          }
          steps = steps.map((s) =>
            posMap.has(s.id) ? { ...s, position: posMap.get(s.id)! } : s,
          );
        }

        // Apply removals (protect start/end nodes)
        if (removeChanges.length > 0) {
          const requestedIds = new Set(removeChanges.map((c) => c.id));
          // Never delete start or end nodes
          const removedIds = new Set(
            [...requestedIds].filter((id) => {
              const step = d.steps.find((s) => s.id === id);
              return step && step.type !== "start" && step.type !== "end";
            }),
          );
          if (removedIds.size > 0) {
            steps = steps.filter((s) => !removedIds.has(s.id));
            edges = edges.filter(
              (e) => !removedIds.has(e.source) && !removedIds.has(e.target),
            );
          }
        }

        return { ...d, steps, edges };
      });
    },
    [],
  );

  const onEdgesChange: OnEdgesChange = useCallback(
    (changes) => {
      setDraft((d) => {
        const rfEdges = d.edges.map((e) => ({
          id: e.id,
          source: e.source,
          target: e.target,
          sourceHandle: e.sourceHandle,
          targetHandle: e.targetHandle,
          label: e.label,
        }));
        const updated = applyEdgeChanges(changes, rfEdges);
        return {
          ...d,
          edges: updated.map((e) => ({
            id: e.id,
            source: e.source,
            target: e.target,
            sourceHandle: e.sourceHandle ?? undefined,
            targetHandle: e.targetHandle ?? undefined,
            label: typeof e.label === "string" ? e.label : undefined,
          })),
        };
      });
    },
    [],
  );

  const onConnect = useCallback(
    (connection: Connection) => {
      if (!connection.source || !connection.target) return;
      setDraft((d) => {
        // Check if this exact connection already exists
        const exists = d.edges.some(
          (e) =>
            e.source === connection.source &&
            e.target === connection.target &&
            e.sourceHandle === (connection.sourceHandle ?? undefined),
        );
        if (exists) return d;

        // ── Edge validation: one outgoing edge per source handle ──
        // If this source+sourceHandle already has an outgoing edge, replace it
        const srcHandle = connection.sourceHandle ?? undefined;
        const existingFromSource = d.edges.find(
          (e) => e.source === connection.source && e.sourceHandle === (srcHandle ?? undefined),
        );
        let filteredEdges = d.edges;
        if (existingFromSource) {
          // Replace the old edge from this source handle
          filteredEdges = d.edges.filter((e) => e.id !== existingFromSource.id);
        }

        // ── Multiple incoming edges are allowed on any node ──
        // (Many branches — e.g. approve from task A AND a condition "yes" — can
        //  both lead to the same downstream step without needing a Merge node.)
        // No de-duplication of incoming edges needed here.

        // Determine label for condition / task-action edges
        let label: string | undefined;
        const sourceStep = d.steps.find((s) => s.id === connection.source);
        if (sourceStep?.type === "condition") {
          label = connection.sourceHandle === "yes" ? "Yes" : connection.sourceHandle === "no" ? "No" : undefined;
        } else if (sourceStep?.type === "task" && connection.sourceHandle && connection.sourceHandle !== "source") {
          // Capitalise the action name as the edge label (approve → Approve)
          label = connection.sourceHandle.charAt(0).toUpperCase() + connection.sourceHandle.slice(1);
        }

        const newEdge: WorkflowEdge = {
          id: `e_${connection.source}_${connection.sourceHandle || "s"}_${connection.target}_${Date.now()}`,
          source: connection.source,
          target: connection.target,
          sourceHandle: connection.sourceHandle ?? undefined,
          targetHandle: connection.targetHandle ?? undefined,
          label,
        };
        return { ...d, edges: [...filteredEdges, newEdge] };
      });
    },
    [],
  );

  /* ── Node selection ── */
  const handleSelectStep = useCallback((id: string | null) => {
    setSelectedId(id);
  }, []);

  /* ── Step CRUD ── */
  const handleStepChange = useCallback((updated: WorkflowStep) => {
    setDraft((d) => ({
      ...d,
      steps: d.steps.map((s) => (s.id === updated.id ? updated : s)),
    }));
  }, []);

  const handleDeleteStep = useCallback((id: string) => {
    // Don't delete start/end
    setDraft((d) => {
      const step = d.steps.find((s) => s.id === id);
      if (step?.type === "start" || step?.type === "end") return d;
      return {
        ...d,
        steps: d.steps.filter((s) => s.id !== id),
        edges: d.edges.filter((e) => e.source !== id && e.target !== id),
      };
    });
    setSelectedId((cur) => (cur === id ? null : cur));
  }, []);

  const handleDeleteEdge = useCallback((id: string) => {
    setDraft((d) => ({ ...d, edges: d.edges.filter((e) => e.id !== id) }));
  }, []);

  const handleCloseEditor = useCallback(() => setSelectedId(null), []);

  /* ── Add node from toolbar ── */
  const addNodeCounter = useRef(0);
  const handleAddNode = useCallback((type: NodeType) => {
    addNodeCounter.current += 1;
    const step = createBlankStep(addNodeCounter.current + draft.steps.length);
    step.type = type;
    step.title = NODE_TYPE_CONFIG[type]?.label ?? `Step ${step.id.slice(-5)}`;
    if (type === "parallel") step.branches = 2;
    if (type === "merge") step.mergeInputs = 2;
    // Place near center, offset slightly for each add
    step.position = {
      x: 250 + (addNodeCounter.current % 4) * 40,
      y: 100 + draft.steps.length * 100,
    };
    setDraft((d) => ({ ...d, steps: [...d.steps, step] }));
    setSelectedId(step.id);
  }, [draft.steps.length]);

  /* ── Sidebar tags ── */
  const addTag = useCallback(() => {
    const tag = tagInput.trim();
    if (tag && !draft.tags.includes(tag)) setDraft((d) => ({ ...d, tags: [...d.tags, tag] }));
    setTagInput("");
  }, [tagInput, draft.tags]);

  const removeTag = useCallback((tag: string) => {
    setDraft((d) => ({ ...d, tags: d.tags.filter((t) => t !== tag) }));
  }, []);

  /* ── Publish ── */
  const realSteps = draft.steps.filter((s) => s.type !== "start" && s.type !== "end");
  const canPublish = draft.name.trim() && realSteps.length > 0;
  // True when at least one field differs from what was loaded
  const hasChanges = !editingId || originalDraftRef.current === null
    ? true
    : JSON.stringify(draft) !== originalDraftRef.current;
  const commitOk = !editingId || commitMessage.trim().length > 0;
  // Only prompt discard when there is actually something to lose
  const needsConfirmation = editingId
    ? hasChanges
    : JSON.stringify(draft) !== JSON.stringify(INITIAL_DRAFT);
  const handleCancelOrBack = useCallback(() => {
    if (needsConfirmation) { setShowDiscardModal(true); } else { handleBack(); }
  }, [needsConfirmation, handleBack]);

  /** Validate the workflow graph is runnable before publishing */
  function validateWorkflow(): string[] {
    const errors: string[] = [];
    const { steps, edges } = draft;

    // ── 1. Exactly one Start & at least one End ───────────────
    const startNodes = steps.filter((s) => s.type === "start");
    const endNodes = steps.filter((s) => s.type === "end");
    if (startNodes.length === 0) errors.push("Workflow must have a Start node.");
    if (startNodes.length > 1) errors.push("Workflow must have exactly one Start node.");
    if (endNodes.length === 0) errors.push("Workflow must have at least one End node.");

    // ── Lookup helpers ────────────────────────────────────────
    const outgoing = (id: string) => edges.filter((e) => e.source === id);
    const incoming = (id: string) => edges.filter((e) => e.target === id);

    // ── 2. Start has outgoing, End has incoming ──────────────
    for (const s of startNodes) {
      if (outgoing(s.id).length === 0)
        errors.push(`Start node "${s.title}" has no outgoing connection.`);
    }
    for (const e of endNodes) {
      if (incoming(e.id).length === 0)
        errors.push(`End node "${e.title}" has no incoming connection.`);
    }

    // ── 3. Every non-end node must have at least one outgoing edge ──
    for (const s of steps) {
      if (s.type === "end") continue;
      if (outgoing(s.id).length === 0)
        errors.push(`"${s.title}" (${s.type}) has no outgoing connection.`);
    }

    // ── 4. Every non-start node must have at least one incoming edge ──
    for (const s of steps) {
      if (s.type === "start") continue;
      if (incoming(s.id).length === 0)
        errors.push(`"${s.title}" (${s.type}) has no incoming connection — it is unreachable.`);
    }

    // ── 5. Condition nodes need both yes & no branches ──────
    for (const s of steps.filter((n) => n.type === "condition")) {
      const out = outgoing(s.id);
      const hasYes = out.some((e) => e.sourceHandle === "yes");
      const hasNo = out.some((e) => e.sourceHandle === "no");
      if (!hasYes) errors.push(`Condition "${s.title}" is missing the Yes branch.`);
      if (!hasNo) errors.push(`Condition "${s.title}" is missing the No branch.`);
    }

    // ── 6. Parallel must have ≥ 2 outgoing branches ─────────
    for (const s of steps.filter((n) => n.type === "parallel")) {
      if (outgoing(s.id).length < 2)
        errors.push(`Parallel "${s.title}" needs at least 2 outgoing branches.`);
    }

    // ── 7. Merge must have ≥ 2 incoming branches ────────────
    for (const s of steps.filter((n) => n.type === "merge")) {
      if (incoming(s.id).length < 2)
        errors.push(`Merge "${s.title}" needs at least 2 incoming branches.`);
    }

    // ── 8. Task nodes need an assigned role ──────────────────
    for (const s of steps.filter((n) => n.type === "task")) {
      if (!s.assignedRole)
        errors.push(`Task "${s.title}" has no assigned role.`);

      // ── 8b. Task nodes with ≥ 2 actions need edges per action handle ──
      const actions = s.taskActions ?? [];
      if (actions.length >= 2) {
        const out = outgoing(s.id);
        for (const act of actions) {
          const hasEdge = out.some((e) => e.sourceHandle === act);
          if (!hasEdge)
            errors.push(`Task "${s.title}" is missing a connection for the "${act}" action branch.`);
        }
      }
    }

    // ── 9. Action nodes need a connector configured ─────────
    for (const s of steps.filter((n) => n.type === "action")) {
      if (!s.connector?.type)
        errors.push(`Action "${s.title}" has no connector selected.`);
    }

    // ── 10. All nodes reachable from Start (BFS) ────────────
    if (startNodes.length === 1) {
      const visited = new Set<string>();
      const queue = [startNodes[0].id];
      while (queue.length > 0) {
        const cur = queue.shift()!;
        if (visited.has(cur)) continue;
        visited.add(cur);
        for (const e of outgoing(cur)) {
          if (!visited.has(e.target)) queue.push(e.target);
        }
      }
      const unreachable = steps.filter((s) => !visited.has(s.id));
      for (const u of unreachable) {
        errors.push(`"${u.title}" (${u.type}) is not reachable from Start.`);
      }

      // ── 11. At least one End is reachable ─────────────────
      const reachableEnds = endNodes.filter((e) => visited.has(e.id));
      if (endNodes.length > 0 && reachableEnds.length === 0)
        errors.push("No End node is reachable from Start.");
    }

    return errors;
  }

  const handlePublish = useCallback(async () => {
    if (!canPublish) return;
    // Guard: update requires actual changes and a commit message
    if (editingId && !hasChanges) { showToast("No changes detected.", "warning"); return; }
    if (editingId && !commitMessage.trim()) { showToast("A commit message is required.", "warning"); return; }

    // Validate before publishing
    const errors = validateWorkflow();
    if (errors.length > 0) {
      setPublishErrors(errors);
      return;
    }
    setPublishErrors([]);

    // ── Convert canvas (steps + edges) → backend schema (node-embedded routing) ──

    // Build a lookup: sourceId → outgoing edges
    const edgesBySource: Record<string, typeof draft.edges> = {};
    for (const e of draft.edges) {
      (edgesBySource[e.source] ??= []).push(e);
    }

    const backendNodes: any[] = [];

    for (const s of draft.steps) {
      const out = edgesBySource[s.id] || [];
      const nodeType = s.type || "task";

      // Base node — fields every node carries
      const node: Record<string, any> = {
        id: s.id,
        type: nodeType,
        title: s.title,
        description: s.description || "",
        position: s.position,
      };

      // ── Routing by node type ──────────────────────
      switch (nodeType) {
        case "start":
        case "action": {
          // Single successor via "next"
          node.next = out.length > 0 ? out[0].target : "";
          break;
        }
        case "task": {
          // If the task has ≥ 2 enabled actions, use action-based branching
          // (each edge's sourceHandle matches the action name).
          const actions = s.taskActions ?? [];
          if (actions.length >= 2) {
            const nextActions: Record<string, string> = {};
            for (const e of out) {
              if (e.sourceHandle && actions.includes(e.sourceHandle as any)) {
                nextActions[e.sourceHandle] = e.target;
              }
            }
            node.next_actions = nextActions;
            // Also keep a fallback "next" pointing to the first target
            node.next = out.length > 0 ? out[0].target : "";
          } else {
            // Single successor — same as start/action
            node.next = out.length > 0 ? out[0].target : "";
          }
          break;
        }
        case "condition": {
          // Two branches: yes / no via sourceHandle
          const yesEdge = out.find((e) => e.sourceHandle === "yes");
          const noEdge = out.find((e) => e.sourceHandle === "no");
          node.condition = s.condition || "";
          node.next_yes = yesEdge?.target || "";
          node.next_no = noEdge?.target || "";
          break;
        }
        case "parallel": {
          // Fan-out to all outgoing edges
          node.next_branches = out.map((e) => e.target);
          break;
        }
        case "merge": {
          // Single successor + required/optional inputs from incoming edges
          node.next = out.length > 0 ? out[0].target : "";
          // requiredInputs stores handle IDs like "input-0"; map them to
          // source node IDs by matching incoming edges on targetHandle
          const incomingEdges = draft.edges.filter((e) => e.target === s.id);
          const requiredHandles = new Set(s.requiredInputs || []);
          const required: string[] = [];
          const optional: string[] = [];
          for (const ie of incomingEdges) {
            if (requiredHandles.has(ie.targetHandle || "")) {
              required.push(ie.source);
            } else {
              optional.push(ie.source);
            }
          }
          // If no explicit toggle was set, treat all as required
          node.required_inputs = required.length > 0 ? required : incomingEdges.map((e) => e.source);
          node.optional_inputs = optional;
          break;
        }
        case "end": {
          // Terminal — no routing
          break;
        }
      }

      // ── Task-specific fields ──────────────────────
      if (nodeType === "task") {
        node.assigned_role = s.assignedRole || "";
        node.assigned_position = s.assignedPosition || "";
        node.assigned_user = s.assignedUser || "";
        node.task_actions = s.taskActions || [];
        node.form_template_id = s.formTemplateId || "";
        node.sla_days = s.slaDays || 0;
      }

      // ── Action connector fields ───────────────────
      if (nodeType === "action" && s.connector) {
        node.connector = {
          type: s.connector.type,
          params: s.connector.params || {},
        };
      }

      backendNodes.push(node);
    }

    // ── Build final payload matching backend Workflow struct ──
    const payload = {
      name: draft.name,
      description: draft.description,
      department: draft.department,
      version: 1,
      status: "active",
      trigger: {
        type: draft.trigger.type === "form_submission" ? "form_submit" : draft.trigger.type,
        config: draft.trigger.config,
      },
      nodes: backendNodes,
      tags: draft.tags,
      // Store the original canvas JSON for re-import later
      raw_json: JSON.stringify({ steps: draft.steps, edges: draft.edges }),
    };

    try {
      // If editing an existing workflow, PUT to update; otherwise POST to create
      const url = editingId
        ? `http://localhost:8085/workflows/${editingId}`
        : "http://localhost:8085/workflows";
      const method = editingId ? "PUT" : "POST";

      // Wrap with commit_message for updates (audit trail, not persisted in DB)
      const requestBody = editingId
        ? { ...payload, commit_message: commitMessage.trim() || "No message" }
        : payload;

      const res = await fetch(url, {
        method,
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(requestBody),
      });
      if (!res.ok) {
        const errText = await res.text();
        throw new Error(`publish failed (${res.status}): ${errText}`);
      }
      const resBody = await res.json();
      setShowPublishModal(false);
      const msg = editingId ? `Workflow updated!` : `Workflow published! ID: ${resBody.id}`;
      router.push(`/dashboard/workstation?toast=${encodeURIComponent(msg)}&toastType=success`);
    } catch (err: any) {
      console.error(err);
      showToast("Failed to publish workflow: " + (err.message || err), "error");
    }
  }, [canPublish, hasChanges, commitMessage, draft, editingId, router, showToast]);

  const handleSaveDraft = useCallback(async () => {
    if (!draft.name.trim()) { showToast("Please enter a workflow name before saving.", "warning"); return; }
    // Build backend nodes (same logic as publish but skip validation)
    const edgesBySource: Record<string, typeof draft.edges> = {};
    for (const e of draft.edges) { (edgesBySource[e.source] ??= []).push(e); }
    const backendNodes: any[] = draft.steps.map((s) => {
      const out = edgesBySource[s.id] || [];
      const node: Record<string, any> = { id: s.id, type: s.type, title: s.title, description: s.description || "", position: s.position };
      if (s.type === "task") {
        node.next = out[0]?.target || "";
        node.assigned_role = s.assignedRole || "";
        node.assigned_position = s.assignedPosition || "";
        node.assigned_user = s.assignedUser || "";
        node.task_actions = s.taskActions || [];
        node.form_template_id = s.formTemplateId || "";
        node.sla_days = s.slaDays || 0;
      } else if (s.type === "condition") {
        node.condition = s.condition || "";
        node.next_yes = out.find((e) => e.sourceHandle === "yes")?.target || "";
        node.next_no = out.find((e) => e.sourceHandle === "no")?.target || "";
      } else if (s.type === "parallel") {
        node.next_branches = out.map((e) => e.target);
      } else {
        node.next = out[0]?.target || "";
      }
      return node;
    });
    const payload = {
      name: draft.name,
      description: draft.description,
      department: draft.department,
      version: 0,
      status: "draft",
      trigger: { type: draft.trigger.type === "form_submission" ? "form_submit" : draft.trigger.type, config: draft.trigger.config },
      nodes: backendNodes,
      tags: draft.tags,
      raw_json: JSON.stringify({ steps: draft.steps, edges: draft.edges }),
    };
    try {
      const url = editingId ? `http://localhost:8085/workflows/${editingId}` : "http://localhost:8085/workflows";
      const method = editingId ? "PUT" : "POST";
      const requestBody = editingId ? { ...payload, commit_message: "Saved as draft" } : payload;
      const res = await fetch(url, { method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(requestBody) });
      if (!res.ok) { const t = await res.text(); throw new Error(`${res.status}: ${t}`); }
      const resBody = await res.json();
      const msg = editingId ? `Draft updated.` : `Draft saved! ID: ${resBody.id}`;
      router.push(`/dashboard/workstation?toast=${encodeURIComponent(msg)}&toastType=success`);
    } catch (err: any) {
      showToast("Failed to save draft: " + (err.message || err), "error");
    }
  }, [draft, editingId, router, showToast]);

  /* ── Selected step for editor ── */
  const selectedStep =
    selectedId && selectedId !== "__trigger__"
      ? (draft.steps.find((s) => s.id === selectedId) ?? null)
      : null;
  const selectedStepIndex = selectedStep
    ? draft.steps.findIndex((s) => s.id === selectedStep.id)
    : -1;

  /* ── Node types available for the toolbar (exclude start/end  — auto-created) ── */
  const addableTypes: NodeType[] = ["task", "action", "condition", "parallel", "merge"];

  return (
    <>
    <RoleGate
      allowed={["org_admin", "admin"]}
      fallback={
        <div className="wfb-fullscreen">
          <div className="wfb-access-denied">
            <h3>Access Restricted</h3>
            <p>Only Admins can create workflows.</p>
            <button className="action-btn action-btn-primary" onClick={handleBack}>Go Back</button>
          </div>
        </div>
      }
    >
      {/* ── Initial Details Dialog ── */}
      {showDetailsDialog && (
        <div className="wfb-dialog-overlay">
          <div className="wfb-dialog">
            <div className="wfb-dialog-header">
              <h2 className="wfb-dialog-title">Create New Workflow</h2>
              <p className="wfb-dialog-subtitle">
                Set up the basics — you can change these anytime from the sidebar.
              </p>
            </div>

            <div className="wfb-dialog-body">
              <div className="wf-field">
                <label className="wf-field-label">Workflow Name</label>
                <input
                  className="wf-input"
                  placeholder="e.g. Employee Onboarding"
                  value={dlgName}
                  onChange={(e) => setDlgName(e.target.value)}
                  autoFocus
                />
              </div>

              <div className="wf-field">
                <label className="wf-field-label">Description</label>
                <textarea
                  className="wf-textarea"
                  placeholder="What does this workflow automate?"
                  rows={3}
                  value={dlgDesc}
                  onChange={(e) => setDlgDesc(e.target.value)}
                />
              </div>

              <div className="wf-field">
                <label className="wf-field-label">Department</label>
                <select
                  className="wf-select"
                  value={dlgDept}
                  onChange={(e) => setDlgDept(e.target.value)}
                >
                  <option value="">Select department...</option>
                  {MOCK_DEPARTMENTS.map((d) => (
                    <option key={d.id} value={d.name}>
                      {d.name}
                    </option>
                  ))}
                </select>
              </div>

              <div className="wf-field">
                <label className="wf-field-label">Tags</label>
                <div className="wf-tag-input-row">
                  <input
                    className="wf-input"
                    placeholder="Add a tag..."
                    value={dlgTagInput}
                    onChange={(e) => setDlgTagInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault();
                        addDlgTag();
                      }
                    }}
                  />
                  <button
                    className="wf-tag-add-btn"
                    onClick={addDlgTag}
                    disabled={!dlgTagInput.trim()}
                  >
                    +
                  </button>
                </div>
                {dlgTags.length > 0 && (
                  <div className="wf-tags-list">
                    {dlgTags.map((tag) => (
                      <span key={tag} className="wf-tag">
                        {tag}
                        <button
                          onClick={() => removeDlgTag(tag)}
                          className="wf-tag-remove"
                        >
                          ×
                        </button>
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>

            <div className="wfb-dialog-actions">
              <button
                className="action-btn action-btn-outline"
                onClick={handleBack}
              >
                Cancel
              </button>
              <button
                className="action-btn action-btn-primary"
                disabled={!dlgName.trim()}
                onClick={handleDialogContinue}
              >
                Continue to Builder
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  strokeWidth={2}
                  stroke="currentColor"
                  width="16"
                  height="16"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M13.5 4.5 21 12m0 0-7.5 7.5M21 12H3"
                  />
                </svg>
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ── Full-Screen Builder ── */}
      {!showDetailsDialog && (
        <div className="wfb-fullscreen">
          {/* Top toolbar */}
          <div className="wfb-toolbar">
            <div className="wfb-toolbar-left">
              <button className="wf-back-btn" onClick={handleCancelOrBack} title="Back to Workstation">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  strokeWidth={2}
                  stroke="currentColor"
                  width="18"
                  height="18"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M10.5 19.5 3 12m0 0 7.5-7.5M3 12h18"
                  />
                </svg>
              </button>

              <div className="wf-toolbar-meta">
                <input
                  className="wf-name-input"
                  placeholder="Untitled Workflow"
                  value={draft.name}
                  onChange={(e) =>
                    setDraft((d) => ({ ...d, name: e.target.value }))
                  }
                />
                <span className="wf-step-count">
                  {realSteps.length} step
                  {realSteps.length !== 1 ? "s" : ""}
                  {draft.department && (
                    <>
                      {" "}
                      · <span style={{ color: "var(--accent)" }}>{draft.department}</span>
                    </>
                  )}
                </span>
              </div>
            </div>

            <div className="wfb-toolbar-right">
              {/* Theme toggle */}
              <button
                className="action-btn action-btn-outline wfb-theme-toggle"
                onClick={toggleTheme}
                title={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
              >
                {theme === "dark" ? (
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v2.25m6.364.386-1.591 1.591M21 12h-2.25m-.386 6.364-1.591-1.591M12 18.75V21m-4.773-4.227-1.591 1.591M5.25 12H3m4.227-4.773L5.636 5.636M15.75 12a3.75 3.75 0 1 1-7.5 0 3.75 3.75 0 0 1 7.5 0Z" />
                  </svg>
                ) : (
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M21.752 15.002A9.72 9.72 0 0 1 18 15.75c-5.385 0-9.75-4.365-9.75-9.75 0-1.33.266-2.597.748-3.752A9.753 9.753 0 0 0 3 11.25C3 16.635 7.365 21 12.75 21a9.753 9.753 0 0 0 9.002-5.998Z" />
                  </svg>
                )}
              </button>
              <button
                className="action-btn action-btn-outline wfb-details-toggle"
                onClick={() => setDetailsSidebarOpen((o) => !o)}
                title="Workflow details"
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  strokeWidth={1.5}
                  stroke="currentColor"
                  width="16"
                  height="16"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M11.25 11.25l.041-.02a.75.75 0 0 1 1.063.852l-.708 2.836a.75.75 0 0 0 1.063.853l.041-.021M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9-3.75h.008v.008H12V8.25Z"
                  />
                </svg>
                Details
              </button>

              <button
                className="action-btn action-btn-outline"
                onClick={handleCancelOrBack}
              >
                Cancel
              </button>

              <button
                className="action-btn action-btn-outline"
                disabled={!draft.name.trim()}
                onClick={handleSaveDraft}
                title="Save without validation as a draft"
              >
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="16" height="16">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M17.593 3.322c1.1.128 1.907 1.046 1.907 2.185V21L12 17.25 4.5 21V5.507c0-1.139.806-2.057 1.907-2.185a48.507 48.507 0 0 1 11.186 0Z" />
                </svg>
                Save Draft
              </button>

              <button
                className="action-btn action-btn-primary"
                disabled={!canPublish || !hasChanges}
                onClick={() => setShowPublishModal(true)}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  fill="none"
                  viewBox="0 0 24 24"
                  strokeWidth={1.5}
                  stroke="currentColor"
                  width="18"
                  height="18"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 16.5V9.75m0 0 3 3m-3-3-3 3M6.75 19.5a4.5 4.5 0 0 1-1.41-8.775 5.25 5.25 0 0 1 10.233-2.33 3 3 0 0 1 3.758 3.848A3.752 3.752 0 0 1 18 19.5H6.75Z"
                  />
                </svg>
                {editingId ? "Update" : "Publish"}
              </button>
            </div>
          </div>

          {/* Builder body */}
          <div className="wfb-body">
            {/* Details sidebar (slide from left) */}
            <aside
              className={`wfb-details-sidebar ${detailsSidebarOpen ? "open" : ""}`}
            >
              <div className="wfb-sidebar-header">
                <h4 className="wf-panel-title">Workflow Details</h4>
                <button
                  className="wf-editor-close"
                  onClick={() => setDetailsSidebarOpen(false)}
                >
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" width="18" height="18">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M6 18 18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              <div className="wfb-sidebar-body">
                <div className="wf-field">
                  <label className="wf-field-label">Name</label>
                  <input className="wf-input" value={draft.name} onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))} />
                </div>
                <div className="wf-field">
                  <label className="wf-field-label">Description</label>
                  <textarea className="wf-textarea" placeholder="What does this workflow automate?" rows={3} value={draft.description} onChange={(e) => setDraft((d) => ({ ...d, description: e.target.value }))} />
                </div>
                <div className="wf-field">
                  <label className="wf-field-label">Department</label>
                  <select className="wf-select" value={draft.department} onChange={(e) => setDraft((d) => ({ ...d, department: e.target.value }))}>
                    <option value="">Select department...</option>
                    {MOCK_DEPARTMENTS.map((dep) => (<option key={dep.id} value={dep.name}>{dep.name}</option>))}
                  </select>
                </div>
                <div className="wf-field">
                  <label className="wf-field-label">Tags</label>
                  <div className="wf-tag-input-row">
                    <input className="wf-input" placeholder="Add a tag..." value={tagInput} onChange={(e) => setTagInput(e.target.value)} onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); addTag(); } }} />
                    <button className="wf-tag-add-btn" onClick={addTag} disabled={!tagInput.trim()}>+</button>
                  </div>
                  {draft.tags.length > 0 && (
                    <div className="wf-tags-list">
                      {draft.tags.map((tag) => (
                        <span key={tag} className="wf-tag">{tag}<button onClick={() => removeTag(tag)} className="wf-tag-remove">&times;</button></span>
                      ))}
                    </div>
                  )}
                </div>

                {/* Trigger config shortcut */}
                <div className="wf-field">
                  <label className="wf-field-label">Trigger</label>
                  <button className="action-btn action-btn-outline" style={{ width: "100%" }} onClick={() => setSelectedId("__trigger__")}>
                    Configure Trigger ({draft.trigger.type.replace(/_/g, " ")})
                  </button>
                </div>
              </div>
            </aside>

            {/* Center canvas */}
            <div className="wfb-canvas-area">
              {/* ── Add Node Toolbar (floating) ── */}
              <div className="wfb-add-toolbar">
                <span className="wfb-add-toolbar-label">Add Node:</span>
                {addableTypes.map((nt) => {
                  const cfg = NODE_TYPE_CONFIG[nt];
                  return (
                    <button
                      key={nt}
                      className="wfb-add-node-btn"
                      style={{ borderColor: cfg.color }}
                      onClick={() => handleAddNode(nt)}
                      title={`Add ${cfg.label} node`}
                    >
                      <span className="wfb-add-node-dot" style={{ background: cfg.color }} />
                      {cfg.label}
                    </button>
                  );
                })}
              </div>

              <WorkflowCanvas
                steps={draft.steps}
                edges={draft.edges}
                selectedStepId={selectedId}
                onSelectStep={handleSelectStep}
                onNodesChange={onNodesChange}
                onEdgesChange={onEdgesChange}
                onConnect={onConnect}
                onDeleteStep={handleDeleteStep}
                onDeleteEdge={handleDeleteEdge}
              />
            </div>

            {/* Right: Step / Trigger editor */}
            <div className={`wf-editor-panel ${selectedId ? "open" : ""}`}>
              {selectedId === "__trigger__" && (
                <TriggerEditor
                  trigger={draft.trigger}
                  onChange={handleTriggerChange}
                  onClose={handleCloseEditor}
                />
              )}
              {selectedStep && (
                <StepEditor
                  step={selectedStep}
                  stepIndex={selectedStepIndex}
                  onChange={handleStepChange}
                  onClose={handleCloseEditor}
                />
              )}
              {!selectedId && (
                <div className="wf-editor-empty">
                  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor" width="48" height="48">
                    <path strokeLinecap="round" strokeLinejoin="round" d="M15.042 21.672 13.684 16.6m0 0-2.51 2.225.569-9.47 5.227 7.917-3.286-.672ZM12 2.25V4.5m5.834.166-1.591 1.591M20.25 10.5H18M7.757 14.743l-1.59 1.59M6 10.5H3.75m4.007-4.243-1.59-1.59" />
                  </svg>
                  <p>Click on a <strong>node</strong> on the canvas to configure it, or drag edges between handles to connect nodes</p>
                </div>
              )}
            </div>
          </div>

          {/* Discard confirmation modal */}
          {showDiscardModal && (
            <div
              className="modal-overlay"
              onClick={() => setShowDiscardModal(false)}
            >
              <div
                className="modal-content"
                onClick={(e) => e.stopPropagation()}
              >
                <h3 className="modal-title">Discard all changes?</h3>
                <p className="modal-desc">
                  Any unsaved changes to{" "}
                  <strong>&ldquo;{draft.name || "this workflow"}&rdquo;</strong>{" "}
                  will be permanently lost. This cannot be undone.
                </p>
                <div className="modal-actions">
                  <button
                    className="action-btn action-btn-outline"
                    onClick={() => setShowDiscardModal(false)}
                  >
                    Keep editing
                  </button>
                  <button
                    className="action-btn"
                    style={{ background: "#ef4444", color: "#fff", border: "none" }}
                    onClick={() => { setShowDiscardModal(false); handleBack(); }}
                  >
                    Discard &amp; leave
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* Publish confirmation modal */}
          {showPublishModal && (
            <div
              className="modal-overlay"
              onClick={() => setShowPublishModal(false)}
            >
              <div
                className="modal-content"
                onClick={(e) => e.stopPropagation()}
              >
                <h3 className="modal-title">Publish Workflow</h3>
                <p className="modal-desc">
                  You are about to publish{" "}
                  <strong>&ldquo;{draft.name}&rdquo;</strong> with{" "}
                  {realSteps.length} step
                  {realSteps.length !== 1 ? "s" : ""}. Once published, it
                  will become active and can be triggered automatically.
                </p>

                {/* ── Validation errors ── */}
                {publishErrors.length > 0 && (
                  <div className="wf-publish-errors">
                    <div className="wf-publish-errors-title">Cannot publish — fix these issues:</div>
                    <ul className="wf-publish-errors-list">
                      {publishErrors.map((err, i) => (
                        <li key={i}>{err}</li>
                      ))}
                    </ul>
                  </div>
                )}

                <div className="wf-publish-summary">
                  <div className="wf-publish-summay-row">
                    <span>Trigger</span>
                    <strong>
                      {draft.trigger.type.replace(/_/g, " ")}
                    </strong>
                  </div>
                  <div className="wf-publish-summay-row">
                    <span>Department</span>
                    <strong>{draft.department || "None"}</strong>
                  </div>
                  <div className="wf-publish-summay-row">
                    <span>Steps</span>
                    <strong>{realSteps.length}</strong>
                  </div>
                  <div className="wf-publish-summay-row">
                    <span>Roles involved</span>
                    <strong>
                      {[
                        ...new Set(
                          draft.steps
                            .map((s) => s.assignedRole)
                            .filter(Boolean)
                        ),
                      ].join(", ") || "None assigned"}
                    </strong>
                  </div>
                </div>

                {/* No-changes warning when updating */}
                {editingId && !hasChanges && (
                  <div style={{ marginTop: 12, padding: "10px 14px", borderRadius: 8, background: "rgba(245,158,11,0.1)", border: "1px solid rgba(245,158,11,0.35)", fontSize: "0.82rem", color: "#b45309" }}>
                    No changes detected. Modify the workflow before updating.
                  </div>
                )}

                {/* Commit message — only when updating */}
                {editingId && (
                  <div className="wf-field" style={{ marginTop: 12 }}>
                    <label className="wf-field-label">
                      Commit Message
                      <span className="wf-required-star" style={{ marginLeft: 4 }}>*</span>
                    </label>
                    <span className="wf-field-hint">Required. Briefly describe what changed (for audit trail).</span>
                    <textarea
                      className="wf-textarea"
                      rows={2}
                      placeholder="e.g. Added finance approval step"
                      value={commitMessage}
                      onChange={(e) => setCommitMessage(e.target.value)}
                      style={!commitMessage.trim() ? { borderColor: "#ef4444" } : {}}
                    />
                    {!commitMessage.trim() && (
                      <span style={{ fontSize: "0.75rem", color: "#ef4444", marginTop: 4, display: "block" }}>
                        A commit message is required to save changes.
                      </span>
                    )}
                  </div>
                )}

                <div className="modal-actions">
                  <button
                    className="action-btn action-btn-outline"
                    onClick={() => { setShowPublishModal(false); setPublishErrors([]); }}
                  >
                    Cancel
                  </button>
                  <button
                    className="action-btn action-btn-primary"
                    disabled={editingId ? !commitOk || !hasChanges : false}
                    onClick={handlePublish}
                  >
                    {publishErrors.length > 0 ? "Re-check & Publish" : editingId ? "Confirm & Update" : "Confirm & Publish"}
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </RoleGate>
    <ToastContainer toasts={toasts} onDismiss={dismissToast} />
    </>
  );
}
