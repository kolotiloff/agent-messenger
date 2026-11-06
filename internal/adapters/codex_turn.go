package adapters

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

// CodexBoundTools must be bound by the controller to one durable attempt. The
// callbacks must recheck database authorization and commit before returning.
// Neither recipient nor message ID is accepted from the model.
type CodexBoundTools struct {
	Read  func() (text, receipt string, err error)
	Reply func(receipt, text string) error
}

// CodexTurnGuard is owned by one serial controller. Construct it only after a
// correlated turn/start RPC response and persisted thread/turn identity. It is
// not a launcher and never authorizes a session or releases its process lock.
type CodexTurnGuard struct {
	thread, turn string
	tools        CodexBoundTools
	receipt      string
	readCall     string
	replied      bool
	terminal     bool
}

func NewCodexTurnGuard(thread, turn string, tools CodexBoundTools) (*CodexTurnGuard, error) {
	if thread == "" || turn == "" || tools.Read == nil || tools.Reply == nil {
		return nil, errors.New("Codex turn requires exact identity and bound tools")
	}
	return &CodexTurnGuard{thread: thread, turn: turn, tools: tools}, nil
}

func (g *CodexTurnGuard) reject() error {
	g.terminal = true
	return errors.New("Codex turn rejected; preserve ownership and do not retry")
}

// Stop revokes further tool calls after EOF, timeout, cancellation or a controller
// failure. The controller separately persists an uncertain attempt and keeps
// the process reservation; this in-memory guard cannot establish termination.
func (g *CodexTurnGuard) Stop() { g.terminal = true }

// Tool handles item/tool/call.params, not its RPC envelope. The controller must
// also correlate and deduplicate server RPC request IDs before calling this.
func (g *CodexTurnGuard) Tool(raw json.RawMessage) (json.RawMessage, error) {
	if g.terminal {
		return nil, g.reject()
	}
	var call struct {
		ThreadID  string          `json:"threadId"`
		TurnID    string          `json:"turnId"`
		CallID    string          `json:"callId"`
		Namespace string          `json:"namespace"`
		Tool      string          `json:"tool"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if strictCodexJSON(raw, &call) != nil || call.ThreadID != g.thread || call.TurnID != g.turn || call.CallID == "" || call.Namespace != "messenger_bound" {
		return nil, g.reject()
	}
	var output any
	switch call.Tool {
	case "read_current":
		var args map[string]json.RawMessage
		if g.readCall != "" || json.Unmarshal(call.Arguments, &args) != nil || args == nil || len(args) != 0 {
			return nil, g.reject()
		}
		text, receipt, err := g.tools.Read()
		if err != nil || receipt == "" {
			return nil, g.reject()
		}
		g.readCall, g.receipt = call.CallID, receipt
		output = map[string]string{"text": text, "receipt": receipt}
	case "reply":
		var args struct {
			Text    string `json:"text"`
			Receipt string `json:"receipt"`
		}
		if g.readCall == "" || g.replied || call.CallID == g.readCall || strictCodexJSON(call.Arguments, &args) != nil || args.Receipt != g.receipt || len(args.Text) < 1 || len(args.Text) > 2000 {
			return nil, g.reject()
		}
		if err := g.tools.Reply(args.Receipt, args.Text); err != nil {
			return nil, g.reject()
		}
		g.replied = true
		output = map[string]bool{"stored": true}
	default:
		return nil, g.reject()
	}
	text, err := json.Marshal(output)
	if err != nil {
		return nil, g.reject()
	}
	return json.Marshal(map[string]any{"success": true, "contentItems": []any{map[string]string{"type": "inputText", "text": string(text)}}})
}

// Complete handles turn/completed.params. A completed status without a durable
// bound reply is not success. The controller must still commit its verification.
func (g *CodexTurnGuard) Complete(raw json.RawMessage) error {
	if g.terminal {
		return g.reject()
	}
	var params struct {
		ThreadID string `json:"threadId"`
		Turn     struct {
			ID     string          `json:"id"`
			Status string          `json:"status"`
			Error  json.RawMessage `json:"error"`
		} `json:"turn"`
	}
	if json.Unmarshal(raw, &params) != nil || params.ThreadID != g.thread || params.Turn.ID != g.turn || params.Turn.Status != "completed" || (len(params.Turn.Error) != 0 && string(params.Turn.Error) != "null") || !g.replied {
		return g.reject()
	}
	g.terminal = true
	return nil
}

func strictCodexJSON(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
