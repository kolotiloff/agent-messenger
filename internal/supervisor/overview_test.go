package supervisor

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

func TestOverviewThroughSocketDoesNotLaunchOrConsumeQueue(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	s := mustStore(t, root)
	env := mkdir(t, filepath.Join(root, "cli"))
	if _, err := s.Register("synthetic", env, "private-conversation", "worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Enqueue("m1", "worker", "sender", "payload"); err != nil {
		t.Fatal(err)
	}
	if err := s.Pause(); err != nil {
		t.Fatal(err)
	}
	s.Close()
	launcher := &countingLauncher{enabled: true}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, launcher) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Error(err)
			}
		case <-time.After(5 * time.Second):
			t.Error("supervisor did not stop")
		}
	}()
	waitSocket(t, root)
	for i := 0; i < 3; i++ {
		r, err := control.Call(root, control.Request{Cmd: "overview"})
		if err != nil || !r.OK {
			t.Fatalf("overview: %+v %v", r, err)
		}
		var view core.Overview
		if err := json.Unmarshal(r.Overview, &view); err != nil {
			t.Fatal(err)
		}
		if !view.Paused || len(view.Sessions) != 1 || view.Sessions[0].Queued != 1 || view.Sessions[0].OwnerRetained {
			t.Fatalf("view: %+v", view)
		}
	}
	if launcher.calls() != 0 {
		t.Fatal("read triggered launcher")
	}
	r, err := control.Call(root, control.Request{Cmd: "inspect"})
	if err != nil || !r.OK {
		t.Fatalf("inspect: %+v %v", r, err)
	}
	var snapshot core.Snapshot
	if err := json.Unmarshal(r.Inspect, &snapshot); err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Attempts) != 0 || len(snapshot.Replies) != 0 || len(snapshot.Requests) != 1 {
		t.Fatal("read consumed queue")
	}
}
