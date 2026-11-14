package localui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"os/exec"
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
	if err != nil || linux.Path != term || linux.Args[len(linux.Args)-1] != `exec "$AM_TERMINAL_SHELL" -lic "$AM_LOGIN; printf '\nГотово. Нажмите Enter, чтобы закрыть окно.'; read am_terminal_close"` {
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
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
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
	if code, _ := post("/api/agent-page", `{"provider":"codex"}`); code != 200 || len(opened) != 1 || !strings.HasPrefix(opened[0], "https://") {
		t.Fatalf("install %d %v", code, opened)
	}
	body, _ := json.Marshal(map[string]string{"provider": "codex", "cli": cli})
	if code, out := post("/api/agent-login", string(body)); code != 200 || len(started) != 1 || started[0] != cli+" login" {
		t.Fatalf("login %d %s %v", code, out, started)
	}
	// «Выйти» runs the agent's own sign-out the same way (T-056).
	if code, out := post("/api/agent-logout", string(body)); code != 200 || len(started) != 2 || started[1] != cli+" logout" {
		t.Fatalf("logout %d %s %v", code, out, started)
	}
	started = started[:1]
	for _, bad := range []struct{ path, body string }{
		{"/api/agent-logout", `{"provider":"gemini","cli":"/bin/sh"}`},
		{"/api/agent-page", `{"provider":"gemini"}`},
		{"/api/agent-page", `{"provider":"codex","url":"https://example.com"}`},
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
	body, _ = json.Marshal(map[string]string{"provider": "cursor", "cli": cli, "environment": filepath.Join(home, ".cursor")})
	if code, out := post("/api/agent-login", string(body)); code != 200 || len(started) != 2 {
		t.Fatalf("fresh Cursor profile blocked login: %d %s", code, out)
	}
	missing := filepath.Join(home, "missing-custom-profile")
	body, _ = json.Marshal(map[string]string{"provider": "cursor", "cli": cli, "environment": missing})
	code, out := post("/api/agent-login", string(body))
	var failure struct {
		Detail string `json:"detail"`
	}
	if json.Unmarshal([]byte(out), &failure) != nil || code != 400 || !strings.Contains(failure.Detail, missing) || len(started) != 2 {
		t.Fatalf("missing actionable profile error: %d %s", code, out)
	}
}

func TestTerminalLaunchReportsImmediateFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX fake launcher")
	}
	cmd := exec.Command("/bin/sh", "-c", "printf 'fixture terminal unavailable' >&2; exit 7")
	err := launchTerminal(cmd, true)
	if !errors.Is(err, errTerminalUnconfirmed) || !strings.Contains(err.Error(), "fixture terminal unavailable") {
		t.Fatal(err)
	}
	if err := launchTerminal(exec.Command("/bin/sh", "-c", "exit 0"), true); err != nil {
		t.Fatal(err)
	}
}
func TestWindowsTerminalCreatesInteractiveChild(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path semantics")
	}
	cmd, err := terminalCommand("windows", []string{`C:\Program Files\agent.exe`, "login"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Join(cmd.Args, " ")
	if !strings.Contains(line, "Start-Process") || strings.Contains(line, "-NoNewWindow") || !strings.Contains(line, "-EncodedCommand") {
		t.Fatal("no independent interactive console", line)
	}
}
