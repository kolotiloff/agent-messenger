package core

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectSnapshotKeepsRemovedProjectAndItsCursor(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.CreateProject("Other", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	other, _, err := f.st.StartTask(StartTaskRequest{Project: "Other", OpID: "other", Provider: "codex", Text: "other task"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.AddFeedback(AudienceOwner, "Other", "another project", "", "", nil); err != nil {
		t.Fatal(err)
	}
	task := f.start(t, "game", "claude", "work")
	run := f.launch(t, task, f.sessionOf(t, task))
	if err := f.st.AppendTaskRunItem(run.ID, "text", "", "worker text", ""); err != nil {
		t.Fatal(err)
	}
	png := []byte("\x89PNG\r\n\x1a\nfixture")
	if _, err := f.st.ReadTask(run.ID, run.Receipt, false); err != nil {
		t.Fatal(err)
	}
	if err := f.st.RecordDispositionImages(run.ID, run.Receipt, DispositionReport, "finished", []TaskImageFile{{Name: "report.png", Data: png}}); err != nil {
		t.Fatal(err)
	}
	if state, err := f.st.FinishTaskRun(run.ID, cleanExit); err != nil || state != "finished" {
		t.Fatalf("finish: %q %v", state, err)
	}
	cursor, err := f.st.LatestTaskEvent("Game")
	if err != nil || cursor == 0 {
		t.Fatalf("cursor: %d, %v", cursor, err)
	}
	if _, err := f.st.AddFeedback(AudienceOwner, "Game", "project feedback", "", "", nil); err != nil {
		t.Fatal(err)
	}
	if err := f.st.RemoveProject("Game"); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "Game snapshot.sqlite")
	// Another process can have a pending write while the exporter reads a
	// committed snapshot. Its uncommitted change must not be copied.
	writer, err := f.st.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if _, err := writer.Exec("UPDATE tasks SET text='uncommitted' WHERE id=?", task); err != nil {
		t.Fatal(err)
	}
	if err := SnapshotProjectDatabase(context.Background(), f.path, "Game", out); err != nil {
		t.Fatal(err)
	}
	if err := writer.Rollback(); err != nil {
		t.Fatal(err)
	}
	archive, err := sql.Open("sqlite", sqliteFileURL(out, url.Values{"mode": {"ro"}}))
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	check := func(query string, want any) {
		t.Helper()
		var got any
		if err := archive.QueryRow(query).Scan(&got); err != nil || got != want {
			t.Fatalf("%s: got %v, want %v, err %v", query, got, want, err)
		}
	}
	check("SELECT project FROM project_snapshot", "Game")
	check("SELECT name FROM projects", "Game")
	check("SELECT COUNT(*) FROM projects", int64(1))
	check("SELECT COUNT(*) FROM tasks", int64(1))
	check("SELECT id FROM tasks", task)
	check("SELECT text FROM tasks", "work")
	check("SELECT id FROM task_runs", run.ID)
	check("SELECT MAX(seq) FROM task_events", cursor)
	check("SELECT text FROM task_run_items", "worker text")
	check("SELECT text FROM feedback", "project feedback")
	check("SELECT COUNT(*) FROM sqlite_schema WHERE type='index' AND name IN ('tasks_session','task_owner_notes_task','task_events_project','task_images_task','task_images_run')", int64(5))
	check("SELECT COUNT(*) FROM sqlite_schema WHERE name IN ('meta','net_peers','net_outbox','computer_agents','coordinator_chats')", int64(0))
	var saved []byte
	if err := archive.QueryRow("SELECT data FROM task_images").Scan(&saved); err != nil || !bytes.Equal(saved, png) {
		t.Fatalf("image: %x, %v", saved, err)
	}
	if _, err := f.st.TaskByID(other); err != nil {
		t.Fatalf("another project's task changed: %v", err)
	}
	if _, err := f.st.TaskByID(task); err != nil {
		t.Fatalf("live history changed: %v", err)
	}
	if accidental, err := Open(out); !errors.Is(err, ErrRejected) {
		accidental.Close()
		t.Fatalf("snapshot opened as a working queue: %v", err)
	}
}

func TestProjectSnapshotDoesNotReplaceFilesOrPublishFailedCopies(t *testing.T) {
	f := newTaskFixture(t)
	dir := t.TempDir()
	out := filepath.Join(dir, "snapshot.sqlite")
	if err := SnapshotProjectDatabase(context.Background(), f.path, "Missing", out); !errors.Is(err, ErrRejected) {
		t.Fatalf("missing project: %v", err)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed snapshot published: %v", err)
	}
	if err := os.WriteFile(out, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SnapshotProjectDatabase(context.Background(), f.path, "Game", out); !errors.Is(err, os.ErrExist) {
		t.Fatalf("existing snapshot: %v", err)
	}
	if data, err := os.ReadFile(out); err != nil || string(data) != "original" {
		t.Fatalf("existing file changed: %q %v", data, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 {
		t.Fatalf("staging files remain: %v %v", files, err)
	}
}
