package core

import (
	"database/sql"
	"errors"
	"fmt"
)

const (
	archiveStored  = "archived"
	archiveRemoved = "removed"
)

func finishedTask(state string) bool {
	return state == "done" || state == "failed" || state == "cancelled"
}

func taskHistoryEditKey(project string) string { return "task_history_edit:" + project }

func advanceTaskHistoryEdit(tx *sql.Tx, project string) error {
	_, err := tx.Exec(`INSERT INTO meta(key,value) VALUES (?,'1')
 ON CONFLICT(key) DO UPDATE SET value=CAST(meta.value AS INTEGER)+1`, taskHistoryEditKey(project))
	return err
}

// SetTaskArchive changes only the originating computer's finished task.
// Removal is a terminal archive state. Task history, files and session
// identity remain available for existing records such as Statistics.
func (s *Store) SetTaskArchive(id, action string, expected int64) error {
	var from, to string
	switch action {
	case "archive":
		to = archiveStored
	case "restore":
		from = archiveStored
	case "remove":
		from, to = archiveStored, archiveRemoved
	default:
		return fmt.Errorf("%w: неизвестное действие с архивом", ErrRejected)
	}
	return s.taskTx(id, func(_ *sql.Tx, tx *sql.Tx) error {
		var state, archive, project string
		var version int64
		var hidden bool
		err := tx.QueryRow(`SELECT state,archive_state,archive_version,hidden_at IS NOT NULL,project
 FROM tasks WHERE id=? AND origin_peer IS NULL`, id).Scan(&state, &archive, &version, &hidden, &project)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && !finishedTask(state)) {
			return fmt.Errorf("%w: архив доступен для завершённой задачи на компьютере, который её создал", ErrRejected)
		}
		if err != nil {
			return err
		}
		if version > 0 && version-1 == expected && archive == to {
			return nil
		}
		if hidden || archive != from || version != expected {
			return fmt.Errorf("%w: состояние архива уже изменилось", ErrRejected)
		}
		version++
		// Keep the removed card hidden even if the user launches an older app.
		if _, err := tx.Exec(`UPDATE tasks SET archive_state=?,archive_version=?,version=version+1,
 hidden_at=CASE WHEN ?='removed' THEN COALESCE(hidden_at,?) ELSE hidden_at END WHERE id=?`,
			to, version, to, now(), id); err != nil {
			return err
		}
		if err := advanceTaskHistoryEdit(tx, project); err != nil {
			return err
		}
		links, err := taskLink(tx, id)
		if err != nil || links.remote == nil {
			return err
		}
		remote := links.remote
		return enqueueNet(tx, remote.broker, remote.peer, fmt.Sprintf("archive.%s.%d", id, version), id,
			netBody{Kind: "archive_task", Task: id, State: to, ArchiveVersion: version})
	})
}

func taskMetadataUpdate(kind string) bool {
	return kind == "rename_task" || kind == "archive_task"
}

func applyTaskMetadata(tx *sql.Tx, from netLink, body netBody) error {
	if body.Kind == "rename_task" {
		return applyTaskTitle(tx, from, body)
	}
	if body.ArchiveVersion <= 0 || (body.State != "" && body.State != archiveStored && body.State != archiveRemoved) {
		return nil
	}
	// A cancellation can close the origin's record before the worker is
	// known to have stopped. Record the lifecycle without hiding a live row.
	var project string
	err := tx.QueryRow(`UPDATE tasks SET archive_state=?,archive_version=?,version=version+1,
 hidden_at=CASE WHEN ?='removed' AND state IN `+finishedWindowStates+` THEN COALESCE(hidden_at,?) ELSE hidden_at END
 WHERE origin_broker=? AND origin_peer=? AND origin_task=? AND archive_version<? AND archive_state<>'removed' RETURNING project`,
		body.State, body.ArchiveVersion, body.State, now(), from.broker, from.peer, from.task, body.ArchiveVersion).Scan(&project)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return advanceTaskHistoryEdit(tx, project)
}
