package core

import (
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
)

// T-158: list_agents gives the Messenger version of this computer and of
// another computer that reported it.
func TestProjectAgentsGiveVersions(t *testing.T) {
	p := newNetPair(t)
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Version: "0.0.0-preview.67"}, {ID: "dev-c", Name: "Старый", Age: 1}}); err != nil {
		t.Fatal(err)
	}
	list, err := p.a.st.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, c := range list["computers"].([]ComputerSummary) {
		got[c.Name] = c.Version
		if c.This && c.Version != buildinfo.Version {
			t.Fatalf("this computer %q", c.Version)
		}
	}
	if got["Ноутбук"] != "0.0.0-preview.67" || got["Старый"] != "" {
		t.Fatalf("versions %v", got)
	}
}
