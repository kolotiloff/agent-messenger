//go:build streamprobe

package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func testProbeGate() *probeGate {
	return &probeGate{secret: "test-secret", persist: func([]*probeTurn) error { return nil }}
}

func readProbeReceipt(t *testing.T, g *probeGate) string {
	t.Helper()
	v, err := g.tool("read_current", nil)
	if err != nil {
		t.Fatal(err)
	}
	return v.(map[string]any)["receipt"].(string)
}

func TestStreamProbeReceiptCannotRouteToNextTurn(t *testing.T) {
	g := testProbeGate()
	if err := g.open("first", "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.tool("read_current", nil); err == nil {
		t.Fatal("MCP opened before identity")
	}
	g.identity = true
	old := readProbeReceipt(t, g)
	if err := g.open("second", "two"); err == nil {
		t.Fatal("parallel turn accepted")
	}
	if _, err := g.tool("reply", map[string]any{"text": "ACK:one", "receipt": old, "recipient": "wrong"}); err == nil {
		t.Fatal("model-selected route accepted")
	}
	if _, err := g.tool("reply", map[string]any{"text": "ACK:one", "receipt": old}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.complete("ACK:one"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.tool("reply", map[string]any{"text": "ACK:one", "receipt": old}); err == nil {
		t.Fatal("idle reply accepted")
	}
	if err := g.open("second", "two"); err != nil {
		t.Fatal(err)
	}
	current := readProbeReceipt(t, g)
	if current == old {
		t.Fatal("receipt reused")
	}
	if _, err := g.tool("reply", map[string]any{"text": "late", "receipt": old}); err == nil {
		t.Fatal("stale reply crossed turns")
	}
	if _, err := g.tool("reply", map[string]any{"text": "ACK:two", "receipt": current}); err != nil {
		t.Fatal(err)
	}
	if _, err := g.complete("ACK:two"); err != nil {
		t.Fatal(err)
	}
	if g.turns[0].ReplyTo != "first" || g.turns[1].ReplyTo != "second" {
		t.Fatal("incorrect reply binding")
	}
}

func TestStreamProbeConcurrentConflictingReplies(t *testing.T) {
	g := testProbeGate()
	g.identity = true
	if err := g.open("one", "payload"); err != nil {
		t.Fatal(err)
	}
	receipt := readProbeReceipt(t, g)
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, text := range []string{"A", "B"} {
		wg.Add(1)
		go func(text string) {
			defer wg.Done()
			_, err := g.tool("reply", map[string]any{"receipt": receipt, "text": text})
			results <- err
		}(text)
	}
	wg.Wait()
	close(results)
	ok := 0
	for err := range results {
		if err == nil {
			ok++
		}
	}
	if ok != 1 {
		t.Fatalf("accepted %d conflicting replies", ok)
	}
	stored := g.current.Reply
	if _, err := g.tool("reply", map[string]any{"receipt": receipt, "text": stored}); err != nil {
		t.Fatal("identical retry not idempotent", err)
	}
	g.close()
	if _, err := g.tool("read_current", nil); err == nil {
		t.Fatal("closed capability accepted")
	}
}

func TestStreamProbePersistenceFailureClosesGate(t *testing.T) {
	g := testProbeGate()
	g.identity = true
	if err := g.open("one", "payload"); err != nil {
		t.Fatal(err)
	}
	receipt := readProbeReceipt(t, g)
	g.persist = func([]*probeTurn) error { return errors.New("disk failed") }
	if _, err := g.tool("reply", map[string]any{"receipt": receipt, "text": "reply"}); err == nil {
		t.Fatal("write failure ignored")
	}
	g.persist = func([]*probeTurn) error { return nil }
	if _, err := g.tool("read_current", nil); err == nil {
		t.Fatal("gate reopened after failed write")
	}
}

func TestStreamProbeMissingOrWrongReplyCannotAdvance(t *testing.T) {
	for _, reply := range []string{"", "wrong"} {
		g := testProbeGate()
		g.identity = true
		if err := g.open("one", "payload"); err != nil {
			t.Fatal(err)
		}
		receipt := readProbeReceipt(t, g)
		if reply != "" {
			if _, err := g.tool("reply", map[string]any{"receipt": receipt, "text": reply}); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := g.complete("expected"); err == nil {
			t.Fatal("unverified reply advanced")
		}
		if err := g.open("two", "new"); err == nil {
			t.Fatal("failed turn allowed another prompt")
		}
	}
}

func TestStreamProbeHTTPAuthorizationAndRouting(t *testing.T) {
	g := testProbeGate()
	for _, tc := range []struct {
		auth, origin string
		status       int
	}{{"", "", 401}, {"Bearer wrong", "", 401}, {"Bearer test-secret", "https://example.com", 401}, {"Bearer test-secret", "", 200}} {
		r := httptest.NewRequest(http.MethodPost, "http://localhost/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
		r.Header.Set("Authorization", tc.auth)
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		g.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatalf("status %d, want %d", w.Code, tc.status)
		}
		if w.Code == 200 {
			var response map[string]any
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response["error"] != nil {
				t.Fatal(response)
			}
		}
	}
}

func TestStreamProbeIdleFailsOnModelActivityOrEOF(t *testing.T) {
	for _, event := range []string{"assistant", "result", "stream_event", "control_request", "user"} {
		ch := make(chan map[string]any, 1)
		ch <- map[string]any{"type": event}
		r := StreamProbeResult{}
		if err := observeProbeIdle(context.Background(), ch, time.Second, &r); err == nil || r.IdleModelEvents != 1 {
			t.Fatal("unsolicited activity not rejected", event)
		}
	}
	closed := make(chan map[string]any)
	close(closed)
	if err := observeProbeIdle(context.Background(), closed, time.Second, &StreamProbeResult{}); err == nil {
		t.Fatal("EOF treated as idle")
	}
	quiet := make(chan map[string]any)
	if err := observeProbeIdle(context.Background(), quiet, time.Millisecond, &StreamProbeResult{}); err != nil {
		t.Fatal(err)
	}
}

func TestStreamProbeRepeatedInitKeepsExactSession(t *testing.T) {
	correct := map[string]any{"type": "system", "subtype": "init", "session_id": "bound"}
	if first, err := checkProbeInit(correct, "bound", false); err != nil || !first {
		t.Fatal("first init rejected", err)
	}
	if first, err := checkProbeInit(correct, "bound", true); err != nil || first {
		t.Fatal("same-session per-turn init not accepted", err)
	}
	for _, id := range []any{"different", "", nil} {
		if _, err := checkProbeInit(map[string]any{"type": "system", "subtype": "init", "session_id": id}, "bound", true); err == nil {
			t.Fatal("incorrect repeated init accepted", id)
		}
	}
}
