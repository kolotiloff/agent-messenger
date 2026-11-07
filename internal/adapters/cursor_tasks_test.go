package adapters

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCursorTaskArgs(t *testing.T) {
	args, err := TaskArgs(taskSpec("cursor", ProfileWorker, "", false))
	if err != nil {
		t.Fatal(err)
	}
	j := joined(args)
	for _, want := range []string{"\x00-p\x00", "\x00--output-format\x00stream-json\x00", "\x00--trust\x00", "\x00--approve-mcps\x00", "\x00--force\x00"} {
		if !strings.Contains(j, want) {
			t.Errorf("missing %q in %v", want, args)
		}
	}
	if strings.Contains(j, "--resume") || strings.Contains(j, "r1") || args[len(args)-1] != TaskWorkerPrompt {
		t.Fatalf("new run %v", args)
	}
	args, err = TaskArgs(taskSpec("cursor", ProfileMessengerOnly, "c8f1a1b2-0000-4000-8000-000000000001", true))
	if err != nil {
		t.Fatal(err)
	}
	j = joined(args)
	if !strings.Contains(j, "\x00--resume=c8f1a1b2-0000-4000-8000-000000000001\x00") || strings.Contains(j, "--force") || !strings.Contains(j, "\x00--sandbox\x00enabled\x00") {
		t.Fatalf("resume %v", args)
	}
	for _, bad := range []TaskRunSpec{
		taskSpec("cursor", ProfileWorker, "", true),
		taskSpec("cursor", ProfileWorker, "c8f1", false),
		taskSpec("cursor", ProfileWorker, "latest", true),
		taskSpec("cursor", ProfileWorker, "-x", true),
	} {
		if _, err := TaskArgs(bad); err == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
	model := taskSpec("cursor", ProfileWorker, "", false)
	model.Model = "gpt-5"
	if args, err := TaskArgs(model); err != nil || !strings.Contains(joined(args), "\x00--model\x00gpt-5\x00") {
		t.Fatalf("model %v %v", args, err)
	}
}

func TestCursorTaskEnv(t *testing.T) {
	if TaskEnv(taskSpec("claude", ProfileWorker, testUUID, false)) != nil {
		t.Fatal("claude got an environment")
	}
	env := TaskEnv(taskSpec("cursor", ProfileWorker, "", false))
	if len(env) != 1 || !strings.HasPrefix(env[0], TaskEnvVariable+"=") {
		t.Fatalf("%v", env)
	}
	var v map[string]string
	if json.Unmarshal([]byte(strings.TrimPrefix(env[0], TaskEnvVariable+"=")), &v) != nil || v["db"] != "/data/queue.sqlite" || v["run"] != "run-1" || v["receipt"] != "r1" {
		t.Fatalf("%v", env)
	}
}

func TestEnsureCursorTaskServerKeepsTheOwnersEntries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".cursor", "mcp.json")
	messenger := filepath.Join(dir, "messenger")
	if err := EnsureCursorTaskServer(path, messenger); err != nil {
		t.Fatal(err)
	}
	read := func() map[string]any {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var v map[string]any
		if err := json.Unmarshal(raw, &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	entry := read()["mcpServers"].(map[string]any)[TaskServerName].(map[string]any)
	if entry["command"] != messenger || entry["env"].(map[string]any)[TaskEnvVariable] != "${env:"+TaskEnvVariable+"}" {
		t.Fatalf("%v", entry)
	}
	// The owner's own servers and settings stay as they are.
	own := `{"mcpServers":{"github":{"command":"gh-mcp","args":["--x"]},"messenger_task":{"command":"/old/messenger"}},"other":{"keep":true}}`
	if err := os.WriteFile(path, []byte(own), 0o644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(path, 0o644)
	if err := EnsureCursorTaskServer(path, messenger); err != nil {
		t.Fatal(err)
	}
	v := read()
	servers := v["mcpServers"].(map[string]any)
	if servers["github"].(map[string]any)["command"] != "gh-mcp" || v["other"].(map[string]any)["keep"] != true || servers[TaskServerName].(map[string]any)["command"] != messenger {
		t.Fatalf("%v", v)
	}
	if info, _ := os.Stat(path); runtime.GOOS != "windows" && info.Mode().Perm() != 0o644 {
		t.Fatalf("mode %v", info.Mode())
	}
	// An entry already in place is not rewritten.
	before, _ := os.Stat(path)
	if err := EnsureCursorTaskServer(path, messenger); err != nil {
		t.Fatal(err)
	}
	if after, _ := os.Stat(path); !after.ModTime().Equal(before.ModTime()) {
		t.Fatal("rewritten without a change")
	}
	// A file that is not valid JSON is never overwritten.
	for _, bad := range []string{`{"mcpServers": [1]}`, `{broken`} {
		_ = os.WriteFile(path, []byte(bad), 0o600)
		if err := EnsureCursorTaskServer(path, messenger); err == nil {
			t.Fatalf("%s overwritten", bad)
		}
		if raw, _ := os.ReadFile(path); string(raw) != bad {
			t.Fatal("changed")
		}
	}
}

func TestCursorConfigFiles(t *testing.T) {
	home := filepath.Join(string(filepath.Separator)+"home", "a")
	if got := CursorConfigFiles(home, ""); len(got) != 1 || got[0] != filepath.Join(home, ".cursor", "mcp.json") {
		t.Fatalf("%v", got)
	}
	if got := CursorConfigFiles(home, filepath.Join(home, ".cursor")); len(got) != 1 {
		t.Fatalf("%v", got)
	}
	if got := CursorConfigFiles(home, filepath.Join(home, "work-cursor")); len(got) != 2 || got[1] != filepath.Join(home, "work-cursor", "mcp.json") {
		t.Fatalf("%v", got)
	}
}

// Event shapes as Cursor CLI wrote them in the Mac research runs (D-049).
func TestParseCursorTaskEvents(t *testing.T) {
	var r TaskEventResult
	for _, line := range []string{
		`{"type":"system","subtype":"init","session_id":"chat-1","model":"x"}`,
		`{"type":"tool_call","subtype":"started","call_id":"1","session_id":"chat-1","tool_call":{"mcpToolCall":{"args":{"providerIdentifier":"messenger_task","toolName":"read_task"}}}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"done","session_id":"chat-1"}`,
	} {
		ParseTaskEvent("cursor", []byte(line), &r)
	}
	if r.Session != "chat-1" || r.Conflicting || !r.Completed || r.Failure != "" {
		t.Fatalf("%+v", r)
	}
	r = TaskEventResult{}
	ParseTaskEvent("cursor", []byte(`{"type":"system","subtype":"init","session_id":"chat-1"}`), &r)
	ParseTaskEvent("cursor", []byte(`{"type":"assistant","session_id":"chat-2","message":{"content":[]}}`), &r)
	if !r.Conflicting {
		t.Fatal("a second chat id was not noticed")
	}
	r = TaskEventResult{}
	ParseTaskEvent("cursor", []byte(`{"type":"system","subtype":"init","session_id":"chat-1"}`), &r)
	ParseTaskEvent("cursor", []byte(`{"type":"result","subtype":"error","is_error":true,"result":"You've hit your usage limit","session_id":"chat-1"}`), &r)
	if r.Completed || r.Failure == "" || r.Reason != ReasonUsageLimit {
		t.Fatalf("%+v", r)
	}
	r = TaskEventResult{}
	ParseTaskEvent("cursor", []byte(`{"type":"result","subtype":"success","is_error":true,"session_id":"chat-1"}`), &r)
	if r.Completed {
		t.Fatal("an error result counted as done")
	}
}

func TestCursorFeedEntries(t *testing.T) {
	lines := []string{
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"Looking at the tests"}]}}`,
		`{"type":"tool_call","subtype":"started","call_id":"1","tool_call":{"shellToolCall":{"args":{"command":"go test ./..."}}}}`,
		`{"type":"tool_call","subtype":"completed","call_id":"1","tool_call":{"shellToolCall":{"args":{"command":"go test ./..."},"result":{"success":{}}}}}`,
		`{"type":"tool_call","subtype":"started","call_id":"2","tool_call":{"mcpToolCall":{"args":{"providerIdentifier":"messenger_task","toolName":"report"}}}}`,
		`{"type":"tool_call","subtype":"started","call_id":"3","tool_call":{"readToolCall":{"args":{"path":"main.go"}}}}`,
		`{"type":"tool_call","subtype":"started","call_id":"4","tool_call":{"getMcpToolsToolCall":{"args":{}}}}`,
	}
	var got []FeedEntry
	for _, line := range lines {
		got = append(got, TaskFeedEntries("cursor", []byte(line))...)
	}
	want := []FeedEntry{{"text", "", "Looking at the tests"}, {"tool", "Команда", "go test ./..."}, {"tool", "read", "main.go"}}
	if len(got) != len(want) {
		t.Fatalf("%+v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%d: %+v", i, got[i])
		}
	}
}
