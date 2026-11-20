package mcp

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

func TestCoordinatorAgentVersion(t *testing.T) {
	st := newCoordinatorStore(t)
	c := startCoordinator(t, st, "Game")
	var result core.AgentVersion
	if _, ok := c.tool("agent_version", map[string]any{"agent": "cursor"}, &result); !ok || result.State != "unknown" || result.Reason != "agent_unavailable" || result.Version != "" {
		t.Fatalf("missing agent: %+v (%v)", result, ok)
	}
	for _, args := range []map[string]any{
		{"agent": "anything"},
		{"agent": "codex", "command": "--help"},
		{"agent": "codex", "computer": "unknown"},
		{"agent": "codex", "request_id": "version-unknown"},
		{"request_id": "version-unknown"},
	} {
		if _, ok := c.tool("agent_version", args, nil); ok {
			t.Fatalf("accepted invalid request: %+v", args)
		}
	}
	if runtime.GOOS == "windows" {
		return // The fixed command is exercised with a POSIX fixture below.
	}
	dir := t.TempDir()
	binary := filepath.Join(dir, "codex-current")
	link := filepath.Join(dir, "codex")
	writeVersion := func(path, version string) {
		t.Helper()
		script := "#!/bin/sh\n[ \"$#\" -eq 1 ] && [ \"$1\" = '--version' ] || exit 9\nprintf '%s\\n' 'codex-cli " + version + "'\n"
		if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	writeVersion(binary, "0.155.1")
	if err := os.Symlink(binary, link); err != nil {
		t.Fatal(err)
	}
	if err := st.SetComputerAgent("codex", link, "", "messenger-only"); err != nil {
		t.Fatal(err)
	}
	check := func(want string) {
		t.Helper()
		result = core.AgentVersion{}
		if _, ok := c.tool("agent_version", map[string]any{"agent": "codex"}, &result); !ok || result.State != "reported" || result.Version != want || result.CheckedAt == "" {
			t.Fatalf("local version: %+v (%v)", result, ok)
		}
	}
	check("0.155.1")
	replacement := filepath.Join(dir, "codex-next")
	writeVersion(replacement, "0.156.0-alpha.1")
	newLink := link + ".new"
	if err := os.Symlink(replacement, newLink); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(newLink, link); err != nil {
		t.Fatal(err)
	}
	check("0.156.0-alpha.1")
}
