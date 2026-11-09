package supervisor

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

func newRunID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func profileVariable(provider string) string {
	switch provider {
	case "claude":
		return "CLAUDE_CONFIG_DIR"
	case "codex":
		return "CODEX_HOME"
	case "cursor":
		return "CURSOR_CONFIG_DIR"
	default:
		return ""
	}
}

// profileEnv binds the process to the registered profile.
// The standard profile is selected by removing the override: Claude stores the
// normal macOS login for ~/.claude and stops seeing it when CLAUDE_CONFIG_DIR is set.
// A different directory still gets the variable, and an inherited value is never kept.
func profileEnv(parent []string, provider, environment string) []string {
	return ProfileEnvironment(parent, provider, environment)
}

// ProfileEnvironment applies the same explicit profile selection to launches
// and diagnostic subcommands. It does not read or export stored credentials.
func ProfileEnvironment(parent []string, provider, environment string) []string {
	key := profileVariable(provider)
	out := make([]string, 0, len(parent)+1)
	for _, item := range parent {
		name, _, _ := strings.Cut(item, "=")
		if key != "" && name == key {
			continue
		}
		out = append(out, item)
	}
	if key != "" && environment != "" && !isDefaultProfile(provider, environment) {
		out = append(out, key+"="+environment)
	}
	return out
}

func isDefaultProfile(provider, environment string) bool {
	def, err := defaultProfileDir(provider)
	if err != nil || def == "" || environment == "" {
		return false
	}
	return filepath.Clean(environment) == filepath.Clean(def)
}

// DefaultProfileDir is the agent's standard profile folder in the home
// folder, whether or not it exists.
func DefaultProfileDir(provider string) (string, error) { return defaultProfileDir(provider) }

func defaultProfileDir(provider string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var name string
	switch provider {
	case "claude":
		name = ".claude"
	case "codex":
		name = ".codex"
	case "cursor":
		name = ".cursor"
	default:
		return "", nil
	}
	path := filepath.Join(home, name)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path), nil
	}
	return resolved, nil
}
