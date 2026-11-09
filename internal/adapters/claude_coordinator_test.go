package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestClaudeAddCoordinatorArgs(t *testing.T) {
	got := ClaudeAddCoordinatorArgs("/bin/claude", "/app/messenger", "/data", "My Game")
	want := []string{"/bin/claude", "mcp", "add", "--scope", "local", "messenger", "--", "/app/messenger", "--root", "/data", "coordinator", "--project", "My Game"}
	if len(got) != len(want) {
		t.Fatalf("%q", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%d: %q", i, got)
		}
	}
}

func TestClaudeUserConfig(t *testing.T) {
	home := t.TempDir()
	if got := ClaudeUserConfig(home, nil); got != filepath.Join(home, ".claude.json") {
		t.Fatal(got)
	}
	dir := filepath.Join(home, "other")
	if got := ClaudeUserConfig(home, []string{"CLAUDE_CONFIG_DIR=" + dir}); got != filepath.Join(dir, ".claude.json") {
		t.Fatal(got)
	}
	if got := ClaudeUserConfig(home, []string{"CLAUDE_CONFIG_DIR=relative"}); got != filepath.Join(home, ".claude.json") {
		t.Fatal(got)
	}
}

func TestCoordinatorLink(t *testing.T) {
	// Claude writes the real path of the folder it runs in; on macOS the
	// temporary folder itself sits behind a symlink.
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "game")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(dir, ".claude.json")
	write := func(key string, server any) {
		raw, _ := json.Marshal(map[string]any{"numStartups": 3, "projects": map[string]any{
			key:          map[string]any{"mcpServers": map[string]any{"messenger": server}},
			"/elsewhere": map[string]any{"mcpServers": map[string]any{}},
		}})
		if err := os.WriteFile(config, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	check := func(want string) {
		t.Helper()
		if got, _ := CoordinatorLink(config, folder, "/app/messenger", "/data", "Game"); got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	}
	check(LinkMissing)
	ours := map[string]any{"type": "stdio", "command": "/app/messenger", "args": CoordinatorArgs("/data", "Game"), "env": map[string]any{}}
	write(folder, ours)
	check(LinkConnected)
	write(filepath.Join(dir, "elsewhere"), ours)
	check(LinkMissing)
	write(folder, map[string]any{"type": "stdio", "command": "/app/messenger", "args": CoordinatorArgs("/data", "Other")})
	check(LinkOther)
	write(folder, map[string]any{"type": "stdio", "command": "/data/runtimes/old/messenger", "args": CoordinatorArgs("/data", "Game")})
	check(LinkStale)
	write(folder, map[string]any{"type": "http", "url": "https://example.invalid"})
	check(LinkOther)
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(folder, link); err != nil {
			t.Fatal(err)
		}
		write(folder, ours)
		if got, _ := CoordinatorLink(config, link, "/app/messenger", "/data", "Game"); got != LinkConnected {
			t.Fatal("a folder opened through a symlink was not matched")
		}
	}
	if err := os.WriteFile(config, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	check(LinkUnknown)
}
