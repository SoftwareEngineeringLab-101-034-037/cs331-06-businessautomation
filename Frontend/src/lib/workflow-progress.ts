export type WorkflowProgressNode = {
  id?: string;
  type?: string;
  next?: string;
  next_yes?: string;
  next_no?: string;
  next_branches?: string[];
  next_actions?: Record<string, string>;
};

export type WorkflowProgressNodeState = {
  status?: string;
};

export type WorkflowProgress = {
  checkpointNumber: number;
  totalCheckpoints: number;
  completedCheckpoints: number;
  percentComplete: number;
};

function isActionableNode(node: WorkflowProgressNode | undefined): boolean {
  if (!node || !node.id) return false;
  return node.type !== "start" && node.type !== "end";
}

function nextNodeIDs(node: WorkflowProgressNode): string[] {
  const out = new Set<string>();

  const push = (candidate?: string) => {
    const value = String(candidate || "").trim();
    if (!value) return;
    out.add(value);
  };

  push(node.next);
  push(node.next_yes);
  push(node.next_no);
  for (const branch of node.next_branches || []) {
    push(branch);
  }
  for (const target of Object.values(node.next_actions || {})) {
    push(target);
  }

  return Array.from(out);
}

function buildDepthMap(nodes: WorkflowProgressNode[]): Map<string, number> {
  const byID = new Map<string, WorkflowProgressNode>();
  const edges: Array<{ from: string; to: string }> = [];
  const inDegree = new Map<string, number>();

  for (const node of nodes) {
    const id = String(node.id || "").trim();
    if (!id) continue;
    byID.set(id, node);
    if (!inDegree.has(id)) {
      inDegree.set(id, 0);
    }
  }

  for (const node of nodes) {
    const from = String(node.id || "").trim();
    if (!from) continue;

    for (const to of nextNodeIDs(node)) {
      if (!byID.has(to)) continue;
      edges.push({ from, to });
      inDegree.set(to, (inDegree.get(to) || 0) + 1);
    }
  }

  const starts = nodes
    .filter((node) => node.type === "start" && String(node.id || "").trim())
    .map((node) => String(node.id || "").trim());

  const fallbackStarts = Array.from(byID.keys()).filter((id) => (inDegree.get(id) || 0) === 0);
  const rootIDs = starts.length > 0 ? starts : (fallbackStarts.length > 0 ? fallbackStarts : Array.from(byID.keys()).slice(0, 1));

  const depth = new Map<string, number>();
  for (const rootID of rootIDs) {
    depth.set(rootID, 0);
  }

  const maxIterations = Math.max(1, byID.size);
  for (let i = 0; i < maxIterations; i += 1) {
    let changed = false;
    for (const edge of edges) {
      const parentDepth = depth.get(edge.from);
      if (parentDepth == null) continue;
      const candidateDepth = parentDepth + 1;
      const current = depth.get(edge.to);
      if (current == null || candidateDepth > current) {
        depth.set(edge.to, candidateDepth);
        changed = true;
      }
    }
    if (!changed) {
      break;
    }
  }

  return depth;
}

export function computeHeightBasedProgress(
  nodes: WorkflowProgressNode[] | undefined,
  nodeStates: Record<string, WorkflowProgressNodeState> | undefined,
  currentNodeID?: string,
  instanceStatus?: string,
): WorkflowProgress {
  const safeNodes = Array.isArray(nodes) ? nodes : [];
  const depthMap = buildDepthMap(safeNodes);

  const actionableDepths = safeNodes
    .filter((node) => isActionableNode(node) && depthMap.has(String(node.id)))
    .map((node) => depthMap.get(String(node.id)) as number);

  const orderedDepths = Array.from(new Set(actionableDepths)).sort((a, b) => a - b);
  if (orderedDepths.length === 0) {
    return {
      checkpointNumber: 1,
      totalCheckpoints: 1,
      completedCheckpoints: 0,
      percentComplete: instanceStatus === "completed" ? 100 : 0,
    };
  }

  const completedDepthSet = new Set<number>();
  let reachedDepth = -1;

  for (const [nodeID, state] of Object.entries(nodeStates || {})) {
    const d = depthMap.get(nodeID);
    if (d == null) continue;

    if (String(state?.status || "").trim()) {
      reachedDepth = Math.max(reachedDepth, d);
    }
    if (state?.status === "completed") {
      completedDepthSet.add(d);
    }
  }

  const currentDepth = depthMap.get(String(currentNodeID || ""));
  if (currentDepth != null) {
    reachedDepth = Math.max(reachedDepth, currentDepth);
  }

  const completedCheckpoints = orderedDepths.filter((d) => completedDepthSet.has(d)).length;

  if (instanceStatus === "completed") {
    return {
      checkpointNumber: orderedDepths.length,
      totalCheckpoints: orderedDepths.length,
      completedCheckpoints: orderedDepths.length,
      percentComplete: 100,
    };
  }

  const reachedCheckpoint = reachedDepth >= 0
    ? orderedDepths.filter((d) => d <= reachedDepth).length
    : 0;

  const checkpointNumber = Math.min(
    orderedDepths.length,
    Math.max(1, reachedCheckpoint > 0 ? reachedCheckpoint : completedCheckpoints + 1),
  );

  const percentComplete = Math.round((completedCheckpoints / orderedDepths.length) * 1000) / 10;

  return {
    checkpointNumber,
    totalCheckpoints: orderedDepths.length,
    completedCheckpoints,
    percentComplete,
  };
}
