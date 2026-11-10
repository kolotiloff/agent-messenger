package adapters

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Worker profiles (D-023). "worker" runs like a session the owner opened in
// the project folder: the provider's normal tools and the project's own
// settings, plus the Messenger worker tools. "messenger-only" is the stripped
// profile for automated tests: Messenger tools only.
const (
	ProfileWorker        = "worker"
	ProfileMessengerOnly = "messenger-only"
)

// TaskServerName is the MCP server name of the worker tools.
const TaskServerName = "messenger_task"

// TaskToolNames are the worker tools, in the order a run uses them.
var TaskToolNames = []string{"read_task", "report", "ask", "ask_owner"}

// TaskWorkerPrompt is the first message of every run. The task itself is read
// through read_task, after the conversation identity is bound.
const TaskWorkerPrompt = "You are a worker started by Agent Messenger for one task in this project folder. " +
	"First call the messenger_task tool read_task to get the task (and, on a resumed run, the answer to your question). " +
	"Do the work with your normal tools, following this project's own instructions and the owner's rules that read_task returns. " +
	"If you changed or created files and the project is under version control, commit (or check in) your work before you report: that is how it reaches the project and the other computers. " +
	"If you cannot commit (files locked by another agent, the copy is behind the server, a conflict) and cannot fix it within your task's limits, do not report: use ask to describe the problem and what you tried, so this conversation stays open; when the answer comes, try again. " +
	"End this run with exactly one messenger_task call: report (the task is done; at most 8 short lines on what you did and what was verified, " +
	"with the change id, tests and files in their own fields; put longer details in a file in the project and list its path), " +
	"ask (a question for the coordinator) or ask_owner (a question for the owner), then stop. " +
	"Do not call any messenger_task tool after that."

// TaskTestPrompt is the first message in the messenger-only profile, which
// has no other tools.
const TaskTestPrompt = "Agent Messenger test run. Use ONLY the messenger_task tools: call read_task, then exactly one of report, ask or ask_owner, then stop."

func taskPrompt(profile string) string {
	if profile == ProfileMessengerOnly {
		return TaskTestPrompt
	}
	return TaskWorkerPrompt
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// TaskRunSpec describes one worker run.
type TaskRunSpec struct {
	Provider  string
	CLI       string
	Messenger string // absolute path of the messenger executable
	DB        string
	RunID     string
	Receipt   string
	// Session is the recorded conversation ID. For Claude it is always set (a
	// new run uses it with --session-id). For Codex it is empty on the first
	// run and set when resuming.
	// For Codex and Cursor it is empty on the first run and set when resuming.
	Session string
	Resume  bool
	Profile string
	// Model is the worker's model; empty keeps the CLI's own default.
	Model string
	// FullAccess lifts the agent's own permission checks and sandbox
	// (T-054, D-065). Ignored in the messenger-only test profile.
	FullAccess bool
}

// taskModelPattern matches core.ValidTaskModel: a model name never starts
// with "-" and has no spaces, quotes or "=", so it can not become a flag or
// break the Codex config override.
var taskModelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/\[\]-]{0,79}$`)

func taskMCPArgs(s TaskRunSpec) []string {
	return []string{"task-mcp", "--db", s.DB, "--run", s.RunID, "--receipt", s.Receipt}
}

// TaskArgs builds the argv of a worker run. It never selects a "latest"
// conversation: a new Claude run names its pre-recorded UUID, and a resumed
// run names the exact recorded ID.
func TaskArgs(s TaskRunSpec) ([]string, error) {
	if s.CLI == "" || s.Messenger == "" || s.DB == "" || s.RunID == "" || s.Receipt == "" {
		return nil, errors.New("incomplete task run")
	}
	if s.Profile != ProfileWorker && s.Profile != ProfileMessengerOnly {
		return nil, errors.New("unknown worker profile")
	}
	if s.Model != "" && !taskModelPattern.MatchString(s.Model) {
		return nil, errors.New("invalid model name")
	}
	switch s.Provider {
	case "claude":
		return claudeTaskArgs(s)
	case "codex":
		return codexTaskArgs(s)
	case "cursor":
		return cursorTaskArgs(s)
	}
	return nil, fmt.Errorf("unsupported task agent %q", s.Provider)
}

func claudeTaskArgs(s TaskRunSpec) ([]string, error) {
	if !uuidPattern.MatchString(s.Session) {
		return nil, errors.New("a Claude task needs its recorded session UUID")
	}
	config, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{
		TaskServerName: map[string]any{"command": s.Messenger, "args": taskMCPArgs(s)},
	}})
	var allowed []string
	for _, name := range TaskToolNames {
		allowed = append(allowed, "mcp__"+TaskServerName+"__"+name)
	}
	args := []string{s.CLI, "-p", "--output-format", "stream-json", "--verbose", "--mcp-config", string(config)}
	if s.Profile == ProfileMessengerOnly {
		args = append(args, "--tools", "", "--strict-mcp-config", "--setting-sources", "",
			"--settings", `{"disableAllHooks":true}`, "--disable-slash-commands", "--no-chrome", "--permission-mode", "dontAsk")
	}
	// The worker profile keeps the project's settings, permissions and tools;
	// only the Messenger tools are pre-approved. File edits are accepted
	// (D-028): a headless run cannot ask, and Codex workers may write to the
	// project folder too. Shell commands still follow the project's rules.
	if s.Profile == ProfileWorker {
		mode := "acceptEdits"
		if s.FullAccess {
			mode = "bypassPermissions"
		}
		args = append(args, "--permission-mode", mode)
	}
	if s.Model != "" {
		args = append(args, "--model", s.Model)
	}
	args = append(args, "--allowedTools", strings.Join(allowed, ","))
	if s.Resume {
		args = append(args, "--resume", s.Session)
	} else {
		args = append(args, "--session-id", s.Session)
	}
	args = append(args, taskPrompt(s.Profile))
	return args, ExplicitResume(args, s.Session)
}

func codexTaskArgs(s TaskRunSpec) ([]string, error) {
	if s.Resume && s.Session == "" {
		return nil, errors.New("a resumed Codex task needs its recorded thread id")
	}
	if !s.Resume && s.Session != "" {
		return nil, errors.New("a new Codex task must not name a thread")
	}
	if s.Session != "" {
		if err := rejectLooseID(s.Session); err != nil {
			return nil, err
		}
	}
	args := []string{s.CLI, "exec", "--json", "--skip-git-repo-check"}
	if s.Profile == ProfileMessengerOnly {
		args = append(args, "--ignore-user-config", "--sandbox", "read-only")
		for _, feature := range []string{"shell_tool", "apps", "plugins", "hooks", "multi_agent", "browser_use", "computer_use"} {
			args = append(args, "--disable", feature)
		}
	} else {
		// D-023: normal tools in the project folder, which is writable. The
		// project's own .codex/config.toml applies when the folder is trusted.
		if s.FullAccess {
			args = append(args, "--dangerously-bypass-approvals-and-sandbox")
		} else {
			args = append(args, "--sandbox", "workspace-write")
		}
	}
	server := map[string]any{
		"command":                     s.Messenger,
		"args":                        taskMCPArgs(s),
		"required":                    true,
		"default_tools_approval_mode": "approve",
	}
	for _, key := range []string{"command", "args", "required", "default_tools_approval_mode"} {
		encoded, _ := json.Marshal(server[key])
		args = append(args, "-c", "mcp_servers."+TaskServerName+"."+key+"="+string(encoded))
	}
	if s.Model != "" {
		encoded, _ := json.Marshal(s.Model)
		args = append(args, "-c", "model="+string(encoded))
	}
	if s.Resume {
		args = append(args, "resume", s.Session, taskPrompt(s.Profile))
		return args, ExplicitResume(args, s.Session)
	}
	return append(args, taskPrompt(s.Profile)), nil
}

// TaskEventResult is what a worker run's event stream showed.
type TaskEventResult struct {
	// Session is the conversation ID the CLI reported first; Conflicting is
	// set when it later reported a different one.
	Session     string
	Conflicting bool
	// Completed means the provider reported a successful end of the run.
	Completed bool
	Failure   string
	// Reason is the failure class (Reason* constants) and ResetAt the time
	// the provider said its limit resets; both only explain the failure.
	Reason  string
	ResetAt time.Time
	// Limit is the last usage-limit state the agent reported (T-044).
	Limit *LimitState
	// LastText is the worker's last message, shortened. It only explains a
	// run that ended without a report (T-056).
	LastText string
	// FinalText is the worker's last message in full (up to FinalTextLimit
	// characters), kept so a long final answer is never lost (T-087).
	FinalText string
	// Usage is what the run spent, when the provider reported it (T-058).
	Usage *Usage
	// Model is the model the CLI said it runs on, from its start event
	// (T-108); empty when the CLI does not say.
	Model string
}

// noteModel keeps the model a start event names.
func (r *TaskEventResult) noteModel(e map[string]any) {
	if e["type"] != "system" || e["subtype"] != "init" {
		return
	}
	if m, _ := e["model"].(string); m != "" && len(m) <= 120 {
		r.Model = m
	}
}

// lastTextLimit bounds LastText in characters.
const lastTextLimit = 300

// FinalTextLimit bounds FinalText in characters.
const FinalTextLimit = 60000

func (r *TaskEventResult) noteText(text string) {
	if full := strings.TrimSpace(text); full != "" {
		if runes := []rune(full); len(runes) > FinalTextLimit {
			full = string(runes[:FinalTextLimit]) + "…"
		}
		r.FinalText = full
	}
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return
	}
	if runes := []rune(text); len(runes) > lastTextLimit {
		text = string(runes[:lastTextLimit]) + "…"
	}
	r.LastText = text
}

// taskNow is the clock used to read "try again at 6:12 PM"; tests replace it.
var taskNow = time.Now

// ParseTaskEvent folds one JSON event line of a worker run into r. Claude
// reports its session ID in every event and ends with a "result" event; Codex
// reports "thread.started" and ends a turn with "turn.completed". Anything
// that says the run failed is kept as Failure.
func ParseTaskEvent(provider string, line []byte, r *TaskEventResult) {
	var e map[string]any
	if json.Unmarshal(line, &e) != nil {
		return
	}
	note := func(id string) {
		if id == "" {
			return
		}
		if r.Session == "" {
			r.Session = id
		} else if r.Session != id {
			r.Conflicting = true
		}
	}
	kind, _ := e["type"].(string)
	switch provider {
	case "claude":
		id, _ := e["session_id"].(string)
		note(id)
		r.noteModel(e)
		switch kind {
		case "result":
			if text, _ := e["result"].(string); text != "" {
				r.noteText(text)
			}
			claudeUsage(e, r)
			subtype, _ := e["subtype"].(string)
			isErr, _ := e["is_error"].(bool)
			if subtype == "success" && !isErr {
				r.Completed = true
			} else {
				r.Completed = false
				r.Failure = "Claude result " + subtype
				r.noteFailure(errorText(e), taskNow())
			}
		case "rate_limit_event":
			// Claude reports its plan limits; only a rejected request is a
			// limit that stopped this run.
			info, _ := e["rate_limit_info"].(map[string]any)
			if l, ok := claudeLimit(info); ok {
				r.noteLimit(l)
			}
			if status, _ := info["status"].(string); status == "rejected" {
				r.Reason = stronger(r.Reason, ReasonUsageLimit)
				if at := unixReset(info["resetsAt"]); !at.IsZero() && r.ResetAt.IsZero() {
					r.ResetAt = at
				}
			}
		case "assistant":
			// An API error is reported as an assistant message with an error
			// code; the run then ends with a failed result.
			if code, _ := e["error"].(string); code != "" {
				text := errorText(e)
				reason := classifyFailure(text)
				switch code {
				case "authentication_failed":
					reason = ReasonSignIn
				case "billing_error":
					reason = ReasonUsageLimit
				case "rate_limit":
					if reason != ReasonUsageLimit {
						reason = ReasonRateLimit
					}
				}
				r.Reason = stronger(r.Reason, reason)
				if at := resetFromText(text, taskNow()); !at.IsZero() && r.ResetAt.IsZero() {
					r.ResetAt = at
				}
			}
		}
	case "codex":
		if l, ok := codexLimit(e); ok {
			r.noteLimit(l)
		}
		switch kind {
		case "thread.started":
			id, _ := e["thread_id"].(string)
			note(id)
		case "item.completed":
			if item, _ := e["item"].(map[string]any); item["type"] == "agent_message" {
				text, _ := item["text"].(string)
				r.noteText(text)
			}
		case "turn.completed":
			codexUsage(e, r)
			if r.Failure == "" {
				r.Completed = true
			}
		case "turn.failed", "error":
			r.Completed = false
			text := errorText(e)
			r.Failure = withErrorText("Codex "+kind, text)
			r.noteFailure(text, taskNow())
		}
	case "cursor":
		parseCursorEvent(e, kind, r, note)
	}
}

// FeedEntry is one item of a worker's event stream kept for the window: the
// worker's own text, or one tool action shown as a collapsible line.
type FeedEntry struct {
	Kind  string // "text" or "tool"
	Title string
	Text  string
}

func toolSummary(input map[string]any) string {
	for _, key := range []string{"command", "cmd", "file_path", "path", "pattern", "query", "url", "description", "prompt"} {
		if v, ok := input[key].(string); ok && v != "" {
			return v
		}
	}
	if len(input) == 0 {
		return ""
	}
	raw, _ := json.Marshal(input)
	return string(raw)
}

// TaskFeedEntries turns one event line into window items. Calls of the
// Messenger worker tools are left out: the report and questions are shown
// from their verified records instead.
func TaskFeedEntries(provider string, line []byte) []FeedEntry {
	var e map[string]any
	if json.Unmarshal(line, &e) != nil {
		return nil
	}
	var out []FeedEntry
	switch provider {
	case "cursor":
		return cursorFeedEntries(e)
	case "claude":
		if e["type"] != "assistant" {
			return nil
		}
		message, _ := e["message"].(map[string]any)
		content, _ := message["content"].([]any)
		for _, c := range content {
			block, _ := c.(map[string]any)
			switch block["type"] {
			case "text":
				if text, _ := block["text"].(string); strings.TrimSpace(text) != "" {
					out = append(out, FeedEntry{Kind: "text", Text: text})
				}
			case "tool_use":
				name, _ := block["name"].(string)
				// ToolSearch only loads tool definitions; it means nothing to
				// the owner.
				if strings.HasPrefix(name, "mcp__"+TaskServerName+"__") || name == "ToolSearch" {
					continue
				}
				input, _ := block["input"].(map[string]any)
				out = append(out, FeedEntry{Kind: "tool", Title: name, Text: toolSummary(input)})
			}
		}
	case "codex":
		if e["type"] != "item.completed" {
			return nil
		}
		item, _ := e["item"].(map[string]any)
		switch item["type"] {
		case "agent_message":
			if text, _ := item["text"].(string); strings.TrimSpace(text) != "" {
				out = append(out, FeedEntry{Kind: "text", Text: text})
			}
		case "command_execution":
			command, _ := item["command"].(string)
			out = append(out, FeedEntry{Kind: "tool", Title: "Команда", Text: command})
		case "file_change":
			var paths []string
			changes, _ := item["changes"].([]any)
			for _, c := range changes {
				if m, ok := c.(map[string]any); ok {
					if p, _ := m["path"].(string); p != "" {
						paths = append(paths, p)
					}
				}
			}
			out = append(out, FeedEntry{Kind: "tool", Title: "Изменение файлов", Text: strings.Join(paths, "\n")})
		case "mcp_tool_call":
			server, _ := item["server"].(string)
			tool, _ := item["tool"].(string)
			if server == TaskServerName {
				return nil
			}
			out = append(out, FeedEntry{Kind: "tool", Title: server + "." + tool})
		case "web_search":
			query, _ := item["query"].(string)
			out = append(out, FeedEntry{Kind: "tool", Title: "Поиск в интернете", Text: query})
		}
	}
	return out
}
