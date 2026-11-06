package core

import (
	"database/sql"
	"errors"
	"fmt"
)

// MaxAutoReplyHops limits how many replies in one chain are delivered to
// another agent automatically. A chain starts with a message that was not
// itself a delivered reply (hops 0). Past the limit the reply is still stored
// and shown, but it is not queued for anyone, so two agents cannot keep
// answering each other forever.
const MaxAutoReplyHops = 3

// routeReply runs in the transaction that stores the reply. When the reply's
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
