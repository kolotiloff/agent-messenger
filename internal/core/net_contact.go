package core

import (
	"database/sql"
	"fmt"
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
	NetworkLostEvent = "network_lost"
	NetworkBackEvent = "network_back"
)

// NoteNetworkContact compares the network's state with the last one noted
// and tells every project's coordinator once when contact with all other
// computers was lost, or came back.
func (s *Store) NoteNetworkContact(at time.Time) (string, error) {
	noted := ""
	err := s.tx(func(tx *sql.Tx) error {
		broker, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
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
		if len(peers) == 0 {
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
			if prevState != "lost" {
				return nil
			}
			noted = NetworkBackEvent
			return noticeAllProjects(tx, NetworkBackEvent, "contact with the network is back: other computers report again. Check tasks before you start or cancel anything: the states of their tasks catch up within a minute, and reports sent meanwhile arrive now.")
		case !fresh && prevState == "ok":
			if err := setMeta(tx, "net_contact", broker+"|lost"); err != nil {
				return err
			}
			noted = NetworkLostEvent
			text, err := networkLostText(tx, peers)
			if err != nil {
				return err
			}
			return noticeAllProjects(tx, NetworkLostEvent, text)
		}
		return nil
	})
	return noted, err
}

// networkLostText names what was lost: the server's computer, the others,
// and the open tasks on them.
func networkLostText(tx *sql.Tx, peers []NetPeer) (string, error) {
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
	var open int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM tasks WHERE COALESCE(remote_broker,'')<>'' AND state IN ('running','waiting_coordinator','waiting_owner')`).Scan(&open); err != nil {
		return "", err
	}
	text := "contact with every other computer of the network is lost (" + strings.Join(names, ", ") + ")"
	if server != "" {
		text += "; the server runs on «" + server + "», so it has probably stopped or left the network"
	} else {
		text += "; this computer's own link to the server may be down"
	}
	text += "."
	if open > 0 {
		text += fmt.Sprintf(" %d open task(s) on other computers: their outcome is unknown; they may still run there and report when contact is back.", open)
	}
	text += " Nothing is retried, cancelled or moved. Do not start the same work again; tell the owner, who can start Messenger on the server computer again. A network_back event follows when contact returns."
	return text, nil
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
