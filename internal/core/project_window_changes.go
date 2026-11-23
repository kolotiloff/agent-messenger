package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
)

func stageProjectRevisions(path string) error {
	db, err := sql.Open("sqlite", sqliteFileURL(path, url.Values{
		"mode": {"rw"}, "_pragma": {sqliteBusyTimeout},
	}))
	if err != nil {
		return err
	}
	defer db.Close()
	if err := installWindowChanges(db); err != nil {
		return err
	}
	return enableProjectWAL(context.Background(), db)
}

func enableProjectWAL(ctx context.Context, db *sql.DB) error {
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode=WAL").Scan(&mode); err != nil {
		return err
	}
	if mode != "wal" {
		return fmt.Errorf("project storage did not enter WAL mode: %s", mode)
	}
	return nil
}

// splitWindowChanges joins small revision readings, not SQLite write
// transactions. A task is tracked only through the requested project.
func splitWindowChanges(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), project, task string) (WindowChanges, error) {
	var shared, local windowChangeParts
	var file sql.NullString
	var registered bool
	err := scanWindowRevision(ctx, computer, `SELECT `+windowChangeFields+`,
 (SELECT file FROM project_storage_stage WHERE project=?),
 EXISTS(SELECT 1 FROM task_storage_routes WHERE id=? AND project=?)`, []any{project, "", project, task, project},
		&shared.global, &shared.project, &shared.feed, &file, &registered)
	if err != nil {
		return WindowChanges{}, err
	}
	if project != "" {
		if !file.Valid {
			return WindowChanges{}, fmt.Errorf("%w: project is missing from the storage catalog", ErrRejected)
		}
		if file.String != projectMigrationFile(project) {
			return WindowChanges{}, fmt.Errorf("%w: invalid project database route", ErrRejected)
		}
		db, err := openProject(file.String)
		if err != nil {
			return WindowChanges{}, err
		}
		if !registered {
			task = ""
		}
		err = scanWindowRevision(ctx, db, `SELECT `+windowChangeFields+` WHERE (SELECT project FROM project_snapshot)=?`, []any{project, task, project},
			&local.global, &local.project, &local.feed)
		if errors.Is(err, sql.ErrNoRows) {
			return WindowChanges{}, fmt.Errorf("%w: the database belongs to another project", ErrRejected)
		}
		if err != nil {
			return WindowChanges{}, err
		}
	}
	return WindowChanges{
		Window: fmt.Sprintf("split.%d.%d.%d.%d", shared.global, shared.project, local.global, local.project),
		Feed:   fmt.Sprint(local.feed),
	}, nil
}
