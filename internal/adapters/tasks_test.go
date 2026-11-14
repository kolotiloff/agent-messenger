package adapters

import (
	"encoding/json"
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
	if _, err := TaskArgs(taskSpec("gemini", ProfileWorker, "", false)); err == nil {
		t.Fatal("unknown agent accepted")
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

// T-075: a worker commits its change, and when it cannot it asks instead of
// finishing, so its conversation stays open.
func TestWorkerPromptKeepsAFailedCommitOpen(t *testing.T) {
	if strings.Contains(TaskWorkerPrompt, "unless") {
		t.Fatal("the commit rule has no exceptions")
	}
	for _, want := range []string{"commit", "cannot commit", "ask", "do not report"} {
		if !strings.Contains(TaskWorkerPrompt, want) {
			t.Fatalf("worker prompt lacks %q: %s", want, TaskWorkerPrompt)
		}
	}
}

// T-087: the last message is kept in full, line breaks included.
func TestFinalTextIsKeptWhole(t *testing.T) {
	long := strings.Repeat("line of the audit\n", 200)
	line, _ := json.Marshal(map[string]any{"type": "result", "session_id": "s", "result": long})
	var r TaskEventResult
	ParseTaskEvent("claude", line, &r)
	if r.FinalText != strings.TrimSpace(long) || len([]rune(r.LastText)) > lastTextLimit+1 {
		t.Fatalf("final %d last %d", len(r.FinalText), len(r.LastText))
	}
	codex, _ := json.Marshal(map[string]any{"type": "item.completed", "item": map[string]any{"type": "agent_message", "text": long}})
	var c TaskEventResult
	ParseTaskEvent("codex", codex, &c)
	if c.FinalText != strings.TrimSpace(long) {
		t.Fatal("codex final text")
	}
}

// T-088: Claude's cache writes are kept apart from its reads.
func TestClaudeCacheWriteIsKept(t *testing.T) {
	line, _ := json.Marshal(map[string]any{"type": "result", "session_id": "s", "usage": map[string]any{"input_tokens": 18, "cache_creation_input_tokens": 55000, "cache_read_input_tokens": 375000, "output_tokens": 3200}})
	var r TaskEventResult
	ParseTaskEvent("claude", line, &r)
	if r.Usage == nil || r.Usage.Input != 430018 || r.Usage.CacheWrite != 55000 || r.Usage.Cached != 375000 || r.Usage.Output != 3200 {
		t.Fatalf("usage %+v", r.Usage)
	}
}

// T-108: the model a Claude worker reports at its start is kept.
func TestClaudeReportsItsModel(t *testing.T) {
	var r TaskEventResult
	ParseTaskEvent("claude", []byte(`{"type":"system","subtype":"init","session_id":"s-1","model":"claude-sonnet-test"}`), &r)
	ParseTaskEvent("claude", []byte(`{"type":"assistant","session_id":"s-1","message":{"content":[]}}`), &r)
	if r.Model != "claude-sonnet-test" {
		t.Fatalf("model %q", r.Model)
	}
}

// T-171: the stored selection must launch the intended model AND effort,
// both for a new task and for its exact-session continuation.
func TestExplicitModelEffortArgs(t *testing.T) {
	for _, provider := range []string{"claude", "codex", "cursor"} {
		for _, resume := range []bool{false, true} {
			session := ""
			if provider == "claude" || resume {
				session = testUUID
			}
			spec := taskSpec(provider, ProfileWorker, session, resume)
			switch provider {
			case "claude":
				spec.Model = "claude-fable-5-2[effort=high]"
			case "codex":
				spec.Model = "gpt-6-astra[effort=medium]"
			case "cursor":
				spec.Model = "grok-4.7-high"
			}
			args, err := TaskArgs(spec)
			if err != nil {
				t.Fatal(err)
			}
			got := joined(args)
			if strings.Contains(got, "[effort=") {
				t.Fatalf("selection leaked as model ID: %v", args)
			}
			want := []string{"--model\x00grok-4.7-high"}
			if provider == "claude" {
				if env := TaskEnv(spec); len(env) != 1 || env[0] != "CLAUDE_CODE_EFFORT_LEVEL=high" {
					t.Fatalf("effort environment: %v", env)
				}
				want = []string{"--model\x00claude-fable-5-2", "--effort\x00high"}
			}
			if provider == "codex" {
				want = []string{`model="gpt-6-astra"`, `model_reasoning_effort="medium"`}
			}
			for _, part := range want {
				if !strings.Contains(got, part) {
					t.Fatalf("missing %q: %v", part, args)
				}
			}
			if resume && !strings.Contains(got, session) {
				t.Fatal("lost exact session")
			}
		}
	}
	for _, bad := range []string{"gpt-6-astra[effort=H]", "x[effort=high,fast=true]", "x[effort=high]--yolo", "x[effort=high]\n", "x[effort=high][effort=low]"} {
		s := taskSpec("codex", ProfileWorker, "", false)
		s.Model = bad
		if _, err := TaskArgs(s); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	s := taskSpec("cursor", ProfileWorker, "", false)
	s.Model = "grok-4.7[effort=high]"
	if _, err := TaskArgs(s); err == nil {
		t.Fatal("Cursor accepted a Messenger effort suffix")
	}
}
