package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServeFlags(t *testing.T) {
	o, err := serveOptions(nil)
	if err != nil || o.AllowTasks {
		t.Fatalf("default = %+v %v", o, err)
	}
	o, err = serveOptions([]string{"--allow-tasks"})
	if err != nil || !o.AllowTasks {
		t.Fatalf("opt-in = %+v %v", o, err)
	}
	// An older program may still pass the removed mode's flag; it changes
	// nothing.
	o, err = serveOptions([]string{"--claude-stream", "--allow-tasks"})
	if err != nil || !o.AllowTasks {
		t.Fatalf("legacy flag = %+v %v", o, err)
	}
	for _, args := range [][]string{{"--root"}, {"--allow-model-calls"}, {"--only-session=worker"}, {"--only-session", "worker"}, {"--claude-stream", "--allow-model-calls"}} {
		if _, err := serveOptions(args); err == nil {
			t.Fatalf("accepted %q", args)
		}
	}
}

func TestRootFlagWorksBeforeOrAfterCommand(t *testing.T) {
	root, args := splitArgs([]string{"--root", "/tmp/data", "serve"})
	if root != "/tmp/data" || len(args) != 1 || args[0] != "serve" {
		t.Fatalf("before = %q %q", root, args)
	}
	root, args = splitArgs([]string{"serve", "--root", "/tmp/data"})
	if root != "/tmp/data" || len(args) != 1 || args[0] != "serve" {
		t.Fatalf("after = %q %q", root, args)
	}
	root, args = splitArgs([]string{"status", "--root", "/tmp/other"})
	if root != "/tmp/other" || args[0] != "status" {
		t.Fatalf("status = %q %q", root, args)
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root, _ = splitArgs([]string{"--root", "data", "status"})
	if root != filepath.Join(cwd, "data") {
		t.Fatalf("relative root = %q", root)
	}
}

// The removed mode's commands explain themselves and never reach the client.
func TestRemovedCommandsExplainThemselves(t *testing.T) {
	for _, cmd := range []string{"mcp", "timeline", "overview", "inspect", "enqueue", "resolve", "stop-job", "register-session", "create-claude-session", "create-codex-session", "setup-codex", "recover-stream", "recover-codex", "recover-failed"} {
		err := run(t.TempDir(), []string{cmd, "--alias", "worker"})
		if err == nil || !strings.Contains(err.Error(), "T-028") {
			t.Fatalf("%s = %v", cmd, err)
		}
	}
	if err := run(t.TempDir(), []string{"status", "extra"}); err == nil {
		t.Fatal("status accepted an argument")
	}
}
