package core

import (
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestNewSetupIsAtomicIdempotentAndNeverRebinds(t *testing.T) {
	s, env := openTest(t)
	cli := filepath.Join(t.TempDir(), exeName("claude"))
	if err := os.WriteFile(cli, []byte("never execute this test file"), 0700); err != nil {
		t.Fatal(err)
	}
	workspace := t.TempDir()
	const id = "a78f1366-38e7-4cec-b0b2-98670cfd79b6"
	before, _ := s.Snapshot()
	if err := s.SetupNewClaude(env, id, "test", cli, filepath.Join(workspace, "missing")); err == nil {
		t.Fatal("missing workspace accepted")
	}
	after, _ := s.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed setup partially registered session")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := s.SetupNewClaude(env, id, "test", cli, workspace); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	first, _ := s.Snapshot()
	if len(first.Sessions) != 1 || len(first.Events) != 1 || len(first.Requests) != 0 || len(first.Attempts) != 0 || first.Sessions[0]["start_mode"] != "new" {
		t.Fatalf("setup did more than register: %+v", first)
	}
	for _, change := range []struct{ id, alias, cli, workspace string }{
		{id, "another-alias", cli, workspace},
		{"f4c73f28-f8e4-40f1-8d89-511c2076b470", "test", cli, workspace},
		{id, "test", cli, t.TempDir()},
	} {
		if err := s.SetupNewClaude(env, change.id, change.alias, change.cli, change.workspace); err == nil {
			t.Fatal("rebind accepted")
		}
	}
	after, _ = s.Snapshot()
	if !reflect.DeepEqual(first, after) {
		t.Fatal("rejected setup changed bindings")
	}
	if _, err := s.Enqueue("m1", "test", "user", "payload"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Claim("test"); err != nil {
		t.Fatal(err)
	}
	claimed, _ := s.Snapshot()
	if err := s.SetupNewClaude(env, id, "test", cli, workspace); err != nil {
		t.Fatal("identical ACK retry failed", err)
	}
	after, _ = s.Snapshot()
	if !reflect.DeepEqual(claimed, after) {
		t.Fatal("ACK retry changed live reservation")
	}
}

func TestNewSetupCannotClaimExistingSession(t *testing.T) {
	s, env := openTest(t)
	const id = "a78f1366-38e7-4cec-b0b2-98670cfd79b6"
	cli := filepath.Join(t.TempDir(), "claude")
	if err := os.WriteFile(cli, []byte("fixture"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Register("claude", env, id, "old"); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Snapshot()
	if err := s.SetupNewClaude(env, id, "old", cli, t.TempDir()); err == nil {
		t.Fatal("old alias accepted")
	}
	if err := s.SetupNewClaude(env, id, "new", cli, t.TempDir()); err == nil {
		t.Fatal("old session accepted through new alias")
	}
	after, _ := s.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("existing session mutated")
	}
}
