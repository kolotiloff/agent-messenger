package core

import (
	"database/sql"
	"regexp"
)

// Tasks follow a computer into a new network (T-132, D-115). A network gives
// every computer a new peer id, and «host» names whichever computer runs
// the server, so after the server moved, tasks bound to the old ids looked
// lost on one side and kept waiting on the other. Every computer has a
// lasting id of its own (meta computer_id); it reports it with its presence
// and puts it on every message. When a known computer shows up under a new
// network or peer id, the tasks shared with it are bound to the new ids, and
// messages still queued for it are sent there instead. A start is never
// re-sent: if the old network delivered it, a second one would start a
// second worker. The receiver applies a message once by its op id, so a
// re-sent answer or cancel that the old network had delivered changes
// nothing.

var computerID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

// validComputer returns id if it can be a computer id, else "".
func validComputer(id string) string {
	if computerID.MatchString(id) {
		return id
	}
	return ""
}

// rebindComputer binds the tasks shared with a computer to its current
// network and peer id. Nothing changes for tasks already bound there.
func rebindComputer(tx *sql.Tx, computer, broker, peer string) error {
	if computer == "" || broker == "" || peer == "" {
		return nil
	}
	if self, err := localComputer(tx); err != nil || self == computer {
		return err
	}
	type old struct{ id, broker, peer string }
	collect := func(query string) ([]old, error) {
		rows, err := tx.Query(query, computer, broker, peer)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []old
		for rows.Next() {
			var o old
			if err := rows.Scan(&o.id, &o.broker, &o.peer); err != nil {
				return nil, err
			}
			out = append(out, o)
		}
		return out, rows.Err()
	}
	sent, err := collect(`SELECT id,remote_broker,remote_peer FROM tasks WHERE remote_computer=? AND NOT (remote_broker=? AND remote_peer=?)`)
	if err != nil {
		return err
	}
	received, err := collect(`SELECT id,origin_broker,origin_peer FROM tasks WHERE origin_computer=? AND NOT (origin_broker=? AND origin_peer=?)`)
	if err != nil {
		return err
	}
	moved := map[[2]string]bool{}
	for _, o := range sent {
		if _, err := tx.Exec(`UPDATE tasks SET remote_broker=?,remote_peer=?,computer=? WHERE id=?`, broker, peer, peer, o.id); err != nil {
			return err
		}
		moved[[2]string{o.broker, o.peer}] = true
	}
	for _, o := range received {
		if _, err := tx.Exec(`UPDATE tasks SET origin_broker=?,origin_peer=? WHERE id=?`, broker, peer, o.id); err != nil {
			return err
		}
		moved[[2]string{o.broker, o.peer}] = true
	}
	for pair := range moved {
		if _, err := tx.Exec(`UPDATE net_outbox SET broker=?,recipient=? WHERE broker=? AND recipient=? AND kind<>'start'`, broker, peer, pair[0], pair[1]); err != nil {
			return err
		}
	}
	return nil
}

// fillComputers records the lasting ids of computers for tasks bound to
// them in the current network, so tasks from before 0.61 can follow them
// into the next network too.
func fillComputers(tx *sql.Tx, broker string, peers []NetPeer) error {
	for _, p := range peers {
		c := validComputer(p.Computer)
		if c == "" || p.Reconnecting {
			continue
		}
		if _, err := tx.Exec(`UPDATE tasks SET remote_computer=? WHERE remote_broker=? AND remote_peer=? AND remote_computer=''`, c, broker, p.ID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE tasks SET origin_computer=? WHERE origin_broker=? AND origin_peer=? AND origin_computer=''`, c, broker, p.ID); err != nil {
			return err
		}
	}
	return nil
}
