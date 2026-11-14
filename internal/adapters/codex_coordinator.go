package adapters

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Codex as a project's coordinator (T-160, D-123).
//
// Codex has no project-scoped "mcp add": that command writes the user
// config, which every project would share. The documented place for one
// project is .codex/config.toml in the project folder, read only when the
// folder is trusted. Messenger writes only the mcp_servers.messenger table
// there, and only when it is missing or is this Messenger's own entry for
// an older program file. Any other contents of the file stay byte for byte.
// A file that cannot be read safely is left untouched.
//
// A Codex worker in the same folder loads this file too. Its process carries
// AGENT_MESSENGER_WORKER_RUN, and the coordinator server then lists no tools.

// CodexCoordinatorToolTimeout is how long one coordinator tool call may run.
// wait blocks for up to 120 s, so the server's own 60 s default is too short.
const CodexCoordinatorToolTimeout = 180

// CodexProjectConfig is the project-scoped Codex config for a folder.
func CodexProjectConfig(folder string) string {
	return filepath.Join(folder, ".codex", "config.toml")
}

// CodexCoordinatorBlock is the table Messenger writes.
func CodexCoordinatorBlock(messenger, root, project string) string {
	args := CoordinatorArgs(root, project)
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = tomlBasic(arg)
	}
	return "[mcp_servers.messenger]\n" +
		"command = " + tomlBasic(messenger) + "\n" +
		"args = [" + strings.Join(quoted, ", ") + "]\n" +
		"default_tools_approval_mode = \"approve\"\n" +
		"tool_timeout_sec = " + fmt.Sprintf("%d", CodexCoordinatorToolTimeout) + "\n"
}

// CodexCoordinatorLink reports whether the project's Codex config starts
// this Messenger for this project. A missing file is LinkMissing.
func CodexCoordinatorLink(path, messenger, root, project string) (string, error) {
	if err := codexConfigLink(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LinkMissing, nil
		}
		return LinkUnknown, err
	}
	// A symlinked .codex directory would make a later write leave the project
	// folder. Report it as unreadable so nothing treats that file as ours.
	if err := codexConfigLink(filepath.Dir(path)); err != nil {
		return LinkUnknown, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return LinkMissing, nil
	}
	if err != nil {
		return LinkUnknown, err
	}
	state, _, err := classifyCodexConfig(string(raw), messenger, root, project)
	return state, err
}

// EnsureCodexCoordinator writes the messenger table when it is missing or
// is this Messenger's own entry for another program file. It does nothing
// when the entry is already right, turned off, or belongs to something else.
func EnsureCodexCoordinator(path, messenger, root, project string) error {
	if err := codexConfigLink(filepath.Dir(path)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := codexConfigLink(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	state, kept, err := classifyCodexConfig(string(raw), messenger, root, project)
	if err != nil {
		return err
	}
	if state != LinkMissing && state != LinkStale {
		return nil
	}
	body := strings.Join(kept, "\n")
	if body != "" {
		body += "\n\n"
	}
	body += CodexCoordinatorBlock(messenger, root, project)
	return writeCodexConfig(path, []byte(body), raw)
}

// codexConfigLink refuses a symlink, so a write cannot leave the project
// folder through one. A path that does not exist yet is not an error.
func codexConfigLink(path string) error {
	fi, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return errors.New("codex config path is a symlink")
	}
	return nil
}

func writeCodexConfig(path string, data, previous []byte) error {
	dir := filepath.Dir(path)
	if err := codexConfigLink(dir); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
	}
	mode := os.FileMode(0o600)
	if len(previous) > 0 || fileExists(path) {
		if fi, err := os.Stat(path); err == nil {
			mode = fi.Mode().Perm()
		}
	}
	tmp := path + ".tmp"
	_ = os.Remove(tmp)
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		if runtime.GOOS == "windows" {
			_ = os.Remove(path)
			if err2 := os.Rename(tmp, path); err2 == nil {
				return nil
			}
		}
		os.Remove(tmp)
		return err
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// maxCodexConfig bounds a project config Messenger is willing to rewrite.
const maxCodexConfig = 1 << 20

// classifyCodexConfig reads a Codex config. kept is every original line that
// is not part of the messenger server, without its line ending. An unreadable
// file returns LinkUnknown and no lines, and the caller must not write.
func classifyCodexConfig(raw, messenger, root, project string) (string, []string, error) {
	if len(raw) > maxCodexConfig {
		return LinkUnknown, nil, errors.New("codex config is too large to update safely")
	}
	if strings.Contains(raw, `"""`) || strings.Contains(raw, "'''") {
		return LinkUnknown, nil, errors.New("codex config has a multiline string; Messenger does not change it")
	}
	lines := strings.Split(raw, "\n")
	// A trailing newline becomes a final empty line. Drop it so joining
	// later does not add a blank line that was not a blank line of content.
	// A kept line still carries its own \r, so a CRLF file stays CRLF.
	trailing := len(lines) > 0 && lines[len(lines)-1] == ""
	if trailing {
		lines = lines[:len(lines)-1]
	}
	var kept []string
	var command string
	var args []string
	var sawCommand, sawArgs, enabledFalse, sawMessenger, unreadable bool
	table := ""
	for _, line := range lines {
		if lineQuoteOpen(line) {
			return LinkUnknown, nil, errors.New("codex config could not be read safely")
		}
		owned, header, key, value, ok := codexLine(table, line)
		if !ok {
			return LinkUnknown, nil, errors.New("codex config could not be read safely")
		}
		if header != "" {
			table = header
		}
		if !owned {
			kept = append(kept, line)
			continue
		}
		sawMessenger = true
		if key == "" {
			continue
		}
		switch key {
		case "command":
			if sawCommand {
				unreadable = true
				break
			}
			sawCommand = true
			command = value
		case "args":
			parsed, good := parseTOMLArray(value)
			if !good || sawArgs {
				unreadable = true
				break
			}
			sawArgs = true
			args = parsed
		case "enabled":
			switch strings.TrimSpace(value) {
			case "false":
				enabledFalse = true
			case "true":
			default:
				unreadable = true
			}
		case "inline":
			cmd, parsed, hasArgs, off, good := parseInlineServer(value)
			if !good || sawCommand || sawArgs {
				unreadable = true
				break
			}
			sawCommand = true
			command = cmd
			if hasArgs {
				sawArgs = true
				args = parsed
			}
			if off {
				enabledFalse = true
			}
		}
	}
	if unreadable {
		return LinkUnknown, nil, errors.New("codex config could not be read safely")
	}
	if !sawMessenger {
		return LinkMissing, kept, nil
	}
	want := CoordinatorArgs(root, project)
	sameArgs := sawArgs && len(args) == len(want)
	if sameArgs {
		for i := range want {
			if args[i] != want[i] {
				sameArgs = false
				break
			}
		}
	}
	if !sameArgs || !sawCommand {
		return LinkOther, nil, nil
	}
	// The owner turned this entry off. Do not turn it back on, even when
	// the program path is an older file of ours.
	if enabledFalse {
		return LinkOff, nil, nil
	}
	if !sameTrustPath(command, messenger) {
		return LinkStale, kept, nil
	}
	return LinkConnected, nil, nil
}

// lineQuoteOpen reports a quote that is still open at the end of one line.
// A basic or literal string cannot cross a line, so such a file is not safe
// to rewrite. Multiline strings are rejected before this runs.
func lineQuoteOpen(line string) bool {
	line = strings.TrimRight(line, "\r")
	in := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if in != 0 {
			if c == '\\' && in == '"' && i+1 < len(line) {
				i++
				continue
			}
			if c == in {
				in = 0
			}
			continue
		}
		if c == '#' {
			return false
		}
		if c == '"' || c == '\'' {
			in = c
		}
	}
	return in != 0
}

// codexLine classifies one line. header is set when the line is a table
// header. owned is true when the line belongs to mcp_servers.messenger.
// key and value are set for that server's own command, args and enabled.
func codexLine(table, line string) (owned bool, header, key, value string, ok bool) {
	trim := strings.TrimSpace(strings.TrimRight(line, "\r"))
	if trim == "" || strings.HasPrefix(trim, "#") {
		return false, "", "", "", true
	}
	if !quotesClosed(trim) {
		return false, "", "", "", false
	}
	if strings.HasPrefix(trim, "[") {
		if !strings.HasSuffix(trim, "]") || strings.Contains(trim, "=") {
			return false, "", "", "", false
		}
		name, good := parseTomlPath(trim[1 : len(trim)-1])
		if !good {
			return false, "", "", "", false
		}
		return messengerTable(name), name, "", "", true
	}
	keyRaw, valueRaw, cut := strings.Cut(trim, "=")
	if !cut {
		return false, "", "", "", false
	}
	path, good := parseTomlPath(keyRaw)
	if !good {
		return false, "", "", "", false
	}
	full := path
	if table != "" {
		full = table + "." + path
	}
	if !strings.HasPrefix(full, "mcp_servers.messenger") || (full != "mcp_servers.messenger" && !strings.HasPrefix(full, "mcp_servers.messenger.")) {
		return false, "", "", "", true
	}
	// A nested table such as mcp_servers.messenger.env is owned, but its
	// keys are not the server's command, args or enabled.
	field := ""
	if table == "mcp_servers.messenger" && !strings.Contains(path, ".") {
		field = path
	} else if table == "mcp_servers" && strings.HasPrefix(path, "messenger.") {
		field = strings.TrimPrefix(path, "messenger.")
	} else if table == "" && strings.HasPrefix(path, "mcp_servers.messenger.") {
		field = strings.TrimPrefix(path, "mcp_servers.messenger.")
	}
	if strings.Contains(field, ".") {
		field = ""
	}
	valueRaw = stripTomlComment(valueRaw)
	if field == "command" || field == "args" || field == "enabled" {
		if field == "command" {
			text, good := tomlScalar(valueRaw)
			if !good {
				return true, "", field, "", false
			}
			return true, "", field, text, true
		}
		if field == "args" && strings.HasPrefix(strings.TrimSpace(valueRaw), "{") {
			return true, "", field, "", false
		}
		return true, "", field, strings.TrimSpace(valueRaw), true
	}
	// An inline table on the server key itself: messenger = { command = … }.
	if (table == "mcp_servers" && path == "messenger") || (table == "" && path == "mcp_servers.messenger") {
		if _, _, _, _, good := parseInlineServer(valueRaw); !good {
			return true, "", "", "", false
		}
		return true, "", "inline", valueRaw, true
	}
	return true, "", "", "", true
}

func messengerTable(name string) bool {
	return name == "mcp_servers.messenger" || strings.HasPrefix(name, "mcp_servers.messenger.")
}

func parseTomlPath(s string) (string, bool) {
	var parts []string
	rest := strings.TrimSpace(s)
	for rest != "" {
		part, next, ok := tomlKey(rest)
		if !ok || part == "" {
			return "", false
		}
		parts = append(parts, part)
		rest = strings.TrimSpace(next)
		if rest == "" {
			break
		}
		if !strings.HasPrefix(rest, ".") {
			return "", false
		}
		rest = strings.TrimSpace(rest[1:])
		if rest == "" {
			return "", false
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "."), true
}

// stripTomlComment removes a trailing comment that is outside a string.
func stripTomlComment(s string) string {
	in := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if in != 0 {
			if c == '\\' && in == '"' && i+1 < len(s) {
				i++
				continue
			}
			if c == in {
				in = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			in = c
		case '#':
			return s[:i]
		}
	}
	return s
}

func tomlScalar(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	if s[0] == '"' || s[0] == '\'' {
		text, rest, ok := tomlKey(s)
		if !ok || strings.TrimSpace(rest) != "" {
			return "", false
		}
		return text, true
	}
	return "", false
}

func parseTOMLArray(s string) ([]string, bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "[") {
		return nil, false
	}
	rest := strings.TrimSpace(s[1:])
	var out []string
	for {
		rest = strings.TrimSpace(rest)
		if rest == "" {
			return nil, false
		}
		if rest[0] == ']' {
			return out, strings.TrimSpace(rest[1:]) == ""
		}
		if rest[0] != '"' && rest[0] != '\'' {
			return nil, false
		}
		text, next, ok := tomlKey(rest)
		if !ok {
			return nil, false
		}
		out = append(out, text)
		rest = strings.TrimSpace(next)
		if strings.HasPrefix(rest, ",") {
			rest = strings.TrimSpace(rest[1:])
			continue
		}
		if strings.HasPrefix(rest, "]") {
			return out, strings.TrimSpace(rest[1:]) == ""
		}
		return nil, false
	}
}

// parseInlineServer reads `{ command = "…", args = […], enabled = false }`.
// hasArgs is true when the args key is present, including an empty array.
func parseInlineServer(s string) (command string, args []string, hasArgs, enabledFalse, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "{") || !strings.HasSuffix(strings.TrimSpace(s), "}") {
		return "", nil, false, false, false
	}
	inner := strings.TrimSpace(s[1 : len(s)-1])
	for inner != "" {
		inner = strings.TrimSpace(inner)
		key, rest, good := tomlKey(inner)
		if !good {
			return "", nil, false, false, false
		}
		rest = strings.TrimSpace(rest)
		if !strings.HasPrefix(rest, "=") {
			return "", nil, false, false, false
		}
		rest = strings.TrimSpace(rest[1:])
		switch key {
		case "command":
			text, next, good := readQuoted(rest)
			if !good {
				return "", nil, false, false, false
			}
			command = text
			rest = next
		case "args":
			end, good := bracketEnd(rest, '[', ']')
			if !good {
				return "", nil, false, false, false
			}
			parsed, good := parseTOMLArray(rest[:end])
			if !good {
				return "", nil, false, false, false
			}
			hasArgs = true
			args = parsed
			rest = rest[end:]
		case "enabled":
			if strings.HasPrefix(rest, "false") {
				enabledFalse = true
				rest = rest[len("false"):]
			} else if strings.HasPrefix(rest, "true") {
				rest = rest[len("true"):]
			} else {
				return "", nil, false, false, false
			}
		default:
			next, good := skipTomlValue(rest)
			if !good {
				return "", nil, false, false, false
			}
			rest = next
		}
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, ",") {
			inner = strings.TrimSpace(rest[1:])
			continue
		}
		if rest == "" {
			return command, args, hasArgs, enabledFalse, command != ""
		}
		return "", nil, false, false, false
	}
	return command, args, hasArgs, enabledFalse, command != ""
}

func readQuoted(s string) (string, string, bool) {
	s = strings.TrimSpace(s)
	if s == "" || (s[0] != '"' && s[0] != '\'') {
		return "", "", false
	}
	return tomlKey(s)
}

func bracketEnd(s string, open, close byte) (int, bool) {
	if len(s) == 0 || s[0] != open {
		return 0, false
	}
	depth := 0
	in := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if in != 0 {
			if c == '\\' && in == '"' && i+1 < len(s) {
				i++
				continue
			}
			if c == in {
				in = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			in = c
		default:
			if c == open {
				depth++
			} else if c == close {
				depth--
				if depth == 0 {
					return i + 1, true
				}
			}
		}
	}
	return 0, false
}

func skipTomlValue(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	switch s[0] {
	case '"', '\'':
		_, rest, ok := tomlKey(s)
		return rest, ok
	case '[':
		n, ok := bracketEnd(s, '[', ']')
		return s[n:], ok
	case '{':
		n, ok := bracketEnd(s, '{', '}')
		return s[n:], ok
	default:
		i := 0
		for i < len(s) && s[i] != ',' && s[i] != '}' && s[i] != ']' {
			i++
		}
		if i == 0 {
			return "", false
		}
		return s[i:], true
	}
}

func quotesClosed(s string) bool {
	in := byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if in != 0 {
			if c == '\\' && in == '"' && i+1 < len(s) {
				i++
				continue
			}
			if c == in {
				in = 0
			}
			continue
		}
		if c == '"' || c == '\'' {
			in = c
		} else if c == '#' {
			return true
		}
	}
	return in == 0
}

func tomlBasic(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\', '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				fmt.Fprintf(&b, `\u%04X`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}
