//go:build !windows

package platform

import "os/exec"

// HideConsole does nothing outside Windows.
func HideConsole(*exec.Cmd) {}

// NewConsole does nothing outside Windows: a terminal program opens the
// window there.
func NewConsole(*exec.Cmd) {}
