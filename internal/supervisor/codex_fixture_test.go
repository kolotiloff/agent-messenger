package supervisor

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const fakeThreadID = "01a1b0a0-1ef6-7861-8fd1-f7fb5912e2f9"

// fakeCodexServer imitates `codex app-server --listen stdio://` for setup.
// It never runs a model. The mode is read from CODEX_HOME/fixture-mode and
// every received method is appended to CODEX_HOME/fixture-log.
func fakeCodexServer() int {
	home := os.Getenv("CODEX_HOME")
	modeRaw, _ := os.ReadFile(filepath.Join(home, "fixture-mode"))
	mode := strings.TrimSpace(string(modeRaw))
	logFile, err := os.OpenFile(filepath.Join(home, "fixture-log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 2
	}
	defer logFile.Close()
	var keys []string
	for _, kv := range os.Environ() {
		name, _, _ := strings.Cut(kv, "=")
		keys = append(keys, name)
	}
	sort.Strings(keys)
	fmt.Fprintf(logFile, "env:%s\n", strings.Join(keys, ","))
	fmt.Fprintf(logFile, "args:%s\n", strings.Join(os.Args[1:4], " "))
	out := bufio.NewWriter(os.Stdout)
	send := func(v any) {
		raw, _ := json.Marshal(v)
		out.Write(append(raw, '\n'))
		out.Flush()
	}
	features := map[string]bool{}
	for _, f := range []string{"shell_tool", "shell_snapshot", "apps", "plugins", "hooks", "multi_agent", "browser_use", "computer_use", "goals", "remote_plugin", "remote_control", "memories"} {
		features[f] = false
	}
	servers := map[string]any{}
	if mode == "mcp" {
		servers["inherited"] = map[string]any{"enabled": true}
	}
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64<<10), 4<<20)
	for scanner.Scan() {
		var req struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(scanner.Bytes(), &req) != nil {
			return 3
		}
		fmt.Fprintf(logFile, "%s\n", req.Method)
		switch req.Method {
		case "initialize":
			send(map[string]any{"method": "configWarning", "params": map[string]any{"summary": "fixture"}})
			send(map[string]any{"id": *req.ID, "result": map[string]any{"userAgent": "fake"}})
		case "initialized":
		case "config/read":
			if mode == "server-request" {
				send(map[string]any{"id": 99, "method": "item/tool/call", "params": map[string]any{}})
				continue
			}
			send(map[string]any{"id": *req.ID, "result": map[string]any{"config": map[string]any{
				"sandbox_mode": "read-only", "approval_policy": "never", "web_search": "disabled",
				"features": features, "mcp_servers": servers,
			}}})
		case "thread/start":
			var params struct {
				Cwd string `json:"cwd"`
			}
			_ = json.Unmarshal(req.Params, &params)
			switch mode {
			case "silent-thread":
				continue
			case "exit-on-thread":
				return 0
			}
			thread := map[string]any{"id": fakeThreadID, "cwd": params.Cwd, "ephemeral": mode == "ephemeral", "status": map[string]any{"type": "idle"}, "turns": []any{}}
			send(map[string]any{"method": "thread/started", "params": map[string]any{"thread": thread}})
			send(map[string]any{"id": *req.ID, "result": map[string]any{
				"approvalPolicy": "never", "sandbox": map[string]any{"type": "readOnly"}, "cwd": params.Cwd, "thread": thread,
			}})
			if mode == "idle-request" {
				send(map[string]any{"id": 77, "method": "item/tool/call", "params": map[string]any{}})
			}
		default:
			// turn/start or anything else is a test failure.
			return 4
		}
	}
	return 0
}
