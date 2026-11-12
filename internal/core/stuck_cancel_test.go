package core

import (
	"errors"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// interrupt marks a launched task as interrupted, as a client restart does.
func interrupt(t *testing.T, st *Store, task string) {
	t.Helper()
	if _, err := st.db.Exec(`UPDATE task_runs SET state='interrupted' WHERE task_id=? AND state='running'`, task); err != nil {
		t.Fatal(err)
	}
	if _, err := st.db.Exec(`UPDATE tasks SET state='interrupted' WHERE id=?`, task); err != nil {
		t.Fatal(err)
	}
}

func goneOK(platform.Observation) (string, error) { return "the computer restarted since", nil }
func stillThere(platform.Observation) (string, error) {
	return "", errors.New("its process or a process of its group is still running")
}

// D-120: an explicit cancel closes a stuck task only when its worker is
// certainly gone; anything unclear leaves it as it was.
func TestExplicitCancelClosesStuckTaskWhenWorkerGone(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "op", "claude", "x")
	f.launch(t, id, f.sessionOf(t, id))
	interrupt(t, f.st, id)

	// Without a check (as before): refused.
	if _, err := f.st.CancelTask(id); !errors.Is(err, ErrRejected) {
		t.Fatalf("cancel without check: %v", err)
	}
	// The worker may still run: refused, nothing changes.
	f.st.SetWorkerCheck(stillThere)
	if _, err := f.st.CancelTask(id); !errors.Is(err, ErrWorkerMayRun) {
		t.Fatalf("cancel with a live worker: %v", err)
	}
	if task, _ := f.st.TaskByID(id); task.State != "interrupted" {
		t.Fatalf("state changed %s", task.State)
	}
	if n, _ := f.st.ActiveWork(); n != 1 {
		t.Fatalf("active work %d", n)
	}
	if list, _ := f.st.ActiveWorkTasks(3); len(list) != 1 || list[0].State != "interrupted" || list[0].Title == "" {
		t.Fatalf("active list %+v", list)
	}
	// Gone: closed, the run too, and the computer is free.
	f.st.SetWorkerCheck(goneOK)
	if state, err := f.st.CancelTask(id); err != nil || state != "cancelled" {
		t.Fatalf("cancel %s %v", state, err)
	}
	if n, _ := f.st.ActiveWork(); n != 0 {
		t.Fatalf("still active %d", n)
	}
	if !strings.Contains(kinds(f.events(t, AudienceCoordinator, 0)), "cancelled") {
		t.Fatal("coordinator not told")
	}
}

// D-120: the cancel of the computer that gave the task closes it on the
// computer that ran it, or tells the giver why not.
func TestRemoteCancelClosesStuckTaskThere(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	run := func(op string) (string, Task) {
		id := p.startRemote(t, op, "codex", "Job "+op)
		p.carry(t, true)
		p.carry(t, false)
		m := p.mirror(t, id)
		p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000c"+op)
		interrupt(t, p.b.st, m.ID)
		return id, m
	}

	// The worker may still run there: the task stays, the giver learns why.
	p.b.st.SetWorkerCheck(stillThere)
	held, heldMirror := run("1")
	if _, err := p.a.st.CancelTask(held); err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	if m, _ := p.b.st.TaskByID(heldMirror.ID); m.State != "interrupted" {
		t.Fatalf("closed with a live worker: %s", m.State)
	}
	p.carry(t, false)
	refused := false
	for _, e := range p.a.events(t, AudienceCoordinator, 0) {
		refused = refused || (e.TaskID == held && e.Kind == "cancel_refused")
	}
	if !refused {
		t.Fatalf("giver not told: %s", kinds(p.a.events(t, AudienceCoordinator, 0)))
	}

	// Gone there: closed there, and the giver's task is cancelled.
	p.b.st.SetWorkerCheck(goneOK)
	freed, freedMirror := run("2")
	if _, err := p.a.st.CancelTask(freed); err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	if m, _ := p.b.st.TaskByID(freedMirror.ID); m.State != "cancelled" {
		t.Fatalf("not closed there: %s", m.State)
	}
	p.carry(t, false)
	if task, _ := p.a.st.TaskByID(freed); task.State != "cancelled" {
		t.Fatalf("giver task %s", task.State)
	}
}
