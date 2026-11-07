//go:build windows

package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// replace puts the new program in place of the running one. Windows lets a
// running program be renamed but not overwritten, so the old file moves
// aside first and is removed at the next start (RemoveLeftover).
func replace(executable, staged string) error {
	old := executable + ".old"
	_ = os.Remove(old)
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

// RemoveLeftover removes the previous program left by an update.
func RemoveLeftover(executable string) { _ = os.Remove(executable + ".old") }

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}
