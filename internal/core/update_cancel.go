package core

import (
	"database/sql"
	"errors"
	"fmt"
)

const updateCancelMinBuild = 100

// Install on both the legacy queue and the machine database of an already
// activated generation. The table is created last in the same transaction.
func ensureUpdateCancellation(db *sql.DB) error {
	const query = `SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE name='net_update_cancels' AND type='table')`
	var installed bool
	if err := db.QueryRow(query).Scan(&installed); err != nil || installed {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := tx.QueryRow(query).Scan(&installed); err != nil || installed {
		return err
	}
	if _, err := tx.Exec(`
ALTER TABLE pending_updates ADD COLUMN target_version TEXT NOT NULL DEFAULT '';
ALTER TABLE net_peers ADD COLUMN update_request_id TEXT NOT NULL DEFAULT '';
CREATE TABLE net_update_cancels (
 incoming INTEGER NOT NULL,
 broker TEXT NOT NULL,
 peer TEXT NOT NULL,
 id TEXT NOT NULL,
 target TEXT NOT NULL,
 project TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY(incoming,broker,peer,id)
);`); err != nil {
		return err
	}
	return tx.Commit()
}

type UpdateCancellation struct {
	Broker, Peer, ID, Target, Project string
}

// RequestUpdateCancellation binds the action to the reported request ID.
// A delayed cancellation can never cancel a later update.
func (s *Store) RequestUpdateCancellation(computer, project string) (string, error) {
	if !ValidProjectName(project) {
		return "", fmt.Errorf("%w: unknown project", ErrRejected)
	}
	return s.requestUpdateCancellation(computer, project, "", "")
}

// CancelObservedUpdate is the owner's action on a specific card. A reconnect
// or a delayed click must never redirect it to another network or request.
func (s *Store) CancelObservedUpdate(computer, broker, target string) (string, error) {
	if !netRef.MatchString(target) || (computer != "" && broker == "") {
		return "", fmt.Errorf("%w: неизвестный запрос обновления", ErrRejected)
	}
	return s.requestUpdateCancellation(computer, "", broker, target)
}

func (s *Store) requestUpdateCancellation(computer, project, expectedBroker, expectedID string) (string, error) {
	var id string
	err := s.tx(func(tx *sql.Tx) error {
		self, err := localComputer(tx)
		if err != nil {
			return err
		}
		r := UpdateCancellation{Project: project}
		local := computer == "" || computer == self
		if local {
			if expectedID != "" {
				if expectedBroker != "" {
					current, err := metaValue(tx, "net_broker")
					if err != nil {
						return err
					}
					if current != expectedBroker {
						return fmt.Errorf("%w: сеть изменилась", ErrRejected)
					}
				}
				r.Target, r.Broker = expectedID, expectedBroker
			} else {
				err = tx.QueryRow(`SELECT id FROM pending_updates ORDER BY created_at,id LIMIT 1`).Scan(&r.Target)
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("%w: этот компьютер не ждёт обновления", ErrRejected)
				}
				if err != nil {
					return err
				}
			}
		} else {
			r.Broker, err = metaValue(tx, "net_broker")
			if err != nil {
				return err
			}
			p, ok, err := netPeer(tx, computer)
			if err != nil {
				return err
			}
			// An exact cancellation can wait in the mailbox while the roster
			// is late. The receiver still checks that it owns the request and
			// has not started its installer. An implicit selection needs a
			// fresh report to establish which request is pending.
			if r.Broker == "" || !ok || p.Reconnecting || (expectedID == "" && (p.Age < 0 || p.Age > PeerFresh)) {
				return fmt.Errorf("%w: компьютер сейчас не на связи", ErrRejected)
			}
			if updateBuild(p.Version) < updateCancelMinBuild {
				return fmt.Errorf("%w: компьютер «%s» пока не поддерживает отмену обновления. Установите Messenger 0.100 или новее на том компьютере", ErrRejected, p.Name)
			}
			if expectedBroker != "" && expectedBroker != r.Broker {
				return fmt.Errorf("%w: сеть изменилась", ErrRejected)
			}
			r.Peer, r.Target = computer, p.UpdateRequestID
			if expectedID != "" {
				r.Target = expectedID
			}
			if p.UpdatePending && r.Target == "" {
				// An older broker can relay the command but omits the ID.
				// The original requester still knows its own queued ID.
				err = tx.QueryRow(`SELECT id FROM net_updates WHERE broker=? AND peer=? AND state IN ('queued','updating')`,
					r.Broker, computer).Scan(&r.Target)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
			}
			if (expectedID == "" && !p.UpdatePending) || r.Target == "" {
				return fmt.Errorf("%w: компьютер «%s» не сообщил идентификатор ожидающего обновления", ErrRejected, p.Name)
			}
		}
		if expectedID != "" {
			// A window may lose the reply after the transaction commits.
			// Reuse the cancellation already queued for this exact update.
			err := tx.QueryRow(`SELECT id FROM net_update_cancels
 WHERE incoming=? AND broker=? AND peer=? AND target=? AND project=''`, local, r.Broker, r.Peer, r.Target).Scan(&id)
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		token, err := randomID()
		if err != nil {
			return err
		}
		r.ID = "ucan-" + token[:16]
		id = r.ID
		if err := writeUpdateCancellation(tx, local, r); err != nil || local {
			return err
		}
		return enqueueNet(tx, r.Broker, r.Peer, "update-cancel."+id, id, netBody{Kind: "update_cancel_request", Task: id, Question: r.Target})
	})
	return id, err
}

type PendingUpdateCancellation struct {
	Computer  string `json:"device"`
	RequestID string `json:"request_id"`
}

// PendingUpdateCancellations shows the owner's queued actions in every local
// window, even if it was reopened or the submission response was lost.
func (s *Store) PendingUpdateCancellations() ([]PendingUpdateCancellation, error) {
	rows, err := s.db.Query(`SELECT DISTINCT peer,target FROM net_update_cancels
 WHERE (incoming=0 OR peer='') AND (broker='' OR broker=(SELECT value FROM meta WHERE key='net_broker'))`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PendingUpdateCancellation{}
	for rows.Next() {
		var r PendingUpdateCancellation
		if err := rows.Scan(&r.Computer, &r.RequestID); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func writeUpdateCancellation(tx *sql.Tx, incoming bool, r UpdateCancellation) error {
	_, err := tx.Exec(`INSERT INTO net_update_cancels(incoming,broker,peer,id,target,project,created_at)
 VALUES (?,?,?,?,?,?,?) ON CONFLICT DO NOTHING`, incoming, r.Broker, r.Peer, r.ID, r.Target, r.Project, now())
	return err
}

func (s *Store) UpdateCancellations(broker string) ([]UpdateCancellation, error) {
	rows, err := s.db.Query(`SELECT broker,peer,id,target,project FROM net_update_cancels
 WHERE incoming=1 AND (broker='' OR broker=?) ORDER BY created_at,id LIMIT 20`, broker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []UpdateCancellation
	for rows.Next() {
		var r UpdateCancellation
		if err := rows.Scan(&r.Broker, &r.Peer, &r.ID, &r.Target, &r.Project); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// FinishUpdateCancellation runs only in the supervisor's network loop, which
// also owns helper launch. Installation or an unconsumed result stays fenced.
func (s *Store) FinishUpdateCancellation(r UpdateCancellation, installing bool) error {
	return s.tx(func(tx *sql.Tx) error {
		// The target window can see the broker's event before it has read
		// the offered request. Keep its exact cancellation until that read.
		if r.Peer == "" && r.Broker != "" {
			var awaiting bool
			err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM net_update_cancels WHERE incoming=1 AND broker=? AND peer='' AND id=?
				AND unixepoch(created_at)>unixepoch('now')-300)
				AND NOT EXISTS(SELECT 1 FROM pending_updates WHERE id=?)
				AND NOT EXISTS(SELECT 1 FROM net_update_requests WHERE broker=? AND id=?)`, r.Broker, r.ID, r.Target, r.Broker, r.Target).Scan(&awaiting)
			if err != nil || awaiting {
				return err
			}
		}
		res, err := tx.Exec(`DELETE FROM net_update_cancels WHERE incoming=1 AND broker=? AND peer=? AND id=? AND target=?`,
			r.Broker, r.Peer, r.ID, r.Target)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil || n == 0 {
			return err
		}
		var pending bool
		if err := tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM pending_updates WHERE id=?)`, r.Target).Scan(&pending); err != nil {
			return err
		}
		state, reason := UpdateRefused, "that update is no longer pending"
		if !pending && r.Target != "" {
			// The request and its cancellation can arrive in the same inbox batch.
			var requester string
			err := tx.QueryRow(`DELETE FROM net_update_requests WHERE broker=? AND id=? RETURNING peer`, r.Broker, r.Target).Scan(&requester)
			if err == nil {
				state, reason = UpdateCancelled, ""
				if err := enqueueNet(tx, r.Broker, requester, updateReplyOp("update-final", r.Target, state, ""), r.Target, netBody{Kind: "update", Task: r.Target, State: state}); err != nil {
					return err
				}
			} else if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if pending && installing {
			reason = "installation has already started"
		}
		if pending && !installing {
			if err := finishPendingUpdate(tx, r.Target, UpdateCancelled, "", ""); err != nil {
				return err
			}
			state, reason = UpdateCancelled, ""
		}
		if r.Peer != "" {
			return enqueueNet(tx, r.Broker, r.Peer, "update-cancel-result."+r.ID, r.ID,
				netBody{Kind: "update_cancel_result", Task: r.ID, Question: r.Target, State: state, Text: reason})
		}
		name, err := computerName(tx)
		if err != nil {
			return err
		}
		return insertCoordinatorNotice(tx, r.Project, MessengerUpdateEvent, cancellationNotice(name, state, reason))
	})
}

func applyUpdateCancellation(tx *sql.Tx, from netLink, body netBody) error {
	if !netRef.MatchString(body.Question) {
		return nil
	}
	return writeUpdateCancellation(tx, true, UpdateCancellation{Broker: from.broker, Peer: from.peer, ID: body.Task, Target: body.Question})
}

func applyUpdateCancellationResult(tx *sql.Tx, from netLink, body netBody) error {
	if body.State != UpdateCancelled && body.State != UpdateRefused && body.State != UpdateFailed {
		return nil
	}
	var project, target string
	err := tx.QueryRow(`DELETE FROM net_update_cancels WHERE incoming=0 AND broker=? AND peer=? AND id=? AND (?='' OR target=?) RETURNING project,target`,
		from.broker, from.peer, from.task, body.Question, body.Question).Scan(&project, &target)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if body.State == UpdateCancelled {
		if _, err := tx.Exec(`UPDATE net_peers SET update_pending=0 WHERE peer=? AND update_request_id=?`,
			from.peer, target); err != nil {
			return err
		}
	}
	return insertCoordinatorNotice(tx, project, MessengerUpdateEvent, cancellationNotice(peerName(tx, from.peer), body.State, clip(body.Text, maxUpdateText)))
}

func cancellationNotice(name, state, reason string) string {
	if state == UpdateCancelled {
		return messengerUpdateEvent(name, state, "", "")
	}
	return "Cancellation of the Messenger update of computer «" + name + "» was not confirmed: " + reason + "."
}
