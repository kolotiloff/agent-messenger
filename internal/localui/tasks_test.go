package localui

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

func TestTaskWindowRoutes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("fixture uses POSIX folder and program paths")
	}
	var got []control.Request
	reject, outdated := false, false
	h := newHandler("127.0.0.1:1234", "secret", func(_ context.Context, r control.Request) (control.Response, error) {
		got = append(got, r)
		if outdated {
			return control.Response{Error: "неизвестная команда"}, nil
		}
		if reject {
			return control.Response{Error: "this question is no longer open"}, nil
		}
		return control.Response{OK: true, Overview: json.RawMessage(`{"projects":[]}`)}, nil
	}, func(context.Context) any { return nil })
	do := func(method, path, body string) (int, string) {
		var r = httptest.NewRequest(method, "http://127.0.0.1:1234"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer secret")
		if method == "POST" {
			r.Header.Set("Origin", "http://127.0.0.1:1234")
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	if code, _ := do("GET", "/api/window?project=Game", ""); code != 200 || got[0].Cmd != "window-view" || got[0].Alias != "Game" {
		t.Fatalf("window %d %+v", code, got)
	}
	if code, _ := do("GET", "/api/task?id=task-1", ""); code != 200 || got[1].Cmd != "task-feed" || got[1].MessageID != "task-1" {
		t.Fatalf("task %d %+v", code, got)
	}
	if code, _ := do("GET", "/api/task", ""); code != 400 {
		t.Fatalf("task without id %d", code)
	}
	code, _ := do("POST", "/api/task-answer", `{"task":"task-1","question":"q-1","op_id":"op-1","text":"yes"}`)
	last := got[len(got)-1]
	if code != 200 || last.Cmd != "task-owner-answer" || last.MessageID != "task-1" || last.ConversationID != "q-1" || last.Confirmation != "op-1" || last.Text != "yes" {
		t.Fatalf("answer %d %+v", code, last)
	}
	// A client older than the window does not know the command: the window
	// is told so instead of showing the client's raw error.
	outdated = true
	if code, body := do("POST", "/api/task-cancel", `{"task":"task-1"}`); code != 502 || !strings.Contains(body, `"client_outdated"`) || strings.Contains(body, "неизвестная") {
		t.Fatalf("outdated client %d %s", code, body)
	}
	outdated = false
	if code, _ := do("POST", "/api/task-cancel", `{"task":"task-1"}`); code != 200 || got[len(got)-1].Cmd != "task-cancel" || got[len(got)-1].MessageID != "task-1" {
		t.Fatalf("cancel %d %+v", code, got[len(got)-1])
	}
	before := len(got)
	for _, bad := range []struct{ path, body string }{
		{"/api/task-answer", `{"task":"task-1","question":"q-1","text":"yes"}`},
		{"/api/task-answer", `{"task":"task-1","question":"q-1","op_id":"o","text":"yes","extra":1}`},
		{"/api/project-create", `{"name":"a/b","folder":"/x"}`},
		{"/api/project-create", `{"name":"Game","folder":"relative"}`},
		{"/api/agent-set", `{"provider":"gemini","cli":"/bin/x"}`},
		{"/api/task-cancel", `{"task":""}`},
		{"/api/coordinator-connect", `{"project":"a/b"}`},
		{"/api/coordinator-connect", `{"project":"Game","cli":"/bin/sh"}`},
	} {
		if code, _ := do("POST", bad.path, bad.body); code != 400 {
			t.Fatalf("%s %s accepted: %d", bad.path, bad.body, code)
		}
	}
	if len(got) != before {
		t.Fatal("invalid request reached the client")
	}
	if code, _ := do("POST", "/api/project-create", `{"name":"Моя игра","folder":"/Users/me/Game"}`); code != 200 || got[len(got)-1].Cmd != "project-create" || got[len(got)-1].Workspace != "/Users/me/Game" {
		t.Fatalf("create %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/coordinator-connect", `{"project":"Моя игра"}`); code != 200 || got[len(got)-1].Cmd != "coordinator-connect" || got[len(got)-1].Alias != "Моя игра" {
		t.Fatalf("coordinator connect %d %+v", code, got[len(got)-1])
	}
	reject = true
	if code, body := do("POST", "/api/task-answer", `{"task":"task-1","question":"q-1","op_id":"op-2","text":"late"}`); code != 409 || !strings.Contains(body, "no longer open") {
		t.Fatalf("rejected answer %d %s", code, body)
	}
}

func TestServerRoutes(t *testing.T) {
	var got []control.Request
	h := newHandler("127.0.0.1:1234", "secret", func(_ context.Context, r control.Request) (control.Response, error) {
		got = append(got, r)
		return control.Response{OK: true, Overview: json.RawMessage(`{"host":{"running":true}}`)}, nil
	}, func(context.Context) any { return nil })
	do := func(method, path, body string) (int, string) {
		r := httptest.NewRequest(method, "http://127.0.0.1:1234"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer secret")
		if method == "POST" {
			r.Header.Set("Origin", "http://127.0.0.1:1234")
			r.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	if code, body := do("GET", "/api/server", ""); code != 200 || got[0].Cmd != "server" || got[0].Action != "status" || !strings.Contains(body, `"running":true`) {
		t.Fatalf("status %d %s %+v", code, body, got)
	}
	code, body := do("POST", "/api/server-action", `{"action":"enable","scope":"lan","port":47100}`)
	last := got[len(got)-1]
	if code != 200 || last.Cmd != "server" || last.Action != "enable" || last.Text != `{"scope":"lan","port":47100}` || !strings.Contains(body, `"result"`) {
		t.Fatalf("enable %d %s %+v", code, body, last)
	}
	code, _ = do("POST", "/api/server-action", `{"action":"accept","on":true}`)
	if last := got[len(got)-1]; code != 200 || last.Action != "accept" || last.Text != `{"on":true}` {
		t.Fatalf("accept %d %+v", code, last)
	}
	for _, bad := range []string{`{"action":"delete-everything"}`, `{"action":"connect","code":"` + strings.Repeat("x", 3000) + `"}`, `{"action":"enable","extra":1}`, `{"action":"enable","port":70000}`} {
		if code, _ := do("POST", "/api/server-action", bad); code != 400 {
			t.Errorf("accepted %s: %d", bad[:min(len(bad), 60)], code)
		}
	}
}
