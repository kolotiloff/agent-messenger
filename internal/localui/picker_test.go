package localui

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

func TestPickRoute(t *testing.T) {
	old := pickFunc
	t.Cleanup(func() { pickFunc = old })
	var kinds, starts []string
	answer, fail := "", error(nil)
	pickFunc = func(_ context.Context, kind, start string) (string, error) {
		kinds, starts = append(kinds, kind), append(starts, start)
		return answer, fail
	}
	h := newHandler("127.0.0.1:1234", "secret", func(context.Context, control.Request) (control.Response, error) {
		t.Fatal("a choice must not reach the client")
		return control.Response{}, nil
	}, func(context.Context) any { return nil })
	do := func(body string, origin bool) (int, map[string]any) {
		r := httptest.NewRequest("POST", "http://127.0.0.1:1234/api/pick", strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer secret")
		r.Header.Set("Content-Type", "application/json")
		if origin {
			r.Header.Set("Origin", "http://127.0.0.1:1234")
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		var out map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	for _, bad := range []string{`{"kind":"run"}`, `{"kind":"file","extra":1}`, `{}`} {
		if code, _ := do(bad, true); code != 400 {
			t.Fatal(bad, code)
		}
	}
	if code, _ := do(`{"kind":"file"}`, false); code != 403 {
		t.Fatal("a request without the window's origin was served", code)
	}
	if len(kinds) != 0 {
		t.Fatal("a refused request opened a dialog")
	}
	dir := t.TempDir()
	answer = filepath.Join(dir, "codex.exe")
	current, _ := json.Marshal(map[string]string{"kind": "file", "current": filepath.Join(dir, "old.exe")})
	if code, out := do(string(current), true); code != 200 || out["path"] != answer || kinds[0] != "file" || starts[0] != dir {
		t.Fatal(code, out, kinds, starts)
	}
	answer = ""
	if code, out := do(`{"kind":"folder"}`, true); code != 200 || out["path"] != "" {
		t.Fatal("a closed dialog", code, out)
	}
	fail = errNoPicker
	if code, out := do(`{"kind":"folder"}`, true); code != 501 || !strings.Contains(out["detail"].(string), "вручную") {
		t.Fatal(code, out)
	}
}

// The dialog programs; none of them is started here.
func TestPickCommands(t *testing.T) {
	t.Setenv("SystemRoot", filepath.Join(t.TempDir(), "Windows"))
	for _, kind := range []string{"file", "folder"} {
		cmd, err := pickCommand("windows", kind, `C:\Users\me`)
		if err != nil || !strings.HasSuffix(cmd.Path, "powershell.exe") || !strings.Contains(strings.Join(cmd.Env, "\n"), `AM_PICK_START=C:\Users\me`) ||
			strings.Contains(strings.Join(cmd.Args, " "), `C:\Users\me`) {
			t.Fatal(kind, cmd, err)
		}
		cmd, err = pickCommand("darwin", kind, "/Users/me")
		if err != nil || cmd.Path != "/usr/bin/osascript" || !strings.Contains(strings.Join(cmd.Args, " "), "choose "+kind) {
			t.Fatal(kind, cmd, err)
		}
	}
}
