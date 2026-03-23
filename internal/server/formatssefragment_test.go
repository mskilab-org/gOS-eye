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
