package supervisor

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// fakeClaudeMCP stands in for `claude mcp add --scope local NAME -- CMD ARGS`.
// Like Claude, it writes the server under the folder it runs in, into
// ~/.claude.json, and keeps everything else in that file.
func fakeClaudeMCP(mode string) int {
	cwd, _ := os.Getwd()
	if log := os.Getenv("MESSENGER_CLAUDE_MCP_LOG"); log != "" {
		f, _ := os.OpenFile(log, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		fmt.Fprintf(f, "%s\x00%s\n", cwd, strings.Join(os.Args[1:], "\x00"))
		f.Close()
	}
	switch mode {
	case "fail":
		fmt.Fprintln(os.Stderr, "MCP server messenger already exists in local config")
		return 1
	case "silent":
		return 0
	}
	a := os.Args[1:]
	if len(a) == 5 && a[0] == "mcp" && a[1] == "remove" && a[2] == "--scope" && a[3] == "local" {
		home, _ := os.UserHomeDir()
		path := filepath.Join(home, ".claude.json")
		settings := map[string]any{}
		raw, _ := os.ReadFile(path)
		_ = json.Unmarshal(raw, &settings)
		projects, _ := settings["projects"].(map[string]any)
		project, _ := projects[cwd].(map[string]any)
		servers, _ := project["mcpServers"].(map[string]any)
		if servers[a[4]] == nil {
			fmt.Fprintln(os.Stderr, "No MCP server found")
			return 1
		}
		delete(servers, a[4])
		raw, _ = json.MarshalIndent(settings, "", "  ")
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			return 3
		}
		return 0
	}
	if len(a) < 7 || a[0] != "mcp" || a[1] != "add" || a[2] != "--scope" || a[3] != "local" || a[5] != "--" {
		fmt.Fprintln(os.Stderr, "unexpected arguments")
		return 2
	}
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude.json")
	settings := map[string]any{}
	if raw, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(raw, &settings)
	}
	projects, _ := settings["projects"].(map[string]any)
	if projects == nil {
		projects = map[string]any{}
	}
	project, _ := projects[cwd].(map[string]any)
	if project == nil {
		project = map[string]any{}
	}
	servers, _ := project["mcpServers"].(map[string]any)
	if servers == nil {
		servers = map[string]any{}
	}
	servers[a[4]] = map[string]any{"type": "stdio", "command": a[6], "args": a[7:], "env": map[string]any{}}
	project["mcpServers"], projects[cwd], settings["projects"] = servers, project, projects
	raw, _ := json.MarshalIndent(settings, "", "  ")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return 3
	}
	fmt.Println("Added stdio MCP server messenger to local config")
	return 0
}

type linkFixture struct {
	sup             *Supervisor
	folder, home    string
	log, messenger  string
	project, config string
}

func newLinkFixture(t *testing.T, withClaude bool) linkFixture {
	t.Helper()
	isolateUser(t)
	home, _ := os.UserHomeDir()
	root := t.TempDir()
	st := mustStore(t, root)
	old := core.TemporaryFolder
	core.TemporaryFolder = func(string) bool { return false }
	t.Cleanup(func() { core.TemporaryFolder = old })
	folder, err := filepath.EvalSymlinks(mkdir(t, filepath.Join(t.TempDir(), "game")))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CreateProject("Game", folder); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	if withClaude {
		if err := st.SetComputerAgent("claude", exe, "", "worker"); err != nil {
			t.Fatal(err)
		}
	}
	f := linkFixture{sup: &Supervisor{store: st, root: root}, folder: folder, home: home, messenger: exe, project: "Game",
		log: filepath.Join(t.TempDir(), "calls"), config: filepath.Join(home, ".claude.json")}
	t.Setenv("MESSENGER_CLAUDE_MCP_LOG", f.log)
	// Background repairs end before the store closes.
	t.Cleanup(f.sup.bg.Wait)
	return f
}

func (f linkFixture) calls(t *testing.T) []string {
	raw, err := os.ReadFile(f.log)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSpace(string(raw)), "\n")
}

func TestConnectCoordinatorRunsClaudeInTheProjectFolder(t *testing.T) {
	f := newLinkFixture(t, true)
	t.Setenv("MESSENGER_CLAUDE_MCP_FIXTURE", "ok")
	// The owner's other settings stay as they are.
	if err := os.WriteFile(f.config, []byte(`{"numStartups":7,"projects":{"/other":{"mcpServers":{"github":{"command":"gh"}}}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := f.sup.CoordinatorLinkState(f.project); got.State != adapters.LinkMissing {
		t.Fatalf("before: %+v", got)
	}
	if got := f.sup.connectCoordinator(f.project); got.State != adapters.LinkConnected {
		t.Fatalf("connect: %+v", got)
	}
	calls := f.calls(t)
	if len(calls) != 1 {
		t.Fatalf("calls %q", calls)
	}
	parts := strings.Split(calls[0], "\x00")
	want := append([]string{"mcp", "add", "--scope", "local", "messenger", "--", f.messenger}, adapters.CoordinatorArgs(f.sup.root, f.project)...)
	if real, _ := filepath.EvalSymlinks(f.folder); parts[0] != real && parts[0] != f.folder || strings.Join(parts[1:], "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("call %q", parts)
	}
	raw, _ := os.ReadFile(f.config)
	if !strings.Contains(string(raw), `"numStartups": 7`) || !strings.Contains(string(raw), `"github"`) {
		t.Fatalf("other settings lost: %s", raw)
	}
	// Connected already: Claude is not run again.
	if got := f.sup.connectCoordinator(f.project); got.State != adapters.LinkConnected || len(f.calls(t)) != 1 {
		t.Fatalf("second connect: %+v %d", got, len(f.calls(t)))
	}
	view, err := f.sup.WindowView(f.project)
	if err != nil {
		t.Fatal(err)
	}
	if link := view.(windowView).CoordinatorLink; link == nil || link.State != adapters.LinkConnected {
		t.Fatalf("view: %+v", link)
	}
}

func TestConnectCoordinatorNeedsClaude(t *testing.T) {
	f := newLinkFixture(t, false)
	t.Setenv("MESSENGER_CLAUDE_MCP_FIXTURE", "ok")
	if got := f.sup.connectCoordinator(f.project); got.State != linkNoClaude || len(f.calls(t)) != 0 {
		t.Fatalf("%+v", got)
	}
}

func TestConnectCoordinatorNeverReplacesAnotherEntry(t *testing.T) {
	f := newLinkFixture(t, true)
	t.Setenv("MESSENGER_CLAUDE_MCP_FIXTURE", "ok")
	other := map[string]any{"projects": map[string]any{f.folder: map[string]any{"mcpServers": map[string]any{
		"messenger": map[string]any{"type": "stdio", "command": "/somewhere/else", "args": []string{"coordinator", "--project", "Game"}},
	}}}}
	raw, _ := json.Marshal(other)
	if err := os.WriteFile(f.config, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := f.sup.connectCoordinator(f.project); got.State != adapters.LinkOther || len(f.calls(t)) != 0 {
		t.Fatalf("%+v", got)
	}
	if after, _ := os.ReadFile(f.config); string(after) != string(raw) {
		t.Fatal("another entry was changed")
	}
}

// T-117: an entry this Messenger made for an earlier program file, such as
// the client's copy of an old version, is replaced by one that starts the
// program file the owner installed.
func TestStaleCoordinatorEntryIsReplaced(t *testing.T) {
	f := newLinkFixture(t, true)
	t.Setenv("MESSENGER_CLAUDE_MCP_FIXTURE", "ok")
	program := filepath.Join(t.TempDir(), "messenger")
	if err := os.WriteFile(program, []byte("program"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := WriteProgramPath(f.sup.root, program); err != nil {
		t.Fatal(err)
	}
	stale := map[string]any{"numStartups": 2, "projects": map[string]any{f.folder: map[string]any{"mcpServers": map[string]any{
		"messenger": map[string]any{"type": "stdio", "command": filepath.Join(f.sup.root, "runtimes", "old", "messenger"), "args": adapters.CoordinatorArgs(f.sup.root, f.project)},
		"other":     map[string]any{"command": "keep"},
	}}}}
	raw, _ := json.Marshal(stale)
	if err := os.WriteFile(f.config, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := f.sup.CoordinatorLinkState(f.project); got.State != adapters.LinkStale {
		t.Fatalf("before: %+v", got)
	}
	if got := f.sup.connectCoordinator(f.project); got.State != adapters.LinkConnected {
		t.Fatalf("repair: %+v", got)
	}
	calls := f.calls(t)
	if len(calls) != 2 || !strings.Contains(calls[0], "remove") || !strings.Contains(calls[1], "\x00"+program+"\x00") {
		t.Fatalf("calls %q", calls)
	}
	after, _ := os.ReadFile(f.config)
	if !strings.Contains(string(after), `"keep"`) {
		t.Fatalf("another server was lost: %s", after)
	}
}

func TestConnectCoordinatorReportsAndNeverRetries(t *testing.T) {
	for _, mode := range []string{"fail", "silent"} {
		t.Run(mode, func(t *testing.T) {
			f := newLinkFixture(t, true)
			t.Setenv("MESSENGER_CLAUDE_MCP_FIXTURE", mode)
			got := f.sup.connectCoordinator(f.project)
			if got.State != linkFailed || got.Detail == "" || len(f.calls(t)) != 1 {
				t.Fatalf("%+v calls %d", got, len(f.calls(t)))
			}
			if mode == "fail" && !strings.Contains(got.Detail, "already exists") {
				t.Fatalf("Claude's message lost: %q", got.Detail)
			}
		})
	}
}
