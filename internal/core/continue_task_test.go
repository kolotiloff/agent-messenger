package core

import (
	"errors"
	"testing"
)

// A finished task continues in the same conversation with a follow-up
// (T-051). Nothing is recovered: an unclear task is not continued.
func TestContinueFinishedTask(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "codex", "Fix the menu")
	if err := f.st.ContinueTask(id, "c0", "Also the map", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("a running task continued: %v", err)
	}
	r := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r, DispositionReport, "Menu fixed")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	if task, _ := f.st.TaskByID(id); task.State != "done" {
		t.Fatalf("task %+v", task)
	}
	for _, bad := range []struct{ op, text string }{{"", "x"}, {"c1", ""}} {
		if err := f.st.ContinueTask(id, bad.op, bad.text, AudienceCoordinator); !errors.Is(err, ErrRejected) {
			t.Fatalf("accepted %+v: %v", bad, err)
		}
	}
	if err := f.st.ContinueTask(id, "c1", "Also the map", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	// Repeating the same call after a lost response opens no second run;
	// another follow-up while it runs is refused.
	if err := f.st.ContinueTask(id, "c1", "Also the map", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	if err := f.st.ContinueTask(id, "c2", "And the sound", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("second follow-up while running: %v", err)
	}
	task, _ := f.st.TaskByID(id)
	if task.State != "running" || task.FinishedAt != "" {
		t.Fatalf("continued task %+v", task)
	}
	runs, _ := f.st.NextTaskRunIntents(10)
	if len(runs) != 1 {
		t.Fatalf("runs %v", runs)
	}
	r2, _, err := f.st.BeginTaskRun(runs[0], "res2")
	if err != nil {
		t.Fatal(err)
	}
	if r2.Kind != "continue" || r2.Ordinal != 2 {
		t.Fatalf("run %+v", r2)
	}
	if err := f.st.RecordTaskRunProcess(r2.ID, taskObs); err != nil {
		t.Fatal(err)
	}
	// It must be the same conversation.
	if err := f.st.BindTaskSession(r2.ID, "019a0000-0000-7000-8000-000000000002"); !errors.Is(err, ErrIdentity) {
		t.Fatalf("other conversation accepted: %v", err)
	}
	if err := f.st.BindTaskSession(r2.ID, "019a0000-0000-7000-8000-000000000001"); err != nil {
		t.Fatal(err)
	}
	brief, err := f.st.ReadTask(r2.ID, r2.Receipt, false)
	if err != nil || brief.FollowUp != "Also the map" || brief.Task != "" || brief.Note == "" || brief.Run != 2 {
		t.Fatalf("brief %+v %v", brief, err)
	}
	if err := f.st.RecordDisposition(r2.ID, r2.Receipt, DispositionReport, "Map fixed"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.FinishTaskRun(r2.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	if task, _ := f.st.TaskByID(id); task.State != "done" || task.FinishedAt == "" {
		t.Fatalf("finished again %+v", task)
	}
	// The window shows the follow-up as the coordinator's message.
	feed, err := f.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range feed.Items {
		found = found || (it.Kind == "answer" && it.Text == "Also the map" && it.Author == AudienceCoordinator)
	}
	if !found {
		t.Fatalf("feed %+v", feed.Items)
	}
	// The op id of the first follow-up cannot be reused for another text.
	if err := f.st.ContinueTask(id, "c1", "Something else", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("op id reused: %v", err)
	}
}

// Cancelled and blocked tasks are not continued: a blocked task's outcome is
// unclear, and unknown is not success.
func TestContinueRefusesUnclearTasks(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "codex", "Fix the menu")
	if _, err := f.st.CancelTask(id); err != nil {
		t.Fatal(err)
	}
	if err := f.st.ContinueTask(id, "c1", "Again", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("cancelled task continued: %v", err)
	}
	if err := f.st.ContinueTask("task-unknown", "c1", "Again", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown task: %v", err)
	}
}
