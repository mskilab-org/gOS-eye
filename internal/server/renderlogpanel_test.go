package server

import (
	"html"
	"strings"
	"testing"
)

func TestRenderLogPanel_BothHaveContent(t *testing.T) {
	got := renderLogPanel("sayHello (1)", "hello world\n", "some warning\n", "")

	// Must have top-level morph target
	if !strings.Contains(got, `<div id="log-panel-content">`) {
		t.Fatal("missing <div id=\"log-panel-content\">")
	}
	// Task name header
	if !strings.Contains(got, "sayHello (1)") {
		t.Fatal("missing task name in output")
	}
	// Two log sections
	if strings.Count(got, `class="log-section"`) != 2 {
		t.Fatalf("expected 2 log-section divs, got %d", strings.Count(got, `class="log-section"`))
	}
	// Labels
	if !strings.Contains(got, ".command.log") {
		t.Fatal("missing .command.log label")
	}
	if !strings.Contains(got, ".command.err") {
		t.Fatal("missing .command.err label")
	}
	// Content
	if !strings.Contains(got, "hello world") {
		t.Fatal("missing stdout content")
	}
	if !strings.Contains(got, "some warning") {
		t.Fatal("missing stderr content")
	}
	// log-content class for both
	if strings.Count(got, `class="log-content"`) != 2 {
		t.Fatalf("expected 2 log-content divs, got %d", strings.Count(got, `class="log-content"`))
	}
	// No error message
	if strings.Contains(got, `class="log-error-msg"`) {
		t.Fatal("should not have error message when errMsg is empty")
	}
}

func TestRenderLogPanel_ErrorMessage(t *testing.T) {
	got := renderLogPanel("align (3)", "", "", "workdir not found")

	if !strings.Contains(got, `<div id="log-panel-content">`) {
		t.Fatal("missing <div id=\"log-panel-content\">")
	}
	// Task name header still present
	if !strings.Contains(got, "align (3)") {
		t.Fatal("missing task name in output")
	}
	// Error message shown
	if !strings.Contains(got, `class="log-error-msg"`) {
		t.Fatal("missing log-error-msg class")
	}
	if !strings.Contains(got, "workdir not found") {
		t.Fatal("missing error message text")
	}
	// No log sections when error
	if strings.Contains(got, `class="log-section"`) {
		t.Fatal("should not have log sections when errMsg is set")
	}
}

func TestRenderLogPanel_StdoutEmpty_StderrHasContent(t *testing.T) {
	got := renderLogPanel("task1 (1)", "", "error output\n", "")

	// stdout section should show (empty)
	if !strings.Contains(got, "(empty)") {
		t.Fatal("missing (empty) placeholder for empty stdout")
	}
	// stderr section should show content
	if !strings.Contains(got, "error output") {
		t.Fatal("missing stderr content")
	}
	// Both sections present
	if strings.Count(got, `class="log-section"`) != 2 {
		t.Fatalf("expected 2 log-section divs, got %d", strings.Count(got, `class="log-section"`))
	}
}

func TestRenderLogPanel_BothEmpty(t *testing.T) {
	got := renderLogPanel("task1 (1)", "", "", "")

	// Both sections present
	if strings.Count(got, `class="log-section"`) != 2 {
		t.Fatalf("expected 2 log-section divs, got %d", strings.Count(got, `class="log-section"`))
	}
	// Both should show (empty)
	if strings.Count(got, "(empty)") != 2 {
		t.Fatalf("expected 2 (empty) placeholders, got %d", strings.Count(got, "(empty)"))
	}
}

func TestRenderLogPanel_HTMLEscaping(t *testing.T) {
	got := renderLogPanel("<script>alert('xss')</script>", "<b>bold</b>", "a & b < c", "")

	// Task name must be escaped
	if strings.Contains(got, "<script>alert") {
		t.Fatal("task name not escaped — XSS vulnerability")
	}
	if !strings.Contains(got, html.EscapeString("<script>alert('xss')</script>")) {
		t.Fatal("task name not properly HTML-escaped")
	}
	// Stdout content must be escaped
	if strings.Contains(got, "<b>bold</b>") {
		t.Fatal("stdout not escaped — XSS vulnerability")
	}
	if !strings.Contains(got, html.EscapeString("<b>bold</b>")) {
		t.Fatal("stdout not properly HTML-escaped")
	}
	// Stderr content must be escaped
	if !strings.Contains(got, html.EscapeString("a & b < c")) {
		t.Fatal("stderr not properly HTML-escaped")
	}
}
