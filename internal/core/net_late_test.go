package core

import (
	"strings"
	"testing"
	"time"
)

// T-150: a report that comes after the task was closed here reaches the
// coordinator as late_report; the task stays closed.
func TestReportAfterCancelReachesTheCoordinator(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op", "codex", "Do it")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	// Closed here while the worker went on there.
	if _, err := p.a.st.db.Exec(`UPDATE tasks SET state='cancelled' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000e1")
	p.b.dispose(t, r, DispositionReport, "committed cs:42")
	if _, err := p.b.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	task, _ := p.a.st.TaskByID(id)
	if task.State != "cancelled" {
		t.Fatalf("task reopened: %s", task.State)
	}
	var late *TaskEvent
	for _, e := range p.a.events(t, AudienceCoordinator, 0) {
		if e.TaskID == id && e.Kind == "late_report" {
			e := e
			late = &e
		}
	}
	if late == nil || !strings.Contains(late.Text, "committed cs:42") || !strings.Contains(late.Text, "closed here before") {
		t.Fatalf("no late report: %s", kinds(p.a.events(t, AudienceCoordinator, 0)))
	}
	feed, err := p.a.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	shown := false
	for _, it := range feed.Items {
		shown = shown || (it.Kind == "report" && strings.Contains(it.Text, "committed cs:42"))
	}
	if !shown {
		t.Fatal("late report not in the window's feed")
	}
}

// T-150: a computer reports how long its oldest unsent message waited; the
// coordinator sees it on the computer and on its tasks.
func TestDelayedOutboxIsShown(t *testing.T) {
	p := newNetPair(t)
	// The worker's side: a message for the listed giver waits 10 minutes.
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op", "codex", "Do it")
	p.carry(t, true)
	if b, err := p.b.st.OutboxBacklog(); err != nil || b.Waiting == 0 {
		t.Fatalf("backlog %+v %v", b, err)
	}
	old := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339Nano)
	if _, err := p.b.st.db.Exec(`UPDATE net_outbox SET created_at=?`, old); err != nil {
		t.Fatal(err)
	}
	b, err := p.b.st.OutboxBacklog()
	if err != nil || b.OldestAge < 590 {
		t.Fatalf("backlog %+v %v", b, err)
	}
	// Messages for a computer that is not listed are not a delay.
	if err := p.b.st.SetNetwork(testBroker, "dev-b", nil); err != nil {
		t.Fatal(err)
	}
	if b, _ := p.b.st.OutboxBacklog(); b.Waiting != 0 {
		t.Fatalf("unlisted recipient counted: %+v", b)
	}

	// The giver's side sees the report of the worker's computer.
	agents := []NetAgent{{Agent: "codex"}}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 10, Agents: agents, Outbox: &NetOutbox{Waiting: 3, OldestAge: 290}}}); err != nil {
		t.Fatal(err)
	}
	r, err := p.a.st.TaskRemote(id)
	if err != nil || r == nil || r.Delayed != 5 {
		t.Fatalf("task remote %+v %v", r, err)
	}
	list, err := p.a.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	cs := list["computers"].([]ComputerSummary)
	if len(cs) != 2 || cs[1].OutboxDelayed != 5 || cs[0].OutboxDelayed != 0 {
		t.Fatalf("computers %+v", cs)
	}
	// A short wait is normal and not shown.
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 5, Agents: agents, Outbox: &NetOutbox{Waiting: 1, OldestAge: 20}}}); err != nil {
		t.Fatal(err)
	}
	if r, _ := p.a.st.TaskRemote(id); r.Delayed != 0 {
		t.Fatalf("short wait shown: %+v", r)
	}
}

// T-149: the computer that runs a task says whether its worker process
// runs now and when it last wrote output.
func TestRemoteWorkerIsShown(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op", "codex", "Do it")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	// The running side maps its mirror to the giver's task id.
	origins, err := p.b.st.OriginTasks([]string{m.ID, "task-unknown"})
	if err != nil || len(origins) != 1 || origins[m.ID] != id {
		t.Fatalf("origins %v %v", origins, err)
	}
	agents := []NetAgent{{Agent: "codex"}}
	set := func(w *NetWorkers) {
		t.Helper()
		if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 2, Agents: agents, Workers: w}}); err != nil {
			t.Fatal(err)
		}
	}
	// A computer before 0.67 says nothing.
	if r, _ := p.a.st.TaskRemote(id); r.Worker != "" {
		t.Fatalf("old computer %+v", r)
	}
	set(&NetWorkers{Runs: []NetWorkerRun{{Task: id, OutputAge: 600}}})
	r, err := p.a.st.TaskRemote(id)
	if err != nil || r.Worker != "running" {
		t.Fatalf("running %+v %v", r, err)
	}
	at, err := time.Parse(time.RFC3339, r.WorkerOutputAt)
	if err != nil || time.Since(at) < 9*time.Minute || time.Since(at) > 11*time.Minute {
		t.Fatalf("output at %q %v", r.WorkerOutputAt, err)
	}
	set(&NetWorkers{Runs: []NetWorkerRun{}})
	if r, _ := p.a.st.TaskRemote(id); r.Worker != "none" || r.WorkerOutputAt != "" {
		t.Fatalf("none %+v", r)
	}
	// Only a working task: one that waits for an answer has no worker.
	if _, err := p.a.st.db.Exec(`UPDATE tasks SET state='waiting_coordinator' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if r, _ := p.a.st.TaskRemote(id); r.Worker != "" {
		t.Fatalf("waiting task %+v", r)
	}
}
