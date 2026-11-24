package broker

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// A window watches just the update controls. Task and mailbox bodies stay private.
type UpdateMark struct {
	ID      string `json:"id"`
	State   string `json:"state"`
	Version string `json:"version,omitempty"`
	At      int64  `json:"at"`
}
type UpdateSnapshot struct {
	Broker string                `json:"broker"`
	Self   string                `json:"self"`
	Cursor string                `json:"cursor"`
	Marks  map[string]UpdateMark `json:"marks"`
}
type updateSignal struct {
	mu sync.Mutex
	ch chan struct{}
}

func (s *updateSignal) listen() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch == nil {
		s.ch = make(chan struct{})
	}
	return s.ch
}
func (s *updateSignal) changed() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ch != nil {
		close(s.ch)
		s.ch = nil
	}
}

// Only routing metadata is used. A reply must come from the target of the
// same request. A duplicate offer never displaces an accepted installation.
func recordUpdateOp(tx *sql.Tx, from, to, op string, at int64) (bool, error) {
	parts := strings.SplitN(op, ".", 4)
	if len(parts) < 2 || !strings.HasPrefix(parts[1], "upd-") || len(parts[1]) > 80 {
		return false, nil
	}
	id := parts[1]
	var res sql.Result
	var err error
	switch parts[0] {
	case "update":
		if len(parts) != 2 {
			return false, nil
		}
		res, err = tx.Exec(`INSERT INTO update_marks(target,requester,id,state,version,at) VALUES(?,?,?,'waiting','',?)
		ON CONFLICT(target) DO UPDATE SET requester=excluded.requester,id=excluded.id,state='waiting',version='',at=excluded.at
		WHERE update_marks.id<>excluded.id AND (state NOT IN ('waiting','queued','updating') OR (state='waiting' AND at<=?-900))`, to, from, id, at, at)
	case "update-reply", "update-progress", "update-final":
		state, version := "legacy", ""
		if parts[0] == "update-final" {
			state = "finished"
		}
		if len(parts) >= 3 {
			state = parts[2]
		}
		switch state {
		case "legacy", "finished", "queued", "updating", "done", "refused", "cancelled", "failed":
		default:
			return false, nil
		}
		if len(parts) == 4 && state == "done" {
			version = parts[3]
		}
		// The receiver may accept a second offer after declining the first.
		// Only that receiver can confirm which request owns its queue.
		if state == "queued" || state == "updating" {
			res, err = tx.Exec(`INSERT INTO update_marks(target,requester,id,state,version,at) VALUES(?,?,?,?,?,?)
			ON CONFLICT(target) DO UPDATE SET requester=excluded.requester,id=excluded.id,state=excluded.state,version='',at=excluded.at
			WHERE id<>excluded.id OR (state IN ('waiting','legacy','queued') AND state<>excluded.state)`, from, to, id, state, "", at)
			break
		}
		res, err = tx.Exec(`UPDATE update_marks SET state=?,version=?,at=? WHERE target=? AND requester=? AND id=?
		 AND state IN ('waiting','legacy','queued','updating') AND (state<>'updating' OR ?<>'queued')`, state, version, at, from, to, id, state)
	default:
		return false, nil
	}
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

// Include updates queued locally (including automatic updates). Only the
// target can publish its own presence, and a delayed report for a completed
// request cannot reopen that request.
func recordUpdatePresence(tx *sql.Tx, peer string, p Presence, previousRequest string, now int64) (bool, error) {
	var res sql.Result
	var err error
	if p.UpdatePending && p.UpdateRequestID != "" {
		state := "queued"
		if p.UpdateInstalling {
			state = "updating"
		}
		res, err = tx.Exec(`INSERT INTO update_marks(target,requester,id,state,version,at) VALUES(?,'',?,?,'',?)
		ON CONFLICT(target) DO UPDATE SET id=excluded.id,state=excluded.state,requester=CASE WHEN id=excluded.id THEN requester ELSE '' END,version='',at=excluded.at
		WHERE (id<>excluded.id AND state NOT IN ('waiting','queued','updating')) OR
		(id=excluded.id AND (state IN ('legacy','waiting') OR (state='queued' AND excluded.state='updating')))`, peer, p.UpdateRequestID, state, now)
	} else if !p.UpdatePending {
		// An empty queue proves that a request is gone only once this
		// executor previously reported that exact request as pending.
		res, err = tx.Exec(`UPDATE update_marks SET state='finished',version=?,at=?
		 WHERE target=? AND (requester='' OR id=?) AND state IN ('queued','updating')`,
			p.Version, now, peer, previousRequest)
	}
	if err != nil || res == nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}

func (b *Broker) updateSnapshot(self string) (UpdateSnapshot, error) {
	s := UpdateSnapshot{Broker: b.id, Self: self, Marks: map[string]UpdateMark{}}
	rows, err := b.db.Query(`SELECT target,id,state,version,at FROM update_marks WHERE target='host' OR target IN (SELECT id FROM devices WHERE revoked_at IS NULL) ORDER BY target`)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var target string
		var m UpdateMark
		if err := rows.Scan(&target, &m.ID, &m.State, &m.Version, &m.At); err != nil {
			return s, err
		}
		s.Marks[target] = m
	}
	if err := rows.Err(); err != nil {
		return s, err
	}
	raw, _ := json.Marshal(s)
	sum := sha256.Sum256(raw)
	s.Cursor = hex.EncodeToString(sum[:])
	return s, nil
}
func (b *Broker) watchUpdates(ctx context.Context, self, after string) (UpdateSnapshot, error) {
	change := b.updates.listen()
	s, err := b.updateSnapshot(self)
	if err != nil || s.Cursor != after {
		return s, err
	}
	timer := time.NewTimer(6 * time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return s, ctx.Err()
	case <-timer.C:
		return s, nil
	case <-change:
		return b.updateSnapshot(self)
	}
}
func (b *Broker) handleUpdateWatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", 405)
		return
	}
	d, ok := b.authorized(w, r)
	if !ok {
		return
	}
	after := r.URL.Query().Get("after")
	if len(after) > 64 {
		http.Error(w, "cursor", 400)
		return
	}
	s, err := b.watchUpdates(r.Context(), d.ID, after)
	if err != nil {
		http.Error(w, "unavailable", 503)
		return
	}
	// Revocation while a request was waiting also closes its view.
	if member, err := b.member(d.ID); err != nil || !member {
		http.Error(w, "device", 401)
		return
	}
	writeJSON(w, 200, s)
}
func (s *Service) WatchUpdates(ctx context.Context, after string) (UpdateSnapshot, error) {
	s.mu.Lock()
	n, ok := s.networkLocked()
	b := s.broker
	var c Connection
	if s.member != nil {
		c = *s.member
	}
	s.mu.Unlock()
	if !ok {
		return UpdateSnapshot{}, ErrNetworkChanged
	}
	if b != nil {
		return b.watchUpdates(ctx, n.Self, after)
	}
	var snap UpdateSnapshot
	code, err := call(ctx, pinnedClient(c.Fingerprint), http.MethodGet, c.Address, "/v1/update-watch?after="+url.QueryEscape(after), c.DeviceKey, nil, &snap)
	if err != nil {
		return snap, err
	}
	if code != 200 || snap.Broker != n.BrokerID || snap.Self != n.Self || len(snap.Cursor) != 64 {
		return UpdateSnapshot{}, errors.New("сервер не передаёт изменения очереди обновлений")
	}
	if current, ok := s.Network(); !ok || current != n {
		return UpdateSnapshot{}, ErrNetworkChanged
	}
	return snap, nil
}
