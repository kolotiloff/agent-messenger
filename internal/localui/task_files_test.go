package localui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

func TestTaskFileRoute(t *testing.T) {
	host, token := "127.0.0.1:1234", "fixture"
	var calls []control.Request
	handler := newHandler(host, token, func(_ context.Context, req control.Request) (control.Response, error) {
		calls = append(calls, req)
		return control.Response{OK: true, Overview: json.RawMessage(`{"name":"PLAN.md","format":"markdown","text":"# Plan"}`)}, nil
	}, nil)
	input := `{"task":"task-1","path":"Docs/PLAN.md","action":"preview"}`
	for _, tc := range []struct {
		method, auth, origin, input string
		want                        int
	}{
		{"GET", token, "", input, 405},
		{"POST", "", "", input, 401},
		{"POST", token, "https://other.invalid", input, 403},
		{"POST", token, "", `{"task":"task-1","path":"PLAN.md","action":"open","computer":"local"}`, 400},
		{"POST", token, "", input + `{"extra":1}`, 400},
		{"POST", token, "", input, 200},
	} {
		req := httptest.NewRequest(tc.method, "http://"+host+taskFileRoute, strings.NewReader(tc.input))
		req.Header.Set("Origin", tc.origin)
		if tc.auth != "" {
			req.Header.Set("Authorization", "Bearer "+tc.auth)
		}
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		if out.Code != tc.want {
			t.Errorf("%s %s: status %d, want %d", tc.method, tc.input, out.Code, tc.want)
		}
		if tc.want == http.StatusOK && !strings.Contains(out.Body.String(), `"text":"# Plan"`) {
			t.Errorf("bad response %q", out.Body.String())
		}
	}
	if len(calls) != 1 || calls[0].Cmd != "task-file" || calls[0].MessageID != "task-1" || calls[0].Workspace != "Docs/PLAN.md" || calls[0].Action != "preview" {
		t.Fatalf("unexpected file calls %+v", calls)
	}
}
