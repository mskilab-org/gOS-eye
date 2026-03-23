package main

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/mskilab-org/nextflow-monitor/internal/server"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// TestMainWiring verifies that the Store and Server wire together correctly
// and the server accepts HTTP connections (the core logic of main minus blocking).
func TestMainWiring(t *testing.T) {
	store := state.NewStore()
	if store == nil {
		t.Fatal("NewStore() returned nil")
	}

	srv := server.NewServer(store)
	if srv == nil {
		t.Fatal("NewServer() returned nil")
	}

	// Start the server on a random available port
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	// Give server a moment to start
	time.Sleep(50 * time.Millisecond)

	// Verify the server responds to the index route (may 500 if web/index.html
	// is missing, but the route must be registered — not 404).
	resp, err := http.Get("http://" + ln.Addr().String() + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Error("GET / returned 404, expected route to be registered")
	}
}

// TestBuildAddr verifies address string construction from host and port.
func TestBuildAddr(t *testing.T) {
	tests := []struct {
		name string
		host string
		port int
		want string
	}{
		{name: "default localhost:8080", host: "localhost", port: 8080, want: "localhost:8080"},
		{name: "empty host", host: "", port: 8080, want: ":8080"},
		{name: "all interfaces", host: "0.0.0.0", port: 3000, want: "0.0.0.0:3000"},
		{name: "custom host and port", host: "10.0.0.1", port: 9090, want: "10.0.0.1:9090"},
		{name: "port zero", host: "localhost", port: 0, want: "localhost:0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildAddr(tt.host, tt.port)
			if got != tt.want {
				t.Errorf("buildAddr(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}

// TestMainServerHandlesWebhook verifies the wired server accepts webhook POSTs.
func TestMainServerHandlesWebhook(t *testing.T) {
	store := state.NewStore()
	srv := server.NewServer(store)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	time.Sleep(50 * time.Millisecond)

	// Webhook route should be registered (GET will likely fail with bad method or bad JSON,
	// but it must not return 404).
	resp, err := http.Get("http://" + ln.Addr().String() + "/webhook")
	if err != nil {
		t.Fatalf("GET /webhook failed: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		t.Error("GET /webhook returned 404, expected route to be registered")
	}
}
