package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// D-188: a task whose start failed between the project commit and the
// computer commit holds no resource, and the next start of the client
// closes it without launching it.
func TestSplitUnpublishedTaskHoldsNothingAndIsClosed(t *testing.T) {
	f := newSplitRunFixture(t)
	ctx := context.Background()
	if _, err := f.computer.Exec(`CREATE TRIGGER fail_route BEFORE INSERT ON task_storage_routes
 BEGIN SELECT RAISE(ABORT, 'fixture route write failed'); END`); err != nil {
		t.Fatal(err)
	}
	orphanReq := StartTaskRequest{Project: "Game", OpID: "orphan", Provider: "codex", Text: "fixture", Lock: "cache"}
	if _, _, err := startSplitTask(ctx, f.computer, f.project, orphanReq); err == nil || !strings.Contains(err.Error(), "fixture route write failed") {
		t.Fatalf("expected route failure: %v", err)
	}
	if _, err := f.computer.Exec("DROP TRIGGER fail_route"); err != nil {
		t.Fatal(err)
	}
	game := f.projects[projectMigrationFile("Game")]
	var orphan string
	if err := game.QueryRow("SELECT id FROM tasks WHERE op_id='orphan'").Scan(&orphan); err != nil {
		t.Fatal(err)
	}
	// The unpublished task is older, which used to make it the holder.
	if _, err := game.Exec("UPDATE tasks SET created_at='2026-01-01T00:00:00Z' WHERE id=?", orphan); err != nil {
		t.Fatal(err)
	}
	// A start for another computer was held back with the route.
	if _, err := game.Exec(`INSERT INTO net_outbox(broker,recipient,op_id,kind,task_id,body,created_at)
 VALUES ('brk','peer','held-start','start',?,'{}','2026-01-01T00:00:00Z')`, orphan); err != nil {
		t.Fatal(err)
	}

	later, created, err := startSplitTask(ctx, f.computer, f.project, StartTaskRequest{Project: "Site", OpID: "later", Provider: "codex", Text: "fixture", Lock: "cache"})
	if err != nil || !created {
		t.Fatalf("create later task: %v", err)
	}
	if _, _, err := beginSplitLocalRun(ctx, f.computer, f.project, "Site", f.runID(t, "Site", later), "res-later"); err != nil {
		t.Fatalf("an unpublished task blocked the resource: %v", err)
	}

	s, err := openSplitStore(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if n, err := s.FailUnpublishedTasks(); err != nil || n != 1 {
		t.Fatalf("closed %d unpublished tasks: %v", n, err)
	}
	var state, detail, runState string
	var held, routes, events int
	if err := game.QueryRow(`SELECT t.state,t.detail,r.state,
 (SELECT COUNT(*) FROM net_outbox WHERE task_id=t.id),
 (SELECT COUNT(*) FROM task_events WHERE task_id=t.id AND kind='failed')
 FROM tasks t JOIN task_runs r ON r.task_id=t.id WHERE t.id=?`, orphan).Scan(&state, &detail, &runState, &held, &events); err != nil {
		t.Fatal(err)
	}
	if state != "failed" || detail != unpublishedTaskFailure || runState != "cancelled" || held != 0 || events != 1 {
		t.Fatalf("unpublished task: state=%s detail=%q run=%s held=%d events=%d", state, detail, runState, held, events)
	}
	if err := f.computer.QueryRow("SELECT COUNT(*) FROM task_storage_routes WHERE id=? AND project='Game' AND remote_broker IS NULL", orphan).Scan(&routes); err != nil || routes != 1 {
		t.Fatalf("route was not published: %d %v", routes, err)
	}
	if n, err := s.FailUnpublishedTasks(); err != nil || n != 0 {
		t.Fatalf("second pass closed %d: %v", n, err)
	}
	// Repeating the operation returns the closed task; nothing new starts.
	id, created, err := startSplitTask(ctx, f.computer, f.project, orphanReq)
	if err != nil || created || id != orphan {
		t.Fatalf("repeated start: %q %t %v", id, created, err)
	}
}

// D-188: a start for a project that is visible but missing from the
// storage catalog is refused and the inbox moves on.
func TestSplitRemoteStartRefusesAProjectWithoutStorage(t *testing.T) {
	f := newSplitRunFixture(t)
	ctx := context.Background()
	if _, err := f.computer.Exec("DELETE FROM project_storage_stage WHERE project='Site'"); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(netBody{Kind: "start", Task: "origin-site", Project: "Site", Agent: "codex", Text: "Work", Computer: "origin-machine"})
	if err != nil {
		t.Fatal(err)
	}
	if err := applySplitRemoteStart(ctx, f.computer, f.project, testBroker, NetIncoming{Seq: 1, From: "dev-a", OpID: "start-site", Body: body}); err != nil {
		t.Fatalf("the start stopped the inbox: %v", err)
	}
	if cursor := cursorForFixture(t, f.computer); cursor != 1 {
		t.Fatalf("cursor = %d", cursor)
	}
	var reply string
	if err := f.computer.QueryRow("SELECT body FROM net_outbox WHERE kind='refused'").Scan(&reply); err != nil || !strings.Contains(reply, "storage catalog") {
		t.Fatalf("refusal: %q %v", reply, err)
	}
}
