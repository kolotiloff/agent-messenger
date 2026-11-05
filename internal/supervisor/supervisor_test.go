package supervisor

import (
	"context"
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
	if os.Getenv("MESSENGER_FIXTURE") == "agent" {
		if err := agentFixture(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestIdleAndPauseDoNotLaunch(t *testing.T) {
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
	go func() { _ = Run(ctx, root, launcher) }()
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
	root := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errc := make(chan error, 1)
	go func() { errc <- Run(ctx, root, DisabledLauncher{}) }()
	waitSocket(t, root)
	if _, err := platform.Acquire(filepath.Join(root, lockName)); !errors.Is(err, platform.ErrAlreadyRunning) {
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
	defer cancel()
	go func() { _ = Run(ctx, root, launcher) }()
	waitSocket(t, root)
	waitStatus(t, root, func(status control.Status) bool {
		return status.Queued == 0 && status.Held == 0 && launcher.calls() == 2
	})
	cancel()
}

func TestOccupiedGroupStaysUncertain(t *testing.T) {
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
	defer cancel()
	go func() { _ = Run(ctx, root, launcher) }()
	waitSocket(t, root)
	waitStatus(t, root, func(status control.Status) bool { return status.Held == 1 && launcher.calls() == 1 })
	response, err := control.Call(root, control.Request{Cmd: "resolve", Action: "retry", Alias: "worker", Confirmation: core.RetryConfirmation})
	if err == nil && response.OK {
		t.Fatal("retry succeeded without descendant proof")
	}
	accepted, err := control.Call(root, control.Request{Cmd: "resolve", Action: "accept", Alias: "worker"})
	if err == nil && accepted.OK {
		t.Fatal("accept succeeded without descendant proof")
	}
	stopped, err := control.Call(root, control.Request{Cmd: "stop-job"})
	if err != nil || !stopped.OK || stopped.Stop == nil || stopped.Stop.Signal != "sent" || stopped.Stop.State != "uncertain" {
		t.Fatalf("stop-job = %+v %v", stopped, err)
	}
	after, err := control.Call(root, control.Request{Cmd: "status"})
	if err != nil || after.Status == nil || after.Status.Held != 1 {
		t.Fatalf("status after stop = %+v %v", after, err)
	}
	cancel()
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
	fmt.Printf("session %s\n", os.Getenv("MESSENGER_EXPECT"))
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
