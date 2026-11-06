package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

func TestWindowAuthorizationIsScopedIdempotentAndNotRestored(t *testing.T) {
	root, st := seedStreamClient(t, "normal")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetupNewClaude(filepath.Join(root, "profile"), "a78f1366-38e7-4cec-b0b2-98670cfd79b6", "chosen", exe, root); err != nil {
		t.Fatal(err)
	}
	for _, m := range []struct{ id, alias string }{{"older-other", "worker"}, {"chosen-one", "chosen"}, {"chosen-two", "chosen"}} {
		if _, err := st.Enqueue(m.id, m.alias, "user", m.id); err != nil {
			t.Fatal(err)
		}
	}
	view, err := st.Overview()
	if err != nil || len(view.Sessions) != 2 || view.Sessions[0].CanLaunchTest || !view.Sessions[1].CanLaunchTest {
		t.Fatal("launch eligibility incorrect", view, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, AdapterLauncher{ClaudeStream: true}) }()
	waitSocket(t, root)
	for _, r := range []control.Request{{Cmd: "authorize-setup", Alias: "chosen"}, {Cmd: "authorize-setup", Alias: "worker", Confirmation: "allow-model-calls"}} {
		response, err := control.Call(root, r)
		if err != nil || response.OK {
			t.Fatal("invalid authorization accepted", response, err)
		}
	}
	snap, _ := st.Snapshot()
	if len(snap.Attempts) != 0 {
		t.Fatal("unapproved model start")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, e := control.Call(root, control.Request{Cmd: "authorize-setup", Alias: "chosen", Confirmation: "allow-model-calls"})
			if e != nil || !r.OK {
				t.Error("authorization failed", r, e)
			}
		}()
	}
	wg.Wait()
	snap = waitStreamState(t, st, func(s core.Snapshot) bool {
		return len(s.Replies) == 2 && len(s.StreamRuns) == 1 && s.StreamRuns[0]["state"] == "idle"
	})
	if len(snap.Attempts) != 2 || snap.Requests[0]["state"] != "queued" || snap.Attempts[0]["pid"] != snap.Attempts[1]["pid"] {
		t.Fatal("authorization escaped selected session")
	}
	count := 0
	for _, e := range snap.Events {
		if e["kind"] == "authorize_setup" {
			count++
		}
	}
	if count != 1 {
		t.Fatal("authorization duplicated", count)
	}
	streamControl(t, root, "pause")
	r, e := control.Call(root, control.Request{Cmd: "authorize-setup", Alias: "chosen", Confirmation: "allow-model-calls"})
	if e != nil || !r.OK {
		t.Fatal(r, e)
	}
	r, e = control.Call(root, control.Request{Cmd: "status"})
	if e != nil || !r.Status.Paused || r.Status.LaunchSession != "chosen" {
		t.Fatal("ACK retry resumed paused work", r, e)
	}
	r, e = control.Call(root, control.Request{Cmd: "authorize-setup", Alias: "worker", Confirmation: "allow-model-calls"})
	if e != nil || r.OK {
		t.Fatal("permission widened", r, e)
	}
	streamControl(t, root, "shutdown")
	select {
	case e := <-done:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown blocked")
	}
	_, again := startStreamClient(t, root, false)
	r, e = control.Call(root, control.Request{Cmd: "status"})
	if e != nil || r.Status.CLILaunch != "not-authorized" || r.Status.LaunchSession != "" {
		t.Fatal("permission restored", r, e)
	}
	r, e = control.Call(root, control.Request{Cmd: "authorize-setup", Alias: "chosen", Confirmation: "allow-model-calls"})
	if e != nil || r.OK {
		t.Fatal("held session reauthorized", r, e)
	}
	streamControl(t, root, "shutdown")
	select {
	case e := <-again:
		if e != nil {
			t.Fatal(e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("second shutdown blocked")
	}
}
