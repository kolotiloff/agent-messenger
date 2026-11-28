package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"modernc.org/sqlite"
)

var registerWindowCancellation sync.Once
var cancelWindowRead func() error

func TestBusySnapshotDiscardsConnectionWithoutReplayingWrite(t *testing.T) {
	f := newTaskFixture(t)
	// A deferred transaction lets us create a real expired snapshot on
	// demand. Normal store transactions already request the lock at BEGIN.
	db, err := sql.Open("sqlite", sqliteFileURL(f.path, nil))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(`CREATE TEMP TABLE connection_marker (id)`); err != nil {
		t.Fatal(err)
	}
	st := &Store{db: db}
	calls := 0
	err = st.tx(func(tx *sql.Tx) error {
		calls++
		if _, err := metaValue(tx, "computer_name"); err != nil {
			return err
		}
		if err := f.st.SetComputerName("Committed by another connection"); err != nil {
			return err
		}
		return setMeta(tx, "computer_name", "Must not be replayed")
	})
	if !busySnapshot(err) || calls != 1 {
		t.Fatalf("expected one attempt and a busy snapshot, got calls=%d, err=%v", calls, err)
	}
	var reused bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM sqlite_temp_schema WHERE name='connection_marker')`).Scan(&reused); err != nil || reused {
		t.Fatalf("expired connection was returned to the pool: reused=%v, err=%v", reused, err)
	}
	if name, err := st.ComputerName(); err != nil || name != "Committed by another connection" {
		t.Fatalf("committed state was changed: name=%q, err=%v", name, err)
	}
	if err := st.SetNetwork("broker", "self", nil); err != nil {
		t.Fatalf("subsequent request could not write: %v", err)
	}
}

func TestWindowRevisionCancellationFinishesRead(t *testing.T) {
	registerWindowCancellation.Do(func() {
		sqlite.MustRegisterScalarFunction("cancel_window_read", 0, func(*sqlite.FunctionContext, []driver.Value) (driver.Value, error) {
			return int64(1), cancelWindowRead()
		})
	})
	f := newTaskFixture(t)
	task := f.start(t, "before-update", "claude", "fixture")
	f.launch(t, task, f.sessionOf(t, task))
	if err := ActivateProjectStorage(context.Background(), f.path); !errors.Is(err, ErrProjectStorageBusy) {
		t.Fatalf("running task must keep the existing database in use: %v", err)
	}
	writer := mustOpen(t, f.path)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cancelWindowRead = func() error {
		// Advance WAL after the reader took its snapshot and cancel while
		// the SQL statement is still on the stack.
		err := writer.SetComputerName("Updated from another process")
		cancel()
		return err
	}
	t.Cleanup(func() { cancelWindowRead = nil })
	var got int
	err := scanWindowRevision(ctx, f.st.db, `SELECT cancel_window_read() FROM projects LIMIT 1`, nil, &got)
	if !errors.Is(err, context.Canceled) || got != 1 {
		t.Fatalf("cancelled revision did not finish reading: value=%d, err=%v", got, err)
	}
	if err := f.st.SetNetwork("broker", "self", nil); err != nil {
		t.Fatalf("write after a cancelled window request: %v", err)
	}
	view, err := f.st.WindowView("Game", "")
	if err != nil || view.Project != "Game" || view.Name != "Updated from another process" {
		t.Fatalf("saved project did not load after cancellation: project=%q, computer=%q, err=%v", view.Project, view.Name, err)
	}
	if len(view.Tasks) != 1 || view.Tasks[0].ID != task {
		t.Fatalf("saved task did not load after cancellation: %+v", view.Tasks)
	}

	// The watch must still be cancellable while waiting for the pool.
	conn, err := f.st.db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	wait, stop := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer stop()
	if _, err := f.st.WindowChanges(wait, "Game", ""); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("blocked window watch did not honor its deadline: %v", err)
	}
}

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
	if err := p.b.st.AppendTaskRunItem(run.ID, "text_delta", "", "remote progress", ""); err != nil {
		t.Fatal(err)
	}
	p.carry(t, false)
	delta, err := p.a.st.TaskFeedDelta(id, feed.Cursor)
	if err != nil || delta.AppendFrom != len(feed.Items) || len(delta.Items) != 1 || delta.Items[0].Kind != "text_delta" {
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
