package localui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

func TestAgentSetupReceiptAndRegistration(t *testing.T) {
	root := t.TempDir()
	cli := filepath.Join(root, "a CLI.exe")
	os.WriteFile(cli, []byte("fake"), 0700)
	launched, registered := 0, 0
	old := startTerminalFunc
	startTerminalFunc = func(argv, env []string) error { launched++; return nil }
	t.Cleanup(func() { startTerminalFunc = old })
	call := func(_ context.Context, r control.Request) (control.Response, error) {
		if r.Cmd == "window-view" {
			b, _ := json.Marshal(core.WindowView{Agents: []core.WindowAgent{{Agent: "codex", Env: root}}})
			return control.Response{OK: true, Overview: b}, nil
		}
		if r.Cmd != "agent-set" || r.CLI != cli || r.Environment != root {
			t.Fatalf("unexpected registration: %+v", r)
		}
		registered++
		return control.Response{OK: true}, nil
	}
	handler := func() http.Handler {
		return newDesktopHandler(actionHost, "secret", call, func(context.Context) any { return nil }, nil, Desktop{setupRoot: root})
	}
	h := handler()
	begin := func() setupState {
		w := postAction(h, "agent-install", `{"provider":"codex","action":"install"}`)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var st setupState
		json.Unmarshal(w.Body.Bytes(), &st)
		return st
	}
	st := begin()
	if !st.OK || st.State != "running" || launched != 1 || registered != 0 {
		t.Fatal(st, launched, registered)
	}
	if second := begin(); second.ID != st.ID || launched != 1 {
		t.Fatal("duplicate launched", second)
	}
	h = handler()
	begin()
	if launched != 1 {
		t.Fatal("window restart repeated installation")
	}
	dir := (&agentSetup{root: root}).dir("codex", "install")
	os.WriteFile(filepath.Join(dir, "result"), []byte("found\n"+cli+"\n"), 0600)
	body, _ := json.Marshal(setupInput{Provider: "codex", Action: "install", ID: st.ID})
	for i := 0; i < 2; i++ {
		w := postAction(h, "agent-setup-finish", string(body))
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
		json.Unmarshal(w.Body.Bytes(), &st)
		if !st.OK || st.State != "connected" {
			t.Fatal(st)
		}
	}
	if registered != 1 {
		t.Fatal("registration repeated", registered)
	}
	if fresh := begin(); fresh.ID == st.ID || launched != 2 {
		t.Fatal("explicit new attempt failed", fresh)
	}
	for _, body := range []string{`{"provider":"other","action":"install"}`, `{"provider":"codex","action":"find"}`, `{"provider":"codex","action":"install","url":"https://example.com"}`} {
		if w := postAction(h, "agent-install", body); w.Code == 200 {
			t.Fatal("invalid install accepted", body)
		}
	}
}

func TestAgentSetupUnknownIsNotRetried(t *testing.T) {
	root := t.TempDir()
	cli := filepath.Join(root, "cli.exe")
	os.WriteFile(cli, []byte("fake"), 0700)
	s := &agentSetup{root: root}
	dir := s.dir("claude", "find")
	os.MkdirAll(dir, 0700)
	os.WriteFile(filepath.Join(dir, "id"), []byte("fictional"), 0600)
	os.WriteFile(filepath.Join(dir, "result"), []byte("found\n"+cli+"\n"), 0600)
	calls := 0
	call := func(_ context.Context, r control.Request) (control.Response, error) {
		if r.Cmd == "window-view" {
			return control.Response{OK: true, Overview: json.RawMessage(`{}`)}, nil
		}
		calls++
		return control.Response{}, errors.New("lost reply")
	}
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/api/agent-setup-finish", nil)
		s.serve(w, r, json.NewDecoder(strings.NewReader(`{"provider":"claude","action":"find","id":"fictional"}`)), call, true)
		if w.Code != 200 {
			t.Fatal(w.Body.String())
		}
	}
	if calls != 1 || s.status("claude", "find").State != "unconfirmed" {
		t.Fatal("uncertain registration retried", calls)
	}
	os.WriteFile(filepath.Join(dir, "result"), []byte("found\nrelative-path\n"), 0600)
	if s.status("claude", "find").State != "failed" {
		t.Fatal("relative path accepted")
	}
}

func TestSetupScriptsWithFakeInstaller(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX script; Windows search is tested separately")
	}
	for _, provider := range []string{"claude", "codex", "cursor"} {
		for _, fail := range []bool{false, true} {
			t.Run(provider+map[bool]string{false: "-success", true: "-failure"}[fail], func(t *testing.T) {
				dir := t.TempDir()
				home := filepath.Join(dir, "home with ' quote")
				bin := filepath.Join(home, ".local", "bin")
				os.MkdirAll(bin, 0700)
				name := provider
				if name == "cursor" {
					name = "cursor-agent"
				}
				fake := filepath.Join(dir, "fake-tools")
				os.Mkdir(fake, 0700)
				installer := `#!/bin/sh
mkdir -p "$HOME/.local/bin"
printf '#!/bin/sh\nexit 0\n' > "$HOME/.local/bin/` + name + `"
chmod +x "$HOME/.local/bin/` + name + `"
`
				if fail {
					installer += "exit 7\n"
				}
				fixture := filepath.Join(dir, "fake-installer")
				os.WriteFile(fixture, []byte(installer), 0700)
				os.WriteFile(filepath.Join(fake, "curl"), []byte("#!/bin/sh\ncp \"$AM_TEST_INSTALLER\" \"$4\"\n"), 0700)
				script, _ := setupScript("darwin", provider, "install", dir)
				script = strings.ReplaceAll(script, "/opt/homebrew/bin /usr/local/bin", shellLine([]string{filepath.Join(dir, "no-global-tools")}, nil))
				path := filepath.Join(dir, "setup.sh")
				os.WriteFile(path, []byte(script), 0700)
				cmd := exec.Command("/bin/sh", path)
				cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+fake+":/usr/bin:/bin", "AM_TEST_INSTALLER="+fixture)
				out, err := cmd.CombinedOutput()
				if !fail && err != nil {
					t.Fatalf("%v %s", err, out)
				}
				raw, _ := os.ReadFile(filepath.Join(dir, "result"))
				want := "found\n" + filepath.Join(bin, name) + "\n"
				if fail {
					want = "failed\n\n"
				}
				if string(raw) != want {
					t.Fatalf("%q want %q (%s)", raw, want, out)
				}
				// Find uses the same terminal PATH but never downloads or runs an installer.
				script, _ = setupScript("darwin", provider, "find", dir)
				if strings.Contains(script, "curl") {
					t.Fatal("find installs")
				}
				os.WriteFile(path, []byte(script), 0700)
				cmd = exec.Command("/bin/sh", path)
				cmd.Env = append(os.Environ(), "HOME="+home, "PATH="+bin+":/usr/bin:/bin")
				if out, err = cmd.CombinedOutput(); err != nil {
					t.Fatal(err, string(out))
				}
				raw, _ = os.ReadFile(filepath.Join(dir, "result"))
				if string(raw) != "found\n"+filepath.Join(bin, name)+"\n" {
					t.Fatal(string(raw))
				}
			})
		}
	}
}

func TestWindowsSetupFind(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("PowerShell execution requires Windows")
	}
	dir := filepath.Join(t.TempDir(), "тест ' path")
	os.Mkdir(dir, 0700)
	cli := filepath.Join(dir, "codex.exe")
	os.WriteFile(cli, []byte("fake; never executed"), 0700)
	script, argv := setupScript("windows", "codex", "find", dir)
	os.WriteFile(filepath.Join(dir, "setup.ps1"), []byte("\ufeff"+script), 0600)
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Env = append(os.Environ(), "PATH="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatal(err, string(out))
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "result"))
	if string(raw) != "found\n"+cli+"\n" {
		t.Fatalf("receipt %q", raw)
	}
}

func TestTrustUsesRegisteredCLIAndExactFolder(t *testing.T) {
	dir := t.TempDir()
	cli := filepath.Join(dir, "fake CLI.exe")
	os.WriteFile(cli, []byte("fake"), 0700)
	launched := 0
	old := startTerminalFunc
	startTerminalFunc = func(argv, env []string) error {
		launched++
		line := strings.Join(argv, " ") + strings.Join(env, " ")
		hasCLI := strings.Contains(line, cli)
		for _, v := range env {
			if strings.HasPrefix(v, "AM_TRUST_ARGS=") {
				var args []string
				json.Unmarshal([]byte(strings.TrimPrefix(v, "AM_TRUST_ARGS=")), &args)
				hasCLI = len(args) > 0 && args[0] == cli
			}
		}
		if !hasCLI || !strings.Contains(line, dir) {
			t.Fatalf("wrong CLI/folder: %q", line)
		}
		return nil
	}
	t.Cleanup(func() { startTerminalFunc = old })
	b, _ := json.Marshal(core.WindowView{Project: "fixture", Folder: dir, Agents: []core.WindowAgent{{Agent: "codex", CLI: cli}}})
	h := newHandler(actionHost, "secret", func(context.Context, control.Request) (control.Response, error) {
		return control.Response{OK: true, Overview: b}, nil
	}, func(context.Context) any { return nil })
	body, _ := json.Marshal(map[string]string{"provider": "codex", "project": "fixture", "folder": dir})
	if w := postAction(h, "agent-trust", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, changes := range []map[string]string{{"provider": "claude", "project": "fixture", "folder": dir}, {"provider": "codex", "project": "fixture", "folder": filepath.Dir(dir)}} {
		body, _ = json.Marshal(changes)
		if w := postAction(h, "agent-trust", string(body)); w.Code == 200 {
			t.Fatal("wrong folder/provider accepted")
		}
	}
	if launched != 1 {
		t.Fatal(launched)
	}
}
