//go:build !windows

package selfupdate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// replace renames the new program over the running one in one step.
func replace(executable, staged string) error {
	if err := os.Rename(staged, executable); err != nil {
		return fmt.Errorf("новая версия не встала на место: %w", err)
	}
	d, err := os.Open(filepath.Dir(executable))
	if err != nil {
		return nil
	}
	defer d.Close()
	_ = d.Sync()
	return nil
}

// RemoveLeftover has nothing to remove outside Windows.
func RemoveLeftover(string) {}

func hideWindow(*exec.Cmd) {}
