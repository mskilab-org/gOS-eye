// Entry point for nextflow-monitor.
// Creates Store, creates Server, listens on the configured host:port.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/mskilab-org/nextflow-monitor/internal/server"
	"github.com/mskilab-org/nextflow-monitor/internal/state"
)

// buildAddr constructs a listen address from host and port.
func buildAddr(host string, port int) string {
	return fmt.Sprintf("%s:%d", host, port)
}

func main() {
	// main wires together the Store and Server, then starts HTTP.
	host := flag.String("host", "localhost", "host to bind to")
	port := flag.Int("port", 8080, "port to listen on")
	flag.Parse()

	store := state.NewStore()
	srv := server.NewServer(store)
	addr := buildAddr(*host, *port)
	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, srv))
}
