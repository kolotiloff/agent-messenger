package core

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// T-207: `codex update` moves the command link to a new release; the next
// task must run the new release, not the file the link pointed at when the
// agent was connected.
func TestAgentFollowsItsCommandLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("command links are a macOS and Linux install")
	}
	f := newTaskFixture(t)
	// macOS keeps temporary folders behind /var -> /private/var.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	release := func(version string) string {
		p := filepath.Join(dir, "releases", version, "codex")
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
		return p
	}
	old, next := release("0.155.1"), release("0.170.0")
	bin := filepath.Join(dir, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(bin, "codex")
	if err := os.Symlink(old, link); err != nil {
		t.Fatal(err)
	}
	saved := agentLinkDirs
	agentLinkDirs = func() []string { return []string{bin} }
	defer func() { agentLinkDirs = saved }()
	agentCLI := func() string {
		agents, _ := f.st.ComputerAgents()
		for _, a := range agents {
			if a.Provider == "codex" {
				return a.CLI
			}
		}
		return ""
	}
	// Connected through its link: the link is kept.
	if err := f.st.SetComputerAgent("codex", link, "", "messenger-only"); err != nil {
		t.Fatal(err)
	}
	if got := agentCLI(); got != link {
		t.Fatalf("stored %q, want the link", got)
	}
	// An agent stored before the fix kept the resolved file; the next task
	// takes the link that still points there.
	if _, err := f.st.db.Exec(`UPDATE computer_agents SET cli_path=? WHERE provider='codex'`, old); err != nil {
		t.Fatal(err)
	}
	task, _, err := f.st.StartTask(StartTaskRequest{Project: "Game", OpID: "op-link", Provider: "codex", Text: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := f.st.TaskByID(task); got.CLI != link || agentCLI() != link {
		t.Fatalf("task CLI %q, agent %q", got.CLI, agentCLI())
	}
	// After the update the link points at the new release; it runs that.
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(next, link); err != nil {
		t.Fatal(err)
	}
	if resolved, _ := filepath.EvalSymlinks(agentCLI()); resolved != next {
		t.Fatalf("link resolves to %q", resolved)
	}
	// A file no link points at stays as it is.
	if got := agentLinkFor("codex", old); got != "" {
		t.Fatalf("unlinked file replaced by %q", got)
	}
}
