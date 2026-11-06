//go:build darwin || linux

package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

type codexHarness struct {
	t                   *testing.T
	root, profile, self string
	errc                chan error
	cancel              context.CancelFunc
}

func newCodexHarness(t *testing.T, mode string, launcher Launcher) *codexHarness {
	t.Helper()
	return newCodexHarnessWith(t, mode, launcher, nil)
}

// newCodexHarnessWith runs prepare on the data directory before the client starts.
func newCodexHarnessWith(t *testing.T, mode string, launcher Launcher, prepare func(root string)) *codexHarness {
	t.Helper()
	isolateUser(t)
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// A short root keeps the control socket path under the Unix limit
	// (108 bytes on Linux); t.TempDir() embeds the long subtest name.
	root, err := os.MkdirTemp("", "cx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	h := &codexHarness{t: t, root: root, self: self}
	h.profile = mkdir(t, filepath.Join(t.TempDir(), "managed-codex"))
	if err := os.WriteFile(filepath.Join(h.profile, "fixture-mode"), []byte(mode), 0o600); err != nil {
		t.Fatal(err)
	}
	if prepare != nil {
		prepare(root)
	}
	h.start(launcher)
	return h
}

func (h *codexHarness) start(launcher Launcher) {
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	h.errc = make(chan error, 1)
	go func() { h.errc <- Run(ctx, h.root, launcher) }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-h.errc:
			h.t.Fatalf("supervisor exited at startup: %v", err)
		default:
		}
		if socketAccepts(h.root) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	h.t.Fatal("command socket was not created")
}

func (h *codexHarness) stop() {
	h.t.Helper()
	_, _ = control.Call(h.root, control.Request{Cmd: "shutdown"})
	select {
	case <-h.errc:
	case <-time.After(5 * time.Second):
		h.cancel()
		h.t.Fatal("supervisor did not exit")
	}
}

func (h *codexHarness) create(alias string) control.Response {
	h.t.Helper()
	response, err := control.Call(h.root, control.Request{Cmd: "create-codex-session", Environment: h.profile, Alias: alias, CLI: h.self})
	if err != nil {
		h.t.Fatal(err)
	}
	return response
}

func (h *codexHarness) setups() []map[string]any {
	h.t.Helper()
	response, err := control.Call(h.root, control.Request{Cmd: "inspect"})
	if err != nil || !response.OK {
		h.t.Fatalf("inspect %v %+v", err, response)
	}
	var snap struct{ CodexSetups []map[string]any }
	if err := json.Unmarshal(response.Inspect, &snap); err != nil {
		h.t.Fatal(err)
	}
	return snap.CodexSetups
}

func (h *codexHarness) waitSetup(done func(map[string]any) bool) map[string]any {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last []map[string]any
	for time.Now().Before(deadline) {
		last = h.setups()
		if len(last) == 1 && done(last[0]) {
			return last[0]
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("setup did not reach the expected state: %+v", last)
	return nil
}

func (h *codexHarness) log() []string {
	raw, _ := os.ReadFile(filepath.Join(h.profile, "fixture-log"))
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func (h *codexHarness) count(method string) int {
	n := 0
	for _, line := range h.log() {
		if line == method {
			n++
		}
	}
	return n
}

// waitGone waits for the fake server to exit so the temp dirs can be removed.
func waitGone(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("fake app-server %d did not exit", pid)
}

func pidOf(setup map[string]any) int {
	v, _ := setup["pid"].(float64)
	return int(v)
}

func codexReservations(t *testing.T) []recordedRun {
	t.Helper()
	runs, err := loadRuns()
	if err != nil {
		t.Fatal(err)
	}
	var out []recordedRun
	for _, run := range runs {
		if run.CodexSetupID != "" {
			out = append(out, run)
		}
	}
	return out
}

func TestCreateCodexSessionOwnsThreadWithoutModelTurn(t *testing.T) {
	h := newCodexHarness(t, "ok", AdapterLauncher{AllowModels: true})
	response := h.create("codex-test")
	if !response.OK {
		t.Fatalf("create failed: %+v", response)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	if setup["thread_id"] != fakeThreadID {
		t.Fatalf("thread not stored: %+v", setup)
	}
	for method, want := range map[string]int{"initialize": 1, "initialized": 1, "config/read": 1, "thread/start": 1, "turn/start": 0} {
		if got := h.count(method); got != want {
			t.Errorf("%s sent %d times, want %d", method, got, want)
		}
	}
	log := h.log()
	if log[0] != "env:CODEX_HOME,HOME,PATH,TMPDIR" {
		t.Errorf("app-server inherited extra environment: %s", log[0])
	}
	if log[1] != "args:app-server --listen stdio://" {
		t.Errorf("unexpected args: %s", log[1])
	}
	runs := codexReservations(t)
	if len(runs) != 1 || runs[0].ConversationID != fakeThreadID || runs[0].PID != pidOf(setup) || runs[0].Provider != "codex" {
		t.Fatalf("reservation %+v", runs)
	}
	// The thread cannot be registered a second time or reconfigured.
	if r, _ := control.Call(h.root, control.Request{Cmd: "register-session", Provider: "codex", Environment: h.profile, ConversationID: fakeThreadID, Alias: "dup", CLI: h.self}); r.OK {
		t.Fatal("owned thread registered as an ordinary session")
	}
	if r := h.create("codex-test"); r.OK {
		t.Fatal("alias reused by a second setup")
	}
	h.stop()
	waitGone(t, pidOf(setup))

	// A restart cannot reach the old owner: the setup becomes uncertain, and the
	// reservation stays, so another data directory is still refused.
	h.start(AdapterLauncher{AllowModels: true})
	after := h.waitSetup(func(s map[string]any) bool { return s["state"] == "uncertain" })
	if after["thread_id"] != fakeThreadID {
		t.Fatalf("thread id lost: %+v", after)
	}
	h.stop()
	if len(codexReservations(t)) != 1 {
		t.Fatal("reservation removed")
	}
	other := t.TempDir()
	if err := Run(context.Background(), other, AdapterLauncher{}); !errors.Is(err, ErrLiveCLI) {
		t.Fatalf("other data directory started despite the reservation: %v", err)
	}
}

func TestCreateCodexSessionFailuresStayUncertainWithoutRetry(t *testing.T) {
	old := codexSetupTimeout
	codexSetupTimeout = 300 * time.Millisecond
	t.Cleanup(func() { codexSetupTimeout = old })
	for _, tc := range []struct {
		mode    string
		threads int
	}{
		{"mcp", 0},
		{"server-request", 0},
		{"silent-thread", 1},
		{"exit-on-thread", 1},
		{"ephemeral", 1},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			h := newCodexHarness(t, tc.mode, AdapterLauncher{AllowModels: true})
			if r := h.create("codex-test"); r.OK {
				t.Fatalf("setup succeeded: %+v", r)
			}
			setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "uncertain" && s["exit_observed"] == float64(1) })
			if setup["thread_id"] != nil {
				t.Fatalf("thread stored after failure: %+v", setup)
			}
			if got := h.count("thread/start"); got != tc.threads {
				t.Fatalf("thread/start sent %d times, want %d", got, tc.threads)
			}
			if h.count("turn/start") != 0 {
				t.Fatal("model turn requested")
			}
			if len(codexReservations(t)) != 1 {
				t.Fatal("reservation removed after an uncertain setup")
			}
			if r := h.create("codex-test"); r.OK {
				t.Fatal("uncertain alias reused")
			}
			h.stop()
			waitGone(t, pidOf(setup))
		})
	}
}

func TestCreateCodexSessionIdleServerRequestMakesSetupUncertain(t *testing.T) {
	h := newCodexHarness(t, "idle-request", AdapterLauncher{AllowModels: true})
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "uncertain" && s["exit_observed"] == float64(1) })
	if setup["thread_id"] != fakeThreadID {
		t.Fatalf("thread id lost: %+v", setup)
	}
	h.stop()
	waitGone(t, pidOf(setup))
}

func TestCreateCodexSessionRefusesWithoutLaunching(t *testing.T) {
	h := newCodexHarness(t, "ok", AdapterLauncher{})
	if r := h.create("codex-test"); r.OK {
		t.Fatal("setup ran without --allow-model-calls")
	}
	h.stop()
	h.start(AdapterLauncher{AllowModels: true, OnlySession: "other"})
	if r := h.create("codex-test"); r.OK {
		t.Fatal("setup ran outside the authorized session")
	}
	defaultProfile := mkdir(t, filepath.Join(os.Getenv("HOME"), ".codex"))
	if r, _ := control.Call(h.root, control.Request{Cmd: "create-codex-session", Environment: defaultProfile, Alias: "codex-test", CLI: h.self}); r.OK {
		t.Fatal("default Codex profile accepted")
	}
	if setups := h.setups(); len(setups) != 0 {
		t.Fatalf("refused setups were recorded: %+v", setups)
	}
	if _, err := os.Stat(filepath.Join(h.profile, "fixture-log")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("app-server was launched")
	}
	if len(codexReservations(t)) != 0 {
		t.Fatal("reservation recorded for a refused setup")
	}
	h.stop()
}

// Opt-in check against an installed Codex CLI. It uses an empty managed
// profile with no login, so no model can be called, and never sends turn/start.
func TestInstalledCodexCreateSession(t *testing.T) {
	cli := os.Getenv("MESSENGER_CODEX_PROBE")
	if cli == "" {
		t.Skip("set MESSENGER_CODEX_PROBE to an absolute Codex executable for a model-free setup check")
	}
	h := newCodexHarness(t, "", AdapterLauncher{AllowModels: true})
	h.self = cli
	response := h.create("codex-installed")
	if !response.OK {
		t.Fatalf("create failed: %+v", response)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	thread, _ := setup["thread_id"].(string)
	if len(thread) != 36 {
		t.Fatal("no thread id stored")
	}
	runs := codexReservations(t)
	if len(runs) != 1 || runs[0].ConversationID != thread {
		t.Fatal("reservation does not name the thread")
	}
	h.stop()
	waitGone(t, pidOf(setup))
}

func (h *codexHarness) send(id, text string) {
	h.t.Helper()
	r, err := control.Call(h.root, control.Request{Cmd: "enqueue", Alias: "codex-test", MessageID: id, Sender: "user", Text: text})
	if err != nil || !r.OK {
		h.t.Fatalf("enqueue %s: %+v %v", id, r, err)
	}
}

type codexSnap struct {
	Events        []map[string]any
	StreamRuns    []map[string]any
	CodexSetups   []map[string]any
	Requests      []map[string]any
	Replies       []map[string]any
	Verifications []map[string]any
	Sessions      []map[string]any
}

func (h *codexHarness) snap() codexSnap {
	h.t.Helper()
	response, err := control.Call(h.root, control.Request{Cmd: "inspect"})
	if err != nil || !response.OK {
		h.t.Fatalf("inspect %v %+v", err, response)
	}
	var s codexSnap
	if err := json.Unmarshal(response.Inspect, &s); err != nil {
		h.t.Fatal(err)
	}
	return s
}

func (h *codexHarness) waitSnap(what string, done func(codexSnap) bool) codexSnap {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last codexSnap
	for time.Now().Before(deadline) {
		last = h.snap()
		if done(last) {
			return last
		}
		time.Sleep(20 * time.Millisecond)
	}
	h.t.Fatalf("%s: %+v", what, last)
	return last
}

func requestState(s codexSnap, id string) string {
	for _, r := range s.Requests {
		if r["id"] == id {
			state, _ := r["state"].(string)
			return state
		}
	}
	return ""
}

func replyTo(s codexSnap, id string) string {
	for _, r := range s.Replies {
		if r["reply_to"] == id {
			text, _ := r["text"].(string)
			return text
		}
	}
	return ""
}

func TestCodexOwnerDeliversTwoMessagesInOneProcess(t *testing.T) {
	for _, mode := range []string{"ok", "early-call", "approval"} {
		t.Run(mode, func(t *testing.T) {
			h := newCodexHarness(t, mode, AdapterLauncher{AllowModels: true})
			if r := h.create("codex-test"); !r.OK {
				t.Fatalf("create failed: %+v", r)
			}
			setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
			h.send("m1", "one")
			h.waitSnap("first reply", func(s codexSnap) bool {
				return requestState(s, "m1") == "completed" && s.CodexSetups[0]["state"] == "created"
			})
			h.send("m2", "two")
			s := h.waitSnap("second reply", func(s codexSnap) bool {
				return requestState(s, "m2") == "completed" && s.CodexSetups[0]["state"] == "created"
			})
			if replyTo(s, "m1") != "ACK:one" || replyTo(s, "m2") != "ACK:two" {
				t.Fatalf("replies %+v", s.Replies)
			}
			for _, r := range s.Replies {
				if r["sender"] != "codex-test" || r["recipient"] != "user" {
					t.Fatalf("reply route %+v", r)
				}
			}
			ok := 0
			for _, v := range s.Verifications {
				if v["provider_ok"] == float64(1) && v["exit_observed"] == float64(0) {
					ok++
				}
			}
			if ok != 2 {
				t.Fatalf("verifications %+v", s.Verifications)
			}
			if h.count("turn/start") != 2 {
				t.Fatalf("turn/start sent %d times", h.count("turn/start"))
			}
			if mode == "approval" && h.count("response:1015:error") != 1 {
				t.Fatalf("approval request was not refused: %v", h.log())
			}
			for _, sess := range s.Sessions {
				if sess["start_mode"] != "codex_owned" || sess["provider"] != "codex" {
					t.Fatalf("session %+v", sess)
				}
			}
			// One process served both turns.
			if pidOf(s.CodexSetups[0]) != pidOf(setup) {
				t.Fatal("owner process changed")
			}
			h.stop()
			waitGone(t, pidOf(setup))
		})
	}
}

func TestCodexOwnerPauseHoldsMessages(t *testing.T) {
	h := newCodexHarness(t, "ok", AdapterLauncher{AllowModels: true})
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	if r, _ := control.Call(h.root, control.Request{Cmd: "pause"}); !r.OK {
		t.Fatal("pause failed")
	}
	h.send("m1", "one")
	time.Sleep(300 * time.Millisecond)
	if h.count("turn/start") != 0 || requestState(h.snap(), "m1") != "queued" {
		t.Fatal("paused client started a turn")
	}
	if r, _ := control.Call(h.root, control.Request{Cmd: "resume"}); !r.OK {
		t.Fatal("resume failed")
	}
	h.waitSnap("reply after resume", func(s codexSnap) bool { return requestState(s, "m1") == "completed" })
	h.stop()
	waitGone(t, pidOf(setup))
}

func TestCodexOwnerTurnFailuresStayUncertainWithoutRetry(t *testing.T) {
	old := codexTurnTimeout
	codexTurnTimeout = 2 * time.Second
	t.Cleanup(func() { codexTurnTimeout = old })
	for _, mode := range []string{"turn-failed", "no-reply", "foreign-turn", "dup-request", "exit-in-turn", "second-fails"} {
		t.Run(mode, func(t *testing.T) {
			h := newCodexHarness(t, mode, AdapterLauncher{AllowModels: true})
			if r := h.create("codex-test"); !r.OK {
				t.Fatalf("create failed: %+v", r)
			}
			setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
			failing := "m1"
			if mode == "second-fails" {
				h.send("m1", "one")
				h.waitSnap("first reply", func(s codexSnap) bool { return requestState(s, "m1") == "completed" })
				failing = "m2"
			}
			h.send(failing, "x")
			h.send("later", "y")
			s := h.waitSnap("uncertain turn", func(s codexSnap) bool {
				return requestState(s, failing) == "uncertain" && s.CodexSetups[0]["state"] == "uncertain" && s.CodexSetups[0]["exit_observed"] == float64(1)
			})
			time.Sleep(200 * time.Millisecond)
			s = h.snap()
			if requestState(s, "later") != "queued" {
				t.Fatalf("message after an uncertain turn was taken: %+v", s.Requests)
			}
			want := 1
			if mode == "second-fails" {
				want = 2
			}
			if h.count("turn/start") != want {
				t.Fatalf("turn/start sent %d times, want %d", h.count("turn/start"), want)
			}
			if len(codexReservations(t)) != 1 {
				t.Fatal("reservation removed")
			}
			if r, _ := control.Call(h.root, control.Request{Cmd: "resolve", Action: "retry", Alias: "codex-test", Confirmation: "actions-may-repeat"}); r.OK {
				t.Fatal("resolve retried a Codex-owned session")
			}
			h.stop()
			waitGone(t, pidOf(setup))
		})
	}
}

func TestCodexOwnedSessionIsNotLaunchedByGenericLoop(t *testing.T) {
	h := newCodexHarness(t, "ok", AdapterLauncher{AllowModels: true, OnlySession: "codex-test"})
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	if r, _ := control.Call(h.root, control.Request{Cmd: "register-session", Provider: "codex", Environment: h.profile, ConversationID: "other-thread", Alias: "codex-test", CLI: h.self}); r.OK {
		t.Fatal("alias rebound")
	}
	h.send("m1", "one")
	h.waitSnap("reply through the owner", func(s codexSnap) bool { return requestState(s, "m1") == "completed" })
	if h.count("turn/start") != 1 || h.count("args:app-server --listen stdio://") != 1 {
		t.Fatalf("unexpected launches: %v", h.log())
	}
	h.stop()
	waitGone(t, pidOf(setup))
}

func fakeEpoch(boot string) func() (platform.RecoveryEpoch, error) {
	return func() (platform.RecoveryEpoch, error) {
		return platform.RecoveryEpoch{Version: 1, Host: strings.Repeat("a", 64), Boot: boot}, nil
	}
}

const bootOne = "11111111-1111-4111-8111-111111111111"
const bootTwo = "22222222-2222-4222-8222-222222222222"

// uncertainCodexOwner leaves a created owner uncertain: the client restarts
// and cannot reach the old app-server.
func uncertainCodexOwner(t *testing.T) *codexHarness {
	t.Helper()
	old := recoveryEpoch
	recoveryEpoch = fakeEpoch(bootOne)
	t.Cleanup(func() { recoveryEpoch = old })
	h := newCodexHarness(t, "ok", AdapterLauncher{AllowModels: true})
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	h.send("m1", "one")
	h.waitSnap("reply", func(s codexSnap) bool { return requestState(s, "m1") == "completed" })
	h.stop()
	waitGone(t, pidOf(setup))
	h.start(AdapterLauncher{})
	h.waitSetup(func(s map[string]any) bool { return s["state"] == "uncertain" })
	h.stop()
	return h
}

func TestRecoverCodexRequiresProvenReboot(t *testing.T) {
	h := uncertainCodexOwner(t)
	if _, err := RecoverCodex(h.root, "codex-test", "wrong"); !errors.Is(err, core.ErrBadConfirmation) {
		t.Fatalf("bad confirmation: %v", err)
	}
	// Same boot: the old process is gone, but that is not proof.
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("same boot accepted: %v", err)
	}
	if len(codexReservations(t)) != 1 {
		t.Fatal("reservation removed without proof")
	}
	// While the client runs, recovery is refused.
	h.start(AdapterLauncher{})
	recoveryEpoch = fakeEpoch(bootTwo)
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation); err == nil {
		t.Fatal("recovery ran next to a live client")
	}
	h.stop()
	if _, err := RecoverCodex(h.root, "unknown", core.StreamRecoveryConfirmation); err == nil {
		t.Fatal("unknown alias accepted")
	}
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation); err != nil {
		t.Fatal(err)
	}
	if len(codexReservations(t)) != 0 {
		t.Fatal("reservation kept after recovery")
	}
	h.start(AdapterLauncher{AllowModels: true})
	s := h.snap()
	if s.CodexSetups[0]["state"] != "retired" || s.Sessions[0]["decision"] != "closed_without_retry" {
		t.Fatalf("after recovery: %+v %+v", s.CodexSetups, s.Sessions)
	}
	if replyTo(s, "m1") != "ACK:one" {
		t.Fatal("history lost")
	}
	if r, _ := control.Call(h.root, control.Request{Cmd: "enqueue", Alias: "codex-test", MessageID: "m2", Sender: "user", Text: "x"}); r.OK {
		t.Fatal("closed session accepted a message")
	}
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation); err == nil {
		t.Fatal("second recovery accepted")
	}
	h.stop()
	if h.count("turn/start") != 1 {
		t.Fatal("a turn was resent")
	}
}

func TestRecoverCodexPendingIsResumable(t *testing.T) {
	h := uncertainCodexOwner(t)
	recoveryEpoch = fakeEpoch(bootTwo)
	beforeRenameHook = func() error { return errors.New("disk full") }
	_, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation)
	beforeRenameHook = nil
	if err == nil {
		t.Fatal("failed registry write reported success")
	}
	if len(codexReservations(t)) != 1 {
		t.Fatal("reservation removed by a failed write")
	}
	// A half-done recovery blocks startup until it is finished.
	if err := Run(context.Background(), h.root, AdapterLauncher{}); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("startup with pending recovery: %v", err)
	}
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation); err != nil {
		t.Fatal(err)
	}
	if len(codexReservations(t)) != 0 {
		t.Fatal("reservation kept")
	}
	h.start(AdapterLauncher{})
	if s := h.snap(); s.CodexSetups[0]["state"] != "retired" {
		t.Fatalf("state %+v", s.CodexSetups)
	}
	h.stop()
}

func TestClaudeAndCodexExchangeUntilAutoReplyLimit(t *testing.T) {
	t.Setenv("MESSENGER_STREAM_FIXTURE", "normal")
	launcher := AdapterLauncher{AllowModels: true, ClaudeStream: true}
	h := newCodexHarnessWith(t, "ok", launcher, func(root string) {
		st := mustStore(t, root)
		defer st.Close()
		env := mkdir(t, filepath.Join(root, "claude-profile"))
		if _, err := st.Register("claude", env, "test-stream", "worker"); err != nil {
			t.Fatal(err)
		}
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		if err := st.ConfigureLaunch("worker", exe, root); err != nil {
			t.Fatal(err)
		}
	})
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	// Claude ("worker") asks Codex; every reply goes back to the other agent.
	if r, err := control.Call(h.root, control.Request{Cmd: "enqueue", Alias: "codex-test", MessageID: "m1", Sender: "worker", Text: "hi"}); err != nil || !r.OK {
		t.Fatalf("enqueue: %+v %v", r, err)
	}
	s := h.waitSnap("exchange to the limit", func(s codexSnap) bool {
		limited := false
		for _, e := range s.Events {
			limited = limited || e["kind"] == "auto_reply_limit"
		}
		return limited && len(s.Replies) == 4
	})
	time.Sleep(300 * time.Millisecond)
	s = h.snap()
	if len(s.Requests) != 1+core.MaxAutoReplyHops || len(s.Replies) != 1+core.MaxAutoReplyHops {
		t.Fatalf("requests %d replies %d", len(s.Requests), len(s.Replies))
	}
	for _, r := range s.Requests {
		if r["state"] != "completed" {
			t.Fatalf("request %+v", r)
		}
	}
	want := map[string]string{"codex-test": "worker", "worker": "codex-test"}
	texts := map[string]bool{}
	for _, r := range s.Replies {
		if want[r["sender"].(string)] != r["recipient"] {
			t.Fatalf("reply route %+v", r)
		}
		texts[r["text"].(string)] = true
	}
	for _, text := range []string{"ACK:hi", "ACK:ACK:hi", "ACK:ACK:ACK:hi", "ACK:ACK:ACK:ACK:hi"} {
		if !texts[text] {
			t.Fatalf("missing reply %q in %+v", text, s.Replies)
		}
	}
	if h.count("turn/start") != 2 {
		t.Fatalf("Codex turns %d", h.count("turn/start"))
	}
	// Both owners stay alive and idle: one Claude stream, one Codex app-server.
	if len(s.StreamRuns) != 1 || s.StreamRuns[0]["state"] != "idle" || s.CodexSetups[0]["state"] != "created" {
		t.Fatalf("owners %+v %+v", s.StreamRuns, s.CodexSetups)
	}
	h.stop()
	waitGone(t, pidOf(setup))
	st := mustStore(t, h.root)
	defer st.Close()
	snap, _ := st.Snapshot()
	for _, run := range snap.StreamRuns {
		if obs, err := st.Observation(text(run["id"])); err == nil {
			if owns, _ := platform.OwnsObservation(obs); owns {
				p, _ := os.FindProcess(obs.PID)
				_ = p.Kill()
				waitGone(t, obs.PID)
			}
		}
	}
}

func TestCodexOwnerRechecksConfigBeforeEachTurn(t *testing.T) {
	h := newCodexHarness(t, "config-change", AdapterLauncher{AllowModels: true})
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	h.send("m1", "one")
	s := h.waitSnap("config change detected", func(s codexSnap) bool {
		return s.CodexSetups[0]["state"] == "uncertain" && s.CodexSetups[0]["exit_observed"] == float64(1)
	})
	if requestState(s, "m1") != "queued" {
		t.Fatalf("unsent message not returned to the queue: %+v", s.Requests)
	}
	if h.count("turn/start") != 0 || h.count("config/read") != 2 {
		t.Fatalf("log %v", h.log())
	}
	h.stop()
	waitGone(t, pidOf(setup))
}

func TestWindowCodexSetupGrantsOnlyThisSession(t *testing.T) {
	h := newCodexHarness(t, "ok", AdapterLauncher{ClaudeStream: true})
	// The window starts its client paused; the setup action opens the queue.
	if r, _ := control.Call(h.root, control.Request{Cmd: "pause"}); !r.OK {
		t.Fatal("pause failed")
	}
	setup := func(alias, env, confirm string) control.Response {
		r, err := control.Call(h.root, control.Request{Cmd: "setup-codex", Alias: alias, CLI: h.self, Environment: env, Confirmation: confirm})
		if err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := setup("codex-test", h.profile, ""); r.OK {
		t.Fatal("setup without confirmation")
	}
	// A refused setup that launched nothing withdraws the grant.
	defaultProfile := mkdir(t, filepath.Join(os.Getenv("HOME"), ".codex"))
	if r := setup("codex-test", defaultProfile, "allow-model-calls"); r.OK {
		t.Fatal("default profile accepted")
	}
	if r := setup("codex-test", h.profile, "allow-model-calls"); !r.OK {
		t.Fatalf("setup failed: %+v", r)
	}
	s := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	// A second session (for example the other side of a Claude ↔ Codex pair)
	// can be added in the same run.
	if r := setup("codex-other", h.profile, "allow-model-calls"); !r.OK {
		t.Fatalf("second session refused: %+v", r)
	}
	h.waitSnap("second owner", func(s codexSnap) bool {
		return len(s.CodexSetups) == 2 && s.CodexSetups[1]["state"] == "created"
	})
	h.send("m1", "one")
	h.waitSnap("reply", func(s codexSnap) bool { return requestState(s, "m1") == "completed" })
	if r, err := control.Call(h.root, control.Request{Cmd: "enqueue", Alias: "codex-other", MessageID: "m2", Sender: "user", Text: "two"}); err != nil || !r.OK {
		t.Fatalf("enqueue: %+v %v", r, err)
	}
	last := h.waitSnap("reply from the second session", func(s codexSnap) bool { return requestState(s, "m2") == "completed" })
	status, _ := control.Call(h.root, control.Request{Cmd: "status"})
	if status.Status == nil || len(status.Status.LaunchSessions) != 2 {
		t.Fatalf("status %+v", status.Status)
	}
	h.stop()
	waitGone(t, pidOf(s))
	waitGone(t, pidOf(last.CodexSetups[1]))
}

func TestWindowGrantsAreBoundedAndRevocable(t *testing.T) {
	s := &Supervisor{launcher: AdapterLauncher{ClaudeStream: true}}
	for i, alias := range []string{"a", "b", "c", "d"} {
		launcher := s.currentLauncher().(AdapterLauncher)
		granted, err := s.grantWindowAliasLocked(launcher, alias)
		if err != nil || !granted {
			t.Fatalf("grant %d: %v %v", i, granted, err)
		}
	}
	if _, err := s.grantWindowAliasLocked(s.currentLauncher().(AdapterLauncher), "e"); err == nil {
		t.Fatal("fifth session granted")
	}
	launcher := s.currentLauncher().(AdapterLauncher)
	if !launcher.Allows("c") || launcher.Allows("e") || len(launcher.SessionAliases()) != 4 {
		t.Fatalf("scope %+v", launcher)
	}
	s.revokeWindowAlias("a")
	s.revokeWindowAlias("c")
	launcher = s.currentLauncher().(AdapterLauncher)
	if launcher.Allows("a") || launcher.Allows("c") || !launcher.Allows("b") || !launcher.Allows("d") {
		t.Fatalf("after revoke %+v", launcher)
	}
	s.revokeWindowAlias("b")
	s.revokeWindowAlias("d")
	if launcher = s.currentLauncher().(AdapterLauncher); launcher.AllowModels {
		t.Fatal("permission kept after every grant was withdrawn")
	}
}

// Claude, as coordinator, writes to Codex with the send tool; Codex's answer
// comes back to Claude, and the chain stops at the automatic-step limit.
func TestClaudeAsksCodexWithSendTool(t *testing.T) {
	t.Setenv("MESSENGER_STREAM_FIXTURE", "send")
	launcher := AdapterLauncher{AllowModels: true, ClaudeStream: true}
	h := newCodexHarnessWith(t, "ok", launcher, func(root string) {
		st := mustStore(t, root)
		defer st.Close()
		env := mkdir(t, filepath.Join(root, "claude-profile"))
		if _, err := st.Register("claude", env, "test-stream", "worker"); err != nil {
			t.Fatal(err)
		}
		exe, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		if err := st.ConfigureLaunch("worker", exe, root); err != nil {
			t.Fatal(err)
		}
	})
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	if r, err := control.Call(h.root, control.Request{Cmd: "enqueue", Alias: "worker", MessageID: "m1", Sender: "user", Text: "ask:hi"}); err != nil || !r.OK {
		t.Fatalf("enqueue: %+v %v", r, err)
	}
	s := h.waitSnap("exchange to the limit", func(s codexSnap) bool {
		for _, e := range s.Events {
			if e["kind"] == "auto_reply_limit" {
				return true
			}
		}
		return false
	})
	time.Sleep(300 * time.Millisecond)
	s = h.snap()
	var sent map[string]any
	for _, r := range s.Requests {
		if id, _ := r["id"].(string); strings.HasPrefix(id, "send-") {
			sent = r
		}
		if r["state"] != "completed" {
			t.Fatalf("request %+v", r)
		}
	}
	if sent == nil || sent["sender"] != "worker" || sent["in_reply_to"] != "m1" || sent["hops"] != float64(1) || sent["text"] != "question from worker" {
		t.Fatalf("send-initiated message %+v", sent)
	}
	if len(s.Requests) != 4 || len(s.Replies) != 4 || replyTo(s, "m1") != "ACK:ask:hi" {
		t.Fatalf("requests %d replies %d: %+v", len(s.Requests), len(s.Replies), s.Replies)
	}
	if h.count("turn/start") != 2 {
		t.Fatalf("Codex turns %d", h.count("turn/start"))
	}
	h.stop()
	waitGone(t, pidOf(setup))
	st := mustStore(t, h.root)
	defer st.Close()
	snap, _ := st.Snapshot()
	for _, run := range snap.StreamRuns {
		if obs, err := st.Observation(text(run["id"])); err == nil {
			if owns, _ := platform.OwnsObservation(obs); owns {
				p, _ := os.FindProcess(obs.PID)
				_ = p.Kill()
				waitGone(t, obs.PID)
			}
		}
	}
}
