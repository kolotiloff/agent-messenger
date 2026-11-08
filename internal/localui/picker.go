package localui

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// The window's «Выбрать…» buttons (D-053). A page in a browser cannot learn
// a chosen file's full path, so the local server, which runs on the same
// computer, opens the system's own dialog and returns the path. Nothing is
// started or changed by the choice: the path only fills the field.

// pickFunc opens the dialog; tests replace it.
var pickFunc = pickPath

// pickerDeadline is how long the owner may keep the dialog open.
const pickerDeadline = 10 * time.Minute

var errNoPicker = errors.New("no file dialog on this computer")

// pickCommand is the dialog program for a kind ("file" or "folder"). The
// start folder travels in an environment variable, never in the script.
func pickCommand(goos, kind, start string) (*exec.Cmd, error) {
	folder := kind == "folder"
	switch goos {
	case "windows":
		root := os.Getenv("SystemRoot")
		if !filepath.IsAbs(root) {
			return nil, errNoPicker
		}
		script := `[Console]::OutputEncoding=[Text.Encoding]::UTF8
Add-Type -AssemblyName System.Windows.Forms
$owner=New-Object System.Windows.Forms.Form -Property @{TopMost=$true}
`
		if folder {
			script += `$d=New-Object System.Windows.Forms.FolderBrowserDialog
if($env:AM_PICK_START){$d.SelectedPath=$env:AM_PICK_START}
if($d.ShowDialog($owner) -eq 'OK'){[Console]::Out.Write($d.SelectedPath)}`
		} else {
			script += `$d=New-Object System.Windows.Forms.OpenFileDialog
$d.Filter='Программы (*.exe)|*.exe|Все файлы (*.*)|*.*'
if($env:AM_PICK_START){$d.InitialDirectory=$env:AM_PICK_START}
if($d.ShowDialog($owner) -eq 'OK'){[Console]::Out.Write($d.FileName)}`
		}
		cmd := exec.Command(filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe"),
			"-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script)
		cmd.Env = append(os.Environ(), "AM_PICK_START="+start)
		platform.HideConsole(cmd)
		return cmd, nil
	case "darwin":
		choose := "choose file"
		if folder {
			choose = "choose folder"
		}
		script := `set p to system attribute "AM_PICK_START"
if p is "" then
	return POSIX path of (` + choose + `)
end if
return POSIX path of (` + choose + ` default location (POSIX file p))`
		cmd := exec.Command("/usr/bin/osascript", "-e", script)
		cmd.Env = append(os.Environ(), "AM_PICK_START="+start)
		return cmd, nil
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			args := []string{"--file-selection", "--title=Agent Messenger"}
			if folder {
				args = append(args, "--directory")
			}
			if start != "" {
				args = append(args, "--filename="+strings.TrimSuffix(start, "/")+"/")
			}
			return exec.Command(path, args...), nil
		}
		if path, err := exec.LookPath("kdialog"); err == nil {
			mode := "--getopenfilename"
			if folder {
				mode = "--getexistingdirectory"
			}
			if start == "" {
				start, _ = os.UserHomeDir()
			}
			return exec.Command(path, mode, start), nil
		}
		return nil, errNoPicker
	}
}

// pickPath opens the dialog and returns the chosen absolute path, or "" when
// the owner closed it without a choice.
func pickPath(ctx context.Context, kind, start string) (string, error) {
	cmd, err := pickCommand(runtime.GOOS, kind, start)
	if err != nil {
		return "", err
	}
	run := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)
	run.Env, run.SysProcAttr = cmd.Env, cmd.SysProcAttr
	run.WaitDelay = time.Second
	var out bytes.Buffer
	run.Stdout = &out
	err = run.Run()
	path := strings.TrimSpace(out.String())
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if path == "" || !filepath.IsAbs(path) {
		// A closed dialog exits with an error and prints nothing.
		if err != nil && !errors.As(err, new(*exec.ExitError)) {
			return "", err
		}
		return "", nil
	}
	return filepath.Clean(path), nil
}

// pickStart is the folder a dialog opens in: the field's folder when it
// exists.
func pickStart(current string) string {
	if !filepath.IsAbs(current) {
		return ""
	}
	if info, err := os.Stat(current); err == nil && info.IsDir() {
		return current
	}
	if info, err := os.Stat(filepath.Dir(current)); err == nil && info.IsDir() {
		return filepath.Dir(current)
	}
	return ""
}

type pickRequest struct {
	Kind    string `json:"kind"`
	Current string `json:"current"`
}

var pickBusy sync.Mutex

func servePick(w http.ResponseWriter, r *http.Request, decoder *json.Decoder, pick func(context.Context, string, string) (string, error)) {
	var req pickRequest
	if err := decoder.Decode(&req); err != nil || (req.Kind != "file" && req.Kind != "folder") || len(req.Current) > 4096 {
		http.Error(w, "invalid request", 400)
		return
	}
	if !pickBusy.TryLock() {
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]any{"ok": false, "detail": "Окно выбора уже открыто."})
		return
	}
	defer pickBusy.Unlock()
	path, err := pick(r.Context(), req.Kind, pickStart(req.Current))
	if errors.Is(err, errNoPicker) {
		w.WriteHeader(http.StatusNotImplemented)
		writeJSON(w, map[string]any{"ok": false, "detail": "На этом компьютере нет окна выбора файлов. Вставьте путь вручную."})
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		writeJSON(w, map[string]any{"ok": false, "detail": "Окно выбора не открылось. Вставьте путь вручную."})
		return
	}
	writeJSON(w, map[string]any{"ok": true, "path": path})
}
