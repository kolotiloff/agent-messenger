package core

import (
	"errors"
	"testing"
)

// «Разрешить любые команды» (T-054, D-065): off by default, set per agent of
// this computer, copied into each new task, shown to the window and to the
// coordinator.
func TestAgentFullAccess(t *testing.T) {
	f := newTaskFixture(t)
	id, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-1", Provider: "codex", Text: "Fix"})
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := f.st.TaskByID(id); task.FullAccess {
		t.Fatal("full access on by default")
	}
	if err := f.st.SetAgentFullAccess("codex", true); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetAgentFullAccess("gemini", true); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown agent: %v", err)
	}
	// A started task keeps what it started with.
	if task, _ := f.st.TaskByID(id); task.FullAccess {
		t.Fatal("started task changed")
	}
	id2, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-2", Provider: "codex", Text: "Commit"})
	if err != nil {
		t.Fatal(err)
	}
	if task, _ := f.st.TaskByID(id2); !task.FullAccess {
		t.Fatal("new task without full access")
	}
	// Saving the program again keeps the switch.
	agents, err := f.st.ComputerAgents()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		if a.Provider == "codex" {
			if err := f.st.SetComputerAgent("codex", a.CLI, a.Environment, a.Profile); err != nil {
				t.Fatal(err)
			}
		}
	}
	view, err := f.st.WindowView("Game", "2000-01-01")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range view.Agents {
		if a.FullAccess != (a.Agent == "codex") {
			t.Fatalf("window %s full_access %v", a.Agent, a.FullAccess)
		}
	}
	list, err := f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list["computers"].([]ComputerSummary)[0].Agents {
		if a.FullAccess != (a.Agent == "codex") {
			t.Fatalf("list_agents %s full_access %v", a.Agent, a.FullAccess)
		}
	}
	if err := f.st.SetAgentFullAccess("codex", false); err != nil {
		t.Fatal(err)
	}
	id3, _, _ := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-3", Provider: "codex", Text: "Again"})
	if task, _ := f.st.TaskByID(id3); task.FullAccess {
		t.Fatal("switched off but task has full access")
	}
}
