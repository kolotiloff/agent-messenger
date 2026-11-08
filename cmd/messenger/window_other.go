//go:build !windows

package main

import (
	"context"
	"github.com/kolotiloff/agent-messenger/internal/localui"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// appBrowsers can show a page as a separate window without tabs or an
// address bar (--app). The first one installed is used.
var appBrowsers = []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "microsoft-edge-stable", "brave-browser"}

// showWindow shows the page in a window of its own on Linux (D-054): a
// Chromium-based browser in app mode with Messenger's own profile, so the
// window has no tabs or address bar and closing it closes the page. Only the
// private forwarding file is passed, never the page's key. Without such a
// browser the page opens in the default browser as before.
func showWindow(ctx context.Context, cancel context.CancelFunc, home string, page windowPage, out io.Writer) {
	if runtime.GOOS == "linux" {
		installMenuEntry()
		// Each window has a profile of its own: a browser started with a
		// profile that is already open hands the page over and exits at once,
		// which would look like a closed window.
		profile, err := os.MkdirTemp(filepath.Join(home), "window-profile-")
		if err == nil {
			defer os.RemoveAll(profile)
			if cmd := appWindowCommand(profile, page.file); cmd != nil && cmd.Start() == nil {
				started := time.Now()
				closed := make(chan error, 1)
				go func() { closed <- cmd.Wait() }()
				select {
				case err := <-closed:
					// A browser that failed right away showed nothing.
					if err == nil || time.Since(started) > 5*time.Second {
						return
					}
				case <-ctx.Done():
					_ = cmd.Process.Kill()
					<-closed
					return
				}
			}
		}
	}
	showInBrowser(ctx, page, out)
}

func appWindowCommand(profile, file string) *exec.Cmd {
	for _, name := range appBrowsers {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		page := url.URL{Scheme: "file", Path: filepath.ToSlash(file)}
		return exec.Command(path, "--app="+page.String(), "--user-data-dir="+profile,
			"--no-first-run", "--no-default-browser-check", "--class=AgentMessenger", "--window-size=1280,820")
	}
	return nil
}

// installMenuEntry puts Agent Messenger into the desktop's application menu
// (~/.local/share/applications), pointing at this program, so it starts like
// any other program. The file is rewritten only when it differs.
func installMenuEntry() {
	exe, err := os.Executable()
	if err != nil || strings.ContainsAny(exe, "\"\n\\`$%") {
		return
	}
	dir := os.Getenv("XDG_DATA_HOME")
	if !filepath.IsAbs(dir) {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		dir = filepath.Join(home, ".local", "share")
	}
	// The icon goes into the user's icon theme folder; the entry names it by
	// its full path, which every desktop reads without an icon cache.
	icon := filepath.Join(dir, "icons", "hicolor", "scalable", "apps", "agent-messenger.svg")
	iconLine := ""
	if data := localui.AppIcon(); len(data) > 0 && !strings.ContainsAny(icon, "\n") {
		if old, err := os.ReadFile(icon); err == nil && string(old) == string(data) {
			iconLine = "Icon=" + icon + "\n"
		} else if os.MkdirAll(filepath.Dir(icon), 0o755) == nil && os.WriteFile(icon, data, 0o644) == nil {
			iconLine = "Icon=" + icon + "\n"
		}
	}
	dir = filepath.Join(dir, "applications")
	entry := "[Desktop Entry]\nType=Application\nName=Agent Messenger\nComment=Задачи для агентов\nExec=\"" + exe + "\" open\n" + iconLine + "Terminal=false\nCategories=Development;\nStartupWMClass=AgentMessenger\n"
	path := filepath.Join(dir, "agent-messenger.desktop")
	if old, err := os.ReadFile(path); err == nil && string(old) == entry {
		return
	}
	if os.MkdirAll(dir, 0o755) == nil {
		_ = os.WriteFile(path, []byte(entry), 0o644)
	}
}
