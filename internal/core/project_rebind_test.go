package core

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSplitReceiverRepairsPeerRouteOnRedelivery(t *testing.T) {
	f := newSplitRunFixture(t)
	ctx := context.Background()
	s, err := openSplitStore(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	message := func(broker, from, op string, seq int64, body netBody) error {
		t.Helper()
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		return s.ApplyNet(broker, NetIncoming{Seq: seq, From: from, OpID: op, Body: raw})
	}
	for _, name := range []string{"Game", "Site"} {
		id := "origin-" + name
		computer := "machine-" + name
		if err := message(testBroker, "old-"+name, "start-"+name, 1, netBody{
			Kind: "start", Task: id, Computer: computer, Project: name, Agent: "claude", Text: name + " work",
		}); err != nil {
			t.Fatal(err)
		}
	}
	const newBroker = "brk-00000000000000bb"
	note := netBody{Kind: "note", Task: "origin-Game", Computer: "wrong-computer", OpID: "note-one", Text: "Read this in Game"}
	// D-188: the message is skipped; the route stays with the old peer.
	if err := message(newBroker, "new-game", "wrong-note", 1, note); err != nil {
		t.Fatalf("an unrouted note stopped the inbox: %v", err)
	}
	var kept string
	if err := f.computer.QueryRow("SELECT origin_peer FROM task_storage_routes WHERE project='Game'").Scan(&kept); err != nil || kept != "old-Game" {
		t.Fatalf("unrelated computer rebound the task: %q %v", kept, err)
	}
	if _, err := f.computer.Exec(`CREATE TRIGGER fail_rebind_cursor BEFORE UPDATE ON meta
 WHEN NEW.key='net_cursor:brk-00000000000000bb' BEGIN SELECT RAISE(ABORT,'fixture rebind cursor failure'); END`); err != nil {
		t.Fatal(err)
	}
	note.Computer = "machine-Game"
	if err := message(newBroker, "new-game", "rebound-note", 2, note); err == nil || !strings.Contains(err.Error(), "fixture rebind cursor failure") {
		t.Fatalf("rebind interruption: %v", err)
	}
	var peer string
	game, site := f.projects[projectMigrationFile("Game")], f.projects[projectMigrationFile("Site")]
	if err := f.computer.QueryRow("SELECT origin_peer FROM task_storage_routes WHERE project='Game'").Scan(&peer); err != nil || peer != "old-Game" {
		t.Fatalf("route changed before cursor commit: %q %v", peer, err)
	}
	if err := game.QueryRow("SELECT origin_peer FROM tasks").Scan(&peer); err != nil || peer != "new-game" {
		t.Fatalf("project change did not commit: %q %v", peer, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := f.computer.Exec("DROP TRIGGER fail_rebind_cursor"); err != nil {
		t.Fatal(err)
	}
	s, err = openSplitStore(ctx, f.root)
	if err != nil {
		t.Fatal(err)
	}
	if err := message(newBroker, "new-game", "rebound-note", 2, note); err != nil {
		t.Fatal(err)
	}
	if err := f.computer.QueryRow("SELECT origin_peer FROM task_storage_routes WHERE project='Game'").Scan(&peer); err != nil || peer != "new-game" {
		t.Fatalf("redelivery did not repair route: %q %v", peer, err)
	}
	if err := f.computer.QueryRow("SELECT recipient FROM net_outbox WHERE kind='accepted' AND task_id='origin-Game'").Scan(&peer); err != nil || peer != "new-game" {
		t.Fatalf("start acceptance stayed in the old mailbox: %q %v", peer, err)
	}
	if err := site.QueryRow("SELECT origin_peer FROM tasks").Scan(&peer); err != nil || peer != "old-Site" {
		t.Fatalf("unrelated route changed: %q %v", peer, err)
	}
	var notes int
	if err := game.QueryRow("SELECT COUNT(*) FROM task_owner_notes").Scan(&notes); err != nil || notes != 1 {
		t.Fatalf("note was replayed more than once: %d %v", notes, err)
	}
	if cursor, err := s.NetCursor(newBroker); err != nil || cursor != 2 {
		t.Fatalf("network cursor: %d %v", cursor, err)
	}
}
