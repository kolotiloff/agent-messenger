package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type client struct {
	t     *testing.T
	in    *io.PipeWriter
	lines chan map[string]any
	done  chan error
}

func start(t *testing.T, dir string) *client {
	t.Helper()
	inR, inW := io.Pipe()
	outR, outW := io.Pipe()
	c := &client{t: t, in: inW, lines: make(chan map[string]any, 16), done: make(chan error, 1)}
	go func() {
		err := newServer(dir).serve(inR, outW)
		_ = outW.Close()
		c.done <- err
	}()
	go func() {
		scanner := bufio.NewScanner(outR)
		for scanner.Scan() {
			var m map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &m); err == nil {
				c.lines <- m
			}
		}
		close(c.lines)
	}()
	t.Cleanup(func() {
		_ = inW.Close()
		select {
		case <-c.done:
		case <-time.After(5 * time.Second):
			t.Error("server did not stop")
		}
	})
	return c
}

func (c *client) send(msg string) {
	c.t.Helper()
	if _, err := io.WriteString(c.in, msg+"\n"); err != nil {
		c.t.Fatal(err)
	}
}

func (c *client) next(timeout time.Duration) map[string]any {
	c.t.Helper()
	select {
	case m := <-c.lines:
		return m
	case <-time.After(timeout):
		c.t.Fatal("no response")
		return nil
	}
}

func (c *client) none(d time.Duration) {
	c.t.Helper()
	select {
	case m := <-c.lines:
		c.t.Fatalf("unexpected response %v", m)
	case <-time.After(d):
	}
}

// payload decodes the JSON text a tool call returned.
func payload(t *testing.T, m map[string]any) map[string]any {
	t.Helper()
	result, _ := m["result"].(map[string]any)
	content, _ := result["content"].([]any)
	if len(content) != 1 || result["isError"] == true {
		t.Fatalf("bad tool result %v", m)
	}
	var p map[string]any
	if err := json.Unmarshal([]byte(content[0].(map[string]any)["text"].(string)), &p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestInitializeAndList(t *testing.T) {
	c := start(t, t.TempDir())
	c.send(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26"}}`)
	m := c.next(time.Second)
	if m["result"].(map[string]any)["protocolVersion"] != "2025-03-26" {
		t.Fatalf("protocol not echoed: %v", m)
	}
	c.send(`{"jsonrpc":"2.0","method":"notifications/initialized"}`)
	c.send(`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`)
	tools := c.next(time.Second)["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 2 {
		t.Fatalf("tools %v", tools)
	}
	c.send(`{"jsonrpc":"2.0","id":3,"method":"nope"}`)
	if c.next(time.Second)["error"] == nil {
		t.Fatal("unknown method accepted")
	}
}

func TestWaitWakesOnEventAndTimesOut(t *testing.T) {
	dir := t.TempDir()
	c := start(t, dir)
	c.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"wait","arguments":{"after":0,"timeout_seconds":1}}}`)
	p := payload(t, c.next(3*time.Second))
	if p["timed_out"] != true || p["cursor"].(float64) != 0 {
		t.Fatalf("expected empty timeout, got %v", p)
	}

	c.send(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"wait","arguments":{"after":0,"timeout_seconds":60}}}`)
	c.none(500 * time.Millisecond)
	began := time.Now()
	if _, err := appendEvent(dir, "worker A: done"); err != nil {
		t.Fatal(err)
	}
	p = payload(t, c.next(3*time.Second))
	if time.Since(began) > 2*time.Second {
		t.Fatal("wait woke too slowly")
	}
	events := p["events"].([]any)
	if p["timed_out"] != false || p["cursor"].(float64) != 1 || len(events) != 1 || events[0].(map[string]any)["text"] != "worker A: done" {
		t.Fatalf("bad wake %v", p)
	}
}

func TestOtherCallsAnsweredWhileWaitBlocks(t *testing.T) {
	dir := t.TempDir()
	c := start(t, dir)
	c.send(`{"jsonrpc":"2.0","id":"w","method":"tools/call","params":{"name":"wait","arguments":{"after":0,"timeout_seconds":60}}}`)
	c.send(`{"jsonrpc":"2.0","id":2,"method":"ping"}`)
	if m := c.next(time.Second); m["id"].(float64) != 2 {
		t.Fatalf("ping blocked: %v", m)
	}
	_, _ = appendEvent(dir, "one")
	c.send(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"status","arguments":{"after":0}}}`)
	got := map[any]map[string]any{}
	for i := 0; i < 2; i++ {
		m := c.next(3 * time.Second)
		got[m["id"]] = m
	}
	if payload(t, got[float64(3)])["new_events"].(float64) != 1 || payload(t, got["w"])["cursor"].(float64) != 1 {
		t.Fatalf("bad results %v", got)
	}
}

// A cancelled wait gets no answer, and the event that arrives meanwhile is not lost:
// the cursor only moves when the caller passes it back.
func TestCancelledWaitLosesNothing(t *testing.T) {
	dir := t.TempDir()
	c := start(t, dir)
	c.send(`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"wait","arguments":{"after":0,"timeout_seconds":60}}}`)
	time.Sleep(200 * time.Millisecond)
	c.send(`{"jsonrpc":"2.0","method":"notifications/cancelled","params":{"requestId":7}}`)
	c.none(600 * time.Millisecond)
	_, _ = appendEvent(dir, "report while nobody listened")
	c.send(`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"wait","arguments":{"after":0,"timeout_seconds":5}}}`)
	p := payload(t, c.next(3*time.Second))
	if len(p["events"].([]any)) != 1 {
		t.Fatalf("event lost: %v", p)
	}
	// Without a cursor the same event is returned again rather than skipped.
	c.send(`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"wait","arguments":{"timeout_seconds":5}}}`)
	if p := payload(t, c.next(3*time.Second)); p["cursor"].(float64) != 1 {
		t.Fatalf("replay failed: %v", p)
	}
	var log strings.Builder
	if err := printLog(dir, &log); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "cancelled: 1") {
		t.Fatalf("log does not show the cancellation:\n%s", log.String())
	}
}

func TestEventNumbersAndTempRefusal(t *testing.T) {
	dir := t.TempDir()
	for i, text := range []string{"a", "b", "a"} {
		seq, err := appendEvent(dir, text)
		if err != nil || seq != i+1 {
			t.Fatalf("event %d got seq %d, %v", i+1, seq, err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "events.jsonl")); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"/tmp/x", "/private/tmp/x", "/var/folders/ab/x"} {
		if _, err := resolveDir(bad); err == nil {
			t.Fatalf("%s accepted", bad)
		}
	}
}

func TestProgressWhileWaiting(t *testing.T) {
	old := progressInterval
	progressInterval = 200 * time.Millisecond
	t.Cleanup(func() { progressInterval = old })
	dir := t.TempDir()
	c := start(t, dir)
	c.send(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"wait","_meta":{"progressToken":"p1"},"arguments":{"after":0,"timeout_seconds":30}}}`)
	m := c.next(2 * time.Second)
	if m["method"] != "notifications/progress" || m["params"].(map[string]any)["progressToken"] != "p1" {
		t.Fatalf("expected progress, got %v", m)
	}
	_, _ = appendEvent(dir, "done")
	for {
		m = c.next(3 * time.Second)
		if m["method"] == nil {
			break
		}
	}
	if payload(t, m)["cursor"].(float64) != 1 {
		t.Fatalf("bad result %v", m)
	}
}
