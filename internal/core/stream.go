package core

import (
	"database/sql"
	"errors"
	"fmt"
)

const streamSchema = `
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

// The session's active_token is retained even between turns. Only this live
// process may rotate it, atomically, to the next attempt. ClaimNext cannot.
func (s *Store) BeginStream(token string) error {
	return s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, token, "launching")
		if err != nil {
			return err
		}
		receipt, err := newID()
		if err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO stream_runs(id,session_key,state,current_token) VALUES (?,?,'starting',?)`, token, row.sessionKey, token); err != nil {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO stream_turns(attempt_token,run_id,receipt) VALUES (?,?,?)`, token, token, receipt); err != nil {
			return err
		}
		return insertEvent(tx, "stream_reserved", row.sessionKey, token, "process owner retained across turns")
	})
}

// FailStreamStart is only for a definite OS refusal before the target executable
// ran, after its provisional user reservation has been removed. It never retries
// the failed message. Attempts and journal history remain; only unused stream
// transport rows are removed so a later, explicitly queued message can start.
func (s *Store) FailStreamStart(run, reason string) error {
	return s.tx(func(tx *sql.Tx) error {
		row, err := owned(tx, run, "launching", "uncertain")
		if err != nil {
			return err
		}
		var safe int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM stream_runs r JOIN stream_turns t ON t.run_id=r.id JOIN attempts a ON a.token=t.attempt_token WHERE r.id=? AND t.attempt_token=? AND t.sent_intent=0 AND COALESCE(a.pid,0)=0 AND NOT EXISTS(SELECT 1 FROM replies WHERE attempt_token=?)`, run, run, run).Scan(&safe); err != nil {
			return err
		}
		if safe != 1 {
			return ErrRejected
		}
		if err := revoke(tx, row, "not_started", reason); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM stream_turns WHERE run_id=?`, run); err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM stream_runs WHERE id=?`, run); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET active_token=NULL WHERE key=? AND active_token=?`, row.sessionKey, run); err != nil {
			return err
		}
		return insertEvent(tx, "stream_not_started", row.sessionKey, run, "definite exec refusal; no automatic retry")
	})
}

func streamCurrent(tx *sql.Tx, run string) (attemptRow, error) {
	var token, state string
	if err := tx.QueryRow(`SELECT COALESCE(current_token,''),state FROM stream_runs WHERE id=?`, run).Scan(&token, &state); err != nil {
		return attemptRow{}, err
	}
	if (state != "starting" && state != "active") || token == "" {
		return attemptRow{}, fmt.Errorf("%w: stream is %s", ErrRejected, state)
	}
	return owned(tx, token, "launching", "running")
}

// PrepareStreamSend is committed BEFORE writing stdin. Failure or restart after
// this point is uncertain, never an automatic resend. Caller serializes this
// operation and the write with pause/stop/shutdown.
func (s *Store) PrepareStreamSend(run string) (string, error) {
	var token string
	err := s.tx(func(tx *sql.Tx) error {
		p, err := isPaused(tx)
		if err != nil {
			return err
		}
		if p {
			return ErrPaused
		}
		row, err := streamCurrent(tx, run)
		if err != nil {
			return err
		}
		r, err := tx.Exec(`UPDATE stream_turns SET sent_intent=1 WHERE attempt_token=? AND sent_intent=0`, row.token)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("%w: turn already sent or uncertain", ErrRejected)
		}
		token = row.token
		return insertEvent(tx, "stream_send_intent", row.sessionKey, row.token, "no automatic replay")
	})
	return token, err
}

func (s *Store) ConfirmStreamIdentity(run, conversationID string) error {
	return s.tx(func(tx *sql.Tx) error {
		row, err := streamCurrent(tx, run)
		if err != nil {
			return err
		}
		var expected string
		var sent int
		if err := tx.QueryRow(`SELECT conversation_id FROM sessions WHERE key=? AND provider='claude'`, row.sessionKey).Scan(&expected); err != nil {
			return err
		}
		if err := tx.QueryRow(`SELECT sent_intent FROM stream_turns WHERE attempt_token=?`, row.token).Scan(&sent); err != nil {
			return err
		}
		if conversationID == "" || conversationID != expected || sent != 1 || row.state != "launching" || row.mcpRevoked {
			return fmt.Errorf("%w: stream initialization mismatch or duplicate", ErrRejected)
		}
		if _, err := tx.Exec(`UPDATE attempts SET state='running' WHERE token=?`, row.token); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state='running' WHERE id=?`, row.requestID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE stream_runs SET state='active' WHERE id=?`, run); err != nil {
			return err
		}
		return insertEvent(tx, "identity_confirmed", row.sessionKey, row.token, "same stream process")
	})
}

type StreamMessage struct {
	Text    string `json:"text"`
	Receipt string `json:"receipt"`
}

func (s *Store) StreamRead(run string) (StreamMessage, error) {
	var msg StreamMessage
	err := s.tx(func(tx *sql.Tx) error {
		row, err := streamCurrent(tx, run)
		if err != nil {
			return err
		}
		if _, err := mcpOpen(tx, row.token); err != nil {
			return err
		}
		if err := tx.QueryRow(`SELECT receipt FROM stream_turns WHERE attempt_token=? AND sent_intent=1`, row.token).Scan(&msg.Receipt); err != nil {
			return err
		}
		msg.Text = row.text
		_, err = tx.Exec(`UPDATE stream_turns SET read_seen=1 WHERE attempt_token=?`, row.token)
		return err
	})
	return msg, err
}

func (s *Store) StreamReply(run, receipt, text string) (Reply, error) {
	var reply Reply
	if len(text) < 1 || len(text) > 2000 {
		return reply, ErrRejected
	}
	err := s.tx(func(tx *sql.Tx) error {
		row, err := streamCurrent(tx, run)
		if err != nil {
			return err
		}
		if _, err := mcpOpen(tx, row.token); err != nil {
			return err
		}
		var expected string
		var read, sent int
		if err := tx.QueryRow(`SELECT receipt,read_seen,sent_intent FROM stream_turns WHERE attempt_token=?`, row.token).Scan(&expected, &read, &sent); err != nil {
			return err
		}
		if receipt == "" || receipt != expected || read != 1 || sent != 1 {
			return fmt.Errorf("%w: stale or unread receipt", ErrRejected)
		}
		reply, err = replyTx(tx, row, row.token, text)
		return err
	})
	return reply, err
}

// A successful turn has NO exit code. Its process is still alive and reserved.
func (s *Store) CompleteStreamTurn(run string, providerOK bool) error {
	if !providerOK {
		return ErrNotVerified
	}
	return s.tx(func(tx *sql.Tx) error {
		row, err := streamCurrent(tx, run)
		if err != nil {
			return err
		}
		if _, err := mcpOpen(tx, row.token); err != nil {
			return err
		}
		var reply string
		if err := tx.QueryRow(`SELECT id FROM replies WHERE attempt_token=? AND reply_to=?`, row.token, row.requestID).Scan(&reply); err != nil {
			return fmt.Errorf("%w: bound reply missing", ErrNotVerified)
		}
		if _, err := tx.Exec(`INSERT INTO verifications(attempt_token,identity_confirmed,reply_id,exit_observed,exit_code,group_empty,provider_ok,detail) VALUES (?,1,?,0,NULL,0,1,'turn finished; process owner retained')`, row.token, reply); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE attempts SET state='turn_completed',mcp_revoked=1 WHERE token=?`, row.token); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state='completed' WHERE id=?`, row.requestID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE stream_runs SET state='idle',current_token=NULL WHERE id=?`, run); err != nil {
			return err
		}
		return insertEvent(tx, "stream_turn_completed", row.sessionKey, row.token, "session remains owned by live process")
	})
}

func (s *Store) NextStreamTurn(run string) (string, error) {
	var next string
	err := s.tx(func(tx *sql.Tx) error {
		p, err := isPaused(tx)
		if err != nil {
			return err
		}
		if p {
			return ErrPaused
		}
		var key, state, previous, alias string
		err = tx.QueryRow(`SELECT r.session_key,r.state,s.active_token,a.alias FROM stream_runs r JOIN sessions s ON s.key=r.session_key JOIN attempts a ON a.token=s.active_token WHERE r.id=? AND s.decision IS NULL`, run).Scan(&key, &state, &previous, &alias)
		if err != nil {
			return err
		}
		if state != "idle" {
			return ErrBusy
		}
		var count int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM stream_turns st JOIN attempts a ON a.token=st.attempt_token WHERE st.run_id=? AND a.token=? AND a.state='turn_completed' AND a.mcp_revoked=1`, run, previous).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return ErrRejected
		}
		var request string
		err = tx.QueryRow(`SELECT id FROM requests WHERE session_key=? AND state='queued' ORDER BY rowid LIMIT 1`, key).Scan(&request)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrEmpty
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE sessions SET active_token=NULL WHERE key=?`, key); err != nil {
			return err
		}
		next, err = reserve(tx, key, request, alias)
		if err != nil {
			return err
		}
		receipt, err := newID()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO stream_turns(attempt_token,run_id,receipt) VALUES (?,?,?)`, next, run, receipt); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE attempts SET state='launching',pid=(SELECT pid FROM attempts WHERE token=?),pgid=(SELECT pgid FROM attempts WHERE token=?),start_token=(SELECT start_token FROM attempts WHERE token=?),boot_id=(SELECT boot_id FROM attempts WHERE token=?) WHERE token=?`, run, run, run, run, next); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE requests SET state='launching' WHERE id=?`, request); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE stream_runs SET state='starting',current_token=? WHERE id=?`, next, run); err != nil {
			return err
		}
		return insertEvent(tx, "stream_next_turn", key, next, "same CLI; no new process")
	})
	return next, err
}

func stopStream(tx *sql.Tx, run, reason string) error {
	var key string
	var token sql.NullString
	if err := tx.QueryRow(`SELECT session_key,current_token FROM stream_runs WHERE id=?`, run).Scan(&key, &token); err != nil {
		return err
	}
	if token.Valid {
		row, err := owned(tx, token.String, "launching", "running", "uncertain")
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO verifications(attempt_token,identity_confirmed,reply_id,exit_observed,exit_code,group_empty,provider_ok,detail) VALUES (?,?,(SELECT id FROM replies WHERE attempt_token=?),0,NULL,0,0,?)`, row.token, boolInt(row.state == "running"), row.token, reason); err != nil {
			return err
		}
		if err := revoke(tx, row, "uncertain", reason); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`UPDATE stream_runs SET state='uncertain',current_token=NULL WHERE id=?`, run); err != nil {
		return err
	}
	return insertEvent(tx, "stream_stopped", key, run, reason+"; process reservation retained")
}

func (s *Store) StopStream(run, reason string) error {
	return s.tx(func(tx *sql.Tx) error { return stopStream(tx, run, reason) })
}

func stopInheritedStreams(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id FROM stream_runs WHERE state!='uncertain'`)
	if err != nil {
		return err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, id := range ids {
		if err := stopStream(tx, id, "supervisor restarted; no replay or second process"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) RecordStreamExit(run string, code int) error {
	return s.tx(func(tx *sql.Tx) error {
		if err := stopStream(tx, run, "CLI exit observed"); err != nil {
			return err
		}
		_, err := tx.Exec(`UPDATE stream_runs SET exit_observed=1,exit_code=? WHERE id=?`, code, run)
		return err
	})
}

func (s *Store) StreamRunForAlias(alias string) (string, error) {
	var run string
	err := s.db.QueryRow(`SELECT r.id FROM stream_runs r JOIN aliases a ON a.session_key=r.session_key WHERE a.alias=?`, alias).Scan(&run)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return run, err
}

func (s *Store) SetNewClaudeSession(alias string) error {
	return s.tx(func(tx *sql.Tx) error {
		key, _, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		r, err := tx.Exec(`UPDATE sessions SET start_mode='new' WHERE key=? AND provider='claude' AND active_token IS NULL AND NOT EXISTS(SELECT 1 FROM attempts WHERE session_key=?)`, key, key)
		if err != nil {
			return err
		}
		n, err := r.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrRejected
		}
		return nil
	})
}
