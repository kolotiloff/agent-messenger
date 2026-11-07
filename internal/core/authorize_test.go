package core

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCheckAuthorizeSetupChangesNothing(t *testing.T) {
	s, env := openTest(t)
	cli := filepath.Join(t.TempDir(), exeName("claude"))
	if err := os.WriteFile(cli, []byte("never execute this test file"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.SetupNewClaude(env, "a78f1366-38e7-4cec-b0b2-98670cfd79b6", "test", cli, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	if err := s.Pause(); err != nil {
		t.Fatal(err)
	}
	before, _ := s.Snapshot()
	if err := s.CheckAuthorizeSetup("test"); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckAuthorizeSetup("other"); !errors.Is(err, ErrRejected) {
		t.Fatalf("unknown alias: %v", err)
	}
	if after, _ := s.Snapshot(); !reflect.DeepEqual(before, after) {
		t.Fatal("a check changed the store")
	}
	for alias, want := range map[string]bool{"test": true, "other": false} {
		if got, err := s.AliasInUse(alias); err != nil || got != want {
			t.Fatalf("%s: %v %v", alias, got, err)
		}
	}
}
