package adapters

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/platform"
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

// The packaged commands share one executable. Only accept the installer's
// relative sibling link, so a similarly named custom wrapper stays untouched.
func coordinatorProgramAlias(command, current string) bool {
	if !filepath.IsAbs(command) || filepath.Base(command) != "messenger" ||
		filepath.Base(current) != "kotel" || !sameTrustPath(filepath.Dir(command), filepath.Dir(current)) {
		return false
	}
	for _, path := range []string{command, current} {
		target, err := os.Readlink(path)
		other := current
		if path == current {
			other = command
		}
		if err == nil && target == filepath.Base(other) {
			info, err := os.Stat(other)
			return err == nil && info.Mode().IsRegular()
		}
	}
	return false
}

// An arbitrary executable taking coordinator-shaped arguments may be a custom wrapper.
func managedCoordinatorProgram(command, current, root string) bool {
	if sameTrustPath(command, current) || coordinatorProgramAlias(command, current) {
		return true
	}
	if !filepath.IsAbs(command) {
		return false
	}
	parent := filepath.Dir(command)
	leaf := filepath.Base(command)
	hash, err := hex.DecodeString(filepath.Base(parent))
	if err != nil || len(hash) != 32 || (leaf != "messenger" && leaf != "messenger.exe") {
		return false
	}
	cache := filepath.Dir(parent)
	if sameTrustPath(cache, filepath.Join(root, "runtimes")) {
		return true
	}
	home, err := platform.DefaultRoot()
	return err == nil && sameTrustPath(cache, filepath.Join(home, "runtimes"))
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
	// LinkOff means the owner explicitly disabled this coordinator.
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
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return LinkUnknown, "", err
	}
	var settings struct {
		MCPServers json.RawMessage `json:"mcpServers"`
		Projects   map[string]struct {
			MCPServers         json.RawMessage `json:"mcpServers"`
			DisabledMCPServers json.RawMessage `json:"disabledMcpServers"`
		} `json:"projects"`
	}
	if err == nil && json.Unmarshal(raw, &settings) != nil {
		return LinkUnknown, "", errors.New("Claude settings are not valid JSON")
	}
	var found string
	var entry json.RawMessage
	var disabled []string
	for key, p := range settings.Projects {
		if !sameFolder(key, folder) {
			continue
		}
		if p.DisabledMCPServers != nil {
			var names []string
			if json.Unmarshal(p.DisabledMCPServers, &names) != nil {
				return LinkUnknown, "", errors.New("Claude disabled MCP servers are not a list")
			}
			disabled = append(disabled, names...)
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
		candidate, ok := servers[name]
		if !ok {
			continue
		}
		if found != "" {
			return LinkUnknown, "", errors.New("multiple Claude project entries match this folder")
		}
		found, entry = name, candidate
	}
	if found != "" {
		state := claudeLocalEntryState(entry, messenger, root, project)
		if state == LinkOther {
			return state, found, nil
		}
		if slices.Contains(disabled, found) {
			return LinkOff, found, nil
		}
		if state == LinkConnected {
			return state, found, nil
		}
	} else {
		found = CoordinatorServerName
		for _, name := range []string{CoordinatorServerName, legacyCoordinatorName} {
			if slices.Contains(disabled, name) {
				return LinkOff, name, nil
			}
		}
	}
	if state, err := claudeOtherScope(settings.MCPServers, folder); err != nil {
		return state, found, err
	}
	if entry == nil {
		return LinkMissing, found, nil
	}
	return LinkStale, found, nil
}

func claudeLocalEntryState(entry json.RawMessage, messenger, root, project string) string {
	var server struct {
		Type    string   `json:"type"`
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	if json.Unmarshal(entry, &server) != nil || (server.Type != "" && server.Type != "stdio") ||
		!slices.Equal(server.Args, CoordinatorArgs(root, project)) || server.Command == "" {
		return LinkOther
	}
	if sameTrustPath(server.Command, messenger) || coordinatorProgramAlias(server.Command, messenger) {
		return LinkConnected
	}
	if !managedCoordinatorProgram(server.Command, messenger, root) {
		return LinkOther
	}
	// Removing and adding a server with custom options would lose them.
	fields, err := mcpConfigObject(entry)
	if err != nil {
		return LinkOther
	}
	for key, value := range fields {
		switch key {
		case "type", "command", "args":
		case "env":
			var env map[string]string
			if json.Unmarshal(value, &env) != nil || len(env) != 0 {
				return LinkOther
			}
		default:
			return LinkOther
		}
	}
	return LinkStale
}

// A local registration overrides the project and user scopes. Do not hide a
// custom server or introduce a second coordinator during a registration.
func claudeOtherScope(userServers json.RawMessage, folder string) (string, error) {
	hasCoordinator := func(raw json.RawMessage) (bool, error) {
		if raw == nil {
			return false, nil
		}
		servers, err := mcpConfigObject(raw)
		_, current := servers[CoordinatorServerName]
		_, legacy := servers[legacyCoordinatorName]
		return current || legacy, err
	}
	if found, err := hasCoordinator(userServers); err != nil {
		return LinkUnknown, errors.New("Claude user-scope MCP configuration is invalid")
	} else if found {
		return LinkOther, errors.New("Claude already has a user-scope kotel or messenger server")
	}
	dir, err := filepath.EvalSymlinks(folder)
	if err != nil {
		return LinkUnknown, err
	}
	for {
		path := filepath.Join(dir, ".mcp.json")
		info, err := os.Lstat(path)
		if err == nil {
			if !info.Mode().IsRegular() {
				return LinkUnknown, errors.New("Claude project-scope MCP configuration is not a regular file")
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return LinkUnknown, err
			}
			config, err := mcpConfigObject(raw)
			if err != nil {
				return LinkUnknown, errors.New("Claude project-scope MCP configuration is invalid")
			}
			if found, err := hasCoordinator(config["mcpServers"]); err != nil {
				return LinkUnknown, errors.New("Claude project-scope MCP configuration is invalid")
			} else if found {
				return LinkOther, errors.New("Claude already has a project-scope kotel or messenger server")
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return LinkUnknown, err
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", nil
		}
		dir = parent
	}
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
