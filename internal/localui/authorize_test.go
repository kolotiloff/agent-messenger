package localui

import (
	"context"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWindowAuthorizationRequiresExplicitFixedEnvelope(t *testing.T) {
	var got []control.Request
	h := newHandler(actionHost, "secret", func(_ context.Context, r control.Request) (control.Response, error) {
		got = append(got, r)
		return control.Response{OK: true}, nil
	}, nil)
	const body = `{"alias":"chosen","confirmation":"allow-model-calls"}`
	for _, bad := range []string{`null`, `{}`, `{"alias":"chosen"}`, `{"alias":"chosen","confirmation":"yes"}`, body + ` {}`, strings.TrimSuffix(body, `}`) + `,"cli":"/bad"}`} {
		if w := postAction(h, "authorize-setup", bad); w.Code != 400 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, origin := range []string{"", "https://foreign.invalid"} {
		r := httptest.NewRequest("POST", "http://"+actionHost+"/api/authorize-setup", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer secret")
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
	if len(got) != 0 {
		t.Fatal("invalid envelope authorized a session")
	}
	if w := postAction(h, "authorize-setup", body); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	if len(got) != 1 || got[0] != (control.Request{Cmd: "authorize-setup", Alias: "chosen", Confirmation: "allow-model-calls"}) {
		t.Fatal(got)
	}
}
