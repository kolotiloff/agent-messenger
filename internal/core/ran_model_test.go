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
