package adapters

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cursor task runs (T-012, D-049).
//
// Cursor CLI takes MCP servers only from its own mcp.json, not from the
// command line. Messenger keeps one entry there, messenger_task, that starts
// `messenger task-mcp --from-env`. The run's database, run ID and receipt
// reach it through AGENT_MESSENGER_TASK, which the entry passes on with
// Cursor's ${env:NAME} interpolation. In a Cursor session that Messenger did
// not start the variable is empty and the server lists no tools.

// TaskEnvVariable names the variable that carries a Cursor run to its tools.
const TaskEnvVariable = "AGENT_MESSENGER_TASK"

// WorkerRunVariable marks every worker run's environment (D-050). A worker
// opened in the project folder may load the owner's coordinator connection;
// the coordinator server lists no tools when it sees this variable, so a
// worker can never start or answer tasks.
const WorkerRunVariable = "AGENT_MESSENGER_WORKER_RUN"

// TaskEnv returns the extra environment of a worker run: only a Cursor run
// needs one.
func TaskEnv(s TaskRunSpec) []string {
	if s.Provider != "cursor" {
		return nil
	}
	raw, _ := json.Marshal(map[string]string{"db": s.DB, "run": s.RunID, "receipt": s.Receipt})
	return append(CursorProgramEnv(s.CLI), TaskEnvVariable+"="+string(raw))
}

func cursorTaskEntry(messenger string) map[string]any {
	return map[string]any{
		"command": messenger,
		"args":    []string{"task-mcp", "--from-env"},
		"env":     map[string]string{TaskEnvVariable: "${env:" + TaskEnvVariable + "}"},
	}
}

// EnsureCursorTaskServer puts Messenger's entry into a Cursor mcp.json, or
// leaves the file alone when the entry is already there. Every other entry
// and key is kept exactly as it was. A file that is not valid JSON is never
// overwritten: the run does not start.
func EnsureCursorTaskServer(path, messenger string) error {
	if !filepath.IsAbs(path) || !filepath.IsAbs(messenger) {
		return errors.New("cursor mcp.json and messenger need absolute paths")
	}
	top := map[string]json.RawMessage{}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if len(bytes.TrimSpace(raw)) > 0 {
			if err := json.Unmarshal(raw, &top); err != nil {
				return fmt.Errorf("%s is not valid JSON; Messenger does not change it: %w", path, err)
			}
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return err
	}
	servers := map[string]json.RawMessage{}
	if existing, ok := top["mcpServers"]; ok && string(existing) != "null" {
		if err := json.Unmarshal(existing, &servers); err != nil {
			return fmt.Errorf("%s: mcpServers is not an object; Messenger does not change it", path)
		}
	}
	want, _ := json.Marshal(cursorTaskEntry(messenger))
	if current, ok := servers[TaskServerName]; ok && sameJSON(current, want) {
		return nil
	}
	servers[TaskServerName] = want
	encodedServers, _ := json.Marshal(servers)
	top["mcpServers"] = encodedServers
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".mcp-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	_, err = tmp.Write(append(out, '\n'))
	if err == nil {
		err = tmp.Chmod(mode)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func sameJSON(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return false
	}
	ea, _ := json.Marshal(x)
	eb, _ := json.Marshal(y)
	return bytes.Equal(ea, eb)
}

// CursorConfigFiles are the mcp.json files a Cursor run may read: the
// default ~/.cursor, and the run's own profile folder (CURSOR_CONFIG_DIR)
// when it has one.
func CursorConfigFiles(home, profile string) []string {
	files := []string{filepath.Join(home, ".cursor", "mcp.json")}
	if profile != "" && filepath.Clean(profile) != filepath.Join(home, ".cursor") {
		files = append(files, filepath.Join(profile, "mcp.json"))
	}
	return files
}

func cursorTaskArgs(s TaskRunSpec) ([]string, error) {
	if s.Resume && s.Session == "" {
		return nil, errors.New("a resumed Cursor task needs its recorded chat id")
	}
	if !s.Resume && s.Session != "" {
		return nil, errors.New("a new Cursor task must not name a chat")
	}
	if s.Session != "" {
		if err := rejectLooseID(s.Session); err != nil {
			return nil, err
		}
		if strings.HasPrefix(s.Session, "-") {
			return nil, errors.New("invalid Cursor chat id")
		}
	}
	args := append(CursorProgram(s.CLI), "-p", "--output-format", "stream-json", "--trust", "--approve-mcps")
	if s.Profile == ProfileMessengerOnly {
		// Tests: tools other than Messenger's are not approved, and the
		// sandbox stays on.
		args = append(args, "--sandbox", "enabled")
	} else {
		// D-023, D-028: a headless run cannot ask, so its actions are
		// approved, as for the other workers. The project's own Cursor
		// rules still apply. Without full access (D-065) the sandbox is
		// asked for explicitly, so a forced run never depends on the
		// owner's own sandbox setting.
		args = append(args, "--force")
		if s.FullAccess {
			args = append(args, "--sandbox", "disabled")
		} else {
			args = append(args, "--sandbox", "enabled")
		}
	}
	if s.Model != "" {
		args = append(args, "--model", s.Model)
	}
	if s.Resume {
		args = append(args, "--resume="+s.Session, taskPrompt(s.Profile))
		return args, ExplicitResume(args, s.Session)
	}
	return append(args, taskPrompt(s.Profile)), nil
}

// cursorToolCall returns the "<kind>ToolCall" object of a tool_call event.
func cursorToolCall(e map[string]any) (string, map[string]any) {
	call, _ := e["tool_call"].(map[string]any)
	for key, value := range call {
		if strings.HasSuffix(key, "ToolCall") {
			m, _ := value.(map[string]any)
			return key, m
		}
	}
	return "", nil
}

func parseCursorEvent(e map[string]any, kind string, r *TaskEventResult, note func(string)) {
	switch kind {
	case "system":
		if e["subtype"] == "init" {
			id, _ := e["session_id"].(string)
			note(id)
			r.noteModel(e)
		}
	case "result":
		id, _ := e["session_id"].(string)
		note(id)
		if text, _ := e["result"].(string); text != "" {
			r.noteText(text)
		}
		cursorUsage(e, r)
		subtype, _ := e["subtype"].(string)
		isErr, _ := e["is_error"].(bool)
		if subtype == "success" && !isErr {
			if r.Failure == "" {
				r.Completed = true
			}
		} else {
			r.Completed = false
			r.Failure = "Cursor result " + subtype
			text, _ := e["result"].(string)
			if text == "" {
				text = errorText(e)
			}
			r.noteFailure(text, taskNow())
		}
	case "error":
		r.Completed = false
		r.Failure = "Cursor error"
		r.noteFailure(errorText(e), taskNow())
	default:
		// Every other event names the same chat; a different one is a
		// conflict.
		if id, _ := e["session_id"].(string); id != "" {
			note(id)
		}
	}
}

func cursorFeedEntries(e map[string]any) []FeedEntry {
	switch e["type"] {
	case "assistant":
		var out []FeedEntry
		message, _ := e["message"].(map[string]any)
		content, _ := message["content"].([]any)
		for _, c := range content {
			block, _ := c.(map[string]any)
			if block["type"] == "text" {
				if text, _ := block["text"].(string); strings.TrimSpace(text) != "" {
					out = append(out, FeedEntry{Kind: "text", Text: text})
				}
			}
		}
		return out
	case "tool_call":
		if e["subtype"] != "started" {
			return nil
		}
		key, call := cursorToolCall(e)
		args, _ := call["args"].(map[string]any)
		switch key {
		case "", "getMcpToolsToolCall":
			return nil
		case "mcpToolCall":
			server, _ := args["providerIdentifier"].(string)
			tool, _ := args["toolName"].(string)
			if server == TaskServerName {
				return nil
			}
			return []FeedEntry{{Kind: "tool", Title: server + "." + tool}}
		case "shellToolCall":
			command, _ := args["command"].(string)
			return []FeedEntry{{Kind: "tool", Title: "Команда", Text: command}}
		}
		return []FeedEntry{{Kind: "tool", Title: strings.TrimSuffix(key, "ToolCall"), Text: toolSummary(args)}}
	}
	return nil
}
