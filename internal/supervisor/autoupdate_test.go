package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

// waitAutoLook waits until the background look for a release has ended.
func waitAutoLook(t *testing.T, s *Supervisor) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		s.auto.mu.Lock()
		looking := s.auto.looking
		s.auto.mu.Unlock()
		if !looking {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("look did not end")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// lookAgain makes the next round look for a release again.
func lookAgain(s *Supervisor) {
	s.auto.mu.Lock()
	s.auto.looked = time.Time{}
	s.auto.mu.Unlock()
}

func autoView(t *testing.T, s *Supervisor, action string, on bool) (serverView, error) {
	t.Helper()
	raw, _ := json.Marshal(map[string]bool{"on": on})
	v, err := s.Server(context.Background(), action, raw)
	if err != nil {
		return serverView{}, err
	}
	return v.(serverView), nil
}

// T-277: with automatic updates on, the client queues one update for each
// newer release, waits for running work like any queued update, and never
// queues a release again after its update failed.
func TestAutoUpdateQueuesEachReleaseOnce(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	oldOS := updateOS
	updateOS = "linux"
	t.Cleanup(func() { updateOS = oldOS })
	s, _ := netComputer(t, ctx, "solo")
	program := filepath.Join(s.root, "messenger")
	if err := os.WriteFile(program, []byte("program"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteProgramPath(s.root, program); err != nil {
		t.Fatal(err)
	}
	var started []RemoteUpdate
	s.startUpdate = func(_ string, job RemoteUpdate) error {
		started = append(started, job)
		return nil
	}
	var mu sync.Mutex
	looks, release := 0, "0.0.0-preview.99"
	var lookErr error
	s.findUpdate = func(context.Context) (string, error) {
		mu.Lock()
		defer mu.Unlock()
		looks++
		return release, lookErr
	}
	round := func() {
		t.Helper()
		s.networkRound(ctx)
		waitAutoLook(t, s)
		s.networkRound(ctx)
	}

	// Off by default: nothing is looked for or queued.
	v, err := autoView(t, s, "status", false)
	if err != nil || v.AutoUpdate || !v.AutoUpdateSupported {
		t.Fatalf("status %+v %v", v, err)
	}
	round()
	if looks != 0 || len(started) != 0 {
		t.Fatalf("looked %d, started %+v while off", looks, started)
	}

	if v, err = autoView(t, s, "auto_update", true); err != nil || !v.AutoUpdate {
		t.Fatalf("turn on %+v %v", v, err)
	}
	task, _, err := s.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "op-1", Provider: "codex", Text: "работа"})
	if err != nil {
		t.Fatal(err)
	}
	round()
	if looks != 1 || len(started) != 0 {
		t.Fatalf("looked %d, started %+v while a task ran", looks, started)
	}
	if pending, _ := s.store.HasPendingUpdate(); !pending {
		t.Fatal("release not queued")
	}
	if _, _, err := s.store.StartTask(core.StartTaskRequest{Project: "Game", OpID: "op-new", Provider: "codex", Text: "ещё"}); err == nil {
		t.Fatal("new task accepted while the update waits")
	}
	// The look is not repeated before its interval.
	s.networkRound(ctx)
	if looks != 1 {
		t.Fatalf("looked again at once: %d", looks)
	}
	if _, err := s.store.CancelTask(task); err != nil {
		t.Fatal(err)
	}
	s.networkRound(ctx)
	if len(started) != 1 || started[0].Broker != "" {
		t.Fatalf("helper %+v", started)
	}

	// The update failed: the same release is never queued again on its own.
	job := started[0]
	job.State, job.Text = core.UpdateFailed, "сбой"
	if err := FinishRemoteUpdate(s.root, job); err != nil {
		t.Fatal(err)
	}
	s.networkRound(ctx)
	if u, ok, _ := s.store.LocalUpdate(); !ok || u.State != core.UpdateFailed {
		t.Fatalf("local result %+v", u)
	}
	lookAgain(s)
	round()
	if looks != 2 || len(started) != 1 {
		t.Fatalf("same release again: looked %d, started %d", looks, len(started))
	}
	if pending, _ := s.store.HasPendingUpdate(); pending {
		t.Fatal("same release queued again")
	}

	// A failed look changes nothing.
	mu.Lock()
	release, lookErr = "0.0.0-preview.100", errors.New("offline")
	mu.Unlock()
	lookAgain(s)
	round()
	if len(started) != 1 {
		t.Fatalf("queued after a failed look: %+v", started)
	}

	// A newer release is queued once more.
	mu.Lock()
	lookErr = nil
	mu.Unlock()
	lookAgain(s)
	round()
	if len(started) != 2 {
		t.Fatalf("newer release not installed: %+v", started)
	}

	// Turned off: nothing more is looked for.
	if v, err = autoView(t, s, "auto_update", false); err != nil || v.AutoUpdate {
		t.Fatalf("turn off %+v %v", v, err)
	}
	before := looks
	lookAgain(s)
	round()
	if looks != before {
		t.Fatalf("looked while off: %d", looks-before)
	}
}

// T-277: a Mac, or a client that cannot update itself, refuses to turn
// automatic updates on and never looks.
func TestAutoUpdateRefusedWhereUnsupported(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	oldOS := updateOS
	t.Cleanup(func() { updateOS = oldOS })
	s, _ := netComputer(t, ctx, "mac")
	s.startUpdate = func(string, RemoteUpdate) error { return nil }
	looks := 0
	s.findUpdate = func(context.Context) (string, error) { looks++; return "0.0.0-preview.99", nil }
	for _, os := range []string{"darwin", "linux"} {
		updateOS = os
		if os == "linux" {
			s.findUpdate = nil
		}
		if _, err := autoView(t, s, "auto_update", true); err == nil {
			t.Fatalf("%s: turned on", os)
		}
		v, err := autoView(t, s, "status", false)
		if err != nil || v.AutoUpdate || v.AutoUpdateSupported {
			t.Fatalf("%s: status %+v %v", os, v, err)
		}
	}
	// Even a setting left on from before is ignored there.
	if err := s.store.SetAutoUpdate(true); err != nil {
		t.Fatal(err)
	}
	updateOS = "darwin"
	s.findUpdate = func(context.Context) (string, error) { looks++; return "0.0.0-preview.99", nil }
	s.networkRound(ctx)
	waitAutoLook(t, s)
	if looks != 0 {
		t.Fatalf("looked on a Mac: %d", looks)
	}
}
