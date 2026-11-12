package core

import (
	"database/sql"
	"fmt"
	"time"
)

// A task sent to a computer that never accepts it used to wait silently
// (coordinator list row 30): the whole night's work on one computer never
// started and nothing said so. After unacceptedAfter the coordinator gets
// one event with the likely reason, and the task's detail says it too.
// Nothing is retried or cancelled on its own: the coordinator decides.

var unacceptedAfter = 5 * time.Minute

// NoteUnaccepted tells the coordinator once about each task this computer
// sent that the other computer has not accepted within unacceptedAfter.
func (s *Store) NoteUnaccepted(at time.Time) (int, error) {
	noted := 0
	err := s.tx(func(tx *sql.Tx) error {
		type waiting struct{ id, project, created, broker, peer, provider string }
		rows, err := tx.Query(`SELECT id,project,created_at,remote_broker,remote_peer,provider FROM tasks
 WHERE state='running' AND COALESCE(remote_broker,'')<>'' AND COALESCE(remote_task,'')=''
 AND NOT EXISTS (SELECT 1 FROM task_events e WHERE e.task_id=tasks.id AND e.kind IN ('not_accepted','accepted'))`)
		if err != nil {
			return err
		}
		var list []waiting
		for rows.Next() {
			var w waiting
			if err := rows.Scan(&w.id, &w.project, &w.created, &w.broker, &w.peer, &w.provider); err != nil {
				rows.Close()
				return err
			}
			list = append(list, w)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, w := range list {
			created, err := time.Parse(time.RFC3339Nano, w.created)
			if err != nil || at.Sub(created) < unacceptedAfter {
				continue
			}
			why, err := unacceptedReason(tx, w.id, w.broker, w.peer, w.provider)
			if err != nil {
				return err
			}
			text := "not accepted in " + unacceptedAfter.String() + ": " + why + ". Nothing is retried. Cancel it (it closes here at once) and start it again only when you know it did not run there, or keep waiting."
			if _, err := tx.Exec(`UPDATE tasks SET detail=?,updated_at=?,version=version+1 WHERE id=?`, "not accepted: "+why, now(), w.id); err != nil {
				return err
			}
			if err := insertTaskEvent(tx, w.project, w.id, AudienceCoordinator, "not_accepted", text, ""); err != nil {
				return err
			}
			noted++
		}
		return nil
	})
	return noted, err
}

// unacceptedReason names the likely reason a task was not accepted.
func unacceptedReason(tx *sql.Tx, task, broker, peerID, provider string) (string, error) {
	var queued int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM net_outbox WHERE op_id=?`, safeNetOp("start."+task)).Scan(&queued); err != nil {
		return "", err
	}
	current, err := metaValue(tx, "net_broker")
	if err != nil {
		return "", err
	}
	name := peerName(tx, peerID)
	peer, found, err := netPeer(tx, peerID)
	if err != nil {
		return "", err
	}
	switch {
	case current == "":
		return "this computer is in no network now", nil
	case current != broker:
		return "the task was sent in an earlier network (the server moved?) and " + name + " has not shown up in this one", nil
	case !found:
		return name + " is not in the network now (an old computer id?)", nil
	case peer.Reconnecting || peer.Age < 0 || peer.Age > PeerFresh:
		return name + " is offline", nil
	case queued > 0:
		return "the start has not left this computer: no link to the server", nil
	case peer.Ready != nil && !peer.Ready.Accept:
		return name + " does not take tasks from the network («Принимать задачи других компьютеров» is off there)", nil
	}
	// Coordinator feedback 2026-11-12: the computer ran the task while its
	// replies never arrived. When it reports this agent busy, say so: the
	// task may be running, so starting it again could do the work twice.
	for _, a := range peer.Agents {
		if a.Agent == provider && a.Running > 0 {
			return fmt.Sprintf("%s is online and reports %d running %s task(s), but has not answered: this task may be running there with its replies not reaching this computer; ask the owner to look there before starting it again", name, a.Running, provider), nil
		}
	}
	return name + " is online but has not answered", nil
}
