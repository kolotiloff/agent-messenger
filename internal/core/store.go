package core

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/platform"

	_ "modernc.org/sqlite"
)

const schema = `
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

// Store is the durable queue and session registry for one user client.
type Store struct {
	db *sql.DB
}

type Message struct {
	ID   string `json:"id"`
	From string `json:"from"`
	Text string `json:"text"`
}

type Reply struct {
	ID        string `json:"id"`
	ReplyTo   string `json:"reply_to"`
	Sender    string `json:"sender"`
	Recipient string `json:"recipient"`
	Text      string `json:"text"`
}

type Row map[string]any

type Snapshot struct {
	Sessions           []Row
	Requests           []Row
	Attempts           []Row
	Replies            []Row
	Events             []Row
	Paused             bool
	StreamRuns         []Row
	Verifications      []Row
	CodexSetups        []Row
	ArchivedStreamRuns []Row
	StreamRecoveries   []Row
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
	if _, err := db.Exec(schema + streamSchema + streamRecoverySchema + uiOutboxSchema + codexSetupSchema + taskSchema + taskRunItemsSchema + taskOutputSchema + networkSchema + netTasksSchema + netLogsSchema + netUpdatesSchema); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureSessionColumns(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureColumn(db, "verifications", "provider_ok", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureColumn(db, "sessions", "start_mode", "TEXT NOT NULL DEFAULT 'resume'"); err != nil {
		db.Close()
		return nil, err
	}
	for _, c := range [][3]string{
		{"replies", "held", "INTEGER NOT NULL DEFAULT 0"},
		{"coordinator_connections", "client", "TEXT"},
		{"task_runs", "launched_at", "TEXT"},
		{"tasks", "model", "TEXT"},
		{"tasks", "ran_model", "TEXT"},
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
		{"requests", "in_reply_to", "TEXT"},
		{"projects", "worker_rules", "TEXT NOT NULL DEFAULT ''"},
		{"projects", "title", "TEXT NOT NULL DEFAULT ''"},
		{"tasks", "rules", "TEXT NOT NULL DEFAULT ''"},
		{"task_runs", "op_id", "TEXT"},
		{"requests", "hops", "INTEGER NOT NULL DEFAULT 0"},
		{"net_peers", "ready", "TEXT NOT NULL DEFAULT ''"},
		{"net_peers", "usage", "TEXT NOT NULL DEFAULT ''"},
		{"net_peers", "missing_since", "TEXT NOT NULL DEFAULT ''"},
		{"net_peers", "version", "TEXT NOT NULL DEFAULT ''"},
		{"tasks", "resource", "TEXT NOT NULL DEFAULT ''"},
		{"task_owner_notes", "author", "TEXT NOT NULL DEFAULT 'owner'"},
		networkTaskColumns[0], networkTaskColumns[1], networkTaskColumns[2],
		networkTaskColumns[3], networkTaskColumns[4], networkTaskColumns[5],
		networkTaskColumns[6],
	} {
		if err := ensureColumn(db, c[0], c[1], c[2]); err != nil {
			db.Close()
			return nil, err
		}
	}
	for column, decl := range map[string]string{
		"session_key":          "TEXT REFERENCES sessions(key)",
		"recovery_fingerprint": "TEXT",
		"recovery_reservation": "TEXT",
	} {
		if err := ensureColumn(db, "codex_setups", column, decl); err != nil {
			db.Close()
			return nil, err
		}
	}
	if err := fillDefaultModels(db); err != nil {
		db.Close()
		return nil, err
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

func (s *Store) Register(provider, environment, conversationID, alias string) (string, error) {
	if strings.TrimSpace(alias) == "" {
		return "", fmt.Errorf("%w: alias is required", ErrRejected)
	}
	key, err := SessionKey(provider, environment, conversationID)
	if err != nil {
		return "", err
	}
	environment, err = CanonicalEnvironment(environment)
	if err != nil {
		return "", err
	}
	provider, err = canonicalProvider(provider)
	if err != nil {
		return "", err
	}
	err = s.tx(func(tx *sql.Tx) error {
		var existingAlias string
		err := tx.QueryRow(`SELECT session_key FROM aliases WHERE alias = ?`, alias).Scan(&existingAlias)
		if err == nil && existingAlias != key {
			return fmt.Errorf("%w: alias is bound to another session", ErrRejected)
		}
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if reserved, err := aliasReservedBySetup(tx, alias); err != nil || reserved {
			if err == nil {
				err = fmt.Errorf("%w: alias is reserved by a Codex session setup", ErrRejected)
			}
			return err
		}
		if owned, err := threadOwnedBySetup(tx, provider, environment, conversationID); err != nil || owned {
			if err == nil {
				err = fmt.Errorf("%w: thread is owned by a Codex session setup", ErrRejected)
			}
			return err
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO sessions(key, provider, environment, conversation_id) VALUES (?,?,?,?)`,
			key, provider, environment, conversationID); err != nil {
			return err
		}
		var gotProvider, gotEnv, gotID string
		if err := tx.QueryRow(`SELECT provider, environment, conversation_id FROM sessions WHERE key = ?`, key).
			Scan(&gotProvider, &gotEnv, &gotID); err != nil {
			return err
		}
		if gotProvider != provider || gotEnv != environment || gotID != conversationID {
			return fmt.Errorf("%w: session identity cannot be rebound", ErrRejected)
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO aliases(alias, session_key) VALUES (?,?)`, alias, key); err != nil {
			return err
		}
		return insertEvent(tx, "register", key, "", "alias "+alias)
	})
	if err != nil {
		return "", err
	}
	return key, nil
}

func (s *Store) Enqueue(messageID, alias, sender, text string) (bool, error) {
	if strings.TrimSpace(messageID) == "" || strings.TrimSpace(sender) == "" || text == "" {
		return false, fmt.Errorf("%w: invalid envelope", ErrRejected)
	}
	created := false
	err := s.tx(func(tx *sql.Tx) error {
		key, decision, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		if decision == "closed_without_retry" {
			return fmt.Errorf("%w: session is closed without retry", ErrClosed)
		}
		var oldKey, oldSender, oldText string
		err = tx.QueryRow(`SELECT session_key, sender, text FROM requests WHERE id = ?`, messageID).
			Scan(&oldKey, &oldSender, &oldText)
		if err == nil {
			if oldKey != key || oldSender != sender || oldText != text {
				return fmt.Errorf("%w: message id reused with different content or route", ErrRejected)
			}
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO requests(id, session_key, sender, text, state, created_at) VALUES (?,?,?,?,?,?)`,
			messageID, key, sender, text, "queued", now()); err != nil {
			return err
		}
		created = true
		return insertEvent(tx, "enqueue", key, "", messageID)
	})
	return created, err
}

// Claim reserves one queued request for this alias. The reservation is committed
// before any process is created. A paused client, a busy session and an empty queue
// do not reserve anything.
func (s *Store) Claim(alias string) (string, error) {
	return s.ClaimProvider(alias, "")
}

// ClaimProvider restricts selection before reserving anything. A missing alias,
// wrong provider or incomplete launch configuration never falls back to another queue.
func (s *Store) ClaimProvider(alias, provider string) (string, error) {
	var token string
	err := s.tx(func(tx *sql.Tx) error {
		paused, err := paused(tx)
		if err != nil {
			return err
		}
		if paused {
			return ErrPaused
		}
		key, decision, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		if provider != "" {
			var matches int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE key=? AND provider=? AND COALESCE(cli_path,'')!=''`, key, provider).Scan(&matches); err != nil {
				return err
			}
			if matches != 1 {
				return ErrRejected
			}
		}
		if decision == "closed_without_retry" {
			return fmt.Errorf("%w: session is closed without retry", ErrClosed)
		}
		if owned, err := codexOwnedSession(tx, key); err != nil || owned {
			if err == nil {
				err = fmt.Errorf("%w: session is served by its Codex owner", ErrRejected)
			}
			return err
		}
		var active sql.NullString
		if err := tx.QueryRow(`SELECT active_token FROM sessions WHERE key = ?`, key).Scan(&active); err != nil {
			return err
		}
		if active.Valid {
			return ErrBusy
		}
		var requestID string
		err = tx.QueryRow(`SELECT id FROM requests WHERE session_key = ? AND state = 'queued' ORDER BY rowid LIMIT 1`, key).Scan(&requestID)
		if err == sql.ErrNoRows {
			return ErrEmpty
		}
		if err != nil {
			return err
		}
		token, err = reserve(tx, key, requestID, alias)
		return err
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

// ClaimNext reserves the oldest queued request of any free session.
func (s *Store) ClaimNext() (string, error) { return s.ClaimNextProvider("") }

func (s *Store) ClaimNextProvider(provider string) (string, error) {
	var token string
	err := s.tx(func(tx *sql.Tx) error {
		paused, err := paused(tx)
		if err != nil {
			return err
		}
		if paused {
			return ErrPaused
		}
		var requestID, key, alias string
		err = tx.QueryRow(`
			SELECT r.id, r.session_key, a.alias
			FROM requests r
			JOIN sessions s ON s.key = r.session_key
			JOIN aliases a ON a.session_key = s.key
			WHERE r.state = 'queued' AND s.active_token IS NULL AND s.decision IS NULL
			  AND s.cli_path IS NOT NULL AND s.cli_path != ''
			  AND s.start_mode != 'codex_owned'
              AND (? = '' OR s.provider = ?)
			ORDER BY r.rowid, a.alias LIMIT 1`, provider, provider).Scan(&requestID, &key, &alias)
		if err == sql.ErrNoRows {
			return ErrEmpty
		}
		if err != nil {
			return err
		}
		token, err = reserve(tx, key, requestID, alias)
		return err
	})
	if err != nil {
		return "", err
	}
	return token, nil
}

func reserve(tx *sql.Tx, key, requestID, alias string) (string, error) {
	token, err := newID()
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(`INSERT INTO attempts(token, session_key, request_id, alias, state, mcp_revoked) VALUES (?,?,?,?, 'reserved', 0)`,
		token, key, requestID, alias); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`UPDATE sessions SET active_token = ? WHERE key = ? AND active_token IS NULL`, token, key); err != nil {
		return "", err
	}
	if _, err := tx.Exec(`UPDATE requests SET state = 'reserved' WHERE id = ?`, requestID); err != nil {
		return "", err
	}
	return token, insertEvent(tx, "reserve", key, token, requestID)
}

// BeginLaunch records the intention to create a process. MCP stays closed until ConfirmIdentity.
func (s *Store) BeginLaunch(token string) error {
	return s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "reserved")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE attempts SET state = 'launching' WHERE token = ?`, token); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state = 'launching' WHERE id = ?`, row.requestID); err != nil {
			return err
		}
		return insertEvent(tx, "launching", row.sessionKey, token, "")
	})
}

// ConfirmIdentity opens MCP only when the started CLI reports the bound conversation.
func (s *Store) ConfirmIdentity(token, provider, environment, conversationID string) error {
	actual, err := SessionKey(provider, environment, conversationID)
	if err != nil {
		_ = s.MarkUncertain(token, "identity could not be canonicalized")
		return err
	}
	mismatch := false
	err = s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "launching")
		if err != nil {
			return err
		}
		if row.sessionKey != actual {
			mismatch = true
			return revoke(tx, row, "uncertain", "cli started a different session")
		}
		if _, err := tx.Exec(`UPDATE attempts SET state = 'running' WHERE token = ?`, token); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state = 'running' WHERE id = ?`, row.requestID); err != nil {
			return err
		}
		return insertEvent(tx, "identity_confirmed", row.sessionKey, token, "")
	})
	if mismatch {
		return fmt.Errorf("%w: cli started a different session", ErrRejected)
	}
	return err
}

func (s *Store) RecordProcess(token string, obs platform.Observation) error {
	if obs.PID <= 0 || obs.PGID <= 0 || obs.StartToken == "" || obs.BootID == "" {
		return fmt.Errorf("%w: incomplete process observation", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "launching", "running")
		if err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE attempts SET pid = ?, pgid = ?, start_token = ?, boot_id = ? WHERE token = ?`,
			obs.PID, obs.PGID, obs.StartToken, obs.BootID, token)
		if err != nil {
			return err
		}
		return insertEvent(tx, "process_recorded", row.sessionKey, token, fmt.Sprintf("pid %d pgid %d", obs.PID, obs.PGID))
	})
}

func (s *Store) CurrentMessage(token string) (Message, error) {
	var msg Message
	err := s.tx(func(tx *sql.Tx) error {
		row, err := mcpOpen(tx, token)
		if err != nil {
			return err
		}
		msg = Message{ID: row.requestID, From: row.sender, Text: row.text}
		return nil
	})
	return msg, err
}

func (s *Store) Reply(token, text string) (Reply, error) {
	if len(text) < 1 || len(text) > 2000 {
		return Reply{}, fmt.Errorf("%w: reply text must contain 1..2000 characters", ErrRejected)
	}
	var reply Reply
	err := s.tx(func(tx *sql.Tx) error {
		row, err := mcpOpen(tx, token)
		if err != nil {
			return err
		}
		reply, err = replyTx(tx, row, token, text)
		return err
	})
	return reply, err
}

func replyTx(tx *sql.Tx, row attemptRow, token, text string) (Reply, error) {
	var old Reply
	err := tx.QueryRow(`SELECT id, reply_to, sender, recipient, text FROM replies WHERE reply_to = ?`, row.requestID).
		Scan(&old.ID, &old.ReplyTo, &old.Sender, &old.Recipient, &old.Text)
	if err == nil {
		if old.Text != text {
			return Reply{}, fmt.Errorf("%w: a different reply is already stored", ErrRejected)
		}
		return old, nil
	}
	if err != sql.ErrNoRows {
		return Reply{}, err
	}
	id, err := newID()
	if err != nil {
		return Reply{}, err
	}
	reply := Reply{ID: id, ReplyTo: row.requestID, Sender: row.alias, Recipient: row.sender, Text: text}
	// The reply is held: it reaches its recipient only when the attempt is
	// verified and marked complete (publishVerifiedTx, T-022).
	_, err = tx.Exec(`INSERT INTO replies(id, reply_to, attempt_token, sender, recipient, text, held) VALUES (?,?,?,?,?,?,1)`,
		reply.ID, reply.ReplyTo, token, reply.Sender, reply.Recipient, reply.Text)
	if err != nil {
		return Reply{}, err
	}
	if err := insertEvent(tx, "reply", row.sessionKey, token, reply.ID+"; held until the run is verified"); err != nil {
		return Reply{}, err
	}
	return reply, nil
}

// publishVerifiedTx runs in the transaction that marks an attempt complete
// after its run was verified. It releases the attempt's held reply and routes
// it, and queues the messages the turn started with send. Nothing held by an
// attempt that is not completed this way is ever published.
func publishVerifiedTx(tx *sql.Tx, row attemptRow) error {
	var reply Reply
	err := tx.QueryRow(`SELECT id, reply_to, sender, recipient, text FROM replies WHERE attempt_token=? AND reply_to=? AND held=1`, row.token, row.requestID).
		Scan(&reply.ID, &reply.ReplyTo, &reply.Sender, &reply.Recipient, &reply.Text)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == nil {
		if _, err := tx.Exec(`UPDATE replies SET held=0 WHERE id=?`, reply.ID); err != nil {
			return err
		}
		if err := insertEvent(tx, "reply_published", row.sessionKey, row.token, reply.ID); err != nil {
			return err
		}
		if err := routeReply(tx, row, reply); err != nil {
			return err
		}
	}
	r, err := tx.Exec(`UPDATE requests SET state='queued' WHERE in_reply_to=? AND state='held' AND id LIKE 'send-%'`, row.requestID)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n > 0 {
		return insertEvent(tx, "sends_published", row.sessionKey, row.token, fmt.Sprintf("%d messages released after verification", n))
	}
	return nil
}

// FinishObserved records the supervisor's own wait and always leaves the session locked.
// An empty process group is stored as a note. It is not proof that detached descendants
// have stopped, so it does not clear the lock or mark the attempt completed.
func (s *Store) FinishObserved(token string, exitCode int, groupEmpty, providerOK bool, groupDetail string) (string, error) {
	const state = "uncertain"
	err := s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "launching", "running")
		if err != nil {
			return err
		}
		var replyID sql.NullString
		err = tx.QueryRow(`SELECT id FROM replies WHERE reply_to = ?`, row.requestID).Scan(&replyID)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		identity := row.state == "running"
		detail := fmt.Sprintf("exit %d identity %t reply %t group_empty %t; empty group does not unlock", exitCode, identity, replyID.Valid, groupEmpty)
		if groupDetail != "" {
			detail += "; " + groupDetail
		}
		_, err = tx.Exec(`INSERT INTO verifications(attempt_token, identity_confirmed, reply_id, exit_observed, exit_code, group_empty, provider_ok, detail)
			VALUES (?,?,?,?,?,?,?,?)
			ON CONFLICT(attempt_token) DO UPDATE SET
			  identity_confirmed = excluded.identity_confirmed,
			  reply_id = excluded.reply_id,
			  exit_observed = excluded.exit_observed,
			  exit_code = excluded.exit_code,
			  group_empty = excluded.group_empty,
			  provider_ok = excluded.provider_ok,
			  detail = excluded.detail`,
			token, boolInt(identity), nullString(replyID), 1, exitCode, boolInt(groupEmpty), boolInt(providerOK), detail)
		if err != nil {
			return err
		}
		return revoke(tx, row, "uncertain", detail)
	})
	return state, err
}

// RevokeInherited drops MCP access for attempts left by a previous supervisor.
// The session lock stays, so the new process cannot launch that session again.
func (s *Store) RevokeInherited() (int, error) {
	var n int
	err := s.tx(func(tx *sql.Tx) error {
		if err := stopInheritedStreams(tx); err != nil {
			return err
		}
		if err := stopInheritedCodexSetups(tx); err != nil {
			return err
		}
		if err := interruptInheritedTaskRuns(tx); err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT active_token FROM sessions WHERE active_token IS NOT NULL AND key NOT IN (SELECT session_key FROM stream_runs)`)
		if err != nil {
			return err
		}
		var tokens []string
		for rows.Next() {
			var token string
			if err := rows.Scan(&token); err != nil {
				rows.Close()
				return err
			}
			tokens = append(tokens, token)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, token := range tokens {
			row, err := owned(tx, token, "reserved", "launching", "running", "uncertain")
			if err != nil {
				return err
			}
			if err := revoke(tx, row, "uncertain", "supervisor restarted; previous mcp revoked"); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return n, err
}

// ConfigureLaunch stores the external CLI that the supervisor may execute for this alias.
func (s *Store) ConfigureLaunch(alias, cliPath, workspace string) error {
	if !filepath.IsAbs(cliPath) {
		return fmt.Errorf("%w: cli path must be absolute", ErrRejected)
	}
	resolved, err := filepath.EvalSymlinks(cliPath)
	if err != nil {
		return fmt.Errorf("%w: cli path must exist", ErrRejected)
	}
	info, err := os.Stat(resolved)
	if err != nil || info.IsDir() {
		return fmt.Errorf("%w: cli path must be a file", ErrRejected)
	}
	if workspace != "" {
		workspace, err = CanonicalEnvironment(workspace)
		if err != nil {
			return err
		}
	}
	return s.tx(func(tx *sql.Tx) error {
		key, _, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		if owned, err := codexOwnedSession(tx, key); err != nil || owned {
			if err == nil {
				err = fmt.Errorf("%w: session is served by its Codex owner", ErrRejected)
			}
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET cli_path = ?, workspace = ? WHERE key = ?`, resolved, workspace, key); err != nil {
			return err
		}
		return insertEvent(tx, "configure_launch", key, "", "cli bound")
	})
}

// MarkUncertain freezes the active attempt and revokes its MCP context.
// It does not clear the session lock and does not start another attempt.
func (s *Store) MarkUncertain(token, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "reserved", "launching", "running", "uncertain")
		if err != nil {
			return err
		}
		if row.state == "uncertain" && row.mcpRevoked {
			return insertEvent(tx, "uncertain", row.sessionKey, token, reason)
		}
		return revoke(tx, row, "uncertain", reason)
	})
}

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

// FailClosed records that the operator will not retry. History stays. The lock stays.
func (s *Store) FailClosed(alias string) error {
	return s.tx(func(tx *sql.Tx) error {
		key, _, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET decision = 'closed_without_retry' WHERE key = ?`, key); err != nil {
			return err
		}
		var token sql.NullString
		if err := tx.QueryRow(`SELECT active_token FROM sessions WHERE key = ?`, key).Scan(&token); err != nil {
			return err
		}
		if token.Valid {
			row, err := owned(tx, token.String, "reserved", "launching", "running", "uncertain")
			if err != nil {
				return err
			}
			if err := revoke(tx, row, "uncertain", "closed without retry"); err != nil {
				return err
			}
		}
		return insertEvent(tx, "fail_closed", key, token.String, "history kept; new attempts refused")
	})
}

// Accept completes a held attempt only when stop is proven and the result was verified.
// Reply text alone is not verification. This method does not invent the proof.
func (s *Store) Accept(alias string, proof platform.Proof) error {
	if !proof.Proven {
		return fmt.Errorf("%w: %s", ErrRecoveryBlocked, emptyDetail(proof.Detail))
	}
	return s.tx(func(tx *sql.Tx) error {
		key, _, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		var token sql.NullString
		if err := tx.QueryRow(`SELECT active_token FROM sessions WHERE key = ?`, key).Scan(&token); err != nil {
			return err
		}
		if !token.Valid || token.String == "" {
			return fmt.Errorf("%w: no held attempt", ErrRejected)
		}
		var identity, exitObserved, exitCode, groupEmpty, providerOK int
		var replyID sql.NullString
		err = tx.QueryRow(`SELECT identity_confirmed, reply_id, exit_observed, exit_code, group_empty, provider_ok FROM verifications WHERE attempt_token = ?`, token.String).
			Scan(&identity, &replyID, &exitObserved, &exitCode, &groupEmpty, &providerOK)
		if err == sql.ErrNoRows {
			return fmt.Errorf("%w: no verification record", ErrNotVerified)
		}
		if err != nil {
			return err
		}
		if identity != 1 || !replyID.Valid || exitObserved != 1 || exitCode != 0 || providerOK != 1 {
			return fmt.Errorf("%w: identity, bound reply, observed exit 0 and a valid provider result are required", ErrNotVerified)
		}
		row, err := owned(tx, token.String, "uncertain", "running", "launching")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE attempts SET state = 'completed', mcp_revoked = 1 WHERE token = ?`, token.String); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state = 'completed' WHERE id = ?`, row.requestID); err != nil {
			return err
		}
		if err := publishVerifiedTx(tx, row); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET active_token = NULL WHERE key = ?`, key); err != nil {
			return err
		}
		return insertEvent(tx, "accept", key, token.String, proof.Detail)
	})
}

// FailedAttemptRecoverable checks a held failure without changing it.
func (s *Store) FailedAttemptRecoverable(provider, environment, conversationID string) error {
	return s.tx(func(tx *sql.Tx) error {
		_, _, _, _, err := failedAttempt(tx, provider, environment, conversationID)
		return err
	})
}

func failedAttempt(tx *sql.Tx, provider, environment, conversationID string) (string, attemptRow, string, bool, error) {
	var key string
	var token sql.NullString
	err := tx.QueryRow(`SELECT key, active_token FROM sessions WHERE provider=? AND environment=? AND conversation_id=?`,
		provider, environment, conversationID).Scan(&key, &token)
	if err == sql.ErrNoRows {
		return "", attemptRow{}, "", false, fmt.Errorf("%w: session is not in the reserved data directory", ErrRejected)
	}
	if err != nil {
		return "", attemptRow{}, "", false, err
	}
	var streamCount int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM stream_runs WHERE session_key=?`, key).Scan(&streamCount); err != nil {
		return "", attemptRow{}, "", false, err
	}
	if streamCount > 0 {
		return "", attemptRow{}, "", false, fmt.Errorf("%w: persistent process remains reserved; recover-failed is for one-shot attempts", ErrRecoveryBlocked)
	}
	if !token.Valid || token.String == "" {
		existing, err := alreadyRecovered(tx, key)
		if err != nil {
			return "", attemptRow{}, "", false, err
		}
		if existing == "" {
			return "", attemptRow{}, "", false, fmt.Errorf("%w: no held attempt", ErrRejected)
		}
		return key, attemptRow{}, existing, true, nil
	}
	var requestID string
	if err := tx.QueryRow(`SELECT request_id FROM attempts WHERE token=?`, token.String).Scan(&requestID); err != nil {
		return "", attemptRow{}, "", false, err
	}
	var replies int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM replies WHERE attempt_token=? OR reply_to=?`, token.String, requestID).Scan(&replies); err != nil {
		return "", attemptRow{}, "", false, err
	}
	if replies > 0 {
		return "", attemptRow{}, "", false, fmt.Errorf("%w: a stored reply cannot be retired as a failure", ErrRejected)
	}
	var providerOK int
	var replyID sql.NullString
	err = tx.QueryRow(`SELECT provider_ok, reply_id FROM verifications WHERE attempt_token=?`, token.String).
		Scan(&providerOK, &replyID)
	if err == nil {
		if providerOK == 1 {
			return "", attemptRow{}, "", false, fmt.Errorf("%w: provider result is valid and cannot be retired as a failure", ErrRejected)
		}
		if replyID.Valid {
			return "", attemptRow{}, "", false, fmt.Errorf("%w: a stored reply cannot be retired as a failure", ErrRejected)
		}
	} else if err != sql.ErrNoRows {
		return "", attemptRow{}, "", false, err
	}
	row, err := owned(tx, token.String, "uncertain", "running", "launching", "reserved")
	if err != nil {
		return "", attemptRow{}, "", false, err
	}
	return key, row, "", false, nil
}

func alreadyRecovered(tx *sql.Tx, key string) (string, error) {
	var detail string
	err := tx.QueryRow(`SELECT detail FROM events WHERE session_key=? AND kind='failed_attempt_recovered' ORDER BY id DESC LIMIT 1`, key).Scan(&detail)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	const prefix = "actions may be repeated; new "
	if !strings.HasPrefix(detail, prefix) || len(detail) == len(prefix) {
		return "", fmt.Errorf("%w: recovery record is incomplete", ErrRecoveryBlocked)
	}
	id := detail[len(prefix):]
	var state string
	var retryOf sql.NullString
	err = tx.QueryRow(`SELECT state, retry_of FROM requests WHERE id=? AND session_key=?`, id, key).Scan(&state, &retryOf)
	if err != nil {
		return "", fmt.Errorf("%w: recovered request is missing", ErrRecoveryBlocked)
	}
	if state != "queued" || !retryOf.Valid || retryOf.String == "" {
		return "", fmt.Errorf("%w: recovered request is not waiting", ErrRecoveryBlocked)
	}
	return id, nil
}

// RecoverFailedAttempt keeps a failed attempt and queues one new request.
// It refuses a valid provider result or a stored reply. The caller must already
// have checked that the recorded process and its group are not running.
func (s *Store) RecoverFailedAttempt(provider, environment, conversationID string) (string, error) {
	var newID string
	err := s.tx(func(tx *sql.Tx) error {
		key, row, existing, already, err := failedAttempt(tx, provider, environment, conversationID)
		if err != nil {
			return err
		}
		if already {
			newID = existing
			return nil
		}
		if err := revoke(tx, row, "uncertain", "failed attempt kept; detached descendants outside the group were not proven"); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET active_token=NULL WHERE key=? AND active_token=?`, key, row.token); err != nil {
			return err
		}
		newID, err = newIDFunc()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO requests(id, session_key, sender, text, state, retry_of, created_at) VALUES (?,?,?,?, 'queued', ?, ?)`,
			newID, key, row.sender, row.text, row.requestID, now()); err != nil {
			return err
		}
		return insertEvent(tx, "failed_attempt_recovered", key, row.token, "actions may be repeated; new "+newID)
	})
	return newID, err
}

// Retry authorizes one new queued request. It refuses unless stop is proven and the
// operator sent the exact warning phrase. Old rows are not deleted.
func (s *Store) Retry(alias, confirmation string, proof platform.Proof) (string, error) {
	if confirmation != RetryConfirmation {
		return "", ErrBadConfirmation
	}
	if !proof.Proven {
		return "", fmt.Errorf("%w: %s", ErrRecoveryBlocked, emptyDetail(proof.Detail))
	}
	var newID string
	err := s.tx(func(tx *sql.Tx) error {
		key, decision, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		if decision == "closed_without_retry" {
			return fmt.Errorf("%w: session was closed without retry", ErrClosed)
		}
		var token sql.NullString
		if err := tx.QueryRow(`SELECT active_token FROM sessions WHERE key = ?`, key).Scan(&token); err != nil {
			return err
		}
		if !token.Valid || token.String == "" {
			return fmt.Errorf("%w: no held attempt", ErrRejected)
		}
		row, err := owned(tx, token.String, "uncertain")
		if err != nil {
			return err
		}
		if err := revoke(tx, row, "uncertain", "retry authorized after stop proof"); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET active_token = NULL WHERE key = ?`, key); err != nil {
			return err
		}
		newID, err = newIDFunc()
		if err != nil {
			return err
		}
		warning := "actions may be repeated"
		if _, err := tx.Exec(`INSERT INTO requests(id, session_key, sender, text, state, retry_of, created_at) VALUES (?,?,?,?, 'queued', ?, ?)`,
			newID, key, row.sender, row.text, row.requestID, now()); err != nil {
			return err
		}
		return insertEvent(tx, "retry_authorized", key, token.String, warning+"; new "+newID)
	})
	return newID, err
}

func (s *Store) Observation(token string) (platform.Observation, error) {
	var obs platform.Observation
	var pid, pgid sql.NullInt64
	var start, boot sql.NullString
	err := s.db.QueryRow(`SELECT pid, pgid, start_token, boot_id FROM attempts WHERE token = ?`, token).
		Scan(&pid, &pgid, &start, &boot)
	if err != nil {
		return obs, err
	}
	if pid.Valid {
		obs.PID = int(pid.Int64)
	}
	if pgid.Valid {
		obs.PGID = int(pgid.Int64)
	}
	obs.StartToken = start.String
	obs.BootID = boot.String
	return obs, nil
}

func (s *Store) ActiveToken(alias string) (string, error) {
	var token sql.NullString
	err := s.tx(func(tx *sql.Tx) error {
		key, _, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		return tx.QueryRow(`SELECT active_token FROM sessions WHERE key = ?`, key).Scan(&token)
	})
	if err != nil {
		return "", err
	}
	return token.String, nil
}

func (s *Store) Snapshot() (Snapshot, error) {
	var snap Snapshot
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		snap.Paused, err = isPaused(tx)
		if err != nil {
			return err
		}
		snap.Sessions, err = readRows(tx, `SELECT key, provider, environment, conversation_id, active_token, decision, cli_path, workspace, start_mode FROM sessions ORDER BY rowid`)
		if err != nil {
			return err
		}
		snap.Requests, err = readRows(tx, `SELECT id, session_key, sender, text, state, retry_of, in_reply_to, hops FROM requests ORDER BY rowid`)
		if err != nil {
			return err
		}
		snap.Attempts, err = readRows(tx, `SELECT token, session_key, request_id, alias, state, pid, pgid, mcp_revoked FROM attempts ORDER BY rowid`)
		if err != nil {
			return err
		}
		snap.Replies, err = readRows(tx, `SELECT id, reply_to, sender, recipient, text, held FROM replies ORDER BY rowid`)
		if err != nil {
			return err
		}
		snap.StreamRuns, err = readRows(tx, `SELECT id,session_key,state,current_token,exit_observed,exit_code FROM stream_runs ORDER BY rowid`)
		if err != nil {
			return err
		}
		snap.ArchivedStreamRuns, err = readRows(tx, `SELECT * FROM archived_stream_runs ORDER BY rowid`)
		if err != nil {
			return err
		}
		snap.StreamRecoveries, err = readRows(tx, `SELECT run_id,phase,proof FROM stream_recoveries ORDER BY rowid`)
		if err != nil {
			return err
		}
		snap.Verifications, err = readRows(tx, `SELECT attempt_token,identity_confirmed,reply_id,exit_observed,exit_code,group_empty,provider_ok,detail FROM verifications ORDER BY rowid`)
		if err != nil {
			return err
		}
		snap.CodexSetups, err = readRows(tx, `SELECT id,alias,state,thread_id,pid,exit_observed,exit_code,detail FROM codex_setups ORDER BY rowid`)
		if err != nil {
			return err
		}
		snap.Events, err = readRows(tx, `SELECT kind, detail, attempt_token FROM events ORDER BY id`)
		return err
	})
	return snap, err
}

type attemptRow struct {
	token, sessionKey, requestID, alias, state, sender, text string
	mcpRevoked                                               bool
}

func owned(tx *sql.Tx, token string, states ...string) (attemptRow, error) {
	var row attemptRow
	var revoked int
	err := tx.QueryRow(`
		SELECT a.token, a.session_key, a.request_id, a.alias, a.state, a.mcp_revoked, r.sender, r.text
		FROM attempts a
		JOIN sessions s ON s.key = a.session_key AND s.active_token = a.token
		JOIN requests r ON r.id = a.request_id
		WHERE a.token = ?`, token).Scan(&row.token, &row.sessionKey, &row.requestID, &row.alias, &row.state, &revoked, &row.sender, &row.text)
	if err == sql.ErrNoRows {
		return row, fmt.Errorf("%w: inactive or unknown execution context", ErrRejected)
	}
	if err != nil {
		return row, err
	}
	row.mcpRevoked = revoked != 0
	if !contains(states, row.state) {
		return row, fmt.Errorf("%w: execution is %s", ErrRejected, row.state)
	}
	return row, nil
}

func mcpOpen(tx *sql.Tx, token string) (attemptRow, error) {
	row, err := owned(tx, token, "running")
	if err != nil {
		return row, err
	}
	if row.mcpRevoked {
		return row, fmt.Errorf("%w: mcp context has been revoked", ErrRejected)
	}
	return row, nil
}

func revoke(tx *sql.Tx, row attemptRow, state, reason string) error {
	if _, err := tx.Exec(`UPDATE attempts SET state = ?, mcp_revoked = 1 WHERE token = ?`, state, row.token); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE requests SET state = ? WHERE id = ?`, state, row.requestID); err != nil {
		return err
	}
	return insertEvent(tx, state, row.sessionKey, row.token, reason)
}

func sessionByAlias(tx *sql.Tx, alias string) (string, string, error) {
	var key string
	var decision sql.NullString
	err := tx.QueryRow(`SELECT a.session_key, s.decision FROM aliases a JOIN sessions s ON s.key = a.session_key WHERE a.alias = ?`, alias).
		Scan(&key, &decision)
	if err == sql.ErrNoRows {
		return "", "", fmt.Errorf("%w: unknown alias", ErrRejected)
	}
	return key, decision.String, err
}

func paused(tx *sql.Tx) (bool, error) { return isPaused(tx) }

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

func readRows(tx *sql.Tx, query string) ([]Row, error) {
	rows, err := tx.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []Row
	for rows.Next() {
		values := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := Row{}
		for i, col := range cols {
			switch v := values[i].(type) {
			case []byte:
				row[col] = string(v)
			default:
				row[col] = v
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
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

func emptyDetail(detail string) string {
	if detail == "" {
		return "stop is not proven"
	}
	return detail
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

func ensureSessionColumns(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA table_info(sessions)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	have := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notNull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &ctype, &notNull, &dflt, &pk); err != nil {
			return err
		}
		have[name] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if !have["cli_path"] {
		if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN cli_path TEXT`); err != nil {
			return err
		}
	}
	if !have["workspace"] {
		if _, err := db.Exec(`ALTER TABLE sessions ADD COLUMN workspace TEXT`); err != nil {
			return err
		}
	}
	return nil
}

func now() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func newID() (string, error) { return newIDFunc() }

func newIDFunc() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
