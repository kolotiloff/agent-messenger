//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// openInBrowser opens the file with the desktop's default program for .html.
// Only the file path is passed, never the page key.
func openInBrowser(path string) error {
	opener := "xdg-open"
	if runtime.GOOS == "darwin" {
		opener = "/usr/bin/open"
	}
	cmd := exec.Command(opener, path)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
