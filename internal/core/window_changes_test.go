package core

import (
	"context"
	"reflect"
	"testing"
)

func TestWindowChangesAndLocalFeedDelta(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "watch-local", "claude", "fixture")
	run := f.launch(t, id, f.sessionOf(t, id))
	ctx := context.Background()
	change, err := f.st.WindowChanges(ctx, "Game", id)
	if err != nil {
		t.Fatal(err)
	}
	feed, err := f.st.TaskFeedDelta(id, "")
	if err != nil || feed.Cursor == "" || len(feed.Items) != 1 {
		t.Fatalf("initial feed: %+v %v", feed, err)
	}
	// A second process uses a separate connection to the same queue.
	writer, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	if err := writer.AppendTaskRunItem(run.ID, "text", "", "new progress", ""); err != nil {
		t.Fatal(err)
	}
	next, err := f.st.WindowChanges(ctx, "Game", id)
	if err != nil || next.Window != change.Window || next.Feed == change.Feed {
		t.Fatalf("progress should only signal its feed: %+v => %+v, %v", change, next, err)
	}
	other, err := f.st.WindowChanges(ctx, "Game", "")
	if err != nil || other.Feed != "0" {
		t.Fatalf("another subscription received the change: %+v, %v", other, err)
	}
	delta, err := f.st.TaskFeedDelta(id, feed.Cursor)
	if err != nil || delta.AppendFrom != 1 || len(delta.Items) != 1 || delta.Items[0].Text != "new progress" {
		t.Fatalf("append: %+v %v", delta, err)
	}
	full, err := f.st.TaskFeed(id)
	if err != nil || !reflect.DeepEqual(append(feed.Items, delta.Items...), full.Items) {
		t.Fatalf("reconstructed feed differs from full feed: %v", err)
	}
	f.dispose(t, run, DispositionReport, "a finished report")
	if _, err := f.st.FinishTaskRun(run.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	replacement, err := f.st.TaskFeedDelta(id, delta.Cursor)
	if err != nil || replacement.AppendFrom != 0 || len(replacement.Items) != 3 || replacement.Items[2].Kind != "report" || !replacement.Items[2].Verified {
		t.Fatalf("verification must replace the feed: %+v %v", replacement, err)
	}
	finished, err := f.st.WindowChanges(ctx, "Game", id)
	if err != nil || finished.Window == next.Window || finished.Feed == next.Feed {
		t.Fatalf("completion must notify list and feed: %+v => %+v, %v", next, finished, err)
	}
}

func TestRemoteFeedDeltaMatchesFullAndResetsForOlderItem(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "watch-remote", "claude", "fixture")
	p.carry(t, true)
	p.carry(t, false)
	mirror := p.mirror(t, id)
	run := p.b.launch(t, mirror.ID, mirror.SessionID)
	p.carry(t, false)
	feed, err := p.a.st.TaskFeedDelta(id, "")
	if err != nil || feed.Cursor == "" || len(feed.Items) < 2 {
		t.Fatalf("initial remote feed: %+v %v", feed, err)
	}
	if err := p.b.st.AppendTaskRunItem(run.ID, "text", "", "remote progress", ""); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	delta, err := p.a.st.TaskFeedDelta(id, feed.Cursor)
	if err != nil || delta.AppendFrom != len(feed.Items) || len(delta.Items) != 1 {
		t.Fatalf("remote progress must append: %+v %v", delta, err)
	}
	full, err := p.a.st.TaskFeed(id)
	if err != nil || !reflect.DeepEqual(append(feed.Items, delta.Items...), full.Items) {
		t.Fatalf("remote reconstruction differs from full feed: %v", err)
	}
	_, err = p.a.st.db.Exec(`INSERT INTO remote_task_items(task_id,seq,kind,title,text,description,created_at)
 VALUES (?,100,'text','','delayed progress','','2000-01-01T00:00:00Z')`, id)
	if err != nil {
		t.Fatal(err)
	}
	fresh, err := p.a.st.TaskFeedDelta(id, delta.Cursor)
	if err != nil || fresh.AppendFrom != 0 || len(fresh.Items) != len(full.Items)+1 || fresh.Items[1].Text != "delayed progress" {
		t.Fatalf("an older item must replace the page in the correct order: %+v %v", fresh, err)
	}
}
