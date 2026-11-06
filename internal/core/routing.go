package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// MaxAutoReplyHops limits how many replies in one chain are delivered to
// another agent automatically. A chain starts with a message that was not
// itself a delivered reply (hops 0). Past the limit the reply is still stored
// and shown, but it is not queued for anyone, so two agents cannot keep
// answering each other forever.
const MaxAutoReplyHops = 3

// routeReply runs in the transaction that publishes a verified reply. When the reply's
// recipient is the alias of another open local session, the reply becomes a
// new queued message for it, from the replying alias, linked to the request it
// answers. The message ID is derived from the reply ID, so it exists exactly
// once. Replies to unregistered senders (for example the user) are not routed.
func routeReply(tx *sql.Tx, row attemptRow, reply Reply) error {
	var key string
	var decision sql.NullString
	err := tx.QueryRow(`SELECT a.session_key,s.decision FROM aliases a JOIN sessions s ON s.key=a.session_key WHERE a.alias=?`, reply.Recipient).Scan(&key, &decision)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if key == row.sessionKey {
		return nil
	}
	if decision.Valid {
		return insertEvent(tx, "reply_not_routed", key, row.token, "recipient session is closed; reply "+reply.ID)
	}
	var hops int
	if err := tx.QueryRow(`SELECT hops FROM requests WHERE id=?`, row.requestID).Scan(&hops); err != nil {
		return err
	}
	if hops+1 > MaxAutoReplyHops {
		return insertEvent(tx, "auto_reply_limit", key, row.token, fmt.Sprintf("reply %s kept; limit of %d automatic replies reached", reply.ID, MaxAutoReplyHops))
	}
	id := "reply-" + reply.ID
	if _, err := tx.Exec(`INSERT INTO requests(id,session_key,sender,text,state,created_at,in_reply_to,hops) VALUES (?,?,?,?,'queued',?,?,?)`,
		id, key, reply.Sender, reply.Text, now(), row.requestID, hops+1); err != nil {
		return err
	}
	return insertEvent(tx, "reply_routed", key, row.token, id)
}

// MaxSendsPerTurn bounds how many new messages one turn may start.
const MaxSendsPerTurn = 3

// otherAgents lists aliases of open sessions other than this one, so an agent
// knows whom it can write to. Bounded; no IDs or paths.
func otherAgents(tx *sql.Tx, sessionKey string) ([]string, error) {
	rows, err := tx.Query(`SELECT a.alias FROM aliases a JOIN sessions s ON s.key=a.session_key WHERE s.key!=? AND s.decision IS NULL ORDER BY a.alias LIMIT 50`, sessionKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var alias string
		if err := rows.Scan(&alias); err != nil {
			return nil, err
		}
		out = append(out, alias)
	}
	return out, rows.Err()
}

// sendTx starts a new message from the agent handling row to another open
// local alias. It counts as one more step of the current chain, so the
// automatic-reply limit also bounds agent-initiated messages.
func sendTx(tx *sql.Tx, row attemptRow, to, text string) (string, error) {
	if len(text) < 1 || len(text) > 2000 || strings.TrimSpace(to) == "" {
		return "", fmt.Errorf("%w: send requires a recipient alias and 1..2000 characters", ErrRejected)
	}
	var key string
	var decision sql.NullString
	err := tx.QueryRow(`SELECT a.session_key,s.decision FROM aliases a JOIN sessions s ON s.key=a.session_key WHERE a.alias=?`, to).Scan(&key, &decision)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: unknown recipient", ErrRejected)
	}
	if err != nil {
		return "", err
	}
	if key == row.sessionKey || decision.Valid {
		return "", fmt.Errorf("%w: recipient is this session or closed", ErrRejected)
	}
	var hops, sent int
	if err := tx.QueryRow(`SELECT hops FROM requests WHERE id=?`, row.requestID).Scan(&hops); err != nil {
		return "", err
	}
	if hops+1 > MaxAutoReplyHops {
		return "", fmt.Errorf("%w: limit of %d automatic steps in this chain reached", ErrRejected, MaxAutoReplyHops)
	}
	if err := tx.QueryRow(`SELECT COUNT(*) FROM requests WHERE in_reply_to=? AND id LIKE 'send-%'`, row.requestID).Scan(&sent); err != nil {
		return "", err
	}
	if sent >= MaxSendsPerTurn {
		return "", fmt.Errorf("%w: at most %d messages per turn", ErrRejected, MaxSendsPerTurn)
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	id = "send-" + id
	// Held until the turn is verified, like its reply (T-022).
	if _, err := tx.Exec(`INSERT INTO requests(id,session_key,sender,text,state,created_at,in_reply_to,hops) VALUES (?,?,?,?,'held',?,?,?)`,
		id, key, row.alias, text, now(), row.requestID, hops+1); err != nil {
		return "", err
	}
	return id, insertEvent(tx, "agent_send", key, row.token, id+"; held until the turn is verified")
}

// StreamSend lets a persistent Claude turn start a message to another agent.
// It requires the turn's current receipt, like reply.
func (s *Store) StreamSend(run, receipt, to, text string) (string, error) {
	var id string
	err := s.tx(func(tx *sql.Tx) error {
		row, err := streamCurrent(tx, run)
		if err != nil {
			return err
		}
		if _, err := mcpOpen(tx, row.token); err != nil {
			return err
		}
		var expected string
		var read int
		if err := tx.QueryRow(`SELECT receipt,read_seen FROM stream_turns WHERE attempt_token=?`, row.token).Scan(&expected, &read); err != nil {
			return err
		}
		if receipt == "" || receipt != expected || read != 1 {
			return fmt.Errorf("%w: stale or unread receipt", ErrRejected)
		}
		id, err = sendTx(tx, row, to, text)
		return err
	})
	return id, err
}
