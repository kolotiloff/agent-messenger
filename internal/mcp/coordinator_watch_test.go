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
