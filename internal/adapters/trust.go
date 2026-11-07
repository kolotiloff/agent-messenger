package adapters

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// Folder trust (D-025). A worker runs in the project folder like a session the
// owner opened there. Claude and Codex load some project settings only for a
// folder the owner has trusted: Codex reads .codex/config.toml only for a
// trusted project, and Claude asks before using a folder. This check reads
// the agent's own configuration file and never changes it.

// Trust states.
const (
	TrustYes     = "trusted"
	TrustNo      = "not_trusted"
	TrustUnknown = "unknown"
)

// FolderTrust is the result of one check.
type FolderTrust struct {
	State string `json:"state"`
	// Hint tells the owner how to trust the folder, in Russian for the window.
	Hint string `json:"hint"`
}

// CheckFolderTrust reports whether provider trusts folder. environment is the
// agent's profile directory (CLAUDE_CONFIG_DIR or CODEX_HOME); empty means
// the standard one in home.
func CheckFolderTrust(provider, folder, environment, home string) FolderTrust {
	switch provider {
	case "claude":
		path := filepath.Join(home, ".claude.json")
		if environment != "" {
			path = filepath.Join(environment, ".claude.json")
		}
		state := claudeTrust(path, folder)
		return FolderTrust{State: state, Hint: "Откройте эту папку в Claude Code (claude в Терминале из этой папки) и подтвердите доверие к ней."}
	case "codex":
		dir := filepath.Join(home, ".codex")
		if environment != "" {
			dir = environment
		}
		state := codexTrust(filepath.Join(dir, "config.toml"), folder)
		return FolderTrust{State: state, Hint: "Откройте эту папку в Codex (codex в Терминале из этой папки) и отметьте её как доверенную; без этого Codex не читает настройки проекта."}
	}
	return FolderTrust{State: TrustUnknown}
}

func claudeTrust(path, folder string) string {
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return TrustNo
	}
	if err != nil {
		return TrustUnknown
	}
	var config struct {
		Projects map[string]struct {
			HasTrustDialogAccepted *bool `json:"hasTrustDialogAccepted"`
		} `json:"projects"`
	}
	if json.Unmarshal(raw, &config) != nil {
		return TrustUnknown
	}
	// Claude also trusts a folder when one of its parents was trusted.
	for dir := filepath.Clean(folder); ; dir = filepath.Dir(dir) {
		// On Windows the key may be written with forward slashes.
		for _, key := range []string{dir, filepath.ToSlash(dir)} {
			if p, ok := config.Projects[key]; ok && p.HasTrustDialogAccepted != nil && *p.HasTrustDialogAccepted {
				return TrustYes
			}
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	return TrustNo
}

// codexTrust reads [projects."<folder>"] trust_level = "trusted" from
// config.toml without a TOML library: only that table and key are read.
func codexTrust(path, folder string) string {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return TrustNo
	}
	if err != nil {
		return TrustUnknown
	}
	defer f.Close()
	folder = filepath.Clean(folder)
	headers := map[string]bool{
		`[projects."` + folder + `"]`: true,
		`[projects.'` + folder + `']`: true,
		// A basic TOML string escapes the backslashes of a Windows path.
		`[projects."` + strings.ReplaceAll(folder, `\`, `\\`) + `"]`: true,
	}
	inTable := false
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inTable = headers[line]
			continue
		}
		if !inTable {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(key) == "trust_level" {
			value = strings.Trim(strings.TrimSpace(value), `"'`)
			if value == "trusted" {
				return TrustYes
			}
			return TrustNo
		}
	}
	if scanner.Err() != nil {
		return TrustUnknown
	}
	return TrustNo
}
