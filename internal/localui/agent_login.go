package localui

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// The agent card's «Войти» and «Установить» buttons (D-057). Sign-in runs
// the agent's own command in a terminal window the owner sees; Messenger
// never reads or stores the sign-in. The separate page action is retained
// for removal instructions; installation is handled by agent_setup.go.

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
		// A desktop Go process has no console handles. Starting PowerShell
		// directly gives it NUL streams even with CREATE_NEW_CONSOLE. Let
		// Windows create an interactive child with its own console instead.
		launcher := `$ErrorActionPreference='Stop'
$encoded=[Convert]::ToBase64String([Text.Encoding]::Unicode.GetBytes($env:AM_TERMINAL_SCRIPT))
Start-Process -FilePath (Join-Path $PSHOME 'powershell.exe') -ArgumentList @('-NoProfile','-NoExit','-ExecutionPolicy','Bypass','-EncodedCommand',$encoded) -ErrorAction Stop`
		cmd := exec.Command(filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
			"-NoProfile", "-NonInteractive", "-Command", launcher)
		cmd.Env = append(append(os.Environ(), env...), "AM_LOGIN="+string(raw), "AM_TERMINAL_SCRIPT="+script)
		platform.HideConsole(cmd)
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
		terminalShell := os.Getenv("SHELL")
		if !filepath.IsAbs(terminalShell) {
			terminalShell = "/bin/sh"
		}
		line := `exec "$AM_TERMINAL_SHELL" -lic "$AM_LOGIN; printf '\nГотово. Нажмите Enter, чтобы закрыть окно.'; read am_terminal_close"`
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
			cmd.Env = append(os.Environ(), "AM_LOGIN="+shellLine(argv, env), "AM_TERMINAL_SHELL="+terminalShell)
			return cmd, nil
		}
		return nil, errNoTerminal
	}
}

// ErrTerminalUnconfirmed means the launcher started but its result is not
// confirmed. In particular an installation must not be retried on this result.
var errTerminalUnconfirmed = errors.New("terminal launch unconfirmed")

func startTerminal(argv, env []string) error {
	cmd, err := terminalCommand(runtime.GOOS, argv, env)
	if err != nil {
		return err
	}
	return launchTerminal(cmd, runtime.GOOS == "darwin" || runtime.GOOS == "windows")
}

func launchTerminal(cmd *exec.Cmd, shortLauncher bool) error {
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	wait := 300 * time.Millisecond
	if shortLauncher {
		wait = 4 * time.Second
	}
	select {
	case err := <-done:
		if err != nil {
			detail := stderr.String()
			if len(detail) > 2048 {
				detail = detail[:2048]
			}
			return fmt.Errorf("%w: %v: %s", errTerminalUnconfirmed, err, strings.TrimSpace(detail))
		}
		return nil
	case <-time.After(wait):
		if shortLauncher {
			return errTerminalUnconfirmed
		}
		return nil // Linux terminal emulators may remain alive with the window.
	}
}

func manualTerminalLine(argv, env []string) string {
	if runtime.GOOS != "windows" {
		return shellLine(argv, env)
	}
	q := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "''") + "'" }
	var parts []string
	for _, v := range env {
		name, value, _ := strings.Cut(v, "=")
		parts = append(parts, "$env:"+name+"="+q(value)+";")
	}
	parts = append(parts, "&")
	for _, v := range argv {
		parts = append(parts, q(v))
	}
	return strings.Join(parts, " ")
}

func serveAgentPage(w http.ResponseWriter, decoder *json.Decoder) {
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

// serveAgentLogin opens the agent's sign-in, or with logout its sign-out
// (T-056), in a terminal window the owner sees.
func serveAgentLogin(w http.ResponseWriter, decoder *json.Decoder, logout bool) {
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
	command := diagnostics.LoginCommand
	if logout {
		command = diagnostics.LogoutCommand
	}
	argv, env, err := command(in.Provider, in.CLI, in.Environment)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writeJSON(w, map[string]any{"ok": false, "detail": err.Error()})
		return
	}
	if err := startTerminalFunc(argv, env); err != nil {
		w.WriteHeader(http.StatusNotImplemented)
		writeJSON(w, map[string]any{"ok": false, "detail": "Открытие терминала не подтверждено. Выполните команду в терминале вручную.", "command": manualTerminalLine(argv, env), "hint": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}
