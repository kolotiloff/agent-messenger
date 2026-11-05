package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestMain(m *testing.M) {
	if root := os.Getenv("MESSENGER_STREAM_SERVE_ROOT"); root != "" {
		os.Unsetenv("MESSENGER_STREAM_SERVE_ROOT")
		if err := Run(context.Background(), root, AdapterLauncher{AllowModels: true, ClaudeStream: true}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if mode := os.Getenv("MESSENGER_STREAM_FIXTURE"); mode != "" {
		if err := streamFixture(mode); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	if os.Getenv("MESSENGER_FIXTURE") == "agent" {
		if err := agentFixture(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	home, err := os.MkdirTemp("", "am-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	os.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	os.Setenv("AppData", filepath.Join(home, "appdata"))
	os.Exit(m.Run())
}

func isolateUser(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "xdg"))
	t.Setenv("AppData", filepath.Join(home, "appdata"))
}

func TestIdleAndPauseDoNotLaunch(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	st := mustStore(t, root)
	if _, err := st.Register("synthetic", env, "conversation-1", "worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Enqueue("m1", "worker", "coordinator", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := st.Pause(); err != nil {
		t.Fatal(err)
	}
	st.Close()
	launcher := &countingLauncher{enabled: true}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, launcher) }()
	waitSocket(t, root)
	time.Sleep(180 * time.Millisecond)
	if launcher.calls() != 0 {
		t.Fatalf("paused client launched %d times", launcher.calls())
	}
	response, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || !response.OK || !response.Status.Paused || response.Status.Queued != 1 {
		t.Fatalf("status = %+v %v", response, err)
	}
	if response.Status.CLILaunch == "" || response.Status.Broker != "not_configured" {
		t.Fatalf("status flags = %+v", response.Status)
	}
	created, err := control.Call(root, control.Request{Cmd: "enqueue", Alias: "worker", MessageID: "m2", Sender: "coordinator", Text: "second"})
	if err != nil || !created.OK || created.Created == nil || !*created.Created {
		t.Fatalf("enqueue = %+v %v", created, err)
	}
	done := make(chan struct{})
	go func() {
		_, _ = control.Call(root, control.Request{Cmd: "shutdown"})
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("shutdown did not finish")
	}
	select {
	case <-errc:
	case <-time.After(3 * time.Second):
		t.Fatal("supervisor did not exit")
	}
	st = mustStore(t, root)
	defer st.Close()
	snap, err := st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Paused || len(snap.Requests) != 2 {
		t.Fatalf("queue after shutdown = paused %t requests %+v", snap.Paused, snap.Requests)
	}
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
	env := mkdir(t, filepath.Join(root, "cli"))
	prepare(t, root, env, "conversation-1", "m1")
	time.Sleep(180 * time.Millisecond)
	idle, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || !idle.OK || idle.Status.Queued != 1 || idle.Status.Held != 0 || idle.Status.CLILaunch != "disabled" {
		t.Fatalf("disabled launcher status = %+v %v", idle, err)
	}
	stopped, err := control.Call(root, control.Request{Cmd: "stop-job"})
	if err == nil && stopped.OK {
		t.Fatal("stop-job succeeded without an active job")
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

func TestSupervisorRunsExplicitSessionAndBlocksOccupiedGroup(t *testing.T) {
	isolateUser(t)
	if _, err := platform.ReadObservation(os.Getpid()); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	prepare(t, root, env, "conversation-1", "m1", "m2")
	launcher := &countingLauncher{enabled: true, start: func(spec Spec) (Started, error) {
		return startAgent(spec)
	}}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, launcher) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errc:
		case <-time.After(3 * time.Second):
		}
	})
	waitSocket(t, root)
	waitStatus(t, root, func(status control.Status) bool {
		return status.Held == 1 && launcher.calls() == 1
	})
}

func TestOccupiedGroupStaysUncertain(t *testing.T) {
	isolateUser(t)
	if _, err := platform.ReadObservation(os.Getpid()); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	prepare(t, root, env, "conversation-1", "m1")
	dir := mkdir(t, filepath.Join(root, "child"))
	t.Cleanup(func() { killRecordedChild(dir) })
	launcher := &countingLauncher{enabled: true, start: func(spec Spec) (Started, error) {
		return startAgent(spec, "MESSENGER_KEEP_CHILD=1", "MESSENGER_DIR="+dir)
	}}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, launcher) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errc:
		case <-time.After(3 * time.Second):
		}
	})
	waitSocket(t, root)
	waitFile(t, filepath.Join(dir, "child.pid"))
	deadline := time.Now().Add(5 * time.Second)
	for {
		inspected, err := control.Call(root, control.Request{Cmd: "inspect"})
		if err == nil && strings.Contains(string(inspected.Inspect), `"state":"uncertain"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("attempt did not settle: %s", inspected.Inspect)
		}
		time.Sleep(20 * time.Millisecond)
	}
	response, err := control.Call(root, control.Request{Cmd: "resolve", Action: "retry", Alias: "worker", Confirmation: core.RetryConfirmation})
	if err == nil && response.OK {
		t.Fatal("retry succeeded without descendant proof")
	}
	accepted, err := control.Call(root, control.Request{Cmd: "resolve", Action: "accept", Alias: "worker"})
	if err == nil && accepted.OK {
		t.Fatal("accept succeeded without descendant proof")
	}
	stopped, err := control.Call(root, control.Request{Cmd: "stop-job"})
	if err != nil || !stopped.OK || stopped.Stop == nil || stopped.Stop.Signal != "not_owner" || stopped.Stop.State != "uncertain" {
		t.Fatalf("stop-job = %+v %v", stopped.Stop, err)
	}
	if text, err := os.ReadFile(filepath.Join(dir, "child.pid")); err == nil {
		pid, _ := strconv.Atoi(strings.TrimSpace(string(text)))
		if _, err := platform.ReadObservation(pid); err != nil {
			t.Fatalf("unowned child was signaled: %v", err)
		}
	}
	after, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || after.Status == nil || after.Status.Held != 1 {
		t.Fatalf("status after stop = %+v %v", after, err)
	}
}

type countingLauncher struct {
	mu      sync.Mutex
	enabled bool
	n       int
	start   func(Spec) (Started, error)
}

func (c *countingLauncher) Enabled() bool { return c.enabled }

func (c *countingLauncher) calls() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

func (c *countingLauncher) Start(spec Spec) (Started, error) {
	c.mu.Lock()
	c.n++
	start := c.start
	c.mu.Unlock()
	if start == nil {
		return Started{}, errors.New("disabled in this test")
	}
	return start(spec)
}

func startAgent(spec Spec, extra ...string) (Started, error) {
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(),
		"MESSENGER_FIXTURE=agent",
		"MESSENGER_DB="+spec.DB,
		"MESSENGER_TOKEN="+spec.Token,
		"MESSENGER_EXPECT="+spec.ConversationID,
	)
	cmd.Env = append(cmd.Env, extra...)
	if err := PrepareCommand(cmd); err != nil {
		return Started{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Started{}, err
	}
	if err := cmd.Start(); err != nil {
		return Started{}, err
	}
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Started{}, err
	}
	return Started{
		PID: obs.PID, PGID: obs.PGID, StartToken: obs.StartToken, BootID: obs.BootID, Stdout: stdout,
		Wait: func() (int, error) {
			err := cmd.Wait()
			if cmd.ProcessState == nil {
				return -1, err
			}
			return cmd.ProcessState.ExitCode(), nil
		},
	}, nil
}

func prepare(t *testing.T, root, env, conversation string, ids ...string) {
	t.Helper()
	st := mustStore(t, root)
	defer st.Close()
	if _, err := st.Register("synthetic", env, conversation, "worker"); err != nil {
		t.Fatal(err)
	}
	if err := st.ConfigureLaunch("worker", os.Args[0], env); err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if _, err := st.Enqueue(id, "worker", "coordinator", "request "+id); err != nil {
			t.Fatal(err)
		}
	}
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

func waitFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

func waitSocket(t *testing.T, root string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(control.SocketPath(root)); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("command socket was not created")
}

func waitStatus(t *testing.T, root string, done func(control.Status) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last control.Response
	for time.Now().Before(deadline) {
		response, err := control.Call(root, control.Request{Cmd: "status"})
		if err == nil && response.Status != nil && done(*response.Status) {
			return
		}
		last = response
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("status timeout: %+v", last)
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

func TestStopJobRevokesMCPBeforeSignalingOwnedProcess(t *testing.T) {
	if _, err := platform.ReadObservation(os.Getpid()); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	st := mustStore(t, root)
	defer st.Close()
	if _, err := st.Register("synthetic", env, "conversation-1", "worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Enqueue("m1", "worker", "coordinator", "hello"); err != nil {
		t.Fatal(err)
	}
	token, err := st.Claim("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BeginLaunch(token); err != nil || st.ConfirmIdentity(token, "synthetic", env, "conversation-1") != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sleep", "30")
	if err := platform.SetNewGroup(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RecordProcess(token, obs); err != nil {
		t.Fatal(err)
	}
	sup := &Supervisor{store: st}
	sup.beforeSignal = func() {
		if _, err := st.Reply(token, "during-signal"); err == nil {
			t.Error("mcp accepted a reply after stop had begun")
		}
	}
	result, err := sup.StopJob()
	if err != nil || result.Signal != "sent" {
		t.Fatalf("stop = %+v %v", result, err)
	}
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("owned process was not signaled")
	}
}

func TestStopJobDoesNotSignalRecycledGroup(t *testing.T) {
	if _, err := platform.ReadObservation(os.Getpid()); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	st := mustStore(t, root)
	defer st.Close()
	if _, err := st.Register("synthetic", env, "conversation-1", "worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Enqueue("m1", "worker", "coordinator", "hello"); err != nil {
		t.Fatal(err)
	}
	token, err := st.Claim("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BeginLaunch(token); err != nil || st.ConfirmIdentity(token, "synthetic", env, "conversation-1") != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("/bin/sleep", "30")
	if err := platform.SetNewGroup(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	obs.StartToken = "stale-start-token"
	if err := st.RecordProcess(token, obs); err != nil {
		t.Fatal(err)
	}
	sup := &Supervisor{store: st}
	result, err := sup.StopJob()
	if err != nil || result.Signal != "not_owner" {
		t.Fatalf("stop = %+v %v", result, err)
	}
	if _, err := platform.ReadObservation(cmd.Process.Pid); err != nil {
		t.Fatalf("unrelated process was signaled: %v", err)
	}
	if _, err := st.Reply(token, "after"); !errors.Is(err, core.ErrRejected) {
		t.Fatalf("mcp after stop = %v", err)
	}
}

func TestAdapterLauncherResumesExactID(t *testing.T) {
	isolateUser(t)
	if _, err := platform.ReadObservation(os.Getpid()); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	workspace := mkdir(t, filepath.Join(root, "workspace"))
	argvPath := filepath.Join(root, "argv.txt")
	envPath := filepath.Join(root, "env.txt")
	script := filepath.Join(root, "fake-cli")
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "wrong-profile"))
	body := "#!/bin/sh\nprintf '%s\\n' \"$@\" > " + shellQuote(argvPath) + "\n" +
		"printf '%s\\n' \"$CLAUDE_CONFIG_DIR\" > " + shellQuote(envPath) + "\n" +
		"id=\nprev=\nfor a in \"$@\"; do\n" +
		"  case \"$a\" in --resume=*) id=${a#--resume=} ;; esac\n" +
		"  if [ \"$prev\" = \"--resume\" ] || [ \"$prev\" = \"resume\" ]; then id=$a; fi\n" +
		"  prev=$a\ndone\n" +
		"printf '%s\\n' \"{\\\"type\\\":\\\"system\\\",\\\"subtype\\\":\\\"init\\\",\\\"session_id\\\":\\\"$id\\\"}\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, AdapterLauncher{AllowModels: true}) }()
	waitSocket(t, root)
	registered, err := control.Call(root, control.Request{
		Cmd: "register-session", Provider: "claude", Environment: env, ConversationID: "conversation-1",
		Alias: "worker", CLI: script, Workspace: workspace,
	})
	if err != nil || !registered.OK {
		t.Fatalf("register = %+v %v", registered, err)
	}
	queued, err := control.Call(root, control.Request{Cmd: "enqueue", Alias: "worker", MessageID: "m1", Sender: "coordinator", Text: "hello"})
	if err != nil || !queued.OK {
		t.Fatalf("enqueue = %+v %v", queued, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var argv string
	for time.Now().Before(deadline) {
		text, err := os.ReadFile(argvPath)
		if err == nil && len(text) > 0 {
			argv = string(text)
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if argv == "" {
		t.Fatal("adapter did not start the bound cli")
	}
	if !strings.Contains(argv, "conversation-1") || !strings.Contains(argv, "--resume") || strings.Contains(argv, "--last") || strings.Contains(argv, "--continue") {
		t.Fatalf("argv = %s", argv)
	}
	gotEnv, err := os.ReadFile(envPath)
	wantEnv, err := core.CanonicalEnvironment(env)
	if err != nil {
		t.Fatal(err)
	}
	gotEnvPath, err := core.CanonicalEnvironment(strings.TrimSpace(string(gotEnv)))
	if err != nil {
		t.Fatal(err)
	}
	if gotEnvPath != wantEnv {
		t.Fatalf("CLAUDE_CONFIG_DIR = %s, want %s", gotEnvPath, wantEnv)
	}
	waitStatus(t, root, func(status control.Status) bool { return status.Held == 1 && status.CLILaunch == "adapters" })
	cancel()
	select {
	case <-errc:
	case <-time.After(3 * time.Second):
		t.Fatal("supervisor did not exit")
	}
}

func TestServeWithoutAuthorizationDoesNotStartCLI(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	workspace := mkdir(t, filepath.Join(root, "workspace"))
	argvPath := filepath.Join(root, "argv.txt")
	script := filepath.Join(root, "fake-cli")
	if err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > "+shellQuote(argvPath)+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, AdapterLauncher{}) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-errc:
		case <-time.After(3 * time.Second):
		}
	})
	waitSocket(t, root)
	if _, err := control.Call(root, control.Request{
		Cmd: "register-session", Provider: "claude", Environment: env, ConversationID: "conversation-1",
		Alias: "worker", CLI: script, Workspace: workspace,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := control.Call(root, control.Request{Cmd: "enqueue", Alias: "worker", MessageID: "m1", Sender: "coordinator", Text: "hello"}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(argvPath); err == nil {
		t.Fatal("cli started without --allow-model-calls")
	}
	status, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || status.Status == nil || status.Status.Queued != 1 || status.Status.Held != 0 || status.Status.CLILaunch != "not-authorized" {
		t.Fatalf("status = %+v %v", status, err)
	}
}

func TestShutdownDoesNotWaitAndOtherRootSeesLiveCLI(t *testing.T) {
	isolateUser(t)
	if _, err := platform.ReadObservation(os.Getpid()); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	root := t.TempDir()
	other := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	workspace := mkdir(t, filepath.Join(root, "workspace"))
	pidPath := filepath.Join(root, "pid")
	script := filepath.Join(root, "fake-cli")
	body := "#!/bin/sh\nprintf '%s\\n' \"$$\" > " + shellQuote(pidPath) + "\n" +
		"printf '%s\\n' '{\"type\":\"system\",\"subtype\":\"init\",\"session_id\":\"conversation-1\"}'\n" +
		"sleep 30\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, AdapterLauncher{AllowModels: true}) }()
	t.Cleanup(func() {
		if text, err := os.ReadFile(pidPath); err == nil {
			pid, _ := strconv.Atoi(strings.TrimSpace(string(text)))
			if pid > 0 {
				if proc, err := os.FindProcess(pid); err == nil {
					_ = proc.Kill()
				}
			}
		}
		cancel()
		select {
		case <-errc:
		case <-time.After(3 * time.Second):
		}
	})
	waitSocket(t, root)
	if _, err := control.Call(root, control.Request{
		Cmd: "register-session", Provider: "claude", Environment: env, ConversationID: "conversation-1",
		Alias: "worker", CLI: script, Workspace: workspace,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := control.Call(root, control.Request{Cmd: "enqueue", Alias: "worker", MessageID: "m1", Sender: "coordinator", Text: "hello"}); err != nil {
		t.Fatal(err)
	}
	waitFile(t, pidPath)
	started := time.Now()
	if _, err := control.Call(root, control.Request{Cmd: "shutdown"}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-errc:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown waited for the hung CLI")
	}
	if time.Since(started) > 2*time.Second {
		t.Fatal("shutdown took too long")
	}
	text, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatal(err)
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(text)))
	if _, err := platform.ReadObservation(pid); err != nil {
		t.Fatalf("cli was stopped by shutdown: %v", err)
	}
	err = Run(context.Background(), other, DisabledLauncher{})
	if !errors.Is(err, ErrLiveCLI) {
		t.Fatalf("other root = %v", err)
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

func shellQuote(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "'\\''") + "'"
}

func killRecordedChild(dir string) {
	text, err := os.ReadFile(filepath.Join(dir, "child.pid"))
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(text)))
	if err != nil || pid <= 0 {
		return
	}
	proc, err := os.FindProcess(pid)
	if err == nil {
		_ = proc.Kill()
	}
}

func agentFixture() error {
	st, err := core.Open(os.Getenv("MESSENGER_DB"))
	if err != nil {
		return err
	}
	defer st.Close()
	token := os.Getenv("MESSENGER_TOKEN")
	if _, err := st.Reply(token, "too-early"); err == nil {
		return errors.New("reply succeeded before identity")
	}
	payload, err := json.Marshal(map[string]any{
		"type": "system", "subtype": "init", "session_id": os.Getenv("MESSENGER_EXPECT"),
	})
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", payload)
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := st.Reply(token, "synthetic reply"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return errors.New("reply was not accepted")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if os.Getenv("MESSENGER_KEEP_CHILD") == "1" {
		cmd := exec.Command("/bin/sleep", "30")
		if err := cmd.Start(); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(os.Getenv("MESSENGER_DIR"), "child.pid"), []byte(strconv.Itoa(cmd.Process.Pid)), 0o600)
	}
	return nil
}
