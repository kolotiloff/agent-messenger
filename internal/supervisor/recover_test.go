package supervisor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestRecoverFailedRefusesIncompleteLiveDescendantAndObservationError(t *testing.T) {
	isolateUser(t)
	if _, err := platform.ReadObservation(os.Getpid()); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	root, environment := seedFailedAttempt(t, false, false)
	if err := saveRuns([]recordedRun{{
		ID: "pending", Root: root, Provider: "claude", Environment: environment, ConversationID: "conversation-1",
	}}); err != nil {
		t.Fatal(err)
	}
	before := readRunsFile(t)
	if _, err := RecoverFailed(core.RetryConfirmation); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("incomplete = %v", err)
	}
	if after := readRunsFile(t); after != before || heldCount(t, root) != 1 {
		t.Fatal("incomplete reservation or history changed")
	}

	cmd := exec.Command("/bin/sleep", "30")
	if err := platform.SetNewGroup(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	if err := saveRuns([]recordedRun{observedRun("live", root, environment, obs)}); err != nil {
		t.Fatal(err)
	}
	before = readRunsFile(t)
	if _, err := RecoverFailed(core.RetryConfirmation); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("live process = %v", err)
	}
	if _, err := platform.ReadObservation(cmd.Process.Pid); err != nil {
		t.Fatal(err)
	}
	if readRunsFile(t) != before || heldCount(t, root) != 1 {
		t.Fatal("live reservation or history changed")
	}

	parent := exec.Command("python3", "-c", `
import os, time, subprocess
subprocess.Popen(["sleep", "30"])
print("READY", flush=True)
time.sleep(0.3)
os._exit(0)
`)
	if err := platform.SetNewGroup(parent); err != nil {
		t.Fatal(err)
	}
	stdout, err := parent.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.Start(); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	if _, err := stdout.Read(buf); err != nil {
		t.Fatal(err)
	}
	parentObs, err := platform.ReadObservation(parent.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = platform.SignalGroup(parentObs.PGID) })
	if err := parent.Wait(); err != nil {
		t.Fatal(err)
	}
	if err := saveRuns([]recordedRun{observedRun("descendant", root, environment, parentObs)}); err != nil {
		t.Fatal(err)
	}
	before = readRunsFile(t)
	if _, err := RecoverFailed(core.RetryConfirmation); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("live descendant = %v", err)
	}
	empty, err := platform.GroupEmpty(parentObs.PGID)
	if err != nil || empty {
		t.Fatalf("descendant not alive: empty=%t %v", empty, err)
	}
	if readRunsFile(t) != before {
		t.Fatal("descendant reservation was rewritten")
	}

	dead := exitedObservation(t)
	if err := saveRuns([]recordedRun{observedRun("dead", root, environment, dead)}); err != nil {
		t.Fatal(err)
	}
	before = readRunsFile(t)
	previous := groupEmpty
	groupEmpty = func(int) (bool, error) { return false, errors.New("snapshot failed") }
	t.Cleanup(func() { groupEmpty = previous })
	if _, err := RecoverFailed(core.RetryConfirmation); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("observation error = %v", err)
	}
	if readRunsFile(t) != before || heldCount(t, root) != 1 {
		t.Fatal("observation error rewrote the reservation or history")
	}
}

func TestRecoverFailedKeepsHistoryAndDoesNotStartCLI(t *testing.T) {
	isolateUser(t)
	if _, err := platform.ReadObservation(os.Getpid()); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			t.Skip(err.Error())
		}
		t.Fatal(err)
	}
	root, environment := seedFailedAttempt(t, false, false)
	obs := exitedObservation(t)
	run := observedRun("failed", root, environment, obs)
	if err := saveRuns([]recordedRun{run}); err != nil {
		t.Fatal(err)
	}
	before := readRunsFile(t)
	if _, err := RecoverFailed("no"); !errors.Is(err, core.ErrBadConfirmation) {
		t.Fatalf("bad confirm = %v", err)
	}
	if readRunsFile(t) != before || heldCount(t, root) != 1 {
		t.Fatal("bad confirmation changed the reservation or history")
	}
	if err := reconcileRuns(t.TempDir()); !errors.Is(err, ErrLiveCLI) {
		t.Fatalf("failed reservation stopped blocking before recovery: %v", err)
	}

	ready, readyEnv := seedFailedAttempt(t, false, false)
	readyObs := exitedObservation(t)
	if err := saveRuns([]recordedRun{
		run,
		observedRun("sibling", ready, readyEnv, readyObs),
		{ID: "pending", Root: t.TempDir(), Provider: "claude", ConversationID: "conversation-1"},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverFailed(core.RetryConfirmation); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("incomplete sibling = %v", err)
	}
	if heldCount(t, root) != 1 || heldCount(t, ready) != 1 {
		t.Fatal("incomplete sibling retired another reservation")
	}

	if err := saveRuns([]recordedRun{run}); err != nil {
		t.Fatal(err)
	}
	previousHook := beforeRenameHook
	beforeRenameHook = func() error { return errors.New("disk") }
	t.Cleanup(func() { beforeRenameHook = previousHook })
	if _, err := RecoverFailed(core.RetryConfirmation); err == nil {
		t.Fatal("registry write failure was ignored")
	}
	if runs, err := loadRuns(); err != nil || len(runs) != 1 || runs[0].ID != "failed" {
		t.Fatalf("failed registry write removed the reservation: %+v %v", runs, err)
	}
	beforeRenameHook = nil
	message, err := RecoverFailed(core.RetryConfirmation)
	if err != nil {
		t.Fatal(err)
	}
	if runs, err := loadRuns(); err != nil || len(runs) != 0 {
		t.Fatalf("retired reservation remains: %+v %v", runs, err)
	}
	if err := reconcileRuns(t.TempDir()); err != nil {
		t.Fatalf("other root still blocked: %v", err)
	}
	snap := snapshot(t, root)
	if snap.Sessions[0]["active_token"] != nil {
		t.Fatal("old lock remained")
	}
	if len(snap.Requests) != 2 || snap.Requests[0]["state"] != "uncertain" || snap.Requests[1]["retry_of"] != "m1" || snap.Requests[1]["state"] != "queued" {
		t.Fatalf("history = %+v", snap.Requests)
	}
	found := false
	for _, event := range snap.Events {
		if event["kind"] == "failed_attempt_recovered" {
			found = true
		}
	}
	if !found || message == "" {
		t.Fatalf("warning missing: %s %+v", message, snap.Events)
	}

	okRoot, okEnv := seedFailedAttempt(t, true, false)
	okObs := exitedObservation(t)
	if err := saveRuns([]recordedRun{observedRun("success", okRoot, okEnv, okObs)}); err != nil {
		t.Fatal(err)
	}
	before = readRunsFile(t)
	if _, err := RecoverFailed(core.RetryConfirmation); err == nil {
		t.Fatal("valid provider result was retired")
	}
	if readRunsFile(t) != before || heldCount(t, okRoot) != 1 {
		t.Fatal("valid reservation changed")
	}

	replyRoot, replyEnv := seedFailedAttempt(t, false, true)
	replyObs := exitedObservation(t)
	if err := saveRuns([]recordedRun{observedRun("reply", replyRoot, replyEnv, replyObs)}); err != nil {
		t.Fatal(err)
	}
	before = readRunsFile(t)
	if _, err := RecoverFailed(core.RetryConfirmation); err == nil {
		t.Fatal("stored reply was retired")
	}
	if readRunsFile(t) != before || heldCount(t, replyRoot) != 1 || len(snapshot(t, replyRoot).Replies) != 1 {
		t.Fatal("stored reply reservation changed")
	}
}

func TestRecoverFailedLeavesCorruptRegistryUntouched(t *testing.T) {
	isolateUser(t)
	path, err := runsPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverFailed(core.RetryConfirmation); !errors.Is(err, ErrRunsCorrupt) {
		t.Fatalf("corrupt = %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "{" {
		t.Fatalf("corrupt file changed: %q %v", raw, err)
	}
}

func seedFailedAttempt(t *testing.T, providerOK, withReply bool) (string, string) {
	t.Helper()
	root := t.TempDir()
	env := filepath.Join(root, "cli")
	if err := os.Mkdir(env, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := core.Open(filepath.Join(root, dbName))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if _, err := store.Register("claude", env, "conversation-1", "claude-test"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Enqueue("m1", "claude-test", "test-coordinator", "PAYLOAD-one"); err != nil {
		t.Fatal(err)
	}
	token, err := store.Claim("claude-test")
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BeginLaunch(token); err != nil {
		t.Fatal(err)
	}
	if err := store.ConfirmIdentity(token, "claude", env, "conversation-1"); err != nil {
		t.Fatal(err)
	}
	if withReply {
		if _, err := store.Reply(token, "kept"); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := store.FinishObserved(token, 1, true, providerOK, "not logged in"); err != nil {
		t.Fatal(err)
	}
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return root, snap.Sessions[0]["environment"].(string)
}

func observedRun(id, root, environment string, obs platform.Observation) recordedRun {
	return recordedRun{
		ID: id, Root: root, PID: obs.PID, PGID: obs.PGID, StartToken: obs.StartToken, BootID: obs.BootID,
		Provider: "claude", Environment: environment, ConversationID: "conversation-1",
	}
}

func exitedObservation(t *testing.T) platform.Observation {
	t.Helper()
	cmd := exec.Command("/bin/sleep", "30")
	if err := platform.SetNewGroup(cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	return obs
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

func snapshot(t *testing.T, root string) core.Snapshot {
	t.Helper()
	store, err := core.Open(filepath.Join(root, dbName))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	snap, err := store.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func heldCount(t *testing.T, root string) int {
	t.Helper()
	snap := snapshot(t, root)
	n := 0
	for _, session := range snap.Sessions {
		if session["active_token"] != nil {
			n++
		}
	}
	return n
}
