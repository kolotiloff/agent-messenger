package core

import (
	"errors"
	"testing"
)

// The owner removes a project from the window (T-055, D-066). It leaves the
// project list and refuses new tasks; its task history stays in the
// database. A project with a task that is not finished cannot be removed.
func TestRemoveProject(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op-1", "codex", "Fix")
	if err := f.st.RemoveProject("Game"); !errors.Is(err, ErrRejected) {
		t.Fatalf("removed with a running task: %v", err)
	}
	if state, err := f.st.CancelTask(id); err != nil || state != "cancelled" {
		t.Fatalf("cancel %q %v", state, err)
	}
	if err := f.st.RemoveProject("Nope"); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown project: %v", err)
	}
	if err := f.st.RemoveProject("Game"); err != nil {
		t.Fatal(err)
	}
	if projects, _ := f.st.Projects(); len(projects) != 0 {
		t.Fatalf("still listed %+v", projects)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-2", Provider: "codex", Text: "Again"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("task for a removed project: %v", err)
	}
	if err := f.st.SetProjectFolder("Game", f.folder); !errors.Is(err, ErrRejected) {
		t.Fatalf("folder for a removed project: %v", err)
	}
	if err := f.st.RemoveProject("Game"); !errors.Is(err, ErrRejected) {
		t.Fatalf("removed twice: %v", err)
	}
	// The history is kept; creating the project again brings it back.
	if task, err := f.st.TaskByID(id); err != nil || task.State != "cancelled" {
		t.Fatalf("history %+v %v", task, err)
	}
	if err := f.st.CreateProject("Game", f.folder); err != nil {
		t.Fatal(err)
	}
	if projects, _ := f.st.Projects(); len(projects) != 1 || projects[0].Folder != f.folder {
		t.Fatalf("recreated %+v", projects)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-3", Provider: "codex", Text: "Again"}); err != nil {
		t.Fatal(err)
	}
}
