//go:build darwin || linux

package supervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/mcp"
)

type taskHarness struct {
	t                    *testing.T
	root, folder, logDir string
	st                   *core.Store
	errc                 chan error
	cancel               context.CancelFunc
}

// newTaskHarness prepares a project "Game" with a folder, registers the test
// binary as the Claude and Codex worker (messenger-only profile), and starts
// a client with the given launcher. The worker behaves as mode says.
func newTaskHarness(t *testing.T, mode string, launcher Launcher) *taskHarness {
	t.Helper()
	isolateUser(t)
	old := core.TemporaryFolder
	core.TemporaryFolder = func(string) bool { return false }
	t.Cleanup(func() { core.TemporaryFolder = old })
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root, err := os.MkdirTemp("", "tk")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	h := &taskHarness{t: t, root: root, logDir: mkdir(t, filepath.Join(t.TempDir(), "log"))}
	h.folder = mkdir(t, filepath.Join(t.TempDir(), "game"))
	// The store keeps the folder with symlinks resolved (/tmp is a link on
	// macOS), and the worker runs there.
	if h.folder, err = filepath.EvalSymlinks(h.folder); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MESSENGER_TASK_FIXTURE", mode)
	t.Setenv("MESSENGER_TASK_LOG", filepath.Join(h.logDir, "runs.jsonl"))
	t.Setenv("MESSENGER_TASK_DIR", h.logDir)
	st, err := core.Open(filepath.Join(root, dbName))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	h.st = st
	if err := st.CreateProject("Game", h.folder); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"claude", "codex"} {
		if err := st.SetComputerAgent(p, self, "", "messenger-only"); err != nil {
			t.Fatal(err)
		}
	}
	h.start(launcher)
	return h
}

func (h *taskHarness) start(launcher Launcher) {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.errc = make(chan error, 1)
	go func() { h.errc <- Run(ctx, h.root, launcher) }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if socketAccepts(h.root) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatal("command socket was not created")
}

func (h *taskHarness) stop() {
	h.t.Helper()
	_, _ = control.Call(h.root, control.Request{Cmd: "shutdown"})
	select {
	case <-h.errc:
	case <-time.After(10 * time.Second):
		h.cancel()
		h.t.Fatal("supervisor did not exit")
	}
}

func (h *taskHarness) startTask(op, provider, text string) string {
	h.t.Helper()
	id, created, err := h.st.StartTask(core.StartTaskRequest{Project: "Game", OpID: op, Provider: provider, Text: text})
	if err != nil || !created {
		h.t.Fatalf("start %v %v", created, err)
	}
	return id
}

func (h *taskHarness) waitState(id string, states ...string) core.Task {
	h.t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	var task core.Task
	for time.Now().Before(deadline) {
		var err error
		task, err = h.st.TaskByID(id)
		if err != nil {
			h.t.Fatal(err)
		}
		for _, s := range states {
			if task.State == s {
				return task
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	runs, _ := h.st.TaskRuns(id)
	h.t.Fatalf("task %s is %s (%s), want %v; runs %+v", id, task.State, task.Detail, states, runs)
	return task
}

type loggedRun struct {
	Args []string `json:"args"`
	Cwd  string   `json:"cwd"`
}

func (h *taskHarness) runs() []loggedRun {
	raw, _ := os.ReadFile(filepath.Join(h.logDir, "runs.jsonl"))
	var out []loggedRun
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		var r loggedRun
		if json.Unmarshal([]byte(line), &r) == nil {
			out = append(out, r)
		}
	}
	return out
}

func taskReservations(t *testing.T) []recordedRun {
	t.Helper()
	runs, err := loadRuns()
	if err != nil {
		t.Fatal(err)
	}
	var out []recordedRun
	for _, r := range runs {
		if r.TaskRunID != "" {
			out = append(out, r)
		}
	}
	return out
}

// waitNoTaskReservations waits for the client to retire the reservations: it
// does so just after the verified state is committed, so a test that saw the
// final state may still find them for a moment.
func waitNoTaskReservations(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		r := taskReservations(t)
		if len(r) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("reservations %+v", r)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func coordinatorEvents(t *testing.T, st *core.Store) []core.TaskEvent {
	t.Helper()
	events, _, err := st.TaskEvents("Game", core.AudienceCoordinator, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func hasArgs(args []string, want ...string) bool {
	joined := "\x00" + strings.Join(args, "\x00") + "\x00"
	return strings.Contains(joined, "\x00"+strings.Join(want, "\x00")+"\x00")
}

func TestTaskWorkerReportsAndIsVerified(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			h := newTaskHarness(t, "report", AdapterLauncher{AllowTasks: true})
			id := h.startTask("op-1", provider, "Fix the menu")
			task := h.waitState(id, "done")
			events := coordinatorEvents(t, h.st)
			last := events[len(events)-1]
			if last.Kind != "report" || last.Text != "done: Fix the menu" {
				t.Fatalf("events %+v", events)
			}
			runs := h.runs()
			if len(runs) != 1 || runs[0].Cwd != h.folder {
				t.Fatalf("runs %+v", runs)
			}
			if provider == "claude" && !hasArgs(runs[0].Args, "--session-id", task.SessionID) {
				t.Fatalf("Claude did not get its recorded id: %v", runs[0].Args)
			}
			if provider == "codex" && (task.SessionID == "" || hasArgs(runs[0].Args, "resume")) {
				t.Fatalf("Codex identity %q args %v", task.SessionID, runs[0].Args)
			}
			// Exit and an empty group were observed: the reservation is retired.
			waitNoTaskReservations(t)
			h.stop()
		})
	}
}

func TestTaskQuestionResumesTheSameConversation(t *testing.T) {
	for _, provider := range []string{"claude", "codex"} {
		t.Run(provider, func(t *testing.T) {
			h := newTaskHarness(t, "ask-then-report", AdapterLauncher{AllowTasks: true})
			id := h.startTask("op-1", provider, "Build level")
			task := h.waitState(id, "waiting_coordinator")
			if task.OpenQuestion == nil || task.OpenQuestion.Text != "Which level?" {
				t.Fatalf("question %+v", task.OpenQuestion)
			}
			// Waiting for an answer runs no process.
			time.Sleep(200 * time.Millisecond)
			if len(h.runs()) != 1 {
				t.Fatal("a run started while waiting")
			}
			if err := h.st.AnswerTask(id, task.OpenQuestion.ID, "ans-1", "Level 2", core.AudienceCoordinator); err != nil {
				t.Fatal(err)
			}
			if err := h.st.AnswerTask(id, task.OpenQuestion.ID, "ans-1", "Level 2", core.AudienceCoordinator); err != nil {
				t.Fatal(err)
			}
			done := h.waitState(id, "done")
			events := coordinatorEvents(t, h.st)
			if last := events[len(events)-1]; last.Text != "done with Level 2 for Which level?" {
				t.Fatalf("report %+v", last)
			}
			runs := h.runs()
			if len(runs) != 2 {
				t.Fatalf("runs %d", len(runs))
			}
			resumeFlag := []string{"--resume", done.SessionID}
			if provider == "codex" {
				resumeFlag = []string{"resume", done.SessionID}
			}
			if !hasArgs(runs[1].Args, resumeFlag...) {
				t.Fatalf("second run did not resume the same conversation: %v", runs[1].Args)
			}
			h.stop()
		})
	}
}

// The coordinator's model choice reaches the CLI on the first run and on
// the resumed run, so an answer never switches the worker to the default.
func TestTaskModelReachesEveryRun(t *testing.T) {
	for _, c := range []struct {
		provider, model string
		flag            []string
	}{
		{"claude", "opus", []string{"--model", "opus"}},
		{"codex", "gpt-5.1-codex", []string{"-c", `model="gpt-5.1-codex"`}},
	} {
		t.Run(c.provider, func(t *testing.T) {
			h := newTaskHarness(t, "ask-then-report", AdapterLauncher{AllowTasks: true})
			id, _, err := h.st.StartTask(core.StartTaskRequest{Project: "Game", OpID: "op-m", Provider: c.provider, Text: "Build level", Model: c.model})
			if err != nil {
				t.Fatal(err)
			}
			task := h.waitState(id, "waiting_coordinator")
			if err := h.st.AnswerTask(id, task.OpenQuestion.ID, "ans-1", "Level 2", core.AudienceCoordinator); err != nil {
				t.Fatal(err)
			}
			h.waitState(id, "done")
			runs := h.runs()
			if len(runs) != 2 || !hasArgs(runs[0].Args, c.flag...) || !hasArgs(runs[1].Args, c.flag...) {
				t.Fatalf("model missing from runs %+v", runs)
			}
			h.stop()
		})
	}
}

func TestTaskOwnerQuestionWaitsForTheOwner(t *testing.T) {
	h := newTaskHarness(t, "ask-owner", AdapterLauncher{AllowTasks: true})
	id := h.startTask("op-1", "claude", "Clean up")
	task := h.waitState(id, "waiting_owner")
	for _, e := range coordinatorEvents(t, h.st) {
		if e.Kind == "question" {
			t.Fatal("the coordinator got the owner's question")
		}
	}
	if err := h.st.AnswerTask(id, task.OpenQuestion.ID, "own-1", "yes", core.AudienceOwner); err != nil {
		t.Fatal(err)
	}
	h.waitState(id, "done")
	h.stop()
}

func TestTaskFailuresAreNotRetried(t *testing.T) {
	for _, mode := range []string{"exit1", "no-disposition", "wrong-session", "two-dispositions"} {
		t.Run(mode, func(t *testing.T) {
			h := newTaskHarness(t, mode, AdapterLauncher{AllowTasks: true})
			id := h.startTask("op-1", "claude", "x")
			want := "failed"
			if mode == "two-dispositions" {
				want = "done" // the second call is refused; the first report stands
			}
			h.waitState(id, want)
			time.Sleep(300 * time.Millisecond)
			if n := len(h.runs()); n != 1 {
				t.Fatalf("%d runs; nothing may be retried", n)
			}
			for _, e := range coordinatorEvents(t, h.st) {
				if e.Kind == "report" && want == "failed" {
					t.Fatal("unverified report published")
				}
			}
			waitNoTaskReservations(t)
			h.stop()
		})
	}
}

func TestTaskWithLeftoverProcessIsBlocked(t *testing.T) {
	old := taskGroupWait
	taskGroupWait = 500 * time.Millisecond
	t.Cleanup(func() { taskGroupWait = old })
	h := newTaskHarness(t, "leave-child", AdapterLauncher{AllowTasks: true})
	t.Cleanup(func() {
		if raw, err := os.ReadFile(filepath.Join(h.logDir, "child.pid")); err == nil {
			if pid, err := strconv.Atoi(string(raw)); err == nil {
				_ = syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	})
	id := h.startTask("op-1", "claude", "x")
	h.waitState(id, "blocked")
	for _, e := range coordinatorEvents(t, h.st) {
		if e.Kind == "report" {
			t.Fatal("report of a blocked run published")
		}
	}
	if r := taskReservations(t); len(r) != 1 {
		t.Fatalf("blocked run must keep its reservation: %+v", r)
	}
	h.stop()
}

func TestCancelStopsARunningTask(t *testing.T) {
	h := newTaskHarness(t, "slow", AdapterLauncher{AllowTasks: true})
	id := h.startTask("op-1", "claude", "x")
	h.waitState(id, "running")
	deadline := time.Now().Add(5 * time.Second)
	for len(h.runs()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if state, err := h.st.CancelTask(id); err != nil || (state != "cancelling" && state != "cancelled") {
		t.Fatalf("cancel %s %v", state, err)
	}
	h.waitState(id, "cancelled")
	waitNoTaskReservations(t)
	h.stop()
}

func TestTasksNeedPermissionAndAnOpenQueue(t *testing.T) {
	h := newTaskHarness(t, "report", AdapterLauncher{AllowModels: true})
	id := h.startTask("op-1", "claude", "x")
	time.Sleep(300 * time.Millisecond)
	if len(h.runs()) != 0 {
		t.Fatal("a worker started without --allow-tasks")
	}
	h.stop()
	if err := h.st.Pause(); err != nil {
		t.Fatal(err)
	}
	h.start(AdapterLauncher{AllowTasks: true})
	time.Sleep(300 * time.Millisecond)
	if len(h.runs()) != 0 {
		t.Fatal("a worker started while paused")
	}
	if r, _ := control.Call(h.root, control.Request{Cmd: "resume"}); !r.OK {
		t.Fatal("resume failed")
	}
	h.waitState(id, "done")
	h.stop()
}

func TestRestartInterruptsARunningTask(t *testing.T) {
	h := newTaskHarness(t, "slow", AdapterLauncher{AllowTasks: true})
	id := h.startTask("op-1", "claude", "x")
	deadline := time.Now().Add(5 * time.Second)
	for len(h.runs()) == 0 && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	h.stop()
	h.start(AdapterLauncher{AllowTasks: true})
	task := h.waitState(id, "interrupted")
	time.Sleep(300 * time.Millisecond)
	if len(h.runs()) != 1 {
		t.Fatal("interrupted task relaunched")
	}
	reservations := taskReservations(t)
	if len(reservations) != 1 {
		t.Fatalf("reservation of an interrupted run removed: %+v", reservations)
	}
	_ = syscall.Kill(-reservations[0].PGID, syscall.SIGKILL)
	_ = task
	h.stop()
}

// End to end on one computer: the coordinator's MCP server records a task,
// the client launches the worker, and wait returns the verified report.
func TestCoordinatorStartsATaskAndWaitsForItsReport(t *testing.T) {
	h := newTaskHarness(t, "ask-then-report", AdapterLauncher{AllowTasks: true})
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	go func() { _ = mcp.ServeCoordinator(h.st, "Game", inR, outW); outW.Close() }()
	defer inW.Close()
	lines := bufio.NewScanner(outR)
	lines.Buffer(make([]byte, 64*1024), 4*1024*1024)
	id := 0
	call := func(name string, args map[string]any) map[string]any {
		id++
		raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
		if _, err := inW.Write(append(raw, '\n')); err != nil {
			t.Fatal(err)
		}
		for lines.Scan() {
			var f struct {
				ID     int `json:"id"`
				Result struct {
					Content []struct{ Text string } `json:"content"`
					IsError bool                    `json:"isError"`
				} `json:"result"`
			}
			if json.Unmarshal(lines.Bytes(), &f) != nil || f.ID != id {
				continue
			}
			if f.Result.IsError || len(f.Result.Content) != 1 {
				t.Fatalf("%s failed: %+v", name, f.Result)
			}
			var out map[string]any
			if err := json.Unmarshal([]byte(f.Result.Content[0].Text), &out); err != nil {
				t.Fatal(err)
			}
			return out
		}
		t.Fatal("server closed")
		return nil
	}
	started := call("start_task", map[string]any{"op_id": "plan-1", "agent": "codex", "text": "Build the level"})
	task := started["task_id"].(string)
	w := call("wait", map[string]any{"after": 0, "timeout_seconds": 20})
	events := w["events"].([]any)
	question := events[0].(map[string]any)
	if question["kind"] != "question" || question["text"] != "Which level?" {
		t.Fatalf("first event %+v", question)
	}
	call("answer", map[string]any{"task_id": task, "question_id": question["question_id"], "op_id": "ans-1", "text": "Level 3"})
	w = call("wait", map[string]any{"after": w["cursor"], "timeout_seconds": 20})
	report := w["events"].([]any)[0].(map[string]any)
	if report["kind"] != "report" || report["text"] != "done with Level 3 for Which level?" {
		t.Fatalf("report %+v", report)
	}
	h.stop()
}
