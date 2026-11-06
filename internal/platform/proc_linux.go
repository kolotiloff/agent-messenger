//go:build linux

package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
)

func Snapshot() (string, []Proc, error) {
	boot, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return "", nil, fmt.Errorf("%w: boot id: %v", ErrNotProven, err)
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return "", nil, fmt.Errorf("%w: %v", ErrNotProven, err)
	}
	var live []Proc
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		proc, err := readProc(pid)
		if err != nil {
			continue
		}
		live = append(live, proc)
	}
	return strings.TrimSpace(string(boot)), live, nil
}

// ProcessTree is Snapshot; on Linux it already carries parent links.
func ProcessTree() (string, []Proc, error) { return Snapshot() }

func ReadObservation(pid int) (Observation, error) {
	boot, _, err := Snapshot()
	if err != nil {
		return Observation{}, err
	}
	proc, err := readProc(pid)
	if err != nil {
		return Observation{}, fmt.Errorf("%w: %v", ErrNotProven, err)
	}
	return Observation{PID: proc.PID, PGID: proc.PGID, StartToken: proc.StartToken, BootID: boot}, nil
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
	if err != nil && !errorsIsESRCH(err) {
		return err
	}
	return nil
}

func errorsIsESRCH(err error) bool {
	return err == syscall.ESRCH
}

func readProc(pid int) (Proc, error) {
	raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return Proc{}, err
	}
	text := string(raw)
	closeParen := strings.LastIndex(text, ")")
	if closeParen < 0 || closeParen+2 >= len(text) {
		return Proc{}, fmt.Errorf("unreadable stat")
	}
	fields := strings.Fields(text[closeParen+2:])
	// After comm: state, ppid, pgrp, session, tty, tpgid, flags, minflt, cminflt,
	// majflt, cmajflt, utime, stime, cutime, cstime, priority, nice, num_threads,
	// itrealvalue, starttime.
	if len(fields) < 20 {
		return Proc{}, fmt.Errorf("short stat")
	}
	pgid, err := strconv.Atoi(fields[2])
	if err != nil {
		return Proc{}, err
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return Proc{}, err
	}
	return Proc{PID: pid, PGID: pgid, StartToken: fields[19], PPID: ppid}, nil
}
