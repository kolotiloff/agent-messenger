package core

import (
	"errors"
	"strings"
	"testing"
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
