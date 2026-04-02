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

	// Create hidden_runs table for soft-delete (hide from sidebar, undo-able).
	const createHiddenRunsTable = `CREATE TABLE IF NOT EXISTS hidden_runs (
		run_id    TEXT PRIMARY KEY,
		hidden_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := db.Exec(createHiddenRunsTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create hidden_runs table: %w", err)
	}

	// Create dag_dots table for per-run DAG snapshots.
	const createDAGTable = `CREATE TABLE IF NOT EXISTS dag_dots (
		run_id       TEXT PRIMARY KEY,
		project_name TEXT NOT NULL,
		dot_text     BLOB NOT NULL,
		created_at   DATETIME DEFAULT CURRENT_TIMESTAMP
	)`
	if _, err := db.Exec(createDAGTable); err != nil {
		db.Close()
		return nil, fmt.Errorf("create dag_dots table: %w", err)
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

// DAGRecord holds a stored DAG DOT snapshot for a single run.
type DAGRecord struct {
	RunID       string
	ProjectName string
	DotText     []byte
}

// SaveDAG persists a raw DAG DOT snapshot for the given run, using INSERT OR REPLACE
// so re-runs with the same runID update the stored DAG.
func (es *EventStore) SaveDAG(runID, projectName string, dotText []byte) error {
	_, err := es.db.Exec(
		"INSERT OR REPLACE INTO dag_dots (run_id, project_name, dot_text) VALUES (?, ?, ?)",
		runID, projectName, dotText,
	)
	return err
}

// LoadAllDAGs returns all stored DAG DOT snapshots, ordered by creation time.
// Used on startup to rebuild in-memory DAG layouts from the database.
func (es *EventStore) LoadAllDAGs() ([]DAGRecord, error) {
	rows, err := es.db.Query("SELECT run_id, project_name, dot_text FROM dag_dots ORDER BY created_at ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recs []DAGRecord
	for rows.Next() {
		var r DAGRecord
		if err := rows.Scan(&r.RunID, &r.ProjectName, &r.DotText); err != nil {
			return nil, err
		}
		recs = append(recs, r)
	}
	return recs, rows.Err()
}

// HideRun marks a run as hidden (soft-delete). Uses INSERT OR IGNORE so
// hiding an already-hidden run is a no-op.
func (es *EventStore) HideRun(runID string) error {
	_, err := es.db.Exec("INSERT OR IGNORE INTO hidden_runs (run_id) VALUES (?)", runID)
	return err
}

// UnhideRun removes a run from the hidden set, restoring it to the sidebar.
func (es *EventStore) UnhideRun(runID string) error {
	_, err := es.db.Exec("DELETE FROM hidden_runs WHERE run_id = ?", runID)
	return err
}

// LoadHiddenRuns returns all run IDs that have been soft-deleted.
// Used on startup to restore the hidden set in memory.
func (es *EventStore) LoadHiddenRuns() ([]string, error) {
	rows, err := es.db.Query("SELECT run_id FROM hidden_runs")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Close closes the underlying database connection.
func (es *EventStore) Close() error {
	return es.db.Close()
}
