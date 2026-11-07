package adapters

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Coordinator connection (T-039, D-050).
//
// The owner's Claude chat becomes a project's coordinator when Claude has a
// local-scope MCP server "messenger" for the project folder that starts
// `messenger --root DATA coordinator --project NAME`. Messenger never edits
// Claude's settings file itself: it runs Claude's own command
// `claude mcp add --scope local`, with the program and arguments passed
// directly (no shell), so the same call works on macOS, Windows and Linux.
// Messenger only reads the settings file to see whether the entry is there.

// CoordinatorServerName is the MCP server name of the coordinator in Claude.
const CoordinatorServerName = "messenger"

// CoordinatorArgs are the arguments of the coordinator server.
func CoordinatorArgs(root, project string) []string {
	return []string{"--root", root, "coordinator", "--project", project}
}

// ClaudeAddCoordinatorArgs is the full command that connects the coordinator
// in the folder it runs in.
func ClaudeAddCoordinatorArgs(cli, messenger, root, project string) []string {
	return append([]string{cli, "mcp", "add", "--scope", "local", CoordinatorServerName, "--", messenger}, CoordinatorArgs(root, project)...)
}

// ClaudeUserConfig is the settings file where Claude keeps local-scope MCP
// servers: $CLAUDE_CONFIG_DIR/.claude.json, or ~/.claude.json.
func ClaudeUserConfig(home string, env []string) string {
	for _, kv := range env {
		if dir, ok := strings.CutPrefix(kv, "CLAUDE_CONFIG_DIR="); ok && filepath.IsAbs(dir) {
			return filepath.Join(dir, ".claude.json")
		}
	}
	return filepath.Join(home, ".claude.json")
}

// Coordinator link states.
const (
	LinkConnected = "connected" // the entry starts this Messenger for this project
	LinkMissing   = "missing"   // no "messenger" entry for the folder
	LinkOther     = "other"     // a "messenger" entry starts something else
	LinkUnknown   = "unknown"   // the settings file could not be read
)

// CoordinatorLink reads Claude's settings file and reports whether the
// folder's "messenger" entry starts exactly this Messenger for this project.
func CoordinatorLink(config, folder, messenger, root, project string) (string, error) {
	raw, err := os.ReadFile(config)
	if errors.Is(err, os.ErrNotExist) {
		return LinkMissing, nil
	}
	if err != nil {
		return LinkUnknown, err
	}
	var settings struct {
		Projects map[string]struct {
			MCPServers map[string]json.RawMessage `json:"mcpServers"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return LinkUnknown, errors.New("Claude settings are not valid JSON")
	}
	found := false
	for key, p := range settings.Projects {
		if !sameFolder(key, folder) {
			continue
		}
		entry, ok := p.MCPServers[CoordinatorServerName]
		if !ok {
			continue
		}
		found = true
		var server struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if json.Unmarshal(entry, &server) != nil || (server.Type != "" && server.Type != "stdio") ||
			server.Command != messenger || strings.Join(server.Args, "\x00") != strings.Join(CoordinatorArgs(root, project), "\x00") {
			return LinkOther, nil
		}
	}
	if found {
		return LinkConnected, nil
	}
	return LinkMissing, nil
}

// sameFolder compares a folder key of Claude's settings with a project
// folder. Claude writes the folder it was started in; symlinks are resolved,
// and on Windows the key may use forward slashes and another letter case.
func sameFolder(key, folder string) bool {
	norm := func(p string) string {
		p = filepath.Clean(filepath.FromSlash(p))
		if runtime.GOOS == "windows" {
			p = strings.ToLower(p)
		}
		return p
	}
	k := norm(key)
	if k == norm(folder) {
		return true
	}
	if real, err := filepath.EvalSymlinks(folder); err == nil && k == norm(real) {
		return true
	}
	return false
}
