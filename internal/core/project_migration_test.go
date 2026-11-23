package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestStageProjectMigrationPartitionsPendingDelivery(t *testing.T) {
	pair := newNetPair(t)
	for _, fixture := range []taskFixture{pair.a, pair.b} {
		if err := fixture.st.CreateProject("Site", t.TempDir()); err != nil {
			t.Fatal(err)
		}
	}
	origin := pair.startRemote(t, "game", "claude", "Game work")
	site, _, err := pair.a.st.StartTask(StartTaskRequest{
		Project: "Site", OpID: "site", Computer: "dev-b", Provider: "codex", Text: "Site work",
	})
	if err != nil {
		t.Fatal(err)
	}
	pair.carry(t, true)
	// This acceptance still waits in a parked mailbox. A staged copy must
	// preserve that status as well as the exact body and operation ID.
	if _, err := pair.b.st.db.Exec("UPDATE net_outbox SET broker=? WHERE kind='accepted' AND task_id=?", parkedPrefix+testBroker, site); err != nil {
		t.Fatal(err)
	}
	mirror := pair.mirror(t, origin)
	run := pair.b.launch(t, mirror.ID, mirror.SessionID)
	pair.b.dispose(t, run, DispositionReport, "project report")
	if state, err := pair.b.st.FinishTaskRun(run.ID, cleanExit); err != nil || state != "finished" {
		t.Fatalf("finish: %q %v", state, err)
	}
	if err := pair.b.st.tx(func(tx *sql.Tx) error {
		if err := enqueueNet(tx, testBroker, HostPeerID, "agent-version-fixture", "version-fixture", netBody{Kind: "agent_version", Task: "version-fixture"}); err != nil {
			return err
		}
		return enqueueNet(tx, testBroker, "old-peer", "refused-unrouted", site, netBody{Kind: "refused", Task: site, Reason: "legacy fixture"})
	}); err != nil {
		t.Fatal(err)
	}

	base := t.TempDir()
	out := filepath.Join(base, "stage with spaces")
	if err := StageProjectMigration(context.Background(), pair.b.path, out); err != nil {
		t.Fatal(err)
	}
	if err := StageProjectMigration(context.Background(), pair.b.path, out); !errors.Is(err, os.ErrExist) {
		t.Fatalf("completed stage replaced: %v", err)
	}
	raw, err := os.ReadFile(filepath.Join(out, migrationManifestFile))
	if err != nil {
		t.Fatal(err)
	}
	var manifest projectMigrationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.ComputerPending != 2 || len(manifest.Projects) != 2 {
		t.Fatalf("manifest: %+v", manifest)
	}
	open := func(name string) *sql.DB {
		t.Helper()
		db, err := sql.Open("sqlite", sqliteFileURL(filepath.Join(out, name), url.Values{"mode": {"ro"}}))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { db.Close() })
		return db
	}
	type outgoing struct {
		NetOutgoing
		broker string
	}
	messages := func(db *sql.DB) map[int64]outgoing {
		t.Helper()
		rows, err := db.Query("SELECT seq,recipient,op_id,body,broker FROM net_outbox ORDER BY seq")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		result := map[int64]outgoing{}
		for rows.Next() {
			var message outgoing
			if err := rows.Scan(&message.Seq, &message.To, &message.OpID, &message.Body, &message.broker); err != nil {
				t.Fatal(err)
			}
			result[message.Seq] = message
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return result
	}
	backup := open(manifest.Backup)
	common := open(manifest.Computer)
	all := messages(backup)
	originalCount := len(all)
	compare := func(staged map[int64]outgoing) {
		t.Helper()
		for seq, message := range staged {
			record, ok := all[seq]
			if !ok || record.OpID != message.OpID || record.To != message.To || record.broker != message.broker || string(record.Body) != string(message.Body) {
				t.Fatalf("pending message %d changed or was assigned twice", seq)
			}
			delete(all, seq)
		}
	}
	compare(messages(common))
	for _, project := range manifest.Projects {
		db := open(project.File)
		staged := messages(db)
		if len(staged) != project.Pending || project.Tasks != 1 {
			t.Fatalf("project: %+v", project)
		}
		for _, message := range staged {
			var body netBody
			if err := json.Unmarshal(message.Body, &body); err != nil {
				t.Fatal(err)
			}
			want := map[string]string{"Game": origin, "Site": site}[project.Name]
			if body.Task != want {
				t.Fatalf("project %s got delivery for %s", project.Name, body.Task)
			}
		}
		compare(staged)
	}
	if len(all) != 0 {
		t.Fatalf("%d pending messages lost", len(all))
	}
	var originProject string
	if err := common.QueryRow("SELECT project FROM task_storage_routes WHERE origin_task=?", origin).Scan(&originProject); err != nil || originProject != "Game" {
		t.Fatalf("origin route: %q %v", originProject, err)
	}
	var ledger, taskTables int
	err = common.QueryRow(`SELECT (SELECT COUNT(*) FROM net_applied),
 (SELECT COUNT(*) FROM sqlite_schema WHERE name='tasks')`).Scan(&ledger, &taskTables)
	if err != nil || ledger != 2 || taskTables != 0 {
		t.Fatalf("computer database: deduplication rows=%d task tables=%d err=%v", ledger, taskTables, err)
	}
	if db, err := Open(filepath.Join(out, manifest.Computer)); !errors.Is(err, ErrRejected) {
		db.Close()
		t.Fatalf("staged computer opened as a working queue: %v", err)
	}
	if err := VerifyProjectMigration(context.Background(), out); err != nil {
		t.Fatalf("published layout changed after a refused open: %v", err)
	}
	if pending := messages(pair.b.st.db); len(pending) != originalCount {
		t.Fatalf("live outbox changed: %d", len(pending))
	}
	if cursor, err := pair.b.st.NetCursor(testBroker); err != nil || cursor != pair.seq {
		t.Fatalf("live cursor changed: %d %v", cursor, err)
	}
}

func TestFailedPublicationDoesNotLeaveACompletedMigration(t *testing.T) {
	scratch, parent := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(scratch, migrationBackupFile), []byte("fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(parent, "partial")
	if err := publishProjectMigration(scratch, out, projectMigrationManifest{}); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected the absent second file to stop publication: %v", err)
	}
	if _, err := os.Stat(out); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed publication left an output directory: %v", err)
	}
}

func TestStageProjectMigrationCanRetryAfterCancellation(t *testing.T) {
	fixture := newTaskFixture(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "migration")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := StageProjectMigration(ctx, fixture.path, path); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("incomplete migration published: %v", err)
	}
	if err := StageProjectMigration(context.Background(), fixture.path, path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(path, migrationManifestFile)); err != nil {
		t.Fatal(err)
	}
	if err := VerifyProjectMigration(context.Background(), path); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filepath.Join(path, migrationComputerFile), 128); err != nil {
		t.Fatal(err)
	}
	if err := VerifyProjectMigration(context.Background(), path); err == nil {
		t.Fatal("verification accepted a damaged database")
	}
}
