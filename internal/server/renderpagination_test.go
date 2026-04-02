package server

import (
	"strings"
	"testing"
)

func TestRenderPagination_SinglePage(t *testing.T) {
	// totalPages <= 1 → empty string
	if got := renderPagination("proc", "run-1", "", "", 1, 1, 0, 5, 5); got != "" {
		t.Errorf("expected empty string for single page, got:\n%s", got)
	}
	if got := renderPagination("proc", "run-1", "", "", 1, 0, 0, 0, 0); got != "" {
		t.Errorf("expected empty string for zero pages, got:\n%s", got)
	}
}

func TestRenderPagination_NewURLFormat(t *testing.T) {
	// Buttons must use /tasks/{runID}/{process}?page=N, NOT /run/.../tasks/.../page
	html := renderPagination("myProc", "run-42", "", "", 1, 3, 0, 10, 30)

	if !strings.Contains(html, "/tasks/run-42/myProc?page=") {
		t.Errorf("expected /tasks/{runID}/{process}?page=N URL format, got:\n%s", html)
	}
	// Old format must NOT appear
	if strings.Contains(html, "/run/") {
		t.Errorf("old /run/ URL format must not appear, got:\n%s", html)
	}
}

func TestRenderPagination_InfoText(t *testing.T) {
	// start=0, end=10, total=25 → "1–10 of 25"
	html := renderPagination("proc", "run-1", "", "", 1, 3, 0, 10, 25)
	if !strings.Contains(html, "1–10 of 25") {
		t.Errorf("expected info text '1–10 of 25', got:\n%s", html)
	}
}

func TestRenderPagination_FirstPageButtons(t *testing.T) {
	// page=1 of 3: first/prev disabled, next/last enabled
	html := renderPagination("proc", "run-1", "", "", 1, 3, 0, 10, 30)

	// « and ‹ buttons should be disabled
	if !strings.Contains(html, `<button class="btn-page" disabled>«</button>`) {
		t.Errorf("expected disabled « button on first page, got:\n%s", html)
	}
	if !strings.Contains(html, `<button class="btn-page" disabled>‹</button>`) {
		t.Errorf("expected disabled ‹ button on first page, got:\n%s", html)
	}
	// › should link to page 2
	if !strings.Contains(html, "page=2") {
		t.Errorf("expected next button to link to page 2, got:\n%s", html)
	}
	// » should link to page 3 (totalPages)
	if !strings.Contains(html, "page=3") {
		t.Errorf("expected last button to link to page 3, got:\n%s", html)
	}
}

func TestRenderPagination_LastPageButtons(t *testing.T) {
	// page=3 of 3: first/prev enabled, next/last disabled
	html := renderPagination("proc", "run-1", "", "", 3, 3, 20, 30, 30)

	// › and » should be disabled
	if !strings.Contains(html, `<button class="btn-page" disabled>›</button>`) {
		t.Errorf("expected disabled › button on last page, got:\n%s", html)
	}
	if !strings.Contains(html, `<button class="btn-page" disabled>»</button>`) {
		t.Errorf("expected disabled » button on last page, got:\n%s", html)
	}
	// « should link to page 1
	if !strings.Contains(html, "page=1") {
		t.Errorf("expected first button to link to page 1, got:\n%s", html)
	}
	// ‹ should link to page 2
	if !strings.Contains(html, "page=2") {
		t.Errorf("expected prev button to link to page 2, got:\n%s", html)
	}
}

func TestRenderPagination_MiddlePageButtons(t *testing.T) {
	// page=2 of 4: all buttons enabled
	html := renderPagination("proc", "run-1", "", "", 2, 4, 10, 20, 40)

	// No disabled buttons
	if strings.Contains(html, "disabled") {
		t.Errorf("expected no disabled buttons on middle page, got:\n%s", html)
	}
	// « → page 1, ‹ → page 1, › → page 3, » → page 4
	if !strings.Contains(html, "page=1") {
		t.Errorf("expected page=1 for first button, got:\n%s", html)
	}
	if !strings.Contains(html, "page=3") {
		t.Errorf("expected page=3 for next button, got:\n%s", html)
	}
	if !strings.Contains(html, "page=4") {
		t.Errorf("expected page=4 for last button, got:\n%s", html)
	}
}

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
