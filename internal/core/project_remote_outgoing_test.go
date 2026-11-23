package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitAssignmentWithholdsStartUntilItsRouteCommits(t *testing.T) {
	pair := newNetPair(t)
	for _, fixture := range []taskFixture{pair.a, pair.b} {
		if err := fixture.st.CreateProject("Site", t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	files := &splitRunFixture{root: filepath.Join(t.TempDir(), "layout"), projects: map[string]*sql.DB{}}
	if err := StageProjectMigration(ctx, pair.a.path, files.root); err != nil {
		t.Fatal(err)
	}
	files.computer = files.open(t, migrationComputerFile)
	gameFile := projectMigrationFile("Game")
	siteFile := projectMigrationFile("Site")
	files.projects[gameFile], files.projects[siteFile] = files.open(t, gameFile), files.open(t, siteFile)

	if _, err := files.computer.Exec(`CREATE TRIGGER fail_game_route BEFORE INSERT ON task_storage_routes
 WHEN NEW.project='Game' BEGIN SELECT RAISE(ABORT,'lost computer commit'); END`); err != nil {
		t.Fatal(err)
	}
	req := StartTaskRequest{Project: "Game", OpID: "split-assign", Computer: "dev-b", Provider: "claude", Text: "Game assignment", SenderAgent: "codex"}
	if _, _, err := startSplitTask(ctx, files.computer, files.project, req); err == nil || !strings.Contains(err.Error(), "lost computer commit") {
		t.Fatalf("expected interrupted assignment: %v", err)
	}
	game := files.projects[gameFile]
	var taskID, body string
	if err := game.QueryRow(`SELECT t.id,n.body FROM tasks t JOIN net_outbox n ON n.task_id=t.id
 WHERE t.op_id=? AND n.kind='start'`, req.OpID).Scan(&taskID, &body); err != nil {
		t.Fatal(err)
	}
	pending, err := splitNetPending(ctx, files.computer, files.project, testBroker, 1, nil)
	if err != nil || len(pending) != 0 {
		t.Fatalf("interrupted start became eligible: %+v %v", pending, err)
	}
	// A later frame in the same file and a new task in the other project
	// are still eligible, even with a one-message batch limit.
	if _, err := game.Exec(`INSERT INTO net_outbox(broker,recipient,op_id,kind,task_id,body,created_at)
 VALUES (?,?,'a-later-event','event','fixture','{}',?)`, testBroker, "dev-b", now()); err != nil {
		t.Fatal(err)
	}
	pending, err = splitNetPending(ctx, files.computer, files.project, testBroker, 1, nil)
	if err != nil || len(pending) != 1 || pending[0].OpID != "a-later-event" {
		t.Fatalf("an interrupted start blocked the next frame: %+v %v", pending, err)
	}
	if err := splitNetSent(ctx, files.computer, files.project, pending[0]); err != nil {
		t.Fatal(err)
	}
	siteReq := req
	siteReq.Project, siteReq.OpID, siteReq.Text = "Site", "other-assign", "Site assignment"
	siteID, created, err := startSplitTask(ctx, files.computer, files.project, siteReq)
	if err != nil || !created {
		t.Fatalf("second project assignment: %q %t %v", siteID, created, err)
	}
	pending, err = splitNetPending(ctx, files.computer, files.project, testBroker, 1, nil)
	if err != nil || len(pending) != 1 || pending[0].file != siteFile || pending[0].OpID != "start."+siteID {
		t.Fatalf("second project's start was lost: %+v %v", pending, err)
	}

	if err := files.computer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := game.Close(); err != nil {
		t.Fatal(err)
	}
	files.computer = files.open(t, migrationComputerFile)
	files.projects[gameFile] = files.open(t, gameFile)
	if _, err := files.computer.Exec("DROP TRIGGER fail_game_route"); err != nil {
		t.Fatal(err)
	}
	id, created, err := startSplitTask(ctx, files.computer, files.project, req)
	if err != nil || created || id != taskID {
		t.Fatalf("explicit replay: %q %t %v", id, created, err)
	}
	pending, err = splitNetPending(ctx, files.computer, files.project, testBroker, 10, nil)
	if err != nil || len(pending) != 2 {
		t.Fatalf("eligible start frames: %+v %v", pending, err)
	}
	for i, message := range pending {
		if message.file == gameFile && (message.OpID != "start."+taskID || string(message.Body) != body) {
			t.Fatalf("the recovered assignment changed: %+v", message)
		}
		in := NetIncoming{Seq: int64(i + 1), From: HostPeerID, OpID: message.OpID, Body: message.Body}
		if err := pair.b.st.ApplyNet(testBroker, in); err != nil {
			t.Fatal(err)
		}
		if err := splitNetSent(ctx, files.computer, files.project, message); err != nil {
			t.Fatal(err)
		}
	}
	reply, err := pair.b.st.NetPending(testBroker, 10)
	if err != nil || len(reply) != 2 {
		t.Fatalf("legacy executor replies: %+v %v", reply, err)
	}
	for i, message := range reply {
		in := NetIncoming{Seq: int64(3 + i), From: "dev-b", OpID: message.OpID, Body: message.Body}
		if err := applySplitProjectResponse(ctx, files.computer, files.project, testBroker, in); err != nil {
			t.Fatal(err)
		}
	}
	for name, id := range map[string]string{"Game": taskID, "Site": siteID} {
		project := files.projects[projectMigrationFile(name)]
		var storedName, mirror string
		if err := project.QueryRow("SELECT project,remote_task FROM tasks WHERE id=?", id).Scan(&storedName, &mirror); err != nil || storedName != name || mirror == "" {
			t.Fatalf("%s acceptance: %q %q %v", name, storedName, mirror, err)
		}
	}
}
