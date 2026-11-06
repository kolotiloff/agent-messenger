package supervisor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

func TestSetupCreatesWorkspaceWithoutAuthorizationOrQueue(t *testing.T) {
	root := t.TempDir()
	store, err := core.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	cli := filepath.Join(root, "cli")
	if err := os.WriteFile(cli, []byte("never execute"), 0700); err != nil {
		t.Fatal(err)
	}
	s := &Supervisor{store: store, root: root, launcher: AdapterLauncher{ClaudeStream: true}}
	const id = "a78f1366-38e7-4cec-b0b2-98670cfd79b6"
	for i := 0; i < 2; i++ {
		if err := s.SetupNewClaude(root, id, "test", cli, ""); err != nil {
			t.Fatal(err)
		}
	}
	status, err := s.Status()
	if err != nil || !status.SessionSetup || status.CLILaunch != "not-authorized" || status.Queued != 0 || status.Held != 0 {
		t.Fatal(status, err)
	}
	info, err := os.Stat(filepath.Join(root, "workspaces", id))
	if err != nil || !info.IsDir() {
		t.Fatal("isolated workspace missing", err)
	}
	if err := s.SetupNewClaude(root, "../escape", "test", cli, ""); err == nil {
		t.Fatal("path traversal accepted")
	}
	if err := s.SetupNewClaude(root, id, "test", cli, root); err == nil {
		t.Fatal("external workspace accepted")
	}
	s.stopping.Store(true)
	if err := s.SetupNewClaude(root, id, "test", cli, ""); err == nil {
		t.Fatal("setup accepted during shutdown")
	}
}

func TestSetupRejectsSymlinkWorkspaceParent(t *testing.T) {
	root := t.TempDir()
	store, err := core.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := os.Symlink(t.TempDir(), filepath.Join(root, "workspaces")); err != nil {
		t.Skip(err)
	}
	s := &Supervisor{store: store, root: root, launcher: AdapterLauncher{ClaudeStream: true}}
	if err := s.SetupNewClaude(root, "a78f1366-38e7-4cec-b0b2-98670cfd79b6", "test", "/missing", ""); err == nil {
		t.Fatal("symlink parent accepted")
	}
	snap, _ := store.Snapshot()
	if len(snap.Sessions) != 0 {
		t.Fatal("partial setup after rejected workspace")
	}
}
