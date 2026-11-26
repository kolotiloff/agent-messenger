package core

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestSplitWindowKeepsHistoryAndActivityInItsProject(t *testing.T) {
	f := newSplitRunFixture(t)
	s, err := openSplitStore(context.Background(), f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	add := func(project, op string, closed bool) {
		t.Helper()
		id, _, err := s.StartTask(StartTaskRequest{Project: project, OpID: op, Provider: "claude", Text: project + " " + op})
		if err != nil {
			t.Fatal(err)
		}
		if closed {
			if _, err := s.CancelTask(id); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := 0; i < 12; i++ {
		add("Game", fmt.Sprintf("old-%d", i), true)
	}
	add("Game", "active-1", false)
	add("Game", "active-2", false)
	add("Site", "finished", true)
	add("Site", "active", false)
	self, err := platform.ReadObservation(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.TouchCoordinatorClient("Site", "codex"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ClaimCoordinator("Site", "fixture-chat", self); err != nil {
		t.Fatal(err)
	}
	activity, err := s.ProjectActivity("Site")
	if err != nil || !activity.Coordinator.Active || activity.Coordinator.Client != "codex" {
		t.Fatalf("Site activity coordinator = %+v: %v", activity.Coordinator, err)
	}

	game, err := s.WindowView("Game", "")
	if err != nil || !game.HistoryPaged || game.Pending != 2 || len(game.Tasks) != 12 || game.HistoryNext == "" {
		t.Fatalf("Game window: pending %d, tasks %d, next page %t: %v", game.Pending, len(game.Tasks), game.HistoryNext != "", err)
	}
	more, err := s.WindowTaskHistory("Game", game.HistoryNext, "", "")
	if err != nil || len(more.Tasks) != 2 || more.Next != "" {
		t.Fatalf("Game next page: %+v %v", more, err)
	}
	site, err := s.WindowView("Site", "")
	if err != nil || site.Pending != 1 || len(site.Tasks) != 2 || site.HistoryNext != "" {
		t.Fatalf("Site window: pending %d, tasks %d: %v", site.Pending, len(site.Tasks), err)
	}
	for _, a := range site.Agents {
		if a.Agent == "claude" && (a.ProjectRunning != 1 || a.Running != 1) {
			t.Fatalf("Site agent status includes Game: %+v", a)
		}
	}
	if err := s.HideWindowHistory("Game", "", ""); err != nil {
		t.Fatal(err)
	}
	site, err = s.WindowView("Site", "")
	if err != nil || len(site.Tasks) != 2 {
		t.Fatalf("clearing Game altered Site: %d %v", len(site.Tasks), err)
	}
}
