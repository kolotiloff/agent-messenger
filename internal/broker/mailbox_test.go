package broker

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// joined returns a hosting service and two member services of its broker.
func joined(t *testing.T) (*Service, *Service, *Service) {
	t.Helper()
	host := newService(t, t.TempDir())
	if err := host.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	var members []*Service
	for _, name := range []string{"Ноутбук", "Студия"} {
		code, err := host.Invite(host.Status().Host.Addresses[0])
		if err != nil {
			t.Fatal(err)
		}
		m := newService(t, t.TempDir())
		if err := m.Connect(ctx(t), code.Code, name); err != nil {
			t.Fatal(err)
		}
		members = append(members, m)
	}
	return host, members[0], members[1]
}

func TestMailboxCarriesMessagesInOrder(t *testing.T) {
	host, a, b := joined(t)
	hn, _ := host.Network()
	an, _ := a.Network()
	bn, _ := b.Network()
	if hn.Self != HostPeer || an.Self == bn.Self || an.BrokerID != hn.BrokerID {
		t.Fatalf("networks %+v %+v %+v", hn, an, bn)
	}

	// Device to device, device to host and host to device.
	for i := 1; i <= 3; i++ {
		if err := a.Send(ctx(t), an, bn.Self, fmt.Sprintf("op-%d", i), []byte(fmt.Sprintf(`{"n":%d}`, i))); err != nil {
			t.Fatal(err)
		}
	}
	// The same operation again (a lost answer) is stored once.
	if err := a.Send(ctx(t), an, bn.Self, "op-2", []byte(`{"n":2}`)); err != nil {
		t.Fatal(err)
	}
	if err := a.Send(ctx(t), an, HostPeer, "to-host", []byte(`{"x":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := host.Send(ctx(t), hn, an.Self, "from-host", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}

	got, err := b.Inbox(ctx(t), bn, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].From != an.Self || got[0].OpID != "op-1" || string(got[2].Body) != `{"n":3}` {
		t.Fatalf("inbox %+v", got)
	}
	// Reading again without acknowledging returns the same messages.
	again, _ := b.Inbox(ctx(t), bn, 0)
	if len(again) != 3 {
		t.Fatalf("unacknowledged messages lost: %+v", again)
	}
	// Acknowledging the first one leaves the rest.
	rest, _ := b.Inbox(ctx(t), bn, got[0].Seq)
	if len(rest) != 2 || rest[0].OpID != "op-2" {
		t.Fatalf("after ack %+v", rest)
	}
	empty, _ := b.Inbox(ctx(t), bn, got[2].Seq)
	if len(empty) != 0 {
		t.Fatalf("after full ack %+v", empty)
	}
	if h, _ := host.Inbox(ctx(t), hn, 0); len(h) != 1 || h[0].From != an.Self || h[0].OpID != "to-host" {
		t.Fatalf("host inbox %+v", h)
	}
	if m, _ := a.Inbox(ctx(t), an, 0); len(m) != 1 || m[0].From != HostPeer {
		t.Fatalf("member inbox %+v", m)
	}
	// Nobody reads another computer's mail: a's inbox never had b's.
}

func TestMailboxRefusals(t *testing.T) {
	host, a, b := joined(t)
	an, _ := a.Network()
	bn, _ := b.Network()

	if err := a.Send(ctx(t), an, "dev-0000000000000000", "x", []byte(`{}`)); !errors.Is(err, ErrRecipient) {
		t.Fatalf("unknown recipient: %v", err)
	}
	for _, body := range []string{`[]`, `not json`, ``, `{"a":` + strings.Repeat(`"x"`, 1) + `}` + "x"} {
		if err := a.Send(ctx(t), an, bn.Self, "bad", []byte(body)); err == nil {
			t.Fatalf("bad body %q accepted", body)
		}
	}
	if err := a.Send(ctx(t), an, bn.Self, "-flag", []byte(`{}`)); err == nil {
		t.Fatal("bad op id accepted")
	}
	big := `{"t":"` + strings.Repeat("x", MaxMessageBody) + `"}`
	if err := a.Send(ctx(t), an, bn.Self, "big", []byte(big)); err == nil {
		t.Fatal("oversized message accepted")
	}

	// A network that changed in between sends nothing.
	if err := a.Send(ctx(t), Network{BrokerID: "brk-0000000000000000", Self: an.Self}, bn.Self, "x", []byte(`{}`)); !errors.Is(err, ErrNetworkChanged) {
		t.Fatalf("stale network: %v", err)
	}

	// A full mailbox refuses more until it is read.
	for i := 0; i < maxPending; i++ {
		// Several senders, each within its own share.
		sender := fmt.Sprintf("dev-%016x", i/maxPendingPerSender)
		if err := host.broker.Send(sender, bn.Self, fmt.Sprintf("fill-%d", i), []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Send(ctx(t), an, bn.Self, "one-more", []byte(`{}`)); !errors.Is(err, ErrMailboxFull) {
		t.Fatalf("full mailbox: %v", err)
	}

	// Revoking a device drops its mail and refuses new mail for it.
	if err := a.Send(ctx(t), an, HostPeer, "before-revoke", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if err := host.Revoke(bn.Self); err != nil {
		t.Fatal(err)
	}
	if err := a.Send(ctx(t), an, bn.Self, "after", []byte(`{}`)); !errors.Is(err, ErrRecipient) {
		t.Fatalf("revoked recipient: %v", err)
	}
	if _, err := b.Inbox(ctx(t), bn, 0); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked reader: %v", err)
	}
	var left int
	if err := host.broker.db.QueryRow(`SELECT COUNT(*) FROM messages WHERE recipient=?`, bn.Self).Scan(&left); err != nil || left != 0 {
		t.Fatalf("revoked mail kept: %d %v", left, err)
	}
}
