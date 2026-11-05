package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/core"
)

func TestMCPExposesOnlyBoundReply(t *testing.T) {
	root := t.TempDir()
	env := filepath.Join(root, "cli")
	if err := os.MkdirAll(env, 0o700); err != nil {
		t.Fatal(err)
	}
	db := filepath.Join(root, "queue.sqlite")
	st, err := core.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.Register("synthetic", env, "conversation-1", "worker"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Enqueue("m1", "worker", "coordinator", "request m1"); err != nil {
		t.Fatal(err)
	}
	token, err := st.Claim("worker")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.BeginLaunch(token); err != nil {
		t.Fatal(err)
	}
	early := call(t, st, token, []string{"read_current", "reply"})
	if !early[0].isError || !early[1].isError {
		t.Fatal("mcp was open before identity confirmation")
	}
	if err := st.ConfirmIdentity(token, "synthetic", env, "conversation-1"); err != nil {
		t.Fatal(err)
	}
	results := call(t, st, token, []string{"tools/list", "read_current", "send_message", "reply-extra", "reply"})
	if strings.Contains(results[0].text, "send_message") {
		t.Fatal("send_message is listed")
	}
	if results[2].isError != true || results[3].isError != true || results[4].isError {
		t.Fatalf("calls = %+v", results)
	}
	if !strings.Contains(results[4].text, `"reply_to":"m1"`) || !strings.Contains(results[4].text, `"sender":"worker"`) {
		t.Fatalf("reply = %s", results[4].text)
	}
	again := call(t, st, token, []string{"reply-same", "reply-other"})
	if again[0].isError || again[1].isError != true {
		t.Fatalf("duplicate = %+v", again)
	}
	if err := st.MarkUncertain(token, "revoked"); err != nil {
		t.Fatal(err)
	}
	revoked := call(t, st, token, []string{"read_current", "reply"})
	if !revoked[0].isError || !revoked[1].isError {
		t.Fatal("revoked context can still use mcp")
	}
}

type toolResult struct {
	text    string
	isError bool
}

func call(t *testing.T, st *core.Store, token string, names []string) []toolResult {
	t.Helper()
	var lines []string
	lines = append(lines, rpc(1, "initialize", nil))
	for i, name := range names {
		switch name {
		case "tools/list":
			lines = append(lines, rpc(i+2, "tools/list", nil))
		case "read_current":
			lines = append(lines, rpc(i+2, "tools/call", map[string]any{"name": "read_current", "arguments": map[string]any{}}))
		case "send_message":
			lines = append(lines, rpc(i+2, "tools/call", map[string]any{"name": "send_message", "arguments": map[string]any{"to": "stranger", "text": "x"}}))
		case "reply-extra":
			lines = append(lines, rpc(i+2, "tools/call", map[string]any{"name": "reply", "arguments": map[string]any{"text": "correct", "message_id": "foreign"}}))
		case "reply", "reply-same":
			lines = append(lines, rpc(i+2, "tools/call", map[string]any{"name": "reply", "arguments": map[string]any{"text": "correct"}}))
		case "reply-other":
			lines = append(lines, rpc(i+2, "tools/call", map[string]any{"name": "reply", "arguments": map[string]any{"text": "different"}}))
		}
	}
	var output strings.Builder
	if err := serveStore(st, token, strings.NewReader(strings.Join(lines, "\n")+"\n"), &output); err != nil {
		t.Fatal(err)
	}
	var results []toolResult
	for _, line := range strings.Split(strings.TrimSpace(output.String()), "\n")[1:] {
		var response struct {
			Result struct {
				Tools   []map[string]any `json:"tools"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
				IsError bool `json:"isError"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &response); err != nil {
			t.Fatal(err)
		}
		text := ""
		if len(response.Result.Tools) > 0 {
			raw, _ := json.Marshal(response.Result.Tools)
			text = string(raw)
		}
		if len(response.Result.Content) > 0 {
			text = response.Result.Content[0].Text
		}
		results = append(results, toolResult{text: text, isError: response.Result.IsError})
	}
	return results
}

func rpc(id int, method string, params any) string {
	body := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		body["params"] = params
	}
	raw, _ := json.Marshal(body)
	return string(raw)
}
