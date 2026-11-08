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

// TaskActivity returns a task's last worker action and its tokens so far.
func (s *Store) TaskActivity(id string) (TaskActivity, error) {
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
