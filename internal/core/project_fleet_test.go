package core

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestSplitFleetWaitsForWorkInEveryProject(t *testing.T) {
	f := newSplitRunFixture(t)
	s, err := openSplitStore(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	const remote = "peer-machine"
	if err := s.SetNetwork(testBroker, HostPeerID, []NetPeer{{
		ID: remote, Name: "Remote", Age: 1, Version: "0.0.0-preview.94", Agents: []NetAgent{{Agent: "claude"}},
	}}); err != nil {
		t.Fatal(err)
	}
	start := func(project, computer, op string) string {
		t.Helper()
		id, _, err := s.StartTask(StartTaskRequest{Project: project, Computer: computer, OpID: op, Provider: "claude", Text: project + " " + op})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	game := start("Game", "", "local")
	site := start("Site", "", "local")
	remoteTask := start("Site", remote, "remote")
	if out, err := s.OutboxBacklog(); err != nil || out.Waiting != 1 {
		t.Fatalf("project outbox backlog: %+v %v", out, err)
	}
	if list, err := s.SharedTasks(time.Now().Add(-time.Hour)); err != nil || len(list) != 3 {
		t.Fatalf("shared work: %+v %v", list, err)
	}
	if count, err := s.NoteUnaccepted(time.Now().Add(unacceptedAfter + time.Minute)); err != nil || count != 1 {
		t.Fatalf("unconfirmed remote task notice: %d %v", count, err)
	}
	report, err := s.ProjectAgents("Game")
	if err != nil {
		t.Fatal(err)
	}
	for _, agent := range report["computers"].([]ComputerSummary)[0].Agents {
		if agent.Agent == "claude" && agent.Running != 1 {
			t.Fatalf("Game reports the other local task: %+v", agent)
		}
	}
	if count, err := s.LiveWork(); err != nil || count != 3 {
		t.Fatalf("live work on both projects: %d %v", count, err)
	}
	if _, err := s.RequestNetUpdate("", "Game"); err != nil {
		t.Fatal(err)
	}
	events, _, err := s.TaskEvents("Game", AudienceCoordinator, 0, 40)
	if err != nil || len(events) != 1 || events[0].Kind != MessengerUpdateEvent || strings.Contains(events[0].Text, "Site") {
		t.Fatalf("update notice exposed work outside the project: %+v %v", events, err)
	}
	if _, err := s.CancelTask(game); err != nil {
		t.Fatal(err)
	}
	if n, err := s.LiveWork(); err != nil || n != 2 {
		t.Fatalf("update forgot work in Site: %d %v", n, err)
	}
	for _, id := range []string{site, remoteTask} {
		if _, err := s.CancelTask(id); err != nil {
			t.Fatal(err)
		}
	}
	if n, err := s.LiveWork(); err != nil || n != 0 {
		t.Fatalf("completed queue holds the update: %d %v", n, err)
	}
}
