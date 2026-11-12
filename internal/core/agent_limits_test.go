package core

import (
	"errors"
	"testing"
	"time"
)

// T-044: the last limit state of an agent reaches list_agents and the window.
func TestAgentLimit(t *testing.T) {
	f := newTaskFixture(t)
	used := 62.5
	if err := f.st.RecordAgentLimit("codex", AgentLimit{State: "ok", UsedPercent: &used, Window: "5h", ResetsAt: "2026-11-12T10:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	list, err := f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list["computers"].([]ComputerSummary)[0].Agents {
		switch a.Agent {
		case "codex":
			if a.Limit == nil || a.Limit.State != "ok" || *a.Limit.UsedPercent != 62.5 || a.Limit.Window != "5h" || a.Limit.ResetsAt != "2026-11-12T10:00:00Z" || a.Limit.ObservedAt == "" {
				t.Fatalf("codex limit %+v", a.Limit)
			}
		case "claude":
			if a.Limit != nil {
				t.Fatalf("claude reported no limit: %+v", a.Limit)
			}
		}
	}
	view, err := f.st.WindowView("Game", "2026-11-08")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range view.Agents {
		if a.Agent == "codex" && (a.Limit == nil || a.Limit.State != "ok") {
			t.Fatalf("window limit %+v", a.Limit)
		}
	}
	agents, _, err := f.st.Presence()
	if err != nil || agents[1].Limit == nil {
		t.Fatalf("presence %+v %v", agents, err)
	}
	bad := -1.0
	for _, l := range []AgentLimit{{State: "fine"}, {State: "ok", UsedPercent: &bad}, {State: "ok", ResetsAt: "tomorrow"}} {
		if err := f.st.RecordAgentLimit("codex", l); !errors.Is(err, ErrRejected) {
			t.Fatalf("%+v accepted: %v", l, err)
		}
	}
}

// Coordinator feedback 2026-11-10: a completed run clears a stale
// «exhausted»; a «warning» stays.
func TestCompletedRunClearsExhausted(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.RecordAgentLimit("codex", AgentLimit{State: "exhausted", Window: "5h", ResetsAt: "2099-01-01T00:00:00Z"}); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	if err := f.st.AgentLimitCleared("codex", at); err != nil {
		t.Fatal(err)
	}
	limit := func() *AgentLimit {
		list, err := f.st.ProjectAgents("Game")
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range list["computers"].([]ComputerSummary)[0].Agents {
			if a.Agent == "codex" {
				return a.Limit
			}
		}
		return nil
	}
	if l := limit(); l == nil || l.State != "ok" || l.Window != "5h" || l.ResetsAt != "" || l.ObservedAt != "2026-11-12T09:00:00Z" {
		t.Fatalf("not cleared: %+v", l)
	}
	if err := f.st.RecordAgentLimit("codex", AgentLimit{State: "warning", Window: "5h"}); err != nil {
		t.Fatal(err)
	}
	if err := f.st.AgentLimitCleared("codex", at); err != nil {
		t.Fatal(err)
	}
	if l := limit(); l == nil || l.State != "warning" {
		t.Fatalf("warning dropped: %+v", l)
	}
	if err := f.st.AgentLimitCleared("claude", at); err != nil {
		t.Fatal(err)
	}
}
