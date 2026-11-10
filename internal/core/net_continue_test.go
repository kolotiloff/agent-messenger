package core

import (
	"errors"
	"strings"
	"testing"
)

// T-113: the coordinator continues a finished task of another computer; the
// same worker conversation resumes there and its report comes back.
func TestRemoteTaskContinue(t *testing.T) {
	p := newNetPair(t)
	agents := []NetAgent{{Agent: "claude"}, {Agent: "codex"}}
	setPeerVersion := func(v string) {
		t.Helper()
		if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Agents: agents, Version: v}}); err != nil {
			t.Fatal(err)
		}
	}
	setPeerVersion("0.0.0-preview.60")
	id := p.startRemote(t, "op1", "codex", "Build level 1")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	if err := p.a.st.ContinueTask(id, "c0", "more", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("continued a running task: %v", err)
	}
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000b1")
	p.b.dispose(t, r, DispositionReport, "Level 1 is built")
	if _, err := p.b.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	if task, _ := p.a.st.TaskByID(id); task.State != "done" {
		t.Fatalf("done %+v", task)
	}
	// An older computer would drop the message: refused here.
	setPeerVersion("0.0.0-preview.59")
	if err := p.a.st.ContinueTask(id, "c1", "Add level 2", AudienceCoordinator); !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "older") {
		t.Fatalf("older computer: %v", err)
	}
	setPeerVersion("0.0.0-preview.60")
	if err := p.a.st.ContinueTask(id, "c1", "Add level 2", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	// Repeating after a lost response sends nothing new.
	if err := p.a.st.ContinueTask(id, "c1", "Add level 2", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	if task, _ := p.a.st.TaskByID(id); task.State != "running" {
		t.Fatalf("continued %+v", task)
	}
	sent := p.carry(t, true)
	if len(sent) != 1 {
		t.Fatalf("continue messages %+v", sent)
	}
	// Read twice, it opens one run.
	if err := p.b.st.ApplyNet(testBroker, sent[0]); err != nil {
		t.Fatal(err)
	}
	r2 := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000b1")
	brief, err := p.b.st.ReadTask(r2.ID, r2.Receipt, false)
	if err != nil || r2.Kind != "continue" || brief.FollowUp != "Add level 2" {
		t.Fatalf("brief %+v %+v %v", r2, brief, err)
	}
	p.b.dispose(t, r2, DispositionReport, "Level 2 is built")
	if _, err := p.b.st.FinishTaskRun(r2.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	task, _ := p.a.st.TaskByID(id)
	events := p.a.events(t, AudienceCoordinator, 0)
	if task.State != "done" || events[len(events)-1].Text != "Level 2 is built" {
		t.Fatalf("second report %+v %+v", task, events)
	}
	if err := p.b.st.ContinueTask(m.ID, "c-b", "x", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("b's coordinator continued a's task: %v", err)
	}
}

// A computer that cannot continue the task says why; the task goes back to
// how it was, and the coordinator hears it.
func TestRemoteTaskContinueRefused(t *testing.T) {
	p := newNetPair(t)
	agents := []NetAgent{{Agent: "codex"}}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Agents: agents, Version: "0.0.0-preview.60"}}); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op1", "codex", "Build level 1")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	// It failed on b before any conversation was recorded.
	if _, err := p.b.st.CancelTask(m.ID); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	if task, _ := p.a.st.TaskByID(id); task.State != "cancelled" {
		t.Fatalf("cancelled mirror is reported as %s", task.State)
	}
	if err := p.a.st.ContinueTask(id, "c1", "more", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("continued a cancelled task: %v", err)
	}
	// Make the task look failed on a, so the follow-up is sent and b refuses.
	if _, err := p.a.st.db.Exec(`UPDATE tasks SET state='failed' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if err := p.a.st.ContinueTask(id, "c2", "more", AudienceCoordinator); err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	p.carry(t, false)
	task, _ := p.a.st.TaskByID(id)
	events := p.a.events(t, AudienceCoordinator, 0)
	if task.State != "failed" || events[len(events)-1].Kind != "continue_refused" || !strings.Contains(events[len(events)-1].Text, "Ноутбук did not continue") {
		t.Fatalf("refused %+v %+v", task, events)
	}
}
