//go:build darwin || linux

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

func backgroundTestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	t.Setenv("MESSENGER_BACKGROUND_TEST_BINARY", "1")
	return filepath.Join(home, "queue")
}

func backgroundCall(t *testing.T, root string, r control.Request) control.Response {
	t.Helper()
	response, err := control.Call(root, r)
	if err != nil || !response.OK {
		t.Fatalf("%s: %v %s", r.Cmd, err, response.Error)
	}
	return response
}

func stopBackground(t *testing.T, root string) {
	t.Helper()
	backgroundCall(t, root, control.Request{Cmd: "shutdown"})
	path, err := platform.SupervisorLockFile()
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		lock, err := platform.Acquire(path)
		if err == nil {
			lock.Release()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background supervisor did not release lock")
}

func TestBackgroundStartReconnectAndExplicitAuthorization(t *testing.T) {
	root := backgroundTestHome(t)
	pid, log, err := startBackground(root, []string{"--claude-stream"})
	if err != nil || pid <= 1 {
		t.Fatal(pid, err)
	}
	live := true
	t.Cleanup(func() {
		if live {
			stopBackground(t, root)
		}
	})
	info, err := os.Stat(log)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatal("log permissions", info, err)
	}
	status := backgroundCall(t, root, control.Request{Cmd: "status"}).Status
	if status.Supervisor != pid || !status.Paused || status.CLILaunch != "not-authorized" {
		t.Fatal("unexpected detached status", status)
	}
	backgroundCall(t, root, control.Request{Cmd: "register-session", Alias: "test", Provider: "claude",
		Environment: root, ConversationID: "12345678-1234-1234-1234-123456789abc", CLI: "/usr/bin/false", Workspace: root})
	backgroundCall(t, root, control.Request{Cmd: "enqueue", Alias: "test", MessageID: "m1", Sender: "tester", Text: "must stay queued"})
	backgroundCall(t, root, control.Request{Cmd: "resume"})
	time.Sleep(120 * time.Millisecond)
	status = backgroundCall(t, root, control.Request{Cmd: "status"}).Status
	if status.Held != 0 || status.Queued != 1 || status.Paused {
		t.Fatal("default startup invoked provider", status)
	}
	// A second start must fail rather than attach with different permissions,
	// pause the existing instance, or report its PID as a new successful launch.
	if _, _, err := startBackground(root, []string{"--allow-model-calls"}); err == nil {
		t.Fatal("second instance accepted")
	}
	if _, _, err := startBackground(filepath.Join(filepath.Dir(root), "other"), nil); err == nil {
		t.Fatal("other root bypassed global owner")
	}
	status = backgroundCall(t, root, control.Request{Cmd: "status"}).Status
	if status.Supervisor != pid || status.Paused || status.CLILaunch != "not-authorized" {
		t.Fatal("second start changed live controller", status)
	}
	stopBackground(t, root)
	live = false
	pid, _, err = startBackground(root, []string{"--claude-stream", "--allow-model-calls"})
	if err != nil {
		t.Fatal(err)
	}
	live = true
	status = backgroundCall(t, root, control.Request{Cmd: "status"}).Status
	if status.Supervisor != pid || !status.Paused || status.Queued != 1 || status.Held != 0 || status.CLILaunch == "not-authorized" {
		t.Fatal("authorized background start did not preserve paused queue", status)
	}
	time.Sleep(120 * time.Millisecond)
	status = backgroundCall(t, root, control.Request{Cmd: "status"}).Status
	if status.Held != 0 || status.Queued != 1 {
		t.Fatal("model launched before explicit resume", status)
	}
}

func TestBackgroundParentDisappearsBeforeCommit(t *testing.T) {
	root := backgroundTestHome(t)
	conn, child, err := startupPair()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	defer child.Close()
	executable, _ := os.Executable()
	cmd := exec.Command(executable, "--root", root, "_background-serve", "--allow-model-calls", "--claude-stream")
	cmd.ExtraFiles = []*os.File{child}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill() }()
	child.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	var ready backgroundReady
	if err := json.NewDecoder(conn).Decode(&ready); err != nil || ready.PID != cmd.Process.Pid || ready.Error != "" {
		t.Fatal("readiness", ready, err)
	}
	// Losing the parent before confirmation must not leave a controller that
	// could later dispatch messages. A fresh start can acquire the same lock.
	conn.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("aborted startup reported success")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("aborted startup remained alive")
	}
	if _, _, err := startBackground(root, nil); err != nil {
		t.Fatal("lock remained after aborted startup", err)
	}
	stopBackground(t, root)
}

func TestBackgroundReportsInitializationFailure(t *testing.T) {
	root := backgroundTestHome(t)
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "queue.sqlite"), []byte("corrupt database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if pid, _, err := startBackground(root, nil); err == nil || pid != 0 {
		t.Fatal("corrupt DB reported as ready", pid, err)
	}
	if _, _, err := startBackground(root, []string{"--force"}); err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatal("unknown flags accepted", err)
	}
}

func TestBackgroundCommandExitLeavesControllerReachable(t *testing.T) {
	root := backgroundTestHome(t)
	executable, _ := os.Executable()
	// Capture pipes as a terminal frontend would: the child must not keep them
	// open after the start command exits, otherwise CombinedOutput hangs.
	cmd := exec.Command(executable, "--root", root, "start", "--claude-stream")
	done := make(chan struct{})
	var output []byte
	var err error
	go func() { output, err = cmd.CombinedOutput(); close(done) }()
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("start retained frontend pipes")
	}
	if err != nil {
		t.Fatalf("start failed: %v %s", err, output)
	}
	defer stopBackground(t, root)
	first := strings.Fields(string(output))[0]
	pid, err := strconv.Atoi(strings.TrimPrefix(first, "background_pid="))
	if err != nil || pid <= 1 {
		t.Fatal("missing background PID", string(output))
	}
	for i := 0; i < 3; i++ {
		status := backgroundCall(t, root, control.Request{Cmd: "status"}).Status
		if status.Supervisor != pid || !status.Paused || status.CLILaunch != "not-authorized" {
			t.Fatal("frontend reconnection changed daemon", status)
		}
	}
}
