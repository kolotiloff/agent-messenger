//go:build unix

package platform

import (
	"errors"
	"os/exec"
	"syscall"
)

// SetNewGroup places the command in its own process group.
// Cancelling a CommandContext is not a substitute for this and is not proof of death.
func SetNewGroup(cmd *exec.Cmd) error {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	return nil
}

// StartGroup starts a command prepared by SetNewGroup. On Unix the group
// exists from the first instruction, so this is a plain start.
func StartGroup(cmd *exec.Cmd) error { return cmd.Start() }

// NeverRan reports whether a failed start proves the executable never ran:
// the OS refused to create the process, or StartGroup stopped it before its
// first instruction.
func NeverRan(cmd *exec.Cmd, err error) bool {
	if errors.Is(err, ErrNotStarted) {
		return true
	}
	return cmd.Process == nil && (errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.ENOENT) ||
		errors.Is(err, syscall.ENOEXEC) || errors.Is(err, syscall.ENOTDIR) || errors.Is(err, exec.ErrNotFound))
}
