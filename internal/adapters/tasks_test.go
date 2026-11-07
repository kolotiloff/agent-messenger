package adapters

import (
	"strings"
	"testing"
)

const testUUID = "0b6f1d3e-5a1c-4c2e-9f00-1234567890ab"

func taskSpec(provider, profile, session string, resume bool) TaskRunSpec {
	return TaskRunSpec{Provider: provider, CLI: "/bin/agent", Messenger: "/bin/messenger", DB: "/data/queue.sqlite", RunID: "run-1", Receipt: "r1", Session: session, Resume: resume, Profile: profile}
}

func joined(args []string) string { return "\x00" + strings.Join(args, "\x00") + "\x00" }

func TestClaudeTaskArgs(t *testing.T) {
	args, err := TaskArgs(taskSpec("claude", ProfileWorker, testUUID, false))
	if err != nil {
		t.Fatal(err)
	}
	j := joined(args)
	for _, want := range []string{"\x00--session-id\x00" + testUUID + "\x00", "\x00-p\x00", "mcp__messenger_task__read_task,mcp__messenger_task__report,mcp__messenger_task__ask,mcp__messenger_task__ask_owner", "\"task-mcp\",\"--db\",\"/data/queue.sqlite\",\"--run\",\"run-1\",\"--receipt\",\"r1\"", "\x00--permission-mode\x00acceptEdits\x00"} {
		if !strings.Contains(j, want) {
			t.Errorf("missing %q in %v", want, args)
		}
	}
	// The worker profile keeps the project's tools and settings (D-023).
	for _, banned := range []string{"\x00--tools\x00", "--setting-sources", "--strict-mcp-config", "--resume", "--continue"} {
		if strings.Contains(j, banned) {
			t.Errorf("worker profile has %q", banned)
		}
	}
	if args[len(args)-1] != TaskWorkerPrompt {
		t.Fatal("worker prompt missing")
	}
	args, err = TaskArgs(taskSpec("claude", ProfileMessengerOnly, testUUID, true))
	if err != nil {
		t.Fatal(err)
	}
	j = joined(args)
	if !strings.Contains(j, "\x00--resume\x00"+testUUID+"\x00") || strings.Contains(j, "--session-id") || !strings.Contains(j, "\x00--tools\x00\x00") || args[len(args)-1] != TaskTestPrompt {
		t.Fatalf("messenger-only resume %v", args)
	}
	if _, err := TaskArgs(taskSpec("claude", ProfileWorker, "", false)); err == nil {
		t.Fatal("Claude task without a recorded id")
	}
}

func TestCodexTaskArgs(t *testing.T) {
	args, err := TaskArgs(taskSpec("codex", ProfileWorker, "", false))
	if err != nil {
		t.Fatal(err)
	}
	j := joined(args)
	if !strings.Contains(j, "\x00exec\x00--json\x00") || !strings.Contains(j, "\x00--sandbox\x00workspace-write\x00") || strings.Contains(j, "--ignore-user-config") || strings.Contains(j, "\x00resume\x00") {
		t.Fatalf("new worker run %v", args)
	}
	args, err = TaskArgs(taskSpec("codex", ProfileMessengerOnly, "019a0000-0000-7000-8000-000000000001", true))
	if err != nil {
		t.Fatal(err)
	}
	j = joined(args)
	if !strings.Contains(j, "\x00resume\x00019a0000-0000-7000-8000-000000000001\x00") || !strings.Contains(j, "--ignore-user-config") || strings.Contains(j, "--last") {
		t.Fatalf("resume %v", args)
	}
	if _, err := TaskArgs(taskSpec("codex", ProfileWorker, "", true)); err == nil {
		t.Fatal("resume without a thread id")
	}
	if _, err := TaskArgs(taskSpec("codex", ProfileWorker, "019a0000-0000-7000-8000-000000000001", false)); err == nil {
		t.Fatal("new run naming a thread")
	}
	if _, err := TaskArgs(taskSpec("cursor", ProfileWorker, "", false)); err == nil {
		t.Fatal("cursor accepted")
	}
	if _, err := TaskArgs(taskSpec("codex", "full", "", false)); err == nil {
		t.Fatal("unknown profile accepted")
	}
}

func TestParseTaskEvents(t *testing.T) {
	var r TaskEventResult
	for _, line := range []string{
		`{"type":"system","subtype":"init","session_id":"` + testUUID + `"}`,
		`{"type":"assistant","session_id":"` + testUUID + `","message":{"content":[{"type":"tool_use","name":"Bash"}]}}`,
		`{"type":"result","subtype":"success","is_error":false,"session_id":"` + testUUID + `"}`,
	} {
		ParseTaskEvent("claude", []byte(line), &r)
	}
	// Normal tool use is allowed; only identity and completion are checked.
	if r.Session != testUUID || !r.Completed || r.Conflicting || r.Failure != "" {
		t.Fatalf("claude %+v", r)
	}
	r = TaskEventResult{}
	ParseTaskEvent("claude", []byte(`{"type":"system","session_id":"a"}`), &r)
	ParseTaskEvent("claude", []byte(`{"type":"result","subtype":"error_max_turns","is_error":true,"session_id":"b"}`), &r)
	if !r.Conflicting || r.Completed || r.Failure == "" {
		t.Fatalf("claude failure %+v", r)
	}
	r = TaskEventResult{}
	for _, line := range []string{`{"type":"thread.started","thread_id":"t1"}`, `{"type":"item.completed","item":{"type":"command_execution"}}`, `{"type":"turn.completed"}`} {
		ParseTaskEvent("codex", []byte(line), &r)
	}
	if r.Session != "t1" || !r.Completed {
		t.Fatalf("codex %+v", r)
	}
	r = TaskEventResult{}
	ParseTaskEvent("codex", []byte(`{"type":"thread.started","thread_id":"t1"}`), &r)
	ParseTaskEvent("codex", []byte(`{"type":"turn.failed"}`), &r)
	ParseTaskEvent("codex", []byte(`{"type":"turn.completed"}`), &r)
	if r.Completed || r.Failure == "" {
		t.Fatalf("codex failure %+v", r)
	}
}

func TestTaskFeedEntries(t *testing.T) {
	got := TaskFeedEntries("claude", []byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Reading the code"},{"type":"tool_use","name":"Bash","input":{"command":"go test ./..."}},{"type":"tool_use","name":"mcp__messenger_task__report","input":{"text":"x"}},{"type":"tool_use","name":"ToolSearch","input":{"query":"messenger"}}]}}`))
	if len(got) != 2 || got[0].Kind != "text" || got[1].Title != "Bash" || got[1].Text != "go test ./..." {
		t.Fatalf("claude %+v", got)
	}
	got = TaskFeedEntries("codex", []byte(`{"type":"item.completed","item":{"type":"command_execution","command":"ls","exit_code":0}}`))
	if len(got) != 1 || got[0].Text != "ls" {
		t.Fatalf("codex %+v", got)
	}
	if got := TaskFeedEntries("codex", []byte(`{"type":"item.completed","item":{"type":"mcp_tool_call","server":"messenger_task","tool":"report"}}`)); len(got) != 0 {
		t.Fatalf("worker tool shown %+v", got)
	}
}

func TestTaskModelIsPassedToTheCLI(t *testing.T) {
	s := taskSpec("claude", ProfileWorker, testUUID, false)
	s.Model = "opus"
	args, err := TaskArgs(s)
	if err != nil || !strings.Contains(joined(args), "\x00--model\x00opus\x00") {
		t.Fatalf("claude model %v %v", args, err)
	}
	s = taskSpec("codex", ProfileWorker, "019a0000-0000-7000-8000-000000000001", true)
	s.Model = "gpt-5.1-codex"
	args, err = TaskArgs(s)
	j := joined(args)
	if err != nil || !strings.Contains(j, "\x00-c\x00model=\"gpt-5.1-codex\"\x00") || strings.Index(j, "model=") > strings.Index(j, "\x00resume\x00") {
		t.Fatalf("codex model %v %v", args, err)
	}
	// No model means the CLI's own default.
	args, _ = TaskArgs(taskSpec("claude", ProfileWorker, testUUID, false))
	if strings.Contains(joined(args), "--model") {
		t.Fatalf("default model named %v", args)
	}
	for _, bad := range []string{"-p", "--dangerously-skip-permissions", "a b", "x\"y", "a=b", strings.Repeat("a", 81)} {
		s := taskSpec("claude", ProfileWorker, testUUID, false)
		s.Model = bad
		if _, err := TaskArgs(s); err == nil {
			t.Errorf("model %q accepted", bad)
		}
	}
}
