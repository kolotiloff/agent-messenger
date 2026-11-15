package supervisor

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// T-191: the Codex limit check stores both windows as a checked limit; a
// failed read keeps the last one; a paused agent is not checked.
func TestCodexLimitCheck(t *testing.T) {
	root := t.TempDir()
	st := mustStore(t, root)
	t.Cleanup(func() { st.Close() })
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetComputerAgent("codex", exe, "", "worker"); err != nil {
		t.Fatal(err)
	}
	if err := st.SetComputerAgent("claude", exe, "", "worker"); err != nil {
		t.Fatal(err)
	}
	s := &Supervisor{store: st, root: root}
	calls := 0
	reset := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC)
	read := func(ctx context.Context, cli, environment, dir string) (adapters.LimitState, error) {
		calls++
		if cli != exe || dir != root {
			t.Errorf("read %q in %q", cli, dir)
		}
		return adapters.LimitState{State: adapters.LimitWarning, UsedPercent: 91, HasPercent: true, Window: "7d", ResetAt: reset,
			Windows: []adapters.LimitWindow{{Window: "5h", UsedPercent: 30, ResetAt: reset}, {Window: "7d", UsedPercent: 91, ResetAt: reset}}}, nil
	}
	if why := s.checkLimitsOnce(context.Background(), read); why != "" || calls != 1 {
		t.Fatalf("check: %q, %d calls", why, calls)
	}
	var l *core.AgentLimit
	agents, _, _ := st.Presence()
	for _, a := range agents {
		if a.Agent == "codex" {
			l = a.Limit
		}
	}
	if l == nil || l.Source != core.LimitSourceCheck || l.State != "warning" || *l.UsedPercent != 91 || len(l.Windows) != 2 || l.Windows[0].UsedPercent != 30 {
		t.Fatalf("stored %+v", l)
	}
	p, err := s.presence()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range p.Agents {
		if a.Agent == "codex" && (a.Limit == nil || a.Limit.Source != "check" || len(a.Limit.Windows) != 2) {
			t.Fatalf("presence %+v", a.Limit)
		}
	}

	failing := func(context.Context, string, string, string) (adapters.LimitState, error) {
		return adapters.LimitState{}, errors.New("Codex did not answer in time")
	}
	if why := s.checkLimitsOnce(context.Background(), failing); why == "" {
		t.Fatal("a failed read was not reported")
	}
	agents, _, _ = st.Presence()
	for _, a := range agents {
		if a.Agent == "codex" && (a.Limit == nil || *a.Limit.UsedPercent != 91) {
			t.Fatalf("a failed read changed the limit: %+v", a.Limit)
		}
	}

	if err := st.SetAgentPaused("codex", true); err != nil {
		t.Fatal(err)
	}
	calls = 0
	if why := s.checkLimitsOnce(context.Background(), read); why != "" || calls != 0 {
		t.Fatalf("paused agent checked: %q, %d calls", why, calls)
	}
}
