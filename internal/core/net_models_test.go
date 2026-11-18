package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestRemoteAgentModelChange(t *testing.T) {
	sender := newTaskFixture(t).st
	worker := newTaskFixture(t).st
	if err := sender.SetNetwork("broker", "sender", []NetPeer{{
		ID: "worker", Name: "Worker", Version: "0.0.0-preview.82",
		Ready:  &NetReady{Accept: true, Projects: []string{"Game"}},
		Agents: []NetAgent{{Agent: "codex", Models: DefaultCodexModels}},
	}}); err != nil {
		t.Fatal(err)
	}
	want := []string{"gpt-6.2-sol[effort=high]"}
	for i, state := range []string{"applied", "refused"} {
		id, err := sender.RequestAgentModels("Game", "worker", "codex", want)
		if err != nil || id == "" {
			t.Fatalf("queue: %q %v", id, err)
		}
		if _, err := sender.RequestAgentModels("Game", "worker", "codex", want); err == nil {
			t.Fatal("second outstanding request accepted")
		}
		requests, err := sender.NetPending("broker", 10)
		if err != nil || len(requests) != 1 {
			t.Fatalf("outbox: %v %v", requests, err)
		}
		request := requests[0]
		if err := sender.NetSent(request.Seq); err != nil {
			t.Fatal(err)
		}
		// The second request uses a stale roster and must preserve the new list.
		models := want
		if i != 0 {
			models = []string{"gpt-6.2-astra[effort=medium]"}
			if err := worker.SetAgentModels("codex", models); err != nil {
				t.Fatal(err)
			}
		}
		incoming := NetIncoming{Seq: int64(i + 1), From: "sender", OpID: request.OpID, Body: request.Body}
		for n := 0; n < 2; n++ {
			if err := worker.ApplyNet("broker", incoming); err != nil {
				t.Fatal(err)
			}
		}
		agents, err := worker.ComputerAgents()
		if err != nil {
			t.Fatal(err)
		}
		for _, agent := range agents {
			if agent.Provider == "codex" && !reflect.DeepEqual(agent.Models, models) {
				t.Fatalf("models %v want %v", agent.Models, models)
			}
		}
		replies, err := worker.NetPending("broker", 10)
		if err != nil || len(replies) != 1 || replies[0].To != "sender" {
			t.Fatalf("reply: %v %v", replies, err)
		}
		reply := replies[0]
		if err := worker.NetSent(reply.Seq); err != nil {
			t.Fatal(err)
		}
		// Only the selected worker can confirm the command.
		if err := sender.ApplyNet("broker", NetIncoming{Seq: int64(2*i + 1), From: "impostor", OpID: "fake." + reply.OpID, Body: reply.Body}); err != nil {
			t.Fatal(err)
		}
		if err := sender.ApplyNet("broker", NetIncoming{Seq: int64(2*i + 2), From: "worker", OpID: reply.OpID, Body: reply.Body}); err != nil {
			t.Fatal(err)
		}
		events, _, err := sender.TaskEvents("Game", AudienceCoordinator, 0, 20)
		if err != nil || len(events) != i+1 || events[i].Kind != agentModelsEvent ||
			!strings.Contains(events[i].Text, state) || !strings.Contains(events[i].Text, id) {
			t.Fatalf("confirmation: %+v %v", events, err)
		}
	}
}

func TestCodexDefaultsUpgradeFrom81(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.SetAgentModels("codex", splitModels(previousCodexDefaults)); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.db.Exec(`DELETE FROM meta WHERE key='default_models_t195'`); err != nil {
		t.Fatal(err)
	}
	if err := refreshDefaultModels(f.st.db); err != nil {
		t.Fatal(err)
	}
	var got string
	if err := f.st.db.QueryRow(`SELECT models FROM computer_agents WHERE provider='codex'`).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != strings.Join(DefaultCodexModels, "\n") {
		t.Fatalf("old list survived: %q", got)
	}
}
