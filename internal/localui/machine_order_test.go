package localui

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestComputerOrderSurvivesWindowRestart(t *testing.T) {
	root := t.TempDir()
	window := func(host string) http.Handler {
		return newDesktopHandler(host, "secret", nil, nil, nil, Desktop{setupRoot: root})
	}
	read := func(h http.Handler, host, want string) {
		t.Helper()
		r := httptest.NewRequest(http.MethodGet, "http://"+host+machineOrderRoute, nil)
		r.Header.Set("Authorization", "Bearer secret")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != http.StatusOK || w.Body.String() != want {
			t.Fatalf("read order: %d %q", w.Code, w.Body.String())
		}
	}
	first := window(actionHost)
	read(first, actionHost, `{"order":null}`)
	order := `{"order":["dev-a","local","computer-1"],"peers":{"host":"computer-1"}}`
	if w := postAction(first, "machine-order", order); w.Code != http.StatusOK {
		t.Fatalf("write order: %d %q", w.Code, w.Body.String())
	}
	if w := postAction(first, "machine-order", `{"order":["local","local"]}`); w.Code != http.StatusBadRequest {
		t.Fatalf("duplicate order entries accepted: %d", w.Code)
	}
	if w := postAction(first, "machine-order", `{"order":["local"],"peers":{"host":"bad id"}}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad peer alias accepted: %d", w.Code)
	}
	const newHost = "127.0.0.1:4321"
	read(window(newHost), newHost, order)
}
