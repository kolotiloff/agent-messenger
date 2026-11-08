package core

import "testing"

func TestProjectAgentsShowReadiness(t *testing.T) {
	f := newTaskFixture(t)
	peers := []NetPeer{
		{ID: "dev-a", Name: "Готов", Age: 1, Ready: &NetReady{Accept: true, Projects: []string{"Game", "Other"}}},
		{ID: "dev-b", Name: "Без папки", Age: 1, Ready: &NetReady{Accept: true, Projects: []string{"Other"}}},
		{ID: "dev-c", Name: "Не принимает", Age: 1, Ready: &NetReady{Accept: false, Projects: []string{"Game"}}},
		{ID: "dev-d", Name: "Старый", Age: 1},
	}
	if err := f.st.SetNetwork("brk-1", "host", peers); err != nil {
		t.Fatal(err)
	}
	list, err := f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ComputerSummary{}
	for _, c := range list["computers"].([]ComputerSummary) {
		got[c.Computer] = c
	}
	yes := func(b *bool) bool { return b != nil && *b }
	no := func(b *bool) bool { return b != nil && !*b }
	if a := got["dev-a"]; !yes(a.AcceptsTasks) || !yes(a.HasProject) || a.Note != "" {
		t.Fatalf("ready %+v", a)
	}
	if b := got["dev-b"]; !yes(b.AcceptsTasks) || !no(b.HasProject) || b.Note == "" {
		t.Fatalf("no folder %+v", b)
	}
	if c := got["dev-c"]; !no(c.AcceptsTasks) || c.Note == "" {
		t.Fatalf("refuses %+v", c)
	}
	if d := got["dev-d"]; d.AcceptsTasks != nil || d.HasProject != nil || d.Note == "" {
		t.Fatalf("unknown %+v", d)
	}
	if here := got[list["computers"].([]ComputerSummary)[0].Computer]; here.AcceptsTasks != nil {
		t.Fatalf("this computer %+v", here)
	}
}
