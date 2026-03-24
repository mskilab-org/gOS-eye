package dag

import "testing"

func TestIsProcessNode_ProcessWithLabel(t *testing.T) {
	// Typical process node: non-empty label, default (empty) shape
	n := dotNode{ID: "v7", Label: "BWA_MEM", Shape: ""}
	if !isProcessNode(n) {
		t.Fatal("expected true for process node with label and no shape")
	}
}

func TestIsProcessNode_OperatorCircle(t *testing.T) {
	// Operator node: empty label, shape=circle
	n := dotNode{ID: "v1", Label: "", Shape: "circle"}
	if isProcessNode(n) {
		t.Fatal("expected false for operator node (circle)")
	}
}

func TestIsProcessNode_ChannelSourcePoint(t *testing.T) {
	// Channel source: empty label, shape=point
	n := dotNode{ID: "v0", Label: "", Shape: "point"}
	if isProcessNode(n) {
		t.Fatal("expected false for channel source (point)")
	}
}

func TestIsProcessNode_TerminalPoint(t *testing.T) {
	// Terminal node: empty label, shape=point
	n := dotNode{ID: "v18", Label: "", Shape: "point"}
	if isProcessNode(n) {
		t.Fatal("expected false for terminal node (point)")
	}
}

func TestIsProcessNode_EmptyLabelEmptyShape(t *testing.T) {
	// No label, no shape — not a process
	n := dotNode{ID: "v99", Label: "", Shape: ""}
	if isProcessNode(n) {
		t.Fatal("expected false when label is empty")
	}
}

func TestIsProcessNode_LabelWithShapePoint(t *testing.T) {
	// Has a label but shape=point — not a process
	n := dotNode{ID: "v5", Label: "FAKE", Shape: "point"}
	if isProcessNode(n) {
		t.Fatal("expected false when shape is point even with label")
	}
}

func TestIsProcessNode_LabelWithShapeCircle(t *testing.T) {
	// Has a label but shape=circle — not a process
	n := dotNode{ID: "v5", Label: "FAKE", Shape: "circle"}
	if isProcessNode(n) {
		t.Fatal("expected false when shape is circle even with label")
	}
}

func TestIsProcessNode_AllFieldsEmpty(t *testing.T) {
	n := dotNode{}
	if isProcessNode(n) {
		t.Fatal("expected false for zero-value dotNode")
	}
}
