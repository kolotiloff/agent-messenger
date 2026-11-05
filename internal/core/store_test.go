package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv("MESSENGER_FIXTURE"); mode != "" {
		if err := runFixture(mode); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestCanonicalEnvironmentRejectsNamesAndRelativePaths(t *testing.T) {
	root := t.TempDir()
	if _, err := CanonicalEnvironment("local-default-test"); err == nil {
		t.Fatal("display name was accepted")
	}
	if _, err := CanonicalEnvironment(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing directory was accepted")
	}
	if _, err := SessionKey("synthetic", "relative", "conversation-1"); err == nil {
		t.Fatal("relative environment was accepted")
	}
}

func TestAliasesAndPathSpellingsShareOneLock(t *testing.T) {
	st, env := openTest(t)
	key, err := st.Register("Claude", env, "conversation-1", "alpha")
	if err != nil {
		t.Fatal(err)
	}
	again, err := st.Register("claude", env, "conversation-1", "beta")
	if err != nil {
		t.Fatal(err)
	}
	if key != again {
		t.Fatalf("provider spelling created two keys")
	}
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(env, link); err != nil {
		t.Skip("symlink is not available")
	}
	linked, err := st.Register("claude", link, "conversation-1", "gamma")
	if err != nil {
		t.Fatal(err)
	}
	if linked != key {
		t.Fatal("symlink path created a second session")
	}
	if _, err := st.Enqueue("m1", "alpha", "coordinator", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Claim("alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Claim("beta"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second alias claim = %v", err)
	}
	if _, err := st.Claim("gamma"); !errors.Is(err, ErrBusy) {
		t.Fatalf("symlink alias claim = %v", err)
	}
}

func TestIndependentEnvironmentsDoNotShareLock(t *testing.T) {
	st, env := openTest(t)
	other := filepath.Join(filepath.Dir(env), "other-home")
	if err := os.Mkdir(other, 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Register("synthetic", env, "conversation-1", "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Register("synthetic", other, "conversation-1", "other"); err != nil {
		t.Fatal(err)
	}
	enqueue(t, st, "alpha", "m1")
	enqueue(t, st, "other", "m2")
	if _, err := st.Claim("alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Claim("other"); err != nil {
		t.Fatal(err)
	}
}

func TestReplyIsBoundAndDuplicateTextDoesNotOverwrite(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	first, err := st.Reply(token, "correct")
	if err != nil {
		t.Fatal(err)
	}
	if first.ReplyTo != "m1" || first.Sender != "worker" || first.Recipient != "coordinator" {
		t.Fatalf("route = %+v", first)
	}
	again, err := st.Reply(token, "correct")
	if err != nil || again.ID != first.ID {
		t.Fatalf("duplicate reply = %+v %v", again, err)
	}
	if _, err := st.Reply(token, "different"); !errors.Is(err, ErrRejected) {
		t.Fatalf("conflicting reply = %v", err)
	}
	snap := mustSnap(t, st)
	if len(snap.Replies) != 1 {
		t.Fatalf("replies = %d", len(snap.Replies))
	}
}

func TestForeignAndCompletedContextCannotReply(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	if _, err := st.Reply(token, "ok"); err != nil {
		t.Fatal(err)
	}
	state, err := st.FinishObserved(token, 0, true, true, "test")
	if err != nil || state != "uncertain" {
		t.Fatalf("finish = %s %v", state, err)
	}
	if _, err := st.Reply(token, "late"); !errors.Is(err, ErrRejected) {
		t.Fatalf("old token reply = %v", err)
	}
	if _, err := st.Reply("invented-context", "late"); !errors.Is(err, ErrRejected) {
		t.Fatalf("invented reply = %v", err)
	}
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("empty group released the lock: %v", err)
	}
}

func TestDuplicateDeliveryDoesNotStartAnotherAttempt(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	created, err := st.Enqueue("m1", "worker", "coordinator", "request m1")
	if err != nil || created {
		t.Fatalf("duplicate created = %t %v", created, err)
	}
	if _, err := st.Enqueue("m1", "worker", "coordinator", "other"); !errors.Is(err, ErrRejected) {
		t.Fatalf("conflict = %v", err)
	}
	if _, err := st.Reply(token, "done"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.FinishObserved(token, 0, true, true, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("claim after observed exit = %v", err)
	}
	if len(mustSnap(t, st).Attempts) != 1 {
		t.Fatal("attempt count")
	}
}

func TestSameReservationCannotLaunchTwice(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	enqueue(t, st, "worker", "m1")
	token, err := st.Claim("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BeginLaunch(token); err != nil {
		t.Fatal(err)
	}
	if err := st.BeginLaunch(token); !errors.Is(err, ErrRejected) {
		t.Fatalf("second launch = %v", err)
	}
}

func TestIdentityGateAndWrongID(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	enqueue(t, st, "worker", "m1")
	token, err := st.Claim("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BeginLaunch(token); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CurrentMessage(token); !errors.Is(err, ErrRejected) {
		t.Fatalf("early read = %v", err)
	}
	if _, err := st.Reply(token, "early"); !errors.Is(err, ErrRejected) {
		t.Fatalf("early reply = %v", err)
	}
	if err := st.ConfirmIdentity(token, "synthetic", env, "conversation-1"); err != nil {
		t.Fatal(err)
	}
	msg, err := st.CurrentMessage(token)
	if err != nil || msg.ID != "m1" {
		t.Fatalf("message = %+v %v", msg, err)
	}

	st2, env2 := openTest(t)
	register(t, st2, env2, "worker")
	enqueue(t, st2, "worker", "m1")
	bad, err := st2.Claim("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := st2.BeginLaunch(bad); err != nil {
		t.Fatal(err)
	}
	if err := st2.ConfirmIdentity(bad, "synthetic", env2, "wrong-id"); !errors.Is(err, ErrRejected) {
		t.Fatalf("mismatch = %v", err)
	}
	if _, err := st2.CurrentMessage(bad); !errors.Is(err, ErrRejected) {
		t.Fatalf("mcp after mismatch = %v", err)
	}
	if _, err := st2.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("claim after mismatch = %v", err)
	}
	if len(mustSnap(t, st2).Replies) != 0 {
		t.Fatal("mismatch stored a reply")
	}
}

func TestExitCodeOrMissingReplyDoesNotComplete(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	state, err := st.FinishObserved(token, 0, true, true, "no reply")
	if err != nil || state != "uncertain" {
		t.Fatalf("clean exit without reply = %s %v", state, err)
	}
	enqueue(t, st, "worker", "m2")
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("claim = %v", err)
	}

	st, env = openTest(t)
	register(t, st, env, "worker")
	token = running(t, st, env, "worker", "m1", "conversation-1")
	if _, err := st.Reply(token, "stored"); err != nil {
		t.Fatal(err)
	}
	state, err = st.FinishObserved(token, 1, true, true, "nonzero")
	if err != nil || state != "uncertain" {
		t.Fatalf("nonzero = %s %v", state, err)
	}
	if _, err := st.CurrentMessage(token); !errors.Is(err, ErrRejected) {
		t.Fatalf("mcp after uncertain = %v", err)
	}
	if len(mustSnap(t, st).Replies) != 1 {
		t.Fatal("reply was deleted")
	}
}

func TestGroupNotEmptyDoesNotCompleteAndAcceptNeedsVerification(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	if _, err := st.Reply(token, "stored"); err != nil {
		t.Fatal(err)
	}
	state, err := st.FinishObserved(token, 0, false, true, "group occupied")
	if err != nil || state != "uncertain" {
		t.Fatalf("occupied group = %s %v", state, err)
	}
	if err := st.Accept("worker", platform.Proof{}); !errors.Is(err, ErrRecoveryBlocked) {
		t.Fatalf("accept without proof = %v", err)
	}
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if err := st.Accept("worker", platform.Proof{Proven: true, Detail: "injected"}); err != nil {
		t.Fatalf("accept after verification = %v", err)
	}
	snap := mustSnap(t, st)
	if snap.Requests[0]["state"] != "completed" || len(snap.Replies) != 1 {
		t.Fatalf("history changed: %+v", snap.Requests)
	}
	if _, err := st.Claim("worker"); !errors.Is(err, ErrEmpty) {
		t.Fatalf("claim after accept = %v", err)
	}
}

func TestAcceptRejectsReplyTextWithoutExitVerification(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	if _, err := st.Reply(token, "stored"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.FinishObserved(token, 7, false, true, "bad exit"); err != nil {
		t.Fatal(err)
	}
	if err := st.Accept("worker", platform.Proof{Proven: true, Detail: "injected"}); !errors.Is(err, ErrNotVerified) {
		t.Fatalf("accept = %v", err)
	}
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatal("lock was cleared")
	}
}

func TestRetryKeepsHistoryAndRealStopStaysBlocked(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	if _, err := st.Reply(token, "stored"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.FinishObserved(token, 0, false, true, "not proven"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Retry("worker", "please", platform.Proof{Proven: true}); !errors.Is(err, ErrBadConfirmation) {
		t.Fatalf("retry phrase = %v", err)
	}
	if _, err := st.Retry("worker", RetryConfirmation, platform.Proof{}); !errors.Is(err, ErrRecoveryBlocked) {
		t.Fatalf("retry without proof = %v", err)
	}
	if len(mustSnap(t, st).Requests) != 1 {
		t.Fatal("blocked retry created a request")
	}
	newID, err := st.Retry("worker", RetryConfirmation, platform.Proof{Proven: true, Detail: "injected"})
	if err != nil {
		t.Fatal(err)
	}
	snap := mustSnap(t, st)
	if len(snap.Requests) != 2 || snap.Requests[0]["state"] != "uncertain" || snap.Requests[1]["retry_of"] != "m1" {
		t.Fatalf("retry rows = %+v", snap.Requests)
	}
	if snap.Requests[1]["id"] != newID {
		t.Fatal("new id")
	}
	var warning bool
	for _, event := range snap.Events {
		if event["kind"] == "retry_authorized" && fmt.Sprint(event["detail"]) != "" {
			warning = true
		}
	}
	if !warning {
		t.Fatal("retry warning was not recorded")
	}
	if _, err := st.CurrentMessage(token); !errors.Is(err, ErrRejected) {
		t.Fatal("old mcp context still open")
	}
}

func TestFailClosedPreservesHistoryAndRefusesRetry(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	if _, err := st.Reply(token, "stored"); err != nil {
		t.Fatal(err)
	}
	if err := st.FailClosed("worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CurrentMessage(token); !errors.Is(err, ErrRejected) {
		t.Fatal("mcp still open")
	}
	if _, err := st.Enqueue("m2", "worker", "coordinator", "next"); !errors.Is(err, ErrClosed) {
		t.Fatalf("enqueue = %v", err)
	}
	if _, err := st.Retry("worker", RetryConfirmation, platform.Proof{Proven: true}); !errors.Is(err, ErrClosed) {
		t.Fatalf("retry after fail = %v", err)
	}
	if len(mustSnap(t, st).Replies) != 1 {
		t.Fatal("reply deleted")
	}
}

func TestAcceptRejectsInvalidProviderResult(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	if _, err := st.Reply(token, "stored"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.FinishObserved(token, 0, false, false, "provider result rejected"); err != nil {
		t.Fatal(err)
	}
	if err := st.Accept("worker", platform.Proof{Proven: true, Detail: "injected"}); !errors.Is(err, ErrNotVerified) {
		t.Fatalf("accept = %v", err)
	}
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("invalid result released the lock: %v", err)
	}
}

func TestEmptyGroupDoesNotReleaseTheSession(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	if _, err := st.Reply(token, "stored"); err != nil {
		t.Fatal(err)
	}
	state, err := st.FinishObserved(token, 0, true, true, "group empty")
	if err != nil || state != "uncertain" {
		t.Fatalf("state = %s %v", state, err)
	}
	enqueue(t, st, "worker", "m2")
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second message was reserved: %v", err)
	}
	if _, err := st.CurrentMessage(token); !errors.Is(err, ErrRejected) {
		t.Fatal("mcp stayed open after finish")
	}
}

func TestRestartRevokesPreviousMCP(t *testing.T) {
	st, env := openTest(t)
	register(t, st, env, "worker")
	token := running(t, st, env, "worker", "m1", "conversation-1")
	if _, err := st.Reply(token, "still-open"); err != nil {
		t.Fatal(err)
	}
	n, err := st.RevokeInherited()
	if err != nil || n != 1 {
		t.Fatalf("revoke = %d %v", n, err)
	}
	if _, err := st.Reply(token, "after-restart"); !errors.Is(err, ErrRejected) {
		t.Fatalf("old mcp after restart = %v", err)
	}
	if _, err := st.CurrentMessage(token); !errors.Is(err, ErrRejected) {
		t.Fatal("old mcp can still read")
	}
	enqueue(t, st, "worker", "m2")
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("restart cleared the lock: %v", err)
	}
}

func TestCaseFoldSharesOneSessionKey(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "profile")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	fold := func(string, string, string) bool { return true }
	lower, err := foldWalk(real, fold)
	if err != nil {
		t.Fatal(err)
	}
	upper, err := foldWalk(filepath.Join(root, "Profile"), fold)
	if err != nil {
		t.Fatal(err)
	}
	if lower != upper || upper != real {
		t.Fatalf("case fold = %s %s", lower, upper)
	}
	if _, err := foldWalk(filepath.Join(root, "Profile"), func(string, string, string) bool { return false }); err == nil {
		t.Fatal("case-sensitive walk accepted a different case")
	}
}

func TestPauseSurvivesRestartAndDoesNotDropQueue(t *testing.T) {
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	db := filepath.Join(root, "queue.sqlite")
	st := mustOpen(t, db)
	register(t, st, env, "worker")
	enqueue(t, st, "worker", "m1")
	if err := st.Pause(); err != nil {
		t.Fatal(err)
	}
	st.Close()
	st = mustOpen(t, db)
	if paused, err := st.Paused(); err != nil || !paused {
		t.Fatalf("paused = %t %v", paused, err)
	}
	if _, err := st.Claim("worker"); !errors.Is(err, ErrPaused) {
		t.Fatalf("claim while paused = %v", err)
	}
	if mustSnap(t, st).Requests[0]["state"] != "queued" {
		t.Fatal("queue was not preserved")
	}
	if err := st.ResumeWork(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Claim("worker"); err != nil {
		t.Fatal(err)
	}
}

func TestSixProcessesOnlyOneClaim(t *testing.T) {
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	db := filepath.Join(root, "queue.sqlite")
	st := mustOpen(t, db)
	register(t, st, env, "worker")
	enqueue(t, st, "worker", "m1")
	enqueue(t, st, "worker", "m2")
	barrier := filepath.Join(root, "barrier")
	var cmds []*exec.Cmd
	var outs []*os.File
	for i := 0; i < 6; i++ {
		ready := filepath.Join(root, "ready-"+strconv.Itoa(i))
		out, err := os.Create(filepath.Join(root, "out-"+strconv.Itoa(i)))
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(),
			"MESSENGER_FIXTURE=claim",
			"MESSENGER_DB="+db,
			"MESSENGER_ALIAS=worker",
			"MESSENGER_BARRIER="+barrier,
			"MESSENGER_READY="+ready,
		)
		cmd.Stdout = out
		cmd.Stderr = out
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		cmds = append(cmds, cmd)
		outs = append(outs, out)
	}
	t.Cleanup(func() {
		for _, cmd := range cmds {
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
		}
	})
	waitFiles(t, root, "ready-", 6)
	if err := os.WriteFile(barrier, []byte("go"), 0o600); err != nil {
		t.Fatal(err)
	}
	wins := 0
	for i, cmd := range cmds {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
		outs[i].Close()
		text, err := os.ReadFile(outs[i].Name())
		if err != nil {
			t.Fatal(err)
		}
		if len(text) > 0 && text[0] != 'B' && text[0] != 'E' {
			wins++
		}
	}
	if wins != 1 {
		t.Fatalf("claims won = %d", wins)
	}
	states := requestStates(mustSnap(t, st))
	if states[0] != "reserved" || states[1] != "queued" {
		t.Fatalf("states = %v", states)
	}
}

func TestCrashLeavesSessionLockedAndChildCanOutliveSupervisor(t *testing.T) {
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	db := filepath.Join(root, "queue.sqlite")
	st := mustOpen(t, db)
	register(t, st, env, "worker")
	enqueue(t, st, "worker", "m1")
	enqueue(t, st, "worker", "m2")
	cmd, dir := startFixture(t, root, "hold", db)
	waitFile(t, filepath.Join(dir, "ready"))
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("claim after crash = %v", err)
	}

	root = t.TempDir()
	env = mkdir(t, filepath.Join(root, "cli"))
	db = filepath.Join(root, "queue.sqlite")
	st = mustOpen(t, db)
	register(t, st, env, "worker")
	enqueue(t, st, "worker", "m1")
	cmd, dir = startFixture(t, root, "orphan", db)
	waitFile(t, filepath.Join(dir, "ready"))
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	before := waitHeartbeat(t, dir)
	time.Sleep(80 * time.Millisecond)
	after, err := os.ReadFile(filepath.Join(dir, "heartbeat"))
	if err != nil || string(after) == before {
		t.Fatalf("child did not survive supervisor: %s %v", after, err)
	}
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("claim with live child = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "release"), []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	waitFile(t, filepath.Join(dir, "child-exited"))
	if _, err := st.Claim("worker"); !errors.Is(err, ErrBusy) {
		t.Fatalf("claim after child exit = %v", err)
	}
}

func openTest(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	env := mkdir(t, filepath.Join(root, "cli"))
	return mustOpen(t, filepath.Join(root, "queue.sqlite")), env
}

func mustOpen(t *testing.T, path string) *Store {
	t.Helper()
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func register(t *testing.T, st *Store, env, alias string) {
	t.Helper()
	if _, err := st.Register("synthetic", env, "conversation-1", alias); err != nil {
		t.Fatal(err)
	}
}

func enqueue(t *testing.T, st *Store, alias, id string) {
	t.Helper()
	if _, err := st.Enqueue(id, alias, "coordinator", "request "+id); err != nil {
		t.Fatal(err)
	}
}

func running(t *testing.T, st *Store, env, alias, id, conversation string) string {
	t.Helper()
	enqueue(t, st, alias, id)
	return claimBeginConfirm(t, st, env, alias, conversation)
}

func claimBeginConfirm(t *testing.T, st *Store, env, alias, conversation string) string {
	t.Helper()
	token, err := st.Claim(alias)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BeginLaunch(token); err != nil {
		t.Fatal(err)
	}
	if err := st.ConfirmIdentity(token, "synthetic", env, conversation); err != nil {
		t.Fatal(err)
	}
	return token
}

func mustSnap(t *testing.T, st *Store) Snapshot {
	t.Helper()
	snap, err := st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func requestStates(snap Snapshot) []string {
	var states []string
	for _, row := range snap.Requests {
		states = append(states, fmt.Sprint(row["state"]))
	}
	return states
}

func startFixture(t *testing.T, root, mode, db string) (*exec.Cmd, string) {
	t.Helper()
	dir := mkdir(t, filepath.Join(root, mode))
	cmd := exec.Command(os.Args[0])
	cmd.Env = append(os.Environ(),
		"MESSENGER_FIXTURE="+mode,
		"MESSENGER_DB="+db,
		"MESSENGER_ALIAS=worker",
		"MESSENGER_DIR="+dir,
	)
	cmd.Stdout = os.Stdout
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(dir, "release"), []byte("1"), 0o600)
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	})
	return cmd, dir
}

func waitFiles(t *testing.T, root, prefix string, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		count := 0
		for i := 0; i < n; i++ {
			if _, err := os.Stat(filepath.Join(root, prefix+strconv.Itoa(i))); err == nil {
				count++
			}
		}
		if count == n {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for fixture files")
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

func waitHeartbeat(t *testing.T, dir string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		text, err := os.ReadFile(filepath.Join(dir, "heartbeat"))
		if err == nil && len(text) > 0 {
			return string(text)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("heartbeat missing")
	return ""
}

func runFixture(mode string) error {
	switch mode {
	case "claim":
		return fixtureClaim()
	case "hold":
		return fixtureHold(false)
	case "orphan":
		return fixtureHold(true)
	case "heartbeat":
		return fixtureHeartbeat()
	default:
		return fmt.Errorf("unknown fixture %s", mode)
	}
}

func fixtureClaim() error {
	ready := os.Getenv("MESSENGER_READY")
	if err := os.WriteFile(ready, []byte("1"), 0o600); err != nil {
		return err
	}
	barrier := os.Getenv("MESSENGER_BARRIER")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(barrier); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return errors.New("barrier timeout")
		}
		time.Sleep(5 * time.Millisecond)
	}
	st, err := Open(os.Getenv("MESSENGER_DB"))
	if err != nil {
		return err
	}
	defer st.Close()
	token, err := st.Claim(os.Getenv("MESSENGER_ALIAS"))
	if err != nil {
		fmt.Print("BUSY")
		return nil
	}
	fmt.Print(token)
	return nil
}

func fixtureHold(orphan bool) error {
	st, err := Open(os.Getenv("MESSENGER_DB"))
	if err != nil {
		return err
	}
	defer st.Close()
	token, err := st.Claim(os.Getenv("MESSENGER_ALIAS"))
	if err != nil {
		return err
	}
	if err := st.BeginLaunch(token); err != nil {
		return err
	}
	dir := os.Getenv("MESSENGER_DIR")
	if orphan {
		cmd := exec.Command(os.Args[0])
		cmd.Env = append(os.Environ(), "MESSENGER_FIXTURE=heartbeat", "MESSENGER_DIR="+dir)
		if err := cmd.Start(); err != nil {
			return err
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "ready"), []byte("1"), 0o600); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Stat(filepath.Join(dir, "release")); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("hold timeout")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func fixtureHeartbeat() error {
	dir := os.Getenv("MESSENGER_DIR")
	for i := 1; ; i++ {
		if _, err := os.Stat(filepath.Join(dir, "release")); err == nil {
			return os.WriteFile(filepath.Join(dir, "child-exited"), []byte("1"), 0o600)
		}
		if err := os.WriteFile(filepath.Join(dir, "heartbeat"), []byte(strconv.Itoa(i)), 0o600); err != nil {
			return err
		}
		time.Sleep(20 * time.Millisecond)
		if i > 500 {
			return errors.New("heartbeat timeout")
		}
	}
}
