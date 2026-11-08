package localui

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

func TestShellLine(t *testing.T) {
	got := shellLine([]string{"/Users/me/My Tools/codex", "login"}, []string{"CODEX_HOME=/Users/me/it's"})
	want := `CODEX_HOME='/Users/me/it'\''s' '/Users/me/My Tools/codex' 'login'`
	if got != want {
		t.Fatalf("got  %s\nwant %s", got, want)
	}
}

func TestTerminalCommands(t *testing.T) {
	argv, env := []string{`/opt/a b/codex`, "login"}, []string{"CODEX_HOME=/p"}
	mac, err := terminalCommand("darwin", argv, env)
	if err != nil || mac.Path != "/usr/bin/osascript" || strings.Contains(strings.Join(mac.Args, " "), "/opt/a b") {
		t.Fatalf("mac %v %v", mac, err)
	}
	if !hasEnv(mac.Env, "AM_LOGIN="+shellLine(argv, env)) {
		t.Fatal("mac command line not in the environment")
	}
	if runtime.GOOS == "windows" {
		return
	}
	dir := t.TempDir()
	term := filepath.Join(dir, "xterm")
	if err := os.WriteFile(term, []byte("#!/bin/sh\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	linux, err := terminalCommand("linux", argv, env)
	if err != nil || linux.Path != term || linux.Args[len(linux.Args)-1] != `exec sh -c "$AM_LOGIN; printf '\nГотово. Нажмите Enter, чтобы закрыть окно.'; read _"` {
		t.Fatalf("linux %v %v", linux, err)
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := terminalCommand("linux", argv, env); err == nil {
		t.Fatal("no terminal program, but a command")
	}
}

func hasEnv(env []string, item string) bool {
	for _, e := range env {
		if e == item {
			return true
		}
	}
	return false
}

func TestAgentLoginAndInstallRoutes(t *testing.T) {
	var opened, started []string
	oldOpen, oldStart := openURLFunc, startTerminalFunc
	openURLFunc = func(u string) error { opened = append(opened, u); return nil }
	startTerminalFunc = func(argv, env []string) error { started = append(started, strings.Join(argv, " ")); return nil }
	t.Cleanup(func() { openURLFunc, startTerminalFunc = oldOpen, oldStart })
	cli := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(cli, []byte("x"), 0o700); err != nil {
		t.Fatal(err)
	}
	h := newHandler("127.0.0.1:1234", "secret", func(context.Context, control.Request) (control.Response, error) {
		return control.Response{OK: true}, nil
	}, func(context.Context) any { return nil })
	post := func(path, body string) (int, string) {
		r := httptest.NewRequest("POST", "http://127.0.0.1:1234"+path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer secret")
		r.Header.Set("Origin", "http://127.0.0.1:1234")
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Code, w.Body.String()
	}
	if code, _ := post("/api/agent-install", `{"provider":"codex"}`); code != 200 || len(opened) != 1 || !strings.HasPrefix(opened[0], "https://") {
		t.Fatalf("install %d %v", code, opened)
	}
	body, _ := json.Marshal(map[string]string{"provider": "codex", "cli": cli})
	if code, out := post("/api/agent-login", string(body)); code != 200 || len(started) != 1 || started[0] != cli+" login" {
		t.Fatalf("login %d %s %v", code, out, started)
	}
	for _, bad := range []struct{ path, body string }{
		{"/api/agent-install", `{"provider":"gemini"}`},
		{"/api/agent-install", `{"provider":"codex","url":"https://example.com"}`},
		{"/api/agent-login", `{"provider":"codex","cli":"codex"}`},
		{"/api/agent-login", `{"provider":"codex","cli":"/no/such/codex"}`},
	} {
		if code, _ := post(bad.path, bad.body); code == 200 {
			t.Fatalf("%s %s accepted", bad.path, bad.body)
		}
	}
	if len(opened) != 1 || len(started) != 1 {
		t.Fatalf("a refused request ran something: %v %v", opened, started)
	}
}
