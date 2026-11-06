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

// fakeCodexServer imitates `codex app-server --listen stdio://` for setup and
// turns. It never runs a model: a turn reads the message through read_current
// and replies "ACK:<text>". The mode is read from CODEX_HOME/fixture-mode and
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
	// await reads the client's answer to one server request.
	await := func(id int) (json.RawMessage, bool) {
		if !scanner.Scan() {
			return nil, false
		}
		var resp struct {
			ID     *int            `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &resp) != nil || resp.ID == nil || *resp.ID != id || resp.Method != "" {
			fmt.Fprintf(logFile, "bad-response\n")
			return nil, false
		}
		if len(resp.Error) != 0 {
			fmt.Fprintf(logFile, "response:%d:error\n", id)
			return nil, true
		}
		fmt.Fprintf(logFile, "response:%d:ok\n", id)
		return resp.Result, true
	}
	turns := 0
	currentThread := fakeThreadID
	threadIndex := 0
	configReads := 0
	runTurn := func(reqID int) bool {
		turns++
		turnMode := mode
		if mode == "second-fails" {
			turnMode = "ok"
			if turns == 2 {
				turnMode = "turn-failed"
			}
		}
		if turnMode == "exit-in-turn" {
			return false
		}
		turnID := fmt.Sprintf("0190aaaa-%04d-7000-8000-%012d", threadIndex, turns)
		call := func(id int, tool string, args any, turn string) (json.RawMessage, bool) {
			send(map[string]any{"id": id, "method": "item/tool/call", "params": map[string]any{
				"threadId": currentThread, "turnId": turn, "callId": fmt.Sprintf("call-%d", id),
				"namespace": "messenger_bound", "tool": tool, "arguments": args,
			}})
			return await(id)
		}
		base := 1000 + 10*turns
		readID := base
		if turnMode == "early-call" {
			send(map[string]any{"id": readID, "method": "item/tool/call", "params": map[string]any{
				"threadId": currentThread, "turnId": turnID, "callId": "early", "namespace": "messenger_bound", "tool": "read_current", "arguments": map[string]any{},
			}})
		}
		send(map[string]any{"method": "turn/started", "params": map[string]any{"threadId": currentThread, "turn": map[string]any{"id": turnID, "status": "inProgress", "items": []any{}}}})
		send(map[string]any{"id": reqID, "result": map[string]any{"turn": map[string]any{"id": turnID, "status": "inProgress", "items": []any{}}}})
		if turnMode == "approval" {
			send(map[string]any{"id": base + 5, "method": "item/commandExecution/requestApproval", "params": map[string]any{}})
			if _, ok := await(base + 5); !ok {
				return false
			}
		}
		var read json.RawMessage
		var ok bool
		switch turnMode {
		case "early-call":
			read, ok = await(readID)
		case "foreign-turn":
			read, ok = call(readID, "read_current", map[string]any{}, "0190ffff-0000-7000-8000-000000000000")
		default:
			read, ok = call(readID, "read_current", map[string]any{}, turnID)
		}
		if !ok || read == nil {
			return ok
		}
		var output struct {
			ContentItems []struct{ Text string } `json:"contentItems"`
		}
		var message struct{ Text, Receipt string }
		if json.Unmarshal(read, &output) != nil || len(output.ContentItems) != 1 || json.Unmarshal([]byte(output.ContentItems[0].Text), &message) != nil {
			return false
		}
		if turnMode != "no-reply" {
			replyID := base + 1
			if turnMode == "dup-request" {
				replyID = readID
			}
			if _, ok := call(replyID, "reply", map[string]any{"receipt": message.Receipt, "text": "ACK:" + message.Text}, turnID); !ok {
				return false
			}
		}
		status := "completed"
		if turnMode == "turn-failed" {
			status = "failed"
		}
		send(map[string]any{"method": "turn/completed", "params": map[string]any{"threadId": currentThread, "turn": map[string]any{"id": turnID, "status": status, "items": []any{}}}})
		return true
	}
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
			configReads++
			// Like the real app-server, key order changes between calls.
			parts := []string{`"sandbox_mode":"read-only"`, `"approval_policy":"never"`, `"web_search":"disabled"`}
			featuresJSON, _ := json.Marshal(features)
			serversJSON, _ := json.Marshal(servers)
			parts = append(parts, `"features":`+string(featuresJSON), `"mcp_servers":`+string(serversJSON))
			if configReads%2 == 0 {
				for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
					parts[i], parts[j] = parts[j], parts[i]
				}
			}
			if mode == "config-change" && configReads > 1 {
				parts = append(parts, `"model":"changed-after-setup"`)
			}
			raw := `{"id":` + fmt.Sprint(*req.ID) + `,"result":{"config":{` + strings.Join(parts, ",") + `}}}`
			out.WriteString(raw + "\n")
			out.Flush()
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
			// The first thread in a profile gets fakeThreadID; later ones differ.
			threadID := fakeThreadID
			counter := filepath.Join(home, "fixture-threads")
			previous, _ := os.ReadFile(counter)
			if n := len(previous); n > 0 {
				threadID = fmt.Sprintf("01a1b0a0-1ef6-7861-8fd1-%012d", n)
			}
			_ = os.WriteFile(counter, append(previous, 'x'), 0o600)
			currentThread = threadID
			threadIndex = len(previous)
			thread := map[string]any{"id": threadID, "cwd": params.Cwd, "ephemeral": mode == "ephemeral", "status": map[string]any{"type": "idle"}, "turns": []any{}}
			send(map[string]any{"method": "thread/started", "params": map[string]any{"thread": thread}})
			send(map[string]any{"id": *req.ID, "result": map[string]any{
				"approvalPolicy": "never", "sandbox": map[string]any{"type": "readOnly"}, "cwd": params.Cwd, "thread": thread,
			}})
			if mode == "idle-request" {
				send(map[string]any{"id": 77, "method": "item/tool/call", "params": map[string]any{}})
			}
		case "turn/start":
			if !runTurn(*req.ID) {
				return 0
			}
		default:
			return 4
		}
	}
	return 0
}
