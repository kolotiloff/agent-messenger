package core

import "testing"

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
	if v, _ = f.st.WindowView("Game", "2000-01-01T00:00:00Z"); v.Pending != 1 || !v.Coordinator.Connected || v.Coordinator.Client != "claude-ai" {
		t.Fatalf("pending %+v", v)
	}
	r := f.launch(t, id, f.sessionOf(t, id))
	if err := f.st.AppendTaskRunItem(r.ID, "text", "", "Looking at the menu"); err != nil {
		t.Fatal(err)
	}
	if err := f.st.AppendTaskRunItem(r.ID, "tool", "Bash", "go test ./..."); err != nil {
		t.Fatal(err)
	}
	if err := f.st.AppendTaskRunItem(r.ID, "other", "", "x"); err == nil {
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
