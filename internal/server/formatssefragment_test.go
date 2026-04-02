package server

import (
	"strings"
	"testing"
)

func TestFormatSSEFragment_SingleLine(t *testing.T) {
	html := `<div id="x">hello</div>`
	got := formatSSEFragment(html)
	want := "event: datastar-patch-elements\ndata: elements <div id=\"x\">hello</div>\n\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatSSEFragment_MultiLine(t *testing.T) {
	html := "<div>\n  <p>one</p>\n  <p>two</p>\n</div>"
	got := formatSSEFragment(html)
	want := "event: datastar-patch-elements\n" +
		"data: elements <div>\n" +
		"data: elements   <p>one</p>\n" +
		"data: elements   <p>two</p>\n" +
		"data: elements </div>\n\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatSSEFragment_EmptyString(t *testing.T) {
	got := formatSSEFragment("")
	want := "event: datastar-patch-elements\ndata: elements \n\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestFormatSSEFragment_StartsWithEventLine(t *testing.T) {
	got := formatSSEFragment("<p>test</p>")
	if !strings.HasPrefix(got, "event: datastar-patch-elements\n") {
		t.Fatalf("output does not start with event line:\n%q", got)
	}
}

func TestFormatSSEFragment_EndsWithDoubleNewline(t *testing.T) {
	got := formatSSEFragment("<p>test</p>")
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("output does not end with double newline:\n%q", got)
	}
}

func TestFormatSSEFragment_EachDataLineHasPrefix(t *testing.T) {
	html := "<ul>\n  <li>a</li>\n  <li>b</li>\n</ul>"
	got := formatSSEFragment(html)
	lines := strings.Split(got, "\n")
	// lines: [event:..., data:..., data:..., data:..., data:..., "", ""]
	// Skip the first line (event) and the last two empty strings from the trailing \n\n.
	dataLines := lines[1 : len(lines)-2]
	for i, line := range dataLines {
		if !strings.HasPrefix(line, "data: elements ") {
			t.Fatalf("data line %d missing prefix: %q", i, line)
		}
	}
}

func TestFormatSSEFragmentWithSelector_IncludesSelectorLine(t *testing.T) {
	html := `<div id="dashboard">hello</div>`
	got := formatSSEFragmentWithSelector(html, `#dashboard[data-run="r1"]`)

	if !strings.HasPrefix(got, "event: datastar-patch-elements\n") {
		t.Fatalf("missing event line, got:\n%q", got)
	}
	if !strings.Contains(got, `data: selector #dashboard[data-run="r1"]`) {
		t.Fatalf("missing selector line, got:\n%q", got)
	}
	if !strings.Contains(got, "data: elements <div") {
		t.Fatalf("missing elements line, got:\n%q", got)
	}
	if !strings.HasSuffix(got, "\n\n") {
		t.Fatalf("missing double newline terminator, got:\n%q", got)
	}
}

func TestFormatSSEFragmentWithSelector_SelectorBeforeElements(t *testing.T) {
	got := formatSSEFragmentWithSelector("<p>x</p>", "#foo")
	lines := strings.Split(got, "\n")
	// lines[0] = "event: ...", lines[1] = "data: selector ...", lines[2] = "data: elements ..."
	if len(lines) < 3 {
		t.Fatalf("expected at least 3 lines, got %d: %q", len(lines), got)
	}
	if !strings.HasPrefix(lines[1], "data: selector ") {
		t.Fatalf("line 1 should be selector, got: %q", lines[1])
	}
	if !strings.HasPrefix(lines[2], "data: elements ") {
		t.Fatalf("line 2 should be elements, got: %q", lines[2])
	}
}
