package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Shared task lists (T-047, D-080). Every computer of a network sends the
// tasks it started in the last days and keeps the latest lists of the other
// computers; the window shows them read-only. A task that another computer
// started and that runs here is not shared by this computer (its origin
// shares it), and the window hides another computer's task that already has
// a mirror here, so no task shows twice.

// Limits of what one computer shares; the broker refuses a list over them.
const (
	MaxSharedTasks  = 50
	MaxSharedTitle  = 200
	MaxSharedReport = 800
)

const netTasksSchema = `
CREATE TABLE IF NOT EXISTS net_tasks (
 peer TEXT PRIMARY KEY,
 name TEXT NOT NULL,
 age INTEGER NOT NULL,
 tasks TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS net_tasks_hidden (
 peer TEXT NOT NULL,
 task TEXT NOT NULL,
 at TEXT NOT NULL,
 PRIMARY KEY(peer, task)
);`

// NetTask is one task of a computer's shared list.
type NetTask struct {
	ID         string `json:"id"`
	Project    string `json:"project"`
	Title      string `json:"title"`
	Agent      string `json:"agent"`
	State      string `json:"state"`
	CreatedAt  string `json:"created_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	Report     string `json:"report,omitempty"`
	Tokens     int64  `json:"tokens,omitempty"`
}

// NetPeerTasks is another computer's shared list. Age is in seconds.
type NetPeerTasks struct {
	Peer  string    `json:"peer"`
	Name  string    `json:"name"`
	Age   int       `json:"age"`
	Tasks []NetTask `json:"tasks"`
}

// SharedTasks lists the tasks this computer started since the given time,
// newest first, at most MaxSharedTasks, with the start of the report.
func (s *Store) SharedTasks(since time.Time) ([]NetTask, error) {
	rows, err := s.db.Query(`SELECT t.id,t.project,t.title,t.provider,t.state,t.created_at,COALESCE(t.finished_at,''),t.usage_input+t.usage_output,
 COALESCE((SELECT r.disposition_text FROM task_runs r WHERE r.task_id=t.id AND r.disposition='report' ORDER BY r.ordinal DESC LIMIT 1),
  (SELECT i.text FROM remote_task_items i WHERE i.task_id=t.id AND i.kind='report' ORDER BY i.seq DESC LIMIT 1),'')
 FROM tasks t WHERE t.origin_peer IS NULL AND t.hidden_at IS NULL AND t.created_at>=? ORDER BY t.created_at DESC, t.rowid DESC LIMIT ?`,
		since.UTC().Format(time.RFC3339Nano), MaxSharedTasks)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NetTask{}
	for rows.Next() {
		var t NetTask
		if err := rows.Scan(&t.ID, &t.Project, &t.Title, &t.Agent, &t.State, &t.CreatedAt, &t.FinishedAt, &t.Tokens, &t.Report); err != nil {
			return nil, err
		}
		t.Title, t.Report = clip(t.Title, MaxSharedTitle), clip(t.Report, MaxSharedReport)
		out = append(out, t)
	}
	return out, rows.Err()
}

// SetNetTasks replaces the other computers' lists.
func (s *Store) SetNetTasks(peers []NetPeerTasks) error {
	return s.tx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`DELETE FROM net_tasks`); err != nil {
			return err
		}
		at := now()
		for _, p := range peers {
			if p.Tasks == nil {
				p.Tasks = []NetTask{}
			}
			raw, _ := json.Marshal(p.Tasks)
			if _, err := tx.Exec(`INSERT INTO net_tasks(peer,name,age,tasks,updated_at) VALUES (?,?,?,?,?) ON CONFLICT(peer) DO UPDATE SET name=excluded.name,age=excluded.age,tasks=excluded.tasks,updated_at=excluded.updated_at`, p.Peer, p.Name, p.Age, string(raw), at); err != nil {
				return err
			}
		}
		return nil
	})
}

// HideTask removes a finished task of this computer from the list (T-131).
// The task stays in the database; the window and the shared list leave it
// out, so the other computers drop it with their next list. A task that
// has not finished is refused: the owner stops it first.
func (s *Store) HideTask(id string) error {
	return s.tx(func(tx *sql.Tx) error {
		var state string
		err := tx.QueryRow(`SELECT state FROM tasks WHERE id=?`, id).Scan(&state)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown task", ErrRejected)
		}
		if err != nil {
			return err
		}
		if state != "done" && state != "failed" && state != "cancelled" {
			return fmt.Errorf("%w: stop the task before removing it", ErrRejected)
		}
		_, err = tx.Exec(`UPDATE tasks SET hidden_at=COALESCE(hidden_at,?) WHERE id=?`, now(), id)
		return err
	})
}

// HideNetTask removes another computer's task from this window's list
// (T-131). It only changes what this computer shows: the task and its
// computer are left as they are, so the owner can drop a task that looks
// stuck without touching anything that may still run there.
func (s *Store) HideNetTask(peer, task string) error {
	if peer == "" || task == "" {
		return errors.New("task not specified")
	}
	_, err := s.db.Exec(`INSERT INTO net_tasks_hidden(peer,task,at) VALUES (?,?,?) ON CONFLICT(peer,task) DO NOTHING`, peer, task, now())
	return err
}

// projectNetTasks returns the other computers' tasks of a project, leaving
// out the computers with none, the tasks that run here and the tasks the
// owner removed from the list.
func projectNetTasks(tx *sql.Tx, project string) ([]NetPeerTasks, error) {
	rows, err := tx.Query(`SELECT peer,name,age,tasks,updated_at FROM net_tasks ORDER BY name, peer`)
	if err != nil {
		return nil, err
	}
	var all []NetPeerTasks
	for rows.Next() {
		var p NetPeerTasks
		var list, at string
		if err := rows.Scan(&p.Peer, &p.Name, &p.Age, &list, &at); err != nil {
			rows.Close()
			return nil, err
		}
		if json.Unmarshal([]byte(list), &p.Tasks) != nil {
			continue
		}
		if when, err := time.Parse(time.RFC3339Nano, at); err == nil {
			p.Age += int(time.Since(when) / time.Second)
		}
		all = append(all, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := []NetPeerTasks{}
	for _, p := range all {
		var keep []NetTask
		for _, t := range p.Tasks {
			if t.Project != project {
				continue
			}
			var here int
			if err := tx.QueryRow(`SELECT (SELECT COUNT(*) FROM tasks WHERE origin_peer=? AND origin_task=?)+(SELECT COUNT(*) FROM net_tasks_hidden WHERE peer=? AND task=?)`, p.Peer, t.ID, p.Peer, t.ID).Scan(&here); err != nil {
				return nil, err
			}
			if here == 0 {
				keep = append(keep, t)
			}
		}
		if len(keep) > 0 {
			p.Tasks = keep
			out = append(out, p)
		}
	}
	return out, nil
}
