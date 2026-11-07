//go:build !windows

package platform

import "os/exec"

// HideConsole does nothing outside Windows.
func HideConsole(*exec.Cmd) {}
