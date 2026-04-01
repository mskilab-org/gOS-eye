// Entry point for nextflow-monitor.
// Creates Store, creates Server, listens on the configured host:port.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/mskilab-org/nextflow-monitor/internal/dag"
	"github.com/mskilab-org/nextflow-monitor/internal/db"
	"github.com/mskilab-org/nextflow-monitor/internal/server"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// buildAddr constructs a listen address from host and port.
func buildAddr(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

// dbDefault returns the default database path, using the NEXTFLOW_MONITOR_DB
// env var if set, otherwise "./nextflow-monitor.db".
func dbDefault() string {
	if envDB := os.Getenv("NEXTFLOW_MONITOR_DB"); envDB != "" {
		return envDB
	}
	return "./nextflow-monitor.db"
}

// replayEvents loads all persisted events from the EventStore, unmarshals each
// into a WebhookEvent, and calls store.HandleEvent to rebuild in-memory state.
// Invalid JSON blobs are logged and skipped. Returns the count of successfully
// replayed events.
func replayEvents(eventStore *db.EventStore, store *state.Store) int {
	blobs, err := eventStore.LoadAll()
	if err != nil {
		log.Printf("warning: failed to load events from db: %v", err)
		return 0
	}

	replayed := 0
	for _, raw := range blobs {
		var event state.WebhookEvent
		if err := json.Unmarshal(raw, &event); err != nil {
			log.Printf("warning: skipping invalid event during replay: %v", err)
			continue
		}
		store.HandleEvent(event)
		replayed++
	}
	return replayed
}

// loadDAGs retrieves all stored DAG DOT snapshots from the database,
// parses each into a Layout, and injects them into the server so that
// DAG views are available immediately after restart without needing
// the filesystem.
func loadDAGs(eventStore *db.EventStore, srv *server.Server) int {
	records, err := eventStore.LoadAllDAGs()
	if err != nil {
		log.Printf("warning: failed to load DAGs from db: %v", err)
		return 0
	}

	loaded := 0
	for _, rec := range records {
		d, err := dag.ParseDOT(bytes.NewReader(rec.DotText))
		if err != nil {
			log.Printf("warning: skipping unparseable DAG for run %s: %v", rec.RunID, err)
			continue
		}
		layout := dag.ComputeLayout(d)
		srv.SetLayout(rec.RunID, layout)
		loaded++
	}
	return loaded
}

// generateSelfSignedCert creates an in-memory self-signed TLS certificate
// for localhost. This enables HTTP/2 so the browser can multiplex many SSE
// streams over a single TCP connection.
func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate RSA key: %w", err)
	}

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generate serial number: %w", err)
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               pkix.Name{Organization: []string{"nextflow-monitor"}},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
		DNSNames:              []string{"localhost"},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("marshal private key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})

	return tls.X509KeyPair(certPEM, keyPEM)
}

// webhookOnly wraps a handler so the HTTP listener only serves /webhook POSTs.
// Any other request gets a small page redirecting the user to the HTTPS dashboard.
func webhookOnly(handler http.Handler, host string, tlsPort int) http.Handler {
	dashURL := fmt.Sprintf("https://%s:%d", host, tlsPort)
	redirectPage := fmt.Sprintf(`<!DOCTYPE html>
<html><head><meta charset="UTF-8"><title>nextflow-monitor</title></head>
<body style="font-family:system-ui;display:flex;justify-content:center;align-items:center;height:100vh;margin:0;background:#1a1a2e;color:#e0e0e0">
<div style="text-align:center">
<h1>⬡ nextflow-monitor</h1>
<p>This port accepts webhook events only.</p>
<p>Open the dashboard at <a href="%s" style="color:#f59e0b">%s</a></p>
</div></body></html>`, dashURL, dashURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/webhook" {
			handler.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusMisdirectedRequest)
		fmt.Fprint(w, redirectPage)
	})
}

func main() {
	// main wires together the Store, EventStore, and Server, then starts HTTP.
	host := flag.String("host", "localhost", "host to bind to")
	port := flag.Int("port", 8080, "port to listen on")
	dbPath := flag.String("db", dbDefault(), "path to SQLite database for event persistence")
	flag.Parse()

	eventStore, err := db.OpenEventStore(*dbPath)
	if err != nil {
		log.Fatalf("failed to open event store at %s: %v", *dbPath, err)
	}
	defer eventStore.Close()

	store := state.NewStore()

	// Replay persisted events to rebuild in-memory state.
	count := replayEvents(eventStore, store)
	log.Printf("replayed %d events from %s", count, *dbPath)

	srv := server.NewServer(store, eventStore)

	// Restore DAG layouts from database so DAG views work after restart.
	dagCount := loadDAGs(eventStore, srv)
	if dagCount > 0 {
		log.Printf("loaded %d DAG layouts from %s", dagCount, *dbPath)
	}

	httpAddr := buildAddr(*host, *port)
	tlsPort := *port + 1
	tlsAddr := buildAddr(*host, tlsPort)

	tlsCert, err := generateSelfSignedCert()
	if err != nil {
		log.Fatalf("failed to generate TLS certificate: %v", err)
	}

	// HTTP listener — accepts webhook POSTs, redirects browsers to HTTPS.
	httpServer := &http.Server{
		Addr:    httpAddr,
		Handler: webhookOnly(srv, *host, tlsPort),
	}

	// HTTPS listener — for browser (HTTP/2 multiplexes all SSE streams over one connection).
	tlsServer := &http.Server{
		Addr:    tlsAddr,
		Handler: srv,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{tlsCert},
		},
	}

	log.Printf("webhook endpoint: http://%s/webhook", httpAddr)
	log.Printf("dashboard:        https://%s", tlsAddr)

	// Start HTTP listener in background, HTTPS in foreground.
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP listener failed: %v", err)
		}
	}()
	log.Fatal(tlsServer.ListenAndServeTLS("", ""))
}

