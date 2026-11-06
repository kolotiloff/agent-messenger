// Command wait-probe is a pilot for RETHINK.md version 3. It answers one
// question: can a coordinator chat in the Claude or Codex app wait for worker
// reports through a blocking MCP tool, without spending model tokens while it
// waits? It is not part of the product and launches no agents.
//
//	wait-probe serve           MCP stdio server with the tools wait and status
//	wait-probe send TEXT       append an event, as a worker report would
//	wait-probe log             summarize what the server saw
//
// Events are lines in events.jsonl; an event's number is its line number, so a
// cursor is just a count. The server never advances a cursor on its own: a
// caller that does not pass the previous cursor gets the same events again.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultWait  = 600
	maxWait      = 3600
	pollInterval = 300 * time.Millisecond
	maxEventText = 4000
	maxReturned  = 20
)

// progressInterval keeps clients from treating a long wait as idle. Claude Code
// aborts a stdio call after 30 silent minutes. Progress does not wake the model.
var progressInterval = 60 * time.Second

const instructions = `Pilot of Agent Messenger coordinator waiting. Events stand for reports and questions from workers.
After the owner asks you to wait, call wait. Always pass after = the cursor from the previous wait or status result.
When wait returns events, tell the owner what arrived in one short line, then call wait again with the new cursor.
When wait returns timed_out with no events, call wait again without commenting.
Stop waiting when the owner says so or an event says STOP.`

type event struct {
	Seq  int    `json:"seq"`
	Time string `json:"time"`
	Text string `json:"text"`
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	cmd := os.Args[1]
	fs := flag.NewFlagSet(cmd, flag.ExitOnError)
	dirFlag := fs.String("dir", "", "data directory (default ~/AgentMessengerTest/wait-probe)")
	delay := fs.Int("delay", 0, "send: seconds to wait before appending")
	_ = fs.Parse(os.Args[2:])
	dir, err := resolveDir(*dirFlag)
	if err != nil {
		fail(err)
	}
	switch cmd {
	case "serve":
		err = newServer(dir).serve(os.Stdin, os.Stdout)
	case "send":
		text := strings.TrimSpace(strings.Join(fs.Args(), " "))
		if text == "" {
			fail(errors.New("send needs a text"))
		}
		time.Sleep(time.Duration(*delay) * time.Second)
		var seq int
		seq, err = appendEvent(dir, text)
		if err == nil {
			fmt.Printf("event %d sent\n", seq)
		}
	case "log":
		err = printLog(dir, os.Stdout)
	default:
		usage()
	}
	if err != nil {
		fail(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: wait-probe serve|send|log [--dir DIR] [--delay SECONDS] [TEXT]")
	os.Exit(2)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "wait-probe:", err)
	os.Exit(1)
}

// resolveDir refuses temporary folders: macOS empties them on reboot.
func resolveDir(dir string) (string, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, "AgentMessengerTest", "wait-probe")
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for _, bad := range []string{"/tmp", "/private/tmp", "/var/folders", "/private/var/folders"} {
		if abs == bad || strings.HasPrefix(abs, bad+"/") {
			return "", fmt.Errorf("refusing temporary directory %s", abs)
		}
	}
	return abs, os.MkdirAll(abs, 0o700)
}

// appendEvent writes one line with O_APPEND, so concurrent senders do not mix lines.
// Its number is its line position, read back after the write.
func appendEvent(dir, text string) (int, error) {
	if len(text) > maxEventText {
		text = text[:maxEventText]
	}
	line, _ := json.Marshal(map[string]string{"time": time.Now().Format(time.RFC3339), "text": text})
	f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	_, err = f.Write(append(line, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return 0, err
	}
	events, err := readEvents(dir)
	if err != nil {
		return 0, err
	}
	for i := len(events) - 1; i >= 0; i-- {
		if string(mustJSON(map[string]string{"time": events[i].Time, "text": events[i].Text})) == string(line) {
			return events[i].Seq, nil
		}
	}
	return len(events), nil
}

func readEvents(dir string) ([]event, error) {
	f, err := os.Open(filepath.Join(dir, "events.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var events []event
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var e event
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			e.Text = "(unreadable line)"
		}
		e.Seq = len(events) + 1
		events = append(events, e)
	}
	return events, scanner.Err()
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

type server struct {
	dir     string
	out     sync.Mutex
	w       io.Writer
	mu      sync.Mutex
	pending map[string]context.CancelFunc
}

func newServer(dir string) *server {
	return &server{dir: dir, pending: map[string]context.CancelFunc{}}
}

type rpcRequest struct {
	ID     json.RawMessage `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

// serve handles each tools/call in its own goroutine, so a blocked wait does
// not stop ping, status or cancellation on the same connection.
func (s *server) serve(in io.Reader, out io.Writer) error {
	s.w = out
	s.record(map[string]any{"kind": "server_start", "pid": os.Getpid()})
	var wg sync.WaitGroup
	defer wg.Wait()
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		var req rpcRequest
		if json.Unmarshal(scanner.Bytes(), &req) != nil {
			continue
		}
		hasID := len(req.ID) > 0 && string(req.ID) != "null"
		switch {
		case req.Method == "notifications/cancelled":
			var p struct {
				RequestID json.RawMessage `json:"requestId"`
			}
			_ = json.Unmarshal(req.Params, &p)
			s.mu.Lock()
			if cancel := s.pending[string(p.RequestID)]; cancel != nil {
				cancel()
			}
			s.mu.Unlock()
		case !hasID:
		case req.Method == "initialize":
			var p struct {
				ProtocolVersion string `json:"protocolVersion"`
			}
			_ = json.Unmarshal(req.Params, &p)
			if p.ProtocolVersion == "" {
				p.ProtocolVersion = "2025-06-18"
			}
			s.respond(req.ID, map[string]any{
				"protocolVersion": p.ProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "wait-probe", "version": "0.1"},
				"instructions":    instructions,
			}, nil)
		case req.Method == "ping":
			s.respond(req.ID, map[string]any{}, nil)
		case req.Method == "tools/list":
			s.respond(req.ID, map[string]any{"tools": toolList()}, nil)
		case req.Method == "tools/call":
			ctx, cancel := context.WithCancel(context.Background())
			s.mu.Lock()
			s.pending[string(req.ID)] = cancel
			s.mu.Unlock()
			wg.Add(1)
			go func(req rpcRequest) {
				defer wg.Done()
				defer func() {
					s.mu.Lock()
					delete(s.pending, string(req.ID))
					s.mu.Unlock()
					cancel()
				}()
				result, err := s.call(ctx, req)
				if ctx.Err() != nil {
					return // cancelled: MCP says not to answer
				}
				if err != nil {
					s.respond(req.ID, toolText(err.Error(), true), nil)
					return
				}
				s.respond(req.ID, toolText(string(mustJSON(result)), false), nil)
			}(req)
		default:
			s.respond(req.ID, nil, map[string]any{"code": -32601, "message": "method not found"})
		}
	}
	s.mu.Lock()
	for _, cancel := range s.pending {
		cancel()
	}
	s.mu.Unlock()
	s.record(map[string]any{"kind": "server_stop"})
	return scanner.Err()
}

func toolList() []map[string]any {
	after := map[string]any{"type": "integer", "minimum": 0, "description": "cursor from the previous wait or status result"}
	return []map[string]any{
		{"name": "wait", "description": "Block until a new event (a worker report or question) arrives after the cursor, or until timeout_seconds pass. Costs no tokens while blocked. Always pass after.",
			"inputSchema": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{
				"after":           after,
				"timeout_seconds": map[string]any{"type": "integer", "minimum": 1, "maximum": maxWait},
			}}},
		{"name": "status", "description": "Return the current cursor and how many events are after the given cursor, without waiting.",
			"inputSchema": map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"after": after}}},
	}
}

func toolText(text string, isError bool) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": isError}
}

func (s *server) call(ctx context.Context, req rpcRequest) (any, error) {
	var p struct {
		Name string `json:"name"`
		Meta struct {
			ProgressToken json.RawMessage `json:"progressToken"`
		} `json:"_meta"`
		Arguments struct {
			After   *int `json:"after"`
			Timeout int  `json:"timeout_seconds"`
		} `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &p); err != nil {
		return nil, err
	}
	after := 0
	if p.Arguments.After != nil {
		after = *p.Arguments.After
	}
	switch p.Name {
	case "status":
		events, err := readEvents(s.dir)
		if err != nil {
			return nil, err
		}
		s.record(map[string]any{"kind": "status", "id": string(req.ID), "after": after, "cursor": len(events)})
		return map[string]any{"cursor": len(events), "new_events": max(0, len(events)-after)}, nil
	case "wait":
		timeout := p.Arguments.Timeout
		if timeout <= 0 {
			timeout = defaultWait
		}
		timeout = min(timeout, maxWait)
		return s.wait(ctx, string(req.ID), after, p.Arguments.After != nil, time.Duration(timeout)*time.Second, p.Meta.ProgressToken)
	}
	return nil, fmt.Errorf("unknown tool %q", p.Name)
}

func (s *server) wait(ctx context.Context, id string, after int, gotAfter bool, timeout time.Duration, progressToken json.RawMessage) (any, error) {
	start := time.Now()
	hasProgress := len(progressToken) > 0 && string(progressToken) != "null"
	s.record(map[string]any{"kind": "wait_start", "id": id, "after": after, "after_given": gotAfter, "timeout_s": int(timeout.Seconds()), "progress": hasProgress})
	lastProgress := start
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	tick := time.NewTicker(pollInterval)
	defer tick.Stop()
	for {
		events, err := readEvents(s.dir)
		if err != nil {
			return nil, err
		}
		if len(events) > after {
			fresh := events[after:]
			if len(fresh) > maxReturned {
				fresh = fresh[:maxReturned]
			}
			cursor := after + len(fresh)
			s.record(map[string]any{"kind": "wait_events", "id": id, "after": after, "cursor": cursor, "count": len(fresh), "waited_s": time.Since(start).Seconds()})
			return map[string]any{"events": fresh, "cursor": cursor, "timed_out": false,
				"next": fmt.Sprintf("Tell the owner what arrived, then call wait with after=%d.", cursor)}, nil
		}
		select {
		case <-ctx.Done():
			s.record(map[string]any{"kind": "wait_cancelled", "id": id, "after": after, "waited_s": time.Since(start).Seconds()})
			return nil, ctx.Err()
		case <-deadline.C:
			s.record(map[string]any{"kind": "wait_timeout", "id": id, "after": after, "waited_s": time.Since(start).Seconds()})
			return map[string]any{"events": []event{}, "cursor": after, "timed_out": true,
				"next": fmt.Sprintf("Nothing yet. Call wait again with after=%d.", after)}, nil
		case <-tick.C:
			if hasProgress && time.Since(lastProgress) >= progressInterval {
				lastProgress = time.Now()
				s.notify("notifications/progress", map[string]any{"progressToken": progressToken,
					"progress": int(time.Since(start).Seconds()), "message": "waiting for worker events"})
			}
		}
	}
}

func (s *server) notify(method string, params any) {
	s.out.Lock()
	defer s.out.Unlock()
	_, _ = s.w.Write(append(mustJSON(map[string]any{"jsonrpc": "2.0", "method": method, "params": params}), '\n'))
}

func (s *server) respond(id json.RawMessage, result any, rpcErr any) {
	msg := map[string]any{"jsonrpc": "2.0", "id": id}
	if rpcErr != nil {
		msg["error"] = rpcErr
	} else {
		msg["result"] = result
	}
	s.out.Lock()
	defer s.out.Unlock()
	_, _ = s.w.Write(append(mustJSON(msg), '\n'))
}

// record appends to calls.jsonl; the log is the pilot's measurement.
func (s *server) record(entry map[string]any) {
	entry["time"] = time.Now().Format(time.RFC3339)
	f, err := os.OpenFile(filepath.Join(s.dir, "calls.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.Write(append(mustJSON(entry), '\n'))
}

func printLog(dir string, out io.Writer) error {
	f, err := os.Open(filepath.Join(dir, "calls.jsonl"))
	if errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(out, "no calls yet")
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	counts := map[string]int{}
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e map[string]any
		if json.Unmarshal(scanner.Bytes(), &e) != nil {
			continue
		}
		kind, _ := e["kind"].(string)
		counts[kind]++
		fmt.Fprintln(out, describe(e))
	}
	fmt.Fprintf(out, "\nwait calls: %d; returned with events: %d; empty timeouts (model woke for nothing): %d; cancelled: %d; server starts: %d\n",
		counts["wait_start"], counts["wait_events"], counts["wait_timeout"], counts["wait_cancelled"], counts["server_start"])
	return scanner.Err()
}

func describe(e map[string]any) string {
	t, _ := e["time"].(string)
	if len(t) >= 19 {
		t = t[11:19]
	}
	num := func(k string) string { return fmt.Sprint(e[k]) }
	secs := func() string {
		v, _ := e["waited_s"].(float64)
		return fmt.Sprintf("%.0fs", v)
	}
	switch e["kind"] {
	case "wait_start":
		return fmt.Sprintf("%s  wait #%s started, after=%s (given: %s), limit %ss, progress: %s", t, num("id"), num("after"), num("after_given"), num("timeout_s"), num("progress"))
	case "wait_events":
		return fmt.Sprintf("%s  wait #%s woke with %s event(s) after %s, cursor=%s", t, num("id"), num("count"), secs(), num("cursor"))
	case "wait_timeout":
		return fmt.Sprintf("%s  wait #%s timed out empty after %s", t, num("id"), secs())
	case "wait_cancelled":
		return fmt.Sprintf("%s  wait #%s cancelled by the client after %s", t, num("id"), secs())
	case "status":
		return fmt.Sprintf("%s  status #%s, cursor=%s", t, num("id"), num("cursor"))
	default:
		return fmt.Sprintf("%s  %s", t, e["kind"])
	}
}
