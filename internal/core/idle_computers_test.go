package core

import (
	"context"
	"database/sql"
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
		{Name: "waiting-update", Online: true, UpdatePending: true, Agents: []AgentSummary{{Agent: "codex"}}},
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

func TestIdleComputersReadsSplitProjectWhileWritersHoldBothDatabases(t *testing.T) {
	f := newSplitRunFixture(t)
	st, err := openSplitStore(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if err := st.SetComputerName("Fixture Mac"); err != nil {
		t.Fatal(err)
	}
	start := func(project, op string) string {
		t.Helper()
		id, _, err := st.StartTask(StartTaskRequest{Project: project, OpID: op, Provider: "codex", Text: "Fixture"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	finished := start("Game", "finished")
	stillRunning := start("Game", "another")
	start("Site", "isolated")
	if _, err := st.CancelTask(finished); err != nil {
		t.Fatal(err)
	}
	events := []TaskEvent{{TaskID: finished, Kind: "cancelled"}, {TaskID: finished, Kind: "cancelled"}}
	if idle, err := st.IdleComputers("Game", events); err != nil || len(idle) != 0 {
		t.Fatalf("computer with a running Game task: %v %v", idle, err)
	}
	if _, err := st.CancelTask(stillRunning); err != nil {
		t.Fatal(err)
	}
	for _, db := range []*sql.DB{f.computer, f.projects[projectMigrationFile("Game")]} {
		tx, err := db.Begin()
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback()
	}
	if idle, err := st.IdleComputers("Game", events); err != nil || !slices.Equal(idle, []string{"Fixture Mac"}) {
		t.Fatalf("idle in Game while Site runs: %v %v", idle, err)
	}
	if idle, err := st.IdleComputers("Site", events); err != nil || len(idle) != 0 {
		t.Fatalf("Game event appeared in Site: %v %v", idle, err)
	}
}

func TestPresenceReadsWhileTheMachineDatabaseHasAWriter(t *testing.T) {
	f := newSplitRunFixture(t)
	if _, err := f.computer.Exec(`ALTER TABLE net_peers DROP COLUMN contact_age`); err != nil {
		t.Fatal(err)
	}
	st, err := openSplitStore(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.NetPeers(); err != nil {
		t.Fatal(err)
	}
	writer, err := f.computer.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	if agents, _, err := st.Presence(); err != nil || len(agents) == 0 {
		t.Fatalf("presence while a writer holds the database: %+v %v", agents, err)
	}
}
