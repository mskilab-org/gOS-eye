package dag

import (
	"os"
	"sort"
	"strings"
	"testing"
)

func TestParseDOT_Fixture(t *testing.T) {
	f, err := os.Open("../../tests/mock-pipeline/dag.dot")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer f.Close()

	dag, err := ParseDOT(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// --- 10 processes ---
	if len(dag.Processes) != 10 {
		t.Fatalf("expected 10 processes, got %d: %v", len(dag.Processes), dag.Processes)
	}

	expectedProcesses := map[string]bool{
		"BWA_MEM": true, "FRAGCOUNTER": true, "GRIDSS": true, "SAGE": true,
		"AMBER": true, "DRYCLEAN": true, "CBS": true, "PURPLE": true,
		"JABBA": true, "EVENTS_FUSIONS": true,
	}
	for _, p := range dag.Processes {
		if !expectedProcesses[p] {
			t.Errorf("unexpected process: %s", p)
		}
	}
	for p := range expectedProcesses {
		found := false
		for _, got := range dag.Processes {
			if got == p {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing process: %s", p)
		}
	}

	// --- Topological order: every edge From appears before To ---
	indexOf := map[string]int{}
	for i, p := range dag.Processes {
		indexOf[p] = i
	}
	for _, e := range dag.Edges {
		if indexOf[e.From] >= indexOf[e.To] {
			t.Errorf("topological order violated: %s (idx %d) -> %s (idx %d)",
				e.From, indexOf[e.From], e.To, indexOf[e.To])
		}
	}

	// --- 16 deduplicated edges ---
	if len(dag.Edges) != 16 {
		t.Fatalf("expected 16 edges, got %d: %v", len(dag.Edges), dag.Edges)
	}

	expectedEdges := []Edge{
		{From: "BWA_MEM", To: "FRAGCOUNTER"},
		{From: "BWA_MEM", To: "GRIDSS"},
		{From: "BWA_MEM", To: "SAGE"},
		{From: "BWA_MEM", To: "AMBER"},
		{From: "FRAGCOUNTER", To: "DRYCLEAN"},
		{From: "DRYCLEAN", To: "CBS"},
		{From: "DRYCLEAN", To: "PURPLE"},
		{From: "DRYCLEAN", To: "JABBA"},
		{From: "GRIDSS", To: "PURPLE"},
		{From: "GRIDSS", To: "JABBA"},
		{From: "SAGE", To: "PURPLE"},
		{From: "SAGE", To: "JABBA"},
		{From: "AMBER", To: "PURPLE"},
		{From: "CBS", To: "JABBA"},
		{From: "PURPLE", To: "JABBA"},
		{From: "JABBA", To: "EVENTS_FUSIONS"},
	}
	edgesMatch(t, dag.Edges, expectedEdges)

	// --- Children map populated ---
	if dag.Children == nil {
		t.Fatal("Children map is nil")
	}
	bwaChildren := dag.Children["BWA_MEM"]
	sort.Strings(bwaChildren)
	wantBwaChildren := []string{"AMBER", "FRAGCOUNTER", "GRIDSS", "SAGE"}
	if len(bwaChildren) != len(wantBwaChildren) {
		t.Fatalf("BWA_MEM children: expected %v, got %v", wantBwaChildren, bwaChildren)
	}
	for i, c := range wantBwaChildren {
		if bwaChildren[i] != c {
			t.Errorf("BWA_MEM children[%d]: expected %s, got %s", i, c, bwaChildren[i])
		}
	}

	// EVENTS_FUSIONS is a terminal node — no children
	efChildren := dag.Children["EVENTS_FUSIONS"]
	if len(efChildren) != 0 {
		t.Errorf("EVENTS_FUSIONS should have 0 children, got %v", efChildren)
	}

	// --- Parents map populated ---
	if dag.Parents == nil {
		t.Fatal("Parents map is nil")
	}
	jabbaParents := dag.Parents["JABBA"]
	sort.Strings(jabbaParents)
	wantJabbaParents := []string{"CBS", "DRYCLEAN", "GRIDSS", "PURPLE", "SAGE"}
	if len(jabbaParents) != len(wantJabbaParents) {
		t.Fatalf("JABBA parents: expected %v, got %v", wantJabbaParents, jabbaParents)
	}
	for i, p := range wantJabbaParents {
		if jabbaParents[i] != p {
			t.Errorf("JABBA parents[%d]: expected %s, got %s", i, p, jabbaParents[i])
		}
	}

	// BWA_MEM is a root — no parents
	bwaParents := dag.Parents["BWA_MEM"]
	if len(bwaParents) != 0 {
		t.Errorf("BWA_MEM should have 0 parents, got %v", bwaParents)
	}
}

func TestParseDOT_Empty(t *testing.T) {
	dag, err := ParseDOT(strings.NewReader(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dag.Processes) != 0 {
		t.Fatalf("expected 0 processes, got %d", len(dag.Processes))
	}
	if len(dag.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(dag.Edges))
	}
	if dag.Children == nil {
		t.Fatal("Children map should not be nil")
	}
	if dag.Parents == nil {
		t.Fatal("Parents map should not be nil")
	}
}

func TestParseDOT_OnlyOperators(t *testing.T) {
	// DOT with only operator nodes (no process nodes)
	input := `digraph "dag" {
v0 [shape=point,label="",fixedsize=true,width=0.1,xlabel="Channel.fromPath"];
v1 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="splitCsv"];
v2 [shape=circle,label="",fixedsize=true,width=0.1,xlabel="map"];
v0 -> v1;
v1 -> v2;
}`
	dag, err := ParseDOT(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(dag.Processes) != 0 {
		t.Fatalf("expected 0 processes, got %d: %v", len(dag.Processes), dag.Processes)
	}
	if len(dag.Edges) != 0 {
		t.Fatalf("expected 0 edges, got %d", len(dag.Edges))
	}
	if dag.Children == nil {
		t.Fatal("Children map should not be nil")
	}
	if dag.Parents == nil {
		t.Fatal("Parents map should not be nil")
	}
}
