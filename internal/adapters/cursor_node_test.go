package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cursorVersion(t *testing.T, versions, name string) string {
	t.Helper()
	dir := filepath.Join(versions, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"node.exe", "index.js"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// The layout seen on the owner's Windows computer (D-052).
func TestCursorProgramUsesTheNewestVersion(t *testing.T) {
	versions := filepath.Join(t.TempDir(), "cursor-agent", "versions")
	old := cursorVersion(t, versions, "2026.11.05-86047df")
	newer := cursorVersion(t, versions, "2026.12.1-12-30-00-abc1234")
	if err := os.MkdirAll(filepath.Join(versions, "not-a-version"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := LatestCursorVersion(versions); got != newer {
		t.Fatal(got)
	}
	got := CursorProgram(filepath.Join(old, "node.exe"))
	if len(got) != 2 || got[0] != filepath.Join(newer, "node.exe") || got[1] != filepath.Join(newer, "index.js") {
		t.Fatal(got)
	}
	spec := taskSpec("cursor", ProfileWorker, "", false)
	spec.CLI = filepath.Join(old, "node.exe")
	args, err := TaskArgs(spec)
	if err != nil || args[0] != filepath.Join(newer, "node.exe") || args[1] != filepath.Join(newer, "index.js") || args[2] != "-p" {
		t.Fatal(args, err)
	}
	if env := strings.Join(TaskEnv(spec), "\n"); !strings.Contains(env, "CURSOR_INVOKED_AS=agent.cmd") || !strings.Contains(env, TaskEnvVariable+"=") {
		t.Fatal(env)
	}
}

// Any other program, and a node.exe outside a Cursor version folder, is
// started as it is.
func TestCursorProgramLeavesOtherPathsAlone(t *testing.T) {
	dir := t.TempDir()
	other := filepath.Join(dir, "node.exe")
	if err := os.WriteFile(filepath.Join(dir, "index.js"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, cli := range []string{"/usr/local/bin/cursor-agent", other} {
		if got := CursorProgram(cli); len(got) != 1 || got[0] != cli {
			t.Fatal(got)
		}
		if env := cursorProgramEnv(cli); env != nil {
			t.Fatal(env)
		}
	}
}
