package core

import (
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// schema holds the tables shared by every part of the client: the event log
// and the pause switch. Tasks, the network and the old mode's tables have
// their own schema constants.
const schema = `
CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    at TEXT NOT NULL,
    kind TEXT NOT NULL,
    session_key TEXT,
    attempt_token TEXT,
    detail TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS control (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    paused INTEGER NOT NULL
);
INSERT OR IGNORE INTO control(id, paused) VALUES (1, 0);
`

// Store is the durable state of one user client: projects, tasks and their
// runs, the network, and the old mode's history (legacy_schema.go).
type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db}
	if err := checkSchemaVersion(db); err != nil {
		db.Close()
		return nil, err
	}
	if _, err := db.Exec(legacySchema + schema + taskSchema + taskRunItemsSchema + taskOutputSchema + networkSchema + netTasksSchema + netLogsSchema + netUpdatesSchema); err != nil {
		db.Close()
		return nil, err
	}
	if err := migrateLegacySchema(db); err != nil {
		db.Close()
		return nil, err
	}
	for _, c := range [][3]string{
		{"coordinator_connections", "client", "TEXT"},
		{"task_runs", "launched_at", "TEXT"},
		{"tasks", "model", "TEXT"},
		{"computer_agents", "models", "TEXT NOT NULL DEFAULT ''"},
		{"computer_agents", "limit_state", "TEXT NOT NULL DEFAULT ''"},
		{"computer_agents", "full_access", "INTEGER NOT NULL DEFAULT 0"},
		{"computer_agents", "paused", "INTEGER NOT NULL DEFAULT 0"},
		{"projects", "removed_at", "TEXT"},
		{"tasks", "full_access", "INTEGER NOT NULL DEFAULT 0"},
		{"tasks", "failure_reason", "TEXT"},
		{"tasks", "failure_reset", "TEXT"},
		{"tasks", "usage_seen", "INTEGER NOT NULL DEFAULT 0"},
		{"tasks", "usage_input", "INTEGER NOT NULL DEFAULT 0"},
		{"tasks", "usage_cached", "INTEGER NOT NULL DEFAULT 0"},
		{"tasks", "usage_cache_write", "INTEGER NOT NULL DEFAULT 0"},
		{"tasks", "usage_output", "INTEGER NOT NULL DEFAULT 0"},
		{"tasks", "usage_cost", "REAL"},
		{"projects", "worker_rules", "TEXT NOT NULL DEFAULT ''"},
		{"projects", "title", "TEXT NOT NULL DEFAULT ''"},
		{"tasks", "rules", "TEXT NOT NULL DEFAULT ''"},
		{"task_runs", "op_id", "TEXT"},
		{"net_peers", "ready", "TEXT NOT NULL DEFAULT ''"},
		{"net_peers", "usage", "TEXT NOT NULL DEFAULT ''"},
		{"net_peers", "missing_since", "TEXT NOT NULL DEFAULT ''"},
		{"net_peers", "version", "TEXT NOT NULL DEFAULT ''"},
		{"tasks", "resource", "TEXT NOT NULL DEFAULT ''"},
		networkTaskColumns[0], networkTaskColumns[1], networkTaskColumns[2],
		networkTaskColumns[3], networkTaskColumns[4], networkTaskColumns[5],
	} {
		if err := ensureColumn(db, c[0], c[1], c[2]); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := recordSchemaVersion(db); err != nil {
		db.Close()
		return nil, err
	}
	_ = os.Chmod(path, 0o600)
	return store, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	err := s.db.Close()
	s.db = nil
	return err
}

func dsn(path string) string {
	// SQLite reads file:///C:/dir/db on Windows; a drive letter must not
	// become the URL's host.
	if filepath.VolumeName(path) != "" {
		path = "/" + filepath.ToSlash(path)
	}
	u := url.URL{Scheme: "file", Path: path}
	q := url.Values{}
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(FULL)")
	q.Add("_txlock", "immediate")
	u.RawQuery = q.Encode()
	return u.String()
}

func (s *Store) tx(fn func(*sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

// Pause stops new task runs from starting; a running worker is not touched.
func (s *Store) Pause() error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE control SET paused = 1 WHERE id = 1`); err != nil {
			return err
		}
		return insertEvent(tx, "pause", "", "", "new claims stopped; active process untouched")
	})
}

func (s *Store) ResumeWork() error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE control SET paused = 0 WHERE id = 1`); err != nil {
			return err
		}
		return insertEvent(tx, "resume", "", "", "new claims allowed")
	})
}

func (s *Store) Paused() (bool, error) {
	var paused bool
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		paused, err = isPaused(tx)
		return err
	})
	return paused, err
}

func isPaused(tx *sql.Tx) (bool, error) {
	var value int
	err := tx.QueryRow(`SELECT paused FROM control WHERE id = 1`).Scan(&value)
	return value != 0, err
}

func insertEvent(tx *sql.Tx, kind, sessionKey, token, detail string) error {
	_, err := tx.Exec(`INSERT INTO events(at, kind, session_key, attempt_token, detail) VALUES (?,?,?,?,?)`,
		now(), kind, sessionKey, token, detail)
	return err
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullString(v sql.NullString) any {
	if v.Valid {
		return v.String
	}
	return nil
}

func ensureColumn(db *sql.DB, table, column, decl string) error {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid, notNull, pk int
		var name, ctype string
		var dflt any
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = db.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + decl)
	return err
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }
