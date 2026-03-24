package dag

import (
	"strings"
	"testing"
)

func TestParseDotFile_Empty(t *testing.T) {
	nodes, edges, err := parseDotFile(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(edges))
	}
}

func TestParseDotFile_HeaderFooterOnly(t *testing.T) {
	input := `digraph "dag" {
}
`
	nodes, edges, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(edges))
	}
}

func TestParseDotFile_SingleProcessNode(t *testing.T) {
	input := `digraph "dag" {
v7 [label="BWA_MEM"];
}
`
	nodes, edges, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != "v7" {
		t.Fatalf("expected ID v7, got %q", nodes[0].ID)
	}
	if nodes[0].Label != "BWA_MEM" {
		t.Fatalf("expected label BWA_MEM, got %q", nodes[0].Label)
	}
	if nodes[0].Shape != "" {
		t.Fatalf("expected empty shape, got %q", nodes[0].Shape)
	}
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(edges))
	}
}

func TestParseDotFile_CircleNode(t *testing.T) {
	input := `v1 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="splitCsv"];`
	nodes, edges, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != "v1" {
		t.Fatalf("expected ID v1, got %q", nodes[0].ID)
	}
	if nodes[0].Label != "" {
		t.Fatalf("expected empty label, got %q", nodes[0].Label)
	}
	if nodes[0].Shape != "circle" {
		t.Fatalf("expected shape circle, got %q", nodes[0].Shape)
	}
	if len(edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(edges))
	}
}

func TestParseDotFile_PointNode(t *testing.T) {
	input := `v18 [shape=point];`
	nodes, _, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].ID != "v18" {
		t.Fatalf("expected ID v18, got %q", nodes[0].ID)
	}
	if nodes[0].Label != "" {
		t.Fatalf("expected empty label, got %q", nodes[0].Label)
	}
	if nodes[0].Shape != "point" {
		t.Fatalf("expected shape point, got %q", nodes[0].Shape)
	}
}

func TestParseDotFile_SingleEdge(t *testing.T) {
	input := `v7 -> v8;`
	nodes, edges, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 0 {
		t.Fatalf("expected 0 nodes, got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].From != "v7" || edges[0].To != "v8" {
		t.Fatalf("expected edge v7->v8, got %s->%s", edges[0].From, edges[0].To)
	}
}

func TestParseDotFile_EdgeWithLabel(t *testing.T) {
	input := `v5 -> v6 [label="normal_fq"];`
	_, edges, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
	if edges[0].From != "v5" || edges[0].To != "v6" {
		t.Fatalf("expected edge v5->v6, got %s->%s", edges[0].From, edges[0].To)
	}
}

func TestParseDotFile_DeduplicateNodes(t *testing.T) {
	// Same node ID declared twice — keep last declaration
	input := `v3 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v3 [label="UPDATED"];
`
	nodes, _, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node after dedup, got %d", len(nodes))
	}
	if nodes[0].Label != "UPDATED" {
		t.Fatalf("expected label UPDATED (last declaration), got %q", nodes[0].Label)
	}
	if nodes[0].Shape != "" {
		t.Fatalf("expected empty shape (last declaration has no shape), got %q", nodes[0].Shape)
	}
}

func TestParseDotFile_MixedNodesAndEdges(t *testing.T) {
	input := `digraph "dag" {
v0 [shape=point,label="",fixedsize=true,width=0.1,xlabel="Channel.fromPath"];
v7 [label="BWA_MEM"];
v0 -> v7;
v7 -> v8;
v8 [label="FRAGCOUNTER"];
}
`
	nodes, edges, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
}

func TestParseDotFile_PointNodeMinimal(t *testing.T) {
	// shape=point with no label attr at all
	input := `v18 [shape=point];`
	nodes, _, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Shape != "point" {
		t.Fatalf("expected shape point, got %q", nodes[0].Shape)
	}
	if nodes[0].Label != "" {
		t.Fatalf("expected empty label, got %q", nodes[0].Label)
	}
}

func TestParseDotFile_FullFixture(t *testing.T) {
	// Use the actual fixture from tests/mock-pipeline/dag.dot
	// Expected: 33 unique nodes (v0..v32), 42 edges
	input := `digraph "dag" {
v0 [shape=point,label="",fixedsize=true,width=0.1,xlabel="Channel.fromPath"];
v1 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="splitCsv"];
v0 -> v1;

v1 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="splitCsv"];
v2 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v1 -> v2;

v2 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v3 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v2 -> v3;

v3 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v5 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v3 -> v5;

v3 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v4 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="multiMap"];
v3 -> v4;

v4 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="multiMap"];
v6 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="mix"];
v4 -> v6;

v4 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="multiMap"];
v24 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v4 -> v24;

v5 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v6 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="mix"];
v5 -> v6 [label="normal_fq"];

v6 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="mix"];
v7 [label="BWA_MEM"];
v6 -> v7 [label="all_fq"];

v7 [label="BWA_MEM"];
v8 [label="FRAGCOUNTER"];
v7 -> v8;

v8 [label="FRAGCOUNTER"];
v9 [label="DRYCLEAN"];
v8 -> v9;

v9 [label="DRYCLEAN"];
v17 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v9 -> v17;

v7 [label="BWA_MEM"];
v10 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v7 -> v10;

v10 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v12 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v10 -> v12;

v10 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v11 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v10 -> v11;

v11 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v13 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v11 -> v13 [label="tumor_bam_keyed"];

v12 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v13 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v12 -> v13 [label="normal_bam_keyed"];

v13 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v14 [label="GRIDSS"];
v13 -> v14 [label="paired_bams"];

v14 [label="GRIDSS"];
v21 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v14 -> v21;

v13 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v15 [label="SAGE"];
v13 -> v15 [label="paired_bams"];

v15 [label="SAGE"];
v22 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v15 -> v22;

v13 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v16 [label="AMBER"];
v13 -> v16 [label="paired_bams"];

v16 [label="AMBER"];
v23 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v16 -> v23;

v17 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v19 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v17 -> v19;

v17 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="branch"];
v18 [shape=point];
v17 -> v18;

v19 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v20 [label="CBS"];
v19 -> v20 [label="tumor_cov_keyed"];

v20 [label="CBS"];
v26 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v20 -> v26;

v19 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v21 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v19 -> v21 [label="tumor_cov_keyed"];

v21 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v22 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v21 -> v22;

v22 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v23 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v22 -> v23;

v23 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v24 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v23 -> v24;

v24 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v25 [label="PURPLE"];
v24 -> v25 [label="purple_in"];

v25 [label="PURPLE"];
v26 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v25 -> v26;

v26 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v27 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v26 -> v27;

v14 [label="GRIDSS"];
v27 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v14 -> v27;

v27 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v28 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v27 -> v28;

v19 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v28 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v19 -> v28 [label="tumor_cov_keyed"];

v28 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v29 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v28 -> v29;

v15 [label="SAGE"];
v29 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v15 -> v29;

v29 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="join"];
v30 [label="JABBA"];
v29 -> v30 [label="jabba_in"];

v30 [label="JABBA"];
v31 [label="EVENTS_FUSIONS"];
v30 -> v31;

v31 [label="EVENTS_FUSIONS"];
v32 [shape=point];
v31 -> v32;

}
`
	nodes, edges, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 33 {
		t.Fatalf("expected 33 unique nodes, got %d", len(nodes))
	}
	if len(edges) != 42 {
		t.Fatalf("expected 42 edges, got %d", len(edges))
	}

	// Build a map for easy lookup
	nodeMap := make(map[string]dotNode)
	for _, n := range nodes {
		nodeMap[n.ID] = n
	}

	// Check all IDs v0..v32 are present
	for i := 0; i <= 32; i++ {
		id := "v" + string(rune('0'+i/10)) + string(rune('0'+i%10))
		if i < 10 {
			id = "v" + string(rune('0'+i))
		}
		if _, ok := nodeMap[id]; !ok {
			t.Errorf("missing node %s", id)
		}
	}

	// Spot-check process nodes
	if n := nodeMap["v7"]; n.Label != "BWA_MEM" || n.Shape != "" {
		t.Errorf("v7: got label=%q shape=%q, want BWA_MEM/empty", n.Label, n.Shape)
	}
	if n := nodeMap["v14"]; n.Label != "GRIDSS" || n.Shape != "" {
		t.Errorf("v14: got label=%q shape=%q, want GRIDSS/empty", n.Label, n.Shape)
	}
	if n := nodeMap["v31"]; n.Label != "EVENTS_FUSIONS" || n.Shape != "" {
		t.Errorf("v31: got label=%q shape=%q, want EVENTS_FUSIONS/empty", n.Label, n.Shape)
	}

	// Spot-check operator nodes
	if n := nodeMap["v0"]; n.Shape != "point" || n.Label != "" {
		t.Errorf("v0: got label=%q shape=%q, want empty/point", n.Label, n.Shape)
	}
	if n := nodeMap["v1"]; n.Shape != "circle" || n.Label != "" {
		t.Errorf("v1: got label=%q shape=%q, want empty/circle", n.Label, n.Shape)
	}
	if n := nodeMap["v18"]; n.Shape != "point" || n.Label != "" {
		t.Errorf("v18: got label=%q shape=%q, want empty/point", n.Label, n.Shape)
	}
	if n := nodeMap["v32"]; n.Shape != "point" || n.Label != "" {
		t.Errorf("v32: got label=%q shape=%q, want empty/point", n.Label, n.Shape)
	}

	// Spot-check some edges
	foundV7toV8 := false
	foundV6toV7 := false
	foundV31toV32 := false
	for _, e := range edges {
		if e.From == "v7" && e.To == "v8" {
			foundV7toV8 = true
		}
		if e.From == "v6" && e.To == "v7" {
			foundV6toV7 = true
		}
		if e.From == "v31" && e.To == "v32" {
			foundV31toV32 = true
		}
	}
	if !foundV7toV8 {
		t.Error("missing edge v7->v8")
	}
	if !foundV6toV7 {
		t.Error("missing edge v6->v7")
	}
	if !foundV31toV32 {
		t.Error("missing edge v31->v32")
	}
}

func TestParseDotFile_NodeWithLabelBeforeShape(t *testing.T) {
	// label appears before shape in the attribute list
	input := `v5 [label="FOO",shape=circle];`
	nodes, _, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 1 {
		t.Fatalf("expected 1 node, got %d", len(nodes))
	}
	if nodes[0].Label != "FOO" {
		t.Fatalf("expected label FOO, got %q", nodes[0].Label)
	}
	if nodes[0].Shape != "circle" {
		t.Fatalf("expected shape circle, got %q", nodes[0].Shape)
	}
}

func TestParseDotFile_IgnoresBlankLines(t *testing.T) {
	input := `

v1 [label="A"];

v2 [label="B"];

v1 -> v2;

`
	nodes, edges, err := parseDotFile(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodes))
	}
	if len(edges) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(edges))
	}
}
