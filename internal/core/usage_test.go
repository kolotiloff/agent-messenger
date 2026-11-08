package core

import "testing"

// What a task's runs spent adds up on the task and is shown in the window
// (T-058). A run that reported nothing adds nothing; a price stays unknown
// until a run reports one.
func TestTaskUsageAddsUp(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "codex", "Fix the menu")
	usage := func() *TaskUsage {
		view, err := f.st.WindowView("Game", "2000-01-01")
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range view.Tasks {
			if w.ID == id {
				return w.Usage
			}
		}
		t.Fatal("task missing")
		return nil
	}
	if u := usage(); u != nil {
		t.Fatalf("usage before any run %+v", u)
	}
	r := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r, DispositionReport, "Menu fixed")
	ev := cleanExit
	cost := 0.25
	ev.Usage = &TaskUsage{InputTokens: 1000, CachedTokens: 800, OutputTokens: 50, CostUSD: &cost}
	if _, err := f.st.FinishTaskRun(r.ID, ev); err != nil {
		t.Fatal(err)
	}
	if err := f.st.ContinueTask(id, "c1", "Commit it", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	r2 := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, r2, DispositionReport, "Committed")
	ev2 := cleanExit
	cost2 := 0.5
	ev2.Usage = &TaskUsage{InputTokens: 200, OutputTokens: 10, CostUSD: &cost2}
	if _, err := f.st.FinishTaskRun(r2.ID, ev2); err != nil {
		t.Fatal(err)
	}
	u := usage()
	if u == nil || u.InputTokens != 1200 || u.CachedTokens != 800 || u.OutputTokens != 60 || u.CostUSD == nil || *u.CostUSD != 0.75 {
		t.Fatalf("usage %+v", u)
	}
	feed, err := f.st.TaskFeed(id)
	if err != nil || feed.Task.Usage == nil || feed.Task.Usage.OutputTokens != 60 {
		t.Fatalf("feed usage %+v %v", feed.Task.Usage, err)
	}

	// Tokens without a price: the price stays unknown.
	id2 := f.start(t, "op2", "codex", "Build")
	r3 := f.launch(t, id2, "019a0000-0000-7000-8000-000000000002")
	ev3 := cleanExit
	ev3.Usage = &TaskUsage{InputTokens: 10, OutputTokens: 1}
	if _, err := f.st.FinishTaskRun(r3.ID, ev3); err != nil {
		t.Fatal(err)
	}
	task, _ := f.st.TaskByID(id2)
	if task.State != "failed" {
		t.Fatalf("no report should fail: %s", task.State)
	}
	view, _ := f.st.WindowView("Game", "2000-01-01")
	for _, w := range view.Tasks {
		if w.ID == id2 && (w.Usage == nil || w.Usage.InputTokens != 10 || w.Usage.CostUSD != nil) {
			t.Fatalf("codex usage %+v", w.Usage)
		}
	}
}

// A task another computer runs reports what it spent back to the computer
// that started it (T-058).
func TestRemoteTaskUsageReachesTheStarter(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "op1", "codex", "Build level 1")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-000000000001")
	p.b.dispose(t, r, DispositionReport, "Built")
	ev := cleanExit
	cost := 1.5
	ev.Usage = &TaskUsage{InputTokens: 3000, CachedTokens: 1000, OutputTokens: 70, CostUSD: &cost}
	if _, err := p.b.st.FinishTaskRun(r.ID, ev); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	feed, err := p.a.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	u := feed.Task.Usage
	if u == nil || u.InputTokens != 3000 || u.CachedTokens != 1000 || u.OutputTokens != 70 || u.CostUSD == nil || *u.CostUSD != 1.5 {
		t.Fatalf("starter usage %+v", u)
	}
}
