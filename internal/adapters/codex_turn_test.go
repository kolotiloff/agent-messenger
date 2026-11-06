package adapters

import (
	"encoding/json"
	"errors"
	"testing"
)

func codexTool(thread, turn, id, tool string, args any) json.RawMessage {
	data, _ := json.Marshal(map[string]any{"threadId": thread, "turnId": turn, "callId": id, "namespace": "messenger_bound", "tool": tool, "arguments": args})
	return data
}

func codexComplete(thread, turn, status string) json.RawMessage {
	data, _ := json.Marshal(map[string]any{"threadId": thread, "turn": map[string]any{"id": turn, "status": status, "error": nil}})
	return data
}

func TestCodexTurnRequiresPersistedReplyAndExactIdentity(t *testing.T) {
	var replies int
	guard, err := NewCodexTurnGuard("thread-a", "turn-1", CodexBoundTools{
		Read: func() (string, string, error) { return "payload", "receipt-1", nil },
		Reply: func(receipt, text string) error {
			if receipt != "receipt-1" || text != "ACK" {
				t.Fatal("incorrect binding")
			}
			replies++
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	read, err := guard.Tool(codexTool("thread-a", "turn-1", "call-1", "read_current", map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Success bool `json:"success"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"contentItems"`
	}
	if json.Unmarshal(read, &response) != nil || !response.Success || len(response.Content) != 1 || response.Content[0].Type != "inputText" {
		t.Fatal("wrong dynamic tool protocol")
	}
	if _, err = guard.Tool(codexTool("thread-a", "turn-1", "call-2", "reply", map[string]any{"text": "ACK", "receipt": "receipt-1"})); err != nil {
		t.Fatal(err)
	}
	if replies != 1 {
		t.Fatal("reply not committed exactly once")
	}
	if err = guard.Complete(codexComplete("thread-a", "turn-1", "completed")); err != nil {
		t.Fatal(err)
	}
	if _, err = guard.Tool(codexTool("thread-a", "turn-1", "call-3", "read_current", map[string]any{})); err == nil {
		t.Fatal("tools open after completion")
	}
	if guard.Complete(codexComplete("thread-a", "turn-1", "completed")) == nil {
		t.Fatal("duplicate completion accepted")
	}
}

func TestCodexTurnFailuresNeverInvokeReplyOrReopen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		frame json.RawMessage
	}{
		{"foreign thread", codexTool("other", "turn-1", "c2", "reply", map[string]any{"text": "ACK", "receipt": "r"})},
		{"foreign turn", codexTool("t", "other", "c2", "reply", map[string]any{"text": "ACK", "receipt": "r"})},
		{"old receipt", codexTool("t", "turn-1", "c2", "reply", map[string]any{"text": "ACK", "receipt": "old"})},
		{"chosen recipient", codexTool("t", "turn-1", "c2", "reply", map[string]any{"text": "ACK", "receipt": "r", "to": "other"})},
		{"reused call", codexTool("t", "turn-1", "c1", "reply", map[string]any{"text": "ACK", "receipt": "r"})},
		{"other tool", codexTool("t", "turn-1", "c2", "shell", map[string]any{})},
		{"null read args", codexTool("t", "turn-1", "c2", "read_current", nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			replies := 0
			g, _ := NewCodexTurnGuard("t", "turn-1", CodexBoundTools{Read: func() (string, string, error) { return "payload", "r", nil }, Reply: func(string, string) error { replies++; return nil }})
			if _, err := g.Tool(codexTool("t", "turn-1", "c1", "read_current", map[string]any{})); err != nil {
				t.Fatal(err)
			}
			if _, err := g.Tool(tc.frame); err == nil {
				t.Fatal("unsafe call accepted")
			}
			if _, err := g.Tool(codexTool("t", "turn-1", "c3", "reply", map[string]any{"text": "ACK", "receipt": "r"})); err == nil {
				t.Fatal("reopened rejected turn")
			}
			if replies != 0 {
				t.Fatal("rejected tool had side effects")
			}
		})
	}
}

func TestCodexTurnCompletionCannotReplaceReceiptOrCommit(t *testing.T) {
	for _, mode := range []string{"no-reply", "write-failed", "failed", "interrupted", "foreign-completion", "stopped"} {
		t.Run(mode, func(t *testing.T) {
			g, _ := NewCodexTurnGuard("t", "turn-1", CodexBoundTools{Read: func() (string, string, error) { return "payload", "r", nil }, Reply: func(string, string) error {
				if mode == "write-failed" {
					return errors.New("ambiguous disk error")
				}
				return nil
			}})
			_, err := g.Tool(codexTool("t", "turn-1", "c1", "read_current", map[string]any{}))
			if err != nil {
				t.Fatal(err)
			}
			if mode != "no-reply" {
				_, err = g.Tool(codexTool("t", "turn-1", "c2", "reply", map[string]any{"text": "ACK", "receipt": "r"}))
				if (err != nil) != (mode == "write-failed") {
					t.Fatal(err)
				}
			}
			status, turn := "completed", "turn-1"
			if mode == "failed" || mode == "interrupted" {
				status = mode
			}
			if mode == "foreign-completion" {
				turn = "turn-other"
			}
			if mode == "stopped" {
				g.Stop()
			}
			if g.Complete(codexComplete("t", turn, status)) == nil {
				t.Fatal("false success")
			}
		})
	}
}
