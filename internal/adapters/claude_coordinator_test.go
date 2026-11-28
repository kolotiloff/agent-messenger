package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestClaudeAddCoordinatorArgs(t *testing.T) {
	got := ClaudeAddCoordinatorArgs("/bin/claude", "kotel", "/app/messenger", "/data", "My Game")
	want := []string{"/bin/claude", "mcp", "add", "--scope", "local", "kotel", "--", "/app/messenger", "--root", "/data", "coordinator", "--project", "My Game"}
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
	root := filepath.Join(dir, "data")
	program := filepath.Join(dir, "app", "messenger")
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
		if got, _ := CoordinatorLink(config, folder, program, root, "Game"); got != want {
			t.Fatalf("got %s, want %s", got, want)
		}
	}
	check(LinkMissing)
	ours := map[string]any{"type": "stdio", "command": program, "args": CoordinatorArgs(root, "Game"), "env": map[string]any{}}
	write(folder, ours)
	check(LinkConnected)
	write(filepath.Join(dir, "elsewhere"), ours)
	check(LinkMissing)
	write(folder, map[string]any{"type": "stdio", "command": program, "args": CoordinatorArgs(root, "Other")})
	check(LinkOther)
	write(folder, map[string]any{"type": "stdio", "command": filepath.Join(root, "runtimes", strings.Repeat("a", 64), "messenger"), "args": CoordinatorArgs(root, "Game")})
	check(LinkStale)
	write(folder, map[string]any{"command": "/custom", "args": CoordinatorArgs(root, "Game"), "env": map[string]string{"SETTING": "keep"}})
	check(LinkOther)
	write(folder, map[string]any{"type": "http", "url": "https://example.invalid"})
	check(LinkOther)
	if runtime.GOOS != "windows" {
		link := filepath.Join(dir, "link")
		if err := os.Symlink(folder, link); err != nil {
			t.Fatal(err)
		}
		write(folder, ours)
		if got, _ := CoordinatorLink(config, link, program, root, "Game"); got != LinkConnected {
			t.Fatal("a folder opened through a symlink was not matched")
		}
	}
	if err := os.WriteFile(config, []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	check(LinkUnknown)
}

func TestClaudeCoordinatorNameCollision(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")
	raw, err := json.Marshal(map[string]any{"projects": map[string]any{dir: map[string]any{"mcpServers": map[string]any{"messenger": map[string]string{"command": "old"}, "kotel": map[string]string{"command": "custom"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if state, name, err := ClaudeCoordinatorEntry(path, dir, "/kotel", "/data", "Game"); state != LinkOther || name != "" || err == nil {
		t.Fatal(state, name, err)
	}
}

func TestClaudeCoordinatorRespectsOtherScopesAndDisabledServers(t *testing.T) {
	tests := []struct {
		name     string
		user     string
		shared   string
		disabled []string
		managed  bool
		want     string
	}{
		{name: "fresh", want: LinkMissing},
		{name: "unrelated user server", user: `{"another":{"command":"custom"}}`, want: LinkMissing},
		{name: "user-scope current name", user: `{"kotel":{"command":"custom"}}`, want: LinkOther},
		{name: "user-scope legacy name", user: `{"messenger":{"command":"custom"}}`, want: LinkOther},
		{name: "project-scope current name", shared: `{"mcpServers":{"kotel":{"command":"custom"}}}`, want: LinkOther},
		{name: "project-scope legacy name", shared: `{"mcpServers":{"messenger":{"command":"custom"}}}`, want: LinkOther},
		{name: "duplicate shared map", shared: `{"mcpServers":{"kotel":{}}, "mcpServers":{}}`, want: LinkUnknown},
		{name: "disabled current name", disabled: []string{"kotel"}, want: LinkOff},
		{name: "disabled legacy name without local entry", disabled: []string{"messenger"}, want: LinkOff},
		{name: "disabled legacy managed entry", disabled: []string{"messenger"}, managed: true, want: LinkOff},
		{name: "unrelated disabled entry", disabled: []string{"other"}, managed: true, want: LinkStale},
		{name: "avoid exposing shadowed server during repair", managed: true, user: `{"messenger":{"command":"custom"}}`, want: LinkOther},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			folder := filepath.Join(dir, "project", "nested")
			if err := os.MkdirAll(folder, 0o700); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(dir, "data")
			app := filepath.Join(dir, "kotel")
			userConfig := filepath.Join(dir, ".claude.json")
			projectSettings := map[string]any{}
			if tc.disabled != nil {
				projectSettings["disabledMcpServers"] = tc.disabled
			}
			if tc.managed {
				projectSettings["mcpServers"] = map[string]any{
					"messenger": map[string]any{
						"command": filepath.Join(root, "runtimes", strings.Repeat("a", 64), "messenger"),
						"args":    CoordinatorArgs(root, "Game"),
					},
				}
			}
			settings := map[string]any{"projects": map[string]any{folder: projectSettings}}
			if tc.user != "" {
				settings["mcpServers"] = json.RawMessage(tc.user)
			}
			raw, err := json.Marshal(settings)
			if err != nil {
				t.Fatal(err)
			}
			// Claude also sees project-scope servers from ancestor folders. The
			// user config may be absent when a shared server is already present.
			if tc.user != "" || len(projectSettings) > 0 {
				if err := os.WriteFile(userConfig, raw, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if tc.shared != "" {
				if err := os.WriteFile(filepath.Join(filepath.Dir(folder), ".mcp.json"), []byte(tc.shared), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			if got, _, err := ClaudeCoordinatorEntry(userConfig, folder, app, root, "Game"); got != tc.want {
				t.Fatalf("state=%s, want %s, detail=%v", got, tc.want, err)
			}
		})
	}
}

func TestCoordinatorProgramAliasRequiresInstallerLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Mac and Linux packages use sibling symlinks")
	}
	for _, owner := range []string{"messenger", "kotel"} {
		dir := t.TempDir()
		old, current := filepath.Join(dir, "messenger"), filepath.Join(dir, "kotel")
		target, link := current, old
		if owner == "messenger" {
			target, link = old, current
		}
		if err := os.WriteFile(target, []byte("program"), 0700); err != nil {
			t.Fatal(err)
		}
		if coordinatorProgramAlias(old, current) {
			t.Fatal("accepted missing sibling")
		}
		if err := os.Symlink(filepath.Base(target), link); err != nil {
			t.Fatal(err)
		}
		if !coordinatorProgramAlias(old, current) {
			t.Fatal("installer alias was not recognized")
		}
	}
}
