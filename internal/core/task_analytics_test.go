package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestModelEfficiencyIncludesRemoteReplacementAndDeduplicatesAttempt(t *testing.T) {
	p := newNetPair(t)
	req := StartTaskRequest{Project: "Game", OpID: "original", Provider: "codex", Text: "Fix the bug",
		Assignment: TaskAssignment{Type: "bug_fix", Difficulty: "simple"}}
	root, _, err := p.a.st.StartTask(req)
	if err != nil {
		t.Fatal(err)
	}
	if again, created, err := p.a.st.StartTask(req); err != nil || created || again != root {
		t.Fatalf("replayed assignment: %s %v %v", again, created, err)
	}
	req.Assignment.Difficulty = "complex"
	if _, _, err := p.a.st.StartTask(req); !errors.Is(err, ErrRejected) {
		t.Fatalf("changed assignment accepted for one operation id: %v", err)
	}
	if _, err := p.a.st.ReviewTask("Game", root, "early", "accepted", "", ""); !errors.Is(err, ErrRejected) {
		t.Fatalf("accepted a running task: %v", err)
	}
	const luna = "gpt-6-luna[effort=high]"
	const sol = "gpt-6.2-sol[effort=medium]"
	finish := func(f taskFixture, task, model string, usage TaskUsage) {
		t.Helper()
		r := f.launch(t, task, "019a0000-0000-7000-8000-0000000000a1")
		if err := f.st.SetTaskRunModel(r.ID, model); err != nil {
			t.Fatal(err)
		}
		f.dispose(t, r, DispositionReport, "Result")
		ev := cleanExit
		ev.Usage = &usage
		if _, err := f.st.FinishTaskRun(r.ID, ev); err != nil {
			t.Fatal(err)
		}
	}
	finish(p.a, root, luna, TaskUsage{InputTokens: 80000, CachedTokens: 50000, CacheWriteTokens: 5000, OutputTokens: 25000})
	if _, err := p.a.st.ReviewTask("Game", root, "stale", "accepted", "", "2026-01-01T00:00:00Z"); !errors.Is(err, ErrRejected) {
		t.Fatalf("stale outcome accepted: %v", err)
	}
	review, err := p.a.st.ReviewTask("Game", root, "review-1", "returned", "implementation_error", "")
	if err != nil {
		t.Fatal(err)
	}
	if again, err := p.a.st.ReviewTask("Game", root, "review-1", "returned", "implementation_error", ""); err != nil || again != review {
		t.Fatalf("review replay changed: %+v %v", again, err)
	}
	if _, err := p.a.st.ReviewTask("Game", root, "conflict", "accepted", "", ""); !errors.Is(err, ErrRejected) {
		t.Fatalf("second review accepted: %v", err)
	}
	retry := StartTaskRequest{Project: "Game", OpID: "replacement", Computer: "dev-b", Provider: "codex", Text: "Repair the result",
		Assignment: TaskAssignment{RetryOf: root}}
	wrong := retry
	wrong.Project = "Site"
	if err := p.a.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := p.a.st.StartTask(wrong); !errors.Is(err, ErrRejected) {
		t.Fatalf("cross-project replacement accepted: %v", err)
	}
	if _, err := p.a.st.ReviewTask("Site", root, "cross-project", "accepted", "", ""); !errors.Is(err, ErrRejected) {
		t.Fatalf("cross-project review accepted: %v", err)
	}
	replacement, created, err := p.a.st.StartTask(retry)
	if err != nil || !created {
		t.Fatalf("create replacement: %s %v %v", replacement, created, err)
	}
	p.carry(t, true)
	mirror := p.mirror(t, replacement)
	if mirror.Text != retry.Text {
		t.Fatalf("metadata reached worker instructions: %q", mirror.Text)
	}
	if err := p.b.st.tx(func(tx *sql.Tx) error {
		a, err := readTaskAssignment(tx, mirror.ID)
		if err != nil {
			return err
		}
		if a.OriginalID != root || a.RetryOf != root {
			t.Fatalf("missing remote lineage: %+v", a)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	finish(p.b, mirror.ID, sol, TaskUsage{InputTokens: 165000, CachedTokens: 13000, OutputTokens: 50000})
	ctx := context.Background()
	pendingLayout := filepath.Join(t.TempDir(), "pending-layout")
	if err := StageProjectMigration(ctx, p.b.path, pendingLayout); err != nil {
		t.Fatal(err)
	}
	pending, err := openReadOnlyDatabase(filepath.Join(pendingLayout, projectMigrationFile("Game")))
	if err != nil {
		t.Fatal(err)
	}
	var attempts int
	err = pending.QueryRow("SELECT COUNT(*) FROM net_outbox WHERE kind='run_usage'").Scan(&attempts)
	pending.Close()
	if err != nil || attempts != 1 {
		t.Fatalf("attempt delivery did not move with its project: %d %v", attempts, err)
	}
	for _, frame := range p.carry(t, false) {
		body, _ := decodeNetIncoming(frame)
		if body.Kind == "run_usage" {
			p.seq++
			frame.Seq, frame.OpID = p.seq, frame.OpID+".duplicate"
			if err := p.a.st.ApplyNet(testBroker, frame); err != nil {
				t.Fatal(err)
			}
		}
	}
	if _, err := p.a.st.ReviewTask("Game", replacement, "accept", "accepted", "", ""); err != nil {
		t.Fatal(err)
	}
	since := time.Now().Add(-time.Hour)
	filter := UsageFilter{Project: "Game", Model: luna}
	check := func(store *Store) {
		t.Helper()
		groups, err := store.UsageEfficiency(ctx, p.a.path, since, "local", filter)
		if err != nil || len(groups) != 1 {
			t.Fatalf("model groups: %+v %v", groups, err)
		}
		g := groups[0]
		if g.Model != luna || g.Type != "bug_fix" || g.Difficulty != "simple" || g.Tasks != 1 || g.Accepted != 1 ||
			g.FirstPassKnown != 1 || g.FirstPassAccepted != 0 || g.ReturnReasons["implementation_error"] != 1 ||
			g.AcceptedTokens != (distribution{Samples: 1, Mean: 252000, Median: 252000, P90: 252000}) ||
			g.AcceptedUsage.CacheRead != 63000 || g.AcceptedUsage.CacheWrite != 5000 || g.TimeToAcceptanceMS.Samples != 1 || g.AcceptedRuntimeMS.Samples != 1 {
			t.Fatalf("incorrect cost through acceptance or cohort: %+v", g)
		}
	}
	check(p.a.st)
	layout := filepath.Join(t.TempDir(), "layout")
	if err := StageProjectMigration(ctx, p.a.path, layout); err != nil {
		t.Fatal(err)
	}
	st, err := openSplitStore(ctx, layout)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	check(st)
}

func TestAnalyticsOldDatabaseAndMissingTelemetryRemainUsable(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "old-task", "codex", "Legacy task")
	if _, err := f.st.db.Exec(`UPDATE tasks SET state='done',finished_at=?,updated_at=?,usage_seen=1,usage_input=9 WHERE id=?;
 DROP TABLE task_reviews; DROP TABLE task_assignments; DROP TABLE task_attempts;`, now(), now(), id); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	at := time.Now()
	old := UsageFilter{Project: "Game"}
	path, n, err := f.st.WriteUsageExport(ctx, f.path, t.TempDir(), at.Add(-time.Hour), at, "", old)
	if err != nil || n != 1 {
		t.Fatalf("legacy export: %d %v", n, err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var export struct {
		Tasks  []usageExportTask `json:"tasks"`
		Groups []modelEfficiency `json:"model_efficiency"`
	}
	if err := json.Unmarshal(data, &export); err != nil || len(export.Tasks) != 1 || len(export.Groups) != 1 {
		t.Fatalf("legacy export fields: %+v %v", export, err)
	}
	task, group := export.Tasks[0], export.Groups[0]
	if task.Assignment != nil || len(task.Reviews) != 0 || len(task.Attempts) != 0 || task.Usage.InputTokens != 9 ||
		group.Unreviewed != 1 || group.AcceptedTokens.Samples != 0 || group.InitialModel != nil {
		t.Fatalf("missing metadata was treated as known: %+v %+v", task, group)
	}
	layout := filepath.Join(t.TempDir(), "pre-upgrade-layout")
	if err := StageProjectMigration(ctx, f.path, layout); err != nil {
		t.Fatal(err)
	}
	split, err := openSplitStore(ctx, layout)
	if err != nil {
		t.Fatal(err)
	}
	defer split.Close()
	if _, err := split.ReviewTask("Game", id, "after-upgrade", "accepted", "", ""); err != nil {
		t.Fatal(err)
	}
	groups, err := split.UsageEfficiency(ctx, f.path, at.Add(-time.Hour), "", old)
	if err != nil || len(groups) != 1 || groups[0].Accepted != 1 || groups[0].FirstPassKnown != 0 || groups[0].AcceptedTokens.Samples != 0 {
		t.Fatalf("old DB upgraded and reviewed with no attributed cost: %+v %v", groups, err)
	}
}
