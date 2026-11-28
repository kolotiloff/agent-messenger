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
	if !ValidProjectName(project) {
		return fmt.Errorf("%w: invalid project name", ErrRejected)
	}
	source, err := filepath.Abs(source)
	if err != nil {
		return err
	}
	legacy, err := sql.Open("sqlite", sqliteFileURL(source, url.Values{"mode": {"ro"}, "_pragma": {sqliteBusyTimeout}}))
	if err != nil {
		return err
	}
	generation, err := readProjectGeneration(legacy)
	_ = legacy.Close()
	if err != nil {
		return err
	}
	if generation == "" {
		return snapshotProjectDatabase(ctx, source, project, output, false, "")
	}
	root := filepath.Join(filepath.Dir(source), generation)
	active, err := openSplitStore(ctx, root)
	if err != nil {
		return err
	}
	defer active.Close()
	if _, err := active.projectStore(project); err != nil {
		return err
	}
	return snapshotProjectDatabase(ctx, filepath.Join(root, projectMigrationFile(project)), project, output, false, filepath.Join(root, migrationComputerFile))
}

func snapshotProjectDatabase(ctx context.Context, source, project, output string, withDelivery bool, computer string) error {
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
	catalog := "main"
	if computer != "" {
		if _, err := db.ExecContext(ctx, "ATTACH DATABASE ? AS computer", sqliteFileURL(computer, url.Values{"mode": {"ro"}})); err != nil {
			return fmt.Errorf("attach computer database: %w", err)
		}
		catalog = "computer"
	}
	if _, err := db.ExecContext(ctx, "ATTACH DATABASE ? AS snapshot", scratch.Name()); err != nil {
		return fmt.Errorf("attach project snapshot: %w", err)
	}
	if err := writeProjectSnapshot(ctx, db, project, withDelivery, catalog); err != nil {
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

func writeProjectSnapshot(ctx context.Context, db *sql.DB, project string, withDelivery bool, catalog string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var name string
	if err := tx.QueryRowContext(ctx, "SELECT name FROM "+catalog+".projects WHERE name=?", project).Scan(&name); err != nil {
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
		{"task_attachments", "task_id IN (SELECT id FROM snapshot.tasks)"},
		{"message_attachments", "task_id IN (SELECT id FROM snapshot.tasks)"},
	}
	if withDelivery {
		tables = append(tables, struct{ name, where string }{
			"net_outbox", "seq IN (SELECT seq FROM (" + migrationOutboxRoutes + ") WHERE project=?)",
		})
		tables = append(tables, struct{ name, where string }{"net_applied", "0"})
		// The feedback journal belongs to the computer. Keep its empty
		// schema for the same revision triggers on all live files.
		tables = append(tables, struct{ name, where string }{"feedback", "0"})
	} else {
		tables = append(tables, struct{ name, where string }{"feedback", "project=?"})
	}
	for _, table := range tables {
		source := "main"
		switch table.name {
		case "projects", "project_folders", "feedback":
			source = catalog
		}
		if err := copyProjectTable(ctx, tx, source, table.name, table.where, project); err != nil {
			return fmt.Errorf("copy project table %s: %w", table.name, err)
		}
	}
	for _, name := range []string{"task_assignments", "task_reviews", "task_attempts"} {
		exists, err := hasAnalyticsTable(ctx, tx, name)
		if err != nil {
			return err
		}
		if exists {
			if err := copyProjectTable(ctx, tx, "main", name, "task_id IN (SELECT id FROM snapshot.tasks)", project); err != nil {
				return fmt.Errorf("copy project table %s: %w", name, err)
			}
		}
	}
	if withDelivery {
		// A task frame must use the same machine identity after cutover.
		// Keep only that identity and the continuation records owned by
		// this project's tasks. Settings and broker cursors stay shared.
		if _, err := tx.ExecContext(ctx, `CREATE TABLE snapshot.project_notice_receipts (seq INTEGER PRIMARY KEY);
 CREATE TABLE snapshot.meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
 INSERT INTO snapshot.meta SELECT key,value FROM main.meta
 WHERE key='computer_id'
 OR (key GLOB 'net_continue_prev:*' AND substr(key,length('net_continue_prev:')+1) IN (SELECT id FROM snapshot.tasks))
 OR (key GLOB 'net_continue:*' AND substr(value,1,instr(value,char(10))-1) IN (SELECT id FROM snapshot.tasks));`); err != nil {
			return fmt.Errorf("copy project delivery metadata: %w", err)
		}
		// New project messages start beyond all already allocated legacy
		// sequence numbers, even when a project had no pending message.
		if _, err := tx.ExecContext(ctx, `DELETE FROM snapshot.sqlite_sequence WHERE name='net_outbox';
 INSERT INTO snapshot.sqlite_sequence(name,seq)
 VALUES ('net_outbox',COALESCE((SELECT seq FROM main.sqlite_sequence WHERE name='net_outbox'),0));
 CREATE INDEX IF NOT EXISTS snapshot.task_run_intents ON task_runs(task_id) WHERE state='intent';
 CREATE INDEX IF NOT EXISTS snapshot.tasks_window_history ON tasks(project,created_at)
 WHERE hidden_at IS NULL AND state IN `+finishedWindowStates+`;
 CREATE INDEX IF NOT EXISTS snapshot.tasks_window_active ON tasks(project,created_at)
 WHERE hidden_at IS NULL AND state NOT IN `+finishedWindowStates+`;
 CREATE INDEX IF NOT EXISTS snapshot.tasks_state ON tasks(state);
 CREATE INDEX IF NOT EXISTS snapshot.task_runs_state ON task_runs(state, task_id);
 CREATE INDEX IF NOT EXISTS snapshot.net_applied_op ON net_applied(op_id)`); err != nil {
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE snapshot.project_snapshot (
 format INTEGER NOT NULL, source_schema INTEGER NOT NULL, project TEXT NOT NULL, created_at TEXT NOT NULL
 )`); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO snapshot.project_snapshot
 SELECT 1, CAST(value AS INTEGER), ?, ? FROM `+catalog+`.meta WHERE key='schema_version'`, name, now()); err != nil {
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

func copyProjectTable(ctx context.Context, tx *sql.Tx, source, table, where, project string) error {
	var ddl string
	if err := tx.QueryRowContext(ctx, "SELECT sql FROM "+source+".sqlite_schema WHERE type='table' AND name=?", table).Scan(&ddl); err != nil {
		if errors.Is(err, sql.ErrNoRows) && (table == "message_attachments" || table == "task_attachments") {
			// The source can predate either attachment table.
			return nil
		}
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
	if _, err := tx.ExecContext(ctx, "INSERT INTO snapshot."+table+" SELECT * FROM "+source+"."+table+" WHERE "+where+" ORDER BY rowid", args...); err != nil {
		return err
	}
	return copyProjectIndexes(ctx, tx, source, table)
}

func copyProjectIndexes(ctx context.Context, tx *sql.Tx, source, table string) error {
	// Implicit indexes come from CREATE TABLE. Preserve explicit indexes
	// too, including the partial unique index for conversation IDs.
	rows, err := tx.QueryContext(ctx, `SELECT sql FROM `+source+`.sqlite_schema
 WHERE type='index' AND tbl_name=? AND sql IS NOT NULL ORDER BY name`, table)
	if err != nil {
		return err
	}
	defer rows.Close()
	var definitions []string
	for rows.Next() {
		var ddl string
		if err := rows.Scan(&ddl); err != nil {
			return err
		}
		prefix, definition, ok := strings.Cut(ddl, "INDEX ")
		if !ok || (prefix != "CREATE " && prefix != "CREATE UNIQUE ") {
			return fmt.Errorf("unsupported index definition")
		}
		definitions = append(definitions, prefix+"INDEX snapshot."+definition)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, ddl := range definitions {
		if _, err := tx.ExecContext(ctx, ddl); err != nil {
			return err
		}
	}
	return nil
}
