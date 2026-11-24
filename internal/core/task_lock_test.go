package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func intentTasks(t *testing.T, st *Store) []string {
	t.Helper()
	runs, err := st.NextTaskRunIntents(50)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, id := range runs {
		r, err := st.TaskRun(id)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, r.TaskID)
	}
	return out
}

func TestRemoteQueueMovesToRunningAfterObservedLaunch(t *testing.T) {
	for _, layout := range []string{"legacy", "split"} {
		t.Run(layout, func(t *testing.T) {
			p := newNetPair(t)
			if layout == "split" {
				for _, f := range []*taskFixture{&p.a, &p.b} {
					if err := ActivateProjectStorage(context.Background(), f.path); err != nil {
						t.Fatal(err)
					}
					st, err := Open(f.path)
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() { st.Close() })
					f.st = st
				}
			}
			start := func(op string) string {
				t.Helper()
				id, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: op, Computer: "dev-b", Provider: "codex", Text: op, Lock: "Shared cache"})
				if err != nil {
					t.Fatal(err)
				}
				p.carry(t, true)
				p.carry(t, false)
				return id
			}
			first := start("first")
			running := p.b.launch(t, p.mirror(t, first).ID, "019a0000-0000-7000-8000-0000000000a1")
			p.carry(t, false)
			second := start("second")
			assertWindow := func(pending bool, wantLock string) {
				t.Helper()
				view, err := p.a.st.WindowView("Game", "")
				if err != nil || len(view.Tasks) != 2 || view.Tasks[0].ID != second {
					t.Fatalf("window tasks %+v: %v", view.Tasks, err)
				}
				feed, err := p.a.st.TaskFeed(second)
				if err != nil {
					t.Fatal(err)
				}
				for _, got := range []WindowTask{view.Tasks[0], feed.Task} {
					if got.PendingLaunch != pending || got.WaitsForLock != wantLock || (got.StartedAt == "") != pending {
						t.Fatalf("launch state %+v, pending=%v, lock=%q", got, pending, wantLock)
					}
				}
				shared, err := p.a.st.SharedTasks(time.Now().Add(-time.Hour))
				if err != nil || len(shared) != 2 || shared[0].ID != second || shared[0].PendingLaunch != pending || shared[0].WaitsForLock != wantLock {
					t.Fatalf("shared launch state %+v: %v", shared, err)
				}
			}
			assertWindow(true, "shared cache")
			worker := p.b
			var err error
			worker.st, err = p.b.st.storeForTask(running.TaskID)
			if err != nil {
				t.Fatal(err)
			}
			worker.dispose(t, running, DispositionReport, "Done")
			if _, err := p.b.st.FinishTaskRun(running.ID, cleanExit); err != nil {
				t.Fatal(err)
			}
			p.b.launch(t, p.mirror(t, second).ID, "")
			p.carry(t, false)
			assertWindow(false, "")
		})
	}
}

func TestTaskLockKeepsSecondTaskWaiting(t *testing.T) {
	f := newTaskFixture(t)
	start := func(op, lock string) string {
		id, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: op, Provider: "claude", Text: "Work " + op, Lock: lock})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := start("a", "Unity editor")
	second := start("b", " unity EDITOR ")
	free := start("c", "")
	other := start("d", "blender")
	got := strings.Join(intentTasks(t, f.st), ",")
	if got != strings.Join([]string{first, free, other}, ",") {
		t.Fatalf("intents %s", got)
	}
	if holder, err := f.st.TaskLockHolder(second); err != nil || holder != first {
		t.Fatalf("holder %q %v", holder, err)
	}
	if holder, _ := f.st.TaskLockHolder(first); holder != "" {
		t.Fatalf("first waits for %q", holder)
	}
	if _, err := f.st.db.Exec(`UPDATE tasks SET state='done' WHERE id=?`, first); err != nil {
		t.Fatal(err)
	}
	if got := intentTasks(t, f.st); len(got) != 3 || got[0] != second {
		t.Fatalf("after the first ends %v", got)
	}
	// The same operation with another lock is a different task.
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "a", Provider: "claude", Text: "Work a", Lock: "other"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("op reuse %v", err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "e", Provider: "claude", Text: "x", Lock: "a\nb"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("bad lock %v", err)
	}
}

func TestTaskLockTravelsToAnotherComputer(t *testing.T) {
	p := newNetPair(t)
	id, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op", Computer: "dev-b", Provider: "codex", Text: "Build", Lock: "Unity"})
	if err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	m := p.mirror(t, id)
	var lock string
	if err := p.b.st.db.QueryRow(`SELECT resource FROM tasks WHERE id=?`, m.ID).Scan(&lock); err != nil || lock != "unity" {
		t.Fatalf("mirror lock %q %v", lock, err)
	}
}

func TestWindowShowsTheWaitedLock(t *testing.T) {
	f := newTaskFixture(t)
	for _, op := range []string{"a", "b"} {
		if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: op, Provider: "claude", Text: "Work " + op, Lock: "Unity"}); err != nil {
			t.Fatal(err)
		}
	}
	v, err := f.st.WindowView("Game", "")
	if err != nil {
		t.Fatal(err)
	}
	waits := map[string]string{}
	for _, w := range v.Tasks {
		waits[w.Title] = w.WaitsForLock
	}
	if waits["Work a"] != "" || waits["Work b"] != "unity" {
		t.Fatalf("waits %v", waits)
	}
}
