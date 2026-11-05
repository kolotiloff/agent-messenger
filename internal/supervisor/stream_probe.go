//go:build streamprobe

package supervisor

import (
	"bufio"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// The entire experiment is excluded from ordinary product builds.
// A process owner remains locked in the product DB and global run registry.
// Per-turn completion never clears that owner and is not process-exit proof.
type StreamProbeResult struct {
	Passed              bool     `json:"passed"`
	Launches            int      `json:"cli_launches"`
	Prompts             int      `json:"prompts_sent"`
	Results             int      `json:"valid_results"`
	Replies             []string `json:"linked_reply_texts"`
	IdleSeconds         int      `json:"idle_seconds"`
	IdleModelEvents     int      `json:"idle_model_events"`
	SameProcess         bool     `json:"same_process"`
	ExitObserved        bool     `json:"exit_observed"`
	ExitCode            int      `json:"exit_code"`
	ReservationRetained bool     `json:"reservation_retained"`
	Error               string   `json:"error,omitempty"`
}

type probeTurn struct {
	ID         string `json:"id"`
	Text       string `json:"text"`
	Receipt    string `json:"receipt"`
	ReplyTo    string `json:"reply_to,omitempty"`
	Reply      string `json:"reply,omitempty"`
	Read       bool   `json:"read"`
	ProviderOK bool   `json:"provider_ok"`
}

// gate is a separate experiment ledger. A receipt is an unguessable per-turn
// capability, not a model-selected address. Late replies cannot bind to a new turn.
type probeGate struct {
	mu       sync.Mutex
	secret   string
	identity bool
	closed   bool
	current  *probeTurn
	turns    []*probeTurn
	persist  func([]*probeTurn) error
	onReply  func() error
}

func (g *probeGate) open(id, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || g.current != nil {
		return errors.New("probe is closed or already processing a turn")
	}
	receipt, err := newRunID()
	if err != nil {
		return err
	}
	t := &probeTurn{ID: id, Text: text, Receipt: receipt}
	g.turns = append(g.turns, t)
	if err := g.persist(g.turns); err != nil {
		g.closed = true
		return err
	}
	g.current = t
	return nil
}

func (g *probeGate) complete(expected string) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	t := g.current
	if g.closed || !g.identity || t == nil || !t.Read || t.ReplyTo != t.ID || t.Reply != expected {
		g.closed = true
		return "", errors.New("missing or incorrect bound reply")
	}
	t.ProviderOK = true
	if err := g.persist(g.turns); err != nil {
		g.closed = true
		return "", err
	}
	g.current = nil
	return t.Reply, nil
}

func (g *probeGate) close() { g.mu.Lock(); g.closed = true; g.mu.Unlock() }

func (g *probeGate) tool(name string, args map[string]any) (any, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed || !g.identity || g.current == nil {
		return nil, errors.New("no authorized active turn")
	}
	t := g.current
	switch name {
	case "read_current":
		if len(args) != 0 {
			return nil, errors.New("read_current takes no arguments")
		}
		t.Read = true
		if err := g.persist(g.turns); err != nil {
			g.closed = true
			return nil, err
		}
		return map[string]any{"text": t.Text, "receipt": t.Receipt}, nil
	case "reply":
		receipt, ok := args["receipt"].(string)
		text, valid := args["text"].(string)
		if len(args) != 2 || !ok || !valid || !t.Read || receipt != t.Receipt || len(text) < 1 || len(text) > 2000 {
			return nil, errors.New("reply requires current read receipt and text; route is not an argument")
		}
		if t.Reply != "" && t.Reply != text {
			return nil, errors.New("conflicting duplicate reply")
		}
		if g.onReply != nil {
			if err := g.onReply(); err != nil {
				g.closed = true
				return nil, err
			}
		}
		t.ReplyTo = t.ID
		t.Reply = text
		if err := g.persist(g.turns); err != nil {
			g.closed = true
			return nil, err
		}
		return map[string]any{"stored": true}, nil
	default:
		return nil, errors.New("unknown tool")
	}
}

func (g *probeGate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/mcp" || r.Header.Get("Origin") != "" || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+g.secret)) != 1 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID     json.RawMessage `json:"id"`
		Method string          `json:"method"`
		Params map[string]any  `json:"params"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "bad JSON", 400)
		return
	}
	if len(req.ID) == 0 || string(req.ID) == "null" {
		w.WriteHeader(202)
		return
	}
	var result any
	switch req.Method {
	case "initialize":
		version, _ := req.Params["protocolVersion"].(string)
		if version != "2025-03-26" && version != "2025-06-18" {
			version = "2025-06-18"
		}
		result = map[string]any{"protocolVersion": version, "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "messenger-stream-probe", "version": "0.1"}}
	case "ping":
		result = map[string]any{}
	case "tools/list":
		result = map[string]any{"tools": []any{
			map[string]any{"name": "read_current", "description": "Read the assigned message and its one-turn receipt.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{}, "additionalProperties": false}},
			map[string]any{"name": "reply", "description": "Reply using the receipt returned by read_current. Routing is assigned by the host.", "inputSchema": map[string]any{"type": "object", "properties": map[string]any{"text": map[string]any{"type": "string", "minLength": 1, "maxLength": 2000}, "receipt": map[string]any{"type": "string"}}, "required": []string{"text", "receipt"}, "additionalProperties": false}},
		}}
	case "tools/call":
		name, _ := req.Params["name"].(string)
		args, _ := req.Params["arguments"].(map[string]any)
		value, err := g.tool(name, args)
		data, _ := json.Marshal(value)
		if err != nil {
			data = []byte(err.Error())
		}
		result = map[string]any{"content": []any{map[string]any{"type": "text", "text": string(data)}}, "isError": err != nil}
	default:
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "error": map[string]any{"code": -32601, "message": "Method not found"}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
}

func writeProbeJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, raw)
}

// RunClaudeStreamProbe makes a fresh, unguessable test session, never resumes an
// existing conversation. Its only CLI launch receives at most two prompts. A
// failure never retries. Requires the same root as any existing global reserve.
func RunClaudeStreamProbe(ctx context.Context, root, cli, private string) (result StreamProbeResult, retErr error) {
	result.ExitCode = -1
	if !filepath.IsAbs(root) || !filepath.IsAbs(cli) || !filepath.IsAbs(private) {
		return result, errors.New("absolute paths required")
	}
	if err := os.Mkdir(private, 0700); err != nil {
		return result, err
	}
	defer func() {
		if retErr != nil {
			result.Passed = false
			result.Error = retErr.Error()
		}
		if err := writeProbeJSON(filepath.Join(private, "result.json"), result); err != nil && retErr == nil {
			retErr = err
		}
	}()
	lockPath, err := platform.SupervisorLockFile()
	if err != nil {
		return result, err
	}
	lock, err := platform.Acquire(lockPath)
	if err != nil {
		return result, err
	}
	defer lock.Release()
	if err := checkStartupLease(); err != nil {
		return result, err
	}
	if err := reconcileRuns(root); err != nil {
		return result, err
	}
	store, err := core.Open(filepath.Join(root, dbName))
	if err != nil {
		return result, err
	}
	defer store.Close()
	random, err := newRunID()
	if err != nil {
		return result, err
	}
	sid := random[:8] + "-" + random[8:12] + "-4" + random[13:16] + "-8" + random[17:20] + "-" + random[20:]
	profile, err := defaultProfileDir("claude")
	if err != nil {
		return result, err
	}
	workspace := filepath.Join(private, "workspace")
	if err := os.Mkdir(workspace, 0700); err != nil {
		return result, err
	}
	alias := "stream-probe-" + random
	if _, err := store.Register("claude", profile, sid, alias); err != nil {
		return result, err
	}
	// No ConfigureLaunch: product serve cannot launch an experiment-owned session.
	if _, err := store.Enqueue("owner-"+random, alias, "stream-probe", "Owner of a bounded streaming experiment; per-turn evidence is in its private ledger."); err != nil {
		return result, err
	}
	token, err := store.Claim(alias)
	if err != nil {
		return result, err
	}
	if err := store.BeginLaunch(token); err != nil {
		return result, err
	}
	defer func() {
		if err := store.MarkUncertain(token, "stream probe ended; session owner remains reserved"); err != nil && retErr == nil {
			retErr = err
		}
	}()
	if err := writeProbeJSON(filepath.Join(private, "binding.json"), map[string]any{"session_id": sid, "owner_token": token, "root": root}); err != nil {
		return result, err
	}
	secret, err := newRunID()
	if err != nil {
		return result, err
	}
	gate := &probeGate{secret: secret, persist: func(turns []*probeTurn) error { return writeProbeJSON(filepath.Join(private, "turns.json"), turns) }}
	gate.onReply = func() error {
		_, err := store.Reply(token, "Streaming experiment received a bound reply; see its private per-turn ledger.")
		return err
	}
	defer gate.close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return result, err
	}
	server := &http.Server{Handler: gate, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second}
	go func() { _ = server.Serve(listener) }()
	defer server.Close()
	config, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"messenger_bound": map[string]any{"type": "http", "url": "http://" + listener.Addr().String() + "/mcp", "headers": map[string]string{"Authorization": "Bearer " + secret}}}})
	args := []string{"-p", "--input-format", "stream-json", "--output-format", "stream-json", "--verbose", "--tools", "", "--strict-mcp-config", "--mcp-config", string(config), "--setting-sources", "", "--settings", `{"disableAllHooks":true}`, "--disable-slash-commands", "--no-chrome", "--permission-mode", "dontAsk", "--allowedTools", "mcp__messenger_bound__read_current,mcp__messenger_bound__reply", "--session-id", sid}
	cmd := exec.Command(cli, args...)
	cmd.Env = profileEnv(os.Environ(), "claude", profile)
	cmd.Dir = workspace
	if err := platform.SetNewGroup(cmd); err != nil {
		return result, err
	}
	in, err := cmd.StdinPipe()
	if err != nil {
		return result, err
	}
	defer in.Close()
	// Own the output pipe: Cmd.Wait must not close it while the scanner drains it.
	out, writer, err := os.Pipe()
	if err != nil {
		return result, err
	}
	defer out.Close()
	defer writer.Close()
	cmd.Stdout = writer
	log, err := os.OpenFile(filepath.Join(private, "cli.stderr"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return result, err
	}
	defer log.Close()
	cmd.Stderr = log
	reservation := recordedRun{ID: random, Root: root, Provider: "claude", Environment: profile, ConversationID: sid}
	if err := rememberRun(reservation); err != nil {
		return result, err
	}
	result.ReservationRetained = true
	if err := cmd.Start(); err != nil {
		return result, err
	}
	result.Launches = 1
	_ = writer.Close()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	var obs platform.Observation
	defer func() {
		gate.close()
		_ = in.Close()
		select {
		case <-done:
			result.ExitObserved = true
		case <-time.After(15 * time.Second):
			if owned, _ := platform.OwnsObservation(obs); owned {
				_ = platform.SignalGroup(obs.PGID)
			}
			select {
			case <-done:
				result.ExitObserved = true
			case <-time.After(5 * time.Second):
			}
		}
		if result.ExitObserved {
			result.ExitCode = cmd.ProcessState.ExitCode()
		}
		if result.ExitObserved && obs.PID > 0 {
			empty, _ := platform.GroupEmpty(obs.PGID)
			if _, err := store.FinishObserved(token, result.ExitCode, empty, result.Results == 2, "stream probe; turn results do not prove descendant exit"); err != nil && retErr == nil {
				retErr = err
			}
		}
		result.Passed = retErr == nil && result.Results == 2 && result.Prompts == 2 && result.Launches == 1 && result.SameProcess && result.IdleModelEvents == 0 && result.ExitObserved && result.ExitCode == 0
		if !result.Passed && retErr == nil {
			retErr = errors.New("probe did not satisfy all acceptance checks")
		}
	}()
	obs, err = platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		return result, err
	}
	reservation.PID = obs.PID
	reservation.PGID = obs.PGID
	reservation.StartToken = obs.StartToken
	reservation.BootID = obs.BootID
	if err := rememberRun(reservation); err != nil {
		return result, err
	}
	if err := store.RecordProcess(token, obs); err != nil {
		return result, err
	}
	if err := writeProbeJSON(filepath.Join(private, "observation.json"), obs); err != nil {
		return result, err
	}
	rawLog, err := os.OpenFile(filepath.Join(private, "cli.stdout.jsonl"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return result, err
	}
	events := make(chan map[string]any, 128)
	readErr := make(chan error, 1)
	readCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		defer close(events)
		defer rawLog.Close()
		s := bufio.NewScanner(out)
		s.Buffer(make([]byte, 65536), 4<<20)
		for s.Scan() {
			if _, err := rawLog.Write(append(append([]byte{}, s.Bytes()...), byte(10))); err != nil {
				readErr <- err
				return
			}
			var ev map[string]any
			if err := json.Unmarshal(s.Bytes(), &ev); err != nil {
				readErr <- err
				return
			}
			select {
			case events <- ev:
			case <-readCtx.Done():
				return
			}
		}
		readErr <- s.Err()
	}()
	initialized := false
	for i, payload := range []string{"PAYLOAD-one", "PAYLOAD-two"} {
		if owned, err := platform.OwnsObservation(obs); err != nil || !owned {
			return result, errors.New("original CLI identity no longer owned")
		}
		if err := gate.open(fmt.Sprintf("stream-%d", i+1), payload); err != nil {
			return result, err
		}
		prompt := "Authorized streaming Messenger test. Use ONLY messenger_bound MCP tools. First call read_current, then reply with text ACK:<the exact text returned by read_current> and its receipt. Never reuse an earlier receipt. Do not choose addresses, do not poll, and do not call other tools."
		frame := map[string]any{"type": "user", "session_id": sid, "message": map[string]any{"role": "user", "content": prompt}, "parent_tool_use_id": nil}
		if err := json.NewEncoder(in).Encode(frame); err != nil {
			return result, err
		}
		result.Prompts++
		fmt.Printf("stream probe: prompt %d sent to the single CLI process\n", i+1)
		var turnEvents []map[string]any
		deadline := time.NewTimer(150 * time.Second)
		finished := false
		for !finished {
			select {
			case <-ctx.Done():
				deadline.Stop()
				return result, ctx.Err()
			case <-deadline.C:
				return result, errors.New("turn timeout; no retry")
			case event, ok := <-events:
				if !ok {
					deadline.Stop()
					return result, fmt.Errorf("CLI stream ended before result: %v", <-readErr)
				}
				if id, ok := event["session_id"].(string); ok && id != sid {
					deadline.Stop()
					return result, errors.New("event session identity mismatch")
				}
				if event["type"] == "system" && event["subtype"] == "init" {
					first, err := checkProbeInit(event, sid, initialized)
					if err != nil {
						deadline.Stop()
						return result, err
					}
					if owned, err := platform.OwnsObservation(obs); err != nil || !owned {
						deadline.Stop()
						return result, errors.New("CLI identity changed at initialization")
					}
					if first {
						if err := store.ConfirmIdentity(token, "claude", profile, sid); err != nil {
							deadline.Stop()
							return result, err
						}
						initialized = true
						gate.mu.Lock()
						gate.identity = true
						gate.mu.Unlock()
					}
				}
				turnEvents = append(turnEvents, event)
				if event["type"] == "result" {
					finished = true
				}
			}
		}
		deadline.Stop()
		if err := writeProbeJSON(filepath.Join(private, fmt.Sprintf("events-%d.json", i+1)), turnEvents); err != nil {
			return result, err
		}
		if !initialized {
			return result, errors.New("no confirmed initialization")
		}
		if err := adapters.ValidateClaude(turnEvents, sid); err != nil {
			return result, err
		}
		reply, err := gate.complete("ACK:" + payload)
		if err != nil {
			return result, err
		}
		result.Results++
		result.Replies = append(result.Replies, reply)
		if err := observeProbeIdle(ctx, events, 20*time.Second, &result); err != nil {
			return result, err
		}
		if owned, err := platform.OwnsObservation(obs); err != nil || !owned {
			return result, errors.New("CLI exited or changed during idle")
		}
	}
	result.SameProcess = true
	return result, nil
}

func observeProbeIdle(ctx context.Context, events <-chan map[string]any, duration time.Duration, result *StreamProbeResult) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			result.IdleSeconds += int(duration / time.Second)
			return nil
		case event, ok := <-events:
			if !ok {
				return errors.New("CLI stream closed while idle")
			}
			t, _ := event["type"].(string)
			if t == "assistant" || t == "result" || t == "stream_event" || t == "control_request" || t == "user" {
				result.IdleModelEvents++
				return errors.New("unexpected model activity while idle")
			}
			if t == "system" && event["subtype"] == "init" {
				return errors.New("CLI reinitialized while idle")
			}
		}
	}
}

// Keep the compiler honest about the probe's explicit local transport only.
var _ http.Handler = (*probeGate)(nil)

// Claude emits system/init at each turn in the tested streaming CLI. Repeated
// init is acceptable only for the exact bound ID and the same observed process.
func checkProbeInit(event map[string]any, sid string, initialized bool) (bool, error) {
	if event["type"] != "system" || event["subtype"] != "init" || event["session_id"] != sid {
		return false, errors.New("incorrect initialization identity")
	}
	return !initialized, nil
}
