package server

import (
	"strings"
	"testing"
)

func TestRenderRunSelector_EmptyLatestRunID(t *testing.T) {
	got := renderRunSelector("")
	want := `<div id="run-selector" style="display:none"></div>`
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestRenderRunSelector_WithLatestRunID(t *testing.T) {
	got := renderRunSelector("abc123")
	if !strings.Contains(got, `data-init="$selectedRun === '' && @get('/select-run/abc123')"`) {
		t.Fatalf("expected data-init with conditional @get, got %q", got)
	}
}

func TestRenderRunSelector_ContainsSelectRunURL(t *testing.T) {
	got := renderRunSelector("run-42")
	if !strings.Contains(got, `/select-run/run-42`) {
		t.Fatalf("expected /select-run/run-42 URL, got %q", got)
	}
	if strings.Contains(got, `/sse/run/run-42`) {
		t.Fatal("should use /select-run/ not /sse/run/")
	}
}

func TestRenderRunSelector_HasHiddenStyle(t *testing.T) {
	got := renderRunSelector("xyz")
	if !strings.Contains(got, `style="display:none"`) {
		t.Fatalf("expected style=\"display:none\", got %q", got)
	}
}

func TestRenderRunSelector_HasCorrectID(t *testing.T) {
	got := renderRunSelector("xyz")
	if !strings.Contains(got, `id="run-selector"`) {
		t.Fatalf("expected id=\"run-selector\", got %q", got)
	}
}
