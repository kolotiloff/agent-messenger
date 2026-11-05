//go:build windows

package platform

import (
	"fmt"
	"os/exec"
)

func SetNewGroup(cmd *exec.Cmd) error {
	return fmt.Errorf("%w: a new process group has no tested Windows implementation", ErrUnsupported)
}
