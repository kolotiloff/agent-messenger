package mcp

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	root := t.TempDir()
	st := newCoordinatorStoreAt(t, root)
	reader, err := core.OpenTaskEventReader(filepath.Join(root, "queue.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	var out bytes.Buffer
	done := make(chan error, 1)
	watchCtx, cancelWatch := context.WithCancel(context.Background())
	defer cancelWatch()
	go func() { done <- watchCoordinator(watchCtx, reader.TaskEvents, "Game", 0, &out) }()
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
	if err := watchCoordinator(ctx, reader.TaskEvents, "Game", cursor, &out); err != nil || !strings.Contains(out.String(), "no new events") {
		t.Fatalf("%v %q", err, out.String())
	}
}

func TestCoordinatorWatchReadsThroughWALWriter(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "queue.sqlite")
	st := newCoordinatorStoreAt(t, root)
	task, _, err := st.StartTask(core.StartTaskRequest{Project: "Game", OpID: "work", Provider: "claude", Text: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	finishRun(t, st, task, core.DispositionReport, "committed before the other project wrote")
	lock := holdSQLiteLock(t, path, "BEGIN IMMEDIATE")
	defer lock()
	var out bytes.Buffer
	done := make(chan error, 1)
	go func() {
		done <- CoordinatorWatchMain(path, []string{"--project", "Game", "--after", "0"}, &out)
	}()
	select {
	case err := <-done:
		if err != nil || !strings.Contains(out.String(), "(report)") {
			t.Fatalf("watch: %v, %q", err, out.String())
		}
	case <-time.After(4 * time.Second):
		t.Fatal("could not read the committed event while the writer held its transaction")
	}
}

func TestCoordinatorWatchRetriesBusyWithoutMovingCursor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rollback.sqlite")
	db, err := sql.Open("sqlite", filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE fixture (id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	release := holdSQLiteLock(t, path, "BEGIN EXCLUSIVE")
	_, err = db.Exec(`SELECT id FROM fixture`)
	release()
	if !sqliteBusy(err) {
		t.Fatalf("expected a real SQLITE_BUSY, got %v", err)
	}
	busy := fmt.Errorf("read fixture: %w", err)
	old := coordinatorWatchPoll
	coordinatorWatchPoll = 10 * time.Millisecond
	defer func() { coordinatorWatchPoll = old }()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var out bytes.Buffer
	attempts := 0
	read := func(_ context.Context, project, audience string, after int64, _ int) ([]core.TaskEvent, int64, error) {
		if project != "Game" || audience != core.AudienceCoordinator || after != 42 {
			t.Fatalf("watch changed scope or cursor: %s %s %d", project, audience, after)
		}
		attempts++
		if attempts < 3 {
			return nil, after, busy
		}
		return []core.TaskEvent{{Seq: 51, Kind: "report"}}, 51, nil
	}
	if err := watchCoordinator(ctx, read, "Game", 42, &out); err != nil || attempts != 3 || !strings.Contains(out.String(), "after=42") {
		t.Fatalf("watch recovery: attempts %d, %v, %q", attempts, err, out.String())
	}
	out.Reset()
	ended, stop := context.WithCancel(context.Background())
	defer stop()
	read = func(context.Context, string, string, int64, int) ([]core.TaskEvent, int64, error) {
		stop()
		return nil, 42, busy
	}
	if err := watchCoordinator(ended, read, "Game", 42, &out); err != nil || !strings.Contains(out.String(), "could not check") || !strings.Contains(out.String(), "cursor 42") {
		t.Fatalf("watch ended during lock: %v, %q", err, out.String())
	}
}

func TestCoordinatorWatchDoesNotCreateAMissingDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.sqlite")
	var out bytes.Buffer
	if err := CoordinatorWatchMain(path, []string{"--project", "Game", "--after", "0"}, &out); err == nil || !strings.Contains(err.Error(), "read coordinator events") {
		t.Fatalf("expected a database read error, got %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing queue should not be created: %v", err)
	}
}

func holdSQLiteLock(t *testing.T, path, begin string) func() {
	t.Helper()
	db, err := sql.Open("sqlite", filepath.ToSlash(path))
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(begin); err != nil {
		db.Close()
		t.Fatal(err)
	}
	return func() {
		_, _ = db.Exec("ROLLBACK")
		_ = db.Close()
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
