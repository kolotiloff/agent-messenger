package core

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
)

func TestActiveProjectMigratesRemoteLiveUsage(t *testing.T) {
	pair := newNetPair(t)
	task := pair.startRemote(t, "upgrade", "codex", "Keep this remote task")
	pair.carry(t, true)
	pair.carry(t, false)
	if err := ActivateProjectStorage(context.Background(), pair.a.path); err != nil {
		t.Fatal(err)
	}
	open := func() *Store {
		t.Helper()
		s, err := Open(pair.a.path)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { s.Close() })
		if s.split == nil {
			t.Fatal("lost selected project generation")
		}
		return s
	}
	s := open()
	before, err := s.TaskFeed(task)
	if err != nil || len(before.Items) == 0 || before.Task.Remote == nil || before.Task.State != "running" {
		t.Fatalf("selected generation lost the remote task: %+v, %v", before.Task, err)
	}
	if err := s.CreateProject("New", t.TempDir()); err != nil {
		t.Fatal(err)
	}
	dir := s.split.root
	s.Close()
	// Inspect the newly published file without letting the project opener repair it.
	db, err := sql.Open("sqlite", workerProjectDSN(filepath.Join(dir, projectMigrationFile("New"))))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec("SELECT remote_live_tokens,remote_live_ordinal FROM tasks LIMIT 0")
	db.Close()
	if err != nil {
		t.Fatal(err)
	}

	db, err = sql.Open("sqlite", workerProjectDSN(filepath.Join(dir, projectMigrationFile("Game"))))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, tc := range []struct {
		name            string
		tokens, ordinal bool
	}{{"neither", false, false}, {"tokens only", true, false}, {"ordinal only", false, true}, {"both manually repaired", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := db.Exec("UPDATE tasks SET remote_live_tokens=123,remote_live_ordinal=7 WHERE id=?", task); err != nil {
				t.Fatal(err)
			}
			if !tc.tokens {
				if _, err := db.Exec("ALTER TABLE tasks DROP COLUMN remote_live_tokens"); err != nil {
					t.Fatal(err)
				}
			}
			if !tc.ordinal {
				if _, err := db.Exec("ALTER TABLE tasks DROP COLUMN remote_live_ordinal"); err != nil {
					t.Fatal(err)
				}
			}
			for i := 0; i < 2; i++ {
				s := open()
				feed, err := s.TaskFeed(task)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(feed.Items, before.Items) {
					t.Fatal("remote task history changed")
				}
				peer := feed.Task.Remote
				if peer == nil || peer.ID != before.Task.Remote.ID || peer.Name != before.Task.Remote.Name ||
					peer.From != before.Task.Remote.From || peer.Lost {
					t.Fatalf("remote task peer changed: %+v", peer)
				}
				got := feed.Task.Quiet
				if tc.tokens && (got == nil || got.LiveTokens == nil || *got.LiveTokens != 123) ||
					!tc.tokens && got != nil && got.LiveTokens != nil {
					t.Fatalf("remote tokens changed: %+v", got)
				}
				var ordinal int
				if err := db.QueryRow("SELECT remote_live_ordinal FROM tasks WHERE id=?", task).Scan(&ordinal); err != nil ||
					tc.ordinal && ordinal != 7 || !tc.ordinal && ordinal != 0 {
					t.Fatalf("remote ordinal = %d, %v", ordinal, err)
				}
				s.Close()
			}
		})
	}

	// When only the ordinal survived a partial repair, the first measurement
	// for that same run must still be accepted through the network mailbox.
	if _, err := db.Exec("UPDATE tasks SET remote_live_tokens=NULL,remote_live_ordinal=1 WHERE id=?", task); err != nil {
		t.Fatal(err)
	}
	pair.a.st = open()
	mirror := pair.mirror(t, task)
	run := pair.b.launch(t, mirror.ID, "019a0000-0000-7000-8000-0000000000b1")
	if err := pair.b.st.NoteLiveTokens(run.ID, 250); err != nil {
		t.Fatal(err)
	}
	pair.carry(t, false)
	feed, err := pair.a.st.TaskFeed(task)
	if err != nil || feed.Task.Quiet == nil || feed.Task.Quiet.LiveTokens == nil || *feed.Task.Quiet.LiveTokens != 250 {
		t.Fatalf("first measurement after partial repair: %+v, %v", feed.Task.Quiet, err)
	}
}
