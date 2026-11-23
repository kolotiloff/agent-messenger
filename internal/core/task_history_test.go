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
