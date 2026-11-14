package mcp

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

// T-120: the background watch exits when an event of its project arrives,
// and only then; another project's events do not wake it.
func TestCoordinatorWatchExitsOnAnEvent(t *testing.T) {
	old := coordinatorWatchPoll
	coordinatorWatchPoll = 20 * time.Millisecond
	defer func() { coordinatorWatchPoll = old }()
	st := newCoordinatorStore(t)
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() { done <- watchCoordinator(context.Background(), st, "Game", 0, &out) }()
	other, _, err := st.StartTask(core.StartTaskRequest{Project: "Other", OpID: "o", Provider: "claude", Text: "other work"})
	if err != nil {
		t.Fatal(err)
	}
	finishRun(t, st, other, core.DispositionAsk, "other question")
	select {
	case <-done:
		t.Fatalf("woken by another project: %s", out.String())
	case <-time.After(200 * time.Millisecond):
	}
	mine, _, _ := st.StartTask(core.StartTaskRequest{Project: "Game", OpID: "m", Provider: "claude", Text: "mine"})
	finishRun(t, st, mine, core.DispositionAsk, "a question")
	select {
	case err := <-done:
		if err != nil || !strings.Contains(out.String(), "new event") || !strings.Contains(out.String(), "after=0") {
			t.Fatalf("%v %q", err, out.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the watch did not exit on the event")
	}
	// With nothing new it ends when its time is up and says so.
	out.Reset()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, cursor, _ := st.TaskEvents("Game", core.AudienceCoordinator, 0, 100)
	if err := watchCoordinator(ctx, st, "Game", cursor, &out); err != nil || !strings.Contains(out.String(), "no new events") {
		t.Fatalf("%v %q", err, out.String())
	}
}

func TestCoordinatorWatchLineIsQuoted(t *testing.T) {
	t.Setenv(CoordinatorProgramVariable, "/Applications/Agent Messenger.app/Contents/MacOS/messenger")
	line := coordinatorWatchLine("/Users/o/data dir/queue.sqlite", "Game's")
	want := `'/Applications/Agent Messenger.app/Contents/MacOS/messenger' --root '/Users/o/data dir' coordinator-watch --project 'Game'\''s' --after CURSOR`
	if line != want {
		t.Fatalf("got  %s\nwant %s", line, want)
	}
}

// Row 28: clients cut long instructions short, so the watch command comes
// first and every wait and tasks result carries it with the cursor in place.
func TestWatchCommandComesFirstAndInResults(t *testing.T) {
	watch := "messenger coordinator-watch --project 'Game' --after CURSOR"
	if note := watchNote(watch); !strings.HasPrefix(note, "Background watch command") || !strings.Contains(note, watch) {
		t.Fatalf("note %q", note)
	}
	c := &coordinator{watch: watch, backgroundWatch: true}
	got := c.withWatch(map[string]any{"cursor": int64(42)}, 42)["watch"]
	if got != "messenger coordinator-watch --project 'Game' --after 42" {
		t.Fatalf("watch %v", got)
	}
	if _, ok := (&coordinator{}).withWatch(map[string]any{}, 1)["watch"]; ok {
		t.Fatal("watch without a command")
	}
}

// A Codex/unknown client must not be told a detached watch will wake its chat.
func TestCoordinatorWaitingMode(t *testing.T) {
	for _, name := range []string{"codex", "Codex Desktop", "cursor", "cursor-agent", "claude-code", "unknown", ""} {
		t.Run(name, func(t *testing.T) {
			st := newCoordinatorStore(t)
			watch := "messenger coordinator-watch --after CURSOR"
			client := startCoordinatorWithWatch(t, st, "Game", watch)
			client.send(1, "initialize", map[string]any{"clientInfo": map[string]any{"name": name}})
			response, _ := client.expect(1, 2*time.Second)
			instructions := response["result"].(map[string]any)["instructions"].(string)
			want := activeWaitInstructions
			prefix := want
			if name == "claude-code" {
				want = backgroundWaitInstructions
				prefix = watchNote(watch) + want
			}
			if !strings.HasPrefix(instructions, prefix) {
				t.Fatal("wrong initialize waiting guidance")
			}
			var tasks map[string]any
			if _, ok := client.tool("tasks", map[string]any{}, &tasks); !ok || tasks["waiting"] != want {
				t.Fatalf("wrong task waiting guidance: %v", tasks)
			}
			if (tasks["watch"] != nil) != (name == "claude-code") {
				t.Fatalf("wrong background watch availability: %v", tasks)
			}
			var waited map[string]any
			if _, ok := client.tool("wait", map[string]any{"after": tasks["cursor"], "timeout_seconds": 1}, &waited); !ok || waited["waiting"] != want {
				t.Fatalf("wrong wait guidance: %v", waited)
			}
		})
	}
	c := &coordinator{watch: "messenger coordinator-watch --after CURSOR"}
	if got := c.withWatch(map[string]any{}, 7); got["watch"] != nil || got["waiting"] != activeWaitInstructions {
		t.Fatalf("unknown client was offered background wake: %v", got)
	}
}

func TestCursorCoordinatorReceivesQuestionAndReport(t *testing.T) {
	st := newCoordinatorStore(t)
	c := startCoordinatorWithWatch(t, st, "Game", "messenger coordinator-watch --after CURSOR")
	c.send(1, "initialize", map[string]any{"clientInfo": map[string]any{"name": "cursor"}})
	c.expect(1, 2*time.Second)
	var started map[string]any
	if _, ok := c.tool("start_task", map[string]any{"op_id": "cursor-start", "agent": "codex", "text": "Build a fixture"}, &started); !ok || started["waiting"] != activeWaitInstructions {
		t.Fatal(started)
	}
	task := started["task_id"].(string)
	c.next++
	waitID := c.next
	c.send(waitID, "tools/call", map[string]any{"name": "wait", "arguments": map[string]any{"after": 0, "timeout_seconds": 30}})
	c.none(waitID, 100*time.Millisecond)
	finishRun(t, st, task, core.DispositionAsk, "Which fixture?")
	response, _ := c.expect(waitID, 3*time.Second)
	var question waitResult
	decodeTool(t, response, &question)
	if len(question.Events) != 1 || question.Events[0].Kind != "question" {
		t.Fatal(question)
	}
	var answered map[string]any
	if _, ok := c.tool("answer", map[string]any{"task_id": task, "question_id": question.Events[0].QuestionID, "op_id": "cursor-answer", "text": "The small fixture"}, &answered); !ok || answered["waiting"] != activeWaitInstructions {
		t.Fatal(answered)
	}
	finishRun(t, st, task, core.DispositionReport, "Fixture built")
	var report waitResult
	if _, ok := c.tool("wait", map[string]any{"after": question.Cursor, "timeout_seconds": 1}, &report); !ok || len(report.Events) != 1 || report.Events[0].Kind != "report" || report.Cursor <= question.Cursor {
		t.Fatal(report)
	}
}
