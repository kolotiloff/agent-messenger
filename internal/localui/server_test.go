package localui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

func TestUIRejectsUntrustedAccessAndMutations(t *testing.T) {
	calls := 0
	h := newHandler("127.0.0.1:1234", "secret", func(context.Context, control.Request) (control.Response, error) {
		calls++
		return control.Response{OK: true}, nil
	}, func(context.Context) any { calls++; return nil })
	for _, test := range []struct {
		method, path, host, origin, site, token string
		code                                    int
	}{
		{"GET", "/api/overview", "127.0.0.1:1234", "", "", "", 401},
		{"GET", "/api/outbox", "127.0.0.1:1234", "", "", "", 401},
		{"GET", "/api/outbox/ack", "127.0.0.1:1234", "", "", "secret", 405},
		{"GET", "/api/overview", "evil.example:1234", "", "", "secret", 403},
		{"GET", "/api/overview", "127.0.0.1:1234", "https://evil.example", "", "secret", 403},
		{"GET", "/api/overview", "127.0.0.1:1234", "", "cross-site", "secret", 403},
		{"POST", "/api/overview", "127.0.0.1:1234", "", "", "secret", 405},
		{"GET", "/api/shutdown", "127.0.0.1:1234", "", "", "secret", 404},
		{"GET", "/api/enqueue", "127.0.0.1:1234", "", "", "secret", 405},
		{"GET", "/api/inspect", "127.0.0.1:1234", "", "", "secret", 404},
		{"GET", "/api/timeline?alias=a&limit=101", "127.0.0.1:1234", "", "", "secret", 400},
		{"GET", "/api/timeline", "127.0.0.1:1234", "", "", "secret", 400},
		{"GET", "/../../go.mod", "127.0.0.1:1234", "", "", "secret", 404},
	} {
		r := httptest.NewRequest(test.method, "http://"+test.host+test.path, nil)
		r.Header.Set("Origin", test.origin)
		r.Header.Set("Sec-Fetch-Site", test.site)
		if test.token != "" {
			r.Header.Set("Authorization", "Bearer "+test.token)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != test.code {
			t.Fatalf("%s %s: %d", test.method, test.path, w.Code)
		}
		if w.Header().Get("Access-Control-Allow-Origin") != "" {
			t.Fatal("CORS enabled")
		}
	}
	if calls != 0 {
		t.Fatal("rejected request reached control API")
	}
}

func TestUIAllowsOnlyCrossSiteStartPageNavigation(t *testing.T) {
	h := newHandler("127.0.0.1:1234", "secret", func(context.Context, control.Request) (control.Response, error) {
		t.Fatal("control API reached")
		return control.Response{}, nil
	}, func(context.Context) any { return nil })
	for _, test := range []struct {
		method, path, mode, dest string
		code                     int
	}{
		{"GET", "/", "navigate", "document", 200},
		{"GET", "/", "navigate", "iframe", 403},
		{"GET", "/", "no-cors", "script", 403},
		{"GET", "/?x=1", "navigate", "document", 403},
		{"GET", "/app.js", "navigate", "document", 403},
		{"GET", "/api/overview", "navigate", "document", 403},
	} {
		r := httptest.NewRequest(test.method, "http://127.0.0.1:1234"+test.path, nil)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		r.Header.Set("Sec-Fetch-Mode", test.mode)
		r.Header.Set("Sec-Fetch-Dest", test.dest)
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != test.code {
			t.Fatalf("%s %s %s/%s: %d", test.method, test.path, test.mode, test.dest, w.Code)
		}
	}
}

func TestUIOnlyForwardsReadCommandsAndRedactsStatus(t *testing.T) {
	var got control.Request
	h := newHandler("127.0.0.1:1234", "secret", func(ctx context.Context, r control.Request) (control.Response, error) {
		got = r
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > 3*time.Second {
			t.Fatal("missing deadline")
		}
		return control.Response{OK: true, Status: &control.Status{Detail: "private diagnostic", Queued: 1}, Overview: json.RawMessage(`{"schema_version":1,"sessions":[]}`), Timeline: json.RawMessage(`{"schema_version":1,"items":[]}`)}, nil
	}, func(context.Context) any { return map[string]string{"os": "darwin"} })
	for _, path := range []string{"/api/status", "/api/overview", "/api/timeline?alias=worker&before=m1&limit=5", "/api/discovery"} {
		r := httptest.NewRequest("GET", "http://127.0.0.1:1234"+path, nil)
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || !json.Valid(w.Body.Bytes()) || strings.Contains(w.Body.String(), "private diagnostic") {
			t.Fatalf("bad response %d %s", w.Code, w.Body.String())
		}
		if path == "/api/timeline?alias=worker&before=m1&limit=5" && (got.Cmd != "timeline" || got.Alias != "worker" || got.Before != "m1" || got.Limit != 5) {
			t.Fatalf("route changed %+v", got)
		}
		if w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") {
			t.Fatal("missing private response headers")
		}
	}
}

func TestUIUnavailableIsNotEmptySuccess(t *testing.T) {
	for _, response := range []control.Response{{OK: false, Error: "private-path"}, {OK: true}, {OK: true, Overview: json.RawMessage(`invalid`)}} {
		h := newHandler("127.0.0.1:1234", "secret", func(context.Context, control.Request) (control.Response, error) { return response, nil }, nil)
		r := httptest.NewRequest("GET", "http://127.0.0.1:1234/api/overview", nil)
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code == 200 || strings.Contains(w.Body.String(), "private-path") {
			t.Fatal("failure disguised or leaked")
		}
	}
	h := newHandler("127.0.0.1:1234", "secret", func(context.Context, control.Request) (control.Response, error) {
		return control.Response{}, errors.New("private-path")
	}, nil)
	r := httptest.NewRequest("GET", "http://127.0.0.1:1234/api/status", nil)
	r.Header.Set("Authorization", "Bearer secret")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 503 || strings.Contains(w.Body.String(), "private-path") {
		t.Fatal("transport error leaked")
	}
}

func TestUIAssetsEmbeddedWithoutPrivateCapability(t *testing.T) {
	h := newHandler("127.0.0.1:1234", "secret-token", nil, nil)
	for _, path := range []string{"/", "/app.js", "/i18n.js", "/app.css", "/design-v1.css"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", "http://127.0.0.1:1234"+path, nil))
		if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "secret-token") {
			t.Fatalf("asset %s invalid", path)
		}
	}
}

func TestUIReadFailureNamesItsKind(t *testing.T) {
	for _, c := range []struct {
		response control.Response
		err      error
		code     int
		kind     string
	}{
		{err: errors.New("private-path"), code: 503, kind: "client_unavailable"},
		{response: control.Response{OK: false, Error: "неизвестная команда"}, code: 502, kind: "client_outdated"},
		{response: control.Response{OK: false, Error: "private-path"}, code: 502, kind: "client_rejected"},
	} {
		h := newHandler("127.0.0.1:1234", "secret", func(context.Context, control.Request) (control.Response, error) { return c.response, c.err }, nil)
		r := httptest.NewRequest("GET", "http://127.0.0.1:1234/api/window?project=Game", nil)
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.code || !strings.Contains(w.Body.String(), c.kind) || strings.Contains(w.Body.String(), "private-path") {
			t.Fatalf("%+v: %d %s", c, w.Code, w.Body.String())
		}
	}
}
