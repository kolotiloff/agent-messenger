package localui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
)

// Setup receipts survive closing the window. An unfinished attempt is never
// relaunched automatically, and an uncertain registration is never retried.
type agentSetup struct {
	root string
	mu   sync.Mutex
}
type setupState struct {
	OK       bool   `json:"ok"`
	Provider string `json:"provider"`
	Action   string `json:"action"`
	ID       string `json:"id"`
	State    string `json:"state"`
	CLI      string `json:"cli,omitempty"`
}
type setupInput struct {
	Provider string `json:"provider"`
	Action   string `json:"action"`
	ID       string `json:"id,omitempty"`
}

func (s *agentSetup) dir(provider, action string) string {
	return filepath.Join(s.root, "agent-setup", provider+"-"+action)
}
func validSetup(provider, action string) bool {
	_, ok := diagnostics.InstallPages[provider]
	return ok && (action == "install" || action == "find")
}
func (s *agentSetup) status(provider, action string) setupState {
	st := setupState{OK: true, Provider: provider, Action: action, State: "idle"}
	if s.root == "" {
		return st
	}
	dir := s.dir(provider, action)
	id, err := os.ReadFile(filepath.Join(dir, "id"))
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			st.State = "unknown"
		}
		return st
	}
	st.ID = string(id)
	// Launching a terminal is not evidence that its script is still running.
	// This also handles receipts left by older versions or a closed terminal.
	st.State = "unknown"
	if b, err := os.ReadFile(filepath.Join(dir, "recovered")); err == nil && string(b) == st.ID {
		st.State = "recovered"
		return st
	}
	raw, err := os.ReadFile(filepath.Join(dir, "result"))
	if err != nil {
		return st
	}
	if len(raw) > 8192 {
		st.State = "failed"
		return st
	}
	kind, path, _ := strings.Cut(strings.TrimSuffix(string(raw), "\n"), "\n")
	if kind == "interrupted" {
		return st
	}
	if kind != "found" {
		st.State = "failed"
		return st
	}
	st.CLI = diagnostics.ResolveTerminalCLI(provider, path)
	if st.CLI == "" {
		st.State = "failed"
		return st
	}
	st.State = "found"
	if b, err := os.ReadFile(filepath.Join(dir, "connected")); err == nil && string(b) == st.ID {
		st.State = "connected"
	} else if _, err := os.Stat(filepath.Join(dir, "claimed")); err == nil {
		st.State = "unconfirmed"
	}
	return st
}
func (s *agentSetup) list() []setupState {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []setupState{}
	for _, p := range []string{"claude", "codex", "cursor"} {
		for _, a := range []string{"install", "find"} {
			out = append(out, s.status(p, a))
		}
	}
	return out
}
func (s *agentSetup) serve(w http.ResponseWriter, r *http.Request, dec *json.Decoder, call caller, finish bool) {
	var in setupInput
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF || !validSetup(in.Provider, in.Action) {
		http.Error(w, "invalid request", 400)
		return
	}
	if !finish && ((r.URL.Path == "/api/agent-install" && in.Action != "install") || (r.URL.Path == "/api/agent-find" && in.Action != "find")) {
		http.Error(w, "invalid action", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == "" {
		http.Error(w, "setup unavailable", 503)
		return
	}
	st := s.status(in.Provider, in.Action)
	dir := s.dir(in.Provider, in.Action)
	if finish {
		if st.ID == "" || st.ID != in.ID || st.State != "found" {
			writeJSON(w, st)
			return
		}
		// Keep a registered agent's profile; discovery must not switch accounts.
		response, err := call(r.Context(), control.Request{Cmd: "window-view"})
		var view core.WindowView
		if err != nil || !response.OK || json.Unmarshal(response.Overview, &view) != nil {
			http.Error(w, "client unavailable", 503)
			return
		}
		profile := ""
		for _, a := range view.Agents {
			if a.Agent == in.Provider {
				profile = a.Env
			}
		}
		claim, err := os.OpenFile(filepath.Join(dir, "claimed"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			http.Error(w, "registration already attempted", 409)
			return
		}
		claim.Close()
		response, err = call(r.Context(), control.Request{Cmd: "agent-set", Provider: in.Provider, CLI: st.CLI, Environment: profile})
		if err == nil && response.OK {
			_ = os.WriteFile(filepath.Join(dir, "connected"), []byte(st.ID), 0600)
			if in.Action == "find" {
				// Finding and registering the CLI resolves the setup UI, not the
				// installer's unknown exit. Keep its evidence and never relaunch it.
				install := s.status(in.Provider, "install")
				if install.ID != "" && (install.State == "unknown" || install.State == "found" || install.State == "failed") {
					_ = os.WriteFile(filepath.Join(s.dir(in.Provider, "install"), "recovered"), []byte(install.ID), 0600)
				}
			}
		}
		writeJSON(w, s.status(in.Provider, in.Action))
		return
	}
	if st.State == "running" || st.State == "unknown" || st.State == "found" || st.State == "unconfirmed" || st.State == "recovered" {
		writeJSON(w, st)
		return
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0700); err != nil {
		http.Error(w, "setup directory unavailable", 500)
		return
	}
	// A completed attempt stays as evidence; a fresh explicit click gets a new ID.
	if st.State != "idle" {
		if err := os.Rename(dir, dir+"-"+st.ID); err != nil {
			http.Error(w, "cannot archive setup", 500)
			return
		}
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		http.Error(w, "setup already exists", 409)
		return
	}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		http.Error(w, "cannot start setup", 500)
		return
	}
	id := hex.EncodeToString(nonce)
	script, argv := setupScript(runtime.GOOS, in.Provider, in.Action, dir)
	extension := ".sh"
	if runtime.GOOS == "windows" {
		extension = ".ps1"
		script = "\ufeff" + script // Windows PowerShell 5 reads UTF-8 only with a BOM.
	}
	err := os.WriteFile(filepath.Join(dir, "setup"+extension), []byte(script), 0600)
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "id"), []byte(id), 0600)
	}
	if err != nil {
		http.Error(w, "cannot write setup", 500)
		return
	}
	if err := startTerminalFunc(argv, nil); err != nil {
		if !errors.Is(err, errTerminalUnconfirmed) {
			_ = os.WriteFile(filepath.Join(dir, "result"), []byte("failed\n"), 0600)
		}
		w.WriteHeader(501)
		writeJSON(w, map[string]any{"ok": false, "detail": "Открытие терминала не подтверждено. Проверьте терминал или выберите файл CLI в настройках агента.", "hint": err.Error()})
		return
	}
	writeJSON(w, s.status(in.Provider, in.Action))
}

// The URLs and shell code are fixed here, never accepted from HTTP requests.
// Download first: a failed curl must not look like a successful empty script.
func setupScript(goos, provider, action, dir string) (string, []string) {
	names := map[string]string{"claude": "claude", "codex": "codex", "cursor": "cursor-agent agent"}[provider]
	urls := map[string]string{"claude": "https://claude.ai/install.sh", "codex": "https://chatgpt.com/codex/install.sh", "cursor": "https://cursor.com/install"}
	if goos == "windows" {
		urls = map[string]string{"claude": "https://claude.ai/install.ps1", "codex": "https://chatgpt.com/codex/install.ps1", "cursor": "https://cursor.com/install?win32=true"}
		// Single quoted PowerShell literals, including paths with apostrophes.
		q := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "''") + "'" }
		script := `$ErrorActionPreference='Stop'
$amDir=` + q(dir) + `
function Complete-Setup($state,$path) {
 [IO.File]::WriteAllText((Join-Path $amDir 'result.tmp'), "$state` + "`n" + `$path` + "`n" + `", (New-Object Text.UTF8Encoding($false)))
 Move-Item -LiteralPath (Join-Path $amDir 'result.tmp') -Destination (Join-Path $amDir 'result') -Force
}
function Find-Agent {
 foreach ($name in ` + q(strings.ReplaceAll(names, " ", ",")) + `.Split(',')) {
  $c=Get-Command $name -CommandType Application,ExternalScript -ErrorAction SilentlyContinue | Select-Object -First 1
  if ($c -and [IO.Path]::IsPathRooted($c.Source)) {return $c.Source}
  foreach ($base in @((Join-Path $env:USERPROFILE '.local\bin'),(Join-Path $env:LOCALAPPDATA 'cursor-agent'))) {
   foreach ($ext in @('.exe','.cmd','.ps1')) { $p=Join-Path $base ($name+$ext); if (Test-Path -LiteralPath $p -PathType Leaf) {return $p} }
  }
 }
 return ''
}
try {
 $env:PATH=$env:PATH+';'+[Environment]::GetEnvironmentVariable('PATH','User')+';'+[Environment]::GetEnvironmentVariable('PATH','Machine')
 $amCLI=Find-Agent
`
		if action == "install" {
			script += ` if (!$amCLI) {
  Invoke-WebRequest -UseBasicParsing -Uri ` + q(urls[provider]) + ` -OutFile (Join-Path $amDir 'installer.ps1')
  & (Join-Path $PSHOME 'powershell.exe') -NoProfile -ExecutionPolicy Bypass -File (Join-Path $amDir 'installer.ps1')
  if ($LASTEXITCODE -ne 0) {throw 'Installer failed'}
  $env:PATH=[Environment]::GetEnvironmentVariable('PATH','User')+';'+[Environment]::GetEnvironmentVariable('PATH','Machine')+';'+$env:PATH
  $amCLI=Find-Agent
 }
`
		}
		script += ` if (!$amCLI) {throw 'CLI not found'}
 Complete-Setup 'found' $amCLI
 Write-Host 'CLI found. Return to Messenger.'
} catch { Complete-Setup 'failed' ''; Write-Host $_ }
`
		return script, []string{filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-ExecutionPolicy", "Bypass", "-File", filepath.Join(dir, "setup.ps1")}
	}
	q := func(v string) string { return shellLine([]string{v}, nil) }
	script := `#!/bin/sh
am_dir=` + q(dir) + `
umask 077
finish_setup() { printf '%s\n%s\n' "$1" "$2" > "$am_dir/result.tmp" && mv "$am_dir/result.tmp" "$am_dir/result"; }
setup_interrupted() { trap '' HUP INT TERM; if [ ! -f "$am_dir/result" ]; then finish_setup interrupted ''; fi; }
trap setup_interrupted EXIT
trap 'setup_interrupted; exit 129' HUP
trap 'setup_interrupted; exit 130' INT
trap 'setup_interrupted; exit 143' TERM
find_agent() {
 for am_name in ` + names + `; do
  am_path=$(command -v "$am_name" 2>/dev/null) || am_path=''
  case "$am_path" in /*) if [ -f "$am_path" ] && [ -x "$am_path" ]; then printf '%s' "$am_path"; return; fi;; esac
  for am_base in "$HOME/.local/bin" /opt/homebrew/bin /usr/local/bin; do
   am_path="$am_base/$am_name"
   if [ -f "$am_path" ] && [ -x "$am_path" ]; then printf '%s' "$am_path"; return; fi
  done
 done
}
am_cli=$(find_agent)
`
	if action == "install" {
		interpreter := "bash"
		if provider == "codex" {
			interpreter = "sh"
		}
		script += `if [ -z "$am_cli" ]; then
 if ! curl -fsSL ` + q(urls[provider]) + ` -o "$am_dir/installer.sh" || ! ` + interpreter + ` "$am_dir/installer.sh"; then
  finish_setup failed ''; printf '\nInstallation failed. Return to Messenger.\n'; exit 1
 fi
 am_cli=$(find_agent)
fi
`
	}
	script += `if [ -n "$am_cli" ]; then finish_setup found "$am_cli"; printf '\nCLI found. Return to Messenger.\n'; else finish_setup failed ''; printf '\nCLI not found. Return to Messenger.\n'; fi
`
	argv := []string{"/bin/sh", filepath.Join(dir, "setup.sh")}
	return script, argv
}

// Trust opens the CLI's own interactive confirmation, without sending a prompt
// or changing approval/sandbox settings. No graphical agent app is required.
func serveAgentTrust(w http.ResponseWriter, r *http.Request, dec *json.Decoder, call caller) {
	var in struct {
		Provider string `json:"provider"`
		Project  string `json:"project"`
		Folder   string `json:"folder"`
	}
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF || len(in.Project) > 256 || !filepath.IsAbs(in.Folder) {
		http.Error(w, "invalid request", 400)
		return
	}
	response, err := call(r.Context(), control.Request{Cmd: "window-view", Alias: in.Project})
	var view core.WindowView
	if err != nil || !response.OK || json.Unmarshal(response.Overview, &view) != nil {
		http.Error(w, "client unavailable", 503)
		return
	}
	if view.Project != in.Project || view.Folder != in.Folder {
		http.Error(w, "project folder changed", 409)
		return
	}
	var agent *core.WindowAgent
	for i := range view.Agents {
		if view.Agents[i].Agent == in.Provider {
			agent = &view.Agents[i]
		}
	}
	if agent == nil {
		http.Error(w, "agent not registered", 400)
		return
	}
	argv, env, err := diagnostics.InteractiveCommand(in.Provider, agent.CLI, agent.Env)
	if err != nil {
		http.Error(w, "invalid agent", 400)
		return
	}
	if runtime.GOOS == "windows" {
		raw, _ := json.Marshal(argv)
		env = append(env, "AM_TRUST_ARGS="+string(raw), "AM_TRUST_FOLDER="+view.Folder)
		argv = []string{filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-NoProfile", "-Command", `Set-Location -LiteralPath $env:AM_TRUST_FOLDER; if (!$?) {exit 1}; $a=$env:AM_TRUST_ARGS | ConvertFrom-Json; & $a[0] @($a | Select-Object -Skip 1)`}
	} else {
		argv = append([]string{"/bin/sh", "-c", `cd "$1" && shift && exec "$@"`, "messenger-trust", view.Folder}, argv...)
	}
	if err = startTerminalFunc(argv, env); err != nil {
		w.WriteHeader(501)
		writeJSON(w, map[string]any{"ok": false, "detail": fmt.Sprintf("Открытие терминала не подтверждено: %v", err)})
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}
