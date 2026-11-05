// Package adapters builds explicit resume commands and checks CLI events.
// It does not execute Claude, Codex or Cursor.
package adapters

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const prompt = "Authorized local Messenger test. Use ONLY messenger_bound MCP tools. " +
	"First call read_current. Then call reply with the response text. " +
	"Do not select an address or message ID. Do not poll or execute shell commands."

var errIdentity = errors.New("session identity mismatch")

// ExplicitResume reports argv that would continue an unbound or latest session.
func ExplicitResume(args []string, conversationID string) error {
	if conversationID == "" {
		return errors.New("exact conversation id is required")
	}
	joined := "\n" + strings.Join(args, "\n") + "\n"
	for _, banned := range []string{"--last", "--continue", "--resume-last"} {
		if strings.Contains(joined, "\n"+banned+"\n") || strings.Contains(joined, banned+"=") {
			return fmt.Errorf("banned resume mode %s", banned)
		}
	}
	if !strings.Contains(joined, conversationID) {
		return errors.New("exact conversation id is absent")
	}
	return nil
}

func ClaudeArgs(cli, mcpBin, db, token, conversationID string) ([]string, error) {
	if err := rejectLooseID(conversationID); err != nil {
		return nil, err
	}
	config, _ := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			"messenger_bound": map[string]any{
				"command": mcpBin,
				"args":    []string{"mcp", "--db", db, "--token", token},
			},
		},
	})
	args := []string{
		cli, "-p", "--output-format", "stream-json", "--verbose",
		"--tools", "", "--strict-mcp-config", "--mcp-config", string(config),
		"--setting-sources", "", "--settings", `{"disableAllHooks":true}`,
		"--disable-slash-commands", "--no-chrome", "--permission-mode", "dontAsk",
		"--allowedTools", "mcp__messenger_bound__read_current,mcp__messenger_bound__reply",
		"--resume", conversationID, prompt,
	}
	return args, ExplicitResume(args, conversationID)
}

func CodexArgs(cli, mcpBin, db, token, conversationID string) ([]string, error) {
	if err := rejectLooseID(conversationID); err != nil {
		return nil, err
	}
	args := []string{cli, "exec", "--ignore-user-config", "--sandbox", "read-only", "--skip-git-repo-check", "--json"}
	for _, feature := range []string{"shell_tool", "apps", "plugins", "hooks", "multi_agent", "browser_use", "computer_use"} {
		args = append(args, "--disable", feature)
	}
	server := map[string]any{
		"command":                     mcpBin,
		"args":                        []string{"mcp", "--db", db, "--token", token},
		"required":                    true,
		"enabled_tools":               []string{"read_current", "reply"},
		"default_tools_approval_mode": "approve",
	}
	for key, value := range server {
		encoded, _ := json.Marshal(value)
		args = append(args, "-c", "mcp_servers.messenger_bound."+key+"="+string(encoded))
	}
	args = append(args, "resume", conversationID, prompt)
	return args, ExplicitResume(args, conversationID)
}

// PrepareCursorWorkspace writes the test MCP and permission files only inside workspace.
// It does not read or change the user's global Cursor profile.
func PrepareCursorWorkspace(workspace, mcpBin, db, token string) error {
	if workspace == "" {
		return errors.New("cursor workspace is required")
	}
	dir := filepath.Join(workspace, ".cursor")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	mcp, err := json.Marshal(map[string]any{
		"mcpServers": map[string]any{
			"messenger_bound": map[string]any{
				"command": mcpBin,
				"args":    []string{"mcp", "--db", db, "--token", token},
			},
		},
	})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, "mcp.json"), mcp, 0o600); err != nil {
		return err
	}
	cli, err := json.Marshal(map[string]any{
		"permissions": map[string]any{
			"allow": []string{"Mcp(messenger_bound:read_current)", "Mcp(messenger_bound:reply)"},
			"deny":  []string{"Shell(*)", "Read(*)", "Write(*)", "WebFetch(*)"},
		},
	})
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "cli.json"), cli, 0o600)
}

func CursorArgs(cli, conversationID, workspace string) ([]string, error) {
	if err := rejectLooseID(conversationID); err != nil {
		return nil, err
	}
	args := []string{
		cli, "-p", "--resume=" + conversationID, "--output-format", "stream-json",
		"--sandbox", "enabled", "--trust", "--approve-mcps", "--workspace", workspace, prompt,
	}
	return args, ExplicitResume(args, conversationID)
}

// CheckCursorIsolation refuses a launch profile that already has a global MCP config.
func CheckCursorIsolation(home, configDir string) error {
	if configDir != "" {
		return errors.New("CURSOR_CONFIG_DIR is set and must be inspected before a Cursor launch")
	}
	path := filepath.Join(home, ".cursor", "mcp.json")
	if _, err := os.Stat(path); err == nil {
		return errors.New("global Cursor MCP config exists")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func rejectLooseID(id string) error {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "", "last", "continue", "latest":
		return errors.New("exact conversation id is required")
	default:
		return nil
	}
}

func StartupID(provider string, event map[string]any) (string, error) {
	switch provider {
	case "codex":
		if event["type"] != "thread.started" {
			return "", nil
		}
		id, _ := event["thread_id"].(string)
		if id == "" {
			return "", fmt.Errorf("%w: missing Codex id", errIdentity)
		}
		return id, nil
	case "cursor", "claude", "synthetic":
		if event["type"] != "system" || event["subtype"] != "init" {
			return "", nil
		}
		id, _ := event["session_id"].(string)
		if id == "" {
			return "", fmt.Errorf("%w: missing session id", errIdentity)
		}
		return id, nil
	default:
		return "", fmt.Errorf("unknown provider %s", provider)
	}
}

func ValidateClaude(events []map[string]any, conversationID string) error {
	var results []map[string]any
	var calls []string
	for _, event := range events {
		if event["type"] == "result" {
			results = append(results, event)
		}
		if event["type"] != "assistant" {
			continue
		}
		message, _ := event["message"].(map[string]any)
		content, _ := message["content"].([]any)
		for _, item := range content {
			block, _ := item.(map[string]any)
			if block["type"] == "tool_use" {
				name, _ := block["name"].(string)
				calls = append(calls, name)
			}
		}
	}
	if len(results) != 1 {
		return errors.New("claude did not finish with one result")
	}
	result := results[0]
	if result["session_id"] != conversationID || result["subtype"] != "success" || truthy(result["is_error"]) || present(result["permission_denials"]) {
		return fmt.Errorf("%w: claude result", errIdentity)
	}
	if strings.Join(calls, ",") != "mcp__messenger_bound__read_current,mcp__messenger_bound__reply" {
		return errors.New("unexpected claude tool sequence")
	}
	return nil
}

func ValidateCodex(events []map[string]any, conversationID string) error {
	var started []string
	var calls []string
	completed := 0
	for _, event := range events {
		switch event["type"] {
		case "thread.started":
			id, _ := event["thread_id"].(string)
			started = append(started, id)
		case "turn.completed":
			completed++
		case "error", "turn.failed":
			return errors.New("codex error event")
		case "item.completed":
			item, _ := event["item"].(map[string]any)
			kind, _ := item["type"].(string)
			if kind == "mcp_tool_call" {
				if item["server"] != "messenger_bound" || item["status"] != "completed" || present(item["error"]) {
					return errors.New("failed or foreign codex mcp call")
				}
				tool, _ := item["tool"].(string)
				calls = append(calls, tool)
			} else if kind != "agent_message" && kind != "reasoning" && kind != "plan_update" {
				return errors.New("unexpected codex action")
			}
		}
	}
	if len(started) != 1 || started[0] != conversationID {
		return fmt.Errorf("%w: codex id", errIdentity)
	}
	if completed != 1 {
		return errors.New("missing or duplicate codex completion")
	}
	if strings.Join(calls, ",") != "read_current,reply" {
		return errors.New("unexpected codex tool sequence")
	}
	return nil
}

func ValidateCursor(events []map[string]any, conversationID string) error {
	var starts []string
	var calls []string
	for _, event := range events {
		if sid, ok := event["session_id"].(string); ok && sid != conversationID {
			return fmt.Errorf("%w: cursor event changed session", errIdentity)
		}
		if event["type"] == "system" && event["subtype"] == "init" {
			id, _ := event["session_id"].(string)
			starts = append(starts, id)
		}
		if event["type"] == "error" {
			return errors.New("cursor error event")
		}
		if event["type"] != "tool_call" || event["subtype"] != "completed" {
			continue
		}
		call, _ := event["tool_call"].(map[string]any)
		if _, ok := call["getMcpToolsToolCall"]; ok {
			continue
		}
		mcp, ok := call["mcpToolCall"].(map[string]any)
		if !ok {
			return errors.New("unexpected cursor tool")
		}
		args, _ := mcp["args"].(map[string]any)
		if args["providerIdentifier"] != "messenger_bound" {
			return errors.New("foreign cursor mcp")
		}
		result, _ := mcp["result"].(map[string]any)
		success, _ := result["success"].(map[string]any)
		if success == nil || truthy(success["isError"]) {
			return errors.New("cursor mcp error")
		}
		name, _ := args["toolName"].(string)
		calls = append(calls, name)
	}
	if len(starts) != 1 || starts[0] != conversationID {
		return fmt.Errorf("%w: cursor id", errIdentity)
	}
	var results int
	for _, event := range events {
		if event["type"] == "result" {
			results++
			if event["subtype"] != "success" || event["is_error"] != false {
				return errors.New("cursor result failed")
			}
		}
	}
	if results != 1 {
		return errors.New("cursor result count")
	}
	if strings.Join(calls, ",") != "read_current,reply" {
		return errors.New("unexpected cursor tool sequence")
	}
	return nil
}

func truthy(v any) bool {
	yes, _ := v.(bool)
	return yes
}

func present(v any) bool {
	switch value := v.(type) {
	case nil:
		return false
	case string:
		return value != ""
	case []any:
		return len(value) > 0
	default:
		return true
	}
}
