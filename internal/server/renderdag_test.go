package server

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderDAG_NilLayout(t *testing.T) {
	got := renderDAG(nil, nil)
	if !strings.Contains(got, `id="dag-view"`) {
		t.Errorf("expected dag-view id, got: %s", got)
	}
	// Should be an empty container with no nodes or SVG
	if strings.Contains(got, "dag-node") {
		t.Errorf("expected no dag-node divs for nil layout, got: %s", got)
	}
}

func TestRenderDAG_EmptyLayout(t *testing.T) {
	layout := &dag.Layout{
		Nodes:      []dag.NodeLayout{},
		Edges:      []dag.Edge{},
		LayerCount: 0,
		MaxWidth:   0,
	}
	got := renderDAG(layout, nil)
	if !strings.Contains(got, `id="dag-view"`) {
		t.Errorf("expected dag-view id, got: %s", got)
	}
	if strings.Contains(got, "dag-node") {
		t.Errorf("expected no dag-node divs for empty layout, got: %s", got)
	}
}

func TestRenderDAG_ContainsDagViewID(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "FOO", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)
	if !strings.Contains(got, `id="dag-view"`) {
		t.Errorf("expected dag-view id for Datastar morphing, got: %s", got)
	}
}

func TestRenderDAG_NilRun_AllNodesPending(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
			{Name: "SORT", Layer: 1, Index: 0},
		},
		Edges:      []dag.Edge{{From: "ALIGN", To: "SORT"}},
		LayerCount: 2,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// Both nodes should have status-pending
	if count := strings.Count(got, "status-pending"); count != 2 {
		t.Errorf("expected 2 status-pending nodes, got %d in: %s", count, got)
	}
	// Both should show 0/0 counts
	if count := strings.Count(got, "0/0"); count != 2 {
		t.Errorf("expected 2 nodes with 0/0 counts, got %d", count)
	}
}

func TestRenderDAG_TwoNodes_OneEdge_SVGHasOnePath(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
			{Name: "SORT", Layer: 1, Index: 0},
		},
		Edges:      []dag.Edge{{From: "ALIGN", To: "SORT"}},
		LayerCount: 2,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	if !strings.Contains(got, "<svg") {
		t.Errorf("expected SVG element, got: %s", got)
	}
	if count := strings.Count(got, `class="dag-edge"`); count != 1 {
		t.Errorf("expected 1 dag-edge path, got %d", count)
	}
	// The path should be a cubic bezier
	if !strings.Contains(got, "<path") {
		t.Errorf("expected path element in SVG, got: %s", got)
	}
}

func TestRenderDAG_StatusDerivation_AllCompleted(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	run := &state.Run{
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "ALIGN", Status: "COMPLETED"},
			2: {TaskID: 2, Process: "ALIGN", Status: "COMPLETED"},
		},
	}
	got := renderDAG(layout, run)

	if !strings.Contains(got, "status-completed") {
		t.Errorf("expected status-completed, got: %s", got)
	}
	if !strings.Contains(got, "2/2") {
		t.Errorf("expected 2/2 count, got: %s", got)
	}
}

func TestRenderDAG_StatusDerivation_AnyFailed(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	run := &state.Run{
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "ALIGN", Status: "COMPLETED"},
			2: {TaskID: 2, Process: "ALIGN", Status: "FAILED"},
			3: {TaskID: 3, Process: "ALIGN", Status: "RUNNING"},
		},
	}
	got := renderDAG(layout, run)

	if !strings.Contains(got, "status-failed") {
		t.Errorf("expected status-failed (failed takes priority), got: %s", got)
	}
	if !strings.Contains(got, "1/3") {
		t.Errorf("expected 1/3 (completed/total) count, got: %s", got)
	}
}

func TestRenderDAG_StatusDerivation_AnyRunning(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "SORT", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	run := &state.Run{
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "SORT", Status: "COMPLETED"},
			2: {TaskID: 2, Process: "SORT", Status: "RUNNING"},
		},
	}
	got := renderDAG(layout, run)

	if !strings.Contains(got, "status-running") {
		t.Errorf("expected status-running, got: %s", got)
	}
}

func TestRenderDAG_StatusDerivation_Submitted(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "CALL", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	run := &state.Run{
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "CALL", Status: "SUBMITTED"},
			2: {TaskID: 2, Process: "CALL", Status: "COMPLETED"},
		},
	}
	got := renderDAG(layout, run)

	if !strings.Contains(got, "status-submitted") {
		t.Errorf("expected status-submitted (not all completed, none running/failed), got: %s", got)
	}
}

func TestRenderDAG_StatusDerivation_NoMatchingTasks(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "MISSING", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	run := &state.Run{
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "OTHER", Status: "COMPLETED"},
		},
	}
	got := renderDAG(layout, run)

	if !strings.Contains(got, "status-pending") {
		t.Errorf("expected status-pending for node with no matching tasks, got: %s", got)
	}
	if !strings.Contains(got, "0/0") {
		t.Errorf("expected 0/0 for node with no matching tasks, got: %s", got)
	}
}

func TestRenderDAG_NodeHTMLStructure(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	run := &state.Run{
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "ALIGN", Status: "COMPLETED"},
			2: {TaskID: 2, Process: "ALIGN", Status: "RUNNING"},
			3: {TaskID: 3, Process: "ALIGN", Status: "SUBMITTED"},
		},
	}
	got := renderDAG(layout, run)

	// Node name span
	if !strings.Contains(got, `<span class="dag-node-name">ALIGN</span>`) {
		t.Errorf("expected dag-node-name span with ALIGN, got: %s", got)
	}
	// Count span: 1 completed / 3 total
	if !strings.Contains(got, `<span class="dag-node-counts">1/3</span>`) {
		t.Errorf("expected dag-node-counts span with 1/3, got: %s", got)
	}
	// Progress bar
	if !strings.Contains(got, `class="dag-node-bar"`) {
		t.Errorf("expected dag-node-bar, got: %s", got)
	}
	if !strings.Contains(got, `class="dag-node-fill"`) {
		t.Errorf("expected dag-node-fill, got: %s", got)
	}
}

func TestRenderDAG_ProgressBarPercentage(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "PROC", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	run := &state.Run{
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "PROC", Status: "COMPLETED"},
			2: {TaskID: 2, Process: "PROC", Status: "COMPLETED"},
			3: {TaskID: 3, Process: "PROC", Status: "RUNNING"},
			4: {TaskID: 4, Process: "PROC", Status: "SUBMITTED"},
		},
	}
	got := renderDAG(layout, run)

	// 2 completed / 4 total = 50%
	if !strings.Contains(got, `width:50%`) {
		t.Errorf("expected width:50%% in progress bar, got: %s", got)
	}
}

func TestRenderDAG_NodePositioning(t *testing.T) {
	// Two layers: layer 0 has 2 nodes, layer 1 has 1 node
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
			{Name: "B", Layer: 0, Index: 1},
			{Name: "C", Layer: 1, Index: 0},
		},
		Edges:      []dag.Edge{{From: "A", To: "C"}, {From: "B", To: "C"}},
		LayerCount: 2,
		MaxWidth:   2,
	}
	got := renderDAG(layout, nil)

	// Constants: nodeWidth=180, nodeHeight=64, spacingX=200, spacingY=100, paddingX=40, paddingY=40
	// Container width = 2*200 + 40*2 - (200-180) = 400 + 80 - 20 = 460
	// Container height = 2*100 + 40*2 - (100-64) = 200 + 80 - 36 = 244

	// Layer 0 has 2 nodes: layerWidth = 2*200 - 20 = 380, offset = (460 - 80 - 380)/2 = 0
	// A: x = 40 + 0 + 0*200 = 40, y = 40 + 0*100 = 40
	// B: x = 40 + 0 + 1*200 = 240, y = 40 + 0*100 = 40
	if !strings.Contains(got, "left:40px") {
		t.Errorf("expected node A at left:40px, got: %s", got)
	}
	if !strings.Contains(got, "left:240px") {
		t.Errorf("expected node B at left:240px, got: %s", got)
	}

	// Layer 1 has 1 node: layerWidth = 1*200 - 20 = 180, offset = (460 - 80 - 180)/2 = 100
	// C: x = 40 + 100 + 0*200 = 140, y = 40 + 1*100 = 140
	if !strings.Contains(got, "left:140px") {
		t.Errorf("expected node C at left:140px (centered), got: %s", got)
	}
	if !strings.Contains(got, "top:140px") {
		t.Errorf("expected node C at top:140px, got: %s", got)
	}
}

func TestRenderDAG_SVGEdgeBezier(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
			{Name: "B", Layer: 1, Index: 0},
		},
		Edges:      []dag.Edge{{From: "A", To: "B"}},
		LayerCount: 2,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// Single layer width: nodeWidth=180, spacingX=200, paddingX=40
	// Container width = 1*200 + 80 - 20 = 260
	// Layer 0: layerWidth = 1*200 - 20 = 180, offset = (260-80-180)/2 = 0
	// A: x=40, y=40
	// B: x=40, y=140
	// Source bottom-center: (40+90, 40+64) = (130, 104)
	// Target top-center: (40+90, 140) = (130, 140)
	// cy1 = 104 + (140-104)*0.5 = 104+18 = 122
	// cy2 = 140 - (140-104)*0.5 = 140-18 = 122

	expectedPath := "M130,104 C130,122 130,122 130,140"
	if !strings.Contains(got, expectedPath) {
		t.Errorf("expected bezier path %q, got: %s", expectedPath, got)
	}
}

func TestRenderDAG_ContainerDimensions(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
			{Name: "B", Layer: 0, Index: 1},
			{Name: "C", Layer: 1, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 2,
		MaxWidth:   2,
	}
	got := renderDAG(layout, nil)

	// Container: width=460 (nodeWidth=180), height=244
	if !strings.Contains(got, "width:460px") {
		t.Errorf("expected container width:460px, got: %s", got)
	}
	if !strings.Contains(got, "height:244px") {
		t.Errorf("expected container height:244px, got: %s", got)
	}
}

func TestRenderDAG_MultipleEdges(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
			{Name: "B", Layer: 0, Index: 1},
			{Name: "C", Layer: 1, Index: 0},
		},
		Edges: []dag.Edge{
			{From: "A", To: "C"},
			{From: "B", To: "C"},
		},
		LayerCount: 2,
		MaxWidth:   2,
	}
	got := renderDAG(layout, nil)

	if count := strings.Count(got, `class="dag-edge"`); count != 2 {
		t.Errorf("expected 2 dag-edge paths, got %d", count)
	}
}

func TestRenderDAG_NodeDimensions(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "X", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// Each node should have width:180px and height:64px
	if !strings.Contains(got, "width:180px") {
		t.Errorf("expected node width:180px, got: %s", got)
	}
	if !strings.Contains(got, "height:64px") {
		t.Errorf("expected node height:64px, got: %s", got)
	}
}

func TestRenderDAG_NoEdges_NoSVGPaths(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "SOLO", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// SVG should exist but have no path elements
	if strings.Contains(got, "<path") {
		t.Errorf("expected no path elements with no edges, got: %s", got)
	}
}

func TestRenderDAG_ProgressBar_ZeroTotal(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "PROC", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// 0 total should result in 0% width
	if !strings.Contains(got, `width:0%`) {
		t.Errorf("expected width:0%% for zero total, got: %s", got)
	}
}

func TestRenderDAG_MultipleProcesses_MixedStatuses(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
			{Name: "SORT", Layer: 1, Index: 0},
		},
		Edges:      []dag.Edge{{From: "ALIGN", To: "SORT"}},
		LayerCount: 2,
		MaxWidth:   1,
	}
	run := &state.Run{
		Tasks: map[int]*state.Task{
			1: {TaskID: 1, Process: "ALIGN", Status: "COMPLETED"},
			2: {TaskID: 2, Process: "ALIGN", Status: "COMPLETED"},
			3: {TaskID: 3, Process: "SORT", Status: "RUNNING"},
		},
	}
	got := renderDAG(layout, run)

	if !strings.Contains(got, "status-completed") {
		t.Errorf("expected status-completed for ALIGN, got: %s", got)
	}
	if !strings.Contains(got, "status-running") {
		t.Errorf("expected status-running for SORT, got: %s", got)
	}
	if !strings.Contains(got, "2/2") {
		t.Errorf("expected 2/2 for ALIGN, got: %s", got)
	}
	if !strings.Contains(got, "0/1") {
		t.Errorf("expected 0/1 for SORT, got: %s", got)
	}
}

func TestRenderDAG_SVGDimensions(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// Container: width = 1*200 + 80 - 20 = 260, height = 1*100 + 80 - 36 = 144
	// SVG should match container dimensions (only rendered when edges exist)
	if strings.Contains(got, "<svg") {
		if !strings.Contains(got, fmt.Sprintf(`width="%d"`, 260)) {
			t.Errorf("expected SVG width=260, got: %s", got)
		}
		if !strings.Contains(got, fmt.Sprintf(`height="%d"`, 144)) {
			t.Errorf("expected SVG height=144, got: %s", got)
		}
	}
}

// --- New tests for scroll container, arrowheads, interactive nodes/edges ---

func TestRenderDAG_ScrollContainer(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// Outer div should be just id="dag-view" with no inline style
	if !strings.Contains(got, `<div id="dag-view">`) {
		t.Errorf("expected outer div with just id=dag-view (no inline style), got: %s", got)
	}
	// Scroll container wrapper should be present
	if !strings.Contains(got, `class="dag-scroll-container"`) {
		t.Errorf("expected dag-scroll-container wrapper, got: %s", got)
	}
	// The position:relative style should still exist (on inner div)
	if !strings.Contains(got, `position:relative`) {
		t.Errorf("expected position:relative on inner div, got: %s", got)
	}
}

func TestRenderDAG_ArrowheadMarker(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
			{Name: "B", Layer: 1, Index: 0},
		},
		Edges:      []dag.Edge{{From: "A", To: "B"}},
		LayerCount: 2,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// SVG should contain arrowhead marker definition
	if !strings.Contains(got, `<marker id="arrowhead"`) {
		t.Errorf("expected arrowhead marker definition in SVG, got: %s", got)
	}
	if !strings.Contains(got, `orient="auto-start-reverse"`) {
		t.Errorf("expected orient=auto-start-reverse on marker, got: %s", got)
	}
	// Each path should reference the arrowhead marker
	if !strings.Contains(got, `marker-end="url(#arrowhead)"`) {
		t.Errorf("expected marker-end on path, got: %s", got)
	}
}

func TestRenderDAG_ArrowheadMarker_NoEdges(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// No edges means no SVG at all, so no arrowhead marker
	if strings.Contains(got, `<marker`) {
		t.Errorf("expected no marker when no edges, got: %s", got)
	}
}

func TestRenderDAG_InteractiveNodeMouseenter(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
			{Name: "SORT", Layer: 1, Index: 0},
		},
		Edges:      []dag.Edge{{From: "ALIGN", To: "SORT"}},
		LayerCount: 2,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	if !strings.Contains(got, `data-on:mouseenter="$_dagHL = 'ALIGN'"`) {
		t.Errorf("expected mouseenter handler for ALIGN, got: %s", got)
	}
	if !strings.Contains(got, `data-on:mouseenter="$_dagHL = 'SORT'"`) {
		t.Errorf("expected mouseenter handler for SORT, got: %s", got)
	}
}

func TestRenderDAG_InteractiveNodeMouseleave(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	if !strings.Contains(got, `data-on:mouseleave="$_dagHL = ''"`) {
		t.Errorf("expected mouseleave handler, got: %s", got)
	}
}

func TestRenderDAG_InteractiveNodeClick(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	expected := `data-on:click="$expandedGroup = $expandedGroup === 'ALIGN' ? '' : 'ALIGN'; $expandedGroup === 'ALIGN' && @get('/tasks//ALIGN'); setTimeout(()=>document.getElementById('process-group-ALIGN')?.scrollIntoView({behavior:'smooth',block:'nearest'}),50)"`
	if !strings.Contains(got, expected) {
		t.Errorf("expected click toggle handler for ALIGN, got: %s", got)
	}
}

func TestRenderDAG_NodeSelectedAttribute(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "PROC", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	if !strings.Contains(got, `data-class:dag-node-selected="$expandedGroup === 'PROC'"`) {
		t.Errorf("expected dag-node-selected class binding, got: %s", got)
	}
}

func TestRenderDAG_DagFadedNeighborList(t *testing.T) {
	// Diamond: A→B, A→C, B→D, C→D
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
			{Name: "B", Layer: 1, Index: 0},
			{Name: "C", Layer: 1, Index: 1},
			{Name: "D", Layer: 2, Index: 0},
		},
		Edges: []dag.Edge{
			{From: "A", To: "B"},
			{From: "A", To: "C"},
			{From: "B", To: "D"},
			{From: "C", To: "D"},
		},
		LayerCount: 3,
		MaxWidth:   2,
	}
	got := renderDAG(layout, nil)

	// A's neighbors: [B, C] (from A→B and A→C)
	if !strings.Contains(got, `dagShouldFade($_dagHL, 'A', ['B','C'])`) {
		t.Errorf("expected A's neighbor list ['B','C'], got: %s", got)
	}
	// B's neighbors: [A, D] (from A→B and B→D)
	if !strings.Contains(got, `dagShouldFade($_dagHL, 'B', ['A','D'])`) {
		t.Errorf("expected B's neighbor list ['A','D'], got: %s", got)
	}
	// D's neighbors: [B, C] (from B→D and C→D)
	if !strings.Contains(got, `dagShouldFade($_dagHL, 'D', ['B','C'])`) {
		t.Errorf("expected D's neighbor list ['B','C'], got: %s", got)
	}
}

func TestRenderDAG_DagFadedNoNeighbors(t *testing.T) {
	// Single node with no edges
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "SOLO", Layer: 0, Index: 0},
		},
		Edges:      []dag.Edge{},
		LayerCount: 1,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// Node with no neighbors should have empty array
	if !strings.Contains(got, `dagShouldFade($_dagHL, 'SOLO', [])`) {
		t.Errorf("expected empty neighbor list for isolated node, got: %s", got)
	}
}

func TestRenderDAG_InteractiveEdgeAttributes(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "ALIGN", Layer: 0, Index: 0},
			{Name: "SORT", Layer: 1, Index: 0},
		},
		Edges:      []dag.Edge{{From: "ALIGN", To: "SORT"}},
		LayerCount: 2,
		MaxWidth:   1,
	}
	got := renderDAG(layout, nil)

	// Edge mouseenter with FROM>TO format
	if !strings.Contains(got, `data-on:mouseenter="$_dagHL = 'ALIGN>SORT'"`) {
		t.Errorf("expected edge mouseenter handler, got: %s", got)
	}
	// Edge mouseleave
	if !strings.Contains(got, `data-on:mouseleave="$_dagHL = ''"`) {
		t.Errorf("expected edge mouseleave handler, got: %s", got)
	}
	// Edge fade binding
	if !strings.Contains(got, `data-class:dag-edge-faded="dagEdgeFade($_dagHL, 'ALIGN', 'SORT')"`) {
		t.Errorf("expected dag-edge-faded binding, got: %s", got)
	}
	// Pointer events style
	if !strings.Contains(got, `pointer-events:visibleStroke`) {
		t.Errorf("expected pointer-events:visibleStroke on edge, got: %s", got)
	}
}

func TestRenderDAG_InteractiveEdgeMultiple(t *testing.T) {
	layout := &dag.Layout{
		Nodes: []dag.NodeLayout{
			{Name: "A", Layer: 0, Index: 0},
			{Name: "B", Layer: 0, Index: 1},
			{Name: "C", Layer: 1, Index: 0},
		},
		Edges: []dag.Edge{
			{From: "A", To: "C"},
			{From: "B", To: "C"},
		},
		LayerCount: 2,
		MaxWidth:   2,
	}
	got := renderDAG(layout, nil)

	// Each edge should have its own unique FROM>TO identifier
	if !strings.Contains(got, `$_dagHL = 'A>C'`) {
		t.Errorf("expected edge identifier A>C, got: %s", got)
	}
	if !strings.Contains(got, `$_dagHL = 'B>C'`) {
		t.Errorf("expected edge identifier B>C, got: %s", got)
	}
	// Each edge should have its own dagEdgeFade binding
	if !strings.Contains(got, `dagEdgeFade($_dagHL, 'A', 'C')`) {
		t.Errorf("expected dagEdgeFade for A→C, got: %s", got)
	}
	if !strings.Contains(got, `dagEdgeFade($_dagHL, 'B', 'C')`) {
		t.Errorf("expected dagEdgeFade for B→C, got: %s", got)
	}
}
