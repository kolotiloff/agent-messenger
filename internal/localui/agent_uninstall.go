package localui

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/diagnostics"
)

// Removing an agent's CLI (owner, 2026-11-15): like installing, it runs in a
// terminal the owner sees, instead of sending them to a web page. The script
// finds the CLI, says what it will remove and asks before removing it. It
// removes only the program: the agent's sign-in and settings and
// Messenger's settings stay. A CLI from npm or Homebrew is removed with
// that tool; any other is removed as files (the launcher and, for the
// native installers, their versions folder).

// uninstallPackages are the package names the agents are published under.
var uninstallPackages = map[string]struct{ npm, brew string }{
	"claude": {"@anthropic-ai/claude-code", "claude-code"},
	"codex":  {"@openai/codex", "codex"},
	"cursor": {"", "cursor-cli"},
}

func (s *agentSetup) uninstall(w http.ResponseWriter, dec *json.Decoder) {
	var in struct {
		Provider string `json:"provider"`
	}
	if dec.Decode(&in) != nil || dec.Decode(new(any)) != io.EOF {
		http.Error(w, "invalid request", 400)
		return
	}
	if _, ok := diagnostics.InstallPages[in.Provider]; !ok {
		http.Error(w, "invalid request", 400)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.root == "" {
		http.Error(w, "setup unavailable", 503)
		return
	}
	nonce := make([]byte, 8)
	if _, err := rand.Read(nonce); err != nil {
		http.Error(w, "cannot start", 500)
		return
	}
	dir := filepath.Join(s.root, "agent-setup", in.Provider+"-uninstall-"+hex.EncodeToString(nonce))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		http.Error(w, "setup directory unavailable", 500)
		return
	}
	script, argv := uninstallScript(runtime.GOOS, in.Provider, dir)
	name := "uninstall.sh"
	if runtime.GOOS == "windows" {
		name = "uninstall.ps1"
		script = "\ufeff" + script
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o600); err != nil {
		http.Error(w, "cannot write script", 500)
		return
	}
	if err := startTerminalFunc(argv, nil); err != nil {
		w.WriteHeader(http.StatusNotImplemented)
		writeJSON(w, map[string]any{"ok": false, "detail": "Открытие терминала не подтверждено.", "hint": err.Error()})
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

// uninstallScript is fixed code; nothing in it comes from the request but
// the provider, which is checked against the known three.
func uninstallScript(goos, provider, dir string) (string, []string) {
	names := map[string]string{"claude": "claude", "codex": "codex", "cursor": "cursor-agent agent"}[provider]
	// The versions folders of the native installers, under the home folder.
	folders := map[string]string{"claude": ".local/share/claude", "codex": "", "cursor": ".local/share/cursor-agent"}[provider]
	pkg := uninstallPackages[provider]
	if goos == "windows" {
		q := func(v string) string { return "'" + strings.ReplaceAll(v, "'", "''") + "'" }
		winFolders := map[string]string{"claude": `$env:USERPROFILE\.local\share\claude`, "codex": "", "cursor": `$env:LOCALAPPDATA\cursor-agent`}[provider]
		script := `$ErrorActionPreference='Stop'
try {
 $env:PATH=$env:PATH+';'+[Environment]::GetEnvironmentVariable('PATH','User')+';'+[Environment]::GetEnvironmentVariable('PATH','Machine')
 $amCLI=''
 foreach ($name in ` + q(strings.ReplaceAll(names, " ", ",")) + `.Split(',')) {
  $c=Get-Command $name -CommandType Application,ExternalScript -ErrorAction SilentlyContinue | Select-Object -First 1
  if ($c -and [IO.Path]::IsPathRooted($c.Source)) {$amCLI=$c.Source; break}
 }
 if (!$amCLI) {Write-Host 'CLI не найден: удалять нечего.'; return}
 Write-Host ('Найден: '+$amCLI)
 $npm=` + q(pkg.npm) + `
 $folder="` + winFolders + `"
 if ($amCLI -match '\\npm\\|node_modules' -and $npm) {Write-Host ('Будет выполнено: npm uninstall -g '+$npm)}
 else {Write-Host ('Будет удалён файл: '+$amCLI); if ($folder -and (Test-Path -LiteralPath $folder)) {Write-Host ('и папка версий: '+$folder)}}
 Write-Host 'Вход в агент, его настройки и настройки Messenger останутся.'
 $a=Read-Host 'Удалить? (y/N)'
 if ($a -ne 'y' -and $a -ne 'Y' -and $a -ne 'д' -and $a -ne 'Д') {Write-Host 'Отменено, ничего не удалено.'; return}
 if ($amCLI -match '\\npm\\|node_modules' -and $npm) {& npm uninstall -g $npm; if ($LASTEXITCODE -ne 0) {throw 'npm uninstall failed'}}
 else {
  Remove-Item -LiteralPath $amCLI -Force
  if ($folder -and (Test-Path -LiteralPath $folder)) {Remove-Item -LiteralPath $folder -Recurse -Force}
 }
 Write-Host 'Готово: CLI удалён. Вернитесь в Messenger.'
} catch { Write-Host ('Не удалось удалить: '+$_) }
`
		_ = dir
		return script, []string{filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe"), "-ExecutionPolicy", "Bypass", "-File", filepath.Join(dir, "uninstall.ps1")}
	}
	q := func(v string) string { return shellLine([]string{v}, nil) }
	script := `#!/bin/sh
find_agent() {
 for am_name in ` + names + `; do
  am_path=$(command -v "$am_name" 2>/dev/null) || am_path=''
  case "$am_path" in /*) if [ -f "$am_path" ]; then printf '%s' "$am_path"; return; fi;; esac
  for am_base in "$HOME/.local/bin" /opt/homebrew/bin /usr/local/bin; do
   am_path="$am_base/$am_name"
   if [ -f "$am_path" ]; then printf '%s' "$am_path"; return; fi
  done
 done
}
# The file a chain of links ends at (macOS has no readlink -f everywhere).
real_path() {
 am_p=$1; am_n=0
 while [ -L "$am_p" ] && [ "$am_n" -lt 20 ]; do
  am_t=$(readlink "$am_p")
  case "$am_t" in /*) am_p=$am_t;; *) am_p=$(dirname "$am_p")/$am_t;; esac
  am_n=$((am_n + 1))
 done
 printf '%s' "$am_p"
}
am_cli=$(find_agent)
if [ -z "$am_cli" ]; then printf 'CLI не найден: удалять нечего.\n'; exit 0; fi
am_real=$(real_path "$am_cli")
am_npm=` + q(pkg.npm) + `
am_brew=` + q(pkg.brew) + `
am_folder=''
if [ -n ` + q(folders) + ` ]; then am_folder="$HOME/"` + q(folders) + `; fi
case "$am_real" in
 */node_modules/*) am_how=npm ;;
 */Cellar/*|*/Caskroom/*|/opt/homebrew/*) am_how=brew ;;
 *) am_how=files ;;
esac
if [ "$am_how" = npm ] && [ -z "$am_npm" ]; then am_how=files; fi
printf 'Найден: %s\n' "$am_cli"
case "$am_how" in
 npm) printf 'Будет выполнено: npm uninstall -g %s\n' "$am_npm" ;;
 brew) printf 'Будет выполнено: brew uninstall %s\n' "$am_brew" ;;
 files) printf 'Будет удалён файл: %s\n' "$am_cli"
  if [ "$am_real" != "$am_cli" ]; then printf 'и файл, на который он ссылается: %s\n' "$am_real"; fi
  if [ -n "$am_folder" ] && [ -d "$am_folder" ]; then printf 'и папка версий: %s\n' "$am_folder"; fi ;;
esac
printf 'Вход в агент, его настройки и настройки Messenger останутся.\n'
printf 'Удалить? (y/N) '
read am_answer
case "$am_answer" in y|Y|д|Д) ;; *) printf 'Отменено, ничего не удалено.\n'; exit 0;; esac
case "$am_how" in
 npm) npm uninstall -g "$am_npm" || { printf 'Не удалось удалить через npm.\n'; exit 1; } ;;
 brew) brew uninstall "$am_brew" || { printf 'Не удалось удалить через Homebrew.\n'; exit 1; } ;;
 files)
  rm -f "$am_cli" || { printf 'Не удалось удалить %s.\n' "$am_cli"; exit 1; }
  case "$am_real" in "$HOME"/*) if [ "$am_real" != "$am_cli" ]; then rm -f "$am_real"; fi;; esac
  if [ -n "$am_folder" ] && [ -d "$am_folder" ]; then rm -rf "$am_folder"; fi ;;
esac
printf '\nГотово: CLI удалён. Вернитесь в Messenger.\n'
`
	return script, []string{"/bin/sh", filepath.Join(dir, "uninstall.sh")}
}
