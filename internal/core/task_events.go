package core

import (
	"context"
	"database/sql"
	"net/url"
)

// TaskEventReader reads committed events without initializing or writing the
// database. This lets a background watcher open an existing queue while the
// client holds a write transaction.
type TaskEventReader struct{ db *sql.DB }

func OpenTaskEventReader(path string) (*TaskEventReader, error) {
	db, err := sql.Open("sqlite", sqliteFileURL(path, url.Values{
		"mode":    {"ro"},
		"_pragma": {sqliteBusyTimeout},
	}))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &TaskEventReader{db: db}, nil
}

func (r *TaskEventReader) Close() error { return r.db.Close() }

func (r *TaskEventReader) TaskEvents(ctx context.Context, project, audience string, after int64, limit int) ([]TaskEvent, int64, error) {
	return readTaskEvents(ctx, r.db, project, audience, after, limit)
}

// TaskEvents returns a project's events after the cursor for one audience,
// at most limit. Reading never moves anything: a caller that repeats the
// same cursor gets the same events again.
func (s *Store) TaskEvents(project, audience string, after int64, limit int) ([]TaskEvent, int64, error) {
	return readTaskEvents(context.Background(), s.db, project, audience, after, limit)
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
