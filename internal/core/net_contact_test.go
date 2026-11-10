package core

import "testing"

// T-126: a task sent to a computer this computer has lost contact with says
// so, and keeps that computer's name after the network is made again.
func TestRemoteTaskShowsLostContact(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "op-lost", "claude", "work")
	remote := func() *RemoteComputer {
		t.Helper()
		r, err := p.a.st.TaskRemote(id)
		if err != nil || r == nil {
			t.Fatalf("remote %v %v", r, err)
		}
		return r
	}
	if r := remote(); r.Lost || r.Name != "Ноутбук" || r.LastSeen == "" {
		t.Fatalf("fresh computer: %+v", r)
	}
	agents := []NetAgent{{Agent: "claude"}}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: PeerFresh + 30, Agents: agents}}); err != nil {
		t.Fatal(err)
	}
	if r := remote(); !r.Lost || r.LastSeen == "" {
		t.Fatalf("offline computer: %+v", r)
	}
	// The network is made again: the old computer ID is gone.
	if err := p.a.st.SetNetwork("brk-00000000000000bb", HostPeerID, []NetPeer{{ID: "dev-c", Name: "Ноутбук", Age: 1, Agents: agents}}); err != nil {
		t.Fatal(err)
	}
	if r := remote(); !r.Lost || r.Name != "Ноутбук" || r.LastSeen != "" {
		t.Fatalf("after a new network: %+v", r)
	}
	// A task of this computer only has no remote.
	local, _, err := p.a.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-here", Provider: "claude", Text: "here"})
	if err != nil {
		t.Fatal(err)
	}
	if r, err := p.a.st.TaskRemote(local); err != nil || r != nil {
		t.Fatalf("local task: %+v %v", r, err)
	}
}

// T-131: a task on a computer out of reach can be closed here at once; a
// cancel that got no answer can be closed after a while; what that computer
// reports later does not reopen it.
func TestRemoteTaskClosesHereWhenOutOfReach(t *testing.T) {
	p := newNetPair(t)
	agents := []NetAgent{{Agent: "claude"}}
	id := p.startRemote(t, "op-a", "claude", "work")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: PeerFresh + 30, Agents: agents}}); err != nil {
		t.Fatal(err)
	}
	if state, err := p.a.st.CancelTask(id); err != nil || state != "cancelled" {
		t.Fatalf("close out of reach: %s %v", state, err)
	}
	// The cancel still goes out for when that computer is back.
	if sent := p.carry(t, true); len(sent) != 1 {
		t.Fatalf("cancel messages %+v", sent)
	}
	// b cancels its copy and reports it; a stays as the owner closed it.
	if mt, _ := p.b.st.TaskByID(m.ID); mt.State != "cancelled" {
		t.Fatalf("copy on b %+v", mt)
	}
	p.carry(t, false)
	if task, _ := p.a.st.TaskByID(id); task.State != "cancelled" {
		t.Fatalf("reopened %+v", task)
	}

	// In contact: the first stop asks; a second one right away does not
	// close it; after the wait without an answer it does.
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Agents: agents}}); err != nil {
		t.Fatal(err)
	}
	id2 := p.startRemote(t, "op-b", "claude", "more work")
	p.carry(t, true)
	p.carry(t, false)
	if state, _ := p.a.st.CancelTask(id2); state != "cancelling" {
		t.Fatalf("first stop %s", state)
	}
	if state, _ := p.a.st.CancelTask(id2); state != "cancelling" {
		t.Fatalf("second stop at once %s", state)
	}
	old := cancelAnswerWait
	cancelAnswerWait = 0
	defer func() { cancelAnswerWait = old }()
	if state, _ := p.a.st.CancelTask(id2); state != "cancelled" {
		t.Fatalf("stop after no answer %s", state)
	}
}
