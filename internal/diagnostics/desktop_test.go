package diagnostics

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopCandidatesArePassiveAndPreservePATHChoice(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"claude", "codex", "agent"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("this file must never be executed"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	r := Report{Agents: []Agent{
		{Provider: "claude", CLI: CLI{State: "not_found", Identity: "not_checked", Version: "not_checked"}},
		{Provider: "codex", CLI: CLI{State: "candidate_found", Path: "/chosen/codex", Source: "PATH"}},
		{Provider: "cursor", CLI: CLI{State: "not_found"}},
	}}
	r = desktopCandidates(r, []string{"relative", dir})
	if r.Agents[0].CLI.Path != filepath.Join(dir, "claude") || r.Agents[0].CLI.Identity != "not_checked" || r.Agents[1].CLI.Path != "/chosen/codex" || r.Agents[2].CLI.Command != "agent" || r.ProbeCLIStarts != 0 {
		t.Fatalf("bad discovery %+v", r)
	}
}
