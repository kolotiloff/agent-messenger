package main

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// fakeClient answers the control socket like a background client.
type fakeClient struct {
	control.Handler
	ln          net.Listener
	paused      bool
	refuse      bool // Shutdown fails
	linger      bool // Shutdown succeeds but the socket keeps answering
	shutdowns   atomic.Int32
	resumes     atomic.Int32
	closeOnce   sync.Once
	version     string
	statusCalls atomic.Int32
}

func (f *fakeClient) Status() (control.Status, error) {
	f.statusCalls.Add(1)
	return control.Status{Paused: f.paused, Version: f.version}, nil
}
func (f *fakeClient) Resume() error { f.resumes.Add(1); return nil }
func (f *fakeClient) Shutdown() error {
	f.shutdowns.Add(1)
	if f.refuse {
		return errors.New("/private/path stream stop failed")
	}
	if !f.linger {
		go func() { time.Sleep(20 * time.Millisecond); f.stop() }()
	}
	return nil
}
func (f *fakeClient) stop() { f.closeOnce.Do(func() { _ = f.ln.Close() }) }

func serveFake(t *testing.T, root string, f *fakeClient) *fakeClient {
	t.Helper()
	ln, err := control.Listen(root)
	if err != nil {
		t.Fatal(err)
	}
	f.ln = ln
	go func() { _ = control.Serve(ln, f) }()
	t.Cleanup(f.stop)
	return f
}

func shortRoot(t *testing.T) string {
	t.Helper()
	// Unix socket paths are short; t.TempDir can be too long on macOS.
	root, err := os.MkdirTemp("", "am")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	return root
}

func TestRestartClientReplacesTheOldClientAndKeepsItRunning(t *testing.T) {
	root := shortRoot(t)
	old := serveFake(t, root, &fakeClient{})
	var fresh *fakeClient
	starts := 0
	err := restartClient(context.Background(), root, func() error {
		starts++
		if old.statusCalls.Load() == 0 || old.shutdowns.Load() != 1 {
			t.Error("started before the old client was asked to stop")
		}
		fresh = serveFake(t, root, &fakeClient{paused: true})
		return nil
	}, 100*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if starts != 1 || fresh.resumes.Load() != 1 {
		t.Fatalf("starts=%d resumes=%d", starts, fresh.resumes.Load())
	}
}

func TestRestartClientKeepsTheOwnersPause(t *testing.T) {
	root := shortRoot(t)
	serveFake(t, root, &fakeClient{paused: true})
	var fresh *fakeClient
	err := restartClient(context.Background(), root, func() error {
		fresh = serveFake(t, root, &fakeClient{paused: true})
		return nil
	}, 100*time.Millisecond)
	if err != nil || fresh.resumes.Load() != 0 {
		t.Fatalf("err=%v resumes=%d", err, fresh.resumes.Load())
	}
}

func TestRestartClientStartsNothingWhenTheOldClientStays(t *testing.T) {
	for name, f := range map[string]*fakeClient{"refused": {refuse: true}, "lingers": {linger: true}} {
		t.Run(name, func(t *testing.T) {
			root := shortRoot(t)
			serveFake(t, root, f)
			err := restartClient(context.Background(), root, func() error {
				t.Error("a second client was started")
				return nil
			}, 40*time.Millisecond)
			if err == nil || strings.Contains(err.Error(), "/private") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestRestartClientRefusesWithoutAnAnsweringClient(t *testing.T) {
	err := restartClient(context.Background(), shortRoot(t), func() error {
		t.Error("started without a client to replace")
		return nil
	}, 40*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "Ничего не остановлено") {
		t.Fatalf("err=%v", err)
	}
}

func TestRestartClientWaitsForTheLockOnly(t *testing.T) {
	root := shortRoot(t)
	serveFake(t, root, &fakeClient{})
	starts := 0
	err := restartClient(context.Background(), root, func() error {
		starts++
		if starts < 3 {
			return platform.ErrAlreadyRunning
		}
		serveFake(t, root, &fakeClient{paused: true})
		return nil
	}, 40*time.Millisecond)
	if err != nil || starts != 3 {
		t.Fatalf("err=%v starts=%d", err, starts)
	}

	root = shortRoot(t)
	serveFake(t, root, &fakeClient{})
	starts = 0
	err = restartClient(context.Background(), root, func() error {
		starts++
		return errors.New("handshake failed: /private/log")
	}, 40*time.Millisecond)
	if err == nil || starts != 1 || strings.Contains(err.Error(), "/private") {
		t.Fatalf("other start errors must not be retried: err=%v starts=%d", err, starts)
	}
}
