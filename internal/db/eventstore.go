// Package db provides persistent storage for raw webhook events using SQLite.
// Events are stored write-ahead and replayed on startup to rebuild in-memory state.
package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// EventStore wraps a SQLite database for persisting raw webhook JSON events.
// It uses modernc.org/sqlite (pure-Go, no CGo) for HPC compatibility.
type EventStore struct {
	db *sql.DB
}

// OpenEventStore opens (or creates) a SQLite database at dbPath, creates the
// events table if it doesn't exist, and returns an EventStore ready for use.
func OpenEventStore(dbPath string) (*EventStore, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Verify the connection works (catches bad paths early).
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	// Enable WAL journal mode for concurrent reads.
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Create events table if it doesn't already exist.
	const createTable = `CREATE TABLE IF NOT EXISTS events (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		raw_json   BLOB NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := db.Exec(createTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create events table: %w", err)
	}

	return &EventStore{db: db}, nil
}

// Save persists a raw webhook JSON event to the database.
// Each event gets an auto-increment id and insertion timestamp.
func (es *EventStore) Save(rawJSON []byte) error {
	_, err := es.db.Exec("INSERT INTO events (raw_json) VALUES (?)", rawJSON)
	return err
}

// LoadAll returns all stored events ordered by insertion id.
// Used on startup to replay events and rebuild in-memory state.
func (es *EventStore) LoadAll() ([][]byte, error) {
	rows, err := es.db.Query("SELECT raw_json FROM events ORDER BY id ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blobs [][]byte
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		blobs = append(blobs, b)
	}
	return blobs, rows.Err()
}

// Close closes the underlying database connection.
func (es *EventStore) Close() error {
	return es.db.Close()
}
