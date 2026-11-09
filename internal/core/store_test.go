package core

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestCanonicalEnvironmentRejectsNamesAndRelativePaths(t *testing.T) {
	root := t.TempDir()
	if _, err := CanonicalEnvironment("local-default-test"); err == nil {
		t.Fatal("display name was accepted")
	}
	if _, err := CanonicalEnvironment(filepath.Join(root, "missing")); err == nil {
		t.Fatal("missing directory was accepted")
	}
	if _, err := CanonicalEnvironment("relative"); err == nil {
		t.Fatal("relative environment was accepted")
	}
}

func TestCaseFoldSharesOneEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the test folds a case-sensitive POSIX folder")
	}
	root := t.TempDir()
	real := filepath.Join(root, "profile")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	fold := func(string, string, string) bool { return true }
	lower, err := foldWalk(real, fold)
	if err != nil {
		t.Fatal(err)
	}
	upper, err := foldWalk(filepath.Join(root, "Profile"), fold)
	if err != nil {
		t.Fatal(err)
	}
	if lower != upper || upper != real {
		t.Fatalf("case fold = %s %s", lower, upper)
	}
	if _, err := foldWalk(filepath.Join(root, "Profile"), func(string, string, string) bool { return false }); err == nil {
		t.Fatal("case-sensitive walk accepted a different case")
	}
}

func TestPauseSurvivesRestart(t *testing.T) {
	db := filepath.Join(t.TempDir(), "queue.sqlite")
	st := mustOpen(t, db)
	if err := st.Pause(); err != nil {
		t.Fatal(err)
	}
	st.Close()
	st = mustOpen(t, db)
	if paused, err := st.Paused(); err != nil || !paused {
		t.Fatalf("paused = %t %v", paused, err)
	}
	if err := st.ResumeWork(); err != nil {
		t.Fatal(err)
	}
	if paused, err := st.Paused(); err != nil || paused {
		t.Fatalf("paused after resume = %t %v", paused, err)
	}
}

// legacyTables are the old mode's tables (T-028, D-104). They must survive
// every open unchanged.
var legacyTables = []string{"sessions", "aliases", "requests", "attempts", "replies", "verifications",
	"stream_runs", "stream_turns", "archived_stream_runs", "archived_stream_turns", "stream_recoveries",
	"ui_outbox", "codex_setups", "codex_turns"}

// A database written by the old mode, with a live persistent session, a
// queued message and an unsent window message, opens in this version and
// keeps every old row as it was. A task can be recorded next to them.
func TestOldModeDatabaseOpensAndKeepsItsRows(t *testing.T) {
	root := t.TempDir()
	db := filepath.Join(root, "queue.sqlite")
	writeOldModeDatabase(t, db)
	before := dumpTables(t, db, legacyTables)
	st := mustOpen(t, db)
	if err := st.Pause(); err != nil {
		t.Fatal(err)
	}
	if err := st.ResumeWork(); err != nil {
		t.Fatal(err)
	}
	folder := realDir(t, mkdir(t, filepath.Join(root, "game")))
	allowTempFolders(t)
	if err := st.CreateProject("Game", folder); err != nil {
		t.Fatal(err)
	}
	st.Close()
	if after := dumpTables(t, db, legacyTables); !reflect.DeepEqual(before, after) {
		t.Fatalf("old rows changed:\nbefore %v\nafter  %v", before, after)
	}
	mustOpen(t, db).Close()
	if after := dumpTables(t, db, legacyTables); !reflect.DeepEqual(before, after) {
		t.Fatal("old rows changed on a second open")
	}
}

// The earliest recorded layout (no CLI path, workspace or start mode on
// sessions, no schema version) still opens and gains the later columns.
func TestEarliestDatabaseLayoutStillOpens(t *testing.T) {
	db := filepath.Join(t.TempDir(), "queue.sqlite")
	raw, err := sql.Open("sqlite", dsn(db))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE sessions (key TEXT PRIMARY KEY, provider TEXT NOT NULL, environment TEXT NOT NULL, conversation_id TEXT NOT NULL, active_token TEXT, decision TEXT, UNIQUE(provider, environment, conversation_id));
INSERT INTO sessions(key,provider,environment,conversation_id) VALUES ('k1','claude','/profile','old-conversation')`); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	mustOpen(t, db).Close()
	dump := dumpTables(t, db, []string{"sessions"})
	if !strings.Contains(dump["sessions"], "old-conversation") || !strings.Contains(dump["sessions"], "start_mode=resume") || !strings.Contains(dump["sessions"], "cli_path=<nil>") {
		t.Fatalf("sessions = %s", dump["sessions"])
	}
}

// writeOldModeDatabase fills the old tables the way the removed mode left
// them on the owner's computer: a persistent Claude session holding its
// reservation, one finished turn, one queued message, a Codex owner and a
// prepared window message.
func writeOldModeDatabase(t *testing.T, path string) {
	t.Helper()
	mustOpen(t, path).Close()
	raw, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	for _, statement := range []string{
		`INSERT INTO sessions(key,provider,environment,conversation_id,active_token,cli_path,workspace,start_mode) VALUES ('s1','claude','/profile','11111111-1111-4111-8111-111111111111','t2','/bin/claude','/work','new')`,
		`INSERT INTO sessions(key,provider,environment,conversation_id,cli_path,workspace) VALUES ('s2','codex','/codex','thread-1','/bin/codex','/work2')`,
		`INSERT INTO aliases(alias,session_key) VALUES ('worker','s1'),('codex-test','s2')`,
		`INSERT INTO requests(id,session_key,sender,text,state,created_at) VALUES ('m1','s1','local-user','first','replied','2026-11-06T10:00:00Z'),('m2','s1','local-user','second','claimed','2026-11-06T10:01:00Z'),('m3','s2','worker','third','queued','2026-11-06T10:02:00Z')`,
		`INSERT INTO attempts(token,session_key,request_id,alias,state,pid,pgid,start_token,boot_id) VALUES ('t1','s1','m1','worker','completed',100,100,'st','boot'),('t2','s1','m2','worker','running',100,100,'st','boot')`,
		`INSERT INTO replies(id,reply_to,attempt_token,sender,recipient,text,held) VALUES ('r1','m1','t1','worker','local-user','answer',0)`,
		`INSERT INTO verifications(attempt_token,identity_confirmed,reply_id,exit_observed,exit_code,group_empty,provider_ok,detail) VALUES ('t1',1,'r1',0,NULL,0,1,'turn verified')`,
		`INSERT INTO stream_runs(id,session_key,state,current_token) VALUES ('t1','s1','running','t2')`,
		`INSERT INTO stream_turns(attempt_token,run_id,receipt,sent_intent,read_seen) VALUES ('t1','t1','rc1',1,1),('t2','t1','rc2',1,0)`,
		`INSERT INTO codex_setups(id,alias,environment,cli_path,workspace,state,thread_id,pid,pgid,start_token,boot_id,created_at,updated_at,session_key) VALUES ('c1','codex-test','/codex','/bin/codex','/work2','created','thread-1',200,200,'st2','boot','2026-11-06T09:00:00Z','2026-11-06T09:00:00Z','s2')`,
		`INSERT INTO ui_outbox(id,session_key,alias,text,state,created_at) VALUES ('ui-00000000-0000-4000-8000-000000000001','s1','worker','not sent yet','prepared','2026-11-06T10:03:00Z')`,
	} {
		if _, err := raw.Exec(statement); err != nil {
			t.Fatalf("%s: %v", statement, err)
		}
	}
}

// dumpTables renders every row of the given tables in a stable order.
func dumpTables(t *testing.T, path string, tables []string) map[string]string {
	t.Helper()
	raw, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	out := map[string]string{}
	for _, table := range tables {
		rows, err := raw.Query(`SELECT * FROM ` + table + ` ORDER BY rowid`)
		if err != nil {
			t.Fatalf("%s: %v", table, err)
		}
		cols, err := rows.Columns()
		if err != nil {
			t.Fatal(err)
		}
		var b strings.Builder
		for rows.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err := rows.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			for i, col := range cols {
				value := values[i]
				if bytes, ok := value.([]byte); ok {
					value = string(bytes)
				}
				fmt.Fprintf(&b, "%s=%v ", col, value)
			}
			b.WriteString("\n")
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		rows.Close()
		out[table] = b.String()
	}
	return out
}

func mustOpen(t *testing.T, path string) *Store {
	t.Helper()
	st, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func mkdir(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}
