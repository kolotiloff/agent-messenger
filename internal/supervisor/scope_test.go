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

func TestScopedStreamLeavesOtherQueueUntouched(t *testing.T) {
	root, st := seedStreamClient(t, "normal")
	if _, err := st.Register("claude", filepath.Join(root, "profile"), "other-test", "other"); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ConfigureLaunch("other", exe, root); err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct{ id, alias string }{{"older-other", "other"}, {"chosen-one", "worker"}, {"chosen-two", "worker"}} {
		if _, err := st.Enqueue(m.id, m.alias, "user", m.id); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, root, AdapterLauncher{AllowModels: true, ClaudeStream: true, OnlySession: "worker"})
	}()
	waitSocket(t, root)
	snap := waitStreamState(t, st, func(s core.Snapshot) bool {
		return len(s.Replies) == 2 && len(s.StreamRuns) == 1 && s.StreamRuns[0]["state"] == "idle"
	})
	if len(snap.Attempts) != 2 || snap.Requests[0]["id"] != "older-other" || snap.Requests[0]["state"] != "queued" {
		t.Fatal("other queue changed")
	}
	if snap.Attempts[0]["pid"] != snap.Attempts[1]["pid"] {
		t.Fatal("second process used")
	}
	status, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || status.Status.LaunchSession != "worker" {
		t.Fatal("scope not reported", err)
	}
	streamControl(t, root, "shutdown")
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown blocked")
	}
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	done2 := make(chan error, 1)
	go func() {
		done2 <- Run(ctx2, root, AdapterLauncher{AllowModels: true, ClaudeStream: true, OnlySession: "worker"})
	}()
	waitSocket(t, root)
	time.Sleep(120 * time.Millisecond)
	after, _ := st.Snapshot()
	if len(after.Attempts) != 2 || after.Requests[0]["state"] != "queued" {
		t.Fatal("restart fell back to another session")
	}
	streamControl(t, root, "shutdown")
	select {
	case err := <-done2:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("restart shutdown blocked")
	}
}

func TestScopedAdapterRejectsWrongAliasBeforeExecution(t *testing.T) {
	l := AdapterLauncher{AllowModels: true, ClaudeStream: true, OnlySession: "chosen"}
	if _, err := l.Start(Spec{Alias: "other"}); err != errModelsNotAuthorized {
		t.Fatal("direct launch escaped scope", err)
	}
	if _, err := l.StartStream(context.Background(), Spec{Alias: "other"}, "", ""); err != errModelsNotAuthorized {
		t.Fatal("stream launch escaped scope", err)
	}
}
