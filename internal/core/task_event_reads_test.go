package core

import (
	"testing"
	"time"
)

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
