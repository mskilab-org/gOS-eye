package server

import (
	"reflect"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// ---- tokenizeCommandLine tests ----

func TestTokenizeCommandLine_Empty(t *testing.T) {
	got := tokenizeCommandLine("")
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestTokenizeCommandLine_SimpleTokens(t *testing.T) {
	got := tokenizeCommandLine("a b c")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeCommandLine_SingleQuoted(t *testing.T) {
	got := tokenizeCommandLine("a 'b c' d")
	want := []string{"a", "b c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeCommandLine_DoubleQuoted(t *testing.T) {
	got := tokenizeCommandLine(`a "b c" d`)
	want := []string{"a", "b c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeCommandLine_EscapedSpace(t *testing.T) {
	got := tokenizeCommandLine(`a b\ c d`)
	want := []string{"a", "b c", "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeCommandLine_MixedQuotes(t *testing.T) {
	got := tokenizeCommandLine(`a 'b "c"' d`)
	want := []string{"a", `b "c"`, "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeCommandLine_MultipleSpaces(t *testing.T) {
	got := tokenizeCommandLine("a   b   c")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeCommandLine_EmptyStringValue(t *testing.T) {
	got := tokenizeCommandLine("--key ''")
	want := []string{"--key", ""}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeCommandLine_DoubleQuoteEscapes(t *testing.T) {
	got := tokenizeCommandLine(`a "b\"c" d`)
	want := []string{"a", `b"c`, "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeCommandLine_BackslashInDoubleQuotes(t *testing.T) {
	got := tokenizeCommandLine(`a "b\\c" d`)
	want := []string{"a", `b\c`, "d"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestTokenizeCommandLine_WhitespaceOnly(t *testing.T) {
	got := tokenizeCommandLine("   ")
	if len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestTokenizeCommandLine_SingleToken(t *testing.T) {
	got := tokenizeCommandLine("hello")
	want := []string{"hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// ---- parseRuntimeFlags tests ----

func TestParseRuntimeFlags_FullCommand(t *testing.T) {
	tokens := []string{
		"nextflow", "run", "nf-core/rnaseq",
		"--input", "s.csv",
		"-profile", "docker",
		"-with-weblog", "http://localhost:8080/webhook",
	}
	params := map[string]any{"input": "s.csv"}
	got := parseRuntimeFlags(tokens, params)
	want := []string{"-profile", "docker", "-with-weblog", "http://localhost:8080/webhook"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseRuntimeFlags_StripsResumeWithValue(t *testing.T) {
	tokens := []string{"nextflow", "run", "proj", "-resume", "abc123"}
	got := parseRuntimeFlags(tokens, nil)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestParseRuntimeFlags_StripsResumeFollowedByFlag(t *testing.T) {
	tokens := []string{"nextflow", "run", "proj", "-resume", "-profile", "docker"}
	got := parseRuntimeFlags(tokens, nil)
	want := []string{"-profile", "docker"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseRuntimeFlags_ShortCommand(t *testing.T) {
	tokens := []string{"nextflow", "run"}
	got := parseRuntimeFlags(tokens, nil)
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

func TestParseRuntimeFlags_BooleanRuntimeFlag(t *testing.T) {
	tokens := []string{"nextflow", "run", "proj", "-stub-run"}
	got := parseRuntimeFlags(tokens, nil)
	want := []string{"-stub-run"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseRuntimeFlags_NilParams(t *testing.T) {
	tokens := []string{"nextflow", "run", "proj", "--input", "s.csv", "-profile", "docker"}
	got := parseRuntimeFlags(tokens, nil)
	want := []string{"-profile", "docker"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseRuntimeFlags_OnlyParams(t *testing.T) {
	tokens := []string{"nextflow", "run", "proj", "--input", "s.csv", "--outdir", "/results"}
	params := map[string]any{"input": "s.csv", "outdir": "/results"}
	got := parseRuntimeFlags(tokens, params)
	if len(got) != 0 {
		t.Fatalf("expected empty, got %v", got)
	}
}

func TestParseRuntimeFlags_ResumeAtEnd(t *testing.T) {
	tokens := []string{"nextflow", "run", "proj", "-profile", "docker", "-resume"}
	got := parseRuntimeFlags(tokens, nil)
	want := []string{"-profile", "docker"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestParseRuntimeFlags_MultipleRuntimeFlags(t *testing.T) {
	tokens := []string{"nextflow", "run", "proj", "-profile", "docker", "-c", "custom.config", "-with-trace"}
	got := parseRuntimeFlags(tokens, nil)
	want := []string{"-profile", "docker", "-c", "custom.config", "-with-trace"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// ---- buildResumeCommand tests ----

func TestBuildResumeCommand_EmptySessionID(t *testing.T) {
	run := &state.Run{
		ProjectName: "nf-core/rnaseq",
		SessionID:   "",
		WorkDir:     "/work",
		Params:      map[string]any{"input": "samples.csv"},
	}
	got := buildResumeCommand(run)
	if got != "" {
		t.Fatalf("expected empty string for empty SessionID, got %q", got)
	}
}

func TestBuildResumeCommand_EmptyProjectName(t *testing.T) {
	run := &state.Run{
		ProjectName: "",
		SessionID:   "abc-123",
		WorkDir:     "/work",
		Params:      map[string]any{"input": "samples.csv"},
	}
	got := buildResumeCommand(run)
	if got != "" {
		t.Fatalf("expected empty string for empty ProjectName, got %q", got)
	}
}

func TestBuildResumeCommand_FullReconstruction(t *testing.T) {
	run := &state.Run{
		ProjectName: "nf-core/rnaseq",
		SessionID:   "uuid-1",
		CommandLine: "nextflow run nf-core/rnaseq --input s.csv --outdir /results -profile docker -with-weblog http://localhost:8080/webhook",
		Params:      map[string]any{"input": "s.csv", "outdir": "/results"},
	}
	got := buildResumeCommand(run)
	want := "nextflow run nf-core/rnaseq --input s.csv --outdir /results -profile docker -with-weblog http://localhost:8080/webhook -resume uuid-1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_WithResumeInOriginal(t *testing.T) {
	run := &state.Run{
		ProjectName: "proj",
		SessionID:   "new-id",
		CommandLine: "nextflow run proj --input s.csv -resume old-id",
		Params:      map[string]any{"input": "s.csv"},
	}
	got := buildResumeCommand(run)
	want := "nextflow run proj --input s.csv -resume new-id"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_NoCommandLineFallback(t *testing.T) {
	run := &state.Run{
		ProjectName: "nextflow-io/hello",
		SessionID:   "sess-001",
		WorkDir:     "/data/work",
		CommandLine: "",
	}
	got := buildResumeCommand(run)
	want := "nextflow run nextflow-io/hello -work-dir /data/work -resume sess-001"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_NoCommandLineWithParams(t *testing.T) {
	run := &state.Run{
		ProjectName: "nf-core/rnaseq",
		SessionID:   "uuid-abc",
		WorkDir:     "/scratch/work",
		CommandLine: "",
		Params:      map[string]any{"input": "samplesheet.csv"},
	}
	got := buildResumeCommand(run)
	want := "nextflow run nf-core/rnaseq --input samplesheet.csv -work-dir /scratch/work -resume uuid-abc"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_NoCommandLineMultipleParamsSorted(t *testing.T) {
	run := &state.Run{
		ProjectName: "nf-core/rnaseq",
		SessionID:   "uuid-abc",
		WorkDir:     "/work",
		CommandLine: "",
		Params: map[string]any{
			"outdir":  "/results",
			"input":   "samples.csv",
			"aligner": "star_salmon",
		},
	}
	got := buildResumeCommand(run)
	want := "nextflow run nf-core/rnaseq --aligner star_salmon --input samples.csv --outdir /results -work-dir /work -resume uuid-abc"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_FallbackWorkDirWithSpaces(t *testing.T) {
	run := &state.Run{
		ProjectName: "pipeline",
		SessionID:   "s1",
		WorkDir:     "/my work dir",
		CommandLine: "",
	}
	got := buildResumeCommand(run)
	want := "nextflow run pipeline -work-dir '/my work dir' -resume s1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_ParamsWithSpecialChars(t *testing.T) {
	run := &state.Run{
		ProjectName: "pipeline",
		SessionID:   "s1",
		CommandLine: "",
		WorkDir:     "/w",
		Params:      map[string]any{"msg": "hello world"},
	}
	got := buildResumeCommand(run)
	want := "nextflow run pipeline --msg 'hello world' -work-dir /w -resume s1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_RuntimeFlagValueWithSpaces(t *testing.T) {
	run := &state.Run{
		ProjectName: "proj",
		SessionID:   "s1",
		CommandLine: "nextflow run proj -c 'my config.conf'",
		Params:      map[string]any{},
	}
	got := buildResumeCommand(run)
	want := "nextflow run proj -c 'my config.conf' -resume s1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_RuntimeFlagsOnly(t *testing.T) {
	run := &state.Run{
		ProjectName: "proj",
		SessionID:   "s1",
		CommandLine: "nextflow run proj -profile docker -with-trace",
		Params:      map[string]any{},
	}
	got := buildResumeCommand(run)
	want := "nextflow run proj -profile docker -with-trace -resume s1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_CommandLineNoParams(t *testing.T) {
	run := &state.Run{
		ProjectName: "nextflow-io/hello",
		SessionID:   "sess-001",
		CommandLine: "nextflow run nextflow-io/hello -work-dir /data/work",
		Params:      map[string]any{},
	}
	got := buildResumeCommand(run)
	want := "nextflow run nextflow-io/hello -work-dir /data/work -resume sess-001"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_EmptyParamsMap(t *testing.T) {
	run := &state.Run{
		ProjectName: "nextflow-io/hello",
		SessionID:   "sess-001",
		WorkDir:     "/data/work",
		CommandLine: "",
		Params:      map[string]any{},
	}
	got := buildResumeCommand(run)
	want := "nextflow run nextflow-io/hello -work-dir /data/work -resume sess-001"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildResumeCommand_ProjectNameQuoted(t *testing.T) {
	run := &state.Run{
		ProjectName: "my project",
		SessionID:   "s1",
		CommandLine: "",
		WorkDir:     "/w",
	}
	got := buildResumeCommand(run)
	want := "nextflow run 'my project' -work-dir /w -resume s1"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
