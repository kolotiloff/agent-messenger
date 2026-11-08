package adapters

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
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
	case "cursor":
		// A Cursor worker is started with --trust: the folder is trusted for
		// that run, and the project's .cursor rules apply.
		return FolderTrust{State: TrustYes}
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

// codexTrust reads the trust_level of a project from config.toml without a
// TOML library: only the projects entries are read, written as a table
// ([projects."<path>"]), inline in [projects] or as dotted keys. Codex trusts
// a project by its folder or by the root of its git repository.
func codexTrust(path, folder string) string {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return TrustNo
	}
	if err != nil {
		return TrustUnknown
	}
	defer f.Close()
	levels := map[string]string{}
	table := ""
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			table = ""
			if inner, ok := strings.CutPrefix(strings.TrimSuffix(line, "]"), "["); ok && !strings.HasPrefix(inner, "[") {
				table = strings.TrimSpace(inner)
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch {
		case table == "projects":
			// "<path>" = { trust_level = "trusted" }
			if project, rest, ok := tomlKey(key); ok && rest == "" {
				if level := inlineTrust(value); level != "" {
					levels[project] = level
				}
			}
		case strings.HasPrefix(table, "projects."):
			if project, rest, ok := tomlKey(strings.TrimPrefix(table, "projects.")); ok && rest == "" && key == "trust_level" {
				levels[project] = tomlValue(value)
			}
		case table == "":
			// projects."<path>".trust_level = "trusted"
			if after, ok := strings.CutPrefix(key, "projects."); ok {
				if project, rest, ok := tomlKey(after); ok && strings.TrimSpace(rest) == ".trust_level" {
					levels[project] = tomlValue(value)
				}
			}
		}
	}
	if scanner.Err() != nil {
		return TrustUnknown
	}
	for _, candidate := range trustCandidates(folder) {
		for project, level := range levels {
			if sameTrustPath(project, candidate) {
				if level == "trusted" {
					return TrustYes
				}
				return TrustNo
			}
		}
	}
	return TrustNo
}

// trustCandidates is the folder and the root of its git repository.
func trustCandidates(folder string) []string {
	folder = filepath.Clean(folder)
	out := []string{folder}
	for dir := folder; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			if dir != folder {
				out = append(out, dir)
			}
			break
		}
		if dir == filepath.Dir(dir) {
			break
		}
	}
	return out
}

// sameTrustPath compares two folders as the system does: macOS and Windows
// ignore letter case, and Windows accepts either slash and a \\?\ prefix.
func sameTrustPath(a, b string) bool {
	norm := func(p string) string {
		if runtime.GOOS == "windows" {
			p = strings.TrimPrefix(strings.ReplaceAll(p, "/", `\`), `\\?\`)
		}
		return filepath.Clean(p)
	}
	a, b = norm(a), norm(b)
	if runtime.GOOS == "linux" {
		return a == b
	}
	return strings.EqualFold(a, b)
}

// tomlKey reads one key (bare, "basic" or 'literal') from the start of s
// and returns the rest.
func tomlKey(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", false
	}
	switch s[0] {
	case '\'':
		end := strings.IndexByte(s[1:], '\'')
		if end < 0 {
			return "", "", false
		}
		return s[1 : 1+end], s[2+end:], true
	case '"':
		var b strings.Builder
		for i := 1; i < len(s); i++ {
			c := s[i]
			if c == '"' {
				return b.String(), s[i+1:], true
			}
			if c == '\\' && i+1 < len(s) {
				i++
				switch s[i] {
				case 'n':
					b.WriteByte('\n')
				case 't':
					b.WriteByte('\t')
				case 'r':
					b.WriteByte('\r')
				case '"', '\\':
					b.WriteByte(s[i])
				case 'u', 'U':
					// A Unicode escape; anything else (a raw Windows path
					// such as "C:\Users") keeps its backslash.
					n := 4
					if s[i] == 'U' {
						n = 8
					}
					if i+n < len(s) {
						if r, err := strconv.ParseUint(s[i+1:i+1+n], 16, 32); err == nil {
							b.WriteRune(rune(r))
							i += n
							continue
						}
					}
					b.WriteByte('\\')
					b.WriteByte(s[i])
				default:
					b.WriteByte('\\')
					b.WriteByte(s[i])
				}
				continue
			}
			b.WriteByte(c)
		}
		return "", "", false
	}
	end := strings.IndexAny(s, ". \t")
	if end < 0 {
		return s, "", true
	}
	return s[:end], s[end:], true
}

// tomlValue is a string value without its quotes and a trailing comment.
func tomlValue(v string) string {
	if value, _, ok := tomlKey(v); ok && (v[0] == '"' || v[0] == '\'') {
		return value
	}
	value, _, _ := strings.Cut(v, "#")
	return strings.TrimSpace(value)
}

// inlineTrust reads trust_level from an inline table { trust_level = "…" }.
func inlineTrust(v string) string {
	inner, ok := strings.CutPrefix(strings.TrimSpace(v), "{")
	if !ok {
		return ""
	}
	inner, _, _ = strings.Cut(inner, "}")
	for _, part := range strings.Split(inner, ",") {
		if key, value, ok := strings.Cut(part, "="); ok && strings.TrimSpace(key) == "trust_level" {
			return tomlValue(strings.TrimSpace(value))
		}
	}
	return ""
}
