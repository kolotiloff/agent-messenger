package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

func TestSplitOutboxKeepsOrderAndAcknowledgesOnlyItsFile(t *testing.T) {
	pair := newNetPair(t)
	if err := pair.a.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	pair.startRemote(t, "game-send", "claude", "Game work")
	if _, _, err := pair.a.st.StartTask(StartTaskRequest{Project: "Site", OpID: "site-send", Computer: "dev-b", Provider: "codex", Text: "Site work"}); err != nil {
		t.Fatal(err)
	}
	if err := pair.a.st.tx(func(tx *sql.Tx) error {
		return enqueueNet(tx, testBroker, "dev-b", "already-stored", "op-fixture", netBody{Kind: "agent_version", Task: "op-fixture"})
	}); err != nil {
		t.Fatal(err)
	}
	legacy, err := pair.a.st.NetPending(testBroker, 10)
	if err != nil || len(legacy) != 3 {
		t.Fatalf("legacy outbox: %+v %v", legacy, err)
	}
	highWater := legacy[2].Seq
	if err := pair.a.st.NetSent(highWater); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	out := filepath.Join(t.TempDir(), "layout")
	if err := StageProjectMigration(ctx, pair.a.path, out); err != nil {
		t.Fatal(err)
	}
	open := func(file string) *sql.DB {
		t.Helper()
		db, err := sql.Open("sqlite", dsn(filepath.Join(out, file)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		return db
	}
	computer := open(migrationComputerFile)
	gameFile, siteFile := projectMigrationFile("Game"), projectMigrationFile("Site")
	games := map[string]*sql.DB{gameFile: open(gameFile), siteFile: open(siteFile)}
	project := func(file string) (*sql.DB, error) {
		db := games[file]
		if db == nil {
			t.Fatalf("unexpected project file: %q", file)
		}
		return db, nil
	}
	pending, err := splitNetPending(ctx, computer, project, testBroker, 10, nil)
	if err != nil || len(pending) != 2 || pending[0].file != gameFile || pending[1].file != siteFile {
		t.Fatalf("migrated pending: %+v %v", pending, err)
	}
	for i, msg := range pending {
		if msg.Seq != legacy[i].Seq || msg.OpID != legacy[i].OpID || string(msg.Body) != string(legacy[i].Body) {
			t.Fatalf("migrated frame %d changed or moved out of order", i)
		}
		if err := splitNetSent(ctx, computer, project, msg); err != nil {
			t.Fatal(err)
		}
	}
	for file, db := range games {
		if _, err := db.Exec(`INSERT INTO net_outbox(broker,recipient,op_id,kind,task_id,body,created_at)
 VALUES (?,?,?,'event','fixture','{}',?)`, testBroker, "dev-b", file, now()); err != nil {
			t.Fatal(err)
		}
	}
	pending, err = splitNetPending(ctx, computer, project, testBroker, 10, nil)
	if err != nil || len(pending) != 2 || pending[0].Seq != highWater+1 || pending[1].Seq != highWater+1 {
		t.Fatalf("new messages must use separate sequence spaces: %+v %v", pending, err)
	}

	stale := pending[0]
	stale.OpID = legacy[0].OpID
	if err := splitNetSent(ctx, computer, project, stale); err != nil {
		t.Fatal(err)
	}
	remaining, err := splitNetPending(ctx, computer, project, testBroker, 10, nil)
	if err != nil || len(remaining) != 2 {
		t.Fatalf("stale acknowledgement deleted a new message: %+v %v", remaining, err)
	}
	first := pending[0]
	if err := splitNetSent(ctx, computer, project, first); err != nil {
		t.Fatal(err)
	}
	if err := splitNetSent(ctx, computer, project, first); err != nil {
		t.Fatal(err)
	}
	remaining, err = splitNetPending(ctx, computer, project, testBroker, 1, nil)
	if err != nil || len(remaining) != 1 || remaining[0].file == first.file || remaining[0].OpID != pending[1].OpID {
		t.Fatalf("another project's delivery was lost: %+v %v", remaining, err)
	}
	remaining, err = splitNetPending(ctx, computer, project, testBroker, 10, []string{"dev-b"})
	if err != nil || len(remaining) != 0 {
		t.Fatalf("recipient backoff ignored: %+v %v", remaining, err)
	}
}

func TestSplitStopRequestSurvivesReopeningAndWakesItsProject(t *testing.T) {
	ctx := context.Background()
	pair := newNetPair(t)
	if err := pair.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	id := pair.startRemote(t, "stop", "codex", "Long job")
	pair.carry(t, true)
	if state, err := pair.a.st.CancelTask(id); err != nil || state != "cancelling" {
		t.Fatalf("cancel: %s %v", state, err)
	}
	if err := ActivateProjectStorage(ctx, pair.a.path); err != nil {
		t.Fatal(err)
	}
	st, err := Open(pair.a.path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	assertStop := func(want string) {
		t.Helper()
		remote, err := st.TaskRemote(id)
		if err != nil || remote == nil || remote.CancelState != want {
			t.Fatalf("stop after reopening: %+v, want %s (%v)", remote, want, err)
		}
	}
	assertStop("queued")
	before, err := st.WindowChanges(ctx, "Game", id)
	if err != nil {
		t.Fatal(err)
	}
	pending, err := st.NetPending(testBroker, 10)
	if err != nil || len(pending) != 1 || pending[0].OpID != "cancel."+id {
		t.Fatalf("pending cancellation: %+v %v", pending, err)
	}
	if err := st.NetDeliverySent(pending[0]); err != nil {
		t.Fatal(err)
	}
	after, err := st.WindowChanges(ctx, "Game", id)
	if err != nil || after.Window == before.Window {
		t.Fatalf("broker receipt did not wake the project: %+v %v", after, err)
	}
	assertStop("sent")
}
