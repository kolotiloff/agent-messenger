package broker

import "testing"

func TestUpdateWatchClearsWhenTheExecutorReportsTheQueueEmpty(t *testing.T) {
	host, requester, executor := joined(t)
	from, _ := requester.Network()
	to, _ := executor.Network()
	const request = "upd-expired"
	b := host.broker
	status := func(want string) {
		t.Helper()
		snapshot, err := b.updateSnapshot(HostPeer)
		if err != nil || snapshot.Marks[to.Self].State != want {
			t.Fatalf("update state = %+v, want %s (%v)", snapshot.Marks[to.Self], want, err)
		}
	}
	report := func(p Presence) {
		t.Helper()
		if err := b.Report(to.Self, p); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Send(from.Self, to.Self, "update."+request, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	report(Presence{Version: "1.0.0-preview.103"})
	status("waiting")
	if err := b.Send(to.Self, from.Self, "update-reply."+request+".queued", []byte("{}")); err != nil {
		t.Fatal(err)
	}
	// A presence report made before queue admission says nothing about
	// this request until the executor has reported its exact ID.
	report(Presence{Version: "1.0.0-preview.103"})
	status("queued")
	report(Presence{Version: "1.0.0-preview.103", UpdatePending: true, UpdateRequestID: request})
	if err := b.Revoke(from.Self); err != nil {
		t.Fatal(err)
	}
	// The requester's mailbox is gone. The executor can still confirm it no
	// longer holds this queued update (for example, after its queue timeout).
	report(Presence{Version: "1.0.0-preview.103"})
	status("finished")
	// Late presence for the same request cannot make it pending again.
	report(Presence{Version: "1.0.0-preview.103", UpdatePending: true, UpdateRequestID: request})
	status("finished")
}
