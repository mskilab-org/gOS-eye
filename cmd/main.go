// Entry point for nextflow-monitor.
// Creates Store, creates Server, listens on the configured host:port.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

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
	addr := buildAddr(*host, *port)
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv))
}

