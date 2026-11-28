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
	if code, _ := do("GET", "/api/window?project=Game&agent=codex&computer=local&history_version=fixture", ""); code != 200 || got[len(got)-1].Confirmation != "fixture" || got[len(got)-1].Provider != "codex" || got[len(got)-1].Text != "local" {
		t.Fatalf("window history filter %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("GET", "/api/task-history?project=Game&before=task-old&agent=codex&computer=local", ""); code != 200 || got[len(got)-1].Cmd != "task-history" || got[len(got)-1].Before != "task-old" {
		t.Fatalf("older history %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("GET", "/api/task-history?project=Game", ""); code != 400 {
		t.Fatalf("missing history cursor %d", code)
	}
	if code, body := do("GET", "/api/project-activity?project=Main%20Game", ""); code != 200 ||
		got[len(got)-1].Cmd != "project-activity" || got[len(got)-1].Project != "Main Game" || body == "" {
		t.Fatalf("project activity %d %q %+v", code, body, got[len(got)-1])
	}
	if code, _ := do("GET", "/api/project-activity?project=Game%2FSite", ""); code != 400 {
		t.Fatalf("invalid activity project: %d", code)
	}
	if code, _ := do("GET", "/api/project-activity?project=Game&history=1&agent=codex", ""); code != 200 ||
		got[len(got)-1].Action != "1" || got[len(got)-1].Provider != "codex" {
		t.Fatalf("activity history filter %d %+v", code, got)
	}
	if code, _ := do("POST", "/api/task-hide-history", `{"project":"Game","agent":"codex","computer":"local"}`); code != 200 || got[len(got)-1].Cmd != "task-hide-history" || got[len(got)-1].Alias != "Game" {
		t.Fatalf("hide filtered history %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/task-archive", `{"task":"task-1","action":"restore","version":2}`); code != 200 || got[len(got)-1].Cmd != "task-archive" || got[len(got)-1].MessageID != "task-1" || got[len(got)-1].Action != "restore" || got[len(got)-1].ArchiveVersion != 2 {
		t.Fatalf("archive action: %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/task-archive", `{"task":"task-1","action":"erase","version":2}`); code != 400 {
		t.Fatalf("unknown archive action: %d", code)
	}
	if code, _ := do("GET", "/api/usage-stats?period=year", ""); code != 400 {
		t.Fatalf("bad period %d", code)
	}
	if code, _ := do("GET", "/api/usage-stats?period=week", ""); code != 200 || got[len(got)-1].Cmd != "usage-stats" || got[len(got)-1].Alias != "week" || got[len(got)-1].Text != "" {
		t.Fatalf("usage stats %d %+v", code, got)
	}
	if code, _ := do("GET", "/api/usage-stats?project=Main%20Game&period=month&computer=dev-1", ""); code != 200 || got[len(got)-1].Alias != "month" || got[len(got)-1].Project != "Main Game" || got[len(got)-1].Text != "dev-1" {
		t.Fatalf("usage stats of a computer %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("GET", "/api/usage-stats?period=week&project=Game%2FSite", ""); code != 400 {
		t.Fatalf("invalid statistics project: %d", code)
	}
	for _, path := range []string{"/api/usage-stats", "/api/usage-efficiency"} {
		if code, _ := do("GET", path+"?period=week&projects=Game&projects=Main%20Site&projects=Tools", ""); code != 200 ||
			strings.Join(got[len(got)-1].Projects, ",") != "Game,Main Site,Tools" {
			t.Fatalf("selected projects: %d %+v", code, got[len(got)-1])
		}
		for _, scope := range []string{"projects=Game", "projects=Game&projects=Site&projects=Game", "projects=Game&projects=Main%2FSite", "project=Other&projects=Game&projects=Site"} {
			if code, _ := do("GET", path+"?period=week&"+scope, ""); code != 400 {
				t.Fatalf("accepted invalid project scope %q: %d", scope, code)
			}
		}
	}
	if code, _ := do("GET", "/api/usage-stats?period=week&agent=codex&model=gpt-6.2-sol%5Beffort%3Dlow%5D", ""); code != 200 || got[len(got)-1].Provider != "codex" || got[len(got)-1].Model != "gpt-6.2-sol[effort=low]" {
		t.Fatalf("usage filter %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("GET", "/api/usage-stats?period=week&agent=other", ""); code != 400 {
		t.Fatalf("unknown agent %d", code)
	}
	if code, _ := do("GET", "/api/usage-efficiency?period=week", ""); code != 400 {
		t.Fatalf("model comparison without a project %d", code)
	}
	if code, _ := do("GET", "/api/usage-efficiency?project=Game&period=month&computer=local&agent=codex", ""); code != 200 ||
		got[len(got)-1].Cmd != "usage-efficiency" || got[len(got)-1].Project != "Game" || got[len(got)-1].Text != "local" {
		t.Fatalf("model comparison did not pass filters: %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("GET", "/api/usage-stats?period=week&computer="+strings.Repeat("x", 65), ""); code != 400 {
		t.Fatalf("long computer %d", code)
	}
	if code, _ := do("POST", "/api/usage-export", `{"period":"week"}`); code != 400 {
		t.Fatalf("export without project: %d", code)
	}
	if code, _ := do("POST", "/api/usage-export", `{"project":"Game","period":"all"}`); code != 400 {
		t.Fatalf("export with unknown period: %d", code)
	}
	if code, _ := do("POST", "/api/usage-export", `{"projects":["Game","Site"],"period":"week"}`); code != 200 ||
		strings.Join(got[len(got)-1].Projects, ",") != "Game,Site" {
		t.Fatalf("selected projects export: %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/usage-export", `{"projects":["Game","Game"],"period":"week"}`); code != 400 {
		t.Fatalf("export with repeated project: %d", code)
	}
	code, _ := do("POST", "/api/usage-export", `{"project":"Game","period":"week","computer":"dev-1","agent":"codex","model":"gpt-6.2-sol[effort=low]"}`)
	if req := got[len(got)-1]; code != 200 || req.Cmd != "usage-export" || req.Project != "Game" || req.Alias != "week" ||
		req.Text != "dev-1" || req.Provider != "codex" || req.Model != "gpt-6.2-sol[effort=low]" {
		t.Fatalf("export did not pass all the filters: %d %+v", code, req)
	}
	code, _ = do("POST", "/api/task-start", `{"project":"Game","computer":"dev-1","agent":"codex","model":"gpt-6.2-sol[effort=low]","op_id":"op-start","text":"Build the menu"}`)
	if req := got[len(got)-1]; code != 200 || req.Cmd != "task-owner-start" || req.Project != "Game" || req.Computer != "dev-1" ||
		req.Provider != "codex" || req.Model != "gpt-6.2-sol[effort=low]" || req.Confirmation != "op-start" || req.Text != "Build the menu" {
		t.Fatalf("start %d %+v", code, req)
	}
	if code, _ := do("POST", "/api/task-start", `{"project":"Game","agent":"codex","op_id":"op-start","text":""}`); code != 400 {
		t.Fatalf("start with empty task: %d", code)
	}
	code, _ = do("POST", "/api/task-answer", `{"task":"task-1","question":"q-1","op_id":"op-1","text":"yes"}`)
	last := got[len(got)-1]
	if code != 200 || last.Cmd != "task-owner-answer" || last.MessageID != "task-1" || last.ConversationID != "q-1" || last.Confirmation != "op-1" || last.Text != "yes" {
		t.Fatalf("answer %d %+v", code, last)
	}
	code, _ = do("POST", "/api/task-note", `{"task":"task-1","op_id":"op-n","text":"Use blue"}`)
	last = got[len(got)-1]
	if code != 200 || last.Cmd != "task-owner-note" || last.MessageID != "task-1" || last.Confirmation != "op-n" || last.Text != "Use blue" {
		t.Fatalf("note %d %+v", code, last)
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
		{"/api/task-note", `{"task":"task-1","text":"x"}`},
		{"/api/task-note", `{"task":"task-1","op_id":"o","text":"  "}`},
		{"/api/task-note", `{"task":"task-1","op_id":"o","text":"x","extra":1}`},
		{"/api/project-create", `{"name":"a/b","folder":"/x"}`},
		{"/api/project-create", `{"name":"Game","folder":"relative"}`},
		{"/api/agent-set", `{"provider":"gemini","cli":"/bin/x"}`},
		{"/api/agent-models", `{"provider":"codex","models":["--yolo"]}`},
		{"/api/agent-models", `{"provider":"codex","models":["a\nb"]}`},
		{"/api/agent-models", `{"provider":"gemini","models":[]}`},
		{"/api/agent-remove", `{"provider":"gemini"}`},
		{"/api/agent-access", `{"provider":"codex"}`},
		{"/api/agent-access", `{"provider":"gemini","on":true}`},
		{"/api/agent-access", `{"provider":"codex","on":"yes"}`},
		{"/api/agent-pause", `{"provider":"codex"}`},
		{"/api/agent-pause", `{"provider":"all","paused":true}`},
		{"/api/agent-pause", `{"provider":"gemini","paused":true}`},
		{"/api/project-rules", `{"name":"a/b","rules":"x"}`},
		{"/api/project-title", `{"name":"a/b","title":"x"}`},
		{"/api/project-title", `{"name":"Game","title":"x","extra":1}`},
		{"/api/project-rules", `{"name":"Game","rules":"x","extra":1}`},
		{"/api/task-cancel", `{"task":""}`},
		{"/api/project-remove", `{"name":"a/b"}`},
		{"/api/coordinator-connect", `{"project":"a/b"}`},
		{"/api/coordinator-connect", `{"project":"Game","cli":"/bin/sh"}`},
		{"/api/coordinator-connect", `{"project":"Game","agent":"unknown"}`},
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
	if code, _ := do("POST", "/api/agent-models", `{"provider":"codex","models":["gpt-5.5","gpt-5.5-mini"]}`); code != 200 || got[len(got)-1].Cmd != "agent-models" || got[len(got)-1].Provider != "codex" || got[len(got)-1].Text != "gpt-5.5\ngpt-5.5-mini" {
		t.Fatalf("agent models %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/project-title", `{"name":"Game","title":"Гонки 2"}`); code != 200 || got[len(got)-1].Cmd != "project-title" || got[len(got)-1].Alias != "Game" || got[len(got)-1].Text != "Гонки 2" {
		t.Fatalf("project-title not forwarded: %d %+v", code, got)
	}
	if code, _ := do("POST", "/api/project-rules", `{"name":"Game","rules":"Отчёт до 8 строк."}`); code != 200 || got[len(got)-1].Cmd != "project-rules" || got[len(got)-1].Alias != "Game" || got[len(got)-1].Text != "Отчёт до 8 строк." {
		t.Fatalf("project rules %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/agent-access", `{"provider":"all","on":false}`); code != 200 || got[len(got)-1].Provider != "all" || got[len(got)-1].Text != "0" {
		t.Fatalf("all agents access: %d %+v", code, got)
	}
	if code, _ := do("POST", "/api/agent-pause", `{"provider":"cursor","paused":true}`); code != 200 || got[len(got)-1].Cmd != "agent-pause" || got[len(got)-1].Provider != "cursor" || got[len(got)-1].Text != "1" {
		t.Fatalf("agent-pause not forwarded: %d %+v", code, got)
	}
	if code, _ := do("POST", "/api/agent-access", `{"provider":"codex","on":true}`); code != 200 || got[len(got)-1].Cmd != "agent-access" || got[len(got)-1].Provider != "codex" || got[len(got)-1].Text != "1" {
		t.Fatalf("agent access %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/project-remove", `{"name":"Тест"}`); code != 200 || got[len(got)-1].Cmd != "project-remove" || got[len(got)-1].Alias != "Тест" {
		t.Fatalf("project remove %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/agent-remove", `{"provider":"cursor"}`); code != 200 || got[len(got)-1].Cmd != "agent-remove" || got[len(got)-1].Provider != "cursor" {
		t.Fatalf("agent remove %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/coordinator-connect", `{"project":"Моя игра"}`); code != 200 || got[len(got)-1].Cmd != "coordinator-connect" || got[len(got)-1].Alias != "Моя игра" || got[len(got)-1].Provider != "claude" {
		t.Fatalf("coordinator connect %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/coordinator-connect", `{"project":"Моя игра","agent":"codex"}`); code != 200 || got[len(got)-1].Provider != "codex" {
		t.Fatalf("codex coordinator %d %+v", code, got[len(got)-1])
	}
	if code, _ := do("POST", "/api/coordinator-connect", `{"project":"Моя игра","agent":"cursor"}`); code != 200 || got[len(got)-1].Provider != "cursor" {
		t.Fatalf("cursor coordinator %d %+v", code, got[len(got)-1])
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
	// T-078: asking another computer to update itself.
	code, _ = do("POST", "/api/server-action", `{"action":"update","device":"dev-b"}`)
	if last := got[len(got)-1]; code != 200 || last.Cmd != "server" || last.Action != "update" || last.Text != `{"device":"dev-b"}` {
		t.Fatalf("update %d %+v", code, last)
	}
	for _, bad := range []string{`{"action":"update","device":"` + strings.Repeat("d", 65) + `"}`, `{"action":"delete-everything"}`, `{"action":"connect","code":"` + strings.Repeat("x", 3000) + `"}`, `{"action":"enable","extra":1}`, `{"action":"enable","port":70000}`} {
		if code, _ := do("POST", "/api/server-action", bad); code != 400 {
			t.Errorf("accepted %s: %d", bad[:min(len(bad), 60)], code)
		}
	}
}
