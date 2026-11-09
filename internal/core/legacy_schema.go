package core

import "database/sql"

// The tables below belonged to the old "message a live session" mode: the
// session queue, the persistent Claude stream, the Codex app-server owner and
// the window's message outbox. That mode was removed in T-028 (D-104).
//
// Nothing reads or writes these tables any more. They are still created and
// migrated, exactly as before, so that a database written by an older version
// opens unchanged and keeps its history, and a new database has the same
// shape. Do not drop or rewrite them: they hold the owner's old messages, and
// old reservations in active-runs.json point at their rows.

const legacyQueueSchema = `
CREATE TABLE IF NOT EXISTS sessions (
    key TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    environment TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    active_token TEXT,
    decision TEXT,
    cli_path TEXT,
    workspace TEXT,
    UNIQUE(provider, environment, conversation_id)
);
CREATE TABLE IF NOT EXISTS aliases (
    alias TEXT PRIMARY KEY,
    session_key TEXT NOT NULL REFERENCES sessions(key)
);
CREATE TABLE IF NOT EXISTS requests (
    id TEXT PRIMARY KEY,
    session_key TEXT NOT NULL REFERENCES sessions(key),
    sender TEXT NOT NULL,
    text TEXT NOT NULL,
    state TEXT NOT NULL,
    retry_of TEXT,
    created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS attempts (
    token TEXT PRIMARY KEY,
    session_key TEXT NOT NULL REFERENCES sessions(key),
    request_id TEXT NOT NULL UNIQUE REFERENCES requests(id),
    alias TEXT NOT NULL,
    state TEXT NOT NULL,
    pid INTEGER,
    pgid INTEGER,
    start_token TEXT,
    boot_id TEXT,
    mcp_revoked INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS replies (
    id TEXT PRIMARY KEY,
    reply_to TEXT NOT NULL UNIQUE REFERENCES requests(id),
    attempt_token TEXT NOT NULL,
    sender TEXT NOT NULL,
    recipient TEXT NOT NULL,
    text TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS verifications (
    attempt_token TEXT PRIMARY KEY,
    identity_confirmed INTEGER NOT NULL,
    reply_id TEXT,
    exit_observed INTEGER NOT NULL,
    exit_code INTEGER,
    group_empty INTEGER NOT NULL,
    provider_ok INTEGER NOT NULL DEFAULT 0,
    detail TEXT NOT NULL
);
`

const legacyStreamSchema = `
CREATE TABLE IF NOT EXISTS stream_runs (
 id TEXT PRIMARY KEY REFERENCES attempts(token),
 session_key TEXT NOT NULL UNIQUE REFERENCES sessions(key),
 state TEXT NOT NULL,
 current_token TEXT,
 exit_observed INTEGER NOT NULL DEFAULT 0,
 exit_code INTEGER
);
CREATE TABLE IF NOT EXISTS stream_turns (
 attempt_token TEXT PRIMARY KEY REFERENCES attempts(token),
 run_id TEXT NOT NULL REFERENCES stream_runs(id),
 receipt TEXT NOT NULL UNIQUE,
 sent_intent INTEGER NOT NULL DEFAULT 0,
 read_seen INTEGER NOT NULL DEFAULT 0
);
`

const legacyStreamRecoverySchema = `
CREATE TABLE IF NOT EXISTS archived_stream_runs (
 id TEXT PRIMARY KEY, session_key TEXT NOT NULL, state TEXT NOT NULL,
 current_token TEXT, exit_observed INTEGER NOT NULL, exit_code INTEGER
);
CREATE TABLE IF NOT EXISTS archived_stream_turns (
 attempt_token TEXT PRIMARY KEY, run_id TEXT NOT NULL, receipt TEXT NOT NULL,
 sent_intent INTEGER NOT NULL, read_seen INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS stream_recoveries (
 run_id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL, phase TEXT NOT NULL,
 proof TEXT NOT NULL, reservation TEXT NOT NULL
);
`

const legacyUIOutboxSchema = `CREATE TABLE IF NOT EXISTS ui_outbox (
 id TEXT PRIMARY KEY,
 session_key TEXT NOT NULL REFERENCES sessions(key),
 alias TEXT NOT NULL,
 text TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('prepared','queued','acknowledged')),
 created_at TEXT NOT NULL
);`

const legacyCodexSetupSchema = `
CREATE TABLE IF NOT EXISTS codex_setups (
 id TEXT PRIMARY KEY,
 alias TEXT NOT NULL,
 environment TEXT NOT NULL,
 cli_path TEXT NOT NULL,
 workspace TEXT NOT NULL,
 state TEXT NOT NULL,
 config_fingerprint TEXT,
 thread_id TEXT UNIQUE,
 pid INTEGER,
 pgid INTEGER,
 start_token TEXT,
 boot_id TEXT,
 exit_observed INTEGER NOT NULL DEFAULT 0,
 exit_code INTEGER,
 detail TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
DROP INDEX IF EXISTS codex_setups_active_alias;
CREATE UNIQUE INDEX IF NOT EXISTS codex_setups_alias_v2 ON codex_setups(alias) WHERE state NOT IN ('not_started','retired');
CREATE TABLE IF NOT EXISTS codex_turns (
 attempt_token TEXT PRIMARY KEY REFERENCES attempts(token),
 setup_id TEXT NOT NULL REFERENCES codex_setups(id),
 receipt TEXT NOT NULL UNIQUE,
 sent_intent INTEGER NOT NULL DEFAULT 0,
 turn_id TEXT UNIQUE,
 read_seen INTEGER NOT NULL DEFAULT 0
);
`

const legacySchema = legacyQueueSchema + legacyStreamSchema + legacyStreamRecoverySchema + legacyUIOutboxSchema + legacyCodexSetupSchema

// migrateLegacySchema adds the columns older versions added to the old
// tables, so a database from any earlier version reaches the same shape.
func migrateLegacySchema(db *sql.DB) error {
	for _, c := range [][3]string{
		{"sessions", "cli_path", "TEXT"},
		{"sessions", "workspace", "TEXT"},
		{"verifications", "provider_ok", "INTEGER NOT NULL DEFAULT 0"},
		{"sessions", "start_mode", "TEXT NOT NULL DEFAULT 'resume'"},
		{"replies", "held", "INTEGER NOT NULL DEFAULT 0"},
		{"requests", "in_reply_to", "TEXT"},
		{"requests", "hops", "INTEGER NOT NULL DEFAULT 0"},
		{"codex_setups", "session_key", "TEXT REFERENCES sessions(key)"},
		{"codex_setups", "recovery_fingerprint", "TEXT"},
		{"codex_setups", "recovery_reservation", "TEXT"},
	} {
		if err := ensureColumn(db, c[0], c[1], c[2]); err != nil {
			return err
		}
	}
	return nil
}
