package core

import (
	"database/sql"
	"errors"
)

// Task activity for the coordinator (T-071): when the worker last did
// something and what, so a working task can be told from a stuck one, and
// the tokens the task has used so far. It only reads what the window
// already keeps: the worker's event items here, or the progress lines the
// running computer forwards for another computer's task.

// maxActivityText bounds the last action shown to the coordinator.
const maxActivityText = 300

// TaskActivity is a task's last worker action and its tokens.
type TaskActivity struct {
	At     string
	Last   string
	Tokens int64
}

// RunningTaskCommand is the last observed start without a matching end.
// A remote start time is when this computer received the observation.
type RunningTaskCommand struct {
	Command string
	At      string
}

// TaskRunningCommand reuses the command timing records from the task feed.
// A full feed may have discarded an ending record, so it supplies no
// active command once its timing allowance is exhausted.
func (s *Store) TaskRunningCommand(id string, remote bool) (RunningTaskCommand, error) {
	store, err := s.storeForTask(id)
	if err != nil {
		return RunningTaskCommand{}, err
	}
	var row *sql.Row
	if remote {
		row = store.db.QueryRow(`WITH timing AS (
 SELECT seq,kind,title,text,created_at,substr(title,1,instr(title,'/')) AS run
 FROM remote_task_items WHERE task_id=? AND kind IN ('command_start','command_end')
), candidate AS (
 SELECT i.text,i.created_at,i.run FROM timing i
 WHERE i.kind='command_start' AND i.text<>''
 AND i.created_at>=COALESCE((SELECT MAX(created_at) FROM task_events
   WHERE project=(SELECT project FROM tasks WHERE id=?) AND task_id=?
   AND kind IN ('continued','worker_still_running')), '')
 AND NOT EXISTS (SELECT 1 FROM timing e WHERE e.seq>i.seq AND e.kind='command_end' AND e.title=i.title)
 ORDER BY i.seq DESC LIMIT 1
)
 SELECT c.text,c.created_at FROM candidate c
 WHERE (SELECT COUNT(*) FROM timing) < ? AND (SELECT COUNT(*) FROM timing WHERE run=c.run) < ?`,
			id, id, id, 2*maxRunItems, 2*maxForwardedItems)
	} else {
		row = store.db.QueryRow(`SELECT i.text,i.created_at FROM task_run_items i
 WHERE i.run_id=(SELECT id FROM task_runs WHERE task_id=? AND state='running' AND final_at IS NULL
   ORDER BY ordinal DESC LIMIT 1)
 AND i.kind='command_start' AND i.text<>''
 AND (SELECT COUNT(*) FROM task_run_items x WHERE x.run_id=i.run_id
   AND x.kind IN ('command_start','command_end')) < ?
 AND NOT EXISTS (SELECT 1 FROM task_run_items e WHERE e.run_id=i.run_id
   AND e.seq>i.seq AND e.kind='command_end' AND e.title=i.title)
 ORDER BY i.seq DESC LIMIT 1`, id, 2*maxRunItems)
	}
	var command RunningTaskCommand
	err = row.Scan(&command.Command, &command.At)
	if errors.Is(err, sql.ErrNoRows) {
		return RunningTaskCommand{}, nil
	}
	command.Command = clip(command.Command, maxActivityText)
	return command, err
}

// TaskActivity returns a task's last worker action and its tokens so far.
func (s *Store) TaskActivity(id string) (TaskActivity, error) {
	if s.split != nil {
		scoped, err := s.storeForTask(id)
		if err != nil {
			return TaskActivity{}, err
		}
		return scoped.TaskActivity(id)
	}
	var a TaskActivity
	err := s.tx(func(tx *sql.Tx) error {
		if err := tx.QueryRow(`SELECT usage_input+usage_output FROM tasks WHERE id=?`, id).Scan(&a.Tokens); err != nil {
			return err
		}
		var at, kind, title, text string
		err := tx.QueryRow(`SELECT created_at,kind,title,text FROM (
 SELECT i.created_at,i.kind,i.title,i.text,r.ordinal AS o,i.seq AS s FROM task_run_items i JOIN task_runs r ON r.id=i.run_id WHERE r.task_id=?
 UNION ALL
 SELECT created_at,kind,title,text,0,seq FROM remote_task_items WHERE task_id=? AND kind IN ('text','tool'))
 ORDER BY created_at DESC, o DESC, s DESC LIMIT 1`, id, id).Scan(&at, &kind, &title, &text)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		a.At, a.Last = at, text
		if kind == "tool" && title != "" {
			a.Last = title + ": " + text
		}
		a.Last = clip(a.Last, maxActivityText)
		return nil
	})
	return a, err
}
