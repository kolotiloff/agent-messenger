package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
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
	write(filepath.Join(home, ".claude.json"), `{"projects":{`+jsonKey(filepath.ToSlash(filepath.Join(home, "Projects")))+`:{"hasTrustDialogAccepted":true}}}`)
	if got := CheckFolderTrust("claude", folder, "", home).State; got != TrustYes {
		t.Fatalf("trusted parent: %s", got)
	}
	write(filepath.Join(home, ".claude.json"), `{"projects":{`+jsonKey(folder)+`:{"hasTrustDialogAccepted":false}}}`)
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

func jsonKey(s string) string {
	raw, _ := json.Marshal(s)
	return string(raw)
}

// Codex trusts a project by the folder or by the root of its git repository,
// and its config may write the entry in any TOML form.
func TestCodexTrustForms(t *testing.T) {
	home := t.TempDir()
	repo := filepath.Join(home, "Game")
	sub := filepath.Join(repo, "Client")
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	quoted, _ := json.Marshal(repo)
	write := func(config string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(config), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for name, config := range map[string]string{
		"table":        "model = \"x\"\n[projects." + string(quoted) + "]\ntrust_level = \"trusted\"\n",
		"inline":       "[projects]\n" + string(quoted) + " = { trust_level = \"trusted\" }\n",
		"dotted":       "projects." + string(quoted) + ".trust_level = \"trusted\"\n",
		"inline-table": "[projects." + string(quoted) + "]\n# trusted in the app\ntrust_level = 'trusted'\n",
	} {
		write(config)
		for _, folder := range []string{repo, sub} {
			if got := CheckFolderTrust("codex", folder, "", home).State; got != TrustYes {
				t.Fatalf("%s %s: %s", name, folder, got)
			}
		}
	}
	write("[projects." + string(quoted) + "]\ntrust_level = \"untrusted\"\n")
	if got := CheckFolderTrust("codex", sub, "", home).State; got != TrustNo {
		t.Fatalf("untrusted: %s", got)
	}
	// Another letter case of the same folder (macOS and Windows ignore case).
	if runtime.GOOS != "linux" {
		upper, _ := json.Marshal(strings.ToUpper(repo))
		write("[projects." + string(upper) + "]\ntrust_level = \"trusted\"\n")
		if got := CheckFolderTrust("codex", repo, "", home).State; got != TrustYes {
			t.Fatalf("case: %s", got)
		}
	}
}
