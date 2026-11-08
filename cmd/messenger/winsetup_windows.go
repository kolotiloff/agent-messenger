//go:build windows

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

const winUninstallKey = `Software\Microsoft\Windows\CurrentVersion\Uninstall\` + winUninstallID

func winPathsNow() (winPaths, bool) {
	return winInstallPaths(os.Getenv("LOCALAPPDATA"), os.Getenv("APPDATA"))
}

func winInstalledVersion() string {
	k, err := registry.OpenKey(registry.CURRENT_USER, winUninstallKey, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer k.Close()
	v, _, _ := k.GetStringValue("DisplayVersion")
	return v
}

// selfInstall runs before a double-click start opens the window. It returns
// true when it started the installed copy and this program should end. Any
// failure leaves the program working from where it is.
func selfInstall(out io.Writer) bool {
	p, ok := winPathsNow()
	if !ok {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	_, statErr := os.Stat(p.Exe)
	switch winInstallAction(exe, p.Exe, buildinfo.Version, winInstalledVersion(), statErr == nil) {
	case "":
		// The installed copy: keep the entry's version current after an
		// update and bring back a removed shortcut.
		if strings.EqualFold(filepath.Clean(exe), filepath.Clean(p.Exe)) && winInstalledVersion() != "" {
			winRegister(p)
		}
		return false
	case "open-installed":
		return winStart(p) == nil
	}
	fmt.Fprintln(out, "Устанавливаем Agent Messenger в", p.Dir)
	if err := winCopy(exe, p); err != nil {
		fmt.Fprintln(out, "Не удалось установить, программа откроется отсюда:", err)
		return false
	}
	winRegister(p)
	return winStart(p) == nil
}

// winCopy copies the running program into the programs folder; an installed
// copy that runs (the background client) is moved aside, as an update does.
func winCopy(exe string, p winPaths) error {
	if err := os.MkdirAll(p.Dir, 0o755); err != nil {
		return err
	}
	selfupdate.RemoveLeftover(p.Exe)
	in, err := os.Open(exe)
	if err != nil {
		return err
	}
	defer in.Close()
	temp, err := os.CreateTemp(p.Dir, ".install-*.exe")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	_, err = io.Copy(temp, in)
	if err == nil {
		err = temp.Sync()
	}
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if _, err := os.Stat(p.Exe); errors.Is(err, os.ErrNotExist) {
		return os.Rename(temp.Name(), p.Exe)
	}
	return selfupdate.Replace(p.Exe, temp.Name())
}

// winRegister writes the «Приложения» entry and the Start menu shortcut.
// Both are conveniences: a failure leaves the program working.
func winRegister(p winPaths) {
	if k, _, err := registry.CreateKey(registry.CURRENT_USER, winUninstallKey, registry.SET_VALUE); err == nil {
		for name, value := range winUninstallValues(p, buildinfo.Version) {
			_ = k.SetStringValue(name, value)
		}
		_ = k.SetDWordValue("NoModify", 1)
		_ = k.SetDWordValue("NoRepair", 1)
		if st, err := os.Stat(p.Exe); err == nil {
			_ = k.SetDWordValue("EstimatedSize", uint32(st.Size()/1024))
		}
		k.Close()
	}
	if _, err := os.Stat(p.Shortcut); err == nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(p.Shortcut), 0o755)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", winShortcutScript(p))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	_ = cmd.Run()
}

func winStart(p winPaths) error {
	cmd := exec.Command(p.Exe)
	cmd.Dir = p.Dir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_CONSOLE}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// runUninstall is «Удалить» in «Приложения». It refuses while any CLI
// reservation exists, stops the background client, removes the shortcut,
// the entry and the program folder, and keeps the data folder.
func runUninstall(root string, out io.Writer) error {
	p, ok := winPathsNow()
	if !ok {
		return errors.New("не найдена папка программ пользователя")
	}
	if n, err := supervisor.ReservedRuns(); err != nil {
		return fmt.Errorf("не удалось проверить запущенные задачи, ничего не удалено: %w", err)
	} else if n > 0 {
		return fmt.Errorf("сейчас выполняются задачи (%d). Дождитесь их окончания и повторите удаление; ничего не удалено", n)
	}
	// Stop the background client; it may not be running.
	_, _ = control.Call(root, control.Request{Cmd: "shutdown"})
	var err error
	for i := 0; i < 30; i++ {
		err = supervisor.WithInstallationLease(func() error {
			if err := registry.DeleteKey(registry.CURRENT_USER, winUninstallKey); err != nil && !errors.Is(err, registry.ErrNotExist) {
				return err
			}
			if err := os.Remove(p.Shortcut); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			return nil
		})
		if !errors.Is(err, platform.ErrAlreadyRunning) {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if errors.Is(err, platform.ErrAlreadyRunning) {
		return errors.New("фоновый клиент не остановился; закройте окно Messenger и повторите удаление. Ничего не удалено")
	}
	if err != nil {
		return fmt.Errorf("ничего не удалено: %w", err)
	}
	data, _ := platform.DefaultRoot()
	if line, ok := winRemoveCommand(p.Dir); ok {
		cmd := exec.Command("cmd.exe")
		cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line, HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW | windows.DETACHED_PROCESS}
		cmd.Dir = os.TempDir()
		if cmd.Start() == nil {
			_ = cmd.Process.Release()
		} else {
			fmt.Fprintln(out, "Папку программы удалите вручную:", p.Dir)
		}
	} else {
		fmt.Fprintln(out, "Папку программы удалите вручную:", p.Dir)
	}
	fmt.Fprintln(out, "Agent Messenger удалён.")
	if data != "" {
		fmt.Fprintln(out, "Данные остались в папке", data, "— задачи, история и настройки. Если они больше не нужны, удалите папку вручную.")
	}
	return nil
}

// runWinInstall is `install` on Windows: the same installation as a double
// click, without opening the window.
func runWinInstall(out io.Writer) error {
	p, ok := winPathsNow()
	if !ok {
		return errors.New("не найдена папка программ пользователя")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Clean(exe), filepath.Clean(p.Exe)) {
		if err := winCopy(exe, p); err != nil {
			return err
		}
	}
	winRegister(p)
	fmt.Fprintln(out, "Installed:", p.Exe)
	return nil
}
