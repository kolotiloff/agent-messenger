package core

import (
	"errors"
	"testing"
)

// The per-card switch (T-061): a turned-off agent keeps its settings, takes
// no new tasks and follow-ups, and is hidden from the coordinator and from
// other computers; turning it on again brings everything back.
func TestAgentPaused(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.SetAgentModels("codex", []string{"gpt-5-codex"}); err != nil {
		t.Fatal(err)
	}
	id := f.start(t, "op-1", "codex", "Fix")
	r := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r, DispositionReport, "Fixed")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetAgentPaused("gemini", true); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown agent: %v", err)
	}
	if err := f.st.SetAgentPaused("codex", true); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-2", Provider: "codex", Text: "More"}); !errors.Is(err, ErrRejected) {
		t.Fatalf("start on a turned-off agent: %v", err)
	}
	if err := f.st.ContinueTask(id, "c1", "Commit it", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("follow-up on a turned-off agent: %v", err)
	}
	list, err := f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range list["computers"].([]ComputerSummary)[0].Agents {
		if a.Agent == "codex" {
			t.Fatal("list_agents shows a turned-off agent")
		}
	}
	presence, _, err := f.st.Presence()
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range presence {
		if a.Agent == "codex" {
			t.Fatal("network presence shows a turned-off agent")
		}
	}
	view, err := f.st.WindowView("Game", "2000-01-01")
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, a := range view.Agents {
		if a.Agent == "codex" {
			seen = true
			if !a.Paused || len(a.Models) != 1 {
				t.Fatalf("window agent %+v", a)
			}
		} else if a.Paused {
			t.Fatalf("%s paused", a.Agent)
		}
	}
	if !seen {
		t.Fatal("window lost the turned-off agent")
	}
	if err := f.st.SetAgentPaused("codex", false); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-3", Provider: "codex", Text: "More"}); err != nil {
		t.Fatal(err)
	}
}

// The card shows today's tokens of the agent on this computer and when its
// last task finished well (T-061, D-070).
func TestAgentCardFacts(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op-1", "codex", "Fix")
	r := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r, DispositionReport, "Fixed")
	ev := cleanExit
	ev.Usage = &TaskUsage{InputTokens: 1000, CachedTokens: 900, OutputTokens: 50}
	if _, err := f.st.FinishTaskRun(r.ID, ev); err != nil {
		t.Fatal(err)
	}
	card := func(today string) WindowAgent {
		view, err := f.st.WindowView("Game", today)
		if err != nil {
			t.Fatal(err)
		}
		for _, a := range view.Agents {
			if a.Agent == "codex" {
				return a
			}
		}
		t.Fatal("codex missing")
		return WindowAgent{}
	}
	a := card("2000-01-01")
	if a.TodayTokens != 1050 || a.LastDone == "" {
		t.Fatalf("card %+v", a)
	}
	if a := card("2999-01-01"); a.TodayTokens != 0 || a.LastDone == "" {
		t.Fatalf("card for a later day %+v", a)
	}
}

// A newly connected Claude lists its own model names (T-061).
func TestClaudeDefaultModels(t *testing.T) {
	f := newTaskFixture(t)
	view, err := f.st.WindowView("Game", "2000-01-01")
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range view.Agents {
		want := 0
		if a.Agent == "claude" {
			want = 3
		}
		if len(a.Models) != want {
			t.Fatalf("%s models %v", a.Agent, a.Models)
		}
	}
}
