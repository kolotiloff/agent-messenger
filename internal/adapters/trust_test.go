package adapters

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFolderTrust(t *testing.T) {
	home := t.TempDir()
	folder := filepath.Join(home, "Projects", "Game")
	if got := CheckFolderTrust("claude", folder, "", home).State; got != TrustNo {
		t.Fatalf("no Claude config: %s", got)
	}
	write := func(path, text string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(home, ".claude.json"), `{"projects":{"`+filepath.Join(home, "Projects")+`":{"hasTrustDialogAccepted":true}}}`)
	if got := CheckFolderTrust("claude", folder, "", home).State; got != TrustYes {
		t.Fatalf("trusted parent: %s", got)
	}
	write(filepath.Join(home, ".claude.json"), `{"projects":{"`+folder+`":{"hasTrustDialogAccepted":false}}}`)
	if got := CheckFolderTrust("claude", folder, "", home).State; got != TrustNo {
		t.Fatalf("declined: %s", got)
	}
	write(filepath.Join(home, ".claude.json"), `not json`)
	if got := CheckFolderTrust("claude", folder, "", home).State; got != TrustUnknown {
		t.Fatalf("broken config: %s", got)
	}
	profile := filepath.Join(home, "codex-profile")
	write(filepath.Join(profile, "config.toml"), "model = \"x\"\n[projects.\""+folder+"\"]\ntrust_level = \"trusted\"\n[other]\ntrust_level = \"untrusted\"\n")
	if got := CheckFolderTrust("codex", folder, profile, home).State; got != TrustYes {
		t.Fatalf("codex trusted: %s", got)
	}
	if got := CheckFolderTrust("codex", filepath.Join(home, "Other"), profile, home).State; got != TrustNo {
		t.Fatalf("codex other folder: %s", got)
	}
	if got := CheckFolderTrust("codex", folder, "", home).State; got != TrustNo {
		t.Fatalf("codex without config: %s", got)
	}
	if h := CheckFolderTrust("codex", folder, "", home).Hint; h == "" {
		t.Fatal("no hint")
	}
}
