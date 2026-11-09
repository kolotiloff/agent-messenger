package localui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

const actionHost = "127.0.0.1:1234"

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
		{"missing origin", "pause", `{}`, "", "secret", "application/json", 403},
		{"foreign origin", "pause", `{}`, "https://evil.example", "secret", "application/json", 403},
		{"missing key", "pause", `{}`, "http://" + actionHost, "", "application/json", 401},
		{"form", "pause", `{}`, "http://" + actionHost, "secret", "text/plain", 415},
		{"extra command", "pause", `{"cmd":"shutdown"}`, "http://" + actionHost, "secret", "application/json", 400},
		{"two objects", "resume", `{} {}`, "http://" + actionHost, "secret", "application/json", 400},
		{"null command", "pause", `null`, "http://" + actionHost, "secret", "application/json", 400},
		{"oversize", "resume", `{"x":"` + strings.Repeat("x", 70000) + `"}`, "http://" + actionHost, "secret", "application/json", 400},
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

func TestExplicitActionsOnly(t *testing.T) {
	var got control.Request
	calls := 0
	h := newHandler(actionHost, "secret", func(ctx context.Context, r control.Request) (control.Response, error) {
		got = r
		calls++
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("no deadline")
		}
		return control.Response{OK: true}, nil
	}, nil)
	for _, cmd := range []string{"pause", "resume"} {
		w := postAction(h, cmd, "{}")
		if w.Code != 200 || got.Cmd != cmd {
			t.Fatalf("%s: %d %+v", cmd, w.Code, got)
		}
		if got.CLI != "" || got.Environment != "" || got.ConversationID != "" || got.Confirmation != "" {
			t.Fatal("unexpected control authority")
		}
	}
	// The removed mode's window routes (T-028) are gone: a write is refused
	// as a wrong method and never reaches the client.
	calls = 0
	for _, path := range []string{"enqueue", "outbox/ack", "setup-claude", "setup-codex", "authorize-setup", "shutdown"} {
		if w := postAction(h, path, "{}"); w.Code != http.StatusMethodNotAllowed {
			t.Fatalf("%s: %d", path, w.Code)
		}
	}
	if calls != 0 {
		t.Fatal("a removed route reached the client")
	}
}
