//go:build darwin

package platform

import (
	"fmt"
	"os/exec"
)

// GroupMembers lists the processes of a process group with their names.
func GroupMembers(pgid int) ([]Member, error) {
	if pgid <= 1 {
		return nil, fmt.Errorf("%w: process group was not recorded", ErrNotProven)
	}
	out, err := exec.Command("/bin/ps", "-ax", "-o", "pid=,pgid=,comm=").Output()
	if err != nil {
		return nil, err
	}
	return parseMembers(string(out), pgid), nil
}
