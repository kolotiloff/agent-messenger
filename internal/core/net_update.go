package core

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Updating another computer of the network (T-078, D-100). The owner asks,
// from this computer's window, another computer to install the newest signed
// update. The request and the replies travel through the mailbox like the
// log requests (T-057). The other computer installs only while it runs no
// tasks; a Mac refuses (it updates in its own app). It replies twice: at
// once (updating, or refused with the reason) and, after its update helper
// finished, with the final result (done or failed). Nothing is retried on
// its own: an unclear result is reported as failed, with the reason.

// UpdateWait is how long a request may stay unanswered before the owner may
// ask again.
const UpdateWait = 15 * time.Minute

// Update states.
const (
	UpdateWaiting  = "waiting"
	UpdateUpdating = "updating"
	UpdateDone     = "done"
	UpdateRefused  = "refused"
	UpdateFailed   = "failed"
)

// maxUpdateText bounds a reason or a version that came from another
// computer.
const (
	maxUpdateText    = 2000
	maxUpdateVersion = 64
)

var updateReplyStates = map[string]bool{UpdateUpdating: true, UpdateDone: true, UpdateRefused: true, UpdateFailed: true}

const netUpdatesSchema = `
CREATE TABLE IF NOT EXISTS net_updates (
 peer TEXT PRIMARY KEY,
 id TEXT NOT NULL,
 broker TEXT NOT NULL,
 state TEXT NOT NULL,
 version TEXT NOT NULL,
 text TEXT NOT NULL,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS net_update_requests (
 broker TEXT NOT NULL,
 peer TEXT NOT NULL,
 id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY (broker, peer, id)
);`

// NetUpdate is the last update this computer asked of another computer.
// Stale: no final answer came within UpdateWait; the owner may ask again.
type NetUpdate struct {
	Computer  string `json:"computer"`
	ID        string `json:"request_id"`
	State     string `json:"state"`
	Version   string `json:"version,omitempty"`
	Text      string `json:"text,omitempty"`
	Stale     bool   `json:"stale,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

// NetUpdateRequest is another computer's request to update this one.
type NetUpdateRequest struct {
	Broker, Peer, ID string
}

// RequestNetUpdate asks another computer of the network to install the
// newest signed update, and returns the request's id. A second request for
// the same computer is refused while the first one waits for its answer.
func (s *Store) RequestNetUpdate(computer string) (string, error) {
	var id string
	err := s.tx(func(tx *sql.Tx) error {
		broker, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
		}
		if broker == "" {
			return fmt.Errorf("%w: этот компьютер не в сети", ErrRejected)
		}
		p, ok, err := netPeer(tx, computer)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: такого компьютера нет в сети", ErrRejected)
		}
		if p.Age < 0 || p.Age > PeerFresh || p.Reconnecting {
			return fmt.Errorf("%w: компьютер «%s» не на связи; обновите его, когда он вернётся", ErrRejected, p.Name)
		}
		// A computer that reports no version is older than remote updates
		// and would never answer.
		if p.Version == "" {
			return fmt.Errorf("%w: на компьютере «%s» программа старше обновления по сети; обновите её там вручную", ErrRejected, p.Name)
		}
		var state, created string
		err = tx.QueryRow(`SELECT state,created_at FROM net_updates WHERE peer=? AND broker=?`, computer, broker).Scan(&state, &created)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && (state == UpdateWaiting || state == UpdateUpdating) && !updateStale(created) {
			return fmt.Errorf("%w: обновление этого компьютера уже запрошено; дождитесь ответа", ErrRejected)
		}
		r, err := randomID()
		if err != nil {
			return err
		}
		id = "upd-" + r[:16]
		at := now()
		if _, err := tx.Exec(`INSERT INTO net_updates(peer,id,broker,state,version,text,created_at,updated_at) VALUES (?,?,?,?,'','',?,?)
 ON CONFLICT(peer) DO UPDATE SET id=excluded.id,broker=excluded.broker,state=excluded.state,version='',text='',created_at=excluded.created_at,updated_at=excluded.updated_at`,
			computer, id, broker, UpdateWaiting, at, at); err != nil {
			return err
		}
		return enqueueNet(tx, broker, computer, "update."+id, id, netBody{Kind: "update_request", Task: id})
	})
	return id, err
}

func updateStale(created string) bool {
	when, err := time.Parse(time.RFC3339Nano, created)
	return err != nil || time.Since(when) > UpdateWait
}

// NetUpdates lists the last update request to each computer of the current
// network.
func (s *Store) NetUpdates() ([]NetUpdate, error) {
	out := []NetUpdate{}
	err := s.tx(func(tx *sql.Tx) error {
		broker, err := metaValue(tx, "net_broker")
		if err != nil || broker == "" {
			return err
		}
		rows, err := tx.Query(`SELECT peer,id,state,version,text,created_at,updated_at FROM net_updates WHERE broker=? ORDER BY peer`, broker)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var u NetUpdate
			var created string
			if err := rows.Scan(&u.Computer, &u.ID, &u.State, &u.Version, &u.Text, &created, &u.UpdatedAt); err != nil {
				return err
			}
			u.Stale = (u.State == UpdateWaiting || u.State == UpdateUpdating) && updateStale(created)
			out = append(out, u)
		}
		return rows.Err()
	})
	return out, err
}

// NetUpdateRequests lists the other computers' requests to update this
// computer, oldest first.
func (s *Store) NetUpdateRequests(broker string) ([]NetUpdateRequest, error) {
	rows, err := s.db.Query(`SELECT broker,peer,id FROM net_update_requests WHERE broker=? ORDER BY created_at, id LIMIT 20`, broker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NetUpdateRequest
	for rows.Next() {
		var r NetUpdateRequest
		if err := rows.Scan(&r.Broker, &r.Peer, &r.ID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReplyNetUpdate gives the first answer to a request (updating, or refused
// or failed with the reason) and forgets the request.
func (s *Store) ReplyNetUpdate(r NetUpdateRequest, state, version, text string) error {
	if !updateReplyStates[state] {
		return fmt.Errorf("unknown update state %q", state)
	}
	return s.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM net_update_requests WHERE broker=? AND peer=? AND id=?`, r.Broker, r.Peer, r.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		return enqueueNet(tx, r.Broker, r.Peer, "update-reply."+r.ID, r.ID, netBody{Kind: "update", Task: r.ID, State: state, Detail: clip(version, maxUpdateVersion), Text: clip(text, maxUpdateText)})
	})
}

// FinishNetUpdate sends the final result of an update this computer
// started for another computer: done, refused or failed. A result for
// another network than the current one is dropped: its recipient cannot be
// reached any more.
func (s *Store) FinishNetUpdate(broker, peer, id, state, version, text string) error {
	if state == UpdateUpdating || !updateReplyStates[state] {
		return fmt.Errorf("%w: unknown final update state %q", ErrRejected, state)
	}
	if !netRef.MatchString(id) || peer == "" || len(peer) > 80 {
		return fmt.Errorf("%w: invalid update request", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		current, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
		}
		if current == "" || current != broker {
			return nil
		}
		return enqueueNet(tx, broker, peer, "update-final."+id, id, netBody{Kind: "update", Task: id, State: state, Detail: clip(version, maxUpdateVersion), Text: clip(text, maxUpdateText)})
	})
}

// ActiveWork counts the tasks that are not finished, or have a run that is
// not closed, in every project. An update waits until it is zero.
// ActiveWorkTasks names up to limit of the tasks ActiveWork counts, newest
// first: their title and state, so a refused update says what holds it.
func (s *Store) ActiveWorkTasks(limit int) ([]ActiveTask, error) {
	rows, err := s.db.Query(`SELECT t.title,t.state FROM tasks t WHERE t.state IN ('running','waiting_coordinator','waiting_owner','blocked','interrupted')
 OR EXISTS (SELECT 1 FROM task_runs r WHERE r.task_id=t.id AND r.state IN ('intent','launching','running','blocked','interrupted'))
 ORDER BY t.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActiveTask
	for rows.Next() {
		var a ActiveTask
		if err := rows.Scan(&a.Title, &a.State); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ActiveTask is a task that keeps a computer from updating.
type ActiveTask struct{ Title, State string }

func (s *Store) ActiveWork() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks t WHERE t.state IN ('running','waiting_coordinator','waiting_owner','blocked','interrupted')
 OR EXISTS (SELECT 1 FROM task_runs r WHERE r.task_id=t.id AND r.state IN ('intent','launching','running','blocked','interrupted'))`).Scan(&n)
	return n, err
}

// applyUpdateRequest records another computer's request; the client
// answers it.
func applyUpdateRequest(tx *sql.Tx, from netLink) error {
	_, err := tx.Exec(`INSERT INTO net_update_requests(broker,peer,id,created_at) VALUES (?,?,?,?) ON CONFLICT DO NOTHING`,
		from.broker, from.peer, from.task, now())
	return err
}

// applyUpdateReply records an answer to this computer's request, only from
// the computer that was asked and only for its current request. A final
// answer is not replaced by a later one.
func applyUpdateReply(tx *sql.Tx, from netLink, b netBody) error {
	if !updateReplyStates[b.State] {
		return nil
	}
	cond := `state IN ('waiting','updating')`
	if b.State == UpdateUpdating {
		cond = `state='waiting'`
	}
	_, err := tx.Exec(`UPDATE net_updates SET state=?,version=?,text=?,updated_at=? WHERE id=? AND broker=? AND peer=? AND `+cond,
		b.State, clip(b.Detail, maxUpdateVersion), clip(b.Text, maxUpdateText), now(), from.task, from.broker, from.peer)
	return err
}

// updateUndeliverable handles an update message the broker refused because
// its recipient left the network: the request fails; a reply is dropped.
func updateUndeliverable(tx *sql.Tx, kind, id string) error {
	if kind != "update_request" {
		return nil
	}
	_, err := tx.Exec(`UPDATE net_updates SET state='failed',text=?,updated_at=? WHERE id=? AND state IN ('waiting','updating')`,
		"компьютер больше не в сети", now(), id)
	return err
}
