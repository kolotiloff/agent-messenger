package core

import (
	"context"
	"testing"
	"time"
)

func TestEventReaderFollowsActivatedProjectAndItsNotices(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenTaskEventReader(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	ctx := context.Background()
	if err := ActivateProjectStorage(ctx, f.path); err != nil {
		t.Fatal(err)
	}
	active, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer active.Close()
	if err := active.NoteMacUpdateRestart(time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, _, err := reader.TaskEvents(ctx, "Game", AudienceCoordinator, 0, 20); err != nil {
		t.Fatal(err)
	}
	if pending, err := reader.PendingProjectNotice(ctx, "Game"); err != nil || !pending {
		t.Fatalf("Game notice pending = %t: %v", pending, err)
	}
	if _, _, err := active.TaskEvents("Game", AudienceCoordinator, 0, 20); err != nil {
		t.Fatal(err)
	}
	events, _, err := reader.TaskEvents(ctx, "Game", AudienceCoordinator, 0, 20)
	if err != nil || len(events) != 1 || events[0].Kind != MacUpdateRestartEvent {
		t.Fatalf("watch after activation: %+v: %v", events, err)
	}
	if events, _, err := reader.TaskEvents(ctx, "Site", AudienceCoordinator, 0, 20); err != nil || len(events) != 0 {
		t.Fatalf("Site saw Game's saved notice: %+v: %v", events, err)
	}
}

func TestSplitProjectTaskVersionTracksReleasedResource(t *testing.T) {
	f := newTaskFixture(t)
	if err := f.st.CreateProject("Site", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := ActivateProjectStorage(context.Background(), f.path); err != nil {
		t.Fatal(err)
	}
	s, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	start := func(project string) string {
		t.Helper()
		id, _, err := s.StartTask(StartTaskRequest{Project: project, OpID: project, Provider: "codex", Text: "Work", Lock: "shared-device"})
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	first, second := start("Site"), start("Game")
	if lock, holder, err := s.ProjectTaskLock("Game", second); err != nil || lock != "shared-device" || holder != "" {
		t.Fatalf("other project leaked or lock missing: %q %q: %v", lock, holder, err)
	}
	before, err := s.TaskListVersion("Game")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CancelTask(first); err != nil {
		t.Fatal(err)
	}
	after, err := s.TaskListVersion("Game")
	if err != nil || before == after {
		t.Fatalf("lock release did not change project version: %q to %q: %v", before, after, err)
	}
}

func TestTaskEventsCanReadWhileAnotherProcessOwnsWriter(t *testing.T) {
	f := newTaskFixture(t)
	reader, err := Open(f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	writer, err := f.st.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	done := make(chan error, 1)
	go func() {
		_, _, err := reader.TaskEvents("Game", AudienceCoordinator, 0, 20)
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("event reader waited for an unrelated writer")
	}
}
