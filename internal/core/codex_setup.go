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
// Terminal states: not_started (proven never launched; alias released),
// uncertain (alias and process reservation kept), and retired (recover-codex
// proved a reboot since launch; reservation removed, session closed, history
// kept). retiring is the pending step of that recovery.
//
// Resuming a saved thread (recover-codex --resume) reuses the same row, so the
// thread keeps one setup and one alias:
//
//	resumable        reservation removed after a proven reboot; no process;
//	                 the session stays open; the alias stays reserved
//	launching ... configured   as above, for a new app-server process
//	resume_requested committed before thread/resume is written; never re-sent
//	created          the new process owns the same thread ID
//
// recovery_mode records which of the two recoveries was prepared, so a rerun of
// a half-done recovery cannot switch between them.
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

// CodexSetup is one row of codex_setups.
type CodexSetup struct {
	ID, Alias, Environment, CLI, Workspace, State, ThreadID string
	ExitObserved                                            bool
	Observation                                             platform.Observation
	RecoveryFingerprint, RecoveryReservation                string
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
	err := tx.QueryRow(`SELECT COUNT(*) FROM codex_setups WHERE alias=? AND state NOT IN ('not_started','retired')`, alias).Scan(&n)
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
		var thread sql.NullString
		if err := tx.QueryRow(`SELECT pid,thread_id FROM codex_setups WHERE id=?`, id).Scan(&pid, &thread); err != nil {
			return err
		}
		if pid.Valid {
			return fmt.Errorf("%w: a process was recorded", ErrRejected)
		}
		if thread.Valid {
			// A resumed setup keeps its thread and alias; see CodexResumeNotStarted.
			return fmt.Errorf("%w: setup already has a thread", ErrRejected)
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

// CodexConfigFingerprint returns the hash of the config checked at setup.
func (s *Store) CodexConfigFingerprint(id string) (string, error) {
	var fingerprint sql.NullString
	err := s.db.QueryRow(`SELECT config_fingerprint FROM codex_setups WHERE id=?`, id).Scan(&fingerprint)
	if err == nil && !fingerprint.Valid {
		err = fmt.Errorf("%w: no checked config", ErrRejected)
	}
	return fingerprint.String, err
}

// AbortCodexTurnUnsent is for a claimed turn whose turn/start was never
// written (send intent not recorded). The message goes back to the queue; the
// owner becomes uncertain with the given reason, so nothing else is claimed.
func (s *Store) AbortCodexTurnUnsent(token, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "launching")
		if err != nil {
			return err
		}
		var setup string
		var sent int
		if err := tx.QueryRow(`SELECT setup_id,sent_intent FROM codex_turns WHERE attempt_token=?`, token).Scan(&setup, &sent); err != nil {
			return err
		}
		if sent != 0 {
			return fmt.Errorf("%w: turn may have been sent", ErrRejected)
		}
		if _, err := tx.Exec(`UPDATE attempts SET state='not_started',mcp_revoked=1 WHERE token=?`, token); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state='queued' WHERE id=?`, row.requestID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET active_token=NULL WHERE key=? AND active_token=?`, row.sessionKey, token); err != nil {
			return err
		}
		if err := insertEvent(tx, "codex_turn_not_sent", row.sessionKey, token, reason+"; message returned to the queue"); err != nil {
			return err
		}
		return codexStep(tx, setup, "uncertain", reason, "busy")
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
		return codexStep(tx, id, "uncertain", reason, "launching", "started", "configured", "thread_requested", "resume_requested", "created", "busy", "uncertain")
	})
}

// RecordCodexSetupExit notes that the owner process exited. That does not prove
// its descendants stopped, so the setup becomes uncertain and keeps its alias.
func (s *Store) RecordCodexSetupExit(id string, code int) error {
	return s.tx(func(tx *sql.Tx) error {
		if err := codexStep(tx, id, "uncertain", "app-server exit observed; descendants not proven stopped", "launching", "started", "configured", "thread_requested", "resume_requested", "created", "busy", "uncertain"); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE codex_setups SET exit_observed=1,exit_code=? WHERE id=?`, code, id)
		return err
	})
}

// CodexSetup reads one setup.
func (s *Store) CodexSetup(id string) (CodexSetup, error) {
	return s.codexSetupWhere(`id=?`, id)
}

// CodexSetupByAlias returns the newest setup that used the alias, if any.
func (s *Store) CodexSetupByAlias(alias string) (CodexSetup, error) {
	return s.codexSetupWhere(`alias=? ORDER BY rowid DESC LIMIT 1`, alias)
}

// CodexSetupForRecovery returns the setup that holds the alias and is waiting
// for, or in the middle of, recover-codex.
func (s *Store) CodexSetupForRecovery(alias string) (CodexSetup, error) {
	c, err := s.codexSetupWhere(`alias=? AND state IN ('uncertain','retiring')`, alias)
	if errors.Is(err, sql.ErrNoRows) {
		return c, fmt.Errorf("%w: no uncertain Codex setup holds this alias", ErrRejected)
	}
	return c, err
}

func (s *Store) codexSetupWhere(where string, arg string) (CodexSetup, error) {
	var c CodexSetup
	var thread, token, boot, fingerprint, reservation sql.NullString
	var pid, pgid sql.NullInt64
	var exit int
	err := s.db.QueryRow(`SELECT id,alias,environment,cli_path,workspace,state,thread_id,exit_observed,pid,pgid,start_token,boot_id,recovery_fingerprint,recovery_reservation FROM codex_setups WHERE `+where, arg).
		Scan(&c.ID, &c.Alias, &c.Environment, &c.CLI, &c.Workspace, &c.State, &thread, &exit, &pid, &pgid, &token, &boot, &fingerprint, &reservation)
	c.ThreadID = thread.String
	c.ExitObserved = exit != 0
	c.Observation = platform.Observation{PID: int(pid.Int64), PGID: int(pgid.Int64), StartToken: token.String, BootID: boot.String}
	c.RecoveryFingerprint, c.RecoveryReservation = fingerprint.String, reservation.String
	return c, err
}

// Codex recovery modes. close retires the setup and closes the session; resume
// keeps the session open so a new owner can resume the saved thread.
const (
	CodexRecoveryClose  = "close"
	CodexRecoveryResume = "resume"
)

// verifiedCodexTurns counts turns of this setup that completed with a stored,
// linked reply. Codex saves a thread to disk only once it has a turn, so a
// thread without one cannot be resumed.
func verifiedCodexTurns(tx *sql.Tx, id string) (int, error) {
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM codex_turns ct JOIN attempts a ON a.token=ct.attempt_token JOIN verifications v ON v.attempt_token=a.token JOIN replies r ON r.id=v.reply_id AND r.attempt_token=a.token AND r.reply_to=a.request_id WHERE ct.setup_id=? AND ct.turn_id IS NOT NULL AND a.state='turn_completed' AND v.identity_confirmed=1 AND v.provider_ok=1`, id).Scan(&n)
	return n, err
}

// PrepareCodexRecovery records the pending recovery before the registry entry
// is removed. Repeating it with the same reservation and mode is a no-op.
func (s *Store) PrepareCodexRecovery(id, fingerprint, reservation, proof, mode string) error {
	if mode != CodexRecoveryClose && mode != CodexRecoveryResume {
		return fmt.Errorf("%w: unknown recovery mode", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		var state string
		var old, oldMode, thread, key sql.NullString
		if err := tx.QueryRow(`SELECT state,recovery_fingerprint,recovery_mode,thread_id,session_key FROM codex_setups WHERE id=?`, id).Scan(&state, &old, &oldMode, &thread, &key); err != nil {
			return err
		}
		if state == "retiring" {
			if old.String != fingerprint {
				return fmt.Errorf("%w: pending recovery reservation changed", ErrRecoveryBlocked)
			}
			// A recovery prepared by an older client has no mode; it closes.
			pending := oldMode.String
			if pending == "" {
				pending = CodexRecoveryClose
			}
			if pending != mode {
				return fmt.Errorf("%w: a recovery in %s mode is pending; rerun it the same way", ErrRecoveryBlocked, pending)
			}
			return nil
		}
		if mode == CodexRecoveryResume {
			if !thread.Valid || !key.Valid {
				return fmt.Errorf("%w: no thread was created; nothing to resume", ErrRecoveryBlocked)
			}
			var decision sql.NullString
			if err := tx.QueryRow(`SELECT decision FROM sessions WHERE key=?`, key.String).Scan(&decision); err != nil {
				return err
			}
			if decision.Valid {
				return fmt.Errorf("%w: session is closed", ErrRecoveryBlocked)
			}
			n, err := verifiedCodexTurns(tx, id)
			if err != nil {
				return err
			}
			if n < 1 {
				return fmt.Errorf("%w: the thread has no verified turn, so Codex has not saved it; recover without --resume", ErrRecoveryBlocked)
			}
		}
		if err := codexStep(tx, id, "retiring", proof+"; mode "+mode, "uncertain"); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE codex_setups SET recovery_fingerprint=?,recovery_reservation=?,recovery_mode=? WHERE id=?`, fingerprint, reservation, mode, id)
		return err
	})
}

// FinishCodexRecovery runs after the registry entry is removed. In close mode
// it closes the session without retry. In resume mode the setup becomes
// resumable: the session stays open, any attempt the old owner held stays
// uncertain but no longer locks the session, and the queue is paused. Either
// way attempts, replies and uncertain messages are kept and nothing is resent.
func (s *Store) FinishCodexRecovery(id, fingerprint string) error {
	return s.tx(func(tx *sql.Tx) error {
		var old, key, mode sql.NullString
		if err := tx.QueryRow(`SELECT recovery_fingerprint,session_key,recovery_mode FROM codex_setups WHERE id=?`, id).Scan(&old, &key, &mode); err != nil {
			return err
		}
		if old.String != fingerprint {
			return fmt.Errorf("%w: pending recovery reservation changed", ErrRecoveryBlocked)
		}
		if mode.String == CodexRecoveryResume {
			return finishCodexResumeRecovery(tx, id, key.String)
		}
		if err := codexStep(tx, id, "retired", "reservation removed after a proven reboot; session closed without retry", "retiring"); err != nil {
			return err
		}
		if key.Valid {
			if _, err := tx.Exec(`UPDATE sessions SET decision='closed_without_retry' WHERE key=?`, key.String); err != nil {
				return err
			}
		}
		return nil
	})
}

func finishCodexResumeRecovery(tx *sql.Tx, id, key string) error {
	rows, err := tx.Query(`SELECT ct.attempt_token FROM codex_turns ct JOIN sessions s ON s.active_token=ct.attempt_token WHERE ct.setup_id=? AND s.key=?`, id, key)
	if err != nil {
		return err
	}
	var held []string
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			rows.Close()
			return err
		}
		held = append(held, token)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	const reason = "owner lost; recovered after a proven reboot; never resent"
	for _, token := range held {
		row, err := owned(tx, token, "reserved", "launching", "running", "uncertain")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO verifications(attempt_token,identity_confirmed,reply_id,exit_observed,exit_code,group_empty,provider_ok,detail) VALUES (?,?,(SELECT id FROM replies WHERE attempt_token=?),0,NULL,0,0,?)`, token, boolInt(row.state == "running"), token, reason); err != nil {
			return err
		}
		if row.state != "uncertain" || !row.mcpRevoked {
			if err := revoke(tx, row, "uncertain", reason); err != nil {
				return err
			}
		}
	}
	// Only the old owner's attempt is released; any other lock stays.
	if _, err := tx.Exec(`UPDATE sessions SET active_token=NULL WHERE key=? AND active_token IN (SELECT attempt_token FROM codex_turns WHERE setup_id=?)`, key, id); err != nil {
		return err
	}
	if err := codexStep(tx, id, "resumable", "reservation removed after a proven reboot; session kept open; resume-codex-session starts a new owner", "retiring"); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE control SET paused=1 WHERE id=1`); err != nil {
		return err
	}
	return insertEvent(tx, "codex_resumable", key, "", "uncertain messages kept; nothing resent; queue paused; no model call")
}

// BeginCodexResume starts a new owner for the saved thread of a resumable
// setup. It is committed before the reservation and exec, like
// MarkCodexSetupLaunching. cli replaces the stored CLI when it is not empty,
// for example after a Codex update moved the executable. The old process
// observation is dropped: a reboot since its launch was proven.
func (s *Store) BeginCodexResume(alias, cli string) (CodexSetup, error) {
	if cli != "" {
		var err error
		if cli, err = canonicalCLI(cli); err != nil {
			return CodexSetup{}, err
		}
	}
	var id string
	err := s.tx(func(tx *sql.Tx) error {
		var thread, key, stored sql.NullString
		err := tx.QueryRow(`SELECT id,thread_id,session_key,cli_path FROM codex_setups WHERE alias=? AND state='resumable'`, alias).Scan(&id, &thread, &key, &stored)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: no resumable Codex session has this alias; run recover-codex --resume first", ErrRejected)
		}
		if err != nil {
			return err
		}
		if !thread.Valid || !key.Valid {
			return fmt.Errorf("%w: resumable setup has no thread", ErrRejected)
		}
		var decision sql.NullString
		var active sql.NullString
		var mode string
		if err := tx.QueryRow(`SELECT decision,active_token,start_mode FROM sessions WHERE key=?`, key.String).Scan(&decision, &active, &mode); err != nil {
			return err
		}
		if decision.Valid || active.Valid || mode != "codex_owned" {
			return fmt.Errorf("%w: session is closed or locked", ErrRejected)
		}
		if cli == "" {
			if cli, err = canonicalCLI(stored.String); err != nil {
				return fmt.Errorf("%w: stored Codex CLI is no longer an executable; pass --cli", ErrRejected)
			}
		}
		if err := codexStep(tx, id, "launching", "resume of thread "+thread.String+"; process may start", "resumable"); err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE codex_setups SET cli_path=?,pid=NULL,pgid=NULL,start_token=NULL,boot_id=NULL,exit_observed=0,exit_code=NULL,config_fingerprint=NULL WHERE id=?`, cli, id)
		return err
	})
	if err != nil {
		return CodexSetup{}, err
	}
	return s.CodexSetup(id)
}

// CodexResumeNotStarted returns a resume to resumable after a definite OS
// refusal before the executable ran. The alias and session are kept.
func (s *Store) CodexResumeNotStarted(id, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		var pid sql.NullInt64
		var thread sql.NullString
		if err := tx.QueryRow(`SELECT pid,thread_id FROM codex_setups WHERE id=?`, id).Scan(&pid, &thread); err != nil {
			return err
		}
		if pid.Valid || !thread.Valid {
			return fmt.Errorf("%w: not a resume that never started", ErrRejected)
		}
		return codexStep(tx, id, "resumable", reason, "launching")
	})
}

// PrepareCodexThreadResume is committed BEFORE thread/resume is written. From
// here a lost response or a crash is uncertain; thread/resume is never re-sent.
func (s *Store) PrepareCodexThreadResume(id string) error {
	return s.tx(func(tx *sql.Tx) error {
		var thread sql.NullString
		if err := tx.QueryRow(`SELECT thread_id FROM codex_setups WHERE id=?`, id).Scan(&thread); err != nil {
			return err
		}
		if !thread.Valid {
			return fmt.Errorf("%w: no thread to resume", ErrRejected)
		}
		return codexStep(tx, id, "resume_requested", "thread/resume sent once; no automatic retry", "configured")
	})
}

// CompleteCodexResume records that the new process resumed exactly the stored
// thread. The owner is idle and serves the session like a new one.
func (s *Store) CompleteCodexResume(id, threadID string) error {
	return s.tx(func(tx *sql.Tx) error {
		var thread, key sql.NullString
		if err := tx.QueryRow(`SELECT thread_id,session_key FROM codex_setups WHERE id=?`, id).Scan(&thread, &key); err != nil {
			return err
		}
		if !thread.Valid || !key.Valid || thread.String != threadID {
			return fmt.Errorf("%w: resumed thread differs from the stored one", ErrRejected)
		}
		var decision sql.NullString
		var mode string
		if err := tx.QueryRow(`SELECT decision,start_mode FROM sessions WHERE key=?`, key.String).Scan(&decision, &mode); err != nil {
			return err
		}
		if decision.Valid || mode != "codex_owned" {
			return fmt.Errorf("%w: session is closed", ErrRejected)
		}
		if err := codexStep(tx, id, "created", "thread resumed; owned by live process", "resume_requested"); err != nil {
			return err
		}
		return insertEvent(tx, "codex_thread_resumed", key.String, "", "same thread id; model not called")
	})
}

// CheckPendingCodexRecovery refuses startup while a recover-codex is half done.
func (s *Store) CheckPendingCodexRecovery() error {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM codex_setups WHERE state='retiring'`).Scan(&n); err != nil {
		return err
	}
	if n != 0 {
		return fmt.Errorf("%w: Codex recovery is pending; rerun the same recover-codex command", ErrRecoveryBlocked)
	}
	return nil
}

// stopInheritedCodexSetups runs at startup. The previous supervisor's pipes to
// any app-server are gone, so every active setup is uncertain. An intent that
// never reached launching started nothing and is released.
func stopInheritedCodexSetups(tx *sql.Tx) error {
	if _, err := tx.Exec(`UPDATE codex_setups SET state='not_started',detail='supervisor restarted before launch',updated_at=? WHERE state='intent'`, now()); err != nil {
		return err
	}
	r, err := tx.Exec(`UPDATE codex_setups SET state='uncertain',detail='supervisor restarted; owner process lost; no retry',updated_at=? WHERE state IN ('launching','started','configured','thread_requested','resume_requested','created','busy')`, now())
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 0 {
		return insertEvent(tx, "codex_setup_inherited", "", "", fmt.Sprintf("%d setups marked uncertain; reservations kept", n))
	}
	return nil
}
