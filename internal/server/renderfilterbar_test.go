package server

import (
	"strings"
	"testing"
)

func TestRenderFilterBar_RendersInputAndSelect(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", "", "")

	if !strings.Contains(got, `<input type="text"`) {
		t.Fatal("missing text input")
	}
	if !strings.Contains(got, `<select`) {
		t.Fatal("missing select element")
	}
	if !strings.Contains(got, `class="task-filter-bar"`) {
		t.Fatal("missing task-filter-bar class")
	}
	if !strings.Contains(got, `class="task-search"`) {
		t.Fatal("missing task-search class on input")
	}
	if !strings.Contains(got, `class="task-status-filter"`) {
		t.Fatal("missing task-status-filter class on select")
	}
	// Check option values exist
	for _, opt := range []string{`value=""`, `value="FAILED"`, `value="RUNNING"`, `value="COMPLETED"`} {
		if !strings.Contains(got, opt) {
			t.Fatalf("missing option with %s", opt)
		}
	}
}

func TestRenderFilterBar_PreFillsQ(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", "hello world", "")

	if !strings.Contains(got, `value="hello world"`) {
		t.Fatalf("expected value attribute with q pre-filled, got:\n%s", got)
	}
}

func TestRenderFilterBar_PreFillsQ_HTMLEscaped(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", `<script>alert("xss")</script>`, "")

	if strings.Contains(got, `<script>`) {
		t.Fatal("q value not HTML-escaped — XSS possible")
	}
	if !strings.Contains(got, `&lt;script&gt;`) {
		t.Fatalf("expected HTML-escaped q in value attribute, got:\n%s", got)
	}
}

func TestRenderFilterBar_PreSelectsStatus(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", "", "FAILED")

	// FAILED option should have selected attribute
	if !strings.Contains(got, `value="FAILED" selected`) {
		t.Fatalf("expected FAILED option to have selected attribute, got:\n%s", got)
	}
	// Other options should NOT have selected
	if strings.Contains(got, `value="RUNNING" selected`) {
		t.Fatal("RUNNING should not be selected")
	}
	if strings.Contains(got, `value="COMPLETED" selected`) {
		t.Fatal("COMPLETED should not be selected")
	}
	if strings.Contains(got, `value="" selected`) {
		t.Fatal("All option should not be selected when FAILED is active")
	}
}

func TestRenderFilterBar_PreSelectsStatus_Running(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", "", "RUNNING")

	if !strings.Contains(got, `value="RUNNING" selected`) {
		t.Fatalf("expected RUNNING option to have selected attribute, got:\n%s", got)
	}
	if strings.Contains(got, `value="FAILED" selected`) {
		t.Fatal("FAILED should not be selected")
	}
}

func TestRenderFilterBar_HasDataIgnoreMorph(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", "", "")

	if !strings.Contains(got, "data-ignore-morph") {
		t.Fatalf("expected data-ignore-morph on outer div, got:\n%s", got)
	}
}

func TestRenderFilterBar_URLsUseOneShotEndpoint(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", "", "")

	// URLs should point to the one-shot /tasks/ endpoint
	if !strings.Contains(got, `@get('/tasks/run-1/sayHello?`) {
		t.Fatalf("expected @get to target /tasks/{runID}/{process} endpoint, got:\n%s", got)
	}
	if strings.Contains(got, `/sse/run/`) {
		t.Fatal("filter bar URLs should NOT use /sse/ endpoint")
	}
	if strings.Contains(got, `/run/`) {
		t.Fatal("filter bar URLs should NOT use old /run/ path")
	}
}

func TestRenderFilterBar_DataBindAttributes(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", "", "")

	if !strings.Contains(got, `data-bind:_task-filter`) {
		t.Fatalf("missing data-bind:_task-filter on input, got:\n%s", got)
	}
	if !strings.Contains(got, `data-bind:_status-filter`) {
		t.Fatalf("missing data-bind:_status-filter on select, got:\n%s", got)
	}
}

func TestRenderFilterBar_ProcessInID(t *testing.T) {
	got := renderFilterBar("alignReads", "run-42", "", "")

	if !strings.Contains(got, `id="task-filter-alignReads"`) {
		t.Fatalf("expected process name in div id, got:\n%s", got)
	}
}

func TestRenderFilterBar_DebouncedInput(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", "", "")

	if !strings.Contains(got, `data-on:input__debounce`) {
		t.Fatalf("expected debounced input event, got:\n%s", got)
	}
}

func TestRenderFilterBar_EmptyQAndStatus(t *testing.T) {
	got := renderFilterBar("sayHello", "run-1", "", "")

	// value should be empty string
	if !strings.Contains(got, `value=""`) {
		t.Fatalf("expected empty value attribute, got:\n%s", got)
	}
	// No option should have selected (All is default by browser)
	if strings.Contains(got, "selected") {
		t.Fatalf("no option should have 'selected' when statusFilter is empty, got:\n%s", got)
	}
}
