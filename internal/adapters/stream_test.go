package adapters

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStreamArgsOnlyExactSessionAndRestrictedTools(t *testing.T) {
	for _, create := range []bool{false, true} {
		args, err := ClaudeStreamArgs("/cli", "exact-id", `{"mcpServers":{}}`, create)
		if err != nil {
			t.Fatal(err)
		}
		all := strings.Join(args, "\n")
		for _, required := range []string{"--input-format\nstream-json", "--output-format\nstream-json", "--strict-mcp-config", "--tools\n\n", "--setting-sources\n\n", "--disable-slash-commands"} {
			if !strings.Contains(all, required) {
				t.Fatal("missing isolation flag", required)
			}
		}
		mode := "--resume"
		if create {
			mode = "--session-id"
		}
		if !strings.Contains(all, mode+"\nexact-id") {
			t.Fatal("wrong session binding")
		}
	}
	if _, err := ClaudeStreamArgs("/cli", "latest", "{}", false); err == nil {
		t.Fatal("loose resume allowed")
	}
	raw, err := ClaudeStreamFrame("exact-id")
	if err != nil {
		t.Fatal(err)
	}
	var frame map[string]any
	if err := json.Unmarshal(raw, &frame); err != nil {
		t.Fatal(err)
	}
	if frame["session_id"] != "exact-id" || frame["type"] != "user" {
		t.Fatal("wrong frame")
	}
}
