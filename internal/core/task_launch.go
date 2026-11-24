package core

import (
	"database/sql"
	"errors"
	"time"
)

const taskLaunchQueued = "launch_queued"
const taskLaunchObserved = "launch_observed"

const taskLaunchEventsIndex = `CREATE INDEX IF NOT EXISTS task_events_launch
 ON task_events(task_id, seq) WHERE kind IN ('launch_queued','launch_observed');`

type taskLaunchQuery interface {
	QueryRow(string, ...any) *sql.Row
}

func runPendingLaunch(db taskLaunchQuery, task string) (bool, error) {
	var state string
	err := db.QueryRow(`SELECT state FROM task_runs WHERE task_id=? ORDER BY ordinal DESC LIMIT 1`, task).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return state == "intent", err
}

func remoteLaunch(db taskLaunchQuery, task string) (pending bool, startedAt, lock string, err error) {
	var kind, text string
	err = db.QueryRow(`SELECT kind,text FROM task_events WHERE task_id=? AND kind IN ('launch_queued','launch_observed')
 ORDER BY seq DESC LIMIT 1`, task).Scan(&kind, &text)
	if errors.Is(err, sql.ErrNoRows) {
		return false, "", "", nil
	}
	if kind == taskLaunchQueued {
		return true, "", text, err
	}
	return false, text, "", err
}

// Applies task launch metadata from the registered executor without changing
// the task outcome or reopening a closed task.
func recordRemoteLaunch(tx *sql.Tx, task Task, kind, text string) error {
	_, started, _, err := remoteLaunch(tx, task.ID)
	if err != nil || started != "" {
		return err
	}
	if kind == taskLaunchObserved {
		if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
			return nil
		}
	} else {
		lock, valid := normalizeLock(text)
		if !valid {
			lock = ""
		}
		text = lock
	}
	if err := insertTaskEvent(tx, task.Project, task.ID, AudienceLog, kind, text, ""); err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE tasks SET version=version+1 WHERE id=?`, task.ID)
	return err
}

// A first launch is published only after the supervisor observes the process.
func reportFirstTaskLaunch(tx *sql.Tx, run string) error {
	var task string
	var origin netLink
	err := tx.QueryRow(`SELECT t.id,t.origin_broker,t.origin_peer,t.origin_task
 FROM task_runs r JOIN tasks t ON t.id=r.task_id
 WHERE r.id=? AND r.ordinal=1 AND t.origin_peer IS NOT NULL`, run).Scan(&task, &origin.broker, &origin.peer, &origin.task)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return enqueueNet(tx, origin.broker, origin.peer, "launch."+run, task, netBody{
		Kind: "event", Task: origin.task, Event: taskLaunchObserved, Audience: AudienceLog, State: "running", Text: now(),
	})
}
