package core

import (
	"errors"
	"strings"
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

// A follow-up or an answer runs with the agent's current setting, so turning
// «Любые команды» on helps a task that was started before (T-056).
func TestFollowUpTakesCurrentFullAccess(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "codex", "Fix the menu")
	r := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r, DispositionReport, "Menu fixed")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetAllAgentsFullAccess(true); err != nil {
		t.Fatal(err)
	}
	if err := f.st.ContinueTask(id, "c1", "Commit it", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	runs, _ := f.st.NextTaskRunIntents(10)
	if len(runs) != 1 {
		t.Fatalf("runs %v", runs)
	}
	_, task, err := f.st.BeginTaskRun(runs[0], "res2")
	if err != nil {
		t.Fatal(err)
	}
	if !task.FullAccess {
		t.Fatal("follow-up kept the old setting")
	}
	if stored, _ := f.st.TaskByID(id); !stored.FullAccess {
		t.Fatal("task row not updated")
	}
	// The switch for all agents covers every agent set up on this computer.
	for _, p := range []string{"claude", "codex"} {
		id, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "all-" + p, Provider: p, Text: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if task, _ := f.st.TaskByID(id); !task.FullAccess {
			t.Fatalf("%s without full access", p)
		}
	}
	if err := f.st.SetAllAgentsFullAccess(false); err != nil {
		t.Fatal(err)
	}
	id4, _, _ := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "off", Provider: "claude", Text: "x"})
	if task, _ := f.st.TaskByID(id4); task.FullAccess {
		t.Fatal("switched off for all but task has full access")
	}
}

// A run that ended without a report says what the worker said last (T-056).
func TestNoReportKeepsLastText(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "codex", "Fix the menu")
	r := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	ev := cleanExit
	ev.LastText = "I can not find the tools"
	if _, err := f.st.FinishTaskRun(r.ID, ev); err != nil {
		t.Fatal(err)
	}
	task, _ := f.st.TaskByID(id)
	if task.State != "failed" || !strings.Contains(task.Detail, "no report; last worker message: I can not find the tools") {
		t.Fatalf("task %+v", task)
	}
}

// A newly connected worker agent has «Любые команды» on (owner, D-067);
// saving it again keeps whatever the owner chose since.
func TestNewWorkerAgentFullAccessOn(t *testing.T) {
	f := newTaskFixture(t)
	agents, _ := f.st.ComputerAgents()
	cli := agents[0].CLI
	if err := f.st.SetComputerAgent("cursor", cli, "", "worker"); err != nil {
		t.Fatal(err)
	}
	full := func() bool {
		view, _ := f.st.WindowView("Game", "2000-01-01")
		for _, a := range view.Agents {
			if a.Agent == "cursor" {
				return a.FullAccess
			}
		}
		t.Fatal("cursor missing")
		return false
	}
	if !full() {
		t.Fatal("new worker agent without full access")
	}
	if err := f.st.SetAgentFullAccess("cursor", false); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetComputerAgent("cursor", cli, "", "worker"); err != nil {
		t.Fatal(err)
	}
	if full() {
		t.Fatal("saving again turned it back on")
	}
}

// T-091: a failed run says why in plain words before the raw evidence.
func TestFailedRunSaysWhy(t *testing.T) {
	for _, c := range []struct {
		ev   RunEvidence
		want string
	}{
		{cleanExit, "ended without sending its report"},
		{RunEvidence{ExitObserved: true, ExitCode: 2, GroupEmpty: true, ProviderOK: true}, "stopped with an error (exit code 2)"},
		{RunEvidence{ExitObserved: true, GroupEmpty: true}, "did not report a successful end"},
		{RunEvidence{ExitObserved: true, ExitCode: 1, GroupEmpty: true, Reason: "sign_in"}, "not signed in"},
	} {
		f := newTaskFixture(t)
		id := f.start(t, "op", "codex", "Fix the menu")
		r := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
		if _, err := f.st.FinishTaskRun(r.ID, c.ev); err != nil {
			t.Fatal(err)
		}
		task, _ := f.st.TaskByID(id)
		if task.State != "failed" || !strings.Contains(task.Detail, c.want) {
			t.Fatalf("want %q: %+v", c.want, task)
		}
		events, _, _ := f.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
		if last := events[len(events)-1]; !strings.Contains(last.Text, c.want) {
			t.Fatalf("event %+v", last)
		}
	}
}
