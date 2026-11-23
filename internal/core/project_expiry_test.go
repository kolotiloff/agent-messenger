package core

import (
	"context"
	"testing"
	"time"
)

func TestSplitExpiresOnlyMessagesToAbsentComputers(t *testing.T) {
	p := newNetPair(t)
	if err := p.a.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	start := func(project, op string) string {
		t.Helper()
		id, _, err := p.a.st.StartTask(StartTaskRequest{Project: project, OpID: op, Computer: "dev-b", Provider: "codex", Text: "Build"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	game, site := start("Game", "old-start"), start("Site", "young-start")
	old := time.Now().UTC().Add(-reconnectGrace - time.Minute).Format(time.RFC3339Nano)
	if _, err := p.a.st.db.Exec(`UPDATE net_outbox SET created_at=? WHERE task_id=?`, old, game); err != nil {
		t.Fatal(err)
	}
	if _, err := p.a.st.db.Exec(`UPDATE net_peers SET missing_since=? WHERE peer='dev-b'`, old); err != nil {
		t.Fatal(err)
	}
	if err := ActivateProjectStorage(context.Background(), p.a.path); err != nil {
		t.Fatal(err)
	}
	st, err := Open(p.a.path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	n, recipients, err := st.ReleaseAbsentOutbox(time.Now())
	if err != nil || n != 1 || len(recipients) != 1 || recipients[0] != "dev-b" {
		t.Fatalf("released %d %v %v", n, recipients, err)
	}
	pending, err := st.NetPending(testBroker, 10)
	if err != nil || len(pending) != 1 || pending[0].OpID != "start."+site {
		t.Fatalf("remaining messages: %+v %v", pending, err)
	}
	rows, err := st.Tasks("Game", 10)
	if err != nil || len(rows) != 1 || rows[0].ID != game || rows[0].State != "failed" {
		t.Fatalf("expired task: %+v %v", rows, err)
	}
}
