package supervisor

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestAudit4FinalDBFailure(t *testing.T) {
	root, st, r := seedRecoverableStream(t, true)
	db, err := sql.Open("sqlite", filepath.Join(root, dbName))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TRIGGER audit_abort_archive BEFORE DELETE ON stream_runs BEGIN SELECT RAISE(ABORT,'audit commit failure'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverStream(root, "worker", r.StreamRunID, core.StreamRecoveryConfirmation); err == nil {
		t.Fatal("DB failure ignored")
	}
	runs, err := loadRuns()
	if err != nil || len(runs) != 0 {
		t.Fatal("expected registry removal before failed final DB transaction", err)
	}
	snap, err := st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.ArchivedStreamRuns) != 0 || len(snap.StreamRuns) != 1 || snap.Sessions[0]["active_token"] == nil || snap.StreamRecoveries[0]["phase"] != "pending" {
		t.Fatal("final transaction did not roll back atomically", snap)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := Run(ctx, root, DisabledLauncher{}); !errors.Is(err, core.ErrRecoveryBlocked) {
		t.Fatal("pending recovery allowed restart", err)
	}
	// A different execution of the same session must never be removed by retry.
	replacement := r
	replacement.ID = "replacement"
	replacement.StreamRunID = "different-execution"
	if err := saveRuns([]recordedRun{replacement}); err != nil {
		t.Fatal(err)
	}
	before := readRunsFile(t)
	if _, err := RecoverStream(root, "worker", r.StreamRunID, core.StreamRecoveryConfirmation); err == nil {
		t.Fatal("recovery accepted replacement owner")
	}
	if readRunsFile(t) != before {
		t.Fatal("replacement reserve changed")
	}
	if err := saveRuns(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DROP TRIGGER audit_abort_archive`); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverStream(root, "worker", r.StreamRunID, core.StreamRecoveryConfirmation); err != nil {
		t.Fatal(err)
	}
	snap, err = st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.ArchivedStreamRuns) != 1 || len(snap.StreamRuns) != 0 || snap.Sessions[0]["active_token"] != nil || len(snap.Attempts) != 2 || len(snap.Requests) != 3 {
		t.Fatal("retry duplicated or lost state", snap)
	}
	var archivedTurns int
	if err := db.QueryRow(`SELECT COUNT(*) FROM archived_stream_turns`).Scan(&archivedTurns); err != nil || archivedTurns != 2 {
		t.Fatal("turn archive lost", archivedTurns, err)
	}
}

func audit4Turn(t *testing.T, st *core.Store, run string, complete bool) core.StreamMessage {
	t.Helper()
	if _, err := st.PrepareStreamSend(run); err != nil {
		t.Fatal(err)
	}
	if err := st.ConfirmStreamIdentity(run, "test-stream"); err != nil {
		t.Fatal(err)
	}
	m, err := st.StreamRead(run)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.StreamReply(run, m.Receipt, "synthetic reply"); err != nil {
		t.Fatal(err)
	}
	if complete {
		if err := st.CompleteStreamTurn(run, true); err != nil {
			t.Fatal(err)
		}
	}
	return m
}

func TestAudit4MultipleGenerations(t *testing.T) {
	root, st, old := seedRecoverableStream(t, true)
	if _, err := RecoverStream(root, "worker", old.StreamRunID, core.StreamRecoveryConfirmation); err != nil {
		t.Fatal(err)
	}
	if err := st.ResumeWork(); err != nil {
		t.Fatal(err)
	}
	run, err := st.Claim("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BeginLaunch(run); err != nil {
		t.Fatal(err)
	}
	if err := st.BeginStream(run); err != nil {
		t.Fatal(err)
	}
	obs := platform.Observation{PID: 123456790, PGID: 123456790, StartToken: "second-start", BootID: "second-observation-boot"}
	if err := st.RecordProcess(run, obs); err != nil {
		t.Fatal(err)
	}
	audit4Turn(t, st, run, true)
	if _, err := st.Enqueue("m4", "worker", "coordinator", "partial-response"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.NextStreamTurn(run); err != nil {
		t.Fatal(err)
	}
	last := audit4Turn(t, st, run, false)
	current := old
	current.ID = "second-reservation"
	current.StreamRunID = run
	current.PID = obs.PID
	current.PGID = obs.PGID
	current.StartToken = obs.StartToken
	current.BootID = obs.BootID
	current.Epoch.Boot = "22222222-2222-2222-2222-222222222222"
	if err := saveRuns([]recordedRun{current}); err != nil {
		t.Fatal(err)
	}
	snap, _ := st.Snapshot()
	before := readRunsFile(t)
	// Repeating the old successful command cannot pause/unlock the new owner.
	if _, err := RecoverStream(root, "worker", old.StreamRunID, core.StreamRecoveryConfirmation); err != nil {
		t.Fatal(err)
	}
	after, _ := st.Snapshot()
	if !reflect.DeepEqual(snap, after) || before != readRunsFile(t) {
		t.Fatal("old command touched new generation")
	}
	if _, err := RecoverStream(root, "worker", run, core.StreamRecoveryConfirmation); err == nil {
		t.Fatal("same boot recovery accepted")
	}
	recoveryEpoch = func() (platform.RecoveryEpoch, error) {
		e := current.Epoch
		e.Boot = "33333333-3333-3333-3333-333333333333"
		return e, nil
	}
	// No StopStream: this is the persisted active state at an abrupt interruption.
	if _, err := RecoverStream(root, "worker", run, core.StreamRecoveryConfirmation); err != nil {
		t.Fatal(err)
	}
	snap, err = st.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !snap.Paused || len(snap.ArchivedStreamRuns) != 2 || len(snap.StreamRecoveries) != 2 || len(snap.Replies) != 3 || len(snap.Attempts) != 4 || len(snap.Requests) != 4 {
		t.Fatal("generation history lost", snap)
	}
	if snap.Requests[1]["state"] != "uncertain" || snap.Requests[3]["state"] != "uncertain" {
		t.Fatal("incomplete outcomes promoted", snap.Requests)
	}
	if _, err := st.StreamReply(run, last.Receipt, "late"); err == nil {
		t.Fatal("archived receipt accepted")
	}
	for _, v := range snap.Verifications {
		if v["attempt_token"] == snap.Attempts[3]["token"] && v["provider_ok"] != int64(0) {
			t.Fatal("partial reply became verified", v)
		}
	}
}
