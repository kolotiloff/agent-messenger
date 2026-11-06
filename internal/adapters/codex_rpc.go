package adapters

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
)

// Setup-phase methods. None of them starts a model turn. turn/start is not
// reachable through this client.
var codexSetupMethods = map[string]bool{"initialize": true, "config/read": true, "thread/start": true}

// Notifications the client may send during setup.
var codexSetupNotifications = map[string]bool{"initialized": true}

// A bound on notifications skipped while waiting for one response, so a noisy
// or broken server cannot keep a setup call waiting forever.
const codexMaxSkippedNotifications = 1024

var ErrCodexProtocol = errors.New("Codex app-server protocol violation; no retry")

type codexFrame struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
	Result json.RawMessage `json:"result"`
	Error  json.RawMessage `json:"error"`
}

func (f codexFrame) hasID() bool { return len(f.ID) != 0 && string(f.ID) != "null" }

// CodexRPC is a JSON-lines client for one stdio app-server process. One serial
// controller owns it. Requests are written once: a write error, a timeout, EOF
// or an unexpected frame is reported as an error and is never retried, because
// the server may already have acted on the request.
type CodexRPC struct {
	w      io.Writer
	frames chan codexFrame
	done   chan struct{}
	errMu  sync.Mutex
	err    error
	next   int
	closed bool
}

// NewCodexRPC starts a reader for r. The reader stops at EOF or on a malformed
// line; after that every call fails.
func NewCodexRPC(w io.Writer, r io.Reader) *CodexRPC {
	c := &CodexRPC{w: w, frames: make(chan codexFrame, 64), done: make(chan struct{})}
	go c.read(r)
	return c
}

func (c *CodexRPC) read(r io.Reader) {
	defer close(c.done)
	defer close(c.frames)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64<<10), 16<<20)
	for scanner.Scan() {
		var frame codexFrame
		if json.Unmarshal(scanner.Bytes(), &frame) != nil {
			c.fail(fmt.Errorf("%w: malformed frame", ErrCodexProtocol))
			return
		}
		c.frames <- frame
	}
	if err := scanner.Err(); err != nil {
		c.fail(fmt.Errorf("app-server output failed: %w", err))
		return
	}
	c.fail(io.EOF)
}

func (c *CodexRPC) fail(err error) {
	c.errMu.Lock()
	if c.err == nil {
		c.err = err
	}
	c.errMu.Unlock()
}

func (c *CodexRPC) readErr() error {
	c.errMu.Lock()
	defer c.errMu.Unlock()
	if c.err == nil {
		return io.EOF
	}
	return c.err
}

func (c *CodexRPC) write(payload any) error {
	if c.closed {
		return fmt.Errorf("%w: client closed after an earlier failure", ErrCodexProtocol)
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	n, err := c.w.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		c.closed = true
		return fmt.Errorf("write to app-server failed; outcome unknown: %w", err)
	}
	return nil
}

// Notify sends a client notification from the setup allowlist.
func (c *CodexRPC) Notify(method string) error {
	if !codexSetupNotifications[method] {
		return fmt.Errorf("%w: notification %q not allowed", ErrCodexProtocol, method)
	}
	return c.write(map[string]any{"method": method})
}

// Call sends one setup request and waits for its response. Notifications that
// arrive first are skipped. A server request, a response with another ID, a
// frame with both result and error, or an error response fails the call; the
// server's error text is not copied into the returned error.
func (c *CodexRPC) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	if !codexSetupMethods[method] {
		return nil, fmt.Errorf("%w: method %q not allowed during setup", ErrCodexProtocol, method)
	}
	c.next++
	id := c.next
	if err := c.write(map[string]any{"id": id, "method": method, "params": params}); err != nil {
		return nil, err
	}
	skipped := 0
	for {
		select {
		case <-ctx.Done():
			c.closed = true
			return nil, fmt.Errorf("no response to %s; outcome unknown: %w", method, ctx.Err())
		case frame, ok := <-c.frames:
			if !ok {
				c.closed = true
				return nil, fmt.Errorf("app-server stopped before answering %s; outcome unknown: %w", method, c.readErr())
			}
			if !frame.hasID() {
				if frame.Method == "" || len(frame.Result) != 0 || len(frame.Error) != 0 {
					c.closed = true
					return nil, fmt.Errorf("%w: response without id", ErrCodexProtocol)
				}
				skipped++
				if skipped > codexMaxSkippedNotifications {
					c.closed = true
					return nil, fmt.Errorf("%w: too many notifications before %s response", ErrCodexProtocol, method)
				}
				continue
			}
			var got int
			if frame.Method != "" || json.Unmarshal(frame.ID, &got) != nil || got != id {
				c.closed = true
				return nil, fmt.Errorf("%w: unexpected server request or response id", ErrCodexProtocol)
			}
			if len(frame.Error) != 0 {
				c.closed = true
				return nil, fmt.Errorf("app-server refused %s", method)
			}
			if len(frame.Result) == 0 || string(frame.Result) == "null" {
				c.closed = true
				return nil, fmt.Errorf("%w: empty result", ErrCodexProtocol)
			}
			return frame.Result, nil
		}
	}
}

// Drain keeps reading after setup so the server never blocks on a full pipe.
// Notifications are dropped. onRequest is called for every server request;
// none is expected while no turn is running. Drain returns when output ends.
func (c *CodexRPC) Drain(onRequest func()) error {
	for frame := range c.frames {
		if frame.hasID() && frame.Method != "" && onRequest != nil {
			onRequest()
		}
	}
	return c.readErr()
}

// Done is closed when the server's output has ended.
func (c *CodexRPC) Done() <-chan struct{} { return c.done }

// CodexInitializeParams identifies Messenger and opts into the experimental
// API, which dynamic tools need.
func CodexInitializeParams(version string) map[string]any {
	return map[string]any{
		"clientInfo":   map[string]any{"name": "agent_messenger", "version": version},
		"capabilities": map[string]any{"experimentalApi": true},
	}
}

// CodexThreadStartParams creates a persistent (non-ephemeral) thread in the
// managed workspace. Dynamic tools must be declared here: a thread cannot gain
// them later. Their names match CodexTurnGuard.
func CodexThreadStartParams(workspace string) map[string]any {
	return map[string]any{
		"cwd":            workspace,
		"sandbox":        "read-only",
		"approvalPolicy": "never",
		"ephemeral":      false,
		"dynamicTools": []any{map[string]any{
			"type":        "namespace",
			"name":        "messenger_bound",
			"description": "Agent Messenger tools bound to the current message.",
			"tools": []any{
				map[string]any{
					"type":        "function",
					"name":        "read_current",
					"description": "Read the current message and its one-time receipt.",
					"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false},
				},
				map[string]any{
					"type":        "function",
					"name":        "reply",
					"description": "Reply to the current message. The recipient is set by Messenger.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"receipt": map[string]any{"type": "string"},
							"text":    map[string]any{"type": "string", "minLength": 1, "maxLength": 2000},
						},
						"required":             []string{"receipt", "text"},
						"additionalProperties": false,
					},
				},
			},
		}},
	}
}

// Codex-generated thread IDs are UUIDv7; accept any lowercase canonical UUID.
var codexThreadIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ParseCodexThreadStart checks a thread/start result and returns the issued
// thread ID. It requires a persistent, idle, empty thread in the requested
// workspace with a read-only sandbox and no approvals.
func ParseCodexThreadStart(raw json.RawMessage, workspace string) (string, error) {
	var result struct {
		ApprovalPolicy json.RawMessage `json:"approvalPolicy"`
		Sandbox        struct {
			Type string `json:"type"`
		} `json:"sandbox"`
		Thread struct {
			ID        string `json:"id"`
			Cwd       string `json:"cwd"`
			Ephemeral *bool  `json:"ephemeral"`
			Status    struct {
				Type string `json:"type"`
			} `json:"status"`
			Turns []json.RawMessage `json:"turns"`
		} `json:"thread"`
	}
	if json.Unmarshal(raw, &result) != nil {
		return "", fmt.Errorf("%w: invalid thread/start result", ErrCodexProtocol)
	}
	t := result.Thread
	if !codexThreadIDPattern.MatchString(t.ID) {
		return "", fmt.Errorf("%w: thread id missing or malformed", ErrCodexProtocol)
	}
	if t.Ephemeral == nil || *t.Ephemeral {
		return "", fmt.Errorf("%w: thread is not persistent", ErrCodexProtocol)
	}
	if t.Status.Type != "idle" || len(t.Turns) != 0 {
		return "", fmt.Errorf("%w: new thread is not idle and empty", ErrCodexProtocol)
	}
	if !samePath(t.Cwd, workspace) {
		return "", fmt.Errorf("%w: thread workspace differs", ErrCodexProtocol)
	}
	var policy string
	if json.Unmarshal(result.ApprovalPolicy, &policy) != nil || policy != "never" {
		return "", fmt.Errorf("%w: approval policy is not never", ErrCodexProtocol)
	}
	if result.Sandbox.Type != "readOnly" {
		return "", fmt.Errorf("%w: sandbox is not read-only", ErrCodexProtocol)
	}
	return t.ID, nil
}

func samePath(a, b string) bool {
	if a == "" || b == "" || !filepath.IsAbs(a) || !filepath.IsAbs(b) {
		return false
	}
	if filepath.Clean(a) == filepath.Clean(b) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return false
	}
	if ra == rb {
		return true
	}
	ia, errA := os.Stat(ra)
	ib, errB := os.Stat(rb)
	return errA == nil && errB == nil && os.SameFile(ia, ib)
}

// CodexTurnPrompt is the only model input Messenger sends. The message itself
// is read through the bound tool, so the reply is tied to its receipt.
const CodexTurnPrompt = "Authorized Messenger message. Use ONLY the messenger_bound tools. First call read_current, then call reply with your response text and the receipt returned for this message. Never reuse a receipt. Do not choose a recipient or message ID. Do not poll or use other tools."

// Bound on frames kept while turn/start has not been answered yet.
const codexMaxEarlyFrames = 256

var ErrCodexIdleRequest = errors.New("Codex app-server sent a request while idle")

// WaitIdle consumes output while no turn runs. Notifications are dropped; a
// server request is an error. Every poll interval it calls ready, and returns
// nil as soon as ready reports true. It also returns on ctx or EOF.
func (c *CodexRPC) WaitIdle(ctx context.Context, poll time.Duration, ready func() bool) error {
	ticker := time.NewTicker(poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case frame, ok := <-c.frames:
			if !ok {
				c.closed = true
				return fmt.Errorf("app-server stopped while idle: %w", c.readErr())
			}
			if frame.hasID() {
				c.closed = true
				return ErrCodexIdleRequest
			}
		case <-ticker.C:
			if ready() {
				return nil
			}
		}
	}
}

func (c *CodexRPC) respond(id json.RawMessage, result json.RawMessage, failure string) error {
	if failure != "" {
		return c.write(map[string]any{"id": id, "error": map[string]any{"code": -32601, "message": failure}})
	}
	return c.write(map[string]any{"id": id, "result": result})
}

// CodexTurnHooks connect a turn to durable state. Prepare commits the send
// intent and runs BEFORE turn/start is written. Bind stores the turn ID from
// the turn/start response before any tool call of that turn is handled.
type CodexTurnHooks struct {
	Prepare func() error
	Bind    func(turnID string) error
	Tools   CodexBoundTools
}

// RunTurn sends exactly one turn/start for the thread and serves that turn
// until turn/completed. It returns nil only when CodexTurnGuard accepted the
// completion, which requires a stored reply. Every other outcome is an error
// and the client is closed: the caller must treat the turn as uncertain and
// must not retry it.
//
// Frames that arrive before the turn/start response are kept (bounded) and
// handled after the turn ID is bound. Server requests other than item/tool/call
// are refused with an error response; the turn can still complete. A repeated
// server request ID ends the turn.
func (c *CodexRPC) RunTurn(ctx context.Context, threadID string, hooks CodexTurnHooks) (err error) {
	defer func() {
		if err != nil {
			c.closed = true
		}
	}()
	if c.closed {
		return fmt.Errorf("%w: client closed after an earlier failure", ErrCodexProtocol)
	}
	if threadID == "" || hooks.Prepare == nil || hooks.Bind == nil {
		return errors.New("Codex turn requires a thread and durable hooks")
	}
	if err := hooks.Prepare(); err != nil {
		return fmt.Errorf("send intent not recorded; turn/start not sent: %w", err)
	}
	c.next++
	id := c.next
	if err := c.write(map[string]any{"id": id, "method": "turn/start", "params": map[string]any{
		"threadId": threadID,
		"input":    []any{map[string]any{"type": "text", "text": CodexTurnPrompt}},
	}}); err != nil {
		return err
	}
	var guard *CodexTurnGuard
	var early []codexFrame
	seen := map[string]bool{}
	handle := func(frame codexFrame) (done bool, err error) {
		if frame.hasID() && frame.Method != "" {
			key := string(frame.ID)
			if seen[key] {
				return false, fmt.Errorf("%w: repeated server request id", ErrCodexProtocol)
			}
			seen[key] = true
			if frame.Method != "item/tool/call" {
				return false, c.respond(frame.ID, nil, "not supported by Agent Messenger")
			}
			result, toolErr := guard.Tool(frame.Params)
			if toolErr != nil {
				_ = c.respond(frame.ID, nil, "tool call rejected")
				return false, toolErr
			}
			return false, c.respond(frame.ID, result, "")
		}
		switch frame.Method {
		case "turn/completed":
			if err := guard.Complete(frame.Params); err != nil {
				return false, err
			}
			return true, nil
		}
		return false, nil
	}
	for {
		select {
		case <-ctx.Done():
			if guard != nil {
				guard.Stop()
			}
			return fmt.Errorf("turn not finished; outcome unknown: %w", ctx.Err())
		case frame, ok := <-c.frames:
			if !ok {
				if guard != nil {
					guard.Stop()
				}
				return fmt.Errorf("app-server stopped during the turn; outcome unknown: %w", c.readErr())
			}
			if guard == nil {
				if frame.hasID() && frame.Method == "" {
					var got int
					if json.Unmarshal(frame.ID, &got) != nil || got != id {
						return fmt.Errorf("%w: unexpected response id", ErrCodexProtocol)
					}
					if len(frame.Error) != 0 || len(frame.Result) == 0 {
						return errors.New("app-server refused turn/start; outcome unknown")
					}
					var result struct {
						Turn struct {
							ID string `json:"id"`
						} `json:"turn"`
					}
					if json.Unmarshal(frame.Result, &result) != nil || result.Turn.ID == "" || len(result.Turn.ID) > 128 {
						return fmt.Errorf("%w: turn id missing", ErrCodexProtocol)
					}
					if err := hooks.Bind(result.Turn.ID); err != nil {
						return fmt.Errorf("turn id not recorded: %w", err)
					}
					g, err := NewCodexTurnGuard(threadID, result.Turn.ID, hooks.Tools)
					if err != nil {
						return err
					}
					guard = g
					for _, buffered := range early {
						done, err := handle(buffered)
						if err != nil || done {
							if err != nil {
								guard.Stop()
							}
							return err
						}
					}
					early = nil
					continue
				}
				if !frame.hasID() && frame.Method == "" {
					return fmt.Errorf("%w: response without id", ErrCodexProtocol)
				}
				if len(early) >= codexMaxEarlyFrames {
					return fmt.Errorf("%w: too many events before turn/start response", ErrCodexProtocol)
				}
				early = append(early, frame)
				continue
			}
			if frame.hasID() && frame.Method == "" {
				guard.Stop()
				return fmt.Errorf("%w: unexpected response during the turn", ErrCodexProtocol)
			}
			done, err := handle(frame)
			if err != nil {
				guard.Stop()
				return err
			}
			if done {
				return nil
			}
		}
	}
}
