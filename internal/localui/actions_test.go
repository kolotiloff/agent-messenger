package localui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

const actionHost = "127.0.0.1:1234"
const actionID = "ui-16f93337-7ea4-4453-9bc8-a12d780731db"
const actionBody = `{"message_id":"` + actionID + `","alias":"worker","text":"hello"}`

func postAction(h http.Handler, path, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "http://"+actionHost+"/api/"+path, strings.NewReader(body))
	r.Header.Set("Origin", "http://"+actionHost)
	r.Header.Set("Authorization", "Bearer secret")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestMutationEnvelopeAndOriginRequired(t *testing.T) {
	calls := 0
	h := newHandler(actionHost, "secret", func(context.Context, control.Request) (control.Response, error) {
		calls++
		return control.Response{OK: true}, nil
	}, nil)
	for _, tc := range []struct {
		name, path, body, origin, token, media string
		code                                   int
	}{
		{"missing origin", "enqueue", actionBody, "", "secret", "application/json", 403},
		{"foreign origin", "enqueue", actionBody, "https://evil.example", "secret", "application/json", 403},
		{"missing key", "enqueue", actionBody, "http://" + actionHost, "", "application/json", 401},
		{"form", "enqueue", actionBody, "http://" + actionHost, "secret", "text/plain", 415},
		{"sender injection", "enqueue", strings.TrimSuffix(actionBody, "}") + `,"sender":"admin"}`, "http://" + actionHost, "secret", "application/json", 400},
		{"extra command", "pause", `{"cmd":"shutdown"}`, "http://" + actionHost, "secret", "application/json", 400},
		{"two objects", "resume", `{} {}`, "http://" + actionHost, "secret", "application/json", 400},
		{"null command", "pause", `null`, "http://" + actionHost, "secret", "application/json", 400},
		{"bad ID", "enqueue", strings.Replace(actionBody, actionID, "new", 1), "http://" + actionHost, "secret", "application/json", 400},
		{"empty text", "enqueue", strings.Replace(actionBody, "hello", "   ", 1), "http://" + actionHost, "secret", "application/json", 400},
		{"oversize", "enqueue", strings.Replace(actionBody, "hello", strings.Repeat("x", 70000), 1), "http://" + actionHost, "secret", "application/json", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "http://"+actionHost+"/api/"+tc.path, strings.NewReader(tc.body))
			r.Header.Set("Origin", tc.origin)
			r.Header.Set("Authorization", "Bearer "+tc.token)
			r.Header.Set("Content-Type", tc.media)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.code {
				t.Fatalf("code %d: %s", w.Code, w.Body.String())
			}
		})
	}
	if calls != 0 {
		t.Fatal("rejected mutation reached client")
	}
}

func TestExplicitActionsOnlyAndQueueAck(t *testing.T) {
	var got control.Request
	var prepared control.Request
	h := newHandler(actionHost, "secret", func(ctx context.Context, r control.Request) (control.Response, error) {
		got = r
		if r.Cmd == "ui-prepare" {
			prepared = r
		}
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("no deadline")
		}
		created := true
		return control.Response{OK: true, Created: &created}, nil
	}, nil)
	for _, cmd := range []string{"enqueue", "pause", "resume"} {
		body := "{}"
		if cmd == "enqueue" {
			body = actionBody
		}
		w := postAction(h, cmd, body)
		expected := cmd
		if cmd == "enqueue" {
			expected = "ui-submit"
		}
		if w.Code != 200 || got.Cmd != expected {
			t.Fatalf("%s: %d %+v", cmd, w.Code, got)
		}
		if got.CLI != "" || got.Environment != "" || got.ConversationID != "" || got.Confirmation != "" {
			t.Fatal("unexpected control authority")
		}
		if cmd == "enqueue" {
			if prepared.Sender != "local-user" || prepared.MessageID != actionID || prepared.Text != "hello" || prepared.Alias != "worker" || got.MessageID != actionID {
				t.Fatalf("wrong envelope: %+v", got)
			}
			if strings.Contains(w.Body.String(), "provider_ok") {
				t.Fatal("queue ACK claims model success")
			}
		}
	}
}

func TestEnqueueLostResponseAndConcurrentRetryPersistOnce(t *testing.T) {
	st, err := core.Open(filepath.Join(t.TempDir(), "queue.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err = st.Register("synthetic", t.TempDir(), "test-only", "worker"); err != nil {
		t.Fatal(err)
	}
	var loseFirst atomic.Bool
	loseFirst.Store(true)
	h := newHandler(actionHost, "secret", func(_ context.Context, r control.Request) (control.Response, error) {
		if r.Cmd == "ui-prepare" {
			err := st.PrepareUIMessage(r.MessageID, r.Alias, r.Text)
			return control.Response{OK: err == nil}, err
		}
		created, err := st.SubmitUIMessage(r.MessageID)
		if err != nil {
			return control.Response{OK: false}, nil
		}
		if loseFirst.Swap(false) {
			return control.Response{}, errors.New("response lost after commit")
		}
		return control.Response{OK: true, Created: &created}, nil
	}, nil)
	first := postAction(h, "enqueue", actionBody)
	if first.Code != 503 || !strings.Contains(first.Header().Get("Content-Type"), "application/json") || !strings.Contains(first.Body.String(), "unconfirmed") {
		t.Fatalf("lost response: %d %s", first.Code, first.Body.String())
	}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := postAction(h, "enqueue", actionBody)
			var ack struct {
				OK, Created bool
				ID          string `json:"message_id"`
			}
			err := json.Unmarshal(w.Body.Bytes(), &ack)
			if w.Code != 200 || err != nil || !ack.OK || ack.Created || ack.ID != actionID {
				t.Errorf("duplicate: %d %s", w.Code, w.Body.String())
			}
		}()
	}
	wg.Wait()
	// Changing the text under the same ID must not be treated as another delivery.
	if w := postAction(h, "enqueue", strings.Replace(actionBody, "hello", "changed", 1)); w.Code != 503 {
		t.Fatal("conflicting body accepted")
	}
	if _, err = st.Claim("worker"); err != nil {
		t.Fatal(err)
	}
	if _, err = st.Claim("worker"); !errors.Is(err, core.ErrBusy) {
		t.Fatalf("second claim = %v", err)
	}
}

func TestMissingQueueAcknowledgmentIsUnconfirmed(t *testing.T) {
	h := newHandler(actionHost, "secret", func(context.Context, control.Request) (control.Response, error) {
		return control.Response{OK: true}, nil
	}, nil)
	if w := postAction(h, "enqueue", actionBody); w.Code != 503 {
		t.Fatal("missing created accepted")
	}
}

func TestUIPrepareDisconnectLeavesRecoverableEnvelopeWithoutQueue(t *testing.T) {
	st, err := core.Open(filepath.Join(t.TempDir(), "queue.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err = st.Register("synthetic", t.TempDir(), "test-only", "worker"); err != nil {
		t.Fatal(err)
	}
	submitted := 0
	h := newHandler(actionHost, "secret", func(_ context.Context, r control.Request) (control.Response, error) {
		switch r.Cmd {
		case "ui-prepare":
			if err := st.PrepareUIMessage(r.MessageID, r.Alias, r.Text); err != nil {
				return control.Response{}, err
			}
			return control.Response{}, errors.New("lost preparation ACK")
		case "ui-submit":
			submitted++
			return control.Response{}, errors.New("must not submit")
		case "ui-outbox":
			out, err := st.UIOutbox()
			raw, _ := json.Marshal(out)
			return control.Response{OK: err == nil, Outbox: raw}, err
		}
		return control.Response{}, errors.New("unexpected action")
	}, nil)
	if w := postAction(h, "enqueue", actionBody); w.Code != 503 {
		t.Fatal("preparation ACK failure hidden")
	}
	for i := 0; i < 3; i++ {
		r := httptest.NewRequest("GET", "http://"+actionHost+"/api/outbox", nil)
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"prepared"`) {
			t.Fatalf("outbox read: %s", w.Body.String())
		}
	}
	snap, err := st.Snapshot()
	if err != nil || len(snap.Requests) != 0 || len(snap.Attempts) != 0 || submitted != 0 {
		t.Fatal("outbox read or preparation triggered dispatch")
	}
}
