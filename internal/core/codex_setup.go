package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// A Codex session gets its ID from the server (thread/start), so it cannot be
// registered up front like a new Claude session. codex_setups records the
// intent to create one before any process starts, and then each step. The
// alias is reserved for the whole life of an active setup.
//
// States, in order:
//
//	intent           recorded; nothing launched
//	launching        the process may be starting; any failure from here is uncertain
//	started          process observed and recorded
//	configured       effective config checked
//	thread_requested committed before thread/start is written; never re-sent
//	created          thread ID stored; the live process owns it and is idle
//	busy             one turn of the owner is in flight (codex_turns)
//
// Terminal states: not_started (proven never launched; alias released) and
// uncertain (alias and process reservation kept until an explicit resolution,
// which is not implemented yet).
const codexSetupSchema = `
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
CREATE UNIQUE INDEX IF NOT EXISTS codex_setups_active_alias ON codex_setups(alias) WHERE state != 'not_started';
CREATE TABLE IF NOT EXISTS codex_turns (
 attempt_token TEXT PRIMARY KEY REFERENCES attempts(token),
 setup_id TEXT NOT NULL REFERENCES codex_setups(id),
 receipt TEXT NOT NULL UNIQUE,
 sent_intent INTEGER NOT NULL DEFAULT 0,
 turn_id TEXT UNIQUE,
 read_seen INTEGER NOT NULL DEFAULT 0
);
`

// CodexSetup is one row of codex_setups.
type CodexSetup struct {
	ID, Alias, Environment, CLI, Workspace, State, ThreadID string
	ExitObserved                                            bool
}

var codexFingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func validAlias(alias string) bool {
	return alias != "" && strings.TrimSpace(alias) == alias && len(alias) <= 128 && !strings.ContainsAny(alias, "\r\n\x00")
}

// canonicalCLI resolves an absolute path to a regular executable file.
func canonicalCLI(cli string) (string, error) {
	if !filepath.IsAbs(cli) {
		return "", ErrRejected
	}
	resolved, err := filepath.EvalSymlinks(cli)
	if err != nil {
		return "", ErrRejected
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", ErrRejected
	}
	return resolved, nil
}

// aliasReservedBySetup reports whether an active Codex setup holds the alias.
func aliasReservedBySetup(tx *sql.Tx, alias string) (bool, error) {
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM codex_setups WHERE alias=? AND state!='not_started'`, alias).Scan(&n)
	return n != 0, err
}

// threadOwnedBySetup reports whether a Codex setup holds this exact thread, so
// it cannot also be registered as an ordinary session and launched twice.
func threadOwnedBySetup(tx *sql.Tx, provider, environment, conversationID string) (bool, error) {
	if provider != "codex" {
		return false, nil
	}
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM codex_setups WHERE thread_id=? AND environment=?`, conversationID, environment).Scan(&n)
	return n != 0, err
}

// BeginCodexSetup records the intent to create a Codex session. It launches
// nothing. Repeating the identical call while still in intent is a no-op;
// anything else that reuses the ID or alias is rejected.
func (s *Store) BeginCodexSetup(id, alias, environment, cli, workspace string) error {
	if !ValidSetupID(id) || !validAlias(alias) {
		return fmt.Errorf("%w: invalid Codex setup identity", ErrRejected)
	}
	environment, err := CanonicalEnvironment(environment)
	if err != nil {
		return err
	}
	workspace, err = CanonicalEnvironment(workspace)
	if err != nil {
		return err
	}
	if cli, err = canonicalCLI(cli); err != nil {
		return err
	}
	return s.tx(func(tx *sql.Tx) error {
		var oldAlias, oldEnv, oldCLI, oldWorkspace, state string
		err := tx.QueryRow(`SELECT alias,environment,cli_path,workspace,state FROM codex_setups WHERE id=?`, id).Scan(&oldAlias, &oldEnv, &oldCLI, &oldWorkspace, &state)
		if err == nil {
			if oldAlias == alias && oldEnv == environment && oldCLI == cli && oldWorkspace == workspace && state == "intent" {
				return nil
			}
			return fmt.Errorf("%w: setup id reused", ErrRejected)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var bound int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM aliases WHERE alias=?`, alias).Scan(&bound); err != nil {
			return err
		}
		reserved, err := aliasReservedBySetup(tx, alias)
		if err != nil {
			return err
		}
		if bound != 0 || reserved {
			return fmt.Errorf("%w: alias is already in use", ErrRejected)
		}
		at := now()
		if _, err := tx.Exec(`INSERT INTO codex_setups(id,alias,environment,cli_path,workspace,state,created_at,updated_at) VALUES (?,?,?,?,?,'intent',?,?)`, id, alias, environment, cli, workspace, at, at); err != nil {
			return err
		}
		return insertEvent(tx, "codex_setup_intent", "", "", "setup "+id+"; nothing launched")
	})
}

// codexStep moves a setup from one of the given states to next.
func codexStep(tx *sql.Tx, id, next, detail string, from ...string) error {
	var state string
	if err := tx.QueryRow(`SELECT state FROM codex_setups WHERE id=?`, id).Scan(&state); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown Codex setup", ErrRejected)
		}
		return err
	}
	if !contains(from, state) {
		return fmt.Errorf("%w: Codex setup is %s", ErrRejected, state)
	}
	if _, err := tx.Exec(`UPDATE codex_setups SET state=?,detail=?,updated_at=? WHERE id=?`, next, detail, now(), id); err != nil {
		return err
	}
	return insertEvent(tx, "codex_setup_"+next, "", "", "setup "+id+"; "+detail)
}

// MarkCodexSetupLaunching is committed before the process reservation and exec.
func (s *Store) MarkCodexSetupLaunching(id string) error {
	return s.tx(func(tx *sql.Tx) error {
		return codexStep(tx, id, "launching", "process may start", "intent")
	})
}

// CodexSetupNotStarted is only for an intent that never launched, or a definite
// OS refusal before the executable ran. It releases the alias.
func (s *Store) CodexSetupNotStarted(id, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		var pid sql.NullInt64
		if err := tx.QueryRow(`SELECT pid FROM codex_setups WHERE id=?`, id).Scan(&pid); err != nil {
			return err
		}
		if pid.Valid {
			return fmt.Errorf("%w: a process was recorded", ErrRejected)
		}
		return codexStep(tx, id, "not_started", reason, "intent", "launching")
	})
}

// RecordCodexSetupProcess stores the observed process.
func (s *Store) RecordCodexSetupProcess(id string, obs platform.Observation) error {
	if obs.PID <= 0 || obs.PGID <= 1 || obs.StartToken == "" || obs.BootID == "" {
		return fmt.Errorf("%w: incomplete process observation", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		if err := codexStep(tx, id, "started", "process observed", "launching"); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE codex_setups SET pid=?,pgid=?,start_token=?,boot_id=? WHERE id=?`, obs.PID, obs.PGID, obs.StartToken, obs.BootID, id)
		return err
	})
}

// ConfirmCodexSetupConfig stores a hash of the checked effective config. The
// config itself is never stored: it may contain private values.
func (s *Store) ConfirmCodexSetupConfig(id, fingerprint string) error {
	if !codexFingerprintPattern.MatchString(fingerprint) {
		return fmt.Errorf("%w: invalid config fingerprint", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		if err := codexStep(tx, id, "configured", "effective config checked", "started"); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE codex_setups SET config_fingerprint=? WHERE id=?`, fingerprint, id)
		return err
	})
}

// PrepareCodexThreadStart is committed BEFORE thread/start is written. From
// here a lost response or a crash is uncertain; thread/start is never re-sent.
func (s *Store) PrepareCodexThreadStart(id string) error {
	return s.tx(func(tx *sql.Tx) error {
		return codexStep(tx, id, "thread_requested", "thread/start sent once; no automatic retry", "configured")
	})
}

// CompleteCodexSetup stores the server-issued thread ID and registers the
// session under the reserved alias, marked codex_owned: only its live owner
// serves it. Generic claims and launches never pick it up.
func (s *Store) CompleteCodexSetup(id, threadID string) error {
	if strings.TrimSpace(threadID) != threadID || threadID == "" || len(threadID) > 128 {
		return fmt.Errorf("%w: invalid thread id", ErrRejected)
	}
	setup, err := s.CodexSetup(id)
	if err != nil {
		return err
	}
	key, err := SessionKey("codex", setup.Environment, threadID)
	if err != nil {
		return err
	}
	return s.tx(func(tx *sql.Tx) error {
		var registered int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE key=? OR (provider='codex' AND environment=? AND conversation_id=?)`, key, setup.Environment, threadID).Scan(&registered); err != nil {
			return err
		}
		var aliased int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM aliases WHERE alias=?`, setup.Alias).Scan(&aliased); err != nil {
			return err
		}
		if registered != 0 || aliased != 0 {
			return fmt.Errorf("%w: thread or alias already registered", ErrRejected)
		}
		if err := codexStep(tx, id, "created", "thread issued; owned by live process", "thread_requested"); err != nil {
			return err
		}
		// cli_path stays NULL on purpose: an older client that does not know
		// codex_owned only launches sessions with a CLI, so it cannot start a
		// second process for this thread. The CLI is kept in codex_setups.
		if _, err := tx.Exec(`INSERT INTO sessions(key,provider,environment,conversation_id,cli_path,workspace,start_mode) VALUES (?,'codex',?,?,NULL,?,'codex_owned')`, key, setup.Environment, threadID, setup.Workspace); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO aliases(alias,session_key) VALUES (?,?)`, setup.Alias, key); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE codex_setups SET thread_id=?,session_key=? WHERE id=?`, threadID, key, id)
		return err
	})
}

// codexOwnedSession reports whether only a Codex owner may serve this session.
func codexOwnedSession(tx *sql.Tx, key string) (bool, error) {
	var mode string
	if err := tx.QueryRow(`SELECT start_mode FROM sessions WHERE key=?`, key).Scan(&mode); err != nil {
		return false, err
	}
	return mode == "codex_owned", nil
}

// CodexOwnedAlias reports whether the alias names a Codex-owned session.
func (s *Store) CodexOwnedAlias(alias string) (bool, error) {
	var owned bool
	err := s.tx(func(tx *sql.Tx) error {
		key, _, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		owned, err = codexOwnedSession(tx, key)
		return err
	})
	return owned, err
}

// ClaimCodexTurn reserves the oldest queued message for an idle owner. It is
// committed before anything is written to the process. The attempt inherits
// the owner's process observation.
func (s *Store) ClaimCodexTurn(id string) (string, error) {
	var token string
	err := s.tx(func(tx *sql.Tx) error {
		p, err := isPaused(tx)
		if err != nil {
			return err
		}
		if p {
			return ErrPaused
		}
		var state, alias string
		var key sql.NullString
		if err := tx.QueryRow(`SELECT state,alias,session_key FROM codex_setups WHERE id=?`, id).Scan(&state, &alias, &key); err != nil {
			return err
		}
		if state != "created" || !key.Valid {
			return fmt.Errorf("%w: Codex owner is %s", ErrRejected, state)
		}
		var active sql.NullString
		var decision sql.NullString
		if err := tx.QueryRow(`SELECT active_token,decision FROM sessions WHERE key=?`, key.String).Scan(&active, &decision); err != nil {
			return err
		}
		if active.Valid || decision.Valid {
			return ErrBusy
		}
		var request string
		err = tx.QueryRow(`SELECT id FROM requests WHERE session_key=? AND state='queued' ORDER BY rowid LIMIT 1`, key.String).Scan(&request)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEmpty
		}
		if err != nil {
			return err
		}
		token, err = reserve(tx, key.String, request, alias)
		if err != nil {
			return err
		}
		receipt, err := newID()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE attempts SET state='launching',pid=(SELECT pid FROM codex_setups WHERE id=?),pgid=(SELECT pgid FROM codex_setups WHERE id=?),start_token=(SELECT start_token FROM codex_setups WHERE id=?),boot_id=(SELECT boot_id FROM codex_setups WHERE id=?) WHERE token=?`, id, id, id, id, token); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state='launching' WHERE id=?`, request); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO codex_turns(attempt_token,setup_id,receipt) VALUES (?,?,?)`, token, id, receipt); err != nil {
			return err
		}
		return codexStep(tx, id, "busy", "turn reserved for message "+request, "created")
	})
	return token, err
}

// PrepareCodexTurnSend is committed BEFORE turn/start is written. From here a
// failure is uncertain; the message is never sent again automatically.
func (s *Store) PrepareCodexTurnSend(token string) error {
	return s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "launching")
		if err != nil {
			return err
		}
		r, err := tx.Exec(`UPDATE codex_turns SET sent_intent=1 WHERE attempt_token=? AND sent_intent=0`, token)
		if err != nil {
			return err
		}
		if n, err := r.RowsAffected(); err != nil || n != 1 {
			if err == nil {
				err = fmt.Errorf("%w: turn already sent", ErrRejected)
			}
			return err
		}
		return insertEvent(tx, "codex_send_intent", row.sessionKey, token, "turn/start sent once; no automatic replay")
	})
}

// BindCodexTurn stores the turn ID from the turn/start response. Only then do
// the bound tools open for this attempt.
func (s *Store) BindCodexTurn(token, turnID string) error {
	if turnID == "" || strings.TrimSpace(turnID) != turnID || len(turnID) > 128 {
		return fmt.Errorf("%w: invalid turn id", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "launching")
		if err != nil {
			return err
		}
		if row.mcpRevoked {
			return fmt.Errorf("%w: tools revoked", ErrRejected)
		}
		r, err := tx.Exec(`UPDATE codex_turns SET turn_id=? WHERE attempt_token=? AND sent_intent=1 AND turn_id IS NULL`, turnID, token)
		if err != nil {
			return err
		}
		if n, err := r.RowsAffected(); err != nil || n != 1 {
			if err == nil {
				err = fmt.Errorf("%w: turn not sent or already bound", ErrRejected)
			}
			return err
		}
		if _, err := tx.Exec(`UPDATE attempts SET state='running' WHERE token=?`, token); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state='running' WHERE id=?`, row.requestID); err != nil {
			return err
		}
		return insertEvent(tx, "identity_confirmed", row.sessionKey, token, "Codex turn bound to this attempt")
	})
}

// CodexRead returns the current message and its one-time receipt.
func (s *Store) CodexRead(token string) (string, string, error) {
	var text, receipt string
	err := s.tx(func(tx *sql.Tx) error {
		row, err := mcpOpen(tx, token)
		if err != nil {
			return err
		}
		if err := tx.QueryRow(`SELECT receipt FROM codex_turns WHERE attempt_token=? AND sent_intent=1 AND turn_id IS NOT NULL`, token).Scan(&receipt); err != nil {
			return err
		}
		text = row.text
		_, err = tx.Exec(`UPDATE codex_turns SET read_seen=1 WHERE attempt_token=?`, token)
		return err
	})
	return text, receipt, err
}

// CodexReply stores the reply for the current message. The recipient is the
// message's sender, set by code.
func (s *Store) CodexReply(token, receipt, text string) error {
	if len(text) < 1 || len(text) > 2000 {
		return ErrRejected
	}
	return s.tx(func(tx *sql.Tx) error {
		row, err := mcpOpen(tx, token)
		if err != nil {
			return err
		}
		var expected string
		var read int
		if err := tx.QueryRow(`SELECT receipt,read_seen FROM codex_turns WHERE attempt_token=?`, token).Scan(&expected, &read); err != nil {
			return err
		}
		if receipt == "" || receipt != expected || read != 1 {
			return fmt.Errorf("%w: stale or unread receipt", ErrRejected)
		}
		_, err = replyTx(tx, row, token, text)
		return err
	})
}

// CompleteCodexTurn records a verified turn: the guard accepted turn/completed
// and the bound reply is stored. The owner becomes idle and keeps the thread.
func (s *Store) CompleteCodexTurn(token string) error {
	return s.tx(func(tx *sql.Tx) error {
		row, err := mcpOpen(tx, token)
		if err != nil {
			return err
		}
		var setup string
		if err := tx.QueryRow(`SELECT setup_id FROM codex_turns WHERE attempt_token=?`, token).Scan(&setup); err != nil {
			return err
		}
		var reply string
		if err := tx.QueryRow(`SELECT id FROM replies WHERE attempt_token=? AND reply_to=?`, token, row.requestID).Scan(&reply); err != nil {
			return fmt.Errorf("%w: bound reply missing", ErrNotVerified)
		}
		if _, err := tx.Exec(`INSERT INTO verifications(attempt_token,identity_confirmed,reply_id,exit_observed,exit_code,group_empty,provider_ok,detail) VALUES (?,1,?,0,NULL,0,1,'Codex turn completed; process owner retained')`, token, reply); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE attempts SET state='turn_completed',mcp_revoked=1 WHERE token=?`, token); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state='completed' WHERE id=?`, row.requestID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET active_token=NULL WHERE key=? AND active_token=?`, row.sessionKey, token); err != nil {
			return err
		}
		if err := insertEvent(tx, "codex_turn_completed", row.sessionKey, token, "session remains owned by live process"); err != nil {
			return err
		}
		return codexStep(tx, setup, "created", "idle; thread owned by live process", "busy")
	})
}

// FailCodexTurn makes the attempt and its owner uncertain. The session stays
// locked by the attempt, the reservation stays, and nothing is retried.
func (s *Store) FailCodexTurn(token, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "launching", "running", "uncertain")
		if err != nil {
			return err
		}
		var setup string
		if err := tx.QueryRow(`SELECT setup_id FROM codex_turns WHERE attempt_token=?`, token).Scan(&setup); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO verifications(attempt_token,identity_confirmed,reply_id,exit_observed,exit_code,group_empty,provider_ok,detail) VALUES (?,?,(SELECT id FROM replies WHERE attempt_token=?),0,NULL,0,0,?)`, token, boolInt(row.state == "running"), token, reason); err != nil {
			return err
		}
		if err := revoke(tx, row, "uncertain", reason); err != nil {
			return err
		}
		return codexStep(tx, setup, "uncertain", reason, "busy", "uncertain")
	})
}

// MarkCodexSetupUncertain keeps the alias and the process reservation. It is
// used for every failure after launching began.
func (s *Store) MarkCodexSetupUncertain(id, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		return codexStep(tx, id, "uncertain", reason, "launching", "started", "configured", "thread_requested", "created", "busy", "uncertain")
	})
}

// RecordCodexSetupExit notes that the owner process exited. That does not prove
// its descendants stopped, so the setup becomes uncertain and keeps its alias.
func (s *Store) RecordCodexSetupExit(id string, code int) error {
	return s.tx(func(tx *sql.Tx) error {
		if err := codexStep(tx, id, "uncertain", "app-server exit observed; descendants not proven stopped", "launching", "started", "configured", "thread_requested", "created", "busy", "uncertain"); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE codex_setups SET exit_observed=1,exit_code=? WHERE id=?`, code, id)
		return err
	})
}

// CodexSetup reads one setup.
func (s *Store) CodexSetup(id string) (CodexSetup, error) {
	var c CodexSetup
	var thread sql.NullString
	var exit int
	err := s.db.QueryRow(`SELECT id,alias,environment,cli_path,workspace,state,thread_id,exit_observed FROM codex_setups WHERE id=?`, id).
		Scan(&c.ID, &c.Alias, &c.Environment, &c.CLI, &c.Workspace, &c.State, &thread, &exit)
	c.ThreadID = thread.String
	c.ExitObserved = exit != 0
	return c, err
}

// stopInheritedCodexSetups runs at startup. The previous supervisor's pipes to
// any app-server are gone, so every active setup is uncertain. An intent that
// never reached launching started nothing and is released.
func stopInheritedCodexSetups(tx *sql.Tx) error {
	if _, err := tx.Exec(`UPDATE codex_setups SET state='not_started',detail='supervisor restarted before launch',updated_at=? WHERE state='intent'`, now()); err != nil {
		return err
	}
	r, err := tx.Exec(`UPDATE codex_setups SET state='uncertain',detail='supervisor restarted; owner process lost; no retry',updated_at=? WHERE state IN ('launching','started','configured','thread_requested','created','busy')`, now())
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 0 {
		return insertEvent(tx, "codex_setup_inherited", "", "", fmt.Sprintf("%d setups marked uncertain; reservations kept", n))
	}
	return nil
}
