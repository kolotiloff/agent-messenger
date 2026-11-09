package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func netUpdate(t *testing.T, st *Store, peer string) NetUpdate {
	t.Helper()
	list, err := st.NetUpdates()
	if err != nil {
		t.Fatal(err)
	}
	for _, u := range list {
		if u.Computer == peer {
			return u
		}
	}
	return NetUpdate{}
}

// newUpdatePair is a network whose other computer reports its version.
func newUpdatePair(t *testing.T) *netPair {
	t.Helper()
	p := newNetPair(t)
	// An older computer reports no version and is not asked.
	if _, err := p.a.st.RequestNetUpdate("dev-b"); !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "вручную") {
		t.Fatalf("computer without a version: %v", err)
	}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Version: "0.0.0-preview.56"}}); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestNetUpdateRequestAndReplies(t *testing.T) {
	p := newUpdatePair(t)
	if _, err := p.a.st.RequestNetUpdate("dev-x"); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown computer: %v", err)
	}
	id, err := p.a.st.RequestNetUpdate("dev-b")
	if err != nil || !netRef.MatchString(id) {
		t.Fatalf("request %q %v", id, err)
	}
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateWaiting || u.ID != id || u.Stale {
		t.Fatalf("waiting %+v", u)
	}
	// A second request waits for the answer to the first.
	if _, err := p.a.st.RequestNetUpdate("dev-b"); !errors.Is(err, ErrRejected) {
		t.Fatalf("duplicate request: %v", err)
	}
	p.carry(t, true)
	reqs, err := p.b.st.NetUpdateRequests(testBroker)
	if err != nil || len(reqs) != 1 || reqs[0].ID != id || reqs[0].Peer != HostPeerID {
		t.Fatalf("requests %+v %v", reqs, err)
	}
	if err := p.b.st.ReplyNetUpdate(reqs[0], "bogus", "", ""); err == nil {
		t.Fatal("unknown state accepted")
	}
	if err := p.b.st.ReplyNetUpdate(reqs[0], UpdateUpdating, "", ""); err != nil {
		t.Fatal(err)
	}
	if reqs, _ := p.b.st.NetUpdateRequests(testBroker); len(reqs) != 0 {
		t.Fatalf("answered request stays %+v", reqs)
	}
	// The final result goes after the first answer.
	if err := p.b.st.FinishNetUpdate(testBroker, HostPeerID, id, UpdateDone, "0.0.0-preview.57", ""); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateDone || u.Version != "0.0.0-preview.57" {
		t.Fatalf("done %+v", u)
	}

	// A final answer is not replaced, and a late "updating" changes nothing.
	_ = p.a.st.ApplyNet(testBroker, NetIncoming{Seq: 900, From: "dev-b", OpID: "late." + id, Body: []byte(`{"kind":"update","task":"` + id + `","state":"updating"}`)})
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateDone {
		t.Fatalf("final state replaced %+v", u)
	}

	// A refusal comes back with its reason; a new request may follow.
	id2, err := p.a.st.RequestNetUpdate("dev-b")
	if err != nil || id2 == id {
		t.Fatalf("second request %q %v", id2, err)
	}
	p.carry(t, true)
	reqs, _ = p.b.st.NetUpdateRequests(testBroker)
	if err := p.b.st.ReplyNetUpdate(reqs[0], UpdateRefused, "", "идут задачи"); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateRefused || u.Text != "идут задачи" || u.ID != id2 {
		t.Fatalf("refused %+v", u)
	}

	// Replies from another computer, for an old request or with an unknown
	// state change nothing.
	id3, _ := p.a.st.RequestNetUpdate("dev-b")
	for i, m := range []NetIncoming{
		{From: "dev-z", Body: []byte(`{"kind":"update","task":"` + id3 + `","state":"done"}`)},
		{From: "dev-b", Body: []byte(`{"kind":"update","task":"` + id + `","state":"failed"}`)},
		{From: "dev-b", Body: []byte(`{"kind":"update","task":"` + id3 + `","state":"installed"}`)},
	} {
		m.Seq, m.OpID = int64(1000+i), "x."+id3+"."+string(rune('a'+i))
		if err := p.a.st.ApplyNet(testBroker, m); err != nil {
			t.Fatal(err)
		}
	}
	if u := netUpdate(t, p.a.st, "dev-b"); u.State != UpdateWaiting || u.ID != id3 {
		t.Fatalf("foreign reply accepted %+v", u)
	}

	// An old unanswered request is shown as stale and may be asked again.
	if _, err := p.a.st.db.Exec(`UPDATE net_updates SET created_at=? WHERE id=?`, time.Now().UTC().Add(-UpdateWait-time.Minute).Format(time.RFC3339Nano), id3); err != nil {
		t.Fatal(err)
	}
	if u := netUpdate(t, p.a.st, "dev-b"); !u.Stale {
		t.Fatalf("not stale %+v", u)
	}
	if _, err := p.a.st.RequestNetUpdate("dev-b"); err != nil {
		t.Fatalf("request after a stale one: %v", err)
	}
}

func TestNetUpdateRefusals(t *testing.T) {
	p := newUpdatePair(t)
	// A computer that is not on line is not asked.
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: PeerFresh + 10, Version: "0.0.0-preview.56"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.a.st.RequestNetUpdate("dev-b"); !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "не на связи") {
		t.Fatalf("offline computer: %v", err)
	}
	if err := p.a.st.SetNetwork("", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := p.a.st.RequestNetUpdate("dev-b"); !errors.Is(err, ErrRejected) {
		t.Fatalf("no network: %v", err)
	}
	// A final result with a bad id or state is refused; one for another
	// network is dropped.
	for _, c := range [][3]string{{"bad id!", UpdateDone, ""}, {"upd-1", UpdateUpdating, ""}, {"upd-1", "installed", ""}} {
		if err := p.b.st.FinishNetUpdate(testBroker, HostPeerID, c[0], c[1], "", ""); !errors.Is(err, ErrRejected) {
			t.Errorf("%v accepted: %v", c, err)
		}
	}
	if err := p.b.st.FinishNetUpdate("other-broker", HostPeerID, "upd-1", UpdateDone, "", ""); err != nil {
		t.Fatal(err)
	}
	if pending, _ := p.b.st.NetPending("other-broker", 10); len(pending) != 0 {
		t.Fatalf("result for another network queued %+v", pending)
	}
}

func TestNetUpdateUndeliverable(t *testing.T) {
	p := newUpdatePair(t)
	id, _ := p.a.st.RequestNetUpdate("dev-b")
	pending, _ := p.a.st.NetPending(testBroker, 10)
	if len(pending) != 1 {
		t.Fatalf("pending %+v", pending)
	}
	if err := p.a.st.NetUndeliverable(pending[0].Seq); err != nil {
		t.Fatal(err)
	}
	if u := netUpdate(t, p.a.st, "dev-b"); u.ID != id || u.State != UpdateFailed {
		t.Fatalf("undeliverable %+v", u)
	}
}

func TestActiveWork(t *testing.T) {
	f := newTaskFixture(t)
	if n, err := f.st.ActiveWork(); err != nil || n != 0 {
		t.Fatalf("empty %d %v", n, err)
	}
	id := f.start(t, "op-1", "claude", "работа")
	if n, _ := f.st.ActiveWork(); n != 1 {
		t.Fatalf("running task not counted: %d", n)
	}
	if _, err := f.st.CancelTask(id); err != nil {
		t.Fatal(err)
	}
	if n, _ := f.st.ActiveWork(); n != 0 {
		t.Fatalf("cancelled task counted: %d", n)
	}
}

func TestNetPeerVersion(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Version: "0.0.0-preview.56"}, {ID: "dev-c", Name: "Старый", Age: 1}}); err != nil {
		t.Fatal(err)
	}
	peers, err := f.st.NetPeers()
	if err != nil || len(peers) != 2 || peers[0].Version != "0.0.0-preview.56" || peers[1].Version != "" {
		t.Fatalf("peers %+v %v", peers, err)
	}
}
