//go:build unix

package platform

import (
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
