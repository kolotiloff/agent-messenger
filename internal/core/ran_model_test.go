package core

import "testing"

// T-108: the model a worker's CLI reports is kept with the task and shown in
// its feed next to the one the coordinator asked for.
func TestTaskFeedShowsTheModelTheWorkerRunsOn(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "claude", "Build the grid")
	r := f.launch(t, id, f.sessionOf(t, id))
	feed, err := f.st.TaskFeed(id)
	if err != nil || feed.Task.RanModel != "" {
		t.Fatalf("before the start event: %+v %v", feed.Task, err)
	}
	for _, m := range []string{"", " "} {
		if err := f.st.SetTaskRunModel(r.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.st.SetTaskRunModel(r.ID, "claude-sonnet-test"); err != nil {
		t.Fatal(err)
	}
	feed, err = f.st.TaskFeed(id)
	if err != nil || feed.Task.RanModel != "claude-sonnet-test" {
		t.Fatalf("after the start event: %+v %v", feed.Task, err)
	}
}

func TestRemoteTaskReceivesReportedModel(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "model", "claude", "Build the grid")
	p.carry(t, true)
	mirror := p.mirror(t, id)
	p.carry(t, false)
	r := p.b.launch(t, mirror.ID, p.b.sessionOf(t, mirror.ID))
	const reported = "claude-sonnet-test"
	for _, model := range []string{reported, reported} {
		if err := p.b.st.SetTaskRunModel(r.ID, model); err != nil {
			t.Fatal(err)
		}
	}
	if messages := p.carry(t, false); len(messages) != 1 {
		t.Fatalf("expected one model report, got %d", len(messages))
	}
	feed, err := p.a.st.TaskFeed(id)
	if err != nil || feed.Task.Model != "" || feed.Task.RanModel != reported || feed.Task.State != "running" {
		t.Fatalf("remote task after model report: %+v %v", feed.Task, err)
	}
	view, err := p.a.st.WindowView("Game", "2026-11-20")
	if err != nil || len(view.Tasks) != 1 || view.Tasks[0].RanModel != reported {
		t.Fatalf("window tasks after model report: %+v %v", view.Tasks, err)
	}
}
