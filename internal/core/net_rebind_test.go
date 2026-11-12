package core

import (
	"strings"
	"testing"
	"time"
)

// carryVia delivers one side's waiting messages of a network to the other
// side, as sent by the given peer id.
func (p *netPair) carryVia(t *testing.T, broker string, fromA bool, sender string) int {
	t.Helper()
	from, to := p.a.st, p.b.st
	if !fromA {
		from, to = p.b.st, p.a.st
	}
	pending, err := from.NetPending(broker, 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range pending {
		p.seq++
		if err := to.ApplyNet(broker, NetIncoming{Seq: p.seq, From: sender, OpID: m.OpID, Body: m.Body}); err != nil {
			t.Fatal(err)
		}
		if err := from.NetSent(m.Seq); err != nil {
			t.Fatal(err)
		}
	}
	return len(pending)
}

// T-132: the server moved to the other computer, so both computers have new
// peer ids in a new network. A task sent before the move follows its
// computer: it is not lost, a cancel queued before the move is sent through
// the new network, and the answer comes back.
func TestTasksFollowAComputerIntoANewNetwork(t *testing.T) {
	p := newNetPair(t)
	agents := []NetAgent{{Agent: "claude"}}
	aID, err := p.a.st.ComputerID()
	if err != nil {
		t.Fatal(err)
	}
	bID, err := p.b.st.ComputerID()
	if err != nil {
		t.Fatal(err)
	}
	// The old network learns the lasting ids from the computers' reports.
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Agents: agents, Computer: bID}}); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op-move", "claude", "night run")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	// A cancel is queued but the old network goes away before it is sent.
	if state, err := p.a.st.CancelTask(id); err != nil || state != "cancelling" {
		t.Fatalf("cancel %s %v", state, err)
	}

	const newBroker = "brk-00000000000000bb"
	if err := p.a.st.SetNetwork(newBroker, "dev-a2", []NetPeer{{ID: HostPeerID, Name: "Ноутбук", Host: true, Age: 1, Agents: agents, Computer: bID}}); err != nil {
		t.Fatal(err)
	}
	if r, err := p.a.st.TaskRemote(id); err != nil || r == nil || r.Lost || r.ID != HostPeerID {
		t.Fatalf("remote after the move %+v %v", r, err)
	}
	// b has not seen the new list yet; the message itself names a.
	if n := p.carryVia(t, newBroker, true, "dev-a2"); n != 1 {
		t.Fatalf("cancel through the new network: %d messages", n)
	}
	if mt, _ := p.b.st.TaskByID(m.ID); mt.State != "cancelled" {
		t.Fatalf("copy on b %+v", mt)
	}
	var origin string
	if err := p.b.st.db.QueryRow(`SELECT origin_broker||'/'||origin_peer||'/'||origin_computer FROM tasks WHERE id=?`, m.ID).Scan(&origin); err != nil || origin != newBroker+"/dev-a2/"+aID {
		t.Fatalf("copy's origin %q %v", origin, err)
	}
	// b's report goes back through the new network.
	if n := p.carryVia(t, newBroker, false, HostPeerID); n == 0 {
		t.Fatal("no report from b")
	}
	if task, _ := p.a.st.TaskByID(id); task.State != "cancelled" {
		t.Fatalf("task on a %+v", task)
	}
	// The same cancel arriving again through the old network changes nothing.
	p.seq++
	if err := p.b.st.ApplyNet(testBroker, NetIncoming{Seq: p.seq, From: HostPeerID, OpID: "cancel." + id, Body: []byte(`{"kind":"cancel","task":"` + id + `"}`)}); err != nil {
		t.Fatal(err)
	}
	if pending, _ := p.b.st.NetPending(testBroker, 10); len(pending) != 0 {
		t.Fatalf("a repeated cancel caused messages %+v", pending)
	}
}

// A start queued for the old network is never re-sent through the new one:
// the old network may have delivered it, and a second start would run a
// second worker.
func TestStartIsNotResentThroughANewNetwork(t *testing.T) {
	p := newNetPair(t)
	agents := []NetAgent{{Agent: "claude"}}
	bID, _ := p.b.st.ComputerID()
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Agents: agents, Computer: bID}}); err != nil {
		t.Fatal(err)
	}
	p.startRemote(t, "op-start", "claude", "work")
	const newBroker = "brk-00000000000000bb"
	if err := p.a.st.SetNetwork(newBroker, "dev-a2", []NetPeer{{ID: HostPeerID, Name: "Ноутбук", Age: 1, Agents: agents, Computer: bID}}); err != nil {
		t.Fatal(err)
	}
	if pending, _ := p.a.st.NetPending(newBroker, 10); len(pending) != 0 {
		t.Fatalf("start re-sent %+v", pending)
	}
}

// Row 30: a task the other computer has not accepted in time is reported to
// the coordinator once, with the reason, and nothing is retried.
func TestUnacceptedTaskIsReported(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "op-wait", "codex", "night audit")
	// The start never left: the server was out of reach.
	if n, err := p.a.st.NoteUnaccepted(time.Now()); err != nil || n != 0 {
		t.Fatalf("too early %d %v", n, err)
	}
	later := time.Now().Add(unacceptedAfter + time.Minute)
	if n, err := p.a.st.NoteUnaccepted(later); err != nil || n != 1 {
		t.Fatalf("noted %d %v", n, err)
	}
	if n, _ := p.a.st.NoteUnaccepted(later); n != 0 {
		t.Fatal("noted twice")
	}
	task, _ := p.a.st.TaskByID(id)
	if task.State != "running" || !strings.Contains(task.Detail, "the start has not left this computer") {
		t.Fatalf("task %+v", task)
	}
	events, _, err := p.a.st.TaskEvents("Game", AudienceCoordinator, 0, 50)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		found = found || (e.Kind == "not_accepted" && e.TaskID == id)
	}
	if !found {
		t.Fatalf("no event %+v", events)
	}
	// T-137: the window says it was not accepted instead of «Работает».
	if r, err := p.a.st.TaskRemote(id); err != nil || r == nil || !r.Unaccepted {
		t.Fatalf("remote %+v %v", r, err)
	}
	// An accepted task is never reported.
	id2 := p.startRemote(t, "op-ok", "codex", "fine")
	p.carry(t, true)
	p.carry(t, false)
	if r, err := p.a.st.TaskRemote(id2); err != nil || r == nil || r.Unaccepted {
		t.Fatalf("accepted remote %+v %v", r, err)
	}
	if n, _ := p.a.st.NoteUnaccepted(time.Now().Add(time.Hour)); n != 0 {
		t.Fatalf("accepted task %s reported", id2)
	}
}

// Row 24: when this computer's client stops refreshing the network's list,
// list_agents says so instead of showing every computer quietly offline.
func TestStaleNetworkIsExplained(t *testing.T) {
	p := newNetPair(t)
	if note, err := p.a.st.networkStale(time.Now()); err != nil || note != "" {
		t.Fatalf("fresh %q %v", note, err)
	}
	note, err := p.a.st.networkStale(time.Now().Add(10 * time.Minute))
	if err != nil || !strings.Contains(note, "has not heard from the server for 10 minutes") {
		t.Fatalf("stale %q %v", note, err)
	}
}

// Coordinator feedback 2026-11-12: a computer that reports the agent busy
// but never answered may run the task with its replies lost; the event
// says so instead of suggesting a new start.
func TestUnacceptedButBusySaysItMayRun(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "op-busy", "codex", "night audit")
	// The start left; the computer reports codex busy and never answered.
	p.carry(t, true)
	agents := []NetAgent{{Agent: "codex", Running: 1}}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Agents: agents}}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.a.st.db.Exec(`UPDATE tasks SET remote_task='' WHERE id=?`, id); err != nil {
		t.Fatal(err)
	}
	if _, err := p.a.st.db.Exec(`DELETE FROM task_events WHERE task_id=? AND kind='accepted'`, id); err != nil {
		t.Fatal(err)
	}
	if n, err := p.a.st.NoteUnaccepted(time.Now().Add(time.Hour)); err != nil || n != 1 {
		t.Fatalf("noted %d %v", n, err)
	}
	task, _ := p.a.st.TaskByID(id)
	if !strings.Contains(task.Detail, "may be running there") {
		t.Fatalf("detail %q", task.Detail)
	}
}
