package core

import (
	"database/sql"
	"fmt"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const StreamRecoveryConfirmation = "preserve-uncertain"

const streamRecoverySchema = `
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

type StreamRecoveryTarget struct {
	RunID, SessionKey, Provider, Environment, ConversationID string
	Observation                                              platform.Observation
	Phase, Fingerprint, Reservation                          string
}

func recoveryTarget(tx *sql.Tx, alias, run string) (StreamRecoveryTarget, error) {
	t := StreamRecoveryTarget{RunID: run}
	var pid, pgid sql.NullInt64
	var start, boot sql.NullString
	err := tx.QueryRow(`SELECT s.key,s.provider,s.environment,s.conversation_id,a.pid,a.pgid,a.start_token,a.boot_id,COALESCE(c.phase,''),COALESCE(c.fingerprint,''),COALESCE(c.reservation,'')
 FROM (SELECT id,session_key FROM stream_runs UNION ALL SELECT id,session_key FROM archived_stream_runs) r
 JOIN sessions s ON s.key=r.session_key JOIN aliases x ON x.session_key=s.key JOIN attempts a ON a.token=r.id
 LEFT JOIN stream_recoveries c ON c.run_id=r.id WHERE r.id=? AND x.alias=? AND s.decision IS NULL`, run, alias).
		Scan(&t.SessionKey, &t.Provider, &t.Environment, &t.ConversationID, &pid, &pgid, &start, &boot, &t.Phase, &t.Fingerprint, &t.Reservation)
	if err != nil {
		return t, fmt.Errorf("%w: stream/alias binding not found: %v", ErrRecoveryBlocked, err)
	}
	t.Observation = platform.Observation{PID: int(pid.Int64), PGID: int(pgid.Int64), StartToken: start.String, BootID: boot.String}
	if t.Provider != "claude" || t.Observation.PID <= 1 || t.Observation.PGID <= 1 || t.Observation.StartToken == "" || t.Observation.BootID == "" {
		return t, fmt.Errorf("%w: incomplete Claude process observation", ErrRecoveryBlocked)
	}
	return t, nil
}

func (s *Store) StreamRecoveryTarget(alias, run string) (StreamRecoveryTarget, error) {
	var target StreamRecoveryTarget
	err := s.tx(func(tx *sql.Tx) error { var err error; target, err = recoveryTarget(tx, alias, run); return err })
	return target, err
}

// Prepare keeps the owner held. A crash before/after registry replacement is
// fenced by this pending row, checked by every serve/probe entry point.
func (s *Store) PrepareStreamRecovery(alias, run, fingerprint, reservation string, proof platform.Proof) error {
	if !proof.Proven || fingerprint == "" || reservation == "" {
		return ErrRecoveryBlocked
	}
	return s.tx(func(tx *sql.Tx) error {
		t, err := recoveryTarget(tx, alias, run)
		if err != nil {
			return err
		}
		if t.Phase != "" {
			if t.Phase == "pending" && t.Fingerprint == fingerprint && t.Reservation == reservation {
				return nil
			}
			return ErrRecoveryBlocked
		}
		var verified, owned int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM stream_turns st JOIN verifications v ON v.attempt_token=st.attempt_token JOIN attempts a ON a.token=st.attempt_token JOIN replies r ON r.id=v.reply_id AND r.attempt_token=a.token AND r.reply_to=a.request_id WHERE st.run_id=? AND a.state='turn_completed' AND v.identity_confirmed=1 AND v.provider_ok=1`, run).Scan(&verified); err != nil {
			return err
		}
		if verified < 1 {
			return fmt.Errorf("%w: this first recovery version requires at least one verified completed turn", ErrRecoveryBlocked)
		}
		if err := tx.QueryRow(`SELECT COUNT(*) FROM sessions s JOIN stream_turns t ON t.attempt_token=s.active_token WHERE s.key=? AND t.run_id=?`, t.SessionKey, run).Scan(&owned); err != nil {
			return err
		}
		if owned != 1 {
			return ErrRecoveryBlocked
		}
		if err := stopStream(tx, run, "explicit recovery after proven host reboot; no replay"); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO stream_recoveries(run_id,fingerprint,phase,proof,reservation) VALUES (?,?,'pending',?,?)`, run, fingerprint, proof.Detail, reservation); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE control SET paused=1 WHERE id=1`); err != nil {
			return err
		}
		return insertEvent(tx, "stream_recovery_prepared", t.SessionKey, run, proof.Detail)
	})
}

func (s *Store) FinishStreamRecovery(alias, run, fingerprint string) error {
	return s.tx(func(tx *sql.Tx) error {
		t, err := recoveryTarget(tx, alias, run)
		if err != nil {
			return err
		}
		if t.Fingerprint != fingerprint {
			return ErrRecoveryBlocked
		}
		if t.Phase == "done" {
			return nil
		}
		if t.Phase != "pending" {
			return ErrRecoveryBlocked
		}
		var held int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM sessions s JOIN stream_turns t ON t.attempt_token=s.active_token WHERE s.key=? AND t.run_id=?`, t.SessionKey, run).Scan(&held); err != nil {
			return err
		}
		if held != 1 {
			return ErrRecoveryBlocked
		}
		if _, err := tx.Exec(`INSERT INTO archived_stream_runs SELECT * FROM stream_runs WHERE id=?`, run); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO archived_stream_turns SELECT * FROM stream_turns WHERE run_id=?`, run); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM stream_turns WHERE run_id=?`, run); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM stream_runs WHERE id=?`, run); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET active_token=NULL,start_mode='resume' WHERE key=?`, t.SessionKey); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE stream_recoveries SET phase='done' WHERE run_id=?`, run); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE control SET paused=1 WHERE id=1`); err != nil {
			return err
		}
		return insertEvent(tx, "stream_recovered", t.SessionKey, run, "history archived; uncertain messages kept; exact resume; queue paused; no model call")
	})
}

func (s *Store) CheckPendingStreamRecovery() error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM stream_recoveries WHERE phase!='done'`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: stream recovery is pending; rerun the same recover-stream command", ErrRecoveryBlocked)
	}
	return nil
}
