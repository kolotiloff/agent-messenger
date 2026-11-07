package supervisor

import (
	"context"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

// A refused window grant must never open model calls, not even for a moment:
// the loop claims every 30 ms, so a grant that is withdrawn afterwards could
// start the refused session's queued message.
func TestRefusedWindowGrantsNeverStartAModel(t *testing.T) {
	root, st := seedStreamClient(t, "normal")
	if _, err := st.Enqueue("waiting", "worker", "user", "waiting"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx, root, AdapterLauncher{ClaudeStream: true}) }()
	waitSocket(t, root)
	for i := 0; i < 150; i++ {
		// "worker" is an attached session, not a new test session.
		if r, err := control.Call(root, control.Request{Cmd: "authorize-setup", Alias: "worker", Confirmation: "allow-model-calls"}); err != nil || r.OK {
			t.Fatal("refused grant accepted", r, err)
		}
		// The name of an existing session cannot become a new Codex session.
		if r, err := control.Call(root, control.Request{Cmd: "setup-codex", Alias: "worker", Environment: root, CLI: "/nonexistent/codex", Confirmation: "allow-model-calls"}); err != nil || r.OK {
			t.Fatal("codex setup over an existing name accepted", r, err)
		}
	}
	snap, err := st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Attempts) != 0 {
		t.Fatalf("a refused grant started a model: %v", snap.Attempts)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
