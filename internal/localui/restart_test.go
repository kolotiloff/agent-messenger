package localui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
)

func TestClientOutdatedNeverOffersADowngrade(t *testing.T) {
	for _, tc := range []struct {
		client, window string
		want           bool
	}{
		{"", "0.1.0-preview.27", true},
		{"", "dev", true},
		{"0.1.0-preview.26", "0.1.0-preview.27", true},
		{"0.1.0-preview.9", "0.1.0-preview.10", true},
		{"0.1.0-preview.27", "0.2.0-preview.1", true},
		{"0.1.0-preview.27", "0.1.0-preview.27", false},
		{"0.1.0-preview.28", "0.1.0-preview.27", false},
		{"0.2.0-preview.1", "0.1.0-preview.27", false},
		{"dev", "0.1.0-preview.27", false},
		{"0.1.0-preview.27", "dev", false},
		{"dev", "dev", false},
	} {
		if got := clientOutdated(tc.client, tc.window); got != tc.want {
			t.Errorf("clientOutdated(%q, %q) = %v", tc.client, tc.window, got)
		}
	}
}

// restartFake answers status and window-view like an older client.
func restartFake(version string, running int, view bool) caller {
	return func(_ context.Context, r control.Request) (control.Response, error) {
		switch r.Cmd {
		case "status":
			return control.Response{OK: true, Status: &control.Status{Version: version}}, nil
		case "window-view":
			if !view {
				return control.Response{Error: "неизвестная команда"}, nil
			}
			raw, _ := json.Marshal(map[string]any{"agents": []map[string]int{{"running": running}, {"running": 0}}})
			return control.Response{OK: true, Overview: raw}, nil
		}
		return control.Response{}, errors.New("unexpected command " + r.Cmd)
	}
}

func restartHandler(call caller, restart Restarter) *httptest.ResponseRecorder {
	return postAction(newHandlerWithChecks(actionHost, "secret", call, nil, nil, diagnostics.CheckCLI, restart), "restart-client", `{"project":"Game"}`)
}

func TestRestartClientRoute(t *testing.T) {
	var restarts atomic.Int32
	restart := func(context.Context) error { restarts.Add(1); return nil }

	// A browser window has no restarter.
	if w := restartHandler(restartFake("", 0, true), nil); w.Code != 404 || restarts.Load() != 0 {
		t.Fatalf("browser window restarted: %d", w.Code)
	}
	// The same version is not restarted.
	if w := restartHandler(restartFake(buildinfo.Version, 0, true), restart); w.Code != 409 || !strings.Contains(w.Body.String(), `"current"`) || restarts.Load() != 0 {
		t.Fatalf("current client restarted: %d %s", w.Code, w.Body)
	}
	// Running tasks, or a client that cannot say, need the owner's confirmation.
	for _, call := range []caller{restartFake("", 2, true), restartFake("", 0, false)} {
		w := restartHandler(call, restart)
		if w.Code != 409 || !strings.Contains(w.Body.String(), `"confirm"`) || restarts.Load() != 0 {
			t.Fatalf("restarted without confirmation: %d %s", w.Code, w.Body)
		}
	}
	h := newHandlerWithChecks(actionHost, "secret", restartFake("", 2, true), nil, nil, diagnostics.CheckCLI, restart)
	if w := postAction(h, "restart-client", `{"project":"Game","confirm":true}`); w.Code != 200 || restarts.Load() != 1 {
		t.Fatalf("confirmed restart: %d %s", w.Code, w.Body)
	}
	// An idle old client is restarted at once.
	if w := restartHandler(restartFake("", 0, true), restart); w.Code != 200 || restarts.Load() != 2 {
		t.Fatalf("idle restart: %d %s", w.Code, w.Body)
	}
	// A failed restart says so and is not repeated.
	failing := func(context.Context) error {
		restarts.Add(1)
		return errors.New("новый не запустился")
	}
	if w := restartHandler(restartFake("", 0, true), failing); w.Code != 502 || !strings.Contains(w.Body.String(), "новый не запустился") || restarts.Load() != 3 {
		t.Fatalf("failed restart: %d %s", w.Code, w.Body)
	}
	// Extra fields are refused before anything is asked of the client.
	if w := postAction(h, "restart-client", `{"project":"Game","force":true}`); w.Code != 400 {
		t.Fatalf("extra field: %d", w.Code)
	}
}

func TestRestartClientRouteRunsOneAtATime(t *testing.T) {
	entered, release := make(chan struct{}), make(chan struct{})
	h := newHandlerWithChecks(actionHost, "secret", restartFake("", 0, true), nil, nil, diagnostics.CheckCLI, func(context.Context) error {
		close(entered)
		<-release
		return nil
	})
	done := make(chan int)
	go func() { done <- postAction(h, "restart-client", `{"project":""}`).Code }()
	<-entered
	if w := postAction(h, "restart-client", `{"project":""}`); w.Code != 409 || !strings.Contains(w.Body.String(), `"busy"`) {
		t.Fatalf("second restart: %d %s", w.Code, w.Body)
	}
	close(release)
	if code := <-done; code != 200 {
		t.Fatalf("first restart: %d", code)
	}
}

func TestClientInfoReportsVersions(t *testing.T) {
	for _, tc := range []struct {
		call    caller
		restart Restarter
		want    string
	}{
		{restartFake("", 0, true), func(context.Context) error { return nil }, `"outdated":true`},
		{restartFake(buildinfo.Version, 0, true), nil, `"outdated":false`},
		{func(context.Context, control.Request) (control.Response, error) {
			return control.Response{}, errors.New("down")
		}, nil, `"client":"unavailable"`},
	} {
		h := newHandlerWithChecks(actionHost, "secret", tc.call, nil, nil, diagnostics.CheckCLI, tc.restart)
		r := httptest.NewRequest("GET", "http://"+actionHost+"/api/client-info", nil)
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), tc.want) || !strings.Contains(w.Body.String(), `"window_version"`) {
			t.Fatalf("client-info: %d %s", w.Code, w.Body)
		}
		if strings.Contains(w.Body.String(), `"restart":true`) != (tc.restart != nil) {
			t.Fatalf("restart flag: %s", w.Body)
		}
	}
}

// T-123: the restart question lists the running tasks of this computer; a
// task this computer sent elsewhere or one still waiting to start is left out.
func TestRestartQuestionListsRunningTasks(t *testing.T) {
	call := func(_ context.Context, r control.Request) (control.Response, error) {
		switch r.Cmd {
		case "status":
			return control.Response{OK: true, Status: &control.Status{Version: ""}}, nil
		case "window-view":
			raw, _ := json.Marshal(map[string]any{
				"agents": []map[string]int{{"running": 3}},
				"tasks": []map[string]any{
					{"agent": "codex", "title": "Build menu", "state": "running", "started_at": "2026-11-09T20:00:00Z"},
					{"agent": "cursor", "title": "From a peer", "state": "running", "remote": map[string]any{"name": "PC", "from": true}},
					{"agent": "claude", "title": "Sent elsewhere", "state": "running", "remote": map[string]any{"name": "PC"}},
					{"agent": "claude", "title": "Not started", "state": "running", "pending_launch": true},
					{"agent": "claude", "title": "Done", "state": "done"},
				},
			})
			return control.Response{OK: true, Overview: raw}, nil
		}
		return control.Response{}, errors.New("unexpected command " + r.Cmd)
	}
	w := restartHandler(call, func(context.Context) error { t.Fatal("restarted"); return nil })
	var got struct {
		State string        `json:"state"`
		Tasks []runningTask `json:"tasks"`
	}
	if w.Code != 409 || json.Unmarshal(w.Body.Bytes(), &got) != nil || got.State != "confirm" {
		t.Fatalf("answer %d %s", w.Code, w.Body)
	}
	if len(got.Tasks) != 2 || got.Tasks[0].Title != "Build menu" || got.Tasks[1].Title != "From a peer" {
		t.Fatalf("tasks %+v", got.Tasks)
	}
}
