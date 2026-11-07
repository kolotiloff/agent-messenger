//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestMain(m *testing.M) {
	if os.Getenv("MESSENGER_BACKGROUND_TEST_BINARY") == "1" && len(os.Args) > 1 && os.Args[1] == "--root" {
		main()
		return
	}
	os.Exit(m.Run())
}

func windowsTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("LocalAppData", filepath.Join(home, "local"))
	t.Setenv("AppData", filepath.Join(home, "roaming"))
	t.Setenv("MESSENGER_BACKGROUND_TEST_BINARY", "1")
	return filepath.Join(home, "queue")
}

func windowsCall(t *testing.T, root string, r control.Request) control.Response {
	t.Helper()
	response, err := control.Call(root, r)
	if err != nil || !response.OK {
		t.Fatalf("%s: %v %s", r.Cmd, err, response.Error)
	}
	return response
}

// The background client starts detached, paused, answers on its socket,
// refuses a second instance, and releases the lock after shutdown.
func TestWindowsBackgroundStartAndShutdown(t *testing.T) {
	root := windowsTestHome(t)
	pid, logPath, err := startBackground(root, []string{"--allow-tasks"})
	if err != nil || pid <= 0 {
		if raw, readErr := os.ReadFile(logPath); readErr == nil {
			t.Log(string(raw))
		}
		t.Fatal(pid, err)
	}
	live := true
	t.Cleanup(func() {
		if live {
			_, _ = control.Call(root, control.Request{Cmd: "shutdown"})
		}
	})
	status := windowsCall(t, root, control.Request{Cmd: "status"}).Status
	if status.Supervisor != pid || !status.Paused {
		t.Fatalf("status %+v", status)
	}
	if _, _, err := startBackground(root, nil); err == nil {
		t.Fatal("second instance accepted")
	}
	windowsCall(t, root, control.Request{Cmd: "resume"})
	windowsCall(t, root, control.Request{Cmd: "shutdown"})
	live = false
	path, err := platform.SupervisorLockFile()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		lock, err := platform.Acquire(path)
		if err == nil {
			lock.Release()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("background client did not release its lock")
		}
		time.Sleep(20 * time.Millisecond)
	}
}
