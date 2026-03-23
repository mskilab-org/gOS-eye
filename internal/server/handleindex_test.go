package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testInDir runs fn with the working directory set to dir, then restores the original.
func testInDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(orig)
	fn()
}

func TestHandleIndex_ServesFileContent(t *testing.T) {
	tmp := t.TempDir()
	webDir := filepath.Join(tmp, "web")
	os.MkdirAll(webDir, 0o755)
	content := "<html><body>hello</body></html>"
	os.WriteFile(filepath.Join(webDir, "index.html"), []byte(content), 0o644)

	s := &Server{}
	testInDir(t, tmp, func() {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		s.handleIndex(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d; want 200", w.Code)
		}
		if got := w.Body.String(); got != content {
			t.Fatalf("body = %q; want %q", got, content)
		}
	})
}

func TestHandleIndex_ContentTypeIsHTML(t *testing.T) {
	tmp := t.TempDir()
	webDir := filepath.Join(tmp, "web")
	os.MkdirAll(webDir, 0o755)
	os.WriteFile(filepath.Join(webDir, "index.html"), []byte("<html></html>"), 0o644)

	s := &Server{}
	testInDir(t, tmp, func() {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		s.handleIndex(w, r)

		ct := w.Header().Get("Content-Type")
		if !strings.HasPrefix(ct, "text/html") {
			t.Fatalf("Content-Type = %q; want text/html", ct)
		}
	})
}

func TestHandleIndex_FileNotFound_Returns500(t *testing.T) {
	tmp := t.TempDir() // empty, no web/index.html

	s := &Server{}
	testInDir(t, tmp, func() {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		s.handleIndex(w, r)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d; want 500", w.Code)
		}
		if body := w.Body.String(); body == "" {
			t.Fatal("expected error message in body")
		}
	})
}

func TestHandleIndex_LargeFile(t *testing.T) {
	tmp := t.TempDir()
	webDir := filepath.Join(tmp, "web")
	os.MkdirAll(webDir, 0o755)
	content := strings.Repeat("<p>line</p>\n", 1000)
	os.WriteFile(filepath.Join(webDir, "index.html"), []byte(content), 0o644)

	s := &Server{}
	testInDir(t, tmp, func() {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		s.handleIndex(w, r)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d; want 200", w.Code)
		}
		if got := w.Body.String(); got != content {
			t.Fatalf("body length = %d; want %d", len(got), len(content))
		}
	})
}
