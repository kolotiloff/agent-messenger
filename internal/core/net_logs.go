package core

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Logs of other computers (T-057, D-083). A coordinator asks another
// computer of its network for the end of its Messenger log. The request and
// the reply travel through the mailbox like task messages. The other
// computer answers only when its owner turned on «Отдавать журнал по сети»
// (off by default); otherwise it replies with a refusal. The client of the
// answering computer reads and cleans the log (the supervisor), the store
// only carries the request and the reply.

// Limits of a shared log.
const (
	DefaultLogLines = 200
	MaxLogLines     = 1000
	MaxLogText      = 64 << 10
)

const netLogsSchema = `
CREATE TABLE IF NOT EXISTS net_logs (
 id TEXT PRIMARY KEY,
 broker TEXT NOT NULL,
 peer TEXT NOT NULL,
 state TEXT NOT NULL,
 text TEXT NOT NULL,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS net_log_requests (
 broker TEXT NOT NULL,
 peer TEXT NOT NULL,
 id TEXT NOT NULL,
 lines INTEGER NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY (broker, peer, id)
);`

// NetLog is a log asked from another computer. State is waiting, done
// (Text is the log) or refused (Text is the reason).
type NetLog struct {
	ID       string `json:"request_id"`
	Computer string `json:"computer"`
	State    string `json:"state"`
	Text     string `json:"text,omitempty"`
}

// NetLogRequest is another computer's request for this computer's log.
type NetLogRequest struct {
	Broker, Peer, ID string
	Lines            int
}

func logLines(n int) int {
	if n <= 0 {
		return DefaultLogLines
	}
	if n > MaxLogLines {
		return MaxLogLines
	}
	return n
}

// NetLogShare reports whether this computer gives its log to the other
// computers of its network. Off until the owner turns it on.
func (s *Store) NetLogShare() (bool, error) {
	var on bool
	err := s.tx(func(tx *sql.Tx) error {
		v, err := metaValue(tx, "net_log_share")
		on = v == "1"
		return err
	})
	return on, err
}

// SetNetLogShare turns giving the log on or off.
func (s *Store) SetNetLogShare(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return s.tx(func(tx *sql.Tx) error { return setMeta(tx, "net_log_share", v) })
}

// RequestNetLog asks another computer of the network for the last lines of
// its log and returns the request's id.
func (s *Store) RequestNetLog(computer string, lines int) (string, error) {
	var id string
	err := s.tx(func(tx *sql.Tx) error {
		broker, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
		}
		if broker == "" {
			return fmt.Errorf("%w: this computer is in no network", ErrRejected)
		}
		if _, ok, err := netPeer(tx, computer); err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("%w: unknown computer %q; list_agents shows the computers of the network", ErrRejected, computer)
		}
		// Old answers are not kept for long.
		if _, err := tx.Exec(`DELETE FROM net_logs WHERE created_at<?`, time.Now().UTC().Add(-24*time.Hour).Format(time.RFC3339Nano)); err != nil {
			return err
		}
		r, err := randomID()
		if err != nil {
			return err
		}
		id = "log-" + r[:16]
		at := now()
		if _, err := tx.Exec(`INSERT INTO net_logs(id,broker,peer,state,text,created_at,updated_at) VALUES (?,?,?,'waiting','',?,?)`, id, broker, computer, at, at); err != nil {
			return err
		}
		return enqueueNet(tx, broker, computer, "log."+id, id, netBody{Kind: "log_request", Task: id, Lines: logLines(lines)})
	})
	return id, err
}

// NetLog returns a log request and its answer, if any.
func (s *Store) NetLog(id string) (NetLog, error) {
	var l NetLog
	var peer string
	err := s.tx(func(tx *sql.Tx) error {
		err := tx.QueryRow(`SELECT id,peer,state,text FROM net_logs WHERE id=?`, id).Scan(&l.ID, &peer, &l.State, &l.Text)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown log request %q", ErrRejected, id)
		}
		if err != nil {
			return err
		}
		l.Computer = peerName(tx, peer)
		return nil
	})
	return l, err
}

// NetLogRequests lists the other computers' requests for this computer's
// log, oldest first.
func (s *Store) NetLogRequests(broker string) ([]NetLogRequest, error) {
	rows, err := s.db.Query(`SELECT broker,peer,id,lines FROM net_log_requests WHERE broker=? ORDER BY created_at, id LIMIT 20`, broker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NetLogRequest
	for rows.Next() {
		var r NetLogRequest
		if err := rows.Scan(&r.Broker, &r.Peer, &r.ID, &r.Lines); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReplyNetLog answers a request with the log, or with the reason it is not
// given, and forgets the request.
func (s *Store) ReplyNetLog(r NetLogRequest, text, refusal string) error {
	return s.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM net_log_requests WHERE broker=? AND peer=? AND id=?`, r.Broker, r.Peer, r.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		body := netBody{Kind: "log", Task: r.ID, Text: clip(text, MaxLogText)}
		if refusal != "" {
			body = netBody{Kind: "log", Task: r.ID, Reason: refusal}
		}
		return enqueueNet(tx, r.Broker, r.Peer, "log-reply."+r.ID, r.ID, body)
	})
}

// applyLogRequest records another computer's request; the client answers it.
func applyLogRequest(tx *sql.Tx, from netLink, b netBody) error {
	_, err := tx.Exec(`INSERT INTO net_log_requests(broker,peer,id,lines,created_at) VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING`,
		from.broker, from.peer, clip(from.task, 80), logLines(b.Lines), now())
	return err
}

// applyLogReply records the answer to this computer's request, only from
// the computer that was asked.
func applyLogReply(tx *sql.Tx, from netLink, b netBody) error {
	state, text := "done", b.Text
	if b.Reason != "" {
		state, text = "refused", b.Reason
	}
	_, err := tx.Exec(`UPDATE net_logs SET state=?,text=?,updated_at=? WHERE id=? AND broker=? AND peer=? AND state='waiting'`,
		state, clip(text, MaxLogText), now(), from.task, from.broker, from.peer)
	return err
}

// logUndeliverable handles a log message the broker refused because its
// recipient left the network: the request is answered with the reason; a
// reply is dropped.
func logUndeliverable(tx *sql.Tx, kind, id string) error {
	if kind != "log_request" {
		return nil
	}
	_, err := tx.Exec(`UPDATE net_logs SET state='refused',text=?,updated_at=? WHERE id=? AND state='waiting'`,
		"the other computer is no longer in the network", now(), id)
	return err
}
