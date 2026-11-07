package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const testSetupID = "11111111-2222-4333-8444-555555555555"
const testThread = "01a1b0a0-1ef6-7861-8fd1-f7fb5912e2f9"

type codexFixture struct {
	st                    *Store
	path, env, cli, space string
}

func newCodexFixture(t *testing.T) codexFixture {
	t.Helper()
	root := t.TempDir()
	f := codexFixture{path: filepath.Join(root, "queue.sqlite")}
	f.env = filepath.Join(root, "profile")
	f.space = filepath.Join(root, "workspace")
	for _, dir := range []string{f.env, f.space} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	f.cli = filepath.Join(root, exeName("codex"))
	if err := os.WriteFile(f.cli, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	st, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	f.st = st
	return f
}

func (f codexFixture) state(t *testing.T) string {
	t.Helper()
	c, err := f.st.CodexSetup(testSetupID)
	if err != nil {
		t.Fatal(err)
	}
	return c.State
}

var testObs = platform.Observation{PID: 4242, PGID: 4242, StartToken: "start", BootID: "boot"}

func (f codexFixture) toCreated(t *testing.T) {
	t.Helper()
	steps := []func() error{
		func() error { return f.st.BeginCodexSetup(testSetupID, "codex-test", f.env, f.cli, f.space) },
		func() error { return f.st.MarkCodexSetupLaunching(testSetupID) },
		func() error { return f.st.RecordCodexSetupProcess(testSetupID, testObs) },
		func() error { return f.st.ConfirmCodexSetupConfig(testSetupID, fingerprint) },
		func() error { return f.st.PrepareCodexThreadStart(testSetupID) },
		func() error { return f.st.CompleteCodexSetup(testSetupID, testThread) },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
}

const fingerprint = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestCodexSetupRegistersOwnedSession(t *testing.T) {
	f := newCodexFixture(t)
	f.toCreated(t)
	c, err := f.st.CodexSetup(testSetupID)
	if err != nil || c.State != "created" || c.ThreadID != testThread {
		t.Fatalf("setup %+v %v", c, err)
	}
	snap, err := f.st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Sessions) != 1 || snap.Sessions[0]["start_mode"] != "codex_owned" || snap.Sessions[0]["provider"] != "codex" || snap.Sessions[0]["cli_path"] != nil {
		t.Fatalf("sessions %+v", snap.Sessions)
	}
	if owned, err := f.st.CodexOwnedAlias("codex-test"); err != nil || !owned {
		t.Fatalf("owned %v %v", owned, err)
	}
	if _, err := f.st.Enqueue("m1", "codex-test", "user", "hi"); err != nil {
		t.Fatal(err)
	}
	// Only the owner may take it: generic claims and launch configuration refuse.
	if _, err := f.st.Claim("codex-test"); !errors.Is(err, ErrRejected) {
		t.Fatalf("generic claim: %v", err)
	}
	if _, err := f.st.ClaimNext(); !errors.Is(err, ErrEmpty) {
		t.Fatalf("claim next: %v", err)
	}
	if err := f.st.ConfigureLaunch("codex-test", f.cli, ""); !errors.Is(err, ErrRejected) {
		t.Fatalf("configure launch: %v", err)
	}
}

func TestCodexTurnBindsReplyToAttempt(t *testing.T) {
	f := newCodexFixture(t)
	f.toCreated(t)
	if _, err := f.st.ClaimCodexTurn(testSetupID); !errors.Is(err, ErrEmpty) {
		t.Fatalf("claim on empty queue: %v", err)
	}
	if _, err := f.st.Enqueue("m1", "codex-test", "user", "hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.Enqueue("m2", "codex-test", "user", "second"); err != nil {
		t.Fatal(err)
	}
	token, err := f.st.ClaimCodexTurn(testSetupID)
	if err != nil {
		t.Fatal(err)
	}
	if f.state(t) != "busy" {
		t.Fatal("owner not busy")
	}
	if _, err := f.st.ClaimCodexTurn(testSetupID); !errors.Is(err, ErrRejected) {
		t.Fatalf("second claim while busy: %v", err)
	}
	if err := f.st.BindCodexTurn(token, "turn-1"); !errors.Is(err, ErrRejected) {
		t.Fatalf("bind before send intent: %v", err)
	}
	if err := f.st.PrepareCodexTurnSend(token); err != nil {
		t.Fatal(err)
	}
	if err := f.st.PrepareCodexTurnSend(token); !errors.Is(err, ErrRejected) {
		t.Fatalf("second send intent: %v", err)
	}
	if _, _, err := f.st.CodexRead(token); !errors.Is(err, ErrRejected) {
		t.Fatalf("read before bind: %v", err)
	}
	if err := f.st.BindCodexTurn(token, "turn-1"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.BindCodexTurn(token, "turn-2"); !errors.Is(err, ErrRejected) {
		t.Fatalf("rebind: %v", err)
	}
	if err := f.st.CodexReply(token, "x", "early"); !errors.Is(err, ErrRejected) {
		t.Fatalf("reply before read: %v", err)
	}
	if err := f.st.CompleteCodexTurn(token); !errors.Is(err, ErrNotVerified) {
		t.Fatalf("complete without reply: %v", err)
	}
	text, receipt, err := f.st.CodexRead(token)
	if err != nil || text != "hello" || receipt == "" {
		t.Fatalf("read %q %q %v", text, receipt, err)
	}
	if err := f.st.CodexReply(token, "wrong", "ack"); !errors.Is(err, ErrRejected) {
		t.Fatalf("wrong receipt: %v", err)
	}
	if err := f.st.CodexReply(token, receipt, "ack"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.CompleteCodexTurn(token); err != nil {
		t.Fatal(err)
	}
	if f.state(t) != "created" {
		t.Fatal("owner not idle after the turn")
	}
	if _, _, err := f.st.CodexRead(token); !errors.Is(err, ErrRejected) {
		t.Fatalf("tools still open after completion: %v", err)
	}
	snap, _ := f.st.Snapshot()
	if len(snap.Replies) != 1 || snap.Replies[0]["recipient"] != "user" || snap.Replies[0]["sender"] != "codex-test" {
		t.Fatalf("replies %+v", snap.Replies)
	}
	next, err := f.st.ClaimCodexTurn(testSetupID)
	if err != nil || next == token {
		t.Fatalf("next claim %q %v", next, err)
	}
}

func TestCodexTurnFailureLocksSession(t *testing.T) {
	f := newCodexFixture(t)
	f.toCreated(t)
	for _, id := range []string{"m1", "m2"} {
		if _, err := f.st.Enqueue(id, "codex-test", "user", id); err != nil {
			t.Fatal(err)
		}
	}
	token, err := f.st.ClaimCodexTurn(testSetupID)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.PrepareCodexTurnSend(token); err != nil {
		t.Fatal(err)
	}
	if err := f.st.BindCodexTurn(token, "turn-1"); err != nil {
		t.Fatal(err)
	}
	_, receipt, err := f.st.CodexRead(token)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.CodexReply(token, receipt, "partial"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.FailCodexTurn(token, "EOF"); err != nil {
		t.Fatal(err)
	}
	// T-022: the reply of a failed turn stays held.
	if snap, _ := f.st.Snapshot(); len(snap.Replies) != 1 || snap.Replies[0]["held"] != int64(1) {
		t.Fatalf("replies %+v", snap.Replies)
	}
	if f.state(t) != "uncertain" {
		t.Fatal("owner not uncertain")
	}
	if _, err := f.st.ClaimCodexTurn(testSetupID); !errors.Is(err, ErrRejected) {
		t.Fatalf("claim after failure: %v", err)
	}
	if _, err := f.st.Claim("codex-test"); err == nil {
		t.Fatal("generic claim after failure")
	}
	snap, _ := f.st.Snapshot()
	states := map[string]any{}
	for _, r := range snap.Requests {
		states[r["id"].(string)] = r["state"]
	}
	if states["m1"] != "uncertain" || states["m2"] != "queued" {
		t.Fatalf("requests %+v", states)
	}
	// A restart keeps the attempt uncertain and does not release the session.
	f.st.Close()
	st, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.RevokeInherited(); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Claim("codex-test"); err == nil {
		t.Fatal("claim after restart")
	}
}

func TestCodexSetupBeginIsIdempotentOnlyWhenIdentical(t *testing.T) {
	f := newCodexFixture(t)
	if err := f.st.BeginCodexSetup(testSetupID, "codex-test", f.env, f.cli, f.space); err != nil {
		t.Fatal(err)
	}
	if err := f.st.BeginCodexSetup(testSetupID, "codex-test", f.env, f.cli, f.space); err != nil {
		t.Fatalf("identical repeat rejected: %v", err)
	}
	if err := f.st.BeginCodexSetup(testSetupID, "other", f.env, f.cli, f.space); !errors.Is(err, ErrRejected) {
		t.Fatalf("reused id accepted: %v", err)
	}
	if err := f.st.MarkCodexSetupLaunching(testSetupID); err != nil {
		t.Fatal(err)
	}
	if err := f.st.BeginCodexSetup(testSetupID, "codex-test", f.env, f.cli, f.space); !errors.Is(err, ErrRejected) {
		t.Fatalf("repeat after launching accepted: %v", err)
	}
	if err := f.st.BeginCodexSetup("not-a-uuid", "x", f.env, f.cli, f.space); err == nil {
		t.Fatal("invalid id accepted")
	}
}

func TestCodexSetupAliasIsExclusive(t *testing.T) {
	f := newCodexFixture(t)
	if _, err := f.st.Register("claude", f.env, "conversation-1", "taken"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.BeginCodexSetup(testSetupID, "taken", f.env, f.cli, f.space); !errors.Is(err, ErrRejected) {
		t.Fatalf("bound alias reused: %v", err)
	}
	if err := f.st.BeginCodexSetup(testSetupID, "codex-test", f.env, f.cli, f.space); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.Register("claude", f.env, "conversation-2", "codex-test"); !errors.Is(err, ErrRejected) {
		t.Fatalf("register took a reserved alias: %v", err)
	}
	if err := f.st.SetupNewClaude(f.env, "22222222-3333-4444-8555-666666666666", "codex-test", f.cli, f.space); !errors.Is(err, ErrRejected) {
		t.Fatalf("Claude setup took a reserved alias: %v", err)
	}
	other := "33333333-4444-4555-8666-777777777777"
	if err := f.st.BeginCodexSetup(other, "codex-test", f.env, f.cli, f.space); !errors.Is(err, ErrRejected) {
		t.Fatalf("second setup took a reserved alias: %v", err)
	}
}

func TestCodexSetupThreadCannotBeRegisteredTwice(t *testing.T) {
	f := newCodexFixture(t)
	f.toCreated(t)
	if _, err := f.st.Register("codex", f.env, testThread, "second-alias"); !errors.Is(err, ErrRejected) {
		t.Fatalf("owned thread registered as an ordinary session: %v", err)
	}
}

func TestCodexSetupCompleteRejectsAlreadyRegisteredThread(t *testing.T) {
	f := newCodexFixture(t)
	if _, err := f.st.Register("codex", f.env, testThread, "earlier"); err != nil {
		t.Fatal(err)
	}
	for _, step := range []func() error{
		func() error { return f.st.BeginCodexSetup(testSetupID, "codex-test", f.env, f.cli, f.space) },
		func() error { return f.st.MarkCodexSetupLaunching(testSetupID) },
		func() error { return f.st.RecordCodexSetupProcess(testSetupID, testObs) },
		func() error { return f.st.ConfirmCodexSetupConfig(testSetupID, fingerprint) },
		func() error { return f.st.PrepareCodexThreadStart(testSetupID) },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.st.CompleteCodexSetup(testSetupID, testThread); !errors.Is(err, ErrRejected) {
		t.Fatalf("duplicate thread accepted: %v", err)
	}
	if f.state(t) != "thread_requested" {
		t.Fatal("failed completion changed state")
	}
}

func TestCodexSetupStepsMustBeInOrder(t *testing.T) {
	f := newCodexFixture(t)
	if err := f.st.BeginCodexSetup(testSetupID, "codex-test", f.env, f.cli, f.space); err != nil {
		t.Fatal(err)
	}
	for name, step := range map[string]func() error{
		"process before launching": func() error { return f.st.RecordCodexSetupProcess(testSetupID, testObs) },
		"config before process":    func() error { return f.st.ConfirmCodexSetupConfig(testSetupID, fingerprint) },
		"thread before config":     func() error { return f.st.PrepareCodexThreadStart(testSetupID) },
		"complete before request":  func() error { return f.st.CompleteCodexSetup(testSetupID, testThread) },
	} {
		if err := step(); !errors.Is(err, ErrRejected) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := f.st.MarkCodexSetupLaunching(testSetupID); err != nil {
		t.Fatal(err)
	}
	if err := f.st.RecordCodexSetupProcess(testSetupID, platform.Observation{PID: 1}); !errors.Is(err, ErrRejected) {
		t.Fatalf("incomplete observation accepted: %v", err)
	}
	if err := f.st.RecordCodexSetupProcess(testSetupID, testObs); err != nil {
		t.Fatal(err)
	}
	if err := f.st.ConfirmCodexSetupConfig(testSetupID, "not-a-hash"); !errors.Is(err, ErrRejected) {
		t.Fatalf("bad fingerprint accepted: %v", err)
	}
	if err := f.st.ConfirmCodexSetupConfig(testSetupID, fingerprint); err != nil {
		t.Fatal(err)
	}
	if err := f.st.PrepareCodexThreadStart(testSetupID); err != nil {
		t.Fatal(err)
	}
	if err := f.st.PrepareCodexThreadStart(testSetupID); !errors.Is(err, ErrRejected) {
		t.Fatalf("thread/start intent recorded twice: %v", err)
	}
}

func TestCodexSetupNotStartedOnlyWithoutProcess(t *testing.T) {
	f := newCodexFixture(t)
	if err := f.st.BeginCodexSetup(testSetupID, "codex-test", f.env, f.cli, f.space); err != nil {
		t.Fatal(err)
	}
	if err := f.st.MarkCodexSetupLaunching(testSetupID); err != nil {
		t.Fatal(err)
	}
	if err := f.st.CodexSetupNotStarted(testSetupID, "exec refused"); err != nil {
		t.Fatal(err)
	}
	// The alias is released and can be used again.
	if err := f.st.BeginCodexSetup("33333333-4444-4555-8666-777777777777", "codex-test", f.env, f.cli, f.space); err != nil {
		t.Fatalf("alias not released: %v", err)
	}

	g := newCodexFixture(t)
	if err := g.st.BeginCodexSetup(testSetupID, "codex-test", g.env, g.cli, g.space); err != nil {
		t.Fatal(err)
	}
	_ = g.st.MarkCodexSetupLaunching(testSetupID)
	_ = g.st.RecordCodexSetupProcess(testSetupID, testObs)
	if err := g.st.CodexSetupNotStarted(testSetupID, "late"); !errors.Is(err, ErrRejected) {
		t.Fatalf("started process released: %v", err)
	}
}

func TestCodexSetupUncertainKeepsAlias(t *testing.T) {
	f := newCodexFixture(t)
	f.toCreated(t)
	if err := f.st.RecordCodexSetupExit(testSetupID, 0); err != nil {
		t.Fatal(err)
	}
	c, err := f.st.CodexSetup(testSetupID)
	if err != nil || c.State != "uncertain" || !c.ExitObserved {
		t.Fatalf("setup %+v %v", c, err)
	}
	if _, err := f.st.Register("claude", f.env, "conversation-3", "codex-test"); !errors.Is(err, ErrRejected) {
		t.Fatalf("uncertain setup released its alias: %v", err)
	}
	if err := f.st.CodexSetupNotStarted(testSetupID, "x"); err == nil {
		t.Fatal("uncertain setup released")
	}
}

func TestCodexSetupRestartMarksInheritedUncertain(t *testing.T) {
	f := newCodexFixture(t)
	f.toCreated(t)
	second := "33333333-4444-4555-8666-777777777777"
	if err := f.st.BeginCodexSetup(second, "never-launched", f.env, f.cli, f.space); err != nil {
		t.Fatal(err)
	}
	f.st.Close()
	st, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.RevokeInherited(); err != nil {
		t.Fatal(err)
	}
	created, _ := st.CodexSetup(testSetupID)
	intent, _ := st.CodexSetup(second)
	if created.State != "uncertain" || created.ThreadID != testThread {
		t.Fatalf("created setup after restart: %+v", created)
	}
	if intent.State != "not_started" {
		t.Fatalf("intent after restart: %+v", intent)
	}
}
