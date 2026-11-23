package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestSplitMachineNoticesDeliverOnlyToTheirProject(t *testing.T) {
	f := newSplitRunFixture(t)
	ctx := context.Background()
	s, err := openSplitStore(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	peer := NetPeer{ID: "dev-b", Name: "Test computer", Computer: "dev-machine", Age: 1, Version: "0.0.0-preview.94",
		Agents: []NetAgent{{Agent: "claude"}}}
	if err := s.SetNetwork(testBroker, HostPeerID, []NetPeer{peer}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.StartTask(StartTaskRequest{Project: "Game", OpID: "game-work", Computer: peer.ID, Provider: "claude", Text: "Game work"}); err != nil {
		t.Fatal(err)
	}
	id, err := s.RequestNetUpdate(peer.ID, "Site")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(netBody{Kind: "update", Task: id, State: UpdateDone, Detail: "0.0.0-preview.95"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyNet(testBroker, NetIncoming{Seq: 1, From: peer.ID, OpID: "site-update", Body: raw}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.computer.Exec(`CREATE TRIGGER fail_notice_ack BEFORE DELETE ON project_notices
 BEGIN SELECT RAISE(ABORT,'fixture notice acknowledgement'); END`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.TaskEvents("Site", AudienceCoordinator, 0, 20); err == nil || !strings.Contains(err.Error(), "fixture notice acknowledgement") {
		t.Fatalf("expected acknowledgement failure: %v", err)
	}
	if events, _, err := s.TaskEvents("Game", AudienceCoordinator, 0, 20); err != nil || len(events) != 0 {
		t.Fatalf("Game could see the Site update: %d %v", len(events), err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.computer.Exec("DROP TRIGGER fail_notice_ack"); err != nil {
		t.Fatal(err)
	}
	s, err = openSplitStore(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	events, siteCursor, err := s.TaskEvents("Site", AudienceCoordinator, 0, 20)
	if err != nil || len(events) != 1 || events[0].Kind != MessengerUpdateEvent {
		t.Fatalf("update was not delivered once: %+v %v", events, err)
	}
	if _, err := s.NoteNetworkContact(time.Now()); err != nil {
		t.Fatal(err)
	}
	peer.Age = PeerFresh + 60
	if err := s.SetNetwork(testBroker, HostPeerID, []NetPeer{peer}); err != nil {
		t.Fatal(err)
	}
	if kind, err := s.NoteNetworkContact(time.Now()); err != nil || kind != NetworkLostEvent {
		t.Fatalf("network loss: %q %v", kind, err)
	}
	for _, project := range []string{"Game", "Site"} {
		var after int64
		if project == "Site" {
			after = siteCursor
		}
		result, _, err := s.TaskEvents(project, AudienceCoordinator, after, 20)
		if err != nil || len(result) != 1 || result[0].Kind != NetworkLostEvent {
			t.Fatalf("%s network notice: %+v %v", project, result, err)
		}
		taskCount := strings.Contains(result[0].Text, "1 open task(s)")
		if taskCount != (project == "Game") {
			t.Fatalf("%s received another project's count: %q", project, result[0].Text)
		}
	}
}
