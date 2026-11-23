package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitProjectResponseResumesAfterCursorWriteFails(t *testing.T) {
	pair := newNetPair(t)
	if err := pair.a.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	origin := pair.startRemote(t, "game-report", "claude", "Game work")
	pair.carry(t, true)
	pair.carry(t, false)
	mirror := pair.mirror(t, origin)
	run := pair.b.launch(t, mirror.ID, mirror.SessionID)
	pair.b.dispose(t, run, DispositionReport, "confirmed fixture result")
	if state, err := pair.b.st.FinishTaskRun(run.ID, cleanExit); err != nil || state != "finished" {
		t.Fatalf("finish: %q %v", state, err)
	}

	outgoing, err := pair.b.st.NetPending(testBroker, 100)
	if err != nil {
		t.Fatal(err)
	}
	cursor := pair.seq
	var report NetIncoming
	for _, item := range outgoing {
		m := NetIncoming{Seq: cursor + 1, From: "dev-b", OpID: item.OpID, Body: item.Body}
		if body, valid := decodeNetIncoming(m); valid && body.Kind == "event" && body.Event == "report" {
			report = m
		}
	}
	if report.OpID == "" {
		t.Fatal("executor did not queue the report")
	}

	ctx := context.Background()
	out := filepath.Join(t.TempDir(), "layout")
	if err := StageProjectMigration(ctx, pair.a.path, out); err != nil {
		t.Fatal(err)
	}
	open := func(name string) *sql.DB {
		t.Helper()
		db, err := sql.Open("sqlite", dsn(filepath.Join(out, name)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		return db
	}
	computer := open(migrationComputerFile)
	game := open(projectMigrationFile("Game"))
	site := open(projectMigrationFile("Site"))
	readCursor := func(db *sql.DB) int64 {
		t.Helper()
		var value int64
		if err := db.QueryRow("SELECT CAST(value AS INTEGER) FROM meta WHERE key=?", "net_cursor:"+testBroker).Scan(&value); err != nil {
			t.Fatal(err)
		}
		return value
	}
	countReports := func(db *sql.DB) int {
		t.Helper()
		var count int
		if err := db.QueryRow("SELECT COUNT(*) FROM task_events WHERE kind='report'").Scan(&count); err != nil {
			t.Fatal(err)
		}
		return count
	}
	resolve := func(file string) (*sql.DB, error) {
		if file != projectMigrationFile("Game") {
			t.Fatalf("report routed to %q", file)
		}
		return game, nil
	}
	if _, err := computer.Exec(`CREATE TRIGGER fail_checkpoint BEFORE UPDATE OF value ON meta
 WHEN OLD.key LIKE 'net_cursor:%' BEGIN SELECT RAISE(ABORT, 'fixture cursor write failed'); END`); err != nil {
		t.Fatal(err)
	}
	if err := applySplitProjectResponse(ctx, computer, resolve, testBroker, report); err == nil || !strings.Contains(err.Error(), "fixture cursor write failed") {
		t.Fatalf("expected cursor write error: %v", err)
	}
	if got := readCursor(computer); got != cursor {
		t.Fatalf("failed checkpoint moved cursor to %d", got)
	}
	if got := countReports(game); got != 1 {
		t.Fatalf("project did not commit the report exactly once: %d", got)
	}
	if got := countReports(site); got != 0 {
		t.Fatalf("other project received %d reports", got)
	}

	// Release both connections and reopen the files before redelivery.
	if err := computer.Close(); err != nil {
		t.Fatal(err)
	}
	if err := game.Close(); err != nil {
		t.Fatal(err)
	}
	computer = open(migrationComputerFile)
	game = open(projectMigrationFile("Game"))
	if _, err := computer.Exec("DROP TRIGGER fail_checkpoint"); err != nil {
		t.Fatal(err)
	}
	if err := applySplitProjectResponse(ctx, computer, resolve, testBroker, report); err != nil {
		t.Fatal(err)
	}
	if got := readCursor(computer); got != report.Seq {
		t.Fatalf("redelivered report did not move cursor to %d: %d", report.Seq, got)
	}
	if got := countReports(game); got != 1 {
		t.Fatalf("redelivery repeated the report: %d", got)
	}
	var text string
	if err := game.QueryRow("SELECT text FROM task_events WHERE task_id=? AND kind='report'", origin).Scan(&text); err != nil || text != "confirmed fixture result" {
		t.Fatalf("retained report: %q %v", text, err)
	}
	var peers int
	if err := computer.QueryRow("SELECT COUNT(*) FROM net_applied WHERE op_id=?", report.OpID).Scan(&peers); err != nil || peers != 1 {
		t.Fatalf("computer receipt: %d %v", peers, err)
	}

	unknown := report
	unknown.Seq++
	unknown.From = "other-peer"
	unknown.OpID += "-unrouted"
	// D-188: a report no route matches is skipped and the inbox moves on.
	if err := applySplitProjectResponse(ctx, computer, resolve, testBroker, unknown); err != nil {
		t.Fatalf("unrouted report stopped the inbox: %v", err)
	}
	if got := readCursor(computer); got != unknown.Seq {
		t.Fatalf("unrouted report did not advance the cursor: %d", got)
	}
	if got := countReports(game); got != 1 {
		t.Fatalf("unrouted report changed a task: %d reports", got)
	}
	if got := countReports(pair.a.st.db); got != 0 {
		t.Fatalf("test wrote back to the legacy working store: %d reports", got)
	}
}
