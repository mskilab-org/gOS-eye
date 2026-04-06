package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestHandleIndex_ServesFileContent(t *testing.T) {
	content := "<html><body>hello</body></html>"
	s := &Server{
		WebFS: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte(content)},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleIndex(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	if got := w.Body.String(); got != content {
		t.Fatalf("body = %q; want %q", got, content)
	}
}

func TestHandleIndex_ContentTypeIsHTML(t *testing.T) {
	s := &Server{
		WebFS: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte("<html></html>")},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleIndex(w, r)

	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("Content-Type = %q; want text/html", ct)
	}
}

func TestHandleIndex_FileNotFound_Returns500(t *testing.T) {
	s := &Server{
		WebFS: fstest.MapFS{}, // empty FS, no index.html
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleIndex(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d; want 500", w.Code)
	}
	if body := w.Body.String(); body == "" {
		t.Fatal("expected error message in body")
	}
}

func TestHandleIndex_LargeFile(t *testing.T) {
	content := strings.Repeat("<p>line</p>\n", 1000)
	s := &Server{
		WebFS: fstest.MapFS{
			"index.html": &fstest.MapFile{Data: []byte(content)},
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	s.handleIndex(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d; want 200", w.Code)
	}
	if got := w.Body.String(); got != content {
		t.Fatalf("body length = %d; want %d", len(got), len(content))
	}
}
