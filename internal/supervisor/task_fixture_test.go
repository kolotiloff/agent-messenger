package supervisor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// taskWorkerFixture imitates a Claude (`-p`) or Codex (`exec`) worker run. It
// is not a model. It reports its conversation ID as the real CLIs do, starts
// the messenger_task MCP server named in its arguments, calls read_task, and
// ends according to mode. Every run appends its argv and working directory to
// $MESSENGER_TASK_LOG.
func taskWorkerFixture(mode string) int {
	args := os.Args[1:]
	codex := len(args) > 0 && args[0] == "exec"
	cursor := false
	for _, a := range args {
		cursor = cursor || a == "--approve-mcps"
	}
	wd, _ := os.Getwd()
	if logPath := os.Getenv("MESSENGER_TASK_LOG"); logPath != "" {
		line, _ := json.Marshal(map[string]any{"args": args, "cwd": wd})
		if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
			f.Write(append(line, '\n'))
			f.Close()
		}
	}
	var session, command string
	var mcpArgs []string
	resume := false
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--session-id" && i+1 < len(args):
			session = args[i+1]
		case args[i] == "--resume" && i+1 < len(args):
			session, resume = args[i+1], true
		case strings.HasPrefix(args[i], "--resume="):
			session, resume = strings.TrimPrefix(args[i], "--resume="), true
		case codex && args[i] == "resume" && i+1 < len(args):
			session, resume = args[i+1], true
		case args[i] == "--mcp-config" && i+1 < len(args):
			var cfg struct {
				MCPServers map[string]struct {
					Command string   `json:"command"`
					Args    []string `json:"args"`
				} `json:"mcpServers"`
			}
			_ = json.Unmarshal([]byte(args[i+1]), &cfg)
			server := cfg.MCPServers["messenger_task"]
			command, mcpArgs = server.Command, server.Args
		case args[i] == "-c" && i+1 < len(args):
			key, value, _ := strings.Cut(args[i+1], "=")
			switch key {
			case "mcp_servers.messenger_task.command":
				_ = json.Unmarshal([]byte(value), &command)
			case "mcp_servers.messenger_task.args":
				_ = json.Unmarshal([]byte(value), &mcpArgs)
			}
		}
	}
	var serverEnv []string
	if cursor {
		// Like Cursor: the server comes from mcp.json, and its environment
		// is a short default list plus the entry's env, where ${env:NAME}
		// takes this process's value.
		dir := os.Getenv("CURSOR_CONFIG_DIR")
		if dir == "" {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, ".cursor")
		}
		raw, _ := os.ReadFile(filepath.Join(dir, "mcp.json"))
		var cfg struct {
			MCPServers map[string]struct {
				Command string            `json:"command"`
				Args    []string          `json:"args"`
				Env     map[string]string `json:"env"`
			} `json:"mcpServers"`
		}
		_ = json.Unmarshal(raw, &cfg)
		entry := cfg.MCPServers["messenger_task"]
		command, mcpArgs = entry.Command, entry.Args
		for _, name := range []string{"HOME", "PATH", "USERPROFILE", "SYSTEMROOT", "TEMP", "TMP", "LOCALAPPDATA"} {
			if v, ok := os.LookupEnv(name); ok {
				serverEnv = append(serverEnv, name+"="+v)
			}
		}
		for name, value := range entry.Env {
			if strings.HasPrefix(value, "${env:") && strings.HasSuffix(value, "}") {
				value = os.Getenv(strings.TrimSuffix(strings.TrimPrefix(value, "${env:"), "}"))
			}
			serverEnv = append(serverEnv, name+"="+value)
		}
		if session == "" {
			session = fmt.Sprintf("cursor-chat-%d", time.Now().UnixNano())
		}
	}
	if codex && session == "" {
		session = "019a0000-0000-7000-8000-" + fmt.Sprintf("%012d", time.Now().UnixNano()%1e12)
	}
	if mode == "wrong-session" {
		session = "00000000-0000-4000-8000-000000000bad"
	}
	emit := func(v any) {
		line, _ := json.Marshal(v)
		fmt.Printf("%s\n", line)
	}
	if codex {
		emit(map[string]any{"type": "thread.started", "thread_id": session})
	} else {
		emit(map[string]any{"type": "system", "subtype": "init", "session_id": session})
	}
	_ = resume

	server := exec.Command(command, mcpArgs...)
	if cursor {
		server.Env = serverEnv
	}
	in, _ := server.StdinPipe()
	outPipe, _ := server.StdoutPipe()
	if err := server.Start(); err != nil {
		return 3
	}
	reader := bufio.NewReader(outPipe)
	id := 0
	call := func(method string, params any) (map[string]any, error) {
		id++
		line, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
		if _, err := in.Write(append(line, '\n')); err != nil {
			return nil, err
		}
		raw, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, err
		}
		var resp struct {
			Result map[string]any `json:"result"`
		}
		return resp.Result, json.Unmarshal(raw, &resp)
	}
	tool := func(name string, arguments map[string]any) (string, bool) {
		result, err := call("tools/call", map[string]any{"name": name, "arguments": arguments})
		if err != nil {
			return err.Error(), false
		}
		isErr, _ := result["isError"].(bool)
		content, _ := result["content"].([]any)
		text := ""
		if len(content) == 1 {
			item, _ := content[0].(map[string]any)
			text, _ = item["text"].(string)
		}
		return text, !isErr
	}
	defer func() {
		in.Close()
		_ = server.Wait()
	}()
	if _, err := call("initialize", map[string]any{"protocolVersion": "2025-06-18"}); err != nil {
		return 4
	}
	var raw string
	var ok bool
	// Tools open once Messenger has bound the conversation; retry briefly.
	for deadline := time.Now().Add(5 * time.Second); ; {
		raw, ok = tool("read_task", map[string]any{})
		if ok || time.Now().After(deadline) || mode == "wrong-session" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	var brief struct {
		Task, Answer, Question string
	}
	_ = json.Unmarshal([]byte(raw), &brief)
	finish := func() {
		if codex {
			emit(map[string]any{"type": "turn.completed"})
		} else {
			emit(map[string]any{"type": "result", "subtype": "success", "is_error": false, "session_id": session})
		}
	}
	switch mode {
	case "report", "wrong-session":
		tool("report", map[string]any{"text": "done: " + brief.Task})
	case "ask-then-report":
		if brief.Answer == "" {
			tool("ask", map[string]any{"text": "Which level?"})
		} else {
			// T-033: a resumed run gets no task text unless it asks.
			var again struct{ Task string }
			full, _ := tool("read_task", map[string]any{"include_task": true})
			_ = json.Unmarshal([]byte(full), &again)
			if brief.Task != "" || again.Task == "" {
				tool("report", map[string]any{"text": "task text: first " + brief.Task + ", again " + again.Task})
				break
			}
			tool("report", map[string]any{"text": "done with " + brief.Answer + " for " + brief.Question})
		}
	case "ask-owner":
		if brief.Answer == "" {
			tool("ask_owner", map[string]any{"text": "May I delete the old saves?"})
		} else {
			tool("report", map[string]any{"text": "owner said " + brief.Answer})
		}
	case "exit1":
		tool("report", map[string]any{"text": "claims done"})
		finish()
		return 1
	case "usage-limit":
		// The provider's limit, in the documented event shapes (T-034).
		if codex {
			emit(map[string]any{"type": "turn.failed", "error": map[string]any{"message": "You've hit your usage limit. Try again at 18:12."}})
		} else if cursor {
			emit(map[string]any{"type": "result", "subtype": "error", "is_error": true, "session_id": session, "result": "You've hit your usage limit. Try again at 18:12."})
		} else {
			emit(map[string]any{"type": "rate_limit_event", "session_id": session, "rate_limit_info": map[string]any{"status": "rejected", "resetsAt": 1762538400, "rateLimitType": "five_hour"}})
			emit(map[string]any{"type": "result", "subtype": "success", "is_error": true, "session_id": session, "result": "Claude AI usage limit reached|1762538400"})
		}
		return 1
	case "no-disposition":
	case "two-dispositions":
		tool("report", map[string]any{"text": "first"})
		if _, ok := tool("ask", map[string]any{"text": "second"}); ok {
			return 5
		}
	case "slow":
		time.Sleep(30 * time.Second)
	case "sleep":
		// A leftover child of "leave-child" on Windows.
		time.Sleep(30 * time.Second)
		return 0
	case "leave-child":
		tool("report", map[string]any{"text": "done, but a child stays"})
		child := exec.Command("/bin/sh", "-c", `trap "" TERM; sleep 30`)
		if runtime.GOOS == "windows" {
			self, _ := os.Executable()
			child = exec.Command(self)
			child.Env = append(os.Environ(), "MESSENGER_TASK_FIXTURE=sleep")
		}
		if err := child.Start(); err != nil {
			return 6
		}
		_ = os.WriteFile(filepath.Join(os.Getenv("MESSENGER_TASK_DIR"), "child.pid"), []byte(fmt.Sprint(child.Process.Pid)), 0o600)
	}
	finish()
	return 0
}
