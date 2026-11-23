package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

func TestSplitWorkerProcessesReportToTheirProjects(t *testing.T) {
	isolateUser(t)
	old := core.TemporaryFolder
	core.TemporaryFolder = func(string) bool { return false }
	t.Cleanup(func() { core.TemporaryFolder = old })
	root, err := os.MkdirTemp("", "ams")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	logDir := t.TempDir()
	t.Setenv("MESSENGER_TASK_FIXTURE", "report")
	t.Setenv("MESSENGER_TASK_LOG", filepath.Join(logDir, "runs.jsonl"))
	t.Setenv("MESSENGER_TASK_DIR", logDir)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(root, dbName)
	legacy, err := core.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer legacy.Close()
	for _, project := range []string{"Game", "Site"} {
		if err := legacy.CreateProject(project, t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	if err := legacy.SetComputerAgent("claude", exe, "", "messenger-only"); err != nil {
		t.Fatal(err)
	}
	for _, project := range []string{"Game", "Site"} {
		_, _, err := legacy.StartTask(core.StartTaskRequest{Project: project, OpID: "work-" + project, Provider: "claude", Text: "Work for " + project})
		if err != nil {
			t.Fatal(err)
		}
	}
	runCtx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- RunWithOptions(runCtx, root, AdapterLauncher{AllowTasks: true}, RunOptions{ProjectStorage: true, StartPaused: true})
	}()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("supervisor stopped: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Error("supervisor did not stop")
		}
	}()
	waitSocket(t, root)
	reply, err := control.Call(root, control.Request{Cmd: "resume"})
	if err != nil || !reply.OK {
		t.Fatalf("resume during startup: %+v %v", reply, err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		status, err := control.Call(root, control.Request{Cmd: "status"})
		if err != nil || !status.OK || status.Status == nil {
			t.Fatalf("startup status: %+v %v", status, err)
		}
		if status.Status.ProjectStorage == "active" && !status.Status.Paused {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("storage not active: %+v", status.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
	store, err := core.Open(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, project := range []string{"Game", "Site"} {
		tasks, err := store.Tasks(project, 10)
		if err != nil || len(tasks) != 1 {
			t.Fatalf("%s listing: %+v %v", project, tasks, err)
		}
		(&taskHarness{t: t, st: store}).waitState(tasks[0].ID, "done")
		events, _, err := store.TaskEvents(project, core.AudienceCoordinator, 0, 10)
		if err != nil || len(events) != 1 || events[0].Kind != "report" || events[0].Text != "done: Work for "+project {
			t.Fatalf("%s report: %+v %v", project, events, err)
		}
	}
	waitNoTaskReservations(t)
	for _, project := range []string{"Game", "Site"} {
		tasks, err := legacy.Tasks(project, 10)
		if err != nil || len(tasks) != 1 {
			t.Fatalf("worker wrote to the original queue: %+v %v", tasks, err)
		}
		runs, err := legacy.TaskRuns(tasks[0].ID)
		if err != nil || len(runs) != 1 || runs[0].State != "intent" {
			t.Fatalf("the original run changed: %+v %v", runs, err)
		}
	}
}
