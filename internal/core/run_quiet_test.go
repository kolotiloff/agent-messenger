package core

import "testing"

func TestCodexExitWarningReachesAssigningComputer(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "op", "codex", "Check work")
	p.carry(t, true)
	mirror := p.mirror(t, id)
	r := p.b.launch(t, mirror.ID, "019a0000-0000-7000-8000-000000000001")
	if err := p.b.st.NotifyCodexStillRunning(r.ID); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	before, cursor, err := p.a.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range before {
		if e.Kind == TaskEventWorkerStillRunning {
			t.Fatal("notification before the turn ended")
		}
	}
	if err := p.b.st.NoteRunFinal(r.ID); err != nil {
		t.Fatal(err)
	}
	if err := p.b.st.NotifyCodexStillRunning(r.ID); err != nil {
		t.Fatal(err)
	}
	sent := p.carry(t, false)
	for _, in := range sent {
		if err := p.a.st.ApplyNet(testBroker, in); err != nil {
			t.Fatal(err)
		}
	}
	events, _, err := p.a.st.TaskEvents("Game", AudienceCoordinator, cursor, 50)
	if err != nil || len(events) != 1 || events[0].Kind != TaskEventWorkerStillRunning || events[0].TaskID != id {
		t.Fatalf("coordinator events: %+v %v", events, err)
	}
	if task, err := p.a.st.TaskByID(id); err != nil || task.State != "running" {
		t.Fatalf("remote task: %+v %v", task, err)
	}
	if idle, err := p.a.st.IdleComputers("Game", events); err != nil || len(idle) != 0 {
		t.Fatalf("idle computers: %v %v", idle, err)
	}
	p.b.dispose(t, r, DispositionReport, "All done")
	if _, err := p.b.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	after, _, err := p.a.st.TaskEvents("Game", AudienceCoordinator, events[0].Seq, 50)
	if err != nil || len(after) != 1 || after[0].Kind != "report" {
		t.Fatalf("completion events: %+v %v", after, err)
	}
}
