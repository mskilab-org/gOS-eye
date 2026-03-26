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
	// Structural wrapper
	if !strings.Contains(got, `<div class="samplesheet-section"`) {
		t.Error("missing samplesheet-section wrapper div")
	}
	if !strings.Contains(got, `data-ignore-morph`) {
		t.Error("missing data-ignore-morph attribute")
	}
	if !strings.Contains(got, `<div class="samplesheet-header"`) {
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
	// Table rendering (not textarea)
	if !strings.Contains(got, `<table`) {
		t.Error("valid CSV should render as a table, not textarea")
	}
	if strings.Contains(got, `<textarea`) {
		t.Error("valid CSV should NOT fall back to textarea")
	}
	// Headers
	if !strings.Contains(got, `<th>sample</th>`) {
		t.Error("missing table header 'sample'")
	}
	if !strings.Contains(got, `<th>fastq_1</th>`) {
		t.Error("missing table header 'fastq_1'")
	}
	if !strings.Contains(got, `<th>fastq_2</th>`) {
		t.Error("missing table header 'fastq_2'")
	}
	// Editable inputs for data cells
	if !strings.Contains(got, `<input type="text"`) {
		t.Error("table cells should contain editable text inputs")
	}
	if !strings.Contains(got, `value="A"`) {
		t.Error("missing input with value 'A'")
	}
	if !strings.Contains(got, `value="a_1.fq"`) {
		t.Error("missing input with value 'a_1.fq'")
	}
	// Buttons
	if !strings.Contains(got, `copySamplesheet`) {
		t.Error("copy button should call copySamplesheet")
	}
	if !strings.Contains(got, `addSamplesheetRow`) {
		t.Error("should have addSamplesheetRow button")
	}
	// Remove button per row
	if !strings.Contains(got, `btn-remove-row`) {
		t.Error("each row should have a remove button")
	}
	if !strings.Contains(got, `onclick="removeSamplesheetRow(this)"`) {
		t.Error("remove button should use onclick with removeSamplesheetRow(this)")
	}
	// Actions column header
	if !strings.Contains(got, `<th class="col-actions"></th>`) {
		t.Error("table should have an empty actions column header")
	}
	// Undo/redo buttons
	if !strings.Contains(got, `undoSamplesheet()`) {
		t.Error("should have undo button")
	}
	if !strings.Contains(got, `redoSamplesheet()`) {
		t.Error("should have redo button")
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
	// Empty file cannot be parsed as CSV → textarea fallback
	if !strings.Contains(got, `<textarea id="samplesheet-content" class="samplesheet-textarea">`) {
		t.Error("empty file should fall back to textarea rendering")
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
	// Valid CSV → table path with copySamplesheet
	if err := os.WriteFile(csvPath, []byte("col1\nval1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	if !strings.Contains(got, `copySamplesheet(evt.target)`) {
		t.Error("copy button should call copySamplesheet(evt.target)")
	}
}

func TestRenderSamplesheet_HTMLEscaping(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "special.csv")
	// CSV with special HTML chars in both headers and values
	content := "\"col<1>\",\"col&2\"\n\"val<a>\",\"val&b\"\n"
	if err := os.WriteFile(csvPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	if got == "" {
		t.Fatal("expected non-empty output")
	}
	// Special chars should be HTML-escaped in value attributes and th content
	if strings.Contains(got, "col<1>") {
		t.Error("< and > should be HTML-escaped")
	}
	if !strings.Contains(got, "&lt;") {
		t.Error("expected &lt; entity for < in content")
	}
	if !strings.Contains(got, "&amp;") {
		t.Error("expected &amp; entity for & in content")
	}
}

func TestRenderSamplesheet_PathEscaping(t *testing.T) {
	tmp := t.TempDir()
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
	if got != "" {
		t.Fatalf("expected empty string for empty input path, got %q", got)
	}
}

func TestRenderSamplesheet_NonCSVFallback(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "notes.txt")
	// Plain text paragraph — not valid CSV (well, single-column CSV parses fine,
	// but content with unbalanced quotes should fail)
	content := "This is just a \"plain text note with unbalanced quote\nand another line\n"
	if err := os.WriteFile(csvPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	if got == "" {
		t.Fatal("expected non-empty output even for non-CSV content")
	}
	// Should fall back to textarea since parseSamplesheetCSV fails
	if !strings.Contains(got, `<textarea`) {
		t.Error("non-CSV content should fall back to textarea rendering")
	}
	if strings.Contains(got, `<table`) {
		t.Error("non-CSV content should NOT render as a table")
	}
}

func TestRenderSamplesheet_AddRowButton(t *testing.T) {
	tmp := t.TempDir()
	csvPath := filepath.Join(tmp, "ss.csv")
	if err := os.WriteFile(csvPath, []byte("sample,fastq\nA,a.fq\n"), 0644); err != nil {
		t.Fatal(err)
	}

	run := &state.Run{Params: map[string]any{"input": csvPath}}
	got := renderSamplesheet(run)

	if !strings.Contains(got, `<button class="btn-add-row"`) {
		t.Error("missing Add Row button with btn-add-row class")
	}
	if !strings.Contains(got, `data-on:click__stop="addSamplesheetRow()"`) {
		t.Error("Add Row button should have data-on:click__stop calling addSamplesheetRow()")
	}
	if !strings.Contains(got, `+ Add Row</button>`) {
		t.Error("Add Row button should display '+ Add Row'")
	}
	// Remove button on each data row
	if !strings.Contains(got, `<td class="cell-actions">`) {
		t.Error("each row should have a cell-actions td")
	}
	if !strings.Contains(got, `btn-remove-row`) {
		t.Error("each row should have a remove button with btn-remove-row class")
	}
}
