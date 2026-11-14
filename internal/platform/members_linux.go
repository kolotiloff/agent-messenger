//go:build linux

package platform

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// GroupMembers lists the processes of a process group with their names.
func GroupMembers(pgid int) ([]Member, error) {
	if pgid <= 1 {
		return nil, fmt.Errorf("%w: process group was not recorded", ErrNotProven)
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var out []Member
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		proc, err := readProc(pid)
		if err != nil || proc.PGID != pgid {
			continue
		}
		name := ""
		if raw, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
			name = strings.TrimSpace(string(raw))
		}
		out = append(out, Member{PID: pid, Name: name})
	}
	return out, nil
}
