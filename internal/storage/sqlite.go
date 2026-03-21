package storage

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite" // register "sqlite" driver
)

const schema = `
CREATE TABLE IF NOT EXISTS deployments (
    id             TEXT    PRIMARY KEY,
    service        TEXT    NOT NULL,
    status         TEXT    NOT NULL,
    mode           TEXT    NOT NULL DEFAULT '',
    image          TEXT    NOT NULL DEFAULT '',
    previous_image TEXT    NOT NULL DEFAULT '',
    rollback_tag   TEXT    NOT NULL DEFAULT '',
    started_at     INTEGER NOT NULL,
    completed_at   INTEGER,
    error_message  TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_deployments_service ON deployments(service);
CREATE TABLE IF NOT EXISTS deployment_logs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    deployment_id TEXT    NOT NULL,
    line          TEXT    NOT NULL,
    logged_at     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_logs_deployment_id ON deployment_logs(deployment_id);
`

// SQLiteStorage is a persistent Storage implementation backed by SQLite.
// All mutations are serialised through a mutex so the single WAL writer
// constraint is respected even when multiple goroutines call concurrently.
type SQLiteStorage struct {
	mu sync.Mutex
	db *sql.DB
}

// NewSQLiteStorage opens (or creates) the SQLite database at path and applies
// the schema. The caller must call Close when finished.
func NewSQLiteStorage(path string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db at %q: %w", path, err)
	}

	// Apply performance and safety PRAGMAs.
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close() //nolint:errcheck // best-effort cleanup
			return nil, fmt.Errorf("apply pragma %q: %w", p, err)
		}
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close() //nolint:errcheck
		return nil, fmt.Errorf("create schema in %q: %w", path, err)
	}

	return &SQLiteStorage{db: db}, nil
}

// Save inserts or replaces the deployment record.
func (s *SQLiteStorage) Save(d *Deployment) error {
	var completedAt sql.NullInt64
	if d.CompletedAt != nil {
		completedAt = sql.NullInt64{Int64: d.CompletedAt.UnixNano(), Valid: true}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`
		INSERT OR REPLACE INTO deployments
		    (id, service, status, mode, image, previous_image, rollback_tag,
		     started_at, completed_at, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		d.ID, d.Service, string(d.Status), d.Mode, d.Image,
		d.PreviousImage, d.RollbackTag,
		d.StartedAt.UnixNano(), completedAt, d.ErrorMessage,
	)
	if err != nil {
		return fmt.Errorf("save deployment %q: %w", d.ID, err)
	}
	return nil
}

// Update reads the deployment with id, applies fn, and persists the result.
// The read-modify-write is serialised under the mutex to prevent lost updates.
func (s *SQLiteStorage) Update(id string, fn func(*Deployment)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	d, err := s.get(id)
	if err != nil {
		return err
	}

	fn(d)

	var completedAt sql.NullInt64
	if d.CompletedAt != nil {
		completedAt = sql.NullInt64{Int64: d.CompletedAt.UnixNano(), Valid: true}
	}

	_, err = s.db.Exec(`
		UPDATE deployments SET
		    service        = ?,
		    status         = ?,
		    mode           = ?,
		    image          = ?,
		    previous_image = ?,
		    rollback_tag   = ?,
		    started_at     = ?,
		    completed_at   = ?,
		    error_message  = ?
		WHERE id = ?`,
		d.Service, string(d.Status), d.Mode, d.Image,
		d.PreviousImage, d.RollbackTag,
		d.StartedAt.UnixNano(), completedAt, d.ErrorMessage,
		id,
	)
	if err != nil {
		return fmt.Errorf("update deployment %q: %w", id, err)
	}
	return nil
}

// Get returns the deployment with the given id.
func (s *SQLiteStorage) Get(id string) (*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.get(id)
}

// get is the internal unlocked read. Callers must hold s.mu.
func (s *SQLiteStorage) get(id string) (*Deployment, error) {
	row := s.db.QueryRow(`
		SELECT id, service, status, mode, image, previous_image, rollback_tag,
		       started_at, completed_at, error_message
		FROM deployments WHERE id = ?`, id)

	d, err := scanDeployment(row)
	if err != nil {
		return nil, err
	}

	logs, err := s.getLogs(d.ID)
	if err != nil {
		return nil, err
	}
	d.Logs = logs
	return d, nil
}

// getLogs returns all log lines for the given deployment id, ordered by id ASC.
// Callers must hold s.mu.
func (s *SQLiteStorage) getLogs(deploymentID string) ([]string, error) {
	rows, err := s.db.Query(`
		SELECT line FROM deployment_logs
		WHERE deployment_id = ? ORDER BY id ASC`, deploymentID)
	if err != nil {
		return nil, fmt.Errorf("query logs for deployment %q: %w", deploymentID, err)
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			return nil, fmt.Errorf("scan log line for deployment %q: %w", deploymentID, err)
		}
		lines = append(lines, line)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate log rows for deployment %q: %w", deploymentID, err)
	}
	return lines, nil
}

// List returns all deployments ordered by started_at descending.
func (s *SQLiteStorage) List() ([]*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rows, err := s.db.Query(`
		SELECT id, service, status, mode, image, previous_image, rollback_tag,
		       started_at, completed_at, error_message
		FROM deployments ORDER BY started_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	defer rows.Close()

	var result []*Deployment
	for rows.Next() {
		d, err := scanDeployment(rows)
		if err != nil {
			return nil, fmt.Errorf("list deployments: scan row: %w", err)
		}
		logs, err := s.getLogs(d.ID)
		if err != nil {
			return nil, fmt.Errorf("list deployments: fetch logs for %q: %w", d.ID, err)
		}
		d.Logs = logs
		result = append(result, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list deployments: rows iteration: %w", err)
	}
	return result, nil
}

// GetLatestByService returns the most recently started deployment for the given
// service, or ErrNotFound if no deployments exist for that service.
func (s *SQLiteStorage) GetLatestByService(service string) (*Deployment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	row := s.db.QueryRow(`
		SELECT id, service, status, mode, image, previous_image, rollback_tag,
		       started_at, completed_at, error_message
		FROM deployments WHERE service = ? ORDER BY started_at DESC LIMIT 1`, service)

	d, err := scanDeployment(row)
	if err != nil {
		if err.Error() == "deployment not found" {
			return nil, fmt.Errorf("service %q: %w", service, ErrNotFound)
		}
		return nil, fmt.Errorf("get latest deployment for service %q: %w", service, err)
	}
	return d, nil
}

// AppendLog inserts a log line for the deployment with the given id.
// Returns an error if the id does not exist.
func (s *SQLiteStorage) AppendLog(id string, line string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify the deployment exists before inserting.
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM deployments WHERE id = ?`, id).Scan(&count); err != nil {
		return fmt.Errorf("check deployment %q for AppendLog: %w", id, err)
	}
	if count == 0 {
		return fmt.Errorf("deployment %q not found", id)
	}

	_, err := s.db.Exec(`
		INSERT INTO deployment_logs (deployment_id, line, logged_at) VALUES (?, ?, ?)`,
		id, line, time.Now().UnixNano(),
	)
	if err != nil {
		return fmt.Errorf("append log to deployment %q: %w", id, err)
	}
	return nil
}

// Close releases the database connection.
func (s *SQLiteStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// scanner is implemented by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanDeployment(s scanner) (*Deployment, error) {
	var (
		d           Deployment
		status      string
		startedNs   int64
		completedNs sql.NullInt64
	)
	err := s.Scan(
		&d.ID, &d.Service, &status, &d.Mode, &d.Image,
		&d.PreviousImage, &d.RollbackTag,
		&startedNs, &completedNs, &d.ErrorMessage,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("deployment not found")
	}
	if err != nil {
		return nil, err
	}

	d.Status = Status(status)
	d.StartedAt = time.Unix(0, startedNs)
	if completedNs.Valid {
		t := time.Unix(0, completedNs.Int64)
		d.CompletedAt = &t
	}
	return &d, nil
}
