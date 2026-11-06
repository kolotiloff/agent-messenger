package adapters

import "encoding/json"

const StreamPrompt = "Authorized Messenger message. Use ONLY messenger_bound MCP tools. First call read_current. If the message asks you to involve another agent, you may call send with one of the listed agent aliases and the receipt. Then always call reply with your response text and the receipt. Never reuse a receipt. Do not choose the reply recipient or message ID. Do not poll or use other tools."

func ClaudeStreamArgs(cli, conversationID, config string, newSession bool) ([]string, error) {
	if err := rejectLooseID(conversationID); err != nil {
		return nil, err
	}
	args := []string{cli, "-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--tools", "", "--strict-mcp-config", "--mcp-config", config, "--setting-sources", "", "--settings", `{"disableAllHooks":true}`, "--disable-slash-commands", "--no-chrome", "--permission-mode", "dontAsk", "--allowedTools", "mcp__messenger_bound__read_current,mcp__messenger_bound__send,mcp__messenger_bound__reply"}
	if newSession {
		args = append(args, "--session-id", conversationID)
	} else {
		args = append(args, "--resume", conversationID)
	}
	return args, ExplicitResume(args, conversationID)
}

func ClaudeStreamFrame(conversationID string) ([]byte, error) {
	b, err := json.Marshal(map[string]any{"type": "user", "session_id": conversationID, "parent_tool_use_id": nil, "message": map[string]any{"role": "user", "content": StreamPrompt}})
	return append(b, '\n'), err
}
