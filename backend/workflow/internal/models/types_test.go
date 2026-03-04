package models

import "testing"

func TestWorkflowHelpers(t *testing.T) {
	wf := Workflow{
		Nodes: []WorkflowNode{
			{ID: "n1", Type: NodeTask, Title: "Task"},
			{ID: "start", Type: NodeStart, Title: "Start"},
		},
	}

	if got := wf.FindNode("n1"); got == nil || got.ID != "n1" {
		t.Fatalf("FindNode did not return expected node: %+v", got)
	}
	if got := wf.FindNode("missing"); got != nil {
		t.Fatalf("expected nil for missing node, got %+v", got)
	}
	if got := wf.StartNode(); got == nil || got.ID != "start" {
		t.Fatalf("StartNode did not return start node: %+v", got)
	}
}

func TestWorkflowNodeNextIDs(t *testing.T) {
	tests := []struct {
		name   string
		node   WorkflowNode
		result string
		want   []string
	}{
		{
			name:   "condition yes",
			node:   WorkflowNode{Type: NodeCondition, NextYes: "yesNode", NextNo: "noNode"},
			result: "yes",
			want:   []string{"yesNode"},
		},
		{
			name:   "condition no",
			node:   WorkflowNode{Type: NodeCondition, NextYes: "yesNode", NextNo: "noNode"},
			result: "no",
			want:   []string{"noNode"},
		},
		{
			name:   "condition no matching branch",
			node:   WorkflowNode{Type: NodeCondition, NextYes: "yesNode"},
			result: "no",
			want:   nil,
		},
		{
			name:   "task next actions hit",
			node:   WorkflowNode{Type: NodeTask, Next: "fallback", NextActions: map[string]string{"approve": "approvedNode"}},
			result: "approve",
			want:   []string{"approvedNode"},
		},
		{
			name:   "task next actions miss falls back",
			node:   WorkflowNode{Type: NodeTask, Next: "fallback", NextActions: map[string]string{"approve": "approvedNode"}},
			result: "reject",
			want:   []string{"fallback"},
		},
		{
			name:   "parallel returns all branches",
			node:   WorkflowNode{Type: NodeParallel, NextBranches: []string{"b1", "b2"}},
			result: "",
			want:   []string{"b1", "b2"},
		},
		{
			name:   "end node has no next",
			node:   WorkflowNode{Type: NodeEnd},
			result: "",
			want:   nil,
		},
		{
			name:   "default uses next",
			node:   WorkflowNode{Type: NodeAction, Next: "nextNode"},
			result: "",
			want:   []string{"nextNode"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.node.NextIDs(tt.result)
			if len(got) != len(tt.want) {
				t.Fatalf("NextIDs length mismatch: got=%v want=%v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("NextIDs mismatch: got=%v want=%v", got, tt.want)
				}
			}
		})
	}
}
