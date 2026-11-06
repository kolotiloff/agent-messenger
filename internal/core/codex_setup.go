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
//	created          thread ID stored; the live process owns it
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

// CompleteCodexSetup stores the server-issued thread ID. The live process that
// returned it owns the thread. No session row is created yet, so no message
// can be queued or claimed for it until delivery to this owner exists.
func (s *Store) CompleteCodexSetup(id, threadID string) error {
	if strings.TrimSpace(threadID) != threadID || threadID == "" || len(threadID) > 128 {
		return fmt.Errorf("%w: invalid thread id", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		var environment string
		if err := tx.QueryRow(`SELECT environment FROM codex_setups WHERE id=?`, id).Scan(&environment); err != nil {
			return err
		}
		var registered int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM sessions WHERE provider='codex' AND environment=? AND conversation_id=?`, environment, threadID).Scan(&registered); err != nil {
			return err
		}
		if registered != 0 {
			return fmt.Errorf("%w: thread already registered", ErrRejected)
		}
		if err := codexStep(tx, id, "created", "thread issued; owned by live process", "thread_requested"); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE codex_setups SET thread_id=? WHERE id=?`, threadID, id)
		return err
	})
}

// MarkCodexSetupUncertain keeps the alias and the process reservation. It is
// used for every failure after launching began.
func (s *Store) MarkCodexSetupUncertain(id, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		return codexStep(tx, id, "uncertain", reason, "launching", "started", "configured", "thread_requested", "created", "uncertain")
	})
}

// RecordCodexSetupExit notes that the owner process exited. That does not prove
// its descendants stopped, so the setup becomes uncertain and keeps its alias.
func (s *Store) RecordCodexSetupExit(id string, code int) error {
	return s.tx(func(tx *sql.Tx) error {
		if err := codexStep(tx, id, "uncertain", "app-server exit observed; descendants not proven stopped", "launching", "started", "configured", "thread_requested", "created", "uncertain"); err != nil {
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
	r, err := tx.Exec(`UPDATE codex_setups SET state='uncertain',detail='supervisor restarted; owner process lost; no retry',updated_at=? WHERE state IN ('launching','started','configured','thread_requested','created')`, now())
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 0 {
		return insertEvent(tx, "codex_setup_inherited", "", "", fmt.Sprintf("%d setups marked uncertain; reservations kept", n))
	}
	return nil
}
