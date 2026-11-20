package core

import (
	"encoding/json"
	"testing"
)

func TestAgentVersionMailbox(t *testing.T) {
	p := newNetPair(t)
	id, err := p.a.st.RequestAgentVersion("dev-b", "codex")
	if err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	req, err := p.b.st.ClaimAgentVersionRequest(testBroker)
	if err != nil || req.ID != id || req.Agent != "codex" {
		t.Fatalf("claim %+v: %v", req, err)
	}
	if again, err := p.b.st.ClaimAgentVersionRequest(testBroker); err != nil || again.ID != "" {
		t.Fatalf("claimed again: %+v %v", again, err)
	}
	foreign, err := json.Marshal(netBody{Kind: "agent_version", Task: id, Agent: "codex", Text: "9.9.9", CheckedAt: now()})
	if err != nil {
		t.Fatal(err)
	}
	if err := p.a.st.ApplyNet(testBroker, NetIncoming{Seq: 900, From: "dev-z", OpID: "foreign-version", Body: foreign}); err != nil {
		t.Fatal(err)
	}
	if v, err := p.a.st.AgentVersion(id); err != nil || v.State != "waiting" || v.Version != "" {
		t.Fatalf("foreign reply applied: %+v %v", v, err)
	}
	// No free-form output may be returned, even if another client sends it.
	if err := p.b.st.ReplyAgentVersion(req, "0.155.1\nraw output", ""); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	if v, err := p.a.st.AgentVersion(id); err != nil || v.State != "unknown" || v.Version != "" || v.Reason != "unrecognized_version_output" {
		t.Fatalf("unvalidated reply: %+v %v", v, err)
	}
}
