package core

import (
	"strings"
	"testing"
	"time"
)

// Shared task lists (T-047): a computer shares the tasks it started (not the
// ones another computer started here), newest first, with the start of the
// report; the window shows the lists of the other computers for the current
// project, without the tasks that already run here, and forgets them when
// the network changes.
func TestSharedTasks(t *testing.T) {
	f := newTaskFixture(t)
	done := f.start(t, "op-01", "codex", "Menu")
	r := f.launch(t, done, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r, DispositionReport, strings.Repeat("я", MaxSharedReport+50))
	ev := cleanExit
	ev.Usage = &TaskUsage{InputTokens: 1000, OutputTokens: 200}
	if _, err := f.st.FinishTaskRun(r.ID, ev); err != nil {
		t.Fatal(err)
	}
	running := f.start(t, "op-02", "claude", "Build")
	theirs := f.start(t, "op-03", "codex", "Their task")
	old := f.start(t, "op-04", "codex", "Old")
	if _, err := f.st.db.Exec(`UPDATE tasks SET origin_broker='b',origin_peer='peer-mac',origin_task='their-1' WHERE id=?`, theirs); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.db.Exec(`UPDATE tasks SET created_at='2000-01-01T00:00:00Z' WHERE id=?`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.db.Exec(`UPDATE tasks SET created_at='2026-11-08T10:00:00Z' WHERE id=?`, done); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.db.Exec(`UPDATE tasks SET created_at='2026-11-08T11:00:00Z' WHERE id=?`, running); err != nil {
		t.Fatal(err)
	}

	mine, err := f.st.SharedTasks(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 2 || mine[0].ID != running || mine[1].ID != done {
		t.Fatalf("shared %+v", mine)
	}
	if m := mine[1]; m.Project != "Game" || m.Agent != "codex" || m.State != "done" || m.Tokens != 1200 || m.FinishedAt == "" || len([]rune(m.Report)) != MaxSharedReport {
		t.Fatalf("done task %+v", m)
	}
	if m := mine[0]; m.State != "running" || m.Report != "" || m.Title != "Build" {
		t.Fatalf("running task %+v", m)
	}

	if err := f.st.SetNetwork("b", "me", nil); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetNetTasks([]NetPeerTasks{{Peer: "peer-mac", Name: "Mac", Age: 3, Tasks: []NetTask{
		{ID: "their-1", Project: "Game", Title: "Their task", Agent: "codex", State: "running", CreatedAt: "2026-11-08T09:00:00Z"},
		{ID: "their-2", Project: "Game", Title: "Docs", Agent: "claude", State: "done", CreatedAt: "2026-11-08T08:00:00Z", Report: "Done"},
		{ID: "their-3", Project: "Other", Title: "Elsewhere", Agent: "claude", State: "done", CreatedAt: "2026-11-08T08:00:00Z"},
	}}, {Peer: "peer-pc", Name: "PC", Tasks: []NetTask{}}}); err != nil {
		t.Fatal(err)
	}
	v, err := f.st.WindowView("Game", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Shared) != 1 || v.Shared[0].Name != "Mac" || v.Shared[0].Age < 3 || len(v.Shared[0].Tasks) != 1 || v.Shared[0].Tasks[0].ID != "their-2" {
		t.Fatalf("window shows %+v", v.Shared)
	}
	// Removing a task from the list hides only that task, and only here
	// (T-131); the next list from the computer keeps it hidden.
	if err := f.st.HideNetTask("peer-mac", "their-2"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.HideNetTask("peer-mac", "their-2"); err != nil {
		t.Fatalf("hiding twice: %v", err)
	}
	if v, _ := f.st.WindowView("Game", ""); len(v.Shared) != 0 {
		t.Fatalf("hidden task still shown: %+v", v.Shared)
	}
	if err := f.st.SetNetTasks([]NetPeerTasks{{Peer: "peer-mac", Name: "Mac", Tasks: []NetTask{
		{ID: "their-2", Project: "Game", Title: "Docs", Agent: "claude", State: "done", CreatedAt: "2026-11-08T08:00:00Z"},
		{ID: "their-4", Project: "Game", Title: "Tests", Agent: "claude", State: "done", CreatedAt: "2026-11-08T10:00:00Z"},
	}}}); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.st.WindowView("Game", ""); len(v.Shared) != 1 || len(v.Shared[0].Tasks) != 1 || v.Shared[0].Tasks[0].ID != "their-4" {
		t.Fatalf("after a new list: %+v", v.Shared)
	}
	if err := f.st.HideNetTask("", "x"); err == nil {
		t.Fatal("hid a task without a computer")
	}
	// The same network keeps the lists; another network drops them.
	if err := f.st.SetNetwork("b", "me", nil); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.st.WindowView("Game", ""); len(v.Shared) != 1 {
		t.Fatalf("lists lost in the same network: %+v", v.Shared)
	}
	if err := f.st.SetNetwork("", "", nil); err != nil {
		t.Fatal(err)
	}
	if v, _ := f.st.WindowView("Game", ""); len(v.Shared) != 0 {
		t.Fatalf("lists kept after leaving the network: %+v", v.Shared)
	}
}

// Removing an own task from the list (T-131): only a finished task goes; it
// leaves the window and the shared list, and stays in the database.
func TestHideOwnTask(t *testing.T) {
	f := newTaskFixture(t)
	done := f.start(t, "op-01", "codex", "Menu")
	r := f.launch(t, done, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r, DispositionReport, "ok")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	running := f.start(t, "op-02", "claude", "Build")
	if err := f.st.HideTask(running); err == nil {
		t.Fatal("removed a running task")
	}
	if err := f.st.HideTask("nope"); err == nil {
		t.Fatal("removed an unknown task")
	}
	if err := f.st.HideTask(done); err != nil {
		t.Fatal(err)
	}
	v, err := f.st.WindowView("Game", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Tasks) != 1 || v.Tasks[0].ID != running {
		t.Fatalf("window shows %+v", v.Tasks)
	}
	mine, err := f.st.SharedTasks(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 || mine[0].ID != running {
		t.Fatalf("shared %+v", mine)
	}
	var state string
	if err := f.st.db.QueryRow(`SELECT state FROM tasks WHERE id=?`, done).Scan(&state); err != nil || state != "done" {
		t.Fatalf("task after removal %q %v", state, err)
	}
}
