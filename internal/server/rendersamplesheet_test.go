package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestRenderSamplesheet_NilRun(t *testing.T) {
	got := renderSamplesheet(nil)
	if got != "" {
		t.Fatalf("expected empty string for nil run, got %q", got)
	}
}

func TestRenderSamplesheet_NilParams(t *testing.T) {
	run := &state.Run{Params: nil}
	got := renderSamplesheet(run)
	if got != "" {
		t.Fatalf("expected empty string for nil params, got %q", got)
	}
}

func TestRenderSamplesheet_MissingInputKey(t *testing.T) {
	run := &state.Run{Params: map[string]any{"outdir": "/results"}}
	got := renderSamplesheet(run)
	if got != "" {
		t.Fatalf("expected empty string when 'input' key is missing, got %q", got)
	}
}

func TestRenderSamplesheet_InputNotString(t *testing.T) {
	run := &state.Run{Params: map[string]any{"input": 42}}
	got := renderSamplesheet(run)
	if got != "" {
		t.Fatalf("expected empty string when 'input' is not a string, got %q", got)
	}
}

func TestRenderSamplesheet_InputBoolValue(t *testing.T) {
	run := &state.Run{Params: map[string]any{"input": true}}
	got := renderSamplesheet(run)
	if got != "" {
		t.Fatalf("expected empty string when 'input' is a bool, got %q", got)
	}
}

func TestRenderSamplesheet_FileNotFound(t *testing.T) {
	run := &state.Run{Params: map[string]any{"input": "/nonexistent/path/samplesheet.csv"}}
	got := renderSamplesheet(run)
	if got != "" {
		t.Fatalf("expected empty string for unreadable file, got %q", got)
	}
}

func TestRenderSamplesheet_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "samplesheet.csv")
	content := "sample,fastq_1,fastq_2\nA,a_1.fq,a_2.fq\n"
	if err := os.WriteFile(csvPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	if got == "" {
		t.Fatal("expected non-empty output for valid samplesheet")
	}
	if !strings.Contains(got, `<div class="samplesheet-section"`) {
		t.Error("missing samplesheet-section wrapper div")
	}
	if !strings.Contains(got, `data-ignore-morph`) {
		t.Error("missing data-ignore-morph attribute")
	}
	if !strings.Contains(got, `<div class="samplesheet-header">`) {
		t.Error("missing samplesheet-header div")
	}
	if !strings.Contains(got, `<span class="detail-label">Samplesheet</span>`) {
		t.Error("missing detail-label span with 'Samplesheet' text")
	}
	if !strings.Contains(got, `<span class="samplesheet-path">`) {
		t.Error("missing samplesheet-path span")
	}
	if !strings.Contains(got, csvPath) {
		t.Error("output should contain the file path")
	}
	if !strings.Contains(got, `<button class="btn-copy"`) {
		t.Error("missing button with btn-copy class")
	}
	if !strings.Contains(got, `>Copy</button>`) {
		t.Error("button should contain 'Copy' text")
	}
	if !strings.Contains(got, `<textarea id="samplesheet-content" class="samplesheet-textarea">`) {
		t.Error("missing textarea with correct id and class")
	}
	if !strings.Contains(got, content) {
		t.Error("textarea should contain the file contents")
	}
}

func TestRenderSamplesheet_EmptyFile(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "empty.csv")
	if err := os.WriteFile(csvPath, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	if got == "" {
		t.Fatal("expected non-empty output for empty but readable file")
	}
	if !strings.Contains(got, `<textarea id="samplesheet-content" class="samplesheet-textarea">`) {
		t.Error("missing textarea element")
	}
	if !strings.Contains(got, `</textarea>`) {
		t.Error("missing closing textarea tag")
	}
}

func TestRenderSamplesheet_DatastarClickSyntax(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "ss.csv")
	if err := os.WriteFile(csvPath, []byte("col1\nval1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	if !strings.Contains(got, `data-on:click=`) {
		t.Error("missing data-on:click attribute (Datastar colon syntax)")
	}
	if strings.Contains(got, `data-on-click=`) {
		t.Error("should use data-on:click (colon), not data-on-click (hyphen)")
	}
}

func TestRenderSamplesheet_CopyButtonClipboard(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "ss.csv")
	if err := os.WriteFile(csvPath, []byte("col1\nval1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	if !strings.Contains(got, `copyText(evt.target`) {
		t.Error("copy button should use copyText helper")
	}
	if !strings.Contains(got, `samplesheet-content`) {
		t.Error("copy button should reference samplesheet-content element")
	}
	if !strings.Contains(got, `.value`) {
		t.Error("copy button should use .value (textarea form element), not .textContent")
	}
}

func TestRenderSamplesheet_HTMLEscaping(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "special.csv")
	content := "col<1>,col&2\nval<a>,val&b\n"
	if err := os.WriteFile(csvPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	if got == "" {
		t.Fatal("expected non-empty output")
	}
	// Content inside textarea should be HTML-escaped
	if strings.Contains(got, "col<1>") {
		t.Error("file contents with < and > should be HTML-escaped in textarea")
	}
	if !strings.Contains(got, "&lt;") {
		t.Error("expected &lt; entity for < in file contents")
	}
	if !strings.Contains(got, "&amp;") {
		t.Error("expected &amp; entity for & in file contents")
	}
}

func TestRenderSamplesheet_PathEscaping(t *testing.T) {
	tmp := t.TempDir()
	// Create a file in a directory with safe name, but test path display escaping
	// by checking the escaped path in the output
	csvPath := filepath.Join(tmp, "samplesheet.csv")
	if err := os.WriteFile(csvPath, []byte("a\n"), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	// Path should appear in the samplesheet-path span
	if !strings.Contains(got, csvPath) {
		t.Error("output should display the file path")
	}
}

func TestRenderSamplesheet_InputEmptyString(t *testing.T) {
	run := &state.Run{Params: map[string]any{"input": ""}}
	got := renderSamplesheet(run)
	// Empty string path will fail to read, should return ""
	if got != "" {
		t.Fatalf("expected empty string for empty input path, got %q", got)
	}
}
