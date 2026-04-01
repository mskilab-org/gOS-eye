package server

import (
	"strings"
	"testing"
)

func TestRenderPagination_IncludesQ(t *testing.T) {
	// 20 items, page 1 of 2, q="hello" — buttons should include &q=hello
	html := renderPagination("proc", "run-1", "hello", "", 1, 2, 0, 10, 20)

	if !strings.Contains(html, "&q=hello") {
		t.Errorf("expected &q=hello in button URLs, got:\n%s", html)
	}
	// Should NOT have &status= when statusFilter is empty.
	if strings.Contains(html, "&status=") {
		t.Errorf("expected no &status= when statusFilter is empty, got:\n%s", html)
	}
}

func TestRenderPagination_IncludesStatus(t *testing.T) {
	// 20 items, page 1 of 2, statusFilter="FAILED"
	html := renderPagination("proc", "run-1", "", "FAILED", 1, 2, 0, 10, 20)

	if !strings.Contains(html, "&status=FAILED") {
		t.Errorf("expected &status=FAILED in button URLs, got:\n%s", html)
	}
	// Should NOT have &q= when q is empty.
	if strings.Contains(html, "&q=") {
		t.Errorf("expected no &q= when q is empty, got:\n%s", html)
	}
}

func TestRenderPagination_BothFilters(t *testing.T) {
	// 20 items, page 1 of 2, both q and statusFilter set
	html := renderPagination("proc", "run-1", "sample", "FAILED", 1, 2, 0, 10, 20)

	if !strings.Contains(html, "&q=sample") {
		t.Errorf("expected &q=sample in button URLs, got:\n%s", html)
	}
	if !strings.Contains(html, "&status=FAILED") {
		t.Errorf("expected &status=FAILED in button URLs, got:\n%s", html)
	}
}

func TestRenderPagination_EmptyFilters(t *testing.T) {
	// 20 items, page 1 of 2, no filters — clean URLs
	html := renderPagination("proc", "run-1", "", "", 1, 2, 0, 10, 20)

	if strings.Contains(html, "&q=") {
		t.Errorf("expected no &q= when q is empty, got:\n%s", html)
	}
	if strings.Contains(html, "&status=") {
		t.Errorf("expected no &status= when statusFilter is empty, got:\n%s", html)
	}
	// Should still have basic pagination buttons.
	if !strings.Contains(html, "btn-page") {
		t.Errorf("expected pagination buttons, got:\n%s", html)
	}
}

func TestRenderPagination_URLEncoded(t *testing.T) {
	// q has special chars: "hello world&foo=bar"
	// url.QueryEscape("hello world&foo=bar") = "hello+world%26foo%3Dbar"
	html := renderPagination("proc", "run-1", "hello world&foo=bar", "", 1, 2, 0, 10, 20)

	if !strings.Contains(html, "&q=hello+world%26foo%3Dbar") {
		t.Errorf("expected URL-encoded q param, got:\n%s", html)
	}
	// Raw unencoded value should NOT appear in the URL.
	if strings.Contains(html, "&q=hello world") {
		t.Errorf("expected q to be URL-encoded, not raw, got:\n%s", html)
	}
}
