package adapters

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandsUseExactIDOnly(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "cursor"} {
		var args []string
		var err error
		switch provider {
		case "claude":
			args, err = ClaudeArgs("claude", "messenger", "db", "token", "conversation-1")
		case "codex":
			args, err = CodexArgs("codex", "messenger", "db", "token", "conversation-1")
		case "cursor":
			args, err = CursorArgs("agent", "conversation-1", "/tmp/workspace")
		}
		if err != nil {
			t.Fatal(provider, err)
		}
		joined := strings.Join(args, "\n")
		if strings.Contains(joined, "--last") || strings.Contains(joined, "--continue") {
			t.Fatalf("%s uses an implicit session: %s", provider, joined)
		}
		if !strings.Contains(joined, "conversation-1") {
			t.Fatalf("%s dropped the exact id", provider)
		}
	}
	if _, err := ClaudeArgs("claude", "messenger", "db", "token", "last"); err == nil {
		t.Fatal("last was accepted as an id")
	}
}

func TestPrepareCursorWorkspaceStaysInsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	if err := PrepareCursorWorkspace(workspace, "/tmp/messenger", "/tmp/queue.sqlite", "token"); err != nil {
		t.Fatal(err)
	}
	mcp, err := os.ReadFile(filepath.Join(workspace, ".cursor", "mcp.json"))
	if err != nil || !strings.Contains(string(mcp), "messenger_bound") {
		t.Fatalf("mcp.json = %s %v", mcp, err)
	}
	cli, err := os.ReadFile(filepath.Join(workspace, ".cursor", "cli.json"))
	if err != nil || !strings.Contains(string(cli), "Shell(*)") {
		t.Fatalf("cli.json = %s %v", cli, err)
	}
	if _, err := os.Stat(filepath.Join(t.TempDir(), ".cursor")); err == nil {
		t.Fatal("cursor files escaped the workspace")
	}
}

func TestCursorIsolationChecksTheGivenHome(t *testing.T) {
	home := t.TempDir()
	if err := CheckCursorIsolation(home, ""); err != nil {
		t.Fatal(err)
	}
	if err := CheckCursorIsolation(home, "/custom"); err == nil {
		t.Fatal("config dir was ignored")
	}
	if err := os.MkdirAll(filepath.Join(home, ".cursor"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".cursor", "mcp.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := CheckCursorIsolation(home, ""); err == nil {
		t.Fatal("global mcp config was ignored")
	}
}

func TestStartupAndCompletionValidation(t *testing.T) {
	if _, err := StartupID("codex", map[string]any{"type": "thread.started"}); err == nil {
		t.Fatal("missing codex id")
	}
	if _, err := StartupID("cursor", map[string]any{"type": "system", "subtype": "init"}); err == nil {
		t.Fatal("missing cursor id")
	}
	id, err := StartupID("codex", map[string]any{"type": "thread.started", "thread_id": "test-id"})
	if err != nil || id != "test-id" {
		t.Fatal(id, err)
	}

	claude := []map[string]any{
		{"type": "assistant", "message": map[string]any{"content": []any{
			map[string]any{"type": "tool_use", "name": "mcp__messenger_bound__read_current"},
			map[string]any{"type": "tool_use", "name": "mcp__messenger_bound__reply"},
		}}},
		{"type": "result", "subtype": "success", "is_error": false, "session_id": "test-id"},
	}
	if err := ValidateClaude(claude, "test-id"); err != nil {
		t.Fatal(err)
	}
	bad := append([]map[string]any{}, claude...)
	bad[1] = map[string]any{"type": "result", "subtype": "success", "is_error": false, "session_id": "test-id", "permission_denials": []any{"denied"}}
	if err := ValidateClaude(bad, "test-id"); err == nil {
		t.Fatal("claude denial accepted")
	}
	if err := ValidateClaude(claude, "wrong-id"); err == nil {
		t.Fatal("claude wrong id accepted")
	}

	codex := []map[string]any{{"type": "thread.started", "thread_id": "test-id"}}
	for _, name := range []string{"read_current", "reply"} {
		codex = append(codex, map[string]any{"type": "item.completed", "item": map[string]any{
			"type": "mcp_tool_call", "server": "messenger_bound", "tool": name, "status": "completed",
		}})
	}
	codex = append(codex, map[string]any{"type": "turn.completed"})
	if err := ValidateCodex(codex, "test-id"); err != nil {
		t.Fatal(err)
	}
	foreign := append([]map[string]any{}, codex...)
	foreign[1] = map[string]any{"type": "item.completed", "item": map[string]any{
		"type": "mcp_tool_call", "server": "foreign", "tool": "read_current", "status": "completed",
	}}
	if err := ValidateCodex(foreign, "test-id"); err == nil {
		t.Fatal("foreign codex server accepted")
	}
	if err := ValidateCodex(append(codex, map[string]any{"type": "turn.completed"}), "test-id"); err == nil {
		t.Fatal("duplicate completion accepted")
	}

	cursor := []map[string]any{{"type": "system", "subtype": "init", "session_id": "test-id"}}
	cursor = append(cursor, map[string]any{"type": "tool_call", "subtype": "completed", "session_id": "test-id",
		"tool_call": map[string]any{"getMcpToolsToolCall": map[string]any{}}})
	for _, name := range []string{"read_current", "reply"} {
		cursor = append(cursor, map[string]any{"type": "tool_call", "subtype": "completed", "session_id": "test-id",
			"tool_call": map[string]any{"mcpToolCall": map[string]any{
				"args":   map[string]any{"providerIdentifier": "messenger_bound", "toolName": name},
				"result": map[string]any{"success": map[string]any{"isError": false}},
			}}})
	}
	cursor = append(cursor, map[string]any{"type": "result", "subtype": "success", "is_error": false, "session_id": "test-id"})
	if err := ValidateCursor(cursor, "test-id"); err != nil {
		t.Fatal(err)
	}
	shell := append([]map[string]any{}, cursor...)
	shell[2] = map[string]any{"type": "tool_call", "subtype": "completed", "session_id": "test-id",
		"tool_call": map[string]any{"shellToolCall": map[string]any{}}}
	if err := ValidateCursor(shell, "test-id"); err == nil {
		t.Fatal("cursor shell accepted")
	}
	if err := ValidateCursor(cursor, "wrong-id"); err == nil {
		t.Fatal("cursor wrong id accepted")
	}
	if !errors.Is(errIdentity, errIdentity) {
		t.Fatal("identity error missing")
	}
}
