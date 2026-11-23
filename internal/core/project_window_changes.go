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
	return installWindowChanges(db)
}

// splitWindowChanges joins small revision readings, not SQLite write
// transactions. A task is tracked only through the requested project.
func splitWindowChanges(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), project, task string) (WindowChanges, error) {
	var shared, local windowChangeParts
	var file sql.NullString
	var registered bool
	err := computer.QueryRowContext(ctx, `SELECT `+windowChangeFields+`,
 (SELECT file FROM project_storage_stage WHERE project=?),
 EXISTS(SELECT 1 FROM task_storage_routes WHERE id=? AND project=?)`, project, "", project, task, project).
		Scan(&shared.global, &shared.project, &shared.feed, &file, &registered)
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
		err = db.QueryRowContext(ctx, `SELECT `+windowChangeFields+` WHERE (SELECT project FROM project_snapshot)=?`, project, task, project).
			Scan(&local.global, &local.project, &local.feed)
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
