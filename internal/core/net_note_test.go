package core

import (
	"errors"
	"strings"
	"testing"
)

// T-149: the coordinator writes to a worker on another computer; that
// computer keeps the note for its worker, or says why not.
func TestCoordinatorNoteReachesAnotherComputer(t *testing.T) {
	p := newNetPair(t)
	if err := p.b.st.SetNetAccept(true); err != nil {
		t.Fatal(err)
	}
	id := p.startRemote(t, "op", "codex", "Long work")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)

	// A computer before 0.67 would drop the message: refused here.
	err := p.a.st.AddCoordinatorNote("Game", id, "note-1", "Stop and report what is done")
	if !errors.Is(err, ErrRejected) || !strings.Contains(err.Error(), "older Messenger") {
		t.Fatalf("old version: %v", err)
	}
	agents := []NetAgent{{Agent: "codex"}}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Agents: agents, Version: "0.0.0-preview.67"}}); err != nil {
		t.Fatal(err)
	}
	if err := p.a.st.AddCoordinatorNote("Game", id, "note-1", "Stop and report what is done"); err != nil {
		t.Fatal(err)
	}
	// The same call again is safe; another text with the same id is not.
	if err := p.a.st.AddCoordinatorNote("Game", id, "note-1", "Stop and report what is done"); err != nil {
		t.Fatalf("repeat: %v", err)
	}
	if err := p.a.st.AddCoordinatorNote("Game", id, "note-1", "Other"); !errors.Is(err, ErrRejected) {
		t.Fatalf("reused id: %v", err)
	}
	if got := noteStates(t, p.a.st, id); got != "sent" {
		t.Fatalf("giver note %s", got)
	}
	p.carry(t, true)
	if got := noteStates(t, p.b.st, m.ID); got != "pending" {
		t.Fatalf("worker note %s", got)
	}
	var author, text string
	if err := p.b.st.db.QueryRow(`SELECT author,text FROM task_owner_notes WHERE task_id=?`, m.ID).Scan(&author, &text); err != nil || author != AudienceCoordinator || text != "Stop and report what is done" {
		t.Fatalf("worker note %q %q %v", author, text, err)
	}
	feed, err := p.a.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	shown := false
	for _, it := range feed.Items {
		shown = shown || (it.Kind == "note" && it.State == "sent")
	}
	if !shown {
		t.Fatal("sent note not in the window's feed")
	}

	// The task stopped there: the next note comes back refused.
	if _, err := p.b.st.db.Exec(`UPDATE tasks SET state='done' WHERE id=?`, m.ID); err != nil {
		t.Fatal(err)
	}
	if err := p.a.st.AddCoordinatorNote("Game", id, "note-2", "One more thing"); err != nil {
		t.Fatal(err)
	}
	p.carry(t, true)
	p.carry(t, false)
	refused := false
	for _, e := range p.a.events(t, AudienceCoordinator, 0) {
		refused = refused || (e.TaskID == id && e.Kind == "note_refused" && strings.Contains(e.Text, "only a working task"))
	}
	if !refused {
		t.Fatalf("no note_refused: %s", kinds(p.a.events(t, AudienceCoordinator, 0)))
	}
	if got := noteStates(t, p.a.st, id); got != "sent,undelivered" {
		t.Fatalf("giver notes %s", got)
	}
}

func TestOwnerNoteWaitsForRemoteRun(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "start", "codex", "Long work")
	p.carry(t, true)
	m := p.mirror(t, id)
	p.carry(t, false)

	if err := p.a.st.AddOwnerNote(id, "owner-note", "Use blue"); !errors.Is(err, ErrRejected) {
		t.Fatalf("peer without note support: %v", err)
	}
	if err := p.a.st.SetNetwork(testBroker, HostPeerID, []NetPeer{{ID: "dev-b", Name: "Ноутбук", Age: 1, Version: "0.0.0-preview.86"}}); err != nil {
		t.Fatal(err)
	}
	const session = "019a0000-0000-7000-8000-0000000000b1"
	run := p.b.launch(t, m.ID, session)
	for range 2 {
		if err := p.a.st.AddOwnerNote(id, "owner-note", "Use blue"); err != nil {
			t.Fatal(err)
		}
	}
	feed, err := p.a.st.TaskFeed(id)
	if err != nil {
		t.Fatal(err)
	}
	notes := 0
	for _, item := range feed.Items {
		if item.Kind == "note" && item.Author == AudienceOwner && item.State == "sent" {
			notes++
		}
	}
	if notes != 1 {
		t.Fatalf("queued owner notes: %d", notes)
	}
	if got := strings.Count(kinds(p.a.events(t, AudienceCoordinator, 0)), "owner_note"); got != 1 {
		t.Fatalf("owner note events: %d", got)
	}
	p.carry(t, true)
	if got := noteStates(t, p.b.st, m.ID); got != "pending" {
		t.Fatalf("worker notes %s", got)
	}
	p.b.dispose(t, run, DispositionReport, "Work complete")
	if _, err := p.b.st.FinishTaskRun(run.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	next := p.b.launch(t, m.ID, session)
	brief, err := p.b.st.ReadTask(next.ID, next.Receipt, false)
	if err != nil || next.Kind != "note" || brief.OwnerNotes != "Use blue" || brief.HeldReport != "Work complete" {
		t.Fatalf("continuation: %+v, %+v, %v", next, brief, err)
	}
}

func noteStates(t *testing.T, st *Store, task string) string {
	t.Helper()
	rows, err := st.db.Query(`SELECT state FROM task_owner_notes WHERE task_id=? ORDER BY created_at,rowid`, task)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			t.Fatal(err)
		}
		out = append(out, s)
	}
	return strings.Join(out, ",")
}
