package localui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

func TestOnboardingRoutesReuseProjectCreationAndPersistPreferences(t *testing.T) {
	root := t.TempDir()
	if err := platform.MarkFreshWindowSetup(root, filepath.Join(root, "queue.sqlite")); err != nil {
		t.Fatal(err)
	}
	created := false
	handler := newDesktopHandler(actionHost, "secret", func(_ context.Context, req control.Request) (control.Response, error) {
		created = true
		if req.Cmd != "project-create" || req.Alias == "" || req.Workspace != filepath.Join(root, "local-chat") {
			t.Fatalf("unexpected project request: %#v", req)
		}
		if fi, err := os.Stat(req.Workspace); err != nil || !fi.IsDir() {
			t.Fatalf("workspace was not prepared: %v", err)
		}
		return control.Response{OK: true}, nil
	}, nil, nil, Desktop{setupRoot: root})
	const model = "codex:gpt-6.2-sol[effort=medium]"
	for _, body := range []string{
		`{"action":"default-model","model":"` + model + `"}`,
		`{"action":"direct-chat"}`,
		`{"action":"dismiss"}`,
	} {
		if got := postAction(handler, "onboarding", body); got.Code != http.StatusOK {
			t.Fatalf("rejected %s: %d %s", body, got.Code, got.Body)
		}
	}
	if !created {
		t.Fatal("direct chat did not ask the client to create a workspace")
	}
	request := httptest.NewRequest(http.MethodGet, "http://"+actionHost+onboardingRoute, nil)
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var state struct {
		Model   string
		Pending bool
	}
	if err := json.Unmarshal(response.Body.Bytes(), &state); err != nil || response.Code != http.StatusOK || state.Pending || state.Model != model {
		t.Fatalf("unexpected saved setup preference: %d %s (%v)", response.Code, response.Body, err)
	}
}
