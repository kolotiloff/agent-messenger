package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

type coordClient struct {
	t      *testing.T
	in     *io.PipeWriter
	frames chan map[string]any
	done   chan error
	next   int
}

func newCoordinatorStore(t *testing.T) *core.Store {
	t.Helper()
	old := core.TemporaryFolder
	core.TemporaryFolder = func(string) bool { return false }
	t.Cleanup(func() { core.TemporaryFolder = old })
	root := t.TempDir()
	st, err := core.Open(filepath.Join(root, "queue.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cli := filepath.Join(root, "agent")
	if err := os.WriteFile(cli, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Game", "Other"} {
		dir := filepath.Join(root, name)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := st.CreateProject(name, dir); err != nil {
			t.Fatal(err)
		}
	}
	for _, p := range []string{"claude", "codex"} {
		if err := st.SetComputerAgent(p, cli, "", "messenger-only"); err != nil {
			t.Fatal(err)
		}
	}
	return st
}

func startCoordinator(t *testing.T, st *core.Store, project string) *coordClient {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &coordClient{t: t, in: inW, frames: make(chan map[string]any, 100), done: make(chan error, 1)}
	go func() {
		c.done <- ServeCoordinator(st, project, inR, outW)
		outW.Close()
	}()
	go func() {
		scanner := bufio.NewScanner(outR)
		scanner.Buffer(make([]byte, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			var f map[string]any
			if json.Unmarshal(scanner.Bytes(), &f) == nil {
				c.frames <- f
			}
		}
		close(c.frames)
	}()
	t.Cleanup(func() {
		inW.Close()
		select {
		case <-c.done:
		case <-time.After(5 * time.Second):
			t.Error("server did not stop")
		}
	})
	return c
}

func (c *coordClient) send(id any, method string, params any) {
	c.t.Helper()
	msg := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	if id != nil {
		msg["id"] = id
	}
	raw, _ := json.Marshal(msg)
	if _, err := c.in.Write(append(raw, '\n')); err != nil {
		c.t.Fatal(err)
	}
}

// expect waits for the response with this id, collecting other frames.
func (c *coordClient) expect(id int, within time.Duration) (map[string]any, []map[string]any) {
	c.t.Helper()
	var others []map[string]any
	timer := time.After(within)
	for {
		select {
		case f, ok := <-c.frames:
			if !ok {
				c.t.Fatal("server closed")
			}
			if v, ok := f["id"].(float64); ok && int(v) == id {
				return f, others
			}
			others = append(others, f)
		case <-timer:
			c.t.Fatalf("no response %d within %v; other frames %v", id, within, others)
		}
	}
}

func (c *coordClient) none(id int, within time.Duration) {
	c.t.Helper()
	timer := time.After(within)
	for {
		select {
		case f := <-c.frames:
			if v, ok := f["id"].(float64); ok && int(v) == id {
				c.t.Fatalf("unexpected response %v", f)
			}
		case <-timer:
			return
		}
	}
}

// tool calls a tool and decodes its JSON text result.
func (c *coordClient) tool(name string, args any, out any) (string, bool) {
	c.t.Helper()
	c.next++
	id := c.next
	c.send(id, "tools/call", map[string]any{"name": name, "arguments": args})
	resp, _ := c.expect(id, 5*time.Second)
	return decodeTool(c.t, resp, out)
}

func decodeTool(t *testing.T, resp map[string]any, out any) (string, bool) {
	t.Helper()
	result, _ := resp["result"].(map[string]any)
	isErr, _ := result["isError"].(bool)
	content, _ := result["content"].([]any)
	if len(content) != 1 {
		t.Fatalf("result %v", resp)
	}
	text, _ := content[0].(map[string]any)["text"].(string)
	if out != nil && !isErr {
		if err := json.Unmarshal([]byte(text), out); err != nil {
			t.Fatalf("decode %q: %v", text, err)
		}
	}
	return text, !isErr
}

// finishRun drives the task's pending run to a verified disposition.
func finishRun(t *testing.T, st *core.Store, task, kind, text string) {
	t.Helper()
	runs, err := st.NextTaskRunIntents(50)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range runs {
		r, _ := st.TaskRun(id)
		if r.TaskID != task {
			continue
		}
		if _, _, err := st.BeginTaskRun(id, "res"); err != nil {
			t.Fatal(err)
		}
		if err := st.RecordTaskRunProcess(id, platform.Observation{PID: 9, PGID: 9, StartToken: "s", BootID: "b"}); err != nil {
			t.Fatal(err)
		}
		tk, _ := st.TaskByID(task)
		session := tk.SessionID
		if session == "" {
			session = "019a0000-0000-7000-8000-" + fmt.Sprintf("%012d", time.Now().UnixNano()%1e12)
		}
		if err := st.BindTaskSession(id, session); err != nil {
			t.Fatal(err)
		}
		if _, err := st.ReadTask(id, r.Receipt); err != nil {
			t.Fatal(err)
		}
		if err := st.RecordDisposition(id, r.Receipt, kind, text); err != nil {
			t.Fatal(err)
		}
		if _, err := st.FinishTaskRun(id, core.RunEvidence{ExitObserved: true, GroupEmpty: true, ProviderOK: true}); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("no pending run for %s", task)
}

type waitResult struct {
	Events   []core.TaskEvent `json:"events"`
	Cursor   int64            `json:"cursor"`
	TimedOut bool             `json:"timed_out"`
}

func TestCoordinatorHandshakeAndTools(t *testing.T) {
	st := newCoordinatorStore(t)
	c := startCoordinator(t, st, "Game")
	c.send(1, "initialize", map[string]any{"protocolVersion": "2025-06-18"})
	resp, _ := c.expect(1, 2*time.Second)
	result := resp["result"].(map[string]any)
	if !strings.Contains(result["instructions"].(string), "Project: Game") {
		t.Fatalf("instructions %v", result["instructions"])
	}
	if at, _ := st.CoordinatorLastSeen("Game"); at == "" {
		t.Fatal("connection not recorded")
	}
	c.send(2, "tools/list", map[string]any{})
	resp, _ = c.expect(2, 2*time.Second)
	var names []string
	for _, tool := range resp["result"].(map[string]any)["tools"].([]any) {
		names = append(names, tool.(map[string]any)["name"].(string))
	}
	if strings.Join(names, ",") != "list_agents,start_task,wait,tasks,answer,cancel" {
		t.Fatalf("tools %v", names)
	}
	var agents struct {
		Computers []core.ComputerSummary `json:"computers"`
	}
	if _, ok := c.tool("list_agents", map[string]any{}, &agents); !ok || len(agents.Computers) != 1 || len(agents.Computers[0].Agents) != 2 || agents.Computers[0].Folder == "" {
		t.Fatalf("agents %+v", agents)
	}
	if text, ok := c.tool("start_task", map[string]any{"op_id": "o1", "agent": "claude", "text": "x", "path": "/etc"}, nil); ok {
		t.Fatalf("unknown argument accepted: %s", text)
	}
}

func TestCoordinatorChoosesTheWorkerModel(t *testing.T) {
	st := newCoordinatorStore(t)
	c := startCoordinator(t, st, "Game")
	var started struct {
		TaskID string `json:"task_id"`
	}
	if _, ok := c.tool("start_task", map[string]any{"op_id": "op-m", "agent": "claude", "text": "Plan the release", "model": "opus"}, &started); !ok {
		t.Fatal("start with a model refused")
	}
	if task, err := st.TaskByID(started.TaskID); err != nil || task.Model != "opus" {
		t.Fatalf("model not recorded: %+v %v", task, err)
	}
	if _, ok := c.tool("start_task", map[string]any{"op_id": "op-x", "agent": "claude", "text": "x", "model": "--dangerously-skip-permissions"}, nil); ok {
		t.Fatal("flag-like model accepted")
	}
}

func TestCoordinatorStartIsIdempotentAndWaitReplays(t *testing.T) {
	st := newCoordinatorStore(t)
	c := startCoordinator(t, st, "Game")
	var started struct {
		TaskID  string `json:"task_id"`
		Created bool   `json:"created"`
	}
	if _, ok := c.tool("start_task", map[string]any{"op_id": "op-1", "agent": "codex", "text": "Build the level", "title": "Level"}, &started); !ok || !started.Created {
		t.Fatalf("start %+v", started)
	}
	first := started.TaskID
	if _, ok := c.tool("start_task", map[string]any{"op_id": "op-1", "agent": "codex", "text": "Build the level", "title": "Level"}, &started); !ok || started.Created || started.TaskID != first {
		t.Fatalf("repeat %+v", started)
	}
	// The start itself does not wake wait.
	var w waitResult
	if _, ok := c.tool("wait", map[string]any{"after": 0, "timeout_seconds": 1}, &w); !ok || !w.TimedOut || len(w.Events) != 0 {
		t.Fatalf("wait %+v", w)
	}
	// An event before waiting is returned at once.
	finishRun(t, st, first, core.DispositionAsk, "Which level?")
	if _, ok := c.tool("wait", map[string]any{"after": 0}, &w); !ok || len(w.Events) != 1 || w.Events[0].Kind != "question" {
		t.Fatalf("wait %+v", w)
	}
	question := w.Events[0].QuestionID
	cursor := w.Cursor
	// Not passing the cursor replays: nothing is lost.
	var again waitResult
	c.tool("wait", map[string]any{"timeout_seconds": 1}, &again)
	if len(again.Events) != 1 || again.Events[0].Seq != w.Events[0].Seq {
		t.Fatalf("replay %+v", again)
	}
	if _, ok := c.tool("answer", map[string]any{"task_id": first, "question_id": question, "op_id": "a-1", "text": "Level 2"}, nil); !ok {
		t.Fatal("answer failed")
	}
	if _, ok := c.tool("answer", map[string]any{"task_id": first, "question_id": question, "op_id": "a-1", "text": "Level 2"}, nil); !ok {
		t.Fatal("repeated answer failed")
	}
	// A wait that blocks wakes when the report arrives.
	c.next++
	waitID := c.next
	c.send(waitID, "tools/call", map[string]any{"name": "wait", "arguments": map[string]any{"after": cursor, "timeout_seconds": 30}})
	c.none(waitID, 300*time.Millisecond)
	finishRun(t, st, first, core.DispositionReport, "Level 2 built")
	resp, _ := c.expect(waitID, 3*time.Second)
	var woke waitResult
	decodeTool(t, resp, &woke)
	if len(woke.Events) != 1 || woke.Events[0].Kind != "report" || woke.Events[0].Text != "Level 2 built" || woke.Cursor <= cursor {
		t.Fatalf("woke %+v", woke)
	}
	var list struct {
		Tasks  []coordinatorTask `json:"tasks"`
		Cursor int64             `json:"cursor"`
	}
	if _, ok := c.tool("tasks", map[string]any{}, &list); !ok || len(list.Tasks) != 1 || list.Tasks[0].State != "done" || list.Cursor < woke.Cursor {
		t.Fatalf("tasks %+v", list)
	}
}

func TestCoordinatorWaitIsCancellableAndConcurrent(t *testing.T) {
	old := coordinatorProgress
	coordinatorProgress = 200 * time.Millisecond
	t.Cleanup(func() { coordinatorProgress = old })
	st := newCoordinatorStore(t)
	c := startCoordinator(t, st, "Game")
	c.send(100, "tools/call", map[string]any{"name": "wait", "arguments": map[string]any{"after": 0, "timeout_seconds": 30}, "_meta": map[string]any{"progressToken": "p1"}})
	// Other calls are answered while wait blocks.
	c.send(101, "ping", map[string]any{})
	_, others := c.expect(101, 2*time.Second)
	_ = others
	time.Sleep(700 * time.Millisecond)
	c.send(nil, "notifications/cancelled", map[string]any{"requestId": 100})
	progress := 0
	timer := time.After(500 * time.Millisecond)
loop:
	for {
		select {
		case f := <-c.frames:
			if f["method"] == "notifications/progress" {
				progress++
			}
			if v, ok := f["id"].(float64); ok && int(v) == 100 {
				t.Fatal("a cancelled wait was answered")
			}
		case <-timer:
			break loop
		}
	}
	if progress == 0 {
		t.Fatal("no progress notifications")
	}
	// The event that arrives after the cancel is delivered by the next wait.
	var started struct {
		TaskID string `json:"task_id"`
	}
	c.tool("start_task", map[string]any{"op_id": "op-1", "agent": "claude", "text": "x"}, &started)
	finishRun(t, st, started.TaskID, core.DispositionReport, "done")
	var w waitResult
	if _, ok := c.tool("wait", map[string]any{"after": 0, "timeout_seconds": 2}, &w); !ok || len(w.Events) != 1 {
		t.Fatalf("after cancel %+v", w)
	}
}

func TestCoordinatorSeesOnlyItsProject(t *testing.T) {
	st := newCoordinatorStore(t)
	other, _, err := st.StartTask(core.StartTaskRequest{Project: "Other", OpID: "o", Provider: "claude", Text: "other work"})
	if err != nil {
		t.Fatal(err)
	}
	finishRun(t, st, other, core.DispositionAsk, "other question")
	c := startCoordinator(t, st, "Game")
	var w waitResult
	c.tool("wait", map[string]any{"after": 0, "timeout_seconds": 1}, &w)
	if len(w.Events) != 0 {
		t.Fatalf("saw another project's events: %+v", w.Events)
	}
	var list struct {
		Tasks []coordinatorTask `json:"tasks"`
	}
	c.tool("tasks", map[string]any{}, &list)
	if len(list.Tasks) != 0 {
		t.Fatalf("saw another project's tasks: %+v", list.Tasks)
	}
	tk, _ := st.TaskByID(other)
	if _, ok := c.tool("answer", map[string]any{"task_id": other, "question_id": tk.OpenQuestion.ID, "op_id": "x", "text": "y"}, nil); ok {
		t.Fatal("answered another project's task")
	}
	if _, ok := c.tool("cancel", map[string]any{"task_id": other}, nil); ok {
		t.Fatal("cancelled another project's task")
	}
	// An owner question of this project does not reach the coordinator.
	mine, _, _ := st.StartTask(core.StartTaskRequest{Project: "Game", OpID: "m", Provider: "claude", Text: "mine"})
	finishRun(t, st, mine, core.DispositionAskOwner, "for the owner")
	c.tool("wait", map[string]any{"after": 0, "timeout_seconds": 1}, &w)
	if len(w.Events) != 0 {
		t.Fatalf("owner question reached the coordinator: %+v", w.Events)
	}
}

func TestCoordinatorUnknownProject(t *testing.T) {
	st := newCoordinatorStore(t)
	c := startCoordinator(t, st, "Missing")
	if text, ok := c.tool("list_agents", map[string]any{}, nil); ok || !strings.Contains(text, "Новый проект") {
		t.Fatalf("unknown project: %s", text)
	}
}
