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
	ID                string       `json:"task_id"`
	OriginID          string       `json:"origin_task_id,omitempty"`
	Title             string       `json:"title"`
	Agent             string       `json:"agent"`
	Computer          string       `json:"computer_id"`
	State             string       `json:"state"`
	Created           string       `json:"created_at"`
	Updated           string       `json:"updated_at"`
	Finished          string       `json:"finished_at,omitempty"`
	DurationMS        *int64       `json:"duration_ms"`
	DurationEstimated bool         `json:"duration_estimated,omitempty"`
	Requested         *exportModel `json:"requested_model"`
	Reported          *exportModel `json:"last_reported_model"`
	Usage             *exportUsage `json:"task_usage"`
}

// WriteUsageExport saves the task records known to this computer for one
// project. Its independent read connection can see committed tasks while the
// client writes, and does not queue behind the store's single connection.
func (s *Store) WriteUsageExport(ctx context.Context, queue, dir string, since, at time.Time, computer string, filter UsageFilter) (string, int, error) {
	if !ValidProjectName(filter.Project) {
		return "", 0, fmt.Errorf("invalid project")
	}
	commonPath := queue
	if s.split != nil {
		commonPath = filepath.Join(s.split.root, migrationComputerFile)
	}
	common, err := openReadOnlyDatabase(commonPath)
	if err != nil {
		return "", 0, err
	}
	defer common.Close()

	var broker string
	if err := common.QueryRowContext(ctx, "SELECT COALESCE((SELECT value FROM meta WHERE key='net_broker'),'')").Scan(&broker); err != nil {
		return "", 0, err
	}
	db := common
	if s.split != nil {
		file, err := splitTaskProjectFile(ctx, common, filter.Project)
		if err != nil {
			return "", 0, err
		}
		db, err = openReadOnlyDatabase(filepath.Join(s.split.root, file))
		if err != nil {
			return "", 0, err
		}
		defer db.Close()
	} else {
		var exists bool
		if err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM projects WHERE name=?)", filter.Project).Scan(&exists); err != nil {
			return "", 0, err
		}
		if !exists {
			return "", 0, fmt.Errorf("unknown project")
		}
	}
	return saveUsageExport(ctx, db, dir, since, at, computer, broker, filter)
}

func saveUsageExport(ctx context.Context, db *sql.DB, dir string, since, at time.Time, computer, broker string, filter UsageFilter) (string, int, error) {
	file, err := os.CreateTemp(dir, ".messenger-statistics-"+at.Format("20060102-150405")+"-*.tmp")
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	out := bufio.NewWriter(file)
	encoder := json.NewEncoder(out)
	header := map[string]any{
		"schema_version": 1,
		"project":        filter.Project,
		"since":          since.Format(time.RFC3339),
		"exported_at":    at.Format(time.RFC3339),
		"time_basis":     "finished_at_or_updated_at",
		"computer":       computer,
		"agent":          filter.Agent,
		"model":          filter.Model,
		"model_filter":   "last_reported_else_requested",
		"coverage":       "task_records_known_to_this_computer",
		"usage_scope":    "task_total_not_allocated_by_run",
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
	n, err := encodeUsageExportTasks(ctx, db, out, encoder, since, computer, broker, filter)
	if err != nil {
		return "", 0, err
	}
	if _, err := out.WriteString("]}\n"); err != nil {
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

func encodeUsageExportTasks(ctx context.Context, db *sql.DB, out *bufio.Writer, encoder *json.Encoder, since time.Time, computer, broker string, filter UsageFilter) (int, error) {
	const query = `SELECT t.id,COALESCE(t.origin_task,''),t.title,t.provider,t.computer,t.state,t.created_at,t.updated_at,
 COALESCE(t.finished_at,''),COALESCE(t.model,''),COALESCE(t.ran_model,''),t.usage_seen,t.usage_input,t.usage_cached,
 t.usage_output,t.usage_cache_write,t.usage_cost,COALESCE(t.remote_peer,''),COALESCE(t.remote_broker,''),
 COALESCE((SELECT MIN(r.launched_at) FROM task_runs r WHERE r.task_id=t.id),''),
 t.remote_duration_ms,
 CASE WHEN t.remote_peer IS NOT NULL AND t.remote_duration_ms IS NULL AND t.state IN ('done','failed')
 THEN COALESCE((SELECT i.created_at FROM remote_task_items i WHERE i.task_id=t.id ORDER BY i.seq LIMIT 1),'') ELSE '' END
 FROM tasks t WHERE t.project=? AND COALESCE(t.finished_at,t.updated_at)>=?
 AND (?='' OR t.provider=?)
 AND (?='' OR COALESCE(NULLIF(t.ran_model,''),NULLIF(t.model,''),'')=?)
 AND (?='' OR (?='local' AND COALESCE(t.remote_peer,'')='') OR (t.remote_peer=? AND t.remote_broker=? AND ?<>''))`
	rows, err := db.QueryContext(ctx, query, filter.Project, since.UTC().Format(time.RFC3339Nano),
		filter.Agent, filter.Agent, filter.Model, filter.Model, computer, computer, computer, broker, broker)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var task usageExportTask
		var usage TaskUsage
		var requested, reported, peer, peerBroker, started, observed string
		var seen bool
		var remoteDuration sql.NullInt64
		var cost sql.NullFloat64
		if err := rows.Scan(&task.ID, &task.OriginID, &task.Title, &task.Agent, &task.Computer, &task.State,
			&task.Created, &task.Updated, &task.Finished, &requested, &reported, &seen,
			&usage.InputTokens, &usage.CachedTokens, &usage.OutputTokens, &usage.CacheWriteTokens, &cost,
			&peer, &peerBroker, &started, &remoteDuration, &observed); err != nil {
			return 0, err
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
		if n != 0 {
			if err := out.WriteByte(','); err != nil {
				return 0, err
			}
		}
		if err := encoder.Encode(task); err != nil {
			return 0, err
		}
		n++
	}
	return n, rows.Err()
}
