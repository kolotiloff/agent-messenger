package localui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
)

type fakeSelfUpdater struct {
	mu      sync.Mutex
	state   selfupdate.Status
	checks  int
	applies int
	fail    bool
}

func (f *fakeSelfUpdater) Status() selfupdate.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

func (f *fakeSelfUpdater) Check(context.Context) selfupdate.Status {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checks++
	f.state = selfupdate.Status{State: "available", Version: "0.0.0-preview.34"}
	return f.state
}

func (f *fakeSelfUpdater) Apply(context.Context) (selfupdate.Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applies++
	if f.fail {
		f.state = selfupdate.Status{State: "available", Version: "0.0.0-preview.34"}
		return f.state, errors.New("signature mismatch")
	}
	f.state = selfupdate.Status{State: "installed", Version: "0.0.0-preview.34"}
	return f.state, nil
}

func selfUpdateHandler(u *SelfUpdate) func(method, body string) *httptest.ResponseRecorder {
	h := newDesktopHandler(actionHost, "secret", nil, nil, diagnostics.CheckCLI, Desktop{SelfUpdate: u})
	return func(method, body string) *httptest.ResponseRecorder {
		if method == "GET" {
			r := httptest.NewRequest("GET", "http://"+actionHost+"/api/self-update", nil)
			r.Header.Set("Authorization", "Bearer secret")
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			return w
		}
		return postAction(h, "self-update", body)
	}
}

func TestSelfUpdateIsOffWithoutAnUpdater(t *testing.T) {
	call := selfUpdateHandler(nil)
	if w := call("GET", ""); w.Code != 404 {
		t.Fatalf("status %d", w.Code)
	}
	if w := call("POST", `{"action":"install"}`); w.Code != 404 {
		t.Fatalf("install %d", w.Code)
	}
}

func TestSelfUpdateCheckInstallAndRelaunch(t *testing.T) {
	fake := &fakeSelfUpdater{state: selfupdate.Status{State: "idle"}}
	relaunched := make(chan struct{}, 1)
	call := selfUpdateHandler(&SelfUpdate{Updater: fake, Relaunch: func() { relaunched <- struct{}{} }})
	if w := call("POST", `{"action":"other"}`); w.Code != 400 {
		t.Fatalf("unknown action %d", w.Code)
	}
	if w := call("POST", `{"action":"check","extra":1}`); w.Code != 400 {
		t.Fatalf("extra field %d", w.Code)
	}
	if w := call("POST", `{"action":"check"}`); w.Code != 200 {
		t.Fatalf("check %d %s", w.Code, w.Body)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		var s map[string]any
		w := call("GET", "")
		_ = json.Unmarshal(w.Body.Bytes(), &s)
		if s["state"] == "available" && s["version"] == "0.0.0-preview.34" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no update shown: %s", w.Body)
		}
		time.Sleep(10 * time.Millisecond)
	}
	fake.fail = true
	if w := call("POST", `{"action":"install"}`); w.Code != 409 {
		t.Fatalf("failed install %d %s", w.Code, w.Body)
	}
	select {
	case <-relaunched:
		t.Fatal("relaunched after a failed install")
	case <-time.After(1500 * time.Millisecond):
	}
	fake.fail = false
	if w := call("POST", `{"action":"install"}`); w.Code != 200 {
		t.Fatalf("install %d %s", w.Code, w.Body)
	}
	select {
	case <-relaunched:
	case <-time.After(5 * time.Second):
		t.Fatal("not relaunched")
	}
}
