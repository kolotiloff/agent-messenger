package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type WindowChanges struct {
	Window string `json:"window"`
	Feed   string `json:"feed"`
}

// WindowChanges reads small committed revisions, including writes made by
// a worker or coordinator in another process. It never reads a transcript.
func (s *Store) WindowChanges(ctx context.Context, project, task string) (WindowChanges, error) {
	if s.split != nil {
		return splitWindowChanges(ctx, s.db, s.split.open, project, task)
	}
	parts, err := readWindowChanges(ctx, s.db, project, task)
	return WindowChanges{Window: fmt.Sprintf("%d.%d", parts.global, parts.project), Feed: fmt.Sprint(parts.feed)}, err
}

type windowChangeParts struct {
	global, project, feed int64
}

const windowChangeFields = `
 COALESCE((SELECT revision FROM window_revisions WHERE scope='window' AND item=''),0),
 COALESCE((SELECT revision FROM window_revisions WHERE scope='project' AND item=?),0),
 COALESCE((SELECT revision FROM window_revisions WHERE scope='feed' AND item=?),0)`

func readWindowChanges(ctx context.Context, db *sql.DB, project, task string) (windowChangeParts, error) {
	var parts windowChangeParts
	err := db.QueryRowContext(ctx, `SELECT `+windowChangeFields, project, task).Scan(&parts.global, &parts.project, &parts.feed)
	return parts, err
}

func installWindowChanges(db *sql.DB) error {
	// The last trigger and table are installed in one SQLite transaction.
	// Task MCP processes normally take just this read when opening the store.
	var machine, history, machineInstalled, historyInstalled bool
	err := db.QueryRow(`SELECT
 EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='control'),
 EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='tasks'),
 EXISTS(SELECT 1 FROM sqlite_schema WHERE type='trigger' AND name='am_window_control_update'),
 EXISTS(SELECT 1 FROM sqlite_schema WHERE type='trigger' AND name='am_window_task_runs_structure')`).
		Scan(&machine, &history, &machineInstalled, &historyInstalled)
	if err != nil || (machine == machineInstalled && history == historyInstalled) {
		return err
	}
	var schema strings.Builder
	schema.WriteString(`CREATE TABLE IF NOT EXISTS window_revisions(
 scope TEXT NOT NULL, item TEXT NOT NULL, revision INTEGER NOT NULL,
 PRIMARY KEY(scope,item));`)
	bump := func(scope, id string) string {
		return `INSERT INTO window_revisions(scope,item,revision) SELECT '` + scope + `',` + id + `,1 WHERE (` + id + `) IS NOT NULL
 ON CONFLICT(scope,item) DO UPDATE SET revision=revision+1;`
	}
	trigger := func(table, event, when, contents string) {
		fmt.Fprintf(&schema, "CREATE TRIGGER IF NOT EXISTS am_window_%s_%s AFTER %s ON %s %s BEGIN %s END;", table, event, event, table, when, contents)
	}
	for _, event := range []string{"insert", "update", "delete"} {
		row := "NEW."
		if event == "delete" {
			row = "OLD."
		}
		project := bump("project", row+"project")
		if history && !historyInstalled {
			trigger("tasks", event, "", project+bump("feed", row+"id")+bump("feed_structure", row+"id"))
			for _, table := range []string{"task_events", "task_questions", "task_owner_notes", "remote_task_items", "task_images", "task_outputs"} {
				change := bump("feed", row+"task_id")
				if table != "remote_task_items" || event != "insert" {
					change += bump("feed_structure", row+"task_id")
				}
				trigger(table, event, "", change)
			}
			taskOfRun := "(SELECT task_id FROM task_runs WHERE id=" + row + "run_id)"
			itemsChanged := bump("feed", taskOfRun)
			if event == "insert" {
				// Later inserts into historical runs cannot be appended at the end.
				taskOfRun = "(SELECT task_id FROM task_runs r WHERE r.id=NEW.run_id AND EXISTS(SELECT 1 FROM task_runs n WHERE n.task_id=r.task_id AND n.ordinal>r.ordinal))"
			}
			trigger("task_run_items", event, "", itemsChanged+bump("feed_structure", taskOfRun))
			if event == "update" {
				// Output heartbeats alone do not change the list or transcript.
				// The window's periodic health snapshot covers their age.
				trigger("task_runs", event, `WHEN OLD.state IS NOT NEW.state OR OLD.launched_at IS NOT NEW.launched_at
 OR OLD.final_at IS NOT NEW.final_at OR OLD.processes IS NOT NEW.processes`,
					bump("project", "(SELECT project FROM tasks WHERE id=NEW.task_id)")+bump("feed", "NEW.task_id"))
				fmt.Fprintf(&schema, `CREATE TRIGGER IF NOT EXISTS am_window_task_runs_structure
 AFTER UPDATE OF kind,input,state,disposition,disposition_text,finished_at,detail,created_at,sender_agent,sender_model ON task_runs
 BEGIN %s END;`, bump("feed_structure", row+"task_id")+bump("feed", row+"task_id"))
			} else {
				trigger("task_runs", event, "", bump("project", "(SELECT project FROM tasks WHERE id="+row+"task_id)")+bump("feed", row+"task_id")+bump("feed_structure", row+"task_id"))
			}
		}
		trigger("feedback", event, "", bump("window", "''"))
		if machine && !machineInstalled {
			for _, table := range []string{"projects", "project_folders", "computer_agents", "net_updates", "pending_updates"} {
				trigger(table, event, "", bump("window", "''"))
			}
			chatChange, connectionChange := "", ""
			if event == "update" {
				chatChange = "WHEN OLD.pid IS NOT NEW.pid OR OLD.start_token IS NOT NEW.start_token OR OLD.boot_id IS NOT NEW.boot_id"
				connectionChange = "WHEN OLD.client IS NOT NEW.client"
			}
			trigger("coordinator_chats", event, chatChange, project)
			trigger("coordinator_connections", event, connectionChange, project)
		}
	}
	if machine && !machineInstalled {
		trigger("control", "update", "WHEN OLD.paused IS NOT NEW.paused", bump("window", "''"))
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(schema.String()); err != nil {
		return err
	}
	return tx.Commit()
}
