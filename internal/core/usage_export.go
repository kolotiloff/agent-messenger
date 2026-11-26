package core

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/modelcatalog"
)

type exportModel struct {
	ID     string `json:"id"`
	Effort string `json:"effort,omitempty"`
}

type exportUsage struct {
	TaskUsage
	NewInputTokens int64 `json:"new_input_tokens"`
}

func usageExportModel(selection string) *exportModel {
	if selection == "" {
		return nil
	}
	id, effort, err := modelcatalog.SplitReported(selection)
	if err != nil {
		return &exportModel{ID: selection}
	}
	return &exportModel{ID: id, Effort: effort}
}

type usageExportTask struct {
	ID                string          `json:"task_id"`
	Project           string          `json:"project,omitempty"`
	OriginID          string          `json:"origin_task_id,omitempty"`
	Title             string          `json:"title"`
	Agent             string          `json:"agent"`
	Computer          string          `json:"computer_id"`
	State             string          `json:"state"`
	Created           string          `json:"created_at"`
	Updated           string          `json:"updated_at"`
	Finished          string          `json:"finished_at,omitempty"`
	DurationMS        *int64          `json:"duration_ms"`
	DurationEstimated bool            `json:"duration_estimated,omitempty"`
	Requested         *exportModel    `json:"requested_model"`
	Reported          *exportModel    `json:"last_reported_model"`
	Usage             *exportUsage    `json:"task_usage"`
	Assignment        *TaskAssignment `json:"assignment,omitempty"`
	Reviews           []TaskReview    `json:"reviews,omitempty"`
	Attempts          []taskAttempt   `json:"attempts,omitempty"`
	local             bool
}

// WriteUsageExport saves the task records known to this computer for the
// selected projects. Its independent read connection sees committed tasks while the
// client writes, and does not queue behind the store's single connection.
func (s *Store) WriteUsageExport(ctx context.Context, queue, dir string, since, at time.Time, computer string, filter UsageFilter) (string, int, error) {
	if !ValidUsageProjects(filter.Project, filter.Projects, false) {
		return "", 0, fmt.Errorf("invalid project scope")
	}
	return s.saveUsageExport(ctx, queue, dir, since, at, computer, filter)
}

func (s *Store) readUsageProject(ctx context.Context, queue, project string, read func(*sql.Tx, string) error) error {
	if !ValidProjectName(project) {
		return fmt.Errorf("invalid project")
	}
	commonPath := queue
	if s.split != nil {
		commonPath = filepath.Join(s.split.root, migrationComputerFile)
	}
	common, err := openReadOnlyDatabase(commonPath)
	if err != nil {
		return err
	}
	defer common.Close()

	var broker string
	if err := common.QueryRowContext(ctx, "SELECT COALESCE((SELECT value FROM meta WHERE key='net_broker'),'')").Scan(&broker); err != nil {
		return err
	}
	db := common
	if s.split != nil {
		file, err := splitTaskProjectFile(ctx, common, project)
		if err != nil {
			return err
		}
		db, err = openReadOnlyDatabase(filepath.Join(s.split.root, file))
		if err != nil {
			return err
		}
		defer db.Close()
	} else {
		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM projects WHERE name=?)", project).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return fmt.Errorf("unknown project")
		}
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return read(tx, broker)
}

func (s *Store) saveUsageExport(ctx context.Context, queue, dir string, since, at time.Time, computer string, filter UsageFilter) (string, int, error) {
	file, err := os.CreateTemp(dir, ".messenger-statistics-"+at.Format("20060102-150405")+"-*.tmp")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	out := bufio.NewWriter(file)
	encoder := json.NewEncoder(out)
	header := map[string]any{
		"schema_version":          1,
		"project":                 filter.Project,
		"since":                   since.Format(time.RFC3339),
		"exported_at":             at.Format(time.RFC3339),
		"time_basis":              "finished_at_or_updated_at",
		"computer":                computer,
		"agent":                   filter.Agent,
		"model":                   filter.Model,
		"model_filter":            "last_reported_else_requested",
		"coverage":                "task_records_known_to_this_computer",
		"usage_scope":             "task_total_and_optional_observed_attempts",
		"efficiency_time_basis":   "original_task_created_at",
		"efficiency_model_filter": "initial_reported_else_requested",
	}
	if len(filter.Projects) != 0 {
		delete(header, "project")
		header["projects"] = filter.Projects
	}
	if _, err := out.WriteString("{\"metadata\":"); err != nil {
		return "", 0, err
	}
	if err := encoder.Encode(header); err != nil {
		return "", 0, err
	}
	if _, err := out.WriteString(",\"tasks\":[\n"); err != nil {
		return "", 0, err
	}
	n := 0
	groups := []modelEfficiency{}
	for _, project := range filter.projectNames() {
		scope := UsageFilter{Project: project, Agent: filter.Agent, Model: filter.Model}
		err := s.readUsageProject(ctx, queue, project, func(db *sql.Tx, broker string) error {
			err := visitUsageExportTasks(ctx, db, since, computer, broker, scope, false, func(task usageExportTask) error {
				if len(filter.Projects) != 0 {
					task.Project = project
				}
				if n != 0 {
					if err := out.WriteByte(','); err != nil {
						return err
					}
				}
				n++
				return encoder.Encode(task)
			})
			if err != nil {
				return err
			}
			projectGroups, err := scopedModelEfficiency(ctx, db, since, computer, broker, scope, len(filter.Projects) != 0)
			groups = append(groups, projectGroups...)
			return err
		})
		if err != nil {
			return "", 0, err
		}
	}
	if _, err := out.WriteString("],\"model_efficiency\":"); err != nil {
		return "", 0, err
	}
	if err := encoder.Encode(groups); err != nil {
		return "", 0, err
	}
	if _, err := out.WriteString("}\n"); err != nil {
		return "", 0, err
	}
	if err := out.Flush(); err != nil {
		return "", 0, err
	}
	if err := file.Close(); err != nil {
		return "", 0, err
	}
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	name := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file.Name()), "."), ".tmp") + ".json"
	path := filepath.Join(dir, name)
	if err := os.Rename(file.Name(), path); err != nil {
		return "", 0, err
	}
	return path, n, nil
}

// An untouched project file from before the upgrade still exports normally.
func visitUsageExportTasks(ctx context.Context, db *sql.Tx, since time.Time, computer, broker string, filter UsageFilter, cohort bool, visit func(usageExportTask) error) error {
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_schema WHERE type='table' AND name IN ('task_assignments','task_reviews','task_attempts')`).Scan(&count); err != nil {
		return err
	}
	analytics := `,NULL,NULL,NULL`
	if count == 3 {
		analytics = `,
 (SELECT json_object('task_type',task_type,'difficulty',difficulty,'retry_of',retry_of,'original_task_id',original_task_id)
  FROM task_assignments WHERE task_id=t.id),
 (SELECT json_group_array(json_object('result',result,'reason',reason,'report_at',report_at,'reviewed_at',reviewed_at))
  FROM (SELECT * FROM task_reviews WHERE task_id=t.id ORDER BY reviewed_at,rowid)),
 (SELECT json_group_array(json_object('run_id',run_id,'ordinal',ordinal,'kind',kind,'state',state,'launched_at',launched_at,
  'finished_at',finished_at,'reported_model',reported_model,'outcome_at',outcome_at,'usage',CASE WHEN json_valid(usage) THEN json(usage) END))
  FROM (SELECT * FROM task_attempts WHERE task_id=t.id AND finished_at<>'' ORDER BY ordinal,run_id))`
	}
	query := `SELECT t.id,COALESCE(t.origin_task,''),t.title,t.provider,t.computer,t.state,t.created_at,t.updated_at,
 COALESCE(t.finished_at,''),COALESCE(t.model,''),COALESCE(t.ran_model,''),t.usage_seen,t.usage_input,t.usage_cached,
 t.usage_output,t.usage_cache_write,t.usage_cost,COALESCE(t.remote_peer,''),COALESCE(t.remote_broker,''),
 COALESCE((SELECT MIN(r.launched_at) FROM task_runs r WHERE r.task_id=t.id),''),
 t.remote_duration_ms,
 CASE WHEN t.remote_peer IS NOT NULL AND t.remote_duration_ms IS NULL AND t.state IN ('done','failed')
 THEN COALESCE((SELECT i.created_at FROM remote_task_items i WHERE i.task_id=t.id ORDER BY i.seq LIMIT 1),'') ELSE '' END` + analytics + ` FROM tasks t WHERE t.project=?`
	args := []any{filter.Project, since.UTC().Format(time.RFC3339Nano)}
	if cohort {
		// Include each retry, even if it switched model, agent, or computer.
		query += ` AND t.created_at>=? AND t.origin_task IS NULL`
	} else {
		query += ` AND COALESCE(t.finished_at,t.updated_at)>=?
 AND (?='' OR t.provider=?)
 AND (?='' OR COALESCE(NULLIF(t.ran_model,''),NULLIF(t.model,''),'')=?)
 AND (?='' OR (?='local' AND COALESCE(t.remote_peer,'')='') OR (t.remote_peer=? AND t.remote_broker=? AND ?<>''))`
		args = append(args, filter.Agent, filter.Agent, filter.Model, filter.Model, computer, computer, computer, broker, broker)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var task usageExportTask
		var usage TaskUsage
		var requested, reported, peer, peerBroker, started, observed string
		var seen bool
		var remoteDuration sql.NullInt64
		var cost sql.NullFloat64
		var assignment, reviews, attempts []byte
		if err := rows.Scan(&task.ID, &task.OriginID, &task.Title, &task.Agent, &task.Computer, &task.State,
			&task.Created, &task.Updated, &task.Finished, &requested, &reported, &seen,
			&usage.InputTokens, &usage.CachedTokens, &usage.OutputTokens, &usage.CacheWriteTokens, &cost,
			&peer, &peerBroker, &started, &remoteDuration, &observed, &assignment, &reviews, &attempts); err != nil {
			return err
		}
		task.local = peer == ""
		for _, field := range []struct {
			raw   []byte
			value any
		}{{assignment, &task.Assignment}, {reviews, &task.Reviews}, {attempts, &task.Attempts}} {
			if len(field.raw) != 0 {
				if err := json.Unmarshal(field.raw, field.value); err != nil {
					return err
				}
			}
		}
		for i := range task.Attempts {
			a := &task.Attempts[i]
			a.ModelInfo = usageExportModel(a.Model)
			if a.State != "blocked" {
				a.DurationMS = taskDurationMS(a.LaunchedAt, a.FinishedAt)
			}
		}
		task.Requested, task.Reported = usageExportModel(requested), usageExportModel(reported)
		if seen {
			if cost.Valid {
				usage.CostUSD = &cost.Float64
			}
			task.Usage = &exportUsage{TaskUsage: usage, NewInputTokens: usage.InputTokens - usage.CachedTokens - usage.CacheWriteTokens}
		}
		if peer != "" && (broker == "" || peerBroker != broker) {
			task.Computer = "old:" + peerBroker + "/" + peer
		}
		if task.State == "done" || task.State == "failed" || task.State == "cancelled" {
			task.DurationMS, task.DurationEstimated = usageTaskDuration(peer == "", task.Finished, started, observed, remoteDuration)
		}
		if err := visit(task); err != nil {
			return err
		}
	}
	return rows.Err()
}
