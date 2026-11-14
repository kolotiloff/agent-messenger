package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCursorCoordinatorKeepsSettings(t *testing.T) {
	path := CursorProjectConfig(t.TempDir())
	messenger := filepath.Join(t.TempDir(), "messenger")
	root := filepath.Join(t.TempDir(), "data")
	if state, err := CursorCoordinatorLink(path, messenger, root, "Game"); err != nil || state != LinkMissing {
		t.Fatal(state, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	original := `{"other":{"large":9007199254740993},"mcpServers":{"messenger_task":{"command":"keep-worker"},"other":{"command":"keep","env":{"SETTING":"unchanged"}}}}`
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := EnsureCursorCoordinator(path, messenger, root, "Game"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	for _, text := range []string{"9007199254740993", "keep-worker", "unchanged", "${env:AGENT_MESSENGER_WORKER_RUN}"} {
		if !strings.Contains(string(raw), text) {
			t.Fatal("lost another setting")
		}
	}
	if state, err := CursorCoordinatorLink(path, messenger, root, "Game"); err != nil || state != LinkConnected {
		t.Fatal(state, err)
	}
	if err := EnsureCursorCoordinator(path, messenger, root, "Game"); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.ReadFile(path); string(after) != string(raw) {
		t.Fatal("connected file rewritten")
	}
	if runtime.GOOS != "windows" {
		if info, _ := os.Stat(path); info.Mode().Perm() != 0o640 {
			t.Fatal("file permissions changed")
		}
	}
	// Replacing our old program preserves custom options inside our entry too.
	top, _ := cursorConfigObject(raw)
	servers, _ := cursorConfigObject(top["mcpServers"])
	entry, _ := cursorConfigObject(servers["messenger"])
	entry["env"] = json.RawMessage(`{"CUSTOM":"keep"}`)
	servers["messenger"], _ = json.Marshal(entry)
	top["mcpServers"], _ = json.Marshal(servers)
	raw, _ = json.Marshal(top)
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	newer := filepath.Join(t.TempDir(), "new-messenger")
	if state, _ := CursorCoordinatorLink(path, newer, root, "Game"); state != LinkStale {
		t.Fatal(state)
	}
	if err := EnsureCursorCoordinator(path, newer, root, "Game"); err != nil {
		t.Fatal(err)
	}
	if state, err := CursorCoordinatorLink(path, newer, root, "Game"); err != nil || state != LinkConnected {
		t.Fatal(state, err)
	}
	if after, _ := os.ReadFile(path); !strings.Contains(string(after), "CUSTOM") {
		t.Fatal("custom server option lost")
	}
}

func TestCursorCoordinatorRefusesAmbiguousOrForeignSettings(t *testing.T) {
	messenger := filepath.Join(t.TempDir(), "messenger")
	root := filepath.Join(t.TempDir(), "data")
	own := map[string]any{"type": "stdio", "command": messenger, "args": CoordinatorArgs(root, "Game")}
	for _, field := range []string{"disabled", "enabled"} {
		entry := map[string]any{}
		for k, v := range own {
			entry[k] = v
		}
		entry[field] = field == "disabled"
		raw, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"messenger": entry}})
		t.Run(field, func(t *testing.T) {
			path := CursorProjectConfig(t.TempDir())
			os.MkdirAll(filepath.Dir(path), 0o700)
			os.WriteFile(path, raw, 0o600)
			// Disabled old versions must not be repaired either.
			if state, err := CursorCoordinatorLink(path, messenger+"-new", root, "Game"); err != nil || state != LinkOff {
				t.Fatal(state, err)
			}
			if err := EnsureCursorCoordinator(path, messenger+"-new", root, "Game"); err != nil {
				t.Fatal(err)
			}
			if after, _ := os.ReadFile(path); string(after) != string(raw) {
				t.Fatal("disabled entry changed")
			}
		})
	}
	for _, tc := range []struct{ name, raw, state string }{
		{"foreign", `{"mcpServers":{"messenger":{"command":"other","args":["other"]}}}`, LinkOther},
		{"empty", "", LinkUnknown}, {"broken", "{broken", LinkUnknown}, {"null", "null", LinkUnknown},
		{"array", `{"mcpServers":[]}`, LinkUnknown},
		{"duplicate top", `{"mcpServers":{},"mcpServers":{}}`, LinkUnknown},
		{"duplicate server", `{"mcpServers":{"messenger":{},"messenger":{}}}`, LinkUnknown},
		{"duplicate command", `{"mcpServers":{"messenger":{"command":"a","command":"b"}}}`, LinkUnknown},
		{"trailing", `{} {}`, LinkUnknown},
		{"large", strings.Repeat(" ", (1<<20)+1), LinkUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := CursorProjectConfig(t.TempDir())
			os.MkdirAll(filepath.Dir(path), 0o700)
			os.WriteFile(path, []byte(tc.raw), 0o600)
			state, err := CursorCoordinatorLink(path, messenger, root, "Game")
			if state != tc.state || (tc.state == LinkUnknown) != (err != nil) {
				t.Fatal(state, err)
			}
			err = EnsureCursorCoordinator(path, messenger, root, "Game")
			if (tc.state == LinkUnknown) != (err != nil) {
				t.Fatal(err)
			}
			if after, _ := os.ReadFile(path); string(after) != tc.raw {
				t.Fatal("file changed")
			}
		})
	}
}

func TestCursorCoordinatorRefusesSymlinks(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "directory"}[directory], func(t *testing.T) {
			folder := t.TempDir()
			path := CursorProjectConfig(folder)
			target := filepath.Join(t.TempDir(), "target")
			if directory {
				os.Mkdir(target, 0o700)
			} else {
				os.Mkdir(filepath.Dir(path), 0o700)
				os.WriteFile(target, []byte("{}"), 0o600)
			}
			link := path
			if directory {
				link = filepath.Dir(path)
			}
			if err := os.Symlink(target, link); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			messenger := filepath.Join(folder, "messenger")
			if state, err := CursorCoordinatorLink(path, messenger, folder, "Game"); state != LinkUnknown || err == nil {
				t.Fatal(state, err)
			}
			if err := EnsureCursorCoordinator(path, messenger, folder, "Game"); err == nil {
				t.Fatal("followed symlink")
			}
		})
	}
}
