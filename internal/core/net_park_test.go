package core

import (
	"strings"
	"testing"
)

// T-146: a report for a computer the server does not know right now is
// kept, not dropped, and goes when that computer is listed again, under
// the same id or (D-115) a new one.
func TestReportForAMissingComputerIsKept(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	aID, err := p.a.st.ComputerID()
	if err != nil {
		t.Fatal(err)
	}
	agents := []NetAgent{{Agent: "codex"}}
	id := p.startRemote(t, "op", "codex", "Report me")
	p.carry(t, true)
	m := p.mirror(t, id)
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000d1")
	p.b.dispose(t, r, DispositionReport, "all done")
	if _, err := p.b.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}

	// The server says the giver is not there: every reply is parked.
	pending, _ := p.b.st.NetPending(testBroker, 100)
	if len(pending) == 0 {
		t.Fatal("no reply queued")
	}
	for _, msg := range pending {
		if err := p.b.st.NetUndeliverable(msg.Seq); err != nil {
			t.Fatal(err)
		}
	}
	if left, _ := p.b.st.NetPending(testBroker, 100); len(left) != 0 {
		t.Fatalf("parked replies still sent %d", len(left))
	}
	var parked int
	if err := p.b.st.db.QueryRow(`SELECT COUNT(*) FROM net_outbox WHERE broker=?`, parkedPrefix+testBroker).Scan(&parked); err != nil || parked != len(pending) {
		t.Fatalf("parked %d of %d (%v)", parked, len(pending), err)
	}

	// The giver is listed again under the same id: the replies go.
	if err := p.b.st.SetNetwork(testBroker, "dev-b", []NetPeer{{ID: HostPeerID, Name: "Студия", Host: true, Age: 1, Agents: agents, Computer: aID}}); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	task, _ := p.a.st.TaskByID(id)
	if task.State != "done" {
		t.Fatalf("giver task %s", task.State)
	}
	got := false
	for _, e := range p.a.events(t, AudienceCoordinator, 0) {
		got = got || (e.TaskID == id && e.Kind == "report" && strings.Contains(e.Text, "all done"))
	}
	if !got {
		t.Fatalf("report not delivered: %s", kinds(p.a.events(t, AudienceCoordinator, 0)))
	}
}

// T-146 with D-115: parked replies follow the giver to its new id.
func TestParkedReportFollowsANewID(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	aID, _ := p.a.st.ComputerID()
	agents := []NetAgent{{Agent: "codex"}}
	if err := p.b.st.SetNetwork(testBroker, "dev-b", []NetPeer{{ID: HostPeerID, Name: "Студия", Host: true, Age: 1, Agents: agents, Computer: aID}}); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op", "codex", "Report me")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	r := p.b.launch(t, m.ID, "019a0000-0000-7000-8000-0000000000d2")
	p.b.dispose(t, r, DispositionReport, "done anyway")
	if _, err := p.b.st.FinishTaskRun(r.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	pending, _ := p.b.st.NetPending(testBroker, 100)
	for _, msg := range pending {
		if err := p.b.st.NetUndeliverable(msg.Seq); err != nil {
			t.Fatal(err)
		}
	}
	// The giver shows up under a new id in the same network.
	if err := p.b.st.SetNetwork(testBroker, "dev-b", []NetPeer{{ID: "host-2", Name: "Студия", Host: true, Age: 1, Agents: agents, Computer: aID}}); err != nil {
		t.Fatal(err)
	}
	var moved int
	if err := p.b.st.db.QueryRow(`SELECT COUNT(*) FROM net_outbox WHERE broker=? AND recipient='host-2'`, testBroker).Scan(&moved); err != nil || moved != len(pending) {
		t.Fatalf("moved %d of %d (%v)", moved, len(pending), err)
	}
}
