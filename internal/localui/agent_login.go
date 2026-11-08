package localui

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// The agent card's «Войти» and «Установить» buttons (D-057). Sign-in runs
// the agent's own command in a terminal window the owner sees; Messenger
// never reads or stores the sign-in. Installation opens the agent's official
// page; nothing is downloaded or run.

// openURLFunc and startTerminalFunc are replaced in tests.
var (
	openURLFunc       = openURL
	startTerminalFunc = startTerminal
)

var errNoTerminal = errors.New("no terminal program on this computer")

// shellLine is a POSIX shell command line for argv with env assignments.
func shellLine(argv, env []string) string {
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
	var parts []string
	for _, e := range env {
		name, value, _ := strings.Cut(e, "=")
		parts = append(parts, name+"="+quote(value))
	}
	for _, a := range argv {
		parts = append(parts, quote(a))
	}
	return strings.Join(parts, " ")
}

// terminalCommand opens a terminal window running argv. The command travels
// in an environment variable, never inside a script.
func terminalCommand(goos string, argv, env []string) (*exec.Cmd, error) {
	switch goos {
	case "windows":
		root := os.Getenv("SystemRoot")
		if !filepath.IsAbs(root) {
			return nil, errNoTerminal
		}
		raw, _ := json.Marshal(argv)
		script := `[Console]::OutputEncoding=[Text.Encoding]::UTF8
$a = $env:AM_LOGIN | ConvertFrom-Json
& $a[0] @($a | Select-Object -Skip 1)
Write-Host ''
Write-Host 'Готово. Окно можно закрыть.'`
		cmd := exec.Command(filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
			"-NoProfile", "-NoExit", "-ExecutionPolicy", "Bypass", "-Command", script)
		cmd.Env = append(append(os.Environ(), env...), "AM_LOGIN="+string(raw))
		platform.NewConsole(cmd)
		return cmd, nil
	case "darwin":
		script := `set c to system attribute "AM_LOGIN"
tell application "Terminal"
	activate
	do script c
end tell`
		cmd := exec.Command("/usr/bin/osascript", "-e", script)
		cmd.Env = append(os.Environ(), "AM_LOGIN="+shellLine(argv, env))
		return cmd, nil
	default:
		line := `exec sh -c "$AM_LOGIN; printf '\nГотово. Нажмите Enter, чтобы закрыть окно.'; read _"`
		for _, t := range []struct {
			name string
			args []string
		}{
			{"x-terminal-emulator", []string{"-e", "sh", "-c"}},
			{"gnome-terminal", []string{"--", "sh", "-c"}},
			{"konsole", []string{"-e", "sh", "-c"}},
			{"xfce4-terminal", []string{"-x", "sh", "-c"}},
			{"xterm", []string{"-e", "sh", "-c"}},
		} {
			path, err := exec.LookPath(t.name)
			if err != nil {
				continue
			}
			cmd := exec.Command(path, append(t.args, line)...)
			cmd.Env = append(os.Environ(), "AM_LOGIN="+shellLine(argv, env))
			return cmd, nil
		}
		return nil, errNoTerminal
	}
}

func startTerminal(argv, env []string) error {
	cmd, err := terminalCommand(runtime.GOOS, argv, env)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func serveAgentInstall(w http.ResponseWriter, decoder *json.Decoder) {
	var in struct {
		Provider string `json:"provider"`
	}
	page, ok := "", false
	if err := decoder.Decode(&in); err == nil && decoder.Decode(new(any)) == io.EOF {
		page, ok = diagnostics.InstallPages[in.Provider]
	}
	if !ok {
		http.Error(w, "invalid request", 400)
		return
	}
	if err := openURLFunc(page); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]any{"ok": false, "detail": "Браузер не открылся. Адрес страницы установки: " + page, "url": page})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "url": page})
}

func serveAgentLogin(w http.ResponseWriter, decoder *json.Decoder) {
	var in struct {
		Provider    string `json:"provider"`
		CLI         string `json:"cli"`
		Environment string `json:"environment"`
	}
	if err := decoder.Decode(&in); err != nil || decoder.Decode(new(any)) != io.EOF || len(in.CLI) > 4096 || len(in.Environment) > 4096 ||
		strings.ContainsRune(in.CLI+in.Environment, 0) || (in.Environment != "" && !filepath.IsAbs(in.Environment)) {
		http.Error(w, "invalid request", 400)
		return
	}
	argv, env, err := diagnostics.LoginCommand(in.Provider, in.CLI, in.Environment)
	if err != nil {
		http.Error(w, "invalid request", 400)
		return
	}
	if err := startTerminalFunc(argv, env); err != nil {
		w.WriteHeader(http.StatusNotImplemented)
		writeJSON(w, map[string]any{"ok": false, "detail": "Окно терминала не открылось. Выполните вход вручную: " + shellLine(argv, env)})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
