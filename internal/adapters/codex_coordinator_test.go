package adapters

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexCoordinatorBlockQuotesPaths(t *testing.T) {
	got := CodexCoordinatorBlock(`C:\Program Files\messenger`, `D:\data`, "My Game")
	for _, want := range []string{
		`command = "C:\\Program Files\\messenger"`,
		`args = ["--root", "D:\\data", "coordinator", "--project", "My Game"]`,
		`default_tools_approval_mode = "approve"`,
		"tool_timeout_sec = 180",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %s in\n%s", want, got)
		}
	}
}

func TestCodexCoordinatorLinkAndWrite(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "game")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	path := CodexProjectConfig(folder)
	messenger := filepath.Join(dir, "messenger")
	const root, project = "/data", "Game"
	if got, err := CodexCoordinatorLink(path, messenger, root, project); err != nil || got != LinkMissing {
		t.Fatalf("missing: %s %v", got, err)
	}
	original := "model = \"gpt\"\n\n[mcp_servers.docs]\ncommand = \"docs\"\nargs = [\"--serve\"]\n"
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCodexCoordinator(path, messenger, root, project); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), original+"\n") {
		t.Fatalf("other settings changed:\n%s", raw)
	}
	if !strings.Contains(string(raw), "command = "+tomlBasic(messenger)) {
		t.Fatalf("command missing:\n%s", raw)
	}
	if got, _ := CodexCoordinatorLink(path, messenger, root, project); got != LinkConnected {
		t.Fatalf("after write: %s", got)
	}
	before := string(raw)
	if err := EnsureCodexCoordinator(path, messenger, root, project); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != before {
		t.Fatal("a connected entry was rewritten")
	}
}

func TestCodexCoordinatorStaleIsReplaced(t *testing.T) {
	dir := t.TempDir()
	path := CodexProjectConfig(dir)
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "old")
	messenger := filepath.Join(dir, "messenger")
	body := "keep = 1\n[mcp_servers.messenger]\ncommand = " + tomlBasic(old) + "\nargs = [\"--root\", \"/data\", \"coordinator\", \"--project\", \"Game\"]\n\n[other]\nx = 1\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := CodexCoordinatorLink(path, messenger, "/data", "Game"); got != LinkStale {
		t.Fatalf("stale: %s", got)
	}
	if err := EnsureCodexCoordinator(path, messenger, "/data", "Game"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	text := string(raw)
	if !strings.Contains(text, "keep = 1") || !strings.Contains(text, "[other]") || !strings.Contains(text, "x = 1") {
		t.Fatalf("other settings lost:\n%s", text)
	}
	if strings.Contains(text, tomlBasic(old)) || !strings.Contains(text, tomlBasic(messenger)) {
		t.Fatalf("program not replaced:\n%s", text)
	}
	if got, _ := CodexCoordinatorLink(path, messenger, "/data", "Game"); got != LinkConnected {
		t.Fatal(got)
	}
}

func TestCodexCoordinatorLeavesAnotherEntry(t *testing.T) {
	dir := t.TempDir()
	path := CodexProjectConfig(dir)
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "[mcp_servers.messenger]\ncommand = \"/somewhere/else\"\nargs = [\"other\"]\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := CodexCoordinatorLink(path, "/app/messenger", "/data", "Game"); got != LinkOther {
		t.Fatalf("other: %s", got)
	}
	if err := EnsureCodexCoordinator(path, "/app/messenger", "/data", "Game"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != body {
		t.Fatalf("another entry changed:\n%s", after)
	}
}

func TestCodexCoordinatorOffIsNotEnabled(t *testing.T) {
	dir := t.TempDir()
	path := CodexProjectConfig(dir)
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	messenger := "/app/messenger"
	body := "[mcp_servers.messenger]\ncommand = " + tomlBasic(messenger) + "\nargs = [\"--root\", \"/data\", \"coordinator\", \"--project\", \"Game\"]\nenabled = false\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := CodexCoordinatorLink(path, messenger, "/data", "Game"); got != LinkOff {
		t.Fatal(got)
	}
	if err := EnsureCodexCoordinator(path, messenger, "/data", "Game"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != body {
		t.Fatal("a turned-off entry was enabled")
	}
}

func TestCodexCoordinatorUnreadableIsKept(t *testing.T) {
	dir := t.TempDir()
	path := CodexProjectConfig(dir)
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "command = \"unterminated\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := CodexCoordinatorLink(path, "/m", "/data", "Game"); got != LinkUnknown || err == nil {
		t.Fatalf("%s %v", got, err)
	}
	if err := EnsureCodexCoordinator(path, "/m", "/data", "Game"); err == nil {
		t.Fatal("an unreadable file was accepted")
	}
	if after, _ := os.ReadFile(path); string(after) != body {
		t.Fatal("an unreadable file was changed")
	}
	multi := "note = \"\"\"\nhello\n\"\"\"\n"
	if err := os.WriteFile(path, []byte(multi), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCodexCoordinator(path, "/m", "/data", "Game"); err == nil {
		t.Fatal("a multiline string was rewritten")
	}
}

func TestCodexCoordinatorRefusesASymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.toml")
	if err := os.WriteFile(real, []byte("keep = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "game")
	if err := os.MkdirAll(filepath.Join(folder, ".codex"), 0o700); err != nil {
		t.Fatal(err)
	}
	link := CodexProjectConfig(folder)
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if got, err := CodexCoordinatorLink(link, "/m", "/data", "Game"); got != LinkUnknown || err == nil {
		t.Fatalf("read through a symlink: %s %v", got, err)
	}
	if err := EnsureCodexCoordinator(link, "/m", "/data", "Game"); err == nil {
		t.Fatal("wrote through a symlink")
	}
	if raw, _ := os.ReadFile(real); string(raw) != "keep = 1\n" {
		t.Fatalf("symlink target changed: %s", raw)
	}
}

func TestCodexCoordinatorInlineAndDotted(t *testing.T) {
	dir := t.TempDir()
	path := CodexProjectConfig(dir)
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	messenger := "/app/messenger"
	inline := "[mcp_servers]\nmessenger = { command = " + tomlBasic(messenger) + ", args = [\"--root\", \"/data\", \"coordinator\", \"--project\", \"Game\"] }\n"
	if err := os.WriteFile(path, []byte(inline), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := CodexCoordinatorLink(path, messenger, "/data", "Game"); err != nil || got != LinkConnected {
		t.Fatalf("inline: %s %v", got, err)
	}
	dotted := "mcp_servers.messenger.command = " + tomlBasic(messenger) + "\n" +
		"mcp_servers.messenger.args = [\"--root\", \"/data\", \"coordinator\", \"--project\", \"Game\"]\n"
	if err := os.WriteFile(path, []byte(dotted), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, err := CodexCoordinatorLink(path, messenger, "/data", "Game"); err != nil || got != LinkConnected {
		t.Fatalf("dotted: %s %v", got, err)
	}
}

func TestCodexCoordinatorKeepsCRLF(t *testing.T) {
	dir := t.TempDir()
	path := CodexProjectConfig(dir)
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "model = \"gpt\"\r\n\r\n[mcp_servers.docs]\r\ncommand = \"docs\"\r\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	messenger := filepath.Join(dir, "messenger")
	if err := EnsureCodexCoordinator(path, messenger, "/data", "Game"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(raw), original+"\r\n") && !strings.HasPrefix(string(raw), original+"\n") {
		t.Fatalf("CRLF lines were not kept:\n%q", raw)
	}
	if !strings.Contains(string(raw), "model = \"gpt\"\r\n") || !strings.Contains(string(raw), "command = \"docs\"\r\n") {
		t.Fatalf("a kept line lost its carriage return:\n%q", raw)
	}
}

func TestCodexCoordinatorRefusesASymlinkedDirectory(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(real, "config.toml"), []byte("keep = 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	folder := filepath.Join(dir, "game")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, filepath.Join(folder, ".codex")); err != nil {
		t.Fatal(err)
	}
	path := CodexProjectConfig(folder)
	if got, err := CodexCoordinatorLink(path, "/m", "/data", "Game"); got != LinkUnknown || err == nil {
		t.Fatalf("read through a symlinked directory: %s %v", got, err)
	}
	if err := EnsureCodexCoordinator(path, "/m", "/data", "Game"); err == nil {
		t.Fatal("wrote through a symlinked .codex")
	}
	if raw, _ := os.ReadFile(filepath.Join(real, "config.toml")); string(raw) != "keep = 1\n" {
		t.Fatalf("symlink target changed: %s", raw)
	}
}

func TestCodexCoordinatorOffStaleIsNotEnabled(t *testing.T) {
	dir := t.TempDir()
	path := CodexProjectConfig(dir)
	if err := os.Mkdir(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	body := "[mcp_servers.messenger]\ncommand = \"/old/messenger\"\nargs = [\"--root\", \"/data\", \"coordinator\", \"--project\", \"Game\"]\nenabled = false\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := CodexCoordinatorLink(path, "/new/messenger", "/data", "Game"); got != LinkOff {
		t.Fatal(got)
	}
	if err := EnsureCodexCoordinator(path, "/new/messenger", "/data", "Game"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != body {
		t.Fatal("a turned-off stale entry was enabled")
	}
}
