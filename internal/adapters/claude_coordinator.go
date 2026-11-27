package adapters

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// Coordinator connection (T-039, D-050).
//
// The owner's Claude chat becomes a project's coordinator when Claude has a
// local-scope MCP server for the project folder that starts
// `messenger --root DATA coordinator --project NAME`. Messenger never edits
// Claude's settings file itself: it runs Claude's own command
// `claude mcp add --scope local`, with the program and arguments passed
// directly (no shell), so the same call works on macOS, Windows and Linux.
// Messenger only reads the settings file to see whether the entry is there.

// CoordinatorServerName is the MCP server name of the coordinator in a fresh project.
const CoordinatorServerName = "kotel"
const legacyCoordinatorName = "messenger"

// A legacy profile keeps its namespace and the permissions attached to it.
// A collision requires an owner decision before either entry is changed.
func configuredCoordinatorName(current, legacy bool) (string, error) {
	if current && legacy {
		return "", errors.New("both kotel and messenger coordinator entries exist")
	}
	if legacy {
		return legacyCoordinatorName, nil
	}
	return CoordinatorServerName, nil
}

// CoordinatorArgs are the arguments of the coordinator server.
func CoordinatorArgs(root, project string) []string {
	return []string{"--root", root, "coordinator", "--project", project}
}

// ClaudeAddCoordinatorArgs is the full command that connects the coordinator
// in the folder it runs in.
func ClaudeAddCoordinatorArgs(cli, name, messenger, root, project string) []string {
	return append([]string{cli, "mcp", "add", "--scope", "local", name, "--", messenger}, CoordinatorArgs(root, project)...)
}

// ClaudeRemoveCoordinatorArgs removes the folder's coordinator entry, before
// a stale one is added again.
func ClaudeRemoveCoordinatorArgs(cli, name string) []string {
	return []string{cli, "mcp", "remove", "--scope", "local", name}
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
	LinkMissing   = "missing"   // no coordinator entry for the folder
	LinkOther     = "other"     // a coordinator entry starts something else
	// LinkStale is this Messenger's own entry (same data folder and
	// project) that starts another program file, such as the client's copy
	// of an earlier version (T-117). It is replaced.
	LinkStale   = "stale"
	LinkUnknown = "unknown" // the settings file could not be read
	// LinkOff is this Messenger's own entry with enabled = false. The owner
	// turned it off; Messenger does not turn it back on.
	LinkOff = "off"
)

// CoordinatorLink reads Claude's settings file and reports whether the
// folder's coordinator entry starts exactly this Messenger for this project.
func CoordinatorLink(config, folder, messenger, root, project string) (string, error) {
	state, _, err := ClaudeCoordinatorEntry(config, folder, messenger, root, project)
	return state, err
}

// ClaudeCoordinatorEntry identifies the local entry used by this project.
func ClaudeCoordinatorEntry(config, folder, messenger, root, project string) (string, string, error) {
	raw, err := os.ReadFile(config)
	if errors.Is(err, os.ErrNotExist) {
		return LinkMissing, CoordinatorServerName, nil
	}
	if err != nil {
		return LinkUnknown, "", err
	}
	var settings struct {
		Projects map[string]struct {
			MCPServers json.RawMessage `json:"mcpServers"`
		} `json:"projects"`
	}
	if err := json.Unmarshal(raw, &settings); err != nil {
		return LinkUnknown, "", errors.New("Claude settings are not valid JSON")
	}
	found := ""
	state := LinkConnected
	for key, p := range settings.Projects {
		if !sameFolder(key, folder) {
			continue
		}
		if p.MCPServers == nil {
			continue
		}
		servers, err := mcpConfigObject(p.MCPServers)
		if err != nil {
			return LinkUnknown, "", err
		}
		_, current := servers[CoordinatorServerName]
		_, legacy := servers[legacyCoordinatorName]
		name, err := configuredCoordinatorName(current, legacy)
		if err != nil {
			return LinkOther, "", err
		}
		entry, ok := servers[name]
		if !ok {
			continue
		}
		if found != "" {
			return LinkUnknown, "", errors.New("multiple Claude project entries match this folder")
		}
		found = name
		var server struct {
			Type    string   `json:"type"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		}
		if json.Unmarshal(entry, &server) != nil || (server.Type != "" && server.Type != "stdio") ||
			!slices.Equal(server.Args, CoordinatorArgs(root, project)) {
			return LinkOther, name, nil
		}
		if server.Command == "" {
			return LinkOther, name, nil
		}
		if server.Command != messenger {
			// Claude replaces entries by removing and adding them. This
			// would discard a manually added environment or other options.
			fields, err := mcpConfigObject(entry)
			if err != nil {
				return LinkUnknown, name, err
			}
			for key, value := range fields {
				switch key {
				case "type", "command", "args":
				case "env":
					var env map[string]string
					if json.Unmarshal(value, &env) != nil || len(env) != 0 {
						return LinkOther, name, nil
					}
				default:
					return LinkOther, name, nil
				}
			}
			state = LinkStale
		}
	}
	if found != "" {
		return state, found, nil
	}
	return LinkMissing, CoordinatorServerName, nil
}

// sameFolder compares a folder key of Claude's settings with a project
// folder. Claude writes the folder it was started in; symlinks on either side
// are resolved, and on Windows the key may use forward slashes and another
// letter case.
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
	real, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return false
	}
	if k == norm(real) {
		return true
	}
	realKey, err := filepath.EvalSymlinks(key)
	return err == nil && norm(realKey) == norm(real)
}
