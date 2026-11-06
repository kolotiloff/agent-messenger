//go:build darwin || linux

package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// setMode changes the fake app-server mode for the next process.
func (h *codexHarness) setMode(mode string) {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(h.profile, "fixture-mode"), []byte(mode), 0o600); err != nil {
		h.t.Fatal(err)
	}
}

func (h *codexHarness) resumeSession() control.Response {
	h.t.Helper()
	r, err := control.Call(h.root, control.Request{Cmd: "resume-codex-session", Alias: "codex-test"})
	if err != nil {
		h.t.Fatal(err)
	}
	return r
}

func paused(t *testing.T, h *codexHarness) bool {
	t.Helper()
	response, err := control.Call(h.root, control.Request{Cmd: "status"})
	if err != nil || !response.OK || response.Status == nil {
		t.Fatalf("status %v %+v", err, response)
	}
	return response.Status.Paused
}

func eventCount(s codexSnap, kind string) int {
	n := 0
	for _, e := range s.Events {
		if e["kind"] == kind {
			n++
		}
	}
	return n
}

// resumableCodexOwner leaves the owner of uncertainCodexOwner resumable: the
// host has booted again and recover-codex --resume removed the reservation.
func resumableCodexOwner(t *testing.T) *codexHarness {
	t.Helper()
	h := uncertainCodexOwner(t)
	recoveryEpoch = fakeEpoch(bootTwo)
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation, core.CodexRecoveryResume); err != nil {
		t.Fatal(err)
	}
	if len(codexReservations(t)) != 0 {
		t.Fatal("reservation kept after recovery")
	}
	return h
}

func TestResumeCodexSessionContinuesSameThreadWithNewOwner(t *testing.T) {
	h := resumableCodexOwner(t)
	h.start(AdapterLauncher{AllowModels: true})
	s := h.snap()
	if s.CodexSetups[0]["state"] != "resumable" || s.Sessions[0]["decision"] != nil {
		t.Fatalf("after recovery: %+v %+v", s.CodexSetups, s.Sessions)
	}
	if !paused(t, h) {
		t.Fatal("recovery did not pause the queue")
	}
	oldPID := pidOf(s.CodexSetups[0])
	// The open session still accepts messages; they wait for an owner.
	h.send("m2", "two")
	time.Sleep(150 * time.Millisecond)
	if requestState(h.snap(), "m2") != "queued" {
		t.Fatal("message taken without an owner")
	}
	if r := h.resumeSession(); !r.OK {
		t.Fatalf("resume failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	if setup["thread_id"] != fakeThreadID || pidOf(setup) == 0 || pidOf(setup) == oldPID {
		t.Fatalf("resumed setup %+v", setup)
	}
	runs := codexReservations(t)
	if len(runs) != 1 || runs[0].ConversationID != fakeThreadID || runs[0].CodexSetupID != setup["id"] || runs[0].PID != pidOf(setup) {
		t.Fatalf("reservation %+v", runs)
	}
	// Resuming starts no turn; the paused queue is not opened by it.
	time.Sleep(150 * time.Millisecond)
	if h.count("turn/start") != 1 || requestState(h.snap(), "m2") != "queued" {
		t.Fatal("resume started a turn")
	}
	if h.count("thread/resume") != 1 || h.count("thread/start") != 1 {
		t.Fatalf("thread requests: %v", h.log())
	}
	if r, _ := control.Call(h.root, control.Request{Cmd: "resume"}); !r.OK {
		t.Fatal("resume queue failed")
	}
	s = h.waitSnap("reply after resume", func(s codexSnap) bool { return requestState(s, "m2") == "completed" })
	if replyTo(s, "m1") != "ACK:one" || replyTo(s, "m2") != "ACK:two" {
		t.Fatalf("replies %+v", s.Replies)
	}
	if eventCount(s, "codex_thread_resumed") != 1 {
		t.Fatal("resume not recorded")
	}
	h.send("m3", "three")
	h.waitSnap("second turn of the new owner", func(s codexSnap) bool { return requestState(s, "m3") == "completed" })
	if h.count("turn/start") != 3 {
		t.Fatalf("turn/start sent %d times", h.count("turn/start"))
	}
	// A second resume is refused while the owner lives.
	if r := h.resumeSession(); r.OK {
		t.Fatal("second owner started")
	}
	h.stop()
	waitGone(t, pidOf(setup))
}

func TestRecoverCodexResumeKeepsInFlightMessageUncertain(t *testing.T) {
	old := recoveryEpoch
	recoveryEpoch = fakeEpoch(bootOne)
	t.Cleanup(func() { recoveryEpoch = old })
	h := newCodexHarness(t, "second-fails", AdapterLauncher{AllowModels: true})
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	h.send("m1", "one")
	h.waitSnap("first reply", func(s codexSnap) bool { return requestState(s, "m1") == "completed" })
	h.send("m2", "lost")
	h.waitSnap("uncertain turn", func(s codexSnap) bool {
		return requestState(s, "m2") == "uncertain" && s.CodexSetups[0]["exit_observed"] == float64(1)
	})
	h.stop()
	waitGone(t, pidOf(setup))
	recoveryEpoch = fakeEpoch(bootTwo)
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation, core.CodexRecoveryResume); err != nil {
		t.Fatal(err)
	}
	h.setMode("ok")
	h.start(AdapterLauncher{AllowModels: true})
	s := h.snap()
	if s.Sessions[0]["active_token"] != nil || requestState(s, "m2") != "uncertain" {
		t.Fatalf("after recovery: %+v %+v", s.Sessions, s.Requests)
	}
	if r := h.resumeSession(); !r.OK {
		t.Fatalf("resume failed: %+v", r)
	}
	setup = h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	if r, _ := control.Call(h.root, control.Request{Cmd: "resume"}); !r.OK {
		t.Fatal("resume queue failed")
	}
	h.send("m3", "three")
	s = h.waitSnap("new message answered", func(s codexSnap) bool { return requestState(s, "m3") == "completed" })
	// The fixture stored a reply before reporting the turn failed; the message
	// stays uncertain and is not answered again.
	replies := 0
	for _, r := range s.Replies {
		if r["reply_to"] == "m2" {
			replies++
		}
	}
	if requestState(s, "m2") != "uncertain" || replies != 1 {
		t.Fatalf("uncertain message changed: %+v", s.Requests)
	}
	if h.count("turn/start") != 3 {
		t.Fatalf("turn/start sent %d times; the uncertain message must not be resent", h.count("turn/start"))
	}
	h.stop()
	waitGone(t, pidOf(setup))
}

func TestRecoverCodexResumeNeedsVerifiedTurn(t *testing.T) {
	old := recoveryEpoch
	recoveryEpoch = fakeEpoch(bootOne)
	t.Cleanup(func() { recoveryEpoch = old })
	h := newCodexHarness(t, "ok", AdapterLauncher{AllowModels: true})
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	h.stop()
	waitGone(t, pidOf(setup))
	h.start(AdapterLauncher{})
	h.waitSetup(func(s map[string]any) bool { return s["state"] == "uncertain" })
	h.stop()
	recoveryEpoch = fakeEpoch(bootTwo)
	// Codex saves no rollout for an empty thread, so there is nothing to resume.
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation, core.CodexRecoveryResume); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("empty thread recovered for resume: %v", err)
	}
	if len(codexReservations(t)) != 1 {
		t.Fatal("refused recovery removed the reservation")
	}
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation, "other"); !errors.Is(err, core.ErrRejected) {
		t.Fatalf("unknown mode: %v", err)
	}
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation, core.CodexRecoveryClose); err != nil {
		t.Fatal(err)
	}
	h.start(AdapterLauncher{AllowModels: true})
	if r := h.resumeSession(); r.OK {
		t.Fatal("closed session resumed")
	}
	if h.count("args:app-server --listen stdio://") != 1 {
		t.Fatal("a process was launched")
	}
	h.stop()
}

func TestRecoverCodexPendingModeCannotSwitch(t *testing.T) {
	h := uncertainCodexOwner(t)
	recoveryEpoch = fakeEpoch(bootTwo)
	beforeRenameHook = func() error { return errors.New("disk full") }
	_, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation, core.CodexRecoveryResume)
	beforeRenameHook = nil
	if err == nil {
		t.Fatal("failed registry write reported success")
	}
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation, core.CodexRecoveryClose); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("pending resume finished as close: %v", err)
	}
	if err := Run(context.Background(), h.root, AdapterLauncher{}); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatalf("startup with pending recovery: %v", err)
	}
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation, core.CodexRecoveryResume); err != nil {
		t.Fatal(err)
	}
	h.start(AdapterLauncher{})
	if s := h.snap(); s.CodexSetups[0]["state"] != "resumable" || s.Sessions[0]["decision"] != nil {
		t.Fatalf("after recovery: %+v %+v", s.CodexSetups, s.Sessions)
	}
	h.stop()
}

func TestResumeCodexSessionFailuresStayUncertainWithoutRetry(t *testing.T) {
	old := codexSetupTimeout
	codexSetupTimeout = 500 * time.Millisecond
	t.Cleanup(func() { codexSetupTimeout = old })
	for _, mode := range []string{"resume-missing", "resume-other", "resume-active", "resume-silent", "mcp"} {
		t.Run(mode, func(t *testing.T) {
			h := resumableCodexOwner(t)
			h.setMode(mode)
			h.start(AdapterLauncher{AllowModels: true})
			h.send("m2", "two")
			if r, _ := control.Call(h.root, control.Request{Cmd: "resume"}); !r.OK {
				t.Fatal("resume queue failed")
			}
			if r := h.resumeSession(); r.OK {
				t.Fatalf("resume accepted: %+v", r)
			}
			setup := h.waitSetup(func(s map[string]any) bool { return s["state"] == "uncertain" && s["exit_observed"] == float64(1) })
			time.Sleep(150 * time.Millisecond)
			s := h.snap()
			if requestState(s, "m2") != "queued" || h.count("turn/start") != 1 {
				t.Fatalf("a turn ran after a failed resume: %+v", s.Requests)
			}
			want := 1
			if mode == "mcp" {
				want = 0 // config rejected before the resume intent
			}
			if h.count("thread/resume") != want || h.count("thread/start") != 1 {
				t.Fatalf("thread requests: %v", h.log())
			}
			if runs := codexReservations(t); len(runs) != 1 || runs[0].PID != pidOf(setup) {
				t.Fatalf("reservation %+v", runs)
			}
			// Nothing is retried, and a new thread never replaces the old one.
			if r := h.resumeSession(); r.OK {
				t.Fatal("resume retried")
			}
			if h.count("thread/resume") != want || h.count("thread/start") != 1 {
				t.Fatalf("thread requests after refusal: %v", h.log())
			}
			h.stop()
			waitGone(t, pidOf(setup))
		})
	}
}

// refusingCodexLauncher reports a definite exec refusal.
type refusingCodexLauncher struct{ AdapterLauncher }

func (refusingCodexLauncher) StartCodexServer(context.Context, CodexServerSpec) (CodexProcess, error) {
	return CodexProcess{}, errNotStarted
}

func TestResumeCodexSessionRefusals(t *testing.T) {
	h := resumableCodexOwner(t)
	// Without model permission nothing is launched.
	h.start(AdapterLauncher{})
	if r := h.resumeSession(); r.OK {
		t.Fatal("resume without model permission")
	}
	h.stop()
	h.start(AdapterLauncher{AllowModels: true, OnlySession: "other"})
	if r := h.resumeSession(); r.OK {
		t.Fatal("resume outside the allowed session")
	}
	h.stop()
	if h.count("args:app-server --listen stdio://") != 1 {
		t.Fatal("a process was launched")
	}
	// A definite exec refusal leaves the session resumable.
	h.start(refusingCodexLauncher{AdapterLauncher{AllowModels: true}})
	if r := h.resumeSession(); r.OK {
		t.Fatal("refused exec reported success")
	}
	if s := h.snap(); s.CodexSetups[0]["state"] != "resumable" {
		t.Fatalf("state %+v", s.CodexSetups)
	}
	h.stop()
	// A registry entry for the thread blocks the resume.
	if err := rememberRun(recordedRun{ID: "stray", Root: h.root, Provider: "codex", ConversationID: fakeThreadID, PID: 1, PGID: 1}); err != nil {
		t.Fatal(err)
	}
	h.start(AdapterLauncher{AllowModels: true})
	if r := h.resumeSession(); r.OK {
		t.Fatal("resume next to a reservation for the same thread")
	}
	if s := h.snap(); s.CodexSetups[0]["state"] != "resumable" {
		t.Fatalf("state %+v", s.CodexSetups)
	}
	h.stop()
	if h.count("args:app-server --listen stdio://") != 1 {
		t.Fatal("a process was launched")
	}
}

// fakeResponsesModel imitates the model behind Codex's Responses API. It is
// not a model: for a user message it calls messenger_bound.read_current, for
// that output it calls reply with "ACK:<text>" and the receipt, and then it
// ends the turn. It records which Messenger tools each request offered.
type fakeResponsesModel struct {
	mu       sync.Mutex
	requests int
	prompts  int
	withTool int
	calls    int
}

func (m *fakeResponsesModel) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Input []json.RawMessage `json:"input"`
		Tools []struct {
			Type, Name string
			Tools      []struct{ Name string } `json:"tools"`
		} `json:"tools"`
	}
	if json.NewDecoder(r.Body).Decode(&req) != nil || len(req.Input) == 0 {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	var last struct {
		Type   string          `json:"type"`
		CallID string          `json:"call_id"`
		Output json.RawMessage `json:"output"`
	}
	_ = json.Unmarshal(req.Input[len(req.Input)-1], &last)
	m.mu.Lock()
	m.requests++
	m.calls++
	call := m.calls
	for _, tool := range req.Tools {
		if tool.Type == "namespace" && tool.Name == "messenger_bound" && len(tool.Tools) == 2 && tool.Tools[0].Name == "read_current" && tool.Tools[1].Name == "reply" {
			m.withTool++
		}
	}
	var item map[string]any
	switch {
	case last.Type == "message":
		m.prompts++
		item = map[string]any{"type": "function_call", "id": fmt.Sprintf("fc_%d", call), "call_id": fmt.Sprintf("read_%d", call), "namespace": "messenger_bound", "name": "read_current", "arguments": "{}"}
	case last.Type == "function_call_output" && len(last.CallID) > 5 && last.CallID[:5] == "read_":
		var text string
		_ = json.Unmarshal(last.Output, &text)
		var message struct{ Text, Receipt string }
		_ = json.Unmarshal([]byte(text), &message)
		args, _ := json.Marshal(map[string]string{"receipt": message.Receipt, "text": "ACK:" + message.Text})
		item = map[string]any{"type": "function_call", "id": fmt.Sprintf("fc_%d", call), "call_id": fmt.Sprintf("reply_%d", call), "namespace": "messenger_bound", "name": "reply", "arguments": string(args)}
	default:
		item = map[string]any{"type": "message", "role": "assistant", "id": fmt.Sprintf("msg_%d", call), "content": []any{map[string]any{"type": "output_text", "text": "done"}}}
	}
	m.mu.Unlock()
	w.Header().Set("Content-Type", "text/event-stream")
	event := func(kind string, data map[string]any) {
		data["type"] = kind
		raw, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", kind, raw)
	}
	event("response.created", map[string]any{"response": map[string]any{"id": "resp"}})
	event("response.output_item.done", map[string]any{"item": item})
	event("response.completed", map[string]any{"response": map[string]any{"id": "resp", "usage": map[string]any{"input_tokens": 1, "output_tokens": 1, "total_tokens": 2}}})
}

func (m *fakeResponsesModel) counts() (requests, prompts, withTool int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requests, m.prompts, m.withTool
}

// Opt-in check of the whole create, turn, recover and resume path against an
// installed Codex CLI. The managed profile points Codex at a local fake model,
// so no real model is called. It checks that the resumed thread still offers
// the Messenger tools, which thread/resume cannot declare.
func TestInstalledCodexResumeWithFakeModel(t *testing.T) {
	cli := os.Getenv("MESSENGER_CODEX_PROBE")
	if cli == "" {
		t.Skip("set MESSENGER_CODEX_PROBE to an absolute Codex executable for a check with a local fake model")
	}
	model := &fakeResponsesModel{}
	server := httptest.NewServer(model)
	defer server.Close()
	old := recoveryEpoch
	recoveryEpoch = fakeEpoch(bootOne)
	t.Cleanup(func() { recoveryEpoch = old })
	h := newCodexHarness(t, "", AdapterLauncher{AllowModels: true})
	h.self = cli
	config := fmt.Sprintf("model = \"fake-model\"\nmodel_provider = \"fake\"\n[model_providers.fake]\nname = \"fake\"\nbase_url = %q\nwire_api = \"responses\"\n", server.URL+"/v1")
	if err := os.WriteFile(filepath.Join(h.profile, "config.toml"), []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if r := h.create("codex-test"); !r.OK {
		t.Fatalf("create failed: %+v", r)
	}
	first := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	thread, _ := first["thread_id"].(string)
	h.send("m1", "one")
	h.waitSnap("first reply", func(s codexSnap) bool { return replyTo(s, "m1") == "ACK:one" && requestState(s, "m1") == "completed" })
	h.stop()
	waitGone(t, pidOf(first))
	h.start(AdapterLauncher{})
	h.waitSetup(func(s map[string]any) bool { return s["state"] == "uncertain" })
	h.stop()
	recoveryEpoch = fakeEpoch(bootTwo)
	if _, err := RecoverCodex(h.root, "codex-test", core.StreamRecoveryConfirmation, core.CodexRecoveryResume); err != nil {
		t.Fatal(err)
	}
	h.start(AdapterLauncher{AllowModels: true})
	if r := h.resumeSession(); !r.OK {
		t.Fatalf("resume failed: %+v", r)
	}
	second := h.waitSetup(func(s map[string]any) bool { return s["state"] == "created" })
	if second["thread_id"] != thread || pidOf(second) == pidOf(first) {
		t.Fatalf("resumed setup %+v", second)
	}
	before, _, _ := model.counts()
	if r, _ := control.Call(h.root, control.Request{Cmd: "resume"}); !r.OK {
		t.Fatal("resume queue failed")
	}
	h.send("m2", "two")
	s := h.waitSnap("reply from the resumed thread", func(s codexSnap) bool { return replyTo(s, "m2") == "ACK:two" && requestState(s, "m2") == "completed" })
	if replyTo(s, "m1") != "ACK:one" {
		t.Fatal("history lost")
	}
	requests, prompts, withTool := model.counts()
	if prompts != 2 || withTool != requests || requests-before != 3 {
		t.Fatalf("model requests %d (after resume %d), prompts %d, with Messenger tools %d", requests, requests-before, prompts, withTool)
	}
	h.stop()
	waitGone(t, pidOf(second))
}
