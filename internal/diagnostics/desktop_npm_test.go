package diagnostics

import (
	"os"
	"path/filepath"
	"testing"
)

// Both npm layouts of Codex on Windows: vendor/<target>/codex and the newer
// vendor/<target>/bin, seen on the owner's computer.
func TestNpmCodexCandidateLayouts(t *testing.T) {
	for _, folder := range []string{"codex", "bin"} {
		npm := t.TempDir()
		exe := filepath.Join(npm, "node_modules", "@openai", "codex", "node_modules", "@openai", "codex-win32-x64",
			"vendor", "x86_64-pc-windows-msvc", folder, "codex.exe")
		if err := os.MkdirAll(filepath.Dir(exe), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(exe, []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		r := npmCodexCandidate(Report{Agents: []Agent{{Provider: "codex", CLI: CLI{State: "not_found"}}}}, npm)
		if a := r.Agents[0]; a.CLI.State != "candidate_found" || a.CLI.Path != exe {
			t.Fatal(folder, a.CLI)
		}
	}
}

func TestCursorNodeCandidate(t *testing.T) {
	versions := filepath.Join(t.TempDir(), "cursor-agent", "versions")
	dir := filepath.Join(versions, "2026.11.05-86047df")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"node.exe", "index.js"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	r := cursorNodeCandidate(Report{Agents: []Agent{{Provider: "cursor", CLI: CLI{State: "not_found"}}}}, versions)
	if a := r.Agents[0]; a.CLI.State != "candidate_found" || a.CLI.Path != filepath.Join(dir, "node.exe") {
		t.Fatal(a.CLI)
	}
}
