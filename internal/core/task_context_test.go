package core

import "testing"

func TestContextSurvivesFinishAndReachesOrigin(t *testing.T) {
	p := newNetPair(t)
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Peer", Age: 1, Agents: []NetAgent{{Agent: "codex"}}, Version: "0.0.0-preview.100"}}); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "context", "codex", "Task context")
	p.carry(t, true)
	mirror := p.mirror(t, id)
	p.carry(t, false)
	r := p.b.launch(t, mirror.ID, "019a0000-0000-7000-8000-0000000000b1")
	check := func(st *Store, task string, want *ContextWindow) {
		t.Helper()
		f, err := st.TaskFeed(task)
		if err != nil {
			t.Fatal(err)
		}
		got := f.Task.ContextWindow
		if (got == nil) != (want == nil) || got != nil && *got != *want {
			t.Fatalf("got %+v want %+v", got, want)
		}
	}
	for _, w := range []*ContextWindow{{800, 1000}, nil, {30, 1000}} {
		if err := p.b.st.NoteTaskContext(r.ID, w); err != nil {
			t.Fatal(err)
		}
		p.carry(t, false)
		check(p.b.st, mirror.ID, w)
		check(p.a.st, id, w)
	}
	p.b.dispose(t, r, DispositionReport, "Complete")
	if _, err := p.b.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	check(p.a.st, id, &ContextWindow{30, 1000})
	if err := p.a.st.ContinueTask(id, "context-continue", "Next", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	check(p.a.st, id, nil)
}
