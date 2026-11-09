package mcp

import (
	"errors"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// T-112: the coordinator reads where a cut worker stopped.
func TestTaskMessagesOfAnInterruptedTask(t *testing.T) {
	st := newCoordinatorStore(t)
	id, _, err := st.StartTask(core.StartTaskRequest{Project: "Game", OpID: "m", Provider: "claude", Text: "Do two things"})
	if err != nil {
		t.Fatal(err)
	}
	runs, _ := st.NextTaskRunIntents(10)
	run := runs[0]
	if _, _, err := st.BeginTaskRun(run, "res"); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordTaskRunProcess(run, platform.Observation{PID: 9, PGID: 9, StartToken: "s", BootID: "b"}); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"first item committed", strings.Repeat("x", 5000), "working on the second item"} {
		if err := st.AppendTaskRunItem(run, "text", "", text); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.InterruptInheritedTaskRuns(); err != nil {
		t.Fatal(err)
	}
	c := startCoordinator(t, st, "Game")
	var got struct {
		State    string               `json:"state"`
		Total    int                  `json:"total"`
		Note     string               `json:"note"`
		Messages []coordinatorMessage `json:"messages"`
	}
	if _, ok := c.tool("task_messages", map[string]any{"task_id": id, "last": 3}, &got); !ok {
		t.Fatal("task_messages failed")
	}
	if got.State != "interrupted" || len(got.Messages) != 3 || got.Total < 5 || got.Note == "" {
		t.Fatalf("%+v", got)
	}
	last := got.Messages[2]
	if last.Kind != "outcome" || !strings.Contains(last.Text, "restarted") {
		t.Fatalf("last message %+v", last)
	}
	if got.Messages[1].Text != "working on the second item" || len([]rune(got.Messages[0].Text)) > coordinatorMessageText {
		t.Fatalf("messages %+v", got.Messages[:2])
	}
	if _, ok := c.tool("task_messages", map[string]any{"task_id": "nope"}, nil); ok {
		t.Fatal("read an unknown task")
	}
}

// T-118, D-108: an interrupted task continues only once its worker is
// certainly gone; otherwise it stays as it is.
func TestContinueInterruptedTask(t *testing.T) {
	st := newCoordinatorStore(t)
	id, _, err := st.StartTask(core.StartTaskRequest{Project: "Game", OpID: "m", Provider: "claude", Text: "Do two things"})
	if err != nil {
		t.Fatal(err)
	}
	runs, _ := st.NextTaskRunIntents(10)
	run := runs[0]
	if _, _, err := st.BeginTaskRun(run, "res"); err != nil {
		t.Fatal(err)
	}
	if err := st.RecordTaskRunProcess(run, platform.Observation{PID: 9, PGID: 9, StartToken: "s", BootID: "b"}); err != nil {
		t.Fatal(err)
	}
	session := "019a0000-0000-7000-8000-000000000001"
	if tk, _ := st.TaskByID(id); tk.SessionID != "" {
		session = tk.SessionID
	}
	if err := st.BindTaskSession(run, session); err != nil {
		t.Fatal(err)
	}
	if err := st.InterruptInheritedTaskRuns(); err != nil {
		t.Fatal(err)
	}
	still := func(platform.Observation) (string, error) { return "", errors.New("still running") }
	if err := st.ContinueTaskChecked(id, "c1", "continue; item 1 is committed", core.AudienceCoordinator, still); err == nil {
		t.Fatal("continued while the worker may run")
	}
	if tk, _ := st.TaskByID(id); tk.State != "interrupted" {
		t.Fatalf("state %s", tk.State)
	}
	if err := st.ContinueTask(id, "c1", "continue", core.AudienceCoordinator); err == nil {
		t.Fatal("plain continue took an interrupted task")
	}
	// The observation names another boot, so the computer restarted since.
	c := startCoordinator(t, st, "Game")
	if text, ok := c.tool("continue_task", map[string]any{"task_id": id, "op_id": "c2", "text": "continue; item 1 is committed"}, nil); !ok {
		t.Fatalf("continue_task: %s", text)
	}
	tk, _ := st.TaskByID(id)
	if tk.State != "running" {
		t.Fatalf("state %s", tk.State)
	}
	old, _ := st.TaskRun(run)
	if old.State != "failed" || !strings.Contains(old.Detail, "closed for a follow-up") {
		t.Fatalf("old run %+v", old)
	}
}
