package core

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitRemoteRequestKeepsItsProjectAndReplaysOnce(t *testing.T) {
	ctx := context.Background()
	f := newSplitRunFixture(t)
	game := f.projects[projectMigrationFile("Game")]
	site := f.projects[projectMigrationFile("Site")]
	shared, err := openSplitStore(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { shared.Close() }()

	send := func(seq int64, peer, op string, body netBody) error {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		return shared.ApplyNet(testBroker, NetIncoming{Seq: seq, From: peer, OpID: op, Body: raw})
	}

	if err := send(1, "dev-a", "start-game", netBody{
		Task: "origin-game", Kind: "start", Project: "Game", Agent: "claude", Text: "Game assignment",
	}); err != nil {
		t.Fatal(err)
	}
	if err := send(2, "dev-a", "start-site", netBody{
		Task: "origin-site", Kind: "start", Project: "Site", Agent: "claude", Text: "Site assignment",
	}); err != nil {
		t.Fatal(err)
	}
	var task string
	if err := game.QueryRow("SELECT id FROM tasks").Scan(&task); err != nil {
		t.Fatal(err)
	}

	note := netBody{Task: "origin-game", Kind: "note", Text: "Use the revised requirements", OpID: "note-one"}
	if err := send(3, "dev-b", "wrong-sender", note); !errors.Is(err, ErrRejected) {
		t.Fatalf("an unrelated computer could write to the task: %v", err)
	}
	if cursor := cursorForFixture(t, f.computer); cursor != 2 {
		t.Fatalf("unrouted request advanced cursor to %d", cursor)
	}
	if _, err := f.computer.Exec(`CREATE TRIGGER fail_note_cursor BEFORE UPDATE ON meta
 WHEN NEW.key LIKE 'net_cursor:%' BEGIN SELECT RAISE(ABORT,'fixture cursor failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := send(3, "dev-a", "note-game", note); err == nil || !strings.Contains(err.Error(), "fixture cursor failure") {
		t.Fatalf("expected cursor failure: %v", err)
	}
	if err := f.computer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := game.Close(); err != nil {
		t.Fatal(err)
	}
	if err := shared.Close(); err != nil {
		t.Fatal(err)
	}
	shared, err = openSplitStore(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	f.computer = f.open(t, migrationComputerFile)
	game = f.open(t, projectMigrationFile("Game"))
	f.projects[projectMigrationFile("Game")] = game
	if _, err := f.computer.Exec("DROP TRIGGER fail_note_cursor"); err != nil {
		t.Fatal(err)
	}
	if err := send(3, "dev-a", "note-game", note); err != nil {
		t.Fatal(err)
	}
	var notes int
	if err := game.QueryRow("SELECT COUNT(*) FROM task_owner_notes WHERE state='pending'").Scan(&notes); err != nil || notes != 1 {
		t.Fatalf("durable note count: %d %v", notes, err)
	}

	first := f.runID(t, "Game", task)
	if ready, err := shared.NextTaskRunIntents(10); err != nil || len(ready) != 2 || ready[0] != first {
		t.Fatalf("both projects should be able to dispatch: %v %v", ready, err)
	}
	run, assigned, err := shared.BeginTaskRun(first, "fixture-first-run")
	if err != nil {
		t.Fatal(err)
	}
	path, err := shared.WorkerTaskDatabase("queue.sqlite", task)
	if err != nil || path != filepath.Join(f.root, projectMigrationFile("Game")) {
		t.Fatalf("worker selected the wrong database: %q %v", path, err)
	}
	store, err := OpenTaskRunStore(ctx, path, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := shared.RecordTaskRunProcess(run.ID, taskObs); err != nil {
		t.Fatal(err)
	}
	if err := shared.BindTaskSession(run.ID, assigned.SessionID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadTask(run.ID, run.Receipt, false); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordDisposition(run.ID, run.Receipt, DispositionAsk, "Which plan?"); err != nil {
		t.Fatal(err)
	}
	if _, err := shared.FinishTaskRun(run.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	var question string
	if err := game.QueryRow("SELECT id FROM task_questions WHERE state='open'").Scan(&question); err != nil {
		t.Fatal(err)
	}
	if err := send(4, "dev-a", "answer-game", netBody{
		Task: "origin-game", Kind: "answer", Question: question, By: AudienceCoordinator, Text: "Use plan A",
	}); err != nil {
		t.Fatal(err)
	}
	var answerRun string
	if err := game.QueryRow("SELECT id FROM task_runs WHERE kind='answer' AND state='intent'").Scan(&answerRun); err != nil {
		t.Fatal(err)
	}
	answered, _, err := shared.BeginTaskRun(answerRun, "fixture-answer-run")
	if err != nil || answered.Kind != "answer" {
		t.Fatalf("remote answer did not get a launch intent: %+v %v", answered, err)
	}
	if err := shared.RecordTaskRunProcess(answerRun, taskObs); err != nil {
		t.Fatal(err)
	}
	if err := shared.BindTaskSession(answerRun, assigned.SessionID); err != nil {
		t.Fatal(err)
	}
	brief, err := store.ReadTask(answerRun, answered.Receipt, false)
	if err != nil || brief.Answer != "Use plan A" || strings.Count(brief.OwnerNotes, note.Text) != 1 {
		t.Fatalf("resumed worker did not get its answer and exactly one note: %+v %v", brief, err)
	}
	if err := store.RecordDisposition(answerRun, answered.Receipt, DispositionReport, "Plan finished"); err != nil {
		t.Fatal(err)
	}
	if _, err := shared.FinishTaskRun(answerRun, cleanExit); err != nil {
		t.Fatal(err)
	}

	if err := shared.SetAgentPaused("claude", true); err != nil {
		t.Fatal(err)
	}
	follow := netBody{Task: "origin-game", Kind: "continue", OpID: "follow-up", Text: "Repeat verification"}
	if err := send(5, "dev-a", "continue-while-paused", follow); err != nil {
		t.Fatal(err)
	}
	var refusals int
	if err := game.QueryRow("SELECT COUNT(*) FROM net_outbox WHERE kind='continue_refused'").Scan(&refusals); err != nil || refusals != 1 {
		t.Fatalf("paused machine policy was not applied: %d %v", refusals, err)
	}
	if err := shared.SetAgentPaused("claude", false); err != nil {
		t.Fatal(err)
	}
	if err := send(6, "dev-a", "continue-game", follow); err != nil {
		t.Fatal(err)
	}
	if err := send(7, "dev-a", "cancel-game", netBody{Task: "origin-game", Kind: "cancel"}); err != nil {
		t.Fatal(err)
	}
	var state string
	if err := game.QueryRow("SELECT state FROM task_runs WHERE kind='continue'").Scan(&state); err != nil || state != "cancelled" {
		t.Fatalf("queued continuation was not cancelled: %q %v", state, err)
	}
	if err := site.QueryRow("SELECT state FROM tasks").Scan(&state); err != nil || state != "running" {
		t.Fatalf("unrelated project was modified: %q %v", state, err)
	}
	if cursor := cursorForFixture(t, f.computer); cursor != 7 {
		t.Fatalf("cursor after replay and follow-ups = %d", cursor)
	}
	siteTasks, err := shared.Tasks("Site", 10)
	if err != nil || len(siteTasks) != 1 || siteTasks[0].Text != "Site assignment" {
		t.Fatalf("Site task list: %+v %v", siteTasks, err)
	}
	feed, err := shared.TaskFeed(task)
	if err != nil || feed.Task.Remote == nil || !feed.Task.Remote.From || feed.Task.State != "cancelled" {
		t.Fatalf("task feed did not use machine and project data: %+v %v", feed.Task, err)
	}
	pending, err := shared.NetPending(testBroker, 100)
	if err != nil {
		t.Fatal(err)
	}
	report := false
	for _, message := range pending {
		if body, ok := decodeNetIncoming(NetIncoming{Body: message.Body}); ok && body.Event == "report" {
			report = true
			if body.Task != "origin-game" || message.file != projectMigrationFile("Game") {
				t.Fatalf("incorrect report route: %+v", body)
			}
		}
		if err := shared.NetDeliverySent(message); err != nil {
			t.Fatal(err)
		}
	}
	if left, err := shared.NetPending(testBroker, 100); err != nil || len(left) != 0 || !report {
		t.Fatalf("delivery acknowledgement: %d, report %t, %v", len(left), report, err)
	}
}
