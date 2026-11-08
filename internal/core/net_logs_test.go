package core

import (
	"errors"
	"strings"
	"testing"
)

func TestNetLogRequestAndReply(t *testing.T) {
	p := newNetPair(t)
	if on, err := p.b.st.NetLogShare(); err != nil || on {
		t.Fatalf("log sharing must start off: %v %v", on, err)
	}
	if _, err := p.a.st.RequestNetLog("dev-x", 50); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown computer: %v", err)
	}
	id, err := p.a.st.RequestNetLog("dev-b", 50)
	if err != nil {
		t.Fatal(err)
	}
	if l, err := p.a.st.NetLog(id); err != nil || l.State != "waiting" || l.Computer != "Ноутбук" {
		t.Fatalf("waiting %+v %v", l, err)
	}
	p.carry(t, true)
	// The same message read twice records one request.
	reqs, err := p.b.st.NetLogRequests(testBroker)
	if err != nil || len(reqs) != 1 || reqs[0].ID != id || reqs[0].Lines != 50 || reqs[0].Peer != HostPeerID {
		t.Fatalf("requests %+v %v", reqs, err)
	}
	if err := p.b.st.ReplyNetLog(reqs[0], "строка 1\nстрока 2", ""); err != nil {
		t.Fatal(err)
	}
	if reqs, _ := p.b.st.NetLogRequests(testBroker); len(reqs) != 0 {
		t.Fatalf("answered request stays %+v", reqs)
	}
	p.carry(t, false)
	l, err := p.a.st.NetLog(id)
	if err != nil || l.State != "done" || l.Text != "строка 1\nстрока 2" {
		t.Fatalf("done %+v %v", l, err)
	}

	// A refusal comes back with its reason.
	id2, _ := p.a.st.RequestNetLog("dev-b", 0)
	p.carry(t, true)
	reqs, _ = p.b.st.NetLogRequests(testBroker)
	if len(reqs) != 1 || reqs[0].Lines != DefaultLogLines {
		t.Fatalf("default lines %+v", reqs)
	}
	_ = p.b.st.ReplyNetLog(reqs[0], "", "выключено")
	p.carry(t, false)
	if l, _ := p.a.st.NetLog(id2); l.State != "refused" || l.Text != "выключено" {
		t.Fatalf("refused %+v", l)
	}

	// A reply from another computer than the one asked changes nothing.
	id3, _ := p.a.st.RequestNetLog("dev-b", 10)
	_ = p.a.st.ApplyNet(testBroker, NetIncoming{Seq: 900, From: "dev-z", OpID: "log-reply." + id3, Body: []byte(`{"kind":"log","task":"` + id3 + `","text":"чужое"}`)})
	if l, _ := p.a.st.NetLog(id3); l.State != "waiting" {
		t.Fatalf("foreign reply accepted %+v", l)
	}

	if err := p.b.st.SetNetLogShare(true); err != nil {
		t.Fatal(err)
	}
	if on, _ := p.b.st.NetLogShare(); !on {
		t.Fatal("switch not kept")
	}
}

func TestNetLogUndeliverable(t *testing.T) {
	p := newNetPair(t)
	id, _ := p.a.st.RequestNetLog("dev-b", 10)
	pending, _ := p.a.st.NetPending(testBroker, 10)
	if len(pending) != 1 {
		t.Fatalf("pending %+v", pending)
	}
	if err := p.a.st.NetUndeliverable(pending[0].Seq); err != nil {
		t.Fatal(err)
	}
	if l, _ := p.a.st.NetLog(id); l.State != "refused" || !strings.Contains(l.Text, "no longer in the network") {
		t.Fatalf("undeliverable %+v", l)
	}
}
