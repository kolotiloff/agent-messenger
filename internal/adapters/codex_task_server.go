package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/kolotiloff/agent-messenger/internal/modelcatalog"
)

// CodexTaskServerArgs preserves the task-scoped MCP configuration while using
// the transport that exposes incremental public assistant messages.
func CodexTaskServerArgs(s TaskRunSpec) []string {
	return appendCodexTaskConfig([]string{s.CLI, "app-server", "--listen", "stdio://"}, s)
}

type codexTaskThread struct {
	Thread struct {
		ID     string
		Status struct{ Type string }
	}
	Cwd            string
	ApprovalPolicy string
	Sandbox        struct{ Type string }
}

type codexTaskTurn struct {
	ID, Status string
	Error      struct{ Message string }
}

// StreamCodexTask runs one turn inside the supervisor's existing worker
// process group. It binds the exact session before starting the turn, when
// task MCP calls first become possible. A terminal event is separate from
// observing the app-server's exit, which the supervisor still owns.
func StreamCodexTask(in io.Writer, out io.Reader, s TaskRunSpec, cwd, version string, bind func(string) error, emit func([]byte)) error {
	rpc := NewCodexRPC(in, out)
	// Keep draining shutdown output until the worker exits or the
	// supervisor closes its pipe. A full pipe could prevent normal exit.
	defer func() { go func() { _ = rpc.Drain(nil) }() }()
	ctx := context.Background()
	if _, err := rpc.Call(ctx, "initialize", CodexInitializeParams(version)); err != nil {
		return err
	}
	if err := rpc.Notify("initialized"); err != nil {
		return err
	}
	sandbox, policy := "workspace-write", "workspaceWrite"
	if s.FullAccess {
		sandbox, policy = "danger-full-access", "dangerFullAccess"
	}
	params := map[string]any{"cwd": cwd, "sandbox": sandbox, "approvalPolicy": "never"}
	method := "thread/start"
	if s.Resume {
		method = "thread/resume"
		params["threadId"] = s.Session
		params["excludeTurns"] = true
	}
	model, effort, err := modelcatalog.Resolve(s.Provider, s.Model)
	if err != nil {
		return err
	}
	if model != "" {
		params["model"] = model
	}
	raw, err := rpc.call(ctx, method, params, nil)
	if err != nil {
		return err
	}
	var thread codexTaskThread
	if json.Unmarshal(raw, &thread) != nil || !uuidPattern.MatchString(thread.Thread.ID) ||
		(s.Resume && thread.Thread.ID != s.Session) || thread.Thread.Status.Type != "idle" || !samePath(thread.Cwd, cwd) ||
		thread.ApprovalPolicy != "never" || thread.Sandbox.Type != policy {
		return fmt.Errorf("%w: unexpected worker thread or permissions", ErrCodexProtocol)
	}
	threadID := thread.Thread.ID
	if err := bind(threadID); err != nil {
		return err
	}
	stream := codexTaskStream{thread: threadID, write: emit}
	params = map[string]any{"threadId": threadID, "input": []map[string]any{
		{"type": "text", "text": taskPrompt(s.Profile), "text_elements": []any{}},
	}}
	if model != "" {
		params["model"] = model
	}
	if effort != "" {
		params["effort"] = effort
	}
	// A turn notification can precede the start response. Keep the relevant
	// frames until the response identifies the exact turn.
	var pending []codexFrame
	var pendingBytes int
	raw, err = rpc.call(ctx, "turn/start", params, func(frame codexFrame) error {
		if !codexTaskNotification(frame.Method) {
			return nil
		}
		pendingBytes += len(frame.Params)
		if pendingBytes > 16<<20 {
			return fmt.Errorf("%w: worker events exceed startup buffer", ErrCodexProtocol)
		}
		pending = append(pending, frame)
		return nil
	})
	if err != nil {
		return err
	}
	var started struct{ Turn codexTaskTurn }
	if json.Unmarshal(raw, &started) != nil || started.Turn.ID == "" {
		return fmt.Errorf("%w: worker turn has no identity", ErrCodexProtocol)
	}
	stream.turn = started.Turn.ID
	for _, frame := range pending {
		if stream.accept(frame) {
			return nil
		}
	}
	for frame := range rpc.frames {
		if frame.hasID() {
			return fmt.Errorf("%w: unexpected worker server request or response", ErrCodexProtocol)
		}
		if stream.accept(frame) {
			return nil
		}
	}
	return fmt.Errorf("app-server stopped before ending the worker turn: %w", rpc.readErr())
}

func codexTaskNotification(method string) bool {
	switch method {
	case "item/agentMessage/delta", "item/started", "item/completed", "turn/completed", "thread/tokenUsage/updated", "error":
		return true
	}
	return false
}

type codexTaskStream struct {
	thread, turn string
	write        func([]byte)
	usage        map[string]int64
}

func (s *codexTaskStream) emit(event map[string]any) {
	raw, _ := json.Marshal(event)
	s.write(raw)
}

func (s *codexTaskStream) accept(frame codexFrame) bool {
	if !codexTaskNotification(frame.Method) {
		return false
	}
	var event struct {
		ThreadID, TurnID, ItemID, Delta string
		Item                            json.RawMessage
		Turn                            codexTaskTurn
		Error                           struct{ Message string }
		WillRetry                       bool
		TokenUsage                      struct {
			Total struct {
				InputTokens, CachedInputTokens, OutputTokens int64
			}
		}
	}
	if json.Unmarshal(frame.Params, &event) != nil || event.ThreadID != s.thread {
		return false
	}
	turn := event.TurnID
	if frame.Method == "turn/completed" {
		turn = event.Turn.ID
	}
	if turn != s.turn {
		return false
	}
	switch frame.Method {
	case "item/agentMessage/delta":
		if event.ItemID != "" && event.Delta != "" {
			s.emit(map[string]any{"type": "item.text_delta", "delta": event.Delta})
		}
	case "item/started", "item/completed":
		s.item(frame.Method, event.Item)
	case "thread/tokenUsage/updated":
		total := event.TokenUsage.Total
		s.usage = map[string]int64{"input_tokens": total.InputTokens, "cached_input_tokens": total.CachedInputTokens, "output_tokens": total.OutputTokens}
	case "error":
		// A retry notification alone is no terminal failure. If the
		// turn does fail, the terminal event supplies the final error.
		if !event.WillRetry {
			s.emit(map[string]any{"type": "error", "message": event.Error.Message})
		}
	case "turn/completed":
		if event.Turn.Status == "completed" {
			s.emit(map[string]any{"type": "turn.completed", "usage": s.usage})
		} else {
			message := event.Turn.Error.Message
			if message == "" {
				message = event.Turn.Status
			}
			s.emit(map[string]any{"type": "turn.failed", "message": message})
		}
		return true
	}
	return false
}

func (s *codexTaskStream) item(method string, raw json.RawMessage) {
	var item struct {
		Type, ID, Text, Command, Status, Server, Tool, Query string
		Changes                                              []struct {
			Path string `json:"path"`
		}
	}
	if json.Unmarshal(raw, &item) != nil {
		return
	}
	event := map[string]any{"type": "item.completed"}
	if method == "item/started" {
		if item.Type != "commandExecution" {
			return
		}
		event["type"] = "item.started"
	}
	switch item.Type {
	case "agentMessage":
		event["item"] = map[string]any{"type": "agent_message", "text": item.Text}
	case "commandExecution":
		event["item"] = map[string]any{"type": "command_execution", "id": item.ID, "command": item.Command}
	case "fileChange":
		event["item"] = map[string]any{"type": "file_change", "status": item.Status, "changes": item.Changes}
	case "mcpToolCall":
		event["item"] = map[string]any{"type": "mcp_tool_call", "server": item.Server, "tool": item.Tool}
	case "webSearch":
		event["item"] = map[string]any{"type": "web_search", "query": item.Query}
	default:
		return
	}
	s.emit(event)
}
