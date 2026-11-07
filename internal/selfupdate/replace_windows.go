//go:build windows

package selfupdate

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// replace puts the new program in place of the running one. Windows lets a
// running program be renamed but not overwritten, so the old file moves
// aside first and is removed at the next start (RemoveLeftover). A background
// client started from an earlier version may still run from an older
// leftover, which then cannot be removed; another name is used.
func replace(executable, staged string) error {
	old := ""
	for i := 0; i < 10 && old == ""; i++ {
		name := leftover(executable, i)
		if err := os.Remove(name); err == nil || errors.Is(err, os.ErrNotExist) {
			old = name
		}
	}
	if old == "" {
		return errors.New("старые версии программы ещё запущены; перезапустите фоновый клиент и повторите")
	}
	if err := os.Rename(executable, old); err != nil {
		return fmt.Errorf("не удалось отодвинуть старую версию: %w", err)
	}
	if err := os.Rename(staged, executable); err != nil {
		if back := os.Rename(old, executable); back != nil {
			return fmt.Errorf("новая версия не встала на место (%v), и старая не вернулась: %w", err, back)
		}
		return fmt.Errorf("новая версия не встала на место: %w", err)
	}
	return nil
}

func leftover(executable string, i int) string {
	if i == 0 {
		return executable + ".old"
	}
	return fmt.Sprintf("%s.old-%d", executable, i)
}

// RemoveLeftover removes earlier programs left by updates. One that still
// runs (an old background client) stays until a later start.
func RemoveLeftover(executable string) {
	for i := 0; i < 10; i++ {
		_ = os.Remove(leftover(executable, i))
	}
}

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}
