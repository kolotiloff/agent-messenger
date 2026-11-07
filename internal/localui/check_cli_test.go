package localui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
)

const diagnosticBody = `{"provider":"claude","cli":"/trusted/claude","environment":"/profiles/test"}`

func TestCLICheckRequiresExplicitTrustedEnvelope(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX program paths")
	}
	var starts atomic.Int32
	h := newHandlerWithChecks(actionHost, "secret", func(context.Context, control.Request) (control.Response, error) {
		t.Error("diagnostic reached supervisor")
		return control.Response{}, nil
	}, func(context.Context) any { return struct{}{} }, nil,
		func(ctx context.Context, provider, cli, profile string) (diagnostics.CLICheck, error) {
			starts.Add(1)
			if provider != "claude" || cli != "/trusted/claude" || profile != "/profiles/test" {
				t.Fatal("changed selection")
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("unbounded diagnostic")
			}
			return diagnostics.CLICheck{Provider: provider, Version: diagnostics.CheckState{State: "unknown"}}, nil
		}, nil)
	for _, tc := range []struct {
		method, origin, token, body string
		code                        int
	}{
		{"GET", "http://" + actionHost, "secret", diagnosticBody, 405},
		{"POST", "", "secret", diagnosticBody, 403},
		{"POST", "https://other.example", "secret", diagnosticBody, 403},
		{"POST", "http://" + actionHost, "", diagnosticBody, 401},
		{"POST", "http://" + actionHost, "secret", `null`, 400},
		{"POST", "http://" + actionHost, "secret", diagnosticBody + ` {}`, 400},
		{"POST", "http://" + actionHost, "secret", strings.Replace(diagnosticBody, `"claude"`, `"other"`, 1), 400},
		{"POST", "http://" + actionHost, "secret", strings.Replace(diagnosticBody, `/trusted/claude`, `claude`, 1), 400},
		{"POST", "http://" + actionHost, "secret", strings.TrimSuffix(diagnosticBody, `}`) + `,"args":["-p","hello"]}`, 400},
	} {
		r := httptest.NewRequest(tc.method, "http://"+actionHost+"/api/check-cli", strings.NewReader(tc.body))
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Authorization", "Bearer "+tc.token)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.code {
			t.Fatalf("%+v: %d", tc, w.Code)
		}
	}
	if starts.Load() != 0 {
		t.Fatal("untrusted request executed a diagnostic")
	}
	if w := postAction(h, "check-cli", diagnosticBody); w.Code != 200 || !strings.Contains(w.Body.String(), `"state":"unknown"`) {
		t.Fatal(w.Code, w.Body.String())
	}
	if starts.Load() != 1 {
		t.Fatal("expected exactly one explicit diagnostic")
	}
}

func TestCLICheckSerializesAndRedactsFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX program paths")
	}
	entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
	h := newHandlerWithChecks(actionHost, "secret", nil, nil, nil, func(context.Context, string, string, string) (diagnostics.CLICheck, error) {
		close(entered)
		<-release
		return diagnostics.CLICheck{}, errors.New("PRIVATE_TOKEN")
	}, nil)
	go func() {
		defer close(done)
		w := postAction(h, "check-cli", diagnosticBody)
		if w.Code != 422 || strings.Contains(w.Body.String(), "PRIVATE_TOKEN") {
			t.Error("failure leaked or accepted", w.Code, w.Body.String())
		}
	}()
	<-entered
	w := postAction(h, "check-cli", diagnosticBody)
	close(release)
	<-done
	if w.Code != http.StatusConflict {
		t.Fatal("parallel probe accepted", w.Code)
	}
}
