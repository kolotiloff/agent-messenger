package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestUsageExportReadsOneProjectWithoutWaitingForWriters(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, time.November, 25, 12, 0, 0, 0, time.UTC)
	since := at.Add(-24 * time.Hour)
	stamp := at.Format(time.RFC3339Nano)
	const requested = "gpt-6.2-sol[effort=medium]"
	const reported = "gpt-6-luna[effort=high]"
	const bulk = 1500
	local := f.start(t, "export-local", "codex", "Local task")
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE tasks SET state='done',finished_at=?,updated_at=?,model=?,ran_model=?,usage_seen=1,usage_input=80,
 usage_cached=60,usage_cache_write=5,usage_output=10 WHERE id=?`, []any{stamp, stamp, requested, reported, local}},
		{`UPDATE task_runs SET launched_at=? WHERE task_id=?`, []any{at.Add(-4 * time.Minute).Format(time.RFC3339Nano), local}},
		{`INSERT INTO meta(key,value) VALUES ('net_broker','net-current')`, nil},
		{`WITH RECURSIVE ids(n) AS (SELECT 1 UNION ALL SELECT n+1 FROM ids WHERE n < ?)
 INSERT INTO tasks(id,project,op_id,computer,provider,cli_path,environment,profile,folder,title,text,state,created_at,updated_at,finished_at,model,remote_peer,remote_broker,remote_duration_ms)
 SELECT 'bulk-'||n,'Game','bulk-'||n,'dev-1','codex','','','','','Remote task','private body','done',?,?,?,?,'dev-1','net-current',1000
 FROM ids`, []any{bulk, stamp, stamp, stamp, requested}},
		{`INSERT INTO tasks(id,project,op_id,computer,provider,cli_path,environment,profile,folder,title,text,state,created_at,updated_at,finished_at,model)
 VALUES ('old','Game','old','local','codex','','','','','Old task','','done',?,?,?,?),
 ('site','Site','site','local','codex','','','','','Other project','','done',?,?,?,?),
 ('claude','Game','claude','local','claude','','','','','Other agent','','running',?,?,NULL,NULL)`,
			[]any{"2026-11-22T11:00:00Z", "2026-11-22T11:00:00Z", "2026-11-22T11:00:00Z", requested, stamp, stamp, stamp, requested, stamp, stamp}},
	} {
		if _, err := f.st.db.Exec(stmt.sql, stmt.args...); err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	layout := filepath.Join(t.TempDir(), "layout")
	if err := StageProjectMigration(ctx, f.path, layout); err != nil {
		t.Fatal(err)
	}
	split, err := openSplitStore(ctx, layout)
	if err != nil {
		t.Fatal(err)
	}
	defer split.Close()
	splitProject, err := split.split.open(projectMigrationFile("Game"))
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name    string
		store   *Store
		writers []*sql.DB
	}{
		{"legacy", f.st, []*sql.DB{f.st.db}},
		{"split", split, []*sql.DB{split.db, splitProject}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, db := range tc.writers {
				tx, err := db.Begin()
				if err != nil {
					t.Fatal(err)
				}
				defer tx.Rollback()
			}
			dir := t.TempDir()
			export := func(computer string, filter UsageFilter) (string, int, error) {
				// Each request must finish before the writer is released. Give
				// each request its own deadline, as the window API does.
				request, cancel := context.WithTimeout(ctx, 3*time.Second)
				defer cancel()
				return tc.store.WriteUsageExport(request, f.path, dir, since, at, computer, filter)
			}
			load := func(computer string, filter UsageFilter, want int) map[string]usageExportTask {
				t.Helper()
				start := time.Now()
				path, count, err := export(computer, filter)
				if err != nil || count != want {
					t.Fatalf("export count %d, want %d, error %v", count, want, err)
				}
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("exported %d tasks, %d bytes in %s", count, len(data), time.Since(start))
				if filepath.Ext(path) != ".json" {
					t.Fatalf("wrong extension: %s", path)
				}
				var payload struct {
					Metadata struct {
						Project  string   `json:"project"`
						Projects []string `json:"projects"`
						Since    string   `json:"since"`
					} `json:"metadata"`
					Tasks []usageExportTask `json:"tasks"`
				}
				if err := json.Unmarshal(data, &payload); err != nil {
					t.Fatal(err)
				}
				if len(payload.Tasks) != want || payload.Metadata.Project != filter.Project ||
					!slices.Equal(payload.Metadata.Projects, filter.Projects) || payload.Metadata.Since != since.Format(time.RFC3339) {
					t.Fatal("incorrect export metadata or task count")
				}
				rows := make(map[string]usageExportTask, len(payload.Tasks))
				for _, row := range payload.Tasks {
					rows[row.ID] = row
				}
				return rows
			}
			rows := load("", UsageFilter{Project: "Game", Agent: "codex"}, bulk+1)
			if len(rows) != bulk+1 || rows["site"].ID != "" || rows["old"].ID != "" || rows["claude"].ID != "" {
				t.Fatal("task leaked from another project, period or agent, or was duplicated")
			}
			row := rows[local]
			if row.Reported == nil || row.Reported.ID != "gpt-6-luna" || row.Reported.Effort != "high" ||
				row.Requested == nil || row.Requested.ID != "gpt-6.2-sol" || row.Requested.Effort != "medium" ||
				row.Usage == nil || row.Usage.NewInputTokens != 15 || row.Usage.CachedTokens != 60 || row.Usage.OutputTokens != 10 ||
				row.DurationMS == nil || *row.DurationMS != 240000 {
				t.Fatalf("bad actual model, usage or duration: %+v", row)
			}
			row = rows["bulk-1"]
			if row.Reported != nil || row.Requested == nil || row.Usage != nil || row.DurationMS == nil || *row.DurationMS != 1000 {
				t.Fatalf("bad unconfirmed model or missing usage: %+v", row)
			}
			load("dev-1", UsageFilter{Project: "Game", Model: requested}, bulk)
			load("local", UsageFilter{Project: "Game", Model: reported}, 1)
			load("local", UsageFilter{Project: "Game", Model: requested}, 0)
			load("", UsageFilter{Project: "Site"}, 1)
			selected := load("local", UsageFilter{Projects: []string{"Game", "Site"}, Agent: "codex"}, 2)
			if selected[local].Project != "Game" || selected["site"].Project != "Site" {
				t.Fatalf("project attribution missing from export: %+v", selected)
			}
			if _, _, err := export("", UsageFilter{Project: "Missing"}); err == nil || errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("expected a prompt rejection for an unknown project, got %v", err)
			}
			if _, _, err := export("", UsageFilter{Projects: []string{"Game", "Missing"}}); err == nil {
				t.Fatal("published an incomplete export")
			}
			if files, err := os.ReadDir(dir); err != nil || len(files) != 6 {
				t.Fatalf("export should only publish a file for each successful request: %v, %v", files, err)
			}
		})
	}
}
