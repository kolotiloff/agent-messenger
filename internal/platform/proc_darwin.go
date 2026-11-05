//go:build darwin

package platform

import (
	"fmt"
	"os/exec"
	"strings"
	"syscall"
)

func Snapshot() (string, []Proc, error) {
	bootOut, err := exec.Command("sysctl", "-n", "kern.boottime").Output()
	if err != nil {
		return "", nil, fmt.Errorf("%w: boot time: %v", ErrNotProven, err)
	}
	psOut, err := exec.Command("ps", "-ax", "-o", "pid=,pgid=,lstart=").Output()
	if err != nil {
		return "", nil, fmt.Errorf("%w: ps: %v", ErrNotProven, err)
	}
	live, err := ParsePS(string(psOut))
	if err != nil {
		return "", nil, err
	}
	return strings.TrimSpace(string(bootOut)), live, nil
}

func ReadObservation(pid int) (Observation, error) {
	boot, live, err := Snapshot()
	if err != nil {
		return Observation{}, err
	}
	for _, proc := range live {
		if proc.PID == pid {
			return Observation{PID: proc.PID, PGID: proc.PGID, StartToken: proc.StartToken, BootID: boot}, nil
		}
	}
	return Observation{}, fmt.Errorf("%w: pid %d is not visible", ErrNotProven, pid)
}

func GroupEmpty(pgid int) (bool, error) {
	if pgid <= 1 {
		return false, fmt.Errorf("%w: process group was not recorded", ErrNotProven)
	}
	_, live, err := Snapshot()
	if err != nil {
		return false, err
	}
	for _, proc := range live {
		if proc.PGID == pgid {
			return false, nil
		}
	}
	return true, nil
}

func SignalGroup(pgid int) error {
	if pgid <= 1 {
		return fmt.Errorf("%w: process group was not recorded", ErrNotProven)
	}
	err := syscall.Kill(-pgid, syscall.SIGTERM)
	if err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}
