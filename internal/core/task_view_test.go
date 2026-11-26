package core

import (
	"os"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestWindowShowsOnlyLiveCoordinatorOfSelectedProject(t *testing.T) {
	f := newTaskFixture(t)
	self, err := platform.ReadObservation(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.st.CreateProject("Other", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := f.st.TouchCoordinatorClient("Game", "codex"); err != nil {
		t.Fatal(err)
	}
	check := func(project string, active bool) {
		t.Helper()
		view, err := f.st.WindowView(project, "")
		if err != nil || view.Coordinator.Active != active {
			t.Fatalf("%s coordinator = %+v, want active=%t: %v", project, view.Coordinator, active, err)
		}
		activity, err := f.st.ProjectActivity(project)
		if err != nil || activity.Coordinator.Active != active || activity.Coordinator.Client != view.Coordinator.Client {
			t.Fatalf("%s activity coordinator = %+v, want active=%t and client=%q: %v", project, activity.Coordinator, active, view.Coordinator.Client, err)
		}
	}
	check("Game", false)
	stale := self
	stale.StartToken += "-old"
	if _, _, err := f.st.ClaimCoordinator("Game", "first", stale); err != nil {
		t.Fatal(err)
	}
	check("Game", false)
	if old, _, err := f.st.ClaimCoordinator("Game", "first", self); err != nil || old != "" {
		t.Fatalf("relay restart replaced %q: %v", old, err)
	}
	check("Game", true)
	check("Other", false)
	if old, _, err := f.st.ClaimCoordinator("Game", "next", self); err != nil || old != "first" {
		t.Fatalf("takeover replaced %q: %v", old, err)
	}
	check("Game", true)
}

func TestWindowViewAndTaskFeed(t *testing.T) {
	f := newTaskFixture(t)
	v, err := f.st.WindowView("", "2000-01-01T00:00:00Z")
	if err != nil || v.Project != "Game" || v.Folder != f.folder || v.Coordinator.Connected || len(v.Agents) != 2 || len(v.Tasks) != 0 {
		t.Fatalf("empty view %+v %v", v, err)
	}
	if err := f.st.TouchCoordinatorClient("Game", "claude-ai"); err != nil {
		t.Fatal(err)
	}
	id := f.start(t, "op", "claude", "Fix the menu\nDetails follow")
	if v, _ = f.st.WindowView("Game", "2000-01-01T00:00:00Z"); v.Pending != 1 || !v.Coordinator.Connected || v.Coordinator.Client != "claude-ai" || len(v.Tasks) != 1 || !v.Tasks[0].PendingLaunch {
		t.Fatalf("pending %+v", v)
	}
	if feed, err := f.st.TaskFeed(id); err != nil || !feed.Task.PendingLaunch {
		t.Fatalf("feed of an unlaunched task %+v %v", feed.Task, err)
	}
	r := f.launch(t, id, f.sessionOf(t, id))
	if v, _ = f.st.WindowView("Game", "2000-01-01T00:00:00Z"); v.Tasks[0].PendingLaunch || v.Tasks[0].StartedAt == "" || v.Tasks[0].StartedAt < v.Tasks[0].CreatedAt {
		t.Fatalf("launched task %+v", v.Tasks[0])
	}
	if err := f.st.AppendTaskRunItem(r.ID, "text", "", "Looking at the menu", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.st.AppendTaskRunItem(r.ID, "tool", "Bash", "go test ./...", ""); err != nil {
		t.Fatal(err)
	}
	if err := f.st.AppendTaskRunItem(r.ID, "other", "", "x", ""); err == nil {
		t.Fatal("unknown item kind accepted")
	}
	f.dispose(t, r, DispositionAskOwner, "May I rename the save file?")
	if _, err := f.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	v, _ = f.st.WindowView("Game", "2000-01-01T00:00:00Z")
	if len(v.Tasks) != 1 || v.Tasks[0].Title != "Fix the menu" || v.Tasks[0].OwnerQuestion == nil {
		t.Fatalf("tasks %+v", v.Tasks)
	}
	var claude WindowAgent
	for _, a := range v.Agents {
		if a.Agent == "claude" {
			claude = a
		}
	}
	if claude.Waiting != 1 || claude.ProjectToday != 1 {
		t.Fatalf("agent %+v", claude)
	}
	q := v.Tasks[0].OwnerQuestion.ID
	if err := f.st.WindowAnswer(id, q, "w-1", "Yes"); err != nil {
		t.Fatal(err)
	}
	feed, err := f.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	var kinds []string
	for _, it := range feed.Items {
		kinds = append(kinds, it.Kind+":"+it.Author)
	}
	want := "task:coordinator,text:worker,tool:worker,question:worker,answer:owner"
	if got := join(kinds); got != want {
		t.Fatalf("feed %s", got)
	}
	if !feed.Items[3].Verified || feed.Items[3].Audience != AudienceOwner {
		t.Fatalf("question item %+v", feed.Items[3])
	}
}

func join(s []string) string {
	out := ""
	for i, x := range s {
		if i > 0 {
			out += ","
		}
		out += x
	}
	return out
}
