package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestSplitRemoteStartRecoversBeforeAcknowledging(t *testing.T) {
	f := newSplitRunFixture(t)
	ctx := context.Background()
	deliver := func(seq int64, op, remoteID, project string) error {
		t.Helper()
		body, err := json.Marshal(netBody{Kind: "start", Task: remoteID, Project: project, Agent: "codex", Text: "Work on " + project, Computer: "origin-machine"})
		if err != nil {
			t.Fatal(err)
		}
		return applySplitRemoteStart(ctx, f.computer, f.project, testBroker, NetIncoming{Seq: seq, From: "dev-a", OpID: op, Body: body})
	}
	assertCount := func(db *sql.DB, query string, want int) {
		t.Helper()
		var got int
		if err := db.QueryRow(query).Scan(&got); err != nil || got != want {
			t.Fatalf("%s: got %d want %d: %v", query, got, want, err)
		}
	}
	if _, err := f.computer.Exec(`CREATE TRIGGER fail_start_cursor BEFORE INSERT ON meta
 WHEN NEW.key LIKE 'net_cursor:%' BEGIN SELECT RAISE(ABORT, 'fixture cursor write failed'); END`); err != nil {
		t.Fatal(err)
	}
	if err := deliver(1, "start-game", "task-origin-game", "Game"); err == nil || !strings.Contains(err.Error(), "fixture cursor write failed") {
		t.Fatalf("expected cursor failure: %v", err)
	}
	gameFile := projectMigrationFile("Game")
	game, site := f.projects[gameFile], f.projects[projectMigrationFile("Site")]
	assertCount(f.computer, "SELECT COUNT(*) FROM task_storage_routes", 0)
	assertCount(f.computer, "SELECT COUNT(*) FROM net_outbox", 0)
	if cursor := cursorForFixture(t, f.computer); cursor != 0 {
		t.Fatalf("uncommitted cursor = %d", cursor)
	}
	assertCount(site, "SELECT COUNT(*) FROM tasks", 0)
	var saved string
	if err := game.QueryRow("SELECT id FROM tasks WHERE origin_task='task-origin-game'").Scan(&saved); err != nil {
		t.Fatal(err)
	}
	run := f.runID(t, "Game", saved)
	if _, _, err := beginSplitLocalRun(ctx, f.computer, f.project, "Game", run, "not-committed"); !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "route was not committed") {
		t.Fatalf("unregistered assignment was eligible for launch: %v", err)
	}
	if err := f.computer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := game.Close(); err != nil {
		t.Fatal(err)
	}
	f.computer, game = f.open(t, migrationComputerFile), f.open(t, gameFile)
	f.projects[gameFile] = game
	if _, err := f.computer.Exec("DROP TRIGGER fail_start_cursor"); err != nil {
		t.Fatal(err)
	}
	for i, delivery := range []struct {
		op, task, project string
	}{
		{"start-game", "task-origin-game", "Game"},
		{"start-game", "task-origin-game", "Game"},
		{"reused-remote-task", "task-origin-game", "Site"},
		{"start-site", "task-origin-site", "Site"},
		{"start-missing", "task-origin-missing", "Missing"},
	} {
		if err := deliver(int64(i+1), delivery.op, delivery.task, delivery.project); err != nil {
			t.Fatalf("%s: %v", delivery.op, err)
		}
	}
	if cursor := cursorForFixture(t, f.computer); cursor != 5 {
		t.Fatalf("cursor = %d", cursor)
	}
	assertCount(game, "SELECT COUNT(*) FROM task_runs", 1)
	assertCount(site, "SELECT COUNT(*) FROM task_runs", 1)
	assertCount(site, "SELECT COUNT(*) FROM tasks WHERE origin_task='task-origin-site'", 1)
	assertCount(f.computer, "SELECT COUNT(*) FROM task_storage_routes WHERE origin_computer='origin-machine'", 2)
	assertCount(f.computer, "SELECT COUNT(*) FROM net_outbox WHERE kind='refused'", 1)
	assertCount(f.computer, "SELECT COUNT(*) FROM net_outbox WHERE kind='accepted'", 2)
	pending, err := splitNetPending(ctx, f.computer, f.project, testBroker, 10, nil)
	if err != nil || len(pending) != 3 {
		t.Fatalf("start replies: %d %v", len(pending), err)
	}
	var accepted netBody
	if err := json.Unmarshal(pending[0].Body, &accepted); err != nil || accepted.Task != "task-origin-game" || accepted.Mirror != saved || accepted.Kind != "accepted" {
		t.Fatalf("acceptance after replay: %+v %v", accepted, err)
	}
	if launched, task, err := beginSplitLocalRun(ctx, f.computer, f.project, "Game", run, "fixture-reservation"); err != nil || launched.State != "launching" || task.ID != saved {
		t.Fatalf("registered run was not eligible for reservation: %q %q %v", launched.State, task.ID, err)
	}
}

func cursorForFixture(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	var cursor int64
	if err := db.QueryRow("SELECT COALESCE((SELECT CAST(value AS INTEGER) FROM meta WHERE key=?),0)", "net_cursor:"+testBroker).Scan(&cursor); err != nil {
		t.Fatal(err)
	}
	return cursor
}
