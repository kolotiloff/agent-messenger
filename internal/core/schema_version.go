package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// SchemaVersion is the newest database layout this client understands.
// Version 1 is the first recorded one: Codex setups and turns, and routed
// replies. Databases from before it have no version and count as 0.
const SchemaVersion = 1

// ErrNewerSchema means the database was written by a newer client. This
// client stops before changing anything in it.
var ErrNewerSchema = errors.New("the data directory was written by a newer Agent Messenger; update this client")

// checkSchemaVersion runs before any other schema statement.
func checkSchemaVersion(db *sql.DB) error {
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return err
	}
	var raw string
	err := db.QueryRow(`SELECT value FROM meta WHERE key='schema_version'`).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	version, err := strconv.Atoi(raw)
	if err != nil || version < 0 {
		return fmt.Errorf("%w: schema version is damaged", ErrRejected)
	}
	if version > SchemaVersion {
		return fmt.Errorf("%w (database %d, client %d)", ErrNewerSchema, version, SchemaVersion)
	}
	return nil
}

// recordSchemaVersion runs after all migrations succeeded.
func recordSchemaVersion(db *sql.DB) error {
	_, err := db.Exec(`INSERT INTO meta(key,value) VALUES ('schema_version',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value WHERE CAST(meta.value AS INTEGER) < CAST(excluded.value AS INTEGER)`, strconv.Itoa(SchemaVersion))
	return err
}
