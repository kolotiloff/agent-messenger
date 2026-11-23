package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"time"
)

// TaskEventReader reads committed events without initializing or writing the
// database. This lets a background watcher open an existing queue while the
// client holds a write transaction.
type TaskEventReader struct {
	db                *sql.DB
	root              string
	computer, project *sql.DB
	generation, name  string
}

func OpenTaskEventReader(path string) (*TaskEventReader, error) {
	db, err := sql.Open("sqlite", sqliteFileURL(path, url.Values{
		"mode":    {"ro"},
		"_pragma": {sqliteBusyTimeout},
	}))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &TaskEventReader{db: db, root: filepath.Dir(path)}, nil
}

func (r *TaskEventReader) Close() error {
	err := r.db.Close()
	if r.project != nil {
		err = errors.Join(err, r.project.Close())
	}
	if r.computer != nil {
		err = errors.Join(err, r.computer.Close())
	}
	return err
}

func (r *TaskEventReader) TaskEvents(ctx context.Context, project, audience string, after int64, limit int) ([]TaskEvent, int64, error) {
	if err := r.selectGeneration(); err != nil {
		return nil, after, err
	}
	if r.computer == nil {
		return readTaskEvents(ctx, r.db, project, audience, after, limit)
	}
	if r.name != project || r.project == nil {
		var file string
		err := r.computer.QueryRowContext(ctx, "SELECT file FROM project_storage_stage WHERE project=?", project).Scan(&file)
		if err != nil {
			return nil, after, err
		}
		if file != projectMigrationFile(project) {
			return nil, after, fmt.Errorf("%w: invalid project database route", ErrRejected)
		}
		db, err := openEventDatabase(filepath.Join(r.root, r.generation, file))
		if err != nil {
			return nil, after, err
		}
		if r.project != nil {
			_ = r.project.Close()
		}
		r.project, r.name = db, project
	}
	return readTaskEvents(ctx, r.project, project, audience, after, limit)
}

func openEventDatabase(path string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", sqliteFileURL(path, url.Values{"mode": {"ro"}, "_pragma": {sqliteBusyTimeout}}))
	if err == nil {
		db.SetMaxOpenConns(1)
	}
	return db, err
}

func (r *TaskEventReader) selectGeneration() error {
	if r.computer != nil {
		return nil
	}
	generation, err := readProjectGeneration(r.db)
	if err != nil || generation == "" {
		return err
	}
	r.computer, err = openEventDatabase(filepath.Join(r.root, generation, migrationComputerFile))
	r.generation = generation
	return err
}

func (r *TaskEventReader) PendingProjectNotice(ctx context.Context, project string) (bool, error) {
	if r.computer == nil {
		return false, nil
	}
	var pending bool
	err := r.computer.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM project_notices WHERE project=?)", project).Scan(&pending)
	return pending, err
}

// MacUpdateRestartOverdue lets a read-only watcher wake the coordinator
// for a contact check without writing network state itself.
func (r *TaskEventReader) MacUpdateRestartOverdue(ctx context.Context, at time.Time) (bool, error) {
	var planned string
	db := r.db
	if r.computer != nil {
		db = r.computer
	}
	if err := db.QueryRowContext(ctx, `SELECT COALESCE((SELECT value FROM meta WHERE key='mac_update_restart'),'')`).Scan(&planned); err != nil {
		return false, err
	}
	_, until := macUpdateRestart(planned)
	return !until.IsZero() && !at.Before(until), nil
}

func readTaskEvents(ctx context.Context, db *sql.DB, project, audience string, after int64, limit int) ([]TaskEvent, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	// A single SELECT already has a consistent snapshot. Store.tx would
	// start BEGIN IMMEDIATE and compete with writers even for this read.
	rows, err := db.QueryContext(ctx, `SELECT seq,task_id,kind,text,COALESCE(question_id,''),created_at
 FROM task_events WHERE project=? AND audience=? AND seq>? ORDER BY seq LIMIT ?`, project, audience, after, limit)
	if err != nil {
		return nil, after, err
	}
	defer rows.Close()
	var out []TaskEvent
	cursor := after
	for rows.Next() {
		var e TaskEvent
		if err := rows.Scan(&e.Seq, &e.TaskID, &e.Kind, &e.Text, &e.QuestionID, &e.CreatedAt); err != nil {
			return nil, after, err
		}
		out = append(out, e)
		cursor = e.Seq
	}
	if err := rows.Err(); err != nil {
		return nil, after, err
	}
	return out, cursor, nil
}
