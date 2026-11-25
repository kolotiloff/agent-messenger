package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExactUpdateCancellationCanWaitForAStalePeer(t *testing.T) {
	p := newUpdatePair(t)
	peer := NetPeer{ID: "dev-b", Name: "Notebook", Version: "1.0.1-preview.104"}
	setPeer := func() {
		t.Helper()
		if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{peer}); err != nil {
			t.Fatal(err)
		}
	}
	setPeer()
	id, err := p.a.st.RequestNetUpdate(peer.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	reqs, err := p.b.st.NetUpdateRequests(testBroker)
	if err != nil || len(reqs) != 1 {
		t.Fatalf("requests %+v: %v", reqs, err)
	}
	if err := p.b.st.RememberPendingUpdate(reqs[0]); err != nil {
		t.Fatal(err)
	}
	if err := p.b.st.ReplyNetUpdate(reqs[0], UpdateQueued, "", ""); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	peer.Age, peer.UpdatePending, peer.UpdateRequestID = PeerFresh+10, true, id
	setPeer()
	if _, err := p.a.st.RequestUpdateCancellation(peer.ID, "Game"); !errors.Is(err, ErrRejected) {
		t.Fatalf("implicit cancellation from a stale report: %v", err)
	}
	if _, err := p.a.st.CancelObservedUpdate(peer.ID, testBroker, id); err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	cancels, err := p.b.st.UpdateCancellations(testBroker)
	if err != nil || len(cancels) != 1 || cancels[0].Target != id {
		t.Fatalf("cancellations %+v: %v", cancels, err)
	}
	if err := p.b.st.FinishUpdateCancellation(cancels[0], false); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	if reply := netUpdate(t, p.a.st, peer.ID); reply.State != UpdateCancelled {
		t.Fatalf("cancelled update: %+v", reply)
	}
}

func TestCancellationCannotTouchAReplacementOrAnInstallingUpdate(t *testing.T) {
	fixture := newSplitRunFixture(t)
	st, err := OpenVerifiedProjectStorage(context.Background(), fixture.root)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	request := func() string {
		t.Helper()
		id, err := st.RequestNetUpdate("", "Game")
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	cancellation := func(project string) UpdateCancellation {
		t.Helper()
		if _, err := st.RequestUpdateCancellation("", project); err != nil {
			t.Fatal(err)
		}
		list, err := st.UpdateCancellations("")
		if err != nil || len(list) != 1 {
			t.Fatalf("cancellations %+v: %v", list, err)
		}
		return list[0]
	}
	first := request()
	delayed := cancellation("Site")
	if err := st.FinishPendingUpdate(first, UpdateDone, "0.0.0-preview.100", ""); err != nil {
		t.Fatal(err)
	}
	second := request()
	if err := st.FinishUpdateCancellation(delayed, false); err != nil {
		t.Fatal(err)
	}
	if pending, ok, err := st.PendingUpdate(); err != nil || !ok || pending.ID != second {
		t.Fatalf("replacement was touched: %+v %v %v", pending, ok, err)
	}
	busy := cancellation("Site")
	if err := st.FinishUpdateCancellation(busy, true); err != nil {
		t.Fatal(err)
	}
	if pending, ok, err := st.PendingUpdate(); err != nil || !ok || pending.ID != second {
		t.Fatalf("installation was touched: %+v %v %v", pending, ok, err)
	}
	ok := cancellation("Site")
	if err := st.FinishUpdateCancellation(ok, false); err != nil {
		t.Fatal(err)
	}
	if err := st.FinishUpdateCancellation(ok, false); err != nil {
		t.Fatal(err)
	}
	if pending, err := st.HasPendingUpdate(); err != nil || pending {
		t.Fatalf("waiting update remains: %v %v", pending, err)
	}
	events, _, err := st.TaskEvents("Site", AudienceCoordinator, 0, 20)
	if err != nil || len(events) != 3 {
		t.Fatalf("cancellation events: %+v %v", events, err)
	}
	if !strings.Contains(events[0].Text, "no longer pending") ||
		!strings.Contains(events[1].Text, "already started") ||
		!strings.Contains(events[2].Text, "was cancelled") {
		t.Fatalf("unexpected outcomes: %+v", events)
	}
}
