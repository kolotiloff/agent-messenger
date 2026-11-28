package core

import (
	"fmt"
	"testing"
)

func TestWindowPagesCompletedHistoryAndKeepsOlderActiveTasks(t *testing.T) {
	f := newTaskFixture(t)
	old := f.start(t, "active-old", "codex", "Older active task")
	var finished []string
	for i := 0; i < 24; i++ {
		provider := "codex"
		if i%2 != 0 {
			provider = "claude"
		}
		id := f.start(t, fmt.Sprintf("archive-%d", i), provider, "Completed task fixture")
		if _, err := f.st.CancelTask(id); err != nil {
			t.Fatal(err)
		}
		finished = append(finished, id)
	}
	v, err := f.st.WindowView("Game", "")
	if err != nil || len(v.Tasks) != 11 || v.Tasks[0].ID != old || v.HistoryNext != finished[14] {
		t.Fatalf("initial page has %d rows, next=%q: %v", len(v.Tasks), v.HistoryNext, err)
	}
	if err := f.st.HideTask(v.HistoryNext); err != nil {
		t.Fatal(err)
	}
	// An inserted row ahead of this cursor must not repeat or skip rows.
	if _, err := f.st.CancelTask(f.start(t, "new-finish", "codex", "Newer archive entry")); err != nil {
		t.Fatal(err)
	}
	p, err := f.st.WindowTaskHistory("Game", v.HistoryNext, "", "")
	if err != nil || len(p.Tasks) != 10 || p.Tasks[0].ID != finished[13] || p.Next != finished[4] {
		t.Fatalf("next history page has %d rows, next=%q: %v", len(p.Tasks), p.Next, err)
	}
	filtered, err := f.st.WindowViewFiltered("Game", "", "claude", "local", "")
	if err != nil || len(filtered.Tasks) != 11 || filtered.Tasks[0].ID != old || filtered.Tasks[1].Agent != "claude" || filtered.HistoryNext == "" {
		t.Fatalf("filtered page has %d rows, next=%q: %v", len(filtered.Tasks), filtered.HistoryNext, err)
	}
	unchanged, err := f.st.WindowViewFiltered("Game", "", "claude", "local", filtered.HistoryVersion)
	if err != nil || !unchanged.HistoryUnchanged || len(unchanged.Tasks) != 1 || unchanged.Tasks[0].ID != old {
		t.Fatalf("the unchanged poll must only return active cards: %+v %v", unchanged.Tasks, err)
	}
	if err := f.st.HideWindowHistory("Game", "claude", "local"); err != nil {
		t.Fatal(err)
	}
	filtered, err = f.st.WindowViewFiltered("Game", "", "claude", "local", filtered.HistoryVersion)
	if err != nil || len(filtered.Tasks) != 1 || filtered.Tasks[0].ID != old || filtered.HistoryNext != "" {
		t.Fatalf("the active task must outlive history cleanup: %d rows, next=%q: %v", len(filtered.Tasks), filtered.HistoryNext, err)
	}
	if _, err := f.st.TaskFeed(finished[23]); err != nil {
		t.Fatalf("hidden history could not be opened directly: %v", err)
	}
}

func TestPersonalConversationStaysInSidebarUntilArchived(t *testing.T) {
	f := newTaskFixture(t)
	id, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "personal", Provider: "codex", Text: "Personal chat", ByOwner: true})
	if err != nil {
		t.Fatal(err)
	}
	run := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, run, DispositionReport, "Reply")
	if _, err := f.st.FinishTaskRun(run.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	check := func(open bool) {
		t.Helper()
		v, err := f.st.WindowView("Game", "")
		if err != nil || len(v.Tasks) != 1 || v.Tasks[0].KeepOpen != open {
			t.Fatalf("open conversations: %+v, %v", v.Tasks, err)
		}
		archive, err := f.st.WindowTaskHistory("Game", "", "", "")
		if err != nil || (len(archive.Tasks) == 0) != open {
			t.Fatalf("archive: %+v, %v", archive, err)
		}
		feed, err := f.st.TaskFeed(id)
		if err != nil || feed.Task.KeepOpen != open || feed.Task.CanContinue != open || len(feed.Items) != 2 {
			t.Fatalf("conversation: %+v, %v", feed, err)
		}
	}
	if err := f.st.HideWindowHistory("Game", "", ""); err != nil {
		t.Fatal(err)
	}
	check(true)
	if err := f.st.SetTaskArchive(id, "archive", 0); err != nil {
		t.Fatal(err)
	}
	check(false)
	if err := f.st.SetTaskArchive(id, "restore", 1); err != nil {
		t.Fatal(err)
	}
	check(true)
}

func TestStoppedPersonalConversationStaysWithActiveTasks(t *testing.T) {
	f := newTaskFixture(t)
	direct, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "direct-stop", Provider: "codex", Text: "Personal conversation", ByOwner: true})
	if err != nil {
		t.Fatal(err)
	}
	coordinated := f.start(t, "coordinator-stop", "codex", "Coordinated task")
	for i, task := range []string{direct, coordinated} {
		run := f.launch(t, task, []string{"019a0000-0000-7000-8000-0000000000a1", "019a0000-0000-7000-8000-0000000000a2"}[i])
		if state, err := f.st.CancelTask(task); err != nil || state != "cancelling" {
			t.Fatalf("request stop: %s, %v", state, err)
		}
		exit := cleanExit
		exit.Cancelled = true
		if state, err := f.st.FinishTaskRun(run.ID, exit); err != nil || state != "cancelled" {
			t.Fatalf("finish stop: %s, %v", state, err)
		}
	}
	view, err := f.st.WindowView("Game", "")
	if err != nil {
		t.Fatal(err)
	}
	open := map[string]bool{}
	for _, task := range view.Tasks {
		open[task.ID] = task.KeepOpen
	}
	if !open[direct] || open[coordinated] {
		t.Fatalf("stopped direct and coordinated tasks: %v", open)
	}
	archive, err := f.st.WindowTaskHistory("Game", "", "", "")
	if err != nil || len(archive.Tasks) != 1 || archive.Tasks[0].ID != coordinated {
		t.Fatalf("only the coordinated task should be archived: %+v, %v", archive, err)
	}
	if err := f.st.SetTaskArchive(direct, "archive", 0); err != nil {
		t.Fatal(err)
	}
	archive, err = f.st.WindowTaskHistory("Game", "", "", "")
	if err != nil || len(archive.Tasks) != 2 {
		t.Fatalf("archive after manual action: %+v, %v", archive, err)
	}
}
