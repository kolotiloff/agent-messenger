package core

import (
	"slices"
	"testing"
)

// Coordinator feedback 2026-11-10: list_agents names the free machines.
func TestIdleComputersInListAgents(t *testing.T) {
	yes, no := true, false
	list := []ComputerSummary{
		{Name: "Mac", This: true, Online: true, Folder: "/game", Agents: []AgentSummary{{Agent: "claude"}, {Agent: "codex", Waiting: 1}}},
		{Name: "busy", Online: true, Agents: []AgentSummary{{Agent: "codex", Running: 1}}},
		{Name: "offline", Agents: []AgentSummary{{Agent: "codex"}}},
		{Name: "refuses", Online: true, AcceptsTasks: &no, HasProject: &yes, Agents: []AgentSummary{{Agent: "codex"}}},
		{Name: "no-folder", Online: true, AcceptsTasks: &yes, HasProject: &no, Agents: []AgentSummary{{Agent: "codex"}}},
		{Name: "reconnecting", Online: true, Reconnecting: true, Agents: []AgentSummary{{Agent: "codex"}}},
		{Name: "no-agents", Online: true, Agents: []AgentSummary{}},
		{Name: "free", Online: true, AcceptsTasks: &yes, HasProject: &yes, Agents: []AgentSummary{{Agent: "codex"}}},
	}
	if got := idleComputers(list); !slices.Equal(got, []string{"Mac", "free"}) {
		t.Fatalf("idle %v", got)
	}
	f := newTaskFixture(t)
	out, err := f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	if idle, _ := out["idle_computers"].([]string); len(idle) != 1 {
		t.Fatalf("fresh computer not idle: %v", out["idle_computers"])
	}
	f.start(t, "op-1", "codex", "x")
	out, _ = f.st.ProjectAgents("Game")
	if _, ok := out["idle_computers"]; ok {
		t.Fatalf("a computer with a running task is idle: %v", out["idle_computers"])
	}
}
