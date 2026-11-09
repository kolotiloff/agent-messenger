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
