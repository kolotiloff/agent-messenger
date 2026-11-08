//go:build !windows

package localui

import (
	"os/exec"
	"runtime"
)

// openURL opens a fixed https page in the default browser.
func openURL(u string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "/usr/bin/open"
	}
	cmd := exec.Command(opener, u)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
