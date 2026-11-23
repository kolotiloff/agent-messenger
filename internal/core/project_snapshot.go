package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// SnapshotProjectDatabase writes a consistent copy of one project's history
// to a standalone SQLite file. It does not change the live queue. The
// snapshot has no network configuration, coordinator claims or active
// registry, and is not an executable queue.
func SnapshotProjectDatabase(ctx context.Context, source, project, output string) error {
	return snapshotProjectDatabase(ctx, source, project, output, false)
}

func snapshotProjectDatabase(ctx context.Context, source, project, output string, withDelivery bool) error {
	if !ValidProjectName(project) {
		return fmt.Errorf("%w: invalid project name", ErrRejected)
	}
	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("%w: an output database file is required", ErrRejected)
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	output, err = filepath.Abs(output)
	if err != nil {
		return err
	}
	scratch, err := os.CreateTemp(filepath.Dir(output), ".messenger-project-*.sqlite")
	if err != nil {
		return err
	}
	defer os.Remove(scratch.Name())
	if err := scratch.Close(); err != nil {
		return err
	}
	db, err := sql.Open("sqlite", sqliteFileURL(source, url.Values{
		"mode":    {"ro"},
		"_pragma": {"busy_timeout(5000)", "foreign_keys(1)"},
	}))
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "ATTACH DATABASE ? AS snapshot", scratch.Name()); err != nil {
		return fmt.Errorf("attach project snapshot: %w", err)
	}
	if err := writeProjectSnapshot(ctx, db, project, withDelivery); err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}
	// Publish only a complete, checked database. Link is atomic and refuses
	// an existing destination, including one created by a concurrent export.
	if err := os.Link(scratch.Name(), output); err != nil {
		return fmt.Errorf("publish project snapshot: %w", err)
	}
	return nil
}

func writeProjectSnapshot(ctx context.Context, db *sql.DB, project string, withDelivery bool) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRowContext(ctx, "SELECT name FROM main.projects WHERE name=?", project).Scan(&name); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown project %q", ErrRejected, project)
		}
		return err
	}
	// Parents precede children, so foreign keys can stay enabled. The source
	// column layout includes any migrations that were applied on opening it.
	tables := []struct{ name, where string }{
		{"projects", "name=?"},
		{"project_folders", "project=?"},
		{"tasks", "project=?"},
		{"task_runs", "task_id IN (SELECT id FROM snapshot.tasks)"},
		{"task_questions", "task_id IN (SELECT id FROM snapshot.tasks)"},
		{"task_owner_notes", "task_id IN (SELECT id FROM snapshot.tasks)"},
		{"task_events", "project=?"},
		{"task_run_items", "run_id IN (SELECT id FROM snapshot.task_runs)"},
		{"remote_task_items", "task_id IN (SELECT id FROM snapshot.tasks)"},
		{"task_outputs", "task_id IN (SELECT id FROM snapshot.tasks)"},
		{"task_images", "task_id IN (SELECT id FROM snapshot.tasks)"},
		{"feedback", "project=?"},
	}
	if withDelivery {
		tables = append(tables, struct{ name, where string }{
			"net_outbox", "seq IN (SELECT seq FROM (" + migrationOutboxRoutes + ") WHERE project=?)",
		})
	}
	for _, table := range tables {
		if err := copyProjectTable(ctx, tx, table.name, table.where, project); err != nil {
			return fmt.Errorf("copy project table %s: %w", table.name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE snapshot.project_snapshot (
 format INTEGER NOT NULL, source_schema INTEGER NOT NULL, project TEXT NOT NULL, created_at TEXT NOT NULL
 )`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO snapshot.project_snapshot
 SELECT 1, CAST(value AS INTEGER), ?, ? FROM main.meta WHERE key='schema_version'`, name, now()); err != nil {
		return err
	}
	var check string
	if err := tx.QueryRowContext(ctx, "PRAGMA snapshot.quick_check").Scan(&check); err != nil {
		return err
	}
	if check != "ok" {
		return fmt.Errorf("project snapshot integrity check failed: %s", check)
	}
	return tx.Commit()
}

func copyProjectTable(ctx context.Context, tx *sql.Tx, table, where, project string) error {
	var ddl string
	if err := tx.QueryRowContext(ctx, "SELECT sql FROM main.sqlite_schema WHERE type='table' AND name=?", table).Scan(&ddl); err != nil {
		return err
	}
	definition, ok := strings.CutPrefix(ddl, "CREATE TABLE ")
	if !ok {
		return fmt.Errorf("unsupported table definition")
	}
	if _, err := tx.ExecContext(ctx, "CREATE TABLE snapshot."+definition); err != nil {
		return err
	}
	var args []any
	if strings.Contains(where, "?") {
		args = append(args, project)
	}
	_, err := tx.ExecContext(ctx, "INSERT INTO snapshot."+table+" SELECT * FROM main."+table+" WHERE "+where+" ORDER BY rowid", args...)
	return err
}
