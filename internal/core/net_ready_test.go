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

// T-036: the computer that runs the server is marked for the coordinator,
// whether it is this computer or another one.
func TestProjectAgentsMarkServer(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.SetNetwork("brk-1", "host", []NetPeer{{ID: "dev-a", Name: "A", Age: 1}}); err != nil {
		t.Fatal(err)
	}
	list, err := f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	computers := list["computers"].([]ComputerSummary)
	if !computers[0].This || !computers[0].Server || computers[1].Server {
		t.Fatalf("hosting: %+v", computers)
	}
	if err := f.st.SetNetwork("brk-1", "dev-me", []NetPeer{{ID: "host", Name: "VPS", Host: true, Age: 1}}); err != nil {
		t.Fatal(err)
	}
	list, err = f.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	computers = list["computers"].([]ComputerSummary)
	if computers[0].Server || !computers[1].Server {
		t.Fatalf("member: %+v", computers)
	}
}
