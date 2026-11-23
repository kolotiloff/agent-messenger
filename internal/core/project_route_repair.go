package core

import (
	"context"
	"database/sql"
)

// unpublishedTaskFailure is what the owner and the coordinator read on a
// task whose explicit start failed between its two commits.
const unpublishedTaskFailure = "the task was saved but not registered on this computer, so it never started; start it again"

// FailUnpublishedTasks closes tasks that a project database recorded while
// the computer route for them was never published: the explicit start
// failed between the project commit and the computer commit (D-188).
// Launch and delivery both wait for that route, so such a task never ran
// and its start was never sent. It is failed rather than started, because
// its start reported an error and an uncertain result is never retried
// automatically. Its route is then published so the window, removal and a
// repeat of the same operation id treat it like any other task.
//
// Tasks received from another computer are left alone: their start was not
// acknowledged, so the broker delivers it again and that publishes the
// route. The shared writer lock is held throughout, so a start in progress
// is never mistaken for a failed one.
func (s *Store) FailUnpublishedTasks() (int, error) {
	if s.split == nil {
		return 0, nil
	}
	ctx := context.Background()
	common, err := beginSplitAdmission(ctx, s.db)
	if err != nil {
		return 0, err
	}
	defer common.Rollback()
	routed := map[string]bool{}
	rows, err := common.QueryContext(ctx, "SELECT id FROM task_storage_routes")
	if err != nil {
		return 0, err
	}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		routed[id] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	files, err := splitProjectFiles(ctx, common)
	if err != nil {
		return 0, err
	}
	failed := 0
	for _, file := range files {
		db, err := s.split.open(file)
		if err != nil {
			return failed, err
		}
		n, err := failUnpublishedInProject(ctx, common, db, routed)
		failed += n
		if err != nil {
			return failed, err
		}
	}
	return failed, common.Commit()
}

type unpublishedTask struct {
	id, project, state string
	started            bool
	broker, peer, host sql.NullString
}

func failUnpublishedInProject(ctx context.Context, common *sql.Tx, db *sql.DB, routed map[string]bool) (int, error) {
	project, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer project.Rollback()
	rows, err := project.QueryContext(ctx, `SELECT id,project,state,remote_broker,remote_peer,remote_computer,
 EXISTS(SELECT 1 FROM task_runs r WHERE r.task_id=t.id AND r.state<>'intent')
 FROM tasks t WHERE COALESCE(origin_broker,'')=''`)
	if err != nil {
		return 0, err
	}
	var missing []unpublishedTask
	for rows.Next() {
		var t unpublishedTask
		if err := rows.Scan(&t.id, &t.project, &t.state, &t.broker, &t.peer, &t.host, &t.started); err != nil {
			rows.Close()
			return 0, err
		}
		if !routed[t.id] {
			missing = append(missing, t)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(missing) == 0 {
		return 0, err
	}
	failed := 0
	for _, t := range missing {
		if t.state != "running" || t.started {
			// Only a task that never ran is closed here. Anything else
			// keeps its state; publishing the route is enough.
			continue
		}
		if _, err := project.ExecContext(ctx, `UPDATE task_runs SET state='cancelled',detail='never registered on this computer' WHERE task_id=? AND state='intent'`, t.id); err != nil {
			return 0, err
		}
		// The start for another computer was held back with the route.
		if _, err := project.ExecContext(ctx, `DELETE FROM net_outbox WHERE task_id=? AND kind='start'`, t.id); err != nil {
			return 0, err
		}
		if err := setTaskState(project, t.id, "failed", unpublishedTaskFailure); err != nil {
			return 0, err
		}
		if err := insertTaskEvent(project, t.project, t.id, AudienceCoordinator, "failed", unpublishedTaskFailure, ""); err != nil {
			return 0, err
		}
		failed++
	}
	if err := project.Commit(); err != nil {
		return 0, err
	}
	for _, t := range missing {
		if _, err := common.ExecContext(ctx, `INSERT INTO task_storage_routes(id,project,remote_broker,remote_peer,remote_computer)
 VALUES (?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, t.id, t.project, t.broker, t.peer, t.host); err != nil {
			return failed, err
		}
	}
	return failed, nil
}
