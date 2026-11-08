package core

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// The owner lists the model names an agent accepts (T-043). The coordinator
// sees them, and Codex and Cursor take only a listed model, so a guessed name
// never reaches the CLI.
func TestAgentModels(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.SetAgentModels("codex", []string{" gpt-5.5 ", "", "gpt-5.5-mini", "gpt-5.5"}); err != nil {
		t.Fatal(err)
	}
	agents, err := f.st.ComputerAgents()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range agents {
		want := []string{}
		if a.Provider == "codex" {
			want = []string{"gpt-5.5", "gpt-5.5-mini"}
		}
		if a.Provider == "claude" {
			want = DefaultClaudeModels // listed from the start (T-061)
		}
		if !reflect.DeepEqual(a.Models, want) {
			t.Fatalf("%s models %q", a.Provider, a.Models)
		}
	}
	// Saving the program again keeps the list.
	if err := f.st.SetComputerAgent("codex", agents[1].CLI, "", "messenger-only"); err != nil {
		t.Fatal(err)
	}
	list, err := f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	computers := list["computers"].([]ComputerSummary)
	var codex AgentSummary
	for _, a := range computers[0].Agents {
		if a.Agent == "codex" {
			codex = a
		}
	}
	if !reflect.DeepEqual(codex.Models, []string{"gpt-5.5", "gpt-5.5-mini"}) {
		t.Fatalf("list_agents models %q", codex.Models)
	}

	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-ok", Provider: "codex", Text: "Fix", Model: "gpt-5.5-mini"}); err != nil {
		t.Fatalf("listed model: %v", err)
	}
	_, _, err = f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-guess", Provider: "codex", Text: "Fix", Model: "gpt-6"})
	if !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "gpt-5.5-mini") {
		t.Fatalf("unlisted model: %v", err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-default", Provider: "codex", Text: "Fix"}); err != nil {
		t.Fatalf("default model: %v", err)
	}
	// Claude's own names (opus, sonnet, haiku) keep working without a list.
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-claude", Provider: "claude", Text: "Fix", Model: "opus"}); err != nil {
		t.Fatalf("claude alias: %v", err)
	}
	// An empty list means no model may be named for Codex.
	if err := f.st.SetAgentModels("codex", nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-none", Provider: "codex", Text: "Fix", Model: "gpt-5.5"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("model without a list: %v", err)
	}

	for name, models := range map[string][]string{
		"flag":     {"--yolo"},
		"space":    {"gpt 5"},
		"too many": strings.Fields(strings.Repeat("m ", MaxAgentModels+1)),
	} {
		if name == "too many" {
			for i := range models {
				models[i] = "m" + strings.Repeat("x", i)
			}
		}
		if err := f.st.SetAgentModels("codex", models); !errors.Is(err, ErrRejected) {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
	if err := f.st.SetAgentModels("cursor", []string{"x"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("agent that is not set up: %v", err)
	}
}

// list_agents names this computer as its owner named it (D-056).
func TestProjectAgentsUsesTheComputerName(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.SetComputerName("Студия"); err != nil {
		t.Fatal(err)
	}
	list, err := f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	if got := list["computers"].([]ComputerSummary)[0].Name; got != "Студия" {
		t.Fatalf("name %q", got)
	}
}

// T-046: the owner disconnects an agent; one with an unfinished task stays.
func TestRemoveComputerAgent(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op-1", "codex", "Fix")
	if err := f.st.RemoveComputerAgent("codex"); !errors.Is(err, ErrRejected) {
		t.Fatalf("agent with a running task removed: %v", err)
	}
	if err := f.st.RemoveComputerAgent("claude"); err != nil {
		t.Fatal(err)
	}
	agents, _ := f.st.ComputerAgents()
	if len(agents) != 1 || agents[0].Provider != "codex" {
		t.Fatalf("agents %+v", agents)
	}
	if err := f.st.RemoveComputerAgent("claude"); !errors.Is(err, ErrRejected) {
		t.Fatalf("second removal: %v", err)
	}
	if task, err := f.st.TaskByID(id); err != nil || task.CLI == "" {
		t.Fatalf("task %+v %v", task, err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-2", Provider: "claude", Text: "x"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("task for a removed agent: %v", err)
	}
}
