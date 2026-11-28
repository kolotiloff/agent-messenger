package core

import (
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestTaskArchiveRetainsHistoryAndProtectsUnfinishedTasks(t *testing.T) {
	f := newTaskFixture(t)
	id := f.start(t, "archive", "codex", "Archived task")
	if err := f.st.SetTaskArchive(id, "archive", 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("archived active task: %v", err)
	}
	run := f.launch(t, id, "019a0000-0000-7000-8000-000000000001")
	f.dispose(t, run, DispositionReport, "Original report")
	if _, err := f.st.FinishTaskRun(run.ID, cleanExit); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetTaskArchive(id, "remove", 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("direct removal bypassed archiving: %v", err)
	}
	state := func(archived bool, count int) {
		t.Helper()
		page, err := f.st.WindowTaskHistory("Game", "", "", "")
		if err != nil || len(page.Tasks) != count || (count > 0 && (page.Tasks[0].ID != id || page.Tasks[0].Archived != archived)) {
			t.Fatalf("archive: %+v, %v", page, err)
		}
	}
	for _, change := range []struct {
		action  string
		version int64
	}{
		{"archive", 0}, {"restore", 1}, {"archive", 2}, {"remove", 3},
	} {
		if err := f.st.SetTaskArchive(id, change.action, change.version); err != nil {
			t.Fatal(err)
		}
		if err := f.st.SetTaskArchive(id, change.action, change.version); err != nil {
			t.Fatalf("repeated %s: %v", change.action, err)
		}
		switch change.action {
		case "archive":
			if err := f.st.HideWindowHistory("Game", "", ""); err != nil {
				t.Fatal(err)
			}
			state(true, 1)
		case "restore":
			state(false, 1)
			if err := f.st.SetTaskArchive(id, "archive", 0); !errors.Is(err, ErrRejected) {
				t.Fatalf("old archive request bypassed newer restore: %v", err)
			}
		case "remove":
			state(false, 0)
		}
	}
	if err := f.st.SetTaskArchive(id, "restore", 4); !errors.Is(err, ErrRejected) {
		t.Fatalf("restored a removed card: %v", err)
	}
	feed, err := f.st.TaskFeed(id)
	if err != nil || feed.Task.ID != id || len(feed.Items) == 0 || f.sessionOf(t, id) == "" {
		t.Fatalf("retained task: %+v, %v", feed, err)
	}
	if err := f.st.ContinueTask(id, "removed-follow-up", "Follow up", AudienceCoordinator); !errors.Is(err, ErrRejected) {
		t.Fatalf("removed task was restarted: %v", err)
	}
	legacy := f.start(t, "legacy-hidden", "codex", "Old hidden card")
	if _, err := f.st.CancelTask(legacy); err != nil {
		t.Fatal(err)
	}
	if err := f.st.HideTask(legacy); err != nil {
		t.Fatal(err)
	}
	if err := f.st.SetTaskArchive(legacy, "archive", 0); !errors.Is(err, ErrRejected) {
		t.Fatalf("converted an old hidden card: %v", err)
	}
}

func TestTaskArchiveReplicatesWithoutHidingUnfinishedMirror(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "remote-archive", "claude", "Origin archived task")
	p.carry(t, true)
	p.carry(t, false)
	mirror := p.mirror(t, id)
	finish := func(st *Store, task string) {
		t.Helper()
		if err := st.taskTx(task, func(_, tx *sql.Tx) error { return setTaskState(tx, task, "cancelled", "fixture cancelled") }); err != nil {
			t.Fatal(err)
		}
	}
	finish(p.a.st, id)
	if err := p.a.st.SetTaskArchive(id, "archive", 0); err != nil {
		t.Fatal(err)
	}
	shared, err := p.a.st.SharedTasks(time.Now().Add(-time.Hour))
	if err != nil || len(shared) != 1 || !shared[0].Archived {
		t.Fatalf("shared archive: %+v, %v", shared, err)
	}
	if err := p.a.st.SetTaskArchive(id, "remove", 1); err != nil {
		t.Fatal(err)
	}
	messages := p.carry(t, true)
	if len(messages) != 2 {
		t.Fatalf("archive deliveries: %+v", messages)
	}
	p.seq++
	messages[0].Seq = p.seq
	if err := p.b.st.ApplyNet(testBroker, messages[0]); err != nil {
		t.Fatal(err)
	}
	if err := p.b.st.SetTaskArchive(mirror.ID, "restore", 2); !errors.Is(err, ErrRejected) {
		t.Fatalf("executor edited archive: %v", err)
	}
	view, err := p.b.st.WindowView("Game", "")
	if err != nil || len(view.Tasks) != 1 || view.Tasks[0].ID != mirror.ID || view.Tasks[0].Archived {
		t.Fatalf("active mirror: %+v, %v", view.Tasks, err)
	}
	finish(p.b.st, mirror.ID)
	view, err = p.b.st.WindowView("Game", "")
	if err != nil || len(view.Tasks) != 0 {
		t.Fatalf("finished mirror: %+v, %v", view.Tasks, err)
	}
	record := p.mirror(t, id)
	if record.archiveState != archiveRemoved || record.archiveVersion != 2 {
		t.Fatalf("mirror removal tombstone: %+v", record)
	}
}
