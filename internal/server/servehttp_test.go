package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestServeHTTP_DelegatesToMux_Webhook(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)

	// POST /webhook with invalid JSON → handleWebhook returns 400.
	// This proves ServeHTTP delegates to the mux (not panicking or returning 404).
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader("not json"))
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("POST /webhook bad JSON: status = %d; want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestServeHTTP_DelegatesToMux_Root(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)

	// GET / → handleIndex. It may return 200 or 500 depending on whether
	// web/index.html exists, but the response should not be a panic or 404
	// from a missing route.
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	// handleIndex returns 200 (file found) or 500 (file not found).
	if rec.Code != http.StatusOK && rec.Code != http.StatusInternalServerError {
		t.Errorf("GET /: status = %d; want 200 or 500", rec.Code)
	}
}

func TestServeHTTP_DelegatesToMux_StaticAsset(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)
	s.WebFS = fstest.MapFS{
		"logo.png": &fstest.MapFile{Data: []byte("png-bytes")},
	}

	req := httptest.NewRequest(http.MethodGet, "/static/logo.png", nil)
	rec := httptest.NewRecorder()

	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /static/logo.png: status = %d; want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Body.String(); got != "png-bytes" {
		t.Fatalf("GET /static/logo.png: body = %q; want %q", got, "png-bytes")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/png") {
		t.Fatalf("GET /static/logo.png: Content-Type = %q; want image/png", ct)
	}
}

func TestServeHTTP_ImplementsHTTPHandler(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)

	// Runtime interface assertion (compile-time check already in server.go).
	var h http.Handler = s
	if h == nil {
		t.Fatal("Server does not satisfy http.Handler")
	}
}
