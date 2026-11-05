package core

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

const outboxTestID = "ui-2e0539ac-1f0a-4bb8-b533-a6b2daa143c5"

func TestUIOutboxReopenDoesNotDispatchAndSubmitSurvivesLostAck(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queue.sqlite")
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = st.Register("synthetic", t.TempDir(), "test-only", "worker"); err != nil {
		t.Fatal(err)
	}
	var syncMode int
	if err = st.db.QueryRow(`PRAGMA synchronous`).Scan(&syncMode); err != nil || syncMode != 2 {
		t.Fatal("durability not FULL", err, syncMode)
	}
	if err = st.PrepareUIMessage(outboxTestID, "worker", "one"); err != nil {
		t.Fatal(err)
	}
	if err = st.AcknowledgeUIMessage(outboxTestID); err == nil {
		t.Fatal("unsubmitted message hidden")
	}
	st.Close()
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	out, err := st.UIOutbox()
	if err != nil || len(out.Items) != 1 || out.Items[0].State != "prepared" {
		t.Fatalf("outbox: %+v %v", out, err)
	}
	snap, err := st.Snapshot()
	if err != nil || len(snap.Requests) != 0 || len(snap.Attempts) != 0 {
		t.Fatal("preparation dispatched")
	}
	if created, err := st.SubmitUIMessage(outboxTestID); err != nil || !created {
		t.Fatalf("submit: %v %v", created, err)
	}
	st.Close()
	st, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	out, err = st.UIOutbox()
	if err != nil || len(out.Items) != 1 || out.Items[0].State != "queued" {
		t.Fatalf("queue ACK lost: %+v %v", out, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c, e := st.SubmitUIMessage(outboxTestID); e != nil || c {
				t.Errorf("repeat: %v %v", c, e)
			}
		}()
	}
	wg.Wait()
	if err = st.AcknowledgeUIMessage(outboxTestID); err != nil {
		t.Fatal(err)
	}
	if err = st.AcknowledgeUIMessage(outboxTestID); err != nil {
		t.Fatal("ack not idempotent", err)
	}
	out, err = st.UIOutbox()
	if err != nil || len(out.Items) != 0 {
		t.Fatal("acknowledged envelope visible", err)
	}
	snap, err = st.Snapshot()
	if err != nil || len(snap.Requests) != 1 || len(snap.Attempts) != 0 {
		t.Fatal("queue lost or model attempted")
	}
	if c, e := st.SubmitUIMessage(outboxTestID); e != nil || c {
		t.Fatal("acknowledged submit replayed")
	}
}

func TestUIOutboxAtomicQueueCommitAndImmutableRoute(t *testing.T) {
	st, env := openTest(t)
	if _, err := st.Register("synthetic", env, "one", "worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Register("synthetic", env, "two", "other"); err != nil {
		t.Fatal(err)
	}
	if err := st.PrepareUIMessage(outboxTestID, "worker", "one"); err != nil {
		t.Fatal(err)
	}
	if err := st.PrepareUIMessage(outboxTestID, "other", "one"); err == nil {
		t.Fatal("route rebound")
	}
	if err := st.PrepareUIMessage(outboxTestID, "worker", "two"); err == nil {
		t.Fatal("text changed")
	}
	if _, err := st.db.Exec(`CREATE TRIGGER fail_outbox BEFORE UPDATE ON ui_outbox WHEN NEW.state='queued' BEGIN SELECT RAISE(ABORT,'fixture write failure'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SubmitUIMessage(outboxTestID); err == nil {
		t.Fatal("commit unexpectedly succeeded")
	}
	snap, _ := st.Snapshot()
	if len(snap.Requests) != 0 {
		t.Fatal("queue insert survived failed outbox update")
	}
	out, _ := st.UIOutbox()
	if out.Items[0].State != "prepared" {
		t.Fatal("prepared marker changed")
	}
	if _, err := st.db.Exec(`DROP TRIGGER fail_outbox`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SubmitUIMessage(outboxTestID); err != nil {
		t.Fatal(err)
	}
	// Simulate damaged data. A stored queued marker never authorizes replay.
	if _, err := st.db.Exec(`DELETE FROM requests WHERE id=?`, outboxTestID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SubmitUIMessage(outboxTestID); err == nil {
		t.Fatal("missing request replayed")
	}
	if _, err := st.UIOutbox(); err == nil {
		t.Fatal("damaged queue reported as persisted")
	}
	if err := st.AcknowledgeUIMessage(outboxTestID); err == nil {
		t.Fatal("damaged envelope hidden")
	}
}

func TestUIOutboxBoundsAndLegacyRequestAdoption(t *testing.T) {
	st, env := openTest(t)
	if _, err := st.Register("synthetic", env, "one", "worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Enqueue(outboxTestID, "worker", "local-user", "one"); err != nil {
		t.Fatal(err)
	}
	if err := st.PrepareUIMessage(outboxTestID, "worker", "one"); err != nil {
		t.Fatal(err)
	}
	out, _ := st.UIOutbox()
	if len(out.Items) != 1 || out.Items[0].State != "queued" {
		t.Fatal("legacy request not recognized")
	}
	for i := 1; i < UIOutboxLimit; i++ {
		if err := st.PrepareUIMessage(fmt.Sprintf("ui-%08x-0000-0000-0000-000000000000", i), "worker", "one"); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.PrepareUIMessage("ui-ffffffff-0000-0000-0000-000000000000", "worker", "one"); err == nil {
		t.Fatal("unbounded outbox")
	}
	if err := st.AcknowledgeUIMessage(outboxTestID); err != nil {
		t.Fatal(err)
	}
	if err := st.PrepareUIMessage("ui-ffffffff-0000-0000-0000-000000000000", "worker", "one"); err != nil {
		t.Fatal("ack did not free slot", err)
	}
}
