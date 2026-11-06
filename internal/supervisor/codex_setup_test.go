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
)

type codexHarness struct {
	t                   *testing.T
	root, profile, self string
	errc                chan error
	cancel              context.CancelFunc
}

func newCodexHarness(t *testing.T, mode string, launcher Launcher) *codexHarness {
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
		if _, err := os.Stat(control.SocketPath(h.root)); err == nil {
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
	// The thread cannot receive messages or be registered a second time yet.
	if r, _ := control.Call(h.root, control.Request{Cmd: "enqueue", Alias: "codex-test", MessageID: "m1", Sender: "user", Text: "hi"}); r.OK {
		t.Fatal("message accepted without a delivery path")
	}
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
