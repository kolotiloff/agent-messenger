package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/localui"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// runOpen is the window on Linux and Windows (D-045): the same page the Mac
// app shows, opened in the default browser. It starts the background client
// the way the Mac window does. The page's address carries a one-time key, so
// the browser is pointed at a private file in the data folder that forwards
// to it: the key never appears in another program's command line.
func runOpen(root string, args []string, out io.Writer) error {
	if len(args) != 0 {
		return errors.New("open takes no arguments")
	}
	if root == "" {
		var err error
		root, err = supervisor.DesktopRoot()
		if err != nil {
			return fmt.Errorf("не удалось выбрать сохранённые данные: %w", err)
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if err := ensureDesktopClient(ctx, root); err != nil {
		return err
	}
	removeOldWindowFiles(root)
	home, err := platform.DefaultRoot()
	if err != nil {
		return err
	}
	desktop := localui.Desktop{Restart: desktopRestart(root)}
	var next relaunch
	if runtime.GOOS == "darwin" {
		desktop.Channel = &updatefeed.Channel{Directory: filepath.Join(home, "updates"), PublicKey: buildinfo.UpdatePublicKey}
	} else {
		desktop.SelfUpdate = windowSelfUpdate(ctx, home, cancel, &next)
	}
	opener := &windowOpener{root: root, out: out}
	err = localui.RunWindow(ctx, root, 0, opener, desktop)
	opener.cleanup()
	if program := next.get(); program != "" && err == nil {
		fmt.Fprintln(out, "Программа обновлена. Окно открывается заново.")
		return startAgain(program)
	}
	return err
}

// windowOpener receives the page address from the server and opens it once.
type windowOpener struct {
	root string
	out  io.Writer
	mu   sync.Mutex
	file string
	done bool
}

func (o *windowOpener) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.done {
		return len(p), nil
	}
	o.done = true
	address := strings.TrimSpace(string(p))
	if !strings.HasPrefix(address, "http://127.0.0.1:") {
		return 0, errors.New("unexpected window address")
	}
	file, err := writeWindowFile(o.root, address)
	if err != nil {
		return 0, err
	}
	o.file = file
	opened := openInBrowser(file)
	fmt.Fprintln(o.out, "Agent Messenger открыт в браузере.")
	if opened != nil {
		fmt.Fprintf(o.out, "Браузер не открылся сам. Откройте в браузере этот файл:\n%s\n", file)
	}
	fmt.Fprintln(o.out, "Страница работает, пока открыто это окно. Закройте его или нажмите Ctrl+C, чтобы закрыть страницу. Клиент и задачи продолжат работать в фоне.")
	return len(p), nil
}

func (o *windowOpener) cleanup() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.file != "" {
		_ = os.Remove(o.file)
	}
}

const windowFilePattern = "window-*.html"

// writeWindowFile writes a new 0600 file that forwards to address. A new
// name each time: an existing file or link is never followed or reused.
func writeWindowFile(root, address string) (string, error) {
	f, err := os.CreateTemp(root, windowFilePattern)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if err := f.Chmod(0o600); err != nil && runtime.GOOS != "windows" {
		os.Remove(f.Name())
		return "", err
	}
	quoted := html.EscapeString(address)
	page := `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta http-equiv="refresh" content="0;url=` + quoted +
		`"><title>Agent Messenger</title></head><body><a href="` + quoted + `">Открыть Agent Messenger</a></body></html>`
	w := bufio.NewWriter(f)
	if _, err := w.WriteString(page); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	if err := w.Flush(); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// removeOldWindowFiles drops forwarding files of earlier windows. Their keys
// ended with those windows.
func removeOldWindowFiles(root string) {
	matches, _ := filepath.Glob(filepath.Join(root, windowFilePattern))
	for _, m := range matches {
		if info, err := os.Lstat(m); err == nil && info.Mode().IsRegular() {
			_ = os.Remove(m)
		}
	}
}
