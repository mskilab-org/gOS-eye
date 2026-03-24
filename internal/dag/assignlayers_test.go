package dag

import (
	"reflect"
	"testing"
)

func TestAssignLayers_Empty(t *testing.T) {
	dag := &DAG{
		Processes: []string{},
		Parents:   map[string][]string{},
	}
	got := assignLayers(dag)
	if len(got) != 0 {
		t.Fatalf("expected empty map, got %v", got)
	}
}

func TestAssignLayers_SingleRoot(t *testing.T) {
	dag := &DAG{
		Processes: []string{"A"},
		Parents:   map[string][]string{},
	}
	got := assignLayers(dag)
	want := map[string]int{"A": 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAssignLayers_MultipleRoots(t *testing.T) {
	dag := &DAG{
		Processes: []string{"A", "B", "C"},
		Parents:   map[string][]string{},
	}
	got := assignLayers(dag)
	want := map[string]int{"A": 0, "B": 0, "C": 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAssignLayers_LinearChain(t *testing.T) {
	// A → B → C → D
	dag := &DAG{
		Processes: []string{"A", "B", "C", "D"},
		Parents: map[string][]string{
			"B": {"A"},
			"C": {"B"},
			"D": {"C"},
		},
	}
	got := assignLayers(dag)
	want := map[string]int{"A": 0, "B": 1, "C": 2, "D": 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAssignLayers_Diamond(t *testing.T) {
	//   A
	//  / \
	// B   C
	//  \ /
	//   D
	dag := &DAG{
		Processes: []string{"A", "B", "C", "D"},
		Parents: map[string][]string{
			"B": {"A"},
			"C": {"A"},
			"D": {"B", "C"},
		},
	}
	got := assignLayers(dag)
	want := map[string]int{"A": 0, "B": 1, "C": 1, "D": 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAssignLayers_UnevenParents(t *testing.T) {
	// A → B → C
	// A ------→ C
	// C should be layer 2 (max(B=1) + 1), not 1
	dag := &DAG{
		Processes: []string{"A", "B", "C"},
		Parents: map[string][]string{
			"B": {"A"},
			"C": {"A", "B"},
		},
	}
	got := assignLayers(dag)
	want := map[string]int{"A": 0, "B": 1, "C": 2}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAssignLayers_MockPipeline(t *testing.T) {
	// The expected mock pipeline DAG from the task description:
	// BWA_MEM → FRAGCOUNTER, GRIDSS, SAGE, AMBER
	// FRAGCOUNTER → DRYCLEAN
	// DRYCLEAN → CBS
	// SAGE, AMBER → PURPLE
	// CBS, PURPLE → JABBA
	// JABBA → EVENTS_FUSIONS
	dag := &DAG{
		Processes: []string{
			"BWA_MEM", "FRAGCOUNTER", "GRIDSS", "SAGE", "AMBER",
			"DRYCLEAN", "CBS", "PURPLE", "JABBA", "EVENTS_FUSIONS",
		},
		Parents: map[string][]string{
			"FRAGCOUNTER":    {"BWA_MEM"},
			"GRIDSS":         {"BWA_MEM"},
			"SAGE":           {"BWA_MEM"},
			"AMBER":          {"BWA_MEM"},
			"DRYCLEAN":       {"FRAGCOUNTER"},
			"CBS":            {"DRYCLEAN"},
			"PURPLE":         {"SAGE", "AMBER"},
			"JABBA":          {"CBS", "PURPLE"},
			"EVENTS_FUSIONS": {"JABBA"},
		},
	}
	got := assignLayers(dag)
	want := map[string]int{
		"BWA_MEM":        0,
		"FRAGCOUNTER":    1,
		"GRIDSS":         1,
		"SAGE":           1,
		"AMBER":          1,
		"DRYCLEAN":       2,
		"CBS":            3,
		"PURPLE":         2,
		"JABBA":          4,
		"EVENTS_FUSIONS": 5,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAssignLayers_NilParentsMap(t *testing.T) {
	// Parents map is nil — all processes are roots
	dag := &DAG{
		Processes: []string{"X", "Y"},
		Parents:   nil,
	}
	got := assignLayers(dag)
	want := map[string]int{"X": 0, "Y": 0}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
