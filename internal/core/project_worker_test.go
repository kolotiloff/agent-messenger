package core

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitWorkerCommitsReportAndDeliveryInItsProject(t *testing.T) {
	pair := newNetPair(t)
	if err := pair.b.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	origin := pair.startRemote(t, "old-wire-start", "claude", "Verify split worker completion")
	starts, err := pair.a.st.NetPending(testBroker, 10)
	if err != nil || len(starts) != 1 {
		t.Fatalf("legacy start: %+v %v", starts, err)
	}
	ctx := context.Background()
	files := &splitRunFixture{root: filepath.Join(t.TempDir(), "layout"), projects: map[string]*sql.DB{}}
	if err := StageProjectMigration(ctx, pair.b.path, files.root); err != nil {
		t.Fatal(err)
	}
	files.computer = files.open(t, migrationComputerFile)
	gameFile, siteFile := projectMigrationFile("Game"), projectMigrationFile("Site")
	game, site := files.open(t, gameFile), files.open(t, siteFile)
	files.projects[gameFile], files.projects[siteFile] = game, site
	m := NetIncoming{Seq: 1, From: HostPeerID, OpID: starts[0].OpID, Body: starts[0].Body}
	if err := applySplitRemoteStart(ctx, files.computer, files.project, testBroker, m); err != nil {
		t.Fatal(err)
	}
	var runID string
	if err := game.QueryRow("SELECT id FROM task_runs").Scan(&runID); err != nil {
		t.Fatal(err)
	}
	if wrong, err := openSplitWorkerStore(ctx, files.computer, files.root, "Site", runID); !errors.Is(err, ErrRejected) {
		if wrong != nil {
			wrong.Close()
		}
		t.Fatalf("another project opened the run: %v", err)
	}
	run, task, err := beginSplitLocalRun(ctx, files.computer, files.project, "Game", runID, "test-run-reservation")
	if err != nil || run.State != "launching" {
		t.Fatalf("register test launch: %q %v", run.State, err)
	}
	worker, err := openSplitWorkerStore(ctx, files.computer, files.root, "Game", run.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer worker.Close()
	// Exercise fetching the persisted identity on a fresh connection.
	worker.db.SetMaxIdleConns(0)
	if _, err := worker.ReadTask(run.ID, run.Receipt, false); !errors.Is(err, ErrRejected) {
		t.Fatalf("receipt opened before process observation: %v", err)
	}
	if err := worker.RecordTaskRunProcess(run.ID, taskObs); err != nil {
		t.Fatal(err)
	}
	if err := worker.BindTaskSession(run.ID, task.SessionID); err != nil {
		t.Fatal(err)
	}
	brief, err := worker.ReadTask(run.ID, run.Receipt, false)
	if err != nil || brief.Project != "Game" {
		t.Fatalf("worker brief: %+v %v", brief, err)
	}
	if _, err := worker.ReadTask(run.ID, "wrong-receipt", false); !errors.Is(err, ErrRejected) {
		t.Fatalf("wrong receipt was allowed: %v", err)
	}
	if err := worker.RecordDisposition(run.ID, run.Receipt, DispositionReport, "Fixture report"); err != nil {
		t.Fatal(err)
	}
	before, err := splitWindowChanges(ctx, files.computer, files.project, "Game", task.ID)
	if err != nil || before.Feed == "0" {
		t.Fatalf("worker revisions missing: %+v %v", before, err)
	}
	other, err := splitWindowChanges(ctx, files.computer, files.project, "Site", task.ID)
	if err != nil || other.Feed != "0" {
		t.Fatalf("another project received the worker's revision: %+v %v", other, err)
	}
	if _, err := game.Exec(`CREATE TRIGGER fail_report BEFORE INSERT ON net_outbox
 WHEN NEW.kind='event' BEGIN SELECT RAISE(ABORT,'fixture report outbox write failed'); END`); err != nil {
		t.Fatal(err)
	}
	evidence := cleanExit
	evidence.FinalText = "Fixture final output"
	evidence.Usage = &TaskUsage{InputTokens: 100, OutputTokens: 20}
	if _, err := worker.FinishTaskRun(run.ID, evidence); err == nil || !strings.Contains(err.Error(), "fixture report outbox write failed") {
		t.Fatalf("expected atomic report failure: %v", err)
	}
	var state string
	var frames, output int
	if err := game.QueryRow(`SELECT state,(SELECT COUNT(*) FROM net_outbox),(SELECT COUNT(*) FROM task_outputs)
 FROM tasks WHERE id=?`, task.ID).Scan(&state, &frames, &output); err != nil || state != "running" || frames != 1 || output != 0 {
		t.Fatalf("a partial completion escaped: %s %d %d %v", state, frames, output, err)
	}
	if change, err := splitWindowChanges(ctx, files.computer, files.project, "Game", task.ID); err != nil || change != before {
		t.Fatalf("a failed report notified the window: %+v %v", change, err)
	}
	if _, err := game.Exec("DROP TRIGGER fail_report"); err != nil {
		t.Fatal(err)
	}
	if final, err := worker.FinishTaskRun(run.ID, evidence); err != nil || final != "finished" {
		t.Fatalf("verified finish: %q %v", final, err)
	}
	if change, err := splitWindowChanges(ctx, files.computer, files.project, "Game", task.ID); err != nil || change.Feed == before.Feed || change.Window == before.Window {
		t.Fatalf("committed report did not notify the window: %+v %v", change, err)
	}
	if change, err := splitWindowChanges(ctx, files.computer, files.project, "Site", task.ID); err != nil || change != other {
		t.Fatalf("committed report notified the other project: %+v %v", change, err)
	}
	if _, err := worker.ReadTask(run.ID, run.Receipt, false); !errors.Is(err, ErrRejected) {
		t.Fatalf("closed receipt could still read work: %v", err)
	}
	if err := site.QueryRow("SELECT COUNT(*) FROM task_events").Scan(&frames); err != nil || frames != 0 {
		t.Fatalf("other project was changed: %d %v", frames, err)
	}
	var identity string
	if err := files.computer.QueryRow("SELECT value FROM meta WHERE key='computer_id'").Scan(&identity); err != nil {
		t.Fatal(err)
	}
	pending, err := splitNetPending(ctx, files.computer, files.project, testBroker, 10, nil)
	if err != nil || len(pending) != 4 {
		t.Fatalf("accepted, launch, output and report frames: %+v %v", pending, err)
	}
	for i, msg := range pending {
		frame := NetIncoming{Seq: int64(i + 1), From: "dev-b", OpID: msg.OpID, Body: msg.Body}
		body, valid := decodeNetIncoming(frame)
		if !valid || body.Task != origin || body.Computer != identity || (i == 0 && msg.file != migrationComputerFile) || (i > 0 && msg.file != gameFile) {
			t.Fatalf("misrouted frame %d: %+v", i, body)
		}
		if i == 1 && body.Event != taskLaunchObserved {
			t.Fatalf("missing first launch frame: %+v", body)
		}
		if err := pair.a.st.ApplyNet(testBroker, frame); err != nil {
			t.Fatal(err)
		}
		if err := splitNetSent(ctx, files.computer, files.project, msg); err != nil {
			t.Fatal(err)
		}
	}
	got, err := pair.a.st.TaskByID(origin)
	if err != nil || got.State != "done" {
		t.Fatalf("legacy origin received no completion: %+v %v", got, err)
	}
}
