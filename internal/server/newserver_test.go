package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

func TestNewServer_ReturnsNonNil(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)
	if s == nil {
		t.Fatal("NewServer() returned nil")
	}
}

func TestNewServer_StoresGivenStore(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)
	if s.store != store {
		t.Fatal("server.store does not point to the provided Store")
	}
}

func TestNewServer_CreatesBroker(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)
	if s.broker == nil {
		t.Fatal("server.broker is nil; expected a Broker created via NewBroker")
	}
}

func TestNewServer_BrokerSubscribersInitialized(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)
	if s.broker.subscribers == nil {
		t.Fatal("broker.subscribers is nil; expected initialized empty map")
	}
}

func TestNewServer_CreatesMux(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)
	if s.mux == nil {
		t.Fatal("server.mux is nil; expected a new http.ServeMux")
	}
}

func TestNewServer_IndependentInstances(t *testing.T) {
	s1 := NewServer(state.NewStore(), nil)
	s2 := NewServer(state.NewStore(), nil)
	if s1 == s2 {
		t.Fatal("two NewServer() calls returned the same pointer")
	}
	if s1.broker == s2.broker {
		t.Fatal("two NewServer() calls share the same Broker")
	}
	if s1.mux == s2.mux {
		t.Fatal("two NewServer() calls share the same ServeMux")
	}
}

// Route registration tests — verify that the mux routes to the correct handlers.
// The handlers are stubs that panic, so we use recover() to confirm they were called.

func TestNewServer_WebhookRouteRegistered(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	w := httptest.NewRecorder()

	// handleWebhook is a stub that panics — catching the panic proves the route is registered.
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		s.mux.ServeHTTP(w, req)
	}()

	if !panicked {
		// If no panic, check it didn't 404 (would mean route not registered)
		if w.Code == http.StatusNotFound {
			t.Fatal("POST /webhook returned 404; route not registered on mux")
		}
	}
}

func TestNewServer_SSESidebarRouteRegistered(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so handleSSE exits its select loop
	req := httptest.NewRequest(http.MethodGet, "/sse/sidebar", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		s.mux.ServeHTTP(w, req)
	}()

	if !panicked {
		if w.Code == http.StatusNotFound {
			t.Fatal("GET /sse/sidebar returned 404; route not registered on mux")
		}
	}
}

func TestNewServer_SSERunRouteRegistered(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)

	req := httptest.NewRequest(http.MethodGet, "/sse/run/test-run-id", nil)
	w := httptest.NewRecorder()

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		s.mux.ServeHTTP(w, req)
	}()

	if !panicked {
		if w.Code == http.StatusNotFound {
			t.Fatal("GET /sse/run/{id} returned 404; route not registered on mux")
		}
	}
}

func TestNewServer_IndexRouteRegistered(t *testing.T) {
	store := state.NewStore()
	s := NewServer(store, nil)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
			}
		}()
		s.mux.ServeHTTP(w, req)
	}()

	if !panicked {
		if w.Code == http.StatusNotFound {
			t.Fatal("GET / returned 404; route not registered on mux")
		}
	}
}
