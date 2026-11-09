package supervisor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/mcp"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "task-mcp" {
		if err := mcp.TaskMain(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if mode := os.Getenv("MESSENGER_CLAUDE_MCP_FIXTURE"); mode != "" {
		os.Exit(fakeClaudeMCP(mode))
	}
	if mode := os.Getenv("MESSENGER_TASK_FIXTURE"); mode != "" {
		os.Exit(taskWorkerFixture(mode))
	}
	home, err := os.MkdirTemp("", "am-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	os.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	os.Setenv("AppData", filepath.Join(home, "appdata"))
	os.Setenv("LocalAppData", filepath.Join(home, "localappdata"))
	os.Exit(m.Run())
}

func isolateUser(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	t.Setenv("AppData", filepath.Join(home, "appdata"))
	t.Setenv("LocalAppData", filepath.Join(home, "localappdata"))
}

func TestCommandsDoNotStartASecondSupervisor(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, DisabledLauncher{}) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errc:
		case <-time.After(3 * time.Second):
		}
	})
	waitSocket(t, root)
	lockPath, err := platform.SupervisorLockFile()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := platform.Acquire(lockPath); !errors.Is(err, platform.ErrAlreadyRunning) {
		t.Fatalf("second lock = %v", err)
	}
	second := make(chan error, 1)
	go func() { second <- Run(context.Background(), root, DisabledLauncher{}) }()
	select {
	case err := <-second:
		if !errors.Is(err, platform.ErrAlreadyRunning) {
			t.Fatalf("second serve = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second serve did not return")
	}
	idle, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || !idle.OK || idle.Status.CLILaunch != "disabled" || idle.Status.TasksEnabled || idle.Status.Broker != "not_configured" {
		t.Fatalf("disabled launcher status = %+v %v", idle, err)
	}
	if r := idle.Status.Reservations; r == nil || r.Tasks != 0 || len(r.Old) != 0 || r.Error != "" {
		t.Fatalf("reservations = %+v", r)
	}
	// The removed mode's commands are gone, not silently accepted.
	for _, cmd := range []string{"enqueue", "register-session", "create-claude-session", "setup-codex", "authorize-setup", "ui-prepare", "timeline", "overview", "inspect", "resolve", "stop-job"} {
		if r, err := control.Call(root, control.Request{Cmd: cmd, Alias: "worker"}); err != nil || r.OK || r.Error != "неизвестная команда" {
			t.Fatalf("%s = %+v %v", cmd, r, err)
		}
	}
	if _, err := control.Call(root, control.Request{Cmd: "shutdown"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-errc:
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not exit after shutdown")
	}
}

func TestStatusReportsTaskLaunches(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, AdapterLauncher{AllowTasks: true}) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errc:
		case <-time.After(3 * time.Second):
		}
	})
	waitSocket(t, root)
	r, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || !r.OK || !r.Status.TasksEnabled || r.Status.CLILaunch != "tasks" || r.Status.Paused {
		t.Fatalf("status = %+v %v", r, err)
	}
}

// Old reservations of the removed persistent mode (T-028, D-104) in the
// client's own data directory: the client starts, lists them read-only in
// status, and the registry file is left byte for byte as it was, through
// startup, status, pause, resume and shutdown. Another data directory is
// still refused while they exist.
func TestOldModeReservationsAreKeptAndListed(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	canonical, err := core.CanonicalEnvironment(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, run := range []recordedRun{
		{ID: "stream", Root: canonical, PID: 1234, PGID: 1234, StartToken: "st", BootID: "boot", Provider: "claude", Environment: "/profile", ConversationID: "11111111-1111-4111-8111-111111111111", StreamRunID: "token-1"},
		{ID: "codex", Root: canonical, PID: 2345, PGID: 2345, StartToken: "st", BootID: "boot", Provider: "codex", Environment: "/codex", CodexSetupID: "setup-1"},
		{ID: "message", Root: canonical, Provider: "cursor", Environment: "/cursor", ConversationID: "chat-1"},
	} {
		if err := rememberRun(run); err != nil {
			t.Fatal(err)
		}
	}
	before := readRunsFile(t)
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, AdapterLauncher{AllowTasks: true}) }()
	t.Cleanup(cancel)
	waitSocket(t, root)
	r, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || !r.OK || r.Status.Reservations == nil {
		t.Fatalf("status = %+v %v", r, err)
	}
	got := r.Status.Reservations
	if got.Tasks != 0 || len(got.Old) != 3 || got.Error != "" {
		t.Fatalf("reservations = %+v", got)
	}
	kinds := []string{}
	for _, old := range got.Old {
		if old.Root != canonical {
			t.Fatalf("root = %s", old.Root)
		}
		kinds = append(kinds, old.Kind+":"+old.Provider)
	}
	if strings.Join(kinds, ",") != "claude-stream:claude,codex-app-server:codex,session:cursor" {
		t.Fatalf("kinds = %v", kinds)
	}
	for _, cmd := range []string{"pause", "resume", "status"} {
		if r, err := control.Call(root, control.Request{Cmd: cmd}); err != nil || !r.OK {
			t.Fatalf("%s = %+v %v", cmd, r, err)
		}
	}
	if _, err := control.Call(root, control.Request{Cmd: "shutdown"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-errc:
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not exit after shutdown")
	}
	if after := readRunsFile(t); after != before {
		t.Fatalf("registry changed:\nbefore %s\nafter  %s", before, after)
	}
	if err := Run(context.Background(), t.TempDir(), DisabledLauncher{}); !errors.Is(err, ErrLiveCLI) {
		t.Fatalf("another data directory = %v", err)
	}
	if after := readRunsFile(t); after != before {
		t.Fatal("registry changed after another data directory was refused")
	}
}

func readRunsFile(t *testing.T) string {
	t.Helper()
	path, err := runsPath()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func mustStore(t *testing.T, root string) *core.Store {
	t.Helper()
	st, err := core.Open(filepath.Join(root, dbName))
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func waitSocket(t *testing.T, root string) {
	t.Helper()
	// The socket file appears at bind(), slightly before listen(). A dial only
	// succeeds once the client accepts connections, so tests never race it.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if socketAccepts(root) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("command socket was not created")
}

func TestDifferentDataRootsCannotRunTwoSupervisors(t *testing.T) {
	isolateUser(t)
	first := t.TempDir()
	second := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, first, DisabledLauncher{}) }()
	waitSocket(t, first)
	err := Run(context.Background(), second, DisabledLauncher{})
	if !errors.Is(err, platform.ErrAlreadyRunning) {
		t.Fatalf("second root started a supervisor: %v", err)
	}
	cancel()
	select {
	case <-errc:
	case <-time.After(3 * time.Second):
		t.Fatal("first supervisor did not exit")
	}
}

func TestPendingReservationAndLiveDescendantBlockOtherRoot(t *testing.T) {
	isolateUser(t)
	if err := rememberRun(recordedRun{ID: "before-start", Root: "/old-data", Provider: "codex", ConversationID: "conversation-1"}); err != nil {
		t.Fatal(err)
	}
	if err := reconcileRuns("/new-data"); !errors.Is(err, ErrLiveCLI) {
		t.Fatalf("reservation before start = %v", err)
	}
	runs, err := loadRuns()
	if err != nil || len(runs) != 1 || runs[0].ID != "before-start" {
		t.Fatalf("pending reservation was dropped: %+v %v", runs, err)
	}

	if _, err := platform.ReadObservation(os.Getpid()); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "-c", `
import os, time, subprocess
subprocess.Popen(["sleep", "30"])
print("READY", flush=True)
time.sleep(0.3)
os._exit(0)
`)
	if err := platform.SetNewGroup(cmd); err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	if _, err := stdout.Read(buf); err != nil {
		t.Fatal(err)
	}
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = platform.SignalGroup(obs.PGID) })
	if err := rememberRun(recordedRun{
		ID: "parent-child", Root: "/old-data", PID: obs.PID, PGID: obs.PGID,
		StartToken: obs.StartToken, BootID: obs.BootID, Provider: "codex", ConversationID: "conversation-1",
	}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	empty, err := platform.GroupEmpty(obs.PGID)
	if err != nil || empty {
		t.Fatalf("descendant not alive: empty=%t %v", empty, err)
	}
	if err := reconcileRuns("/new-data"); !errors.Is(err, ErrLiveCLI) {
		t.Fatalf("other root was allowed after the parent exited: %v", err)
	}
	runs, err = loadRuns()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, run := range runs {
		if run.ID == "parent-child" {
			found = true
		}
	}
	if !found {
		t.Fatal("reservation was deleted though a descendant is still running")
	}
}

func socketAccepts(root string) bool {
	conn, err := net.DialTimeout("unix", control.SocketPath(root), time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
