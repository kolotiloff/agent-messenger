package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/modelcatalog"
)

const taskAnalyticsSchema = `
CREATE TABLE IF NOT EXISTS task_assignments (
 task_id TEXT PRIMARY KEY REFERENCES tasks(id),
 task_type TEXT NOT NULL,
 difficulty TEXT NOT NULL,
 retry_of TEXT NOT NULL,
 original_task_id TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS task_reviews (
 task_id TEXT NOT NULL REFERENCES tasks(id),
 op_id TEXT NOT NULL,
 result TEXT NOT NULL,
 reason TEXT NOT NULL,
 report_at TEXT NOT NULL,
 reviewed_at TEXT NOT NULL,
 PRIMARY KEY (task_id,op_id),
 UNIQUE (task_id,report_at)
);
CREATE TABLE IF NOT EXISTS task_attempts (
 task_id TEXT NOT NULL REFERENCES tasks(id),
 run_id TEXT NOT NULL,
 ordinal INTEGER NOT NULL,
 kind TEXT NOT NULL,
 state TEXT NOT NULL,
 launched_at TEXT NOT NULL,
 finished_at TEXT NOT NULL,
 reported_model TEXT NOT NULL,
 outcome_at TEXT NOT NULL DEFAULT '',
 usage TEXT,
 PRIMARY KEY (task_id,run_id)
);`

// TaskAssignment holds optional labels for analysis. It never enters a worker prompt.
type TaskAssignment struct {
	Type       string `json:"task_type,omitempty"`
	Difficulty string `json:"difficulty,omitempty"`
	RetryOf    string `json:"retry_of,omitempty"`
	OriginalID string `json:"original_task_id,omitempty"`
}

var taskTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}$`)

func (a TaskAssignment) valid() bool {
	return (a.Type == "" || taskTypePattern.MatchString(a.Type)) &&
		(a.Difficulty == "" || a.Difficulty == "simple" || a.Difficulty == "medium" || a.Difficulty == "complex") &&
		(a.RetryOf == "" || netRef.MatchString(a.RetryOf)) &&
		(a.OriginalID == "" || netRef.MatchString(a.OriginalID))
}

func readTaskAssignment(tx *sql.Tx, task string) (TaskAssignment, error) {
	var a TaskAssignment
	err := tx.QueryRow(`SELECT task_type,difficulty,retry_of,original_task_id FROM task_assignments WHERE task_id=?`, task).
		Scan(&a.Type, &a.Difficulty, &a.RetryOf, &a.OriginalID)
	if errors.Is(err, sql.ErrNoRows) {
		return a, nil
	}
	return a, err
}

func storeTaskAssignment(tx *sql.Tx, task, project string, a TaskAssignment, mirror bool) (TaskAssignment, error) {
	if a == (TaskAssignment{}) {
		return a, nil
	}
	if !mirror && a.RetryOf != "" {
		var original string
		var state string
		err := tx.QueryRow(`SELECT t.state,COALESCE(a.original_task_id,'')
 FROM tasks t LEFT JOIN task_assignments a ON a.task_id=t.id
 WHERE t.id=? AND t.project=? AND t.origin_task IS NULL`, a.RetryOf, project).
			Scan(&state, &original)
		if errors.Is(err, sql.ErrNoRows) || (err == nil && state != "done" && state != "failed" && state != "cancelled") {
			return a, fmt.Errorf("%w: retry_of must name a closed task managed in this project", ErrRejected)
		}
		if err != nil {
			return a, err
		}
		a.OriginalID = original
		if a.OriginalID == "" {
			a.OriginalID = a.RetryOf
		}
	}
	_, err := tx.Exec(`INSERT INTO task_assignments(task_id,task_type,difficulty,retry_of,original_task_id) VALUES (?,?,?,?,?)`,
		task, a.Type, a.Difficulty, a.RetryOf, a.OriginalID)
	return a, err
}

type TaskReview struct {
	Result     string `json:"result"`
	Reason     string `json:"reason,omitempty"`
	ReportAt   string `json:"report_at"`
	ReviewedAt string `json:"reviewed_at"`
}

var returnReasons = map[string]bool{
	"implementation_error": true, "missing_scope": true, "unclear_assignment": true,
	"changed_requirements": true, "technical_failure": true, "model_limit": true, "other": true,
}

// ReviewTask records the coordinator's assessment without changing the worker
// state, publishing a report, or launching a follow-up.
func (s *Store) ReviewTask(project, task, op, result, reason, reportAt string) (TaskReview, error) {
	var out TaskReview
	if !ValidProjectName(project) || !netRef.MatchString(task) || strings.TrimSpace(op) == "" || len(op) > 128 ||
		!(result == "accepted" && reason == "" || result == "returned" && returnReasons[reason]) {
		return out, fmt.Errorf("%w: review needs an operation id and accepted or returned with a known reason", ErrRejected)
	}
	store, err := s.projectStore(project)
	if err != nil {
		return out, err
	}
	err = store.tx(func(tx *sql.Tx) error {
		var state, finished string
		err := tx.QueryRow(`SELECT state,COALESCE(finished_at,'') FROM tasks WHERE id=? AND project=? AND origin_task IS NULL`, task, project).
			Scan(&state, &finished)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown task in this project", ErrRejected)
		}
		if err != nil {
			return err
		}
		err = tx.QueryRow(`SELECT result,reason,report_at,reviewed_at FROM task_reviews WHERE task_id=? AND op_id=?`, task, op).
			Scan(&out.Result, &out.Reason, &out.ReportAt, &out.ReviewedAt)
		if err == nil {
			if out.Result != result || out.Reason != reason || reportAt != "" && out.ReportAt != reportAt {
				return fmt.Errorf("%w: op_id already records a different review", ErrRejected)
			}
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if finished == "" || !(state == "done" || result == "returned" && (state == "failed" || state == "cancelled")) {
			return fmt.Errorf("%w: wait for the task to finish before reviewing it", ErrRejected)
		}
		if reportAt != "" && finished != reportAt {
			return fmt.Errorf("%w: this task has a newer outcome; read it before reviewing", ErrRejected)
		}
		out = TaskReview{Result: result, Reason: reason, ReportAt: finished, ReviewedAt: now()}
		r, err := tx.Exec(`INSERT INTO task_reviews(task_id,op_id,result,reason,report_at,reviewed_at)
 VALUES (?,?,?,?,?,?) ON CONFLICT(task_id,report_at) DO NOTHING`, task, op, result, reason, finished, out.ReviewedAt)
		if err != nil {
			return err
		}
		if count, err := r.RowsAffected(); err != nil || count != 1 {
			if err != nil {
				return err
			}
			return fmt.Errorf("%w: this outcome already has a review", ErrRejected)
		}
		return nil
	})
	return out, err
}

type taskAttempt struct {
	ID         string     `json:"run_id"`
	Ordinal    int        `json:"ordinal"`
	Kind       string     `json:"kind"`
	State      string     `json:"state"`
	LaunchedAt string     `json:"launched_at,omitempty"`
	FinishedAt string     `json:"finished_at"`
	Model      string     `json:"reported_model,omitempty"`
	Usage      *TaskUsage `json:"usage,omitempty"`
	OutcomeAt  string     `json:"outcome_at,omitempty"`
	// Calculated only for export, outside the execution and delivery path.
	DurationMS *int64       `json:"duration_ms,omitempty"`
	ModelInfo  *exportModel `json:"model,omitempty"`
}

func markAttemptOutcome(tx *sql.Tx, task, run string) error {
	_, err := tx.Exec(`UPDATE task_attempts SET outcome_at=(SELECT COALESCE(finished_at,'') FROM tasks WHERE id=?)
 WHERE task_id=? AND run_id=? AND finished_at<>''`, task, task, run)
	return err
}

func recordAttemptModel(tx *sql.Tx, run, task, model string) error {
	if model == "" {
		return nil
	}
	if _, _, err := modelcatalog.SplitReported(model); err != nil {
		return nil
	}
	_, err := tx.Exec(`INSERT INTO task_attempts(task_id,run_id,ordinal,kind,state,launched_at,finished_at,reported_model)
 SELECT ?,id,ordinal,kind,'','','',? FROM task_runs WHERE id=?
 ON CONFLICT(task_id,run_id) DO UPDATE SET reported_model=excluded.reported_model
 WHERE task_attempts.finished_at='' AND task_attempts.reported_model<>excluded.reported_model`, task, model, run)
	return err
}

func finishTaskAttempt(tx *sql.Tx, t Task, r TaskRun, state string, usage *TaskUsage) error {
	a := taskAttempt{ID: r.ID, Ordinal: r.Ordinal, Kind: r.Kind, State: state}
	err := tx.QueryRow(`SELECT COALESCE(r.launched_at,''),r.finished_at,COALESCE(a.reported_model,'')
 FROM task_runs r LEFT JOIN task_attempts a ON a.task_id=r.task_id AND a.run_id=r.id WHERE r.id=?`, r.ID).
		Scan(&a.LaunchedAt, &a.FinishedAt, &a.Model)
	if err != nil {
		return err
	}
	if usage.valid() {
		a.Usage = usage
	}
	if err := saveTaskAttempt(tx, t.ID, a); err != nil {
		return err
	}
	link, err := taskLink(tx, t.ID)
	if err != nil || link.origin == nil {
		return err
	}
	body, err := json.Marshal(a)
	if err != nil {
		return err
	}
	o := link.origin
	return enqueueNet(tx, o.broker, o.peer, "attempt."+r.ID, t.ID, netBody{Kind: "run_usage", Task: o.task, Analytics: body})
}

func saveTaskAttempt(tx *sql.Tx, task string, a taskAttempt) error {
	var usage any
	if a.Usage.valid() {
		raw, err := json.Marshal(a.Usage)
		if err != nil {
			return err
		}
		usage = string(raw)
	}
	_, err := tx.Exec(`INSERT INTO task_attempts(task_id,run_id,ordinal,kind,state,launched_at,finished_at,reported_model,usage)
 VALUES (?,?,?,?,?,?,?,?,?) ON CONFLICT(task_id,run_id) DO UPDATE SET
 ordinal=excluded.ordinal,kind=excluded.kind,state=excluded.state,launched_at=excluded.launched_at,
 finished_at=excluded.finished_at,reported_model=excluded.reported_model,usage=excluded.usage
 WHERE task_attempts.finished_at=''`, task, a.ID, a.Ordinal, a.Kind, a.State, a.LaunchedAt, a.FinishedAt, a.Model, usage)
	return err
}

func applyTaskAttempt(tx *sql.Tx, from netLink, raw json.RawMessage) error {
	var a taskAttempt
	if json.Unmarshal(raw, &a) != nil || !netRef.MatchString(a.ID) || a.Ordinal < 1 || len(a.Kind) > 32 ||
		(a.State != "finished" && a.State != "failed" && a.State != "blocked" && a.State != "cancelled") {
		return nil
	}
	if _, err := time.Parse(time.RFC3339Nano, a.FinishedAt); err != nil {
		return nil
	}
	if _, err := time.Parse(time.RFC3339Nano, a.LaunchedAt); err != nil {
		a.LaunchedAt = ""
	}
	if _, _, err := modelcatalog.SplitReported(a.Model); err != nil {
		a.Model = ""
	}
	var task string
	err := tx.QueryRow(`SELECT id FROM tasks WHERE id=? AND remote_broker=? AND remote_peer=?`, from.task, from.broker, from.peer).Scan(&task)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return saveTaskAttempt(tx, task, a)
}

func hasAnalyticsTable(ctx context.Context, q projectRouteQuerier, name string) (bool, error) {
	var exists bool
	err := q.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name=?)`, name).Scan(&exists)
	return exists, err
}
