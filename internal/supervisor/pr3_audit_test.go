package supervisor

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestAuditPR3ExecDeniedMustNotPermanentlyReserveUser(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	profile := mkdir(t, filepath.Join(root, "profile"))
	cli := filepath.Join(root, "not-executable")
	if err := os.WriteFile(cli, []byte("not an executable\n"), 0600); err != nil {
		t.Fatal(err)
	}
	st := mustStore(t, root)
	defer st.Close()
	if _, err := st.Register("claude", profile, "test-id", "worker"); err != nil {
		t.Fatal(err)
	}
	// This configuration is accepted by the public registration path.
	if err := st.ConfigureLaunch("worker", cli, root); err != nil {
		t.Fatal(err)
	}
	_, err := (AdapterLauncher{AllowModels: true, ClaudeStream: true}).StartStream(context.Background(), Spec{Provider: "claude", Environment: profile, ConversationID: "test-id", CLI: cli, Workspace: root, DataRoot: root}, "http://127.0.0.1:1/mcp", "fixture-only")
	if err == nil {
		t.Fatal("non-executable unexpectedly launched")
	}
	runs, loadErr := loadRuns()
	if loadErr != nil {
		t.Fatal(loadErr)
	}
	recoveryErr := error(nil)
	if len(runs) > 0 {
		recoveryErr = reservationReadyToRetire(runs[0])
	}
	blocked := reconcileRuns(t.TempDir())
	if len(runs) > 0 && errors.Is(blocked, ErrLiveCLI) && errors.Is(recoveryErr, core.ErrRecoveryBlocked) {
		t.Fatalf("permission-denied start created no CLI but left %d incomplete reservation(s), blocked another root and cannot pass recovery: launch=%v recovery=%v", len(runs), err, recoveryErr)
	}
}

func TestExecRefusalKeepsReserveWhenCleanupWriteFails(t *testing.T) {
	isolateUser(t)
	root := t.TempDir()
	cli := filepath.Join(root, "not-executable")
	if err := os.WriteFile(cli, []byte("not executable"), 0600); err != nil {
		t.Fatal(err)
	}
	writes := 0
	beforeRenameHook = func() error {
		writes++
		if writes == 2 {
			return errors.New("injected registry write failure")
		}
		return nil
	}
	defer func() { beforeRenameHook = nil }()
	_, err := (AdapterLauncher{AllowModels: true, ClaudeStream: true}).StartStream(context.Background(), Spec{Provider: "claude", Environment: root, ConversationID: "test-id", CLI: cli, Workspace: root, DataRoot: root}, "http://127.0.0.1:1/mcp", "fixture-only")
	if err == nil || errors.Is(err, errNotStarted) {
		t.Fatal("cleanup failure incorrectly permits releasing the session", err)
	}
	runs, err := loadRuns()
	if err != nil || len(runs) != 1 {
		t.Fatal("reservation lost", runs, err)
	}
	if err := reconcileRuns(t.TempDir()); !errors.Is(err, ErrLiveCLI) {
		t.Fatal("another root not fenced", err)
	}
}

type auditHeldOutputLauncher struct {
	parent      *exec.Cmd
	inputRead   *os.File
	outputWrite *os.File
	reaped      chan struct{}
}

func (*auditHeldOutputLauncher) Enabled() bool          { return true }
func (*auditHeldOutputLauncher) StreamingEnabled() bool { return true }
func (*auditHeldOutputLauncher) Start(Spec) (Started, error) {
	return Started{}, errors.New("one-shot not supported")
}
func (l *auditHeldOutputLauncher) StartStream(_ context.Context, spec Spec, _ string, _ string) (StreamProcess, error) {
	cmd := exec.Command("/bin/sleep", "30")
	if err := platform.SetNewGroup(cmd); err != nil {
		return StreamProcess{}, err
	}
	if err := cmd.Start(); err != nil {
		return StreamProcess{}, err
	}
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		cmd.Process.Kill()
		cmd.Wait()
		return StreamProcess{}, err
	}
	inRead, inWrite, err := os.Pipe()
	if err != nil {
		return StreamProcess{}, err
	}
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		return StreamProcess{}, err
	}
	l.parent = cmd
	l.inputRead = inRead
	l.outputWrite = outWrite
	go func() {
		reader := bufio.NewReader(inRead)
		if _, err := reader.ReadString('\n'); err == nil {
			_ = json.NewEncoder(outWrite).Encode(map[string]any{"type": "system", "subtype": "init", "session_id": spec.ConversationID})
		}
	}()
	return StreamProcess{Observation: obs, Input: inWrite, Output: outRead, Wait: func() (int, error) { _ = cmd.Wait(); close(l.reaped); return cmd.ProcessState.ExitCode(), nil }}, nil
}

func TestAuditPR3ObservedExitMustRevokeMCPWithOutputStillOpen(t *testing.T) {
	root, st := seedStreamClient(t, "normal")
	if _, err := st.Enqueue("m1", "worker", "coordinator", "one"); err != nil {
		t.Fatal(err)
	}
	launcher := &auditHeldOutputLauncher{reaped: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, launcher) }()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("cleanup stuck")
		}
		if launcher.parent != nil {
			_ = launcher.parent.Process.Kill()
		}
		if launcher.inputRead != nil {
			launcher.inputRead.Close()
		}
		if launcher.outputWrite != nil {
			launcher.outputWrite.Close()
		}
	}()
	waitSocket(t, root)
	snap := waitStreamState(t, st, func(s core.Snapshot) bool { return len(s.StreamRuns) == 1 && s.StreamRuns[0]["state"] == "active" })
	run := text(snap.StreamRuns[0]["id"])
	before, err := st.StreamRead(run)
	if err != nil {
		t.Fatal(err)
	}
	// The synthetic CLI is now known to have exited. Its output descriptor stays
	// open, exactly as when a descendant inherits stdout.
	if err := launcher.parent.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	<-launcher.reaped
	time.Sleep(300 * time.Millisecond)
	message, readErr := st.StreamRead(run)
	var replyErr error
	if readErr == nil {
		_, replyErr = st.StreamReply(run, message.Receipt, "late reply after observed CLI death")
	}
	snap, err = st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if readErr == nil && replyErr == nil {
		t.Fatalf("CLI Wait completed, but stream is %v and late MCP read/reply were both accepted", snap.StreamRuns[0]["state"])
	}
	if _, err := st.StreamReply(run, before.Receipt, "late reply using an earlier receipt"); err == nil {
		t.Fatal("old receipt accepted after CLI exit")
	}
	if snap.StreamRuns[0]["exit_observed"] != int64(1) || snap.StreamRuns[0]["state"] != "uncertain" {
		t.Fatal("exit not recorded", snap.StreamRuns)
	}
}

type auditBlockedStartup struct{ entered, release chan struct{} }

func (*auditBlockedStartup) Enabled() bool               { return true }
func (*auditBlockedStartup) StreamingEnabled() bool      { return true }
func (*auditBlockedStartup) Start(Spec) (Started, error) { return Started{}, errors.New("unexpected") }
func (l *auditBlockedStartup) StartStream(context.Context, Spec, string, string) (StreamProcess, error) {
	close(l.entered)
	<-l.release
	return StreamProcess{}, errors.New("startup failed after delay")
}

func TestAuditPR3ShutdownMustNotWaitForStalledStartup(t *testing.T) {
	root, st := seedStreamClient(t, "normal")
	if _, err := st.Enqueue("m1", "worker", "coordinator", "one"); err != nil {
		t.Fatal(err)
	}
	launcher := &auditBlockedStartup{entered: make(chan struct{}), release: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, launcher) }()
	waitSocket(t, root)
	<-launcher.entered
	answered := make(chan error, 1)
	go func() {
		r, err := control.Call(root, control.Request{Cmd: "shutdown"})
		if err == nil && !r.OK {
			err = errors.New(r.Error)
		}
		answered <- err
	}()
	defer func() {
		close(launcher.release)
		waitStartupReleased(t)
	}()
	select {
	case err := <-answered:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown waited for blocked StartStream")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("serve did not exit while startup was still blocked")
	}
	for _, other := range []string{root, t.TempDir()} {
		probe, stop := context.WithTimeout(context.Background(), time.Second)
		err := Run(probe, other, DisabledLauncher{})
		stop()
		if !errors.Is(err, platform.ErrAlreadyRunning) {
			t.Fatal("late startup did not fence restart", err)
		}
	}
	if _, err := RecoverFailed(core.RetryConfirmation); !errors.Is(err, platform.ErrAlreadyRunning) {
		t.Fatal("recovery bypassed late startup", err)
	}
	snap, err := st.Snapshot()
	if err != nil || snap.StreamRuns[0]["state"] != "uncertain" {
		t.Fatal("startup was not revoked", snap, err)
	}
}

func waitStartupReleased(t *testing.T) {
	t.Helper()
	until := time.Now().Add(2 * time.Second)
	for time.Now().Before(until) {
		if err := checkStartupLease(); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("startup lease did not release")
}

type lateSuccessfulStartup struct {
	entered, release chan struct{}
	exit             chan int
}

func (*lateSuccessfulStartup) StreamingEnabled() bool { return true }
func (l *lateSuccessfulStartup) StartStream(_ context.Context, spec Spec, url, secret string) (StreamProcess, error) {
	close(l.entered)
	<-l.release
	// Simulate an OS launch completing successfully after cancellation.
	p, err := (AdapterLauncher{AllowModels: true, ClaudeStream: true}).StartStream(context.Background(), spec, url, secret)
	if err != nil {
		l.exit <- -999
		return p, err
	}
	wait := p.Wait
	p.Wait = func() (int, error) {
		code, err := wait()
		l.exit <- code
		return code, err
	}
	return p, nil
}

func TestLateSuccessfulStartupGetsNoPromptAndKeepsReservation(t *testing.T) {
	root, _ := seedStreamClient(t, "normal")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	launcher := &lateSuccessfulStartup{entered: make(chan struct{}), release: make(chan struct{}), exit: make(chan int, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := &Supervisor{}
	done := make(chan error, 1)
	go func() {
		_, err := s.startStream(ctx, launcher, Spec{Provider: "claude", Environment: filepath.Join(root, "profile"), ConversationID: "test-stream", CLI: exe, Workspace: root, DataRoot: root}, "http://127.0.0.1:1/mcp", "fixture-only")
		done <- err
	}()
	<-launcher.entered
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Error(err)
		}
	case <-time.After(time.Second):
		t.Error("controller waited for late launch")
	}
	if err := checkStartupLease(); !errors.Is(err, platform.ErrAlreadyRunning) {
		t.Error("startup lease released early", err)
	}
	close(launcher.release)
	select {
	case code := <-launcher.exit:
		// The fixture exits cleanly on EOF without input. If given a prompt it
		// would attempt MCP at the deliberately unavailable URL and fail.
		if code != 0 {
			t.Fatal("late process got input or failed cleanup", code)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("late process was not closed and reaped")
	}
	waitStartupReleased(t)
	runs, err := loadRuns()
	if err != nil || len(runs) != 1 || runs[0].PID == 0 {
		t.Fatal("late successful launch lost its reserve", runs, err)
	}
	if err := reconcileRuns(t.TempDir()); !errors.Is(err, ErrLiveCLI) {
		t.Fatal("late process reserve did not fence another root", err)
	}
}

func TestStreamExecRefusalPreservesHistoryAndAllowsNewMessage(t *testing.T) {
	root, st := seedStreamClient(t, "normal")
	cli := filepath.Join(root, "not-executable")
	if err := os.WriteFile(cli, []byte("not executable"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := st.ConfigureLaunch("worker", cli, root); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Enqueue("m1", "worker", "coordinator", "one"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, AdapterLauncher{AllowModels: true, ClaudeStream: true}) }()
	defer func() { cancel(); <-done }()
	waitSocket(t, root)
	snap := waitStreamState(t, st, func(s core.Snapshot) bool {
		return len(s.Attempts) == 1 && s.Attempts[0]["state"] == "not_started"
	})
	if len(snap.Replies) != 0 || len(snap.StreamRuns) != 0 || snap.Sessions[0]["active_token"] != nil {
		t.Fatal("definite refusal left an owner or fabricated reply", snap)
	}
	// No automatic replay: the failed request remains historical.
	if snap.Requests[0]["state"] != "not_started" {
		t.Fatal(snap.Requests)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.ConfigureLaunch("worker", exe, root); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Enqueue("m2", "worker", "coordinator", "two"); err != nil {
		t.Fatal(err)
	}
	snap = waitStreamState(t, st, func(s core.Snapshot) bool {
		return len(s.Replies) == 1 && len(s.StreamRuns) == 1 && s.StreamRuns[0]["state"] == "idle"
	})
	if len(snap.Attempts) != 2 || snap.Replies[0]["reply_to"] != "m2" {
		t.Fatal(snap)
	}
}
