package core

import (
	"database/sql"
	"strings"
	"time"
)

// T-177: when the computer that runs the server stops, every other computer
// drops out at once. The coordinator used to learn it only task by task,
// if at all. Now it gets one event when this computer loses contact with
// every other computer of the network, and one when contact is back. The
// events only inform: no task is closed, retried or moved, and the server
// is not moved to another computer (the owner said not now, 2026-11-13).

const (
	// NetworkLostEvent and NetworkBackEvent are the coordinator events.
	NetworkLostEvent      = "network_lost"
	NetworkBackEvent      = "network_back"
	MacUpdateRestartEvent = "messenger_updating"
	macUpdateRestartGrace = 2 * time.Minute
)

// NoteMacUpdateRestart is called by the Mac window just before it stops
// the old client. Store the notice and the grace period together so they
// survive the client restart and are visible to a waiting coordinator.
func (s *Store) NoteMacUpdateRestart(at time.Time) error {
	return s.tx(func(tx *sql.Tx) error {
		broker, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
		}
		planned, err := metaValue(tx, "mac_update_restart")
		if err != nil {
			return err
		}
		if b, until := macUpdateRestart(planned); b == broker && at.Before(until) {
			return nil
		}
		if err := setMeta(tx, "mac_update_restart", broker+"|"+at.Add(macUpdateRestartGrace).UTC().Format(time.RFC3339Nano)); err != nil {
			return err
		}
		return noticeAllProjects(tx, MacUpdateRestartEvent, "Messenger on this Mac is updating. Its background client will restart and network contact may briefly go away. Keep waiting for task updates. Tell the owner if network_lost follows; no warning is needed for the planned restart itself.")
	})
}

func macUpdateRestart(value string) (string, time.Time) {
	broker, stamp, _ := strings.Cut(value, "|")
	until, _ := time.Parse(time.RFC3339Nano, stamp)
	return broker, until
}

// NoteNetworkContact compares the network's state with the last one noted
// and tells every project's coordinator once when contact with all other
// computers was lost, or came back.
func (s *Store) NoteNetworkContact(at time.Time) (string, error) {
	return s.noteNetworkContact(at, false)
}

// CheckExpiredMacUpdateRestart lets a waiting coordinator report a
// prolonged outage even if the old client stopped and the new one has yet
// to start. The supervisor still checks ordinary outages.
func (s *Store) CheckExpiredMacUpdateRestart(at time.Time) error {
	// Wait polls often. Reserve a write transaction only when a deadline
	// is actually due, then check again in the transaction.
	var planned string
	if err := s.db.QueryRow(`SELECT COALESCE((SELECT value FROM meta WHERE key='mac_update_restart'),'')`).Scan(&planned); err != nil {
		return err
	}
	_, until := macUpdateRestart(planned)
	if planned == "" || at.Before(until) {
		return nil
	}
	_, err := s.noteNetworkContact(at, true)
	return err
}

func (s *Store) noteNetworkContact(at time.Time, expiredUpdateOnly bool) (string, error) {
	noted := ""
	err := s.tx(func(tx *sql.Tx) error {
		planned, err := metaValue(tx, "mac_update_restart")
		if err != nil {
			return err
		}
		plannedBroker, until := macUpdateRestart(planned)
		inGrace := at.Before(until)
		if expiredUpdateOnly && (planned == "" || inGrace) {
			return nil
		}
		broker, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
		}
		if planned != "" && (!inGrace || (broker != "" && plannedBroker != broker)) {
			if err := setMeta(tx, "mac_update_restart", ""); err != nil {
				return err
			}
		}
		prev, err := metaValue(tx, "net_contact")
		if err != nil {
			return err
		}
		if broker == "" {
			if prev != "" {
				return setMeta(tx, "net_contact", "")
			}
			return nil
		}
		peers, err := netPeers(tx)
		if err != nil {
			return err
		}
		if len(peers) == 0 && planned == "" {
			return nil
		}
		fresh := false
		for _, p := range peers {
			fresh = fresh || (p.Age >= 0 && p.Age <= PeerFresh)
		}
		// The state belongs to one network: joining another starts afresh.
		prevState := ""
		if b, state, ok := strings.Cut(prev, "|"); ok && b == broker {
			prevState = state
		}
		switch {
		case fresh && prevState != "ok":
			if err := setMeta(tx, "net_contact", broker+"|ok"); err != nil {
				return err
			}
			if prevState == "updating" && plannedBroker == broker {
				return setMeta(tx, "mac_update_restart", "")
			}
			if prevState != "lost" {
				return nil
			}
			noted = NetworkBackEvent
			return noticeAllProjects(tx, NetworkBackEvent, "contact with the network is back: other computers report again. Check tasks before you start or cancel anything: the states of their tasks catch up within a minute, and reports sent meanwhile arrive now.")
		case !fresh && (prevState == "ok" || prevState == "updating"):
			if inGrace && plannedBroker == broker {
				if prevState == "ok" {
					return setMeta(tx, "net_contact", broker+"|updating")
				}
				return nil
			}
			if err := setMeta(tx, "net_contact", broker+"|lost"); err != nil {
				return err
			}
			noted = NetworkLostEvent
			return noticeAllProjects(tx, NetworkLostEvent, networkLostText(peers))
		}
		return nil
	})
	return noted, err
}

// networkLostText describes the disconnected computers.
func networkLostText(peers []NetPeer) string {
	var names []string
	server := ""
	for _, p := range peers {
		name := p.Name
		if name == "" {
			name = p.ID
		}
		if p.Host {
			server = name
		}
		if len(names) < 12 {
			names = append(names, "«"+name+"»")
		}
	}
	text := "contact with every other computer of the network is lost"
	if len(names) > 0 {
		text += " (" + strings.Join(names, ", ") + ")"
	}
	if server != "" {
		text += "; the server runs on «" + server + "», so it has probably stopped or left the network"
	} else {
		text += "; this computer's own link to the server may be down"
	}
	text += "."
	text += " Open tasks of this project may still run and report when contact is back."
	text += " Nothing is retried, cancelled or moved. Do not start the same work again; tell the owner, who can start Messenger on the server computer again. A network_back event follows when contact returns."
	return text
}

// noticeAllProjects gives every project's coordinator the same event.
func noticeAllProjects(tx *sql.Tx, kind, text string) error {
	rows, err := tx.Query(`SELECT name FROM projects WHERE removed_at IS NULL`)
	if err != nil {
		return err
	}
	var projects []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		projects = append(projects, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, p := range projects {
		if err := insertCoordinatorNotice(tx, p, kind, text); err != nil {
			return err
		}
	}
	return nil
}
