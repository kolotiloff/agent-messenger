package platform

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsePS reads `ps -o pid=,pgid=,lstart=` lines. Any unparsable line fails closed.
func ParsePS(out string) ([]Proc, error) {
	var live []Proc
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, fmt.Errorf("%w: unexpected ps line %q", ErrNotProven, line)
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("%w: pid in %q", ErrNotProven, line)
		}
		pgid, err := strconv.Atoi(fields[1])
		if err != nil {
			return nil, fmt.Errorf("%w: pgid in %q", ErrNotProven, line)
		}
		live = append(live, Proc{PID: pid, PGID: pgid, StartToken: strings.Join(fields[2:], " ")})
	}
	if len(live) == 0 {
		return nil, fmt.Errorf("%w: ps returned no processes", ErrNotProven)
	}
	return live, nil
}

// ParsePSTree reads `ps -o pid=,ppid=,pgid=,lstart=` lines. Any unparsable line
// fails closed.
func ParsePSTree(out string) ([]Proc, error) {
	var live []Proc
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			return nil, fmt.Errorf("%w: unexpected ps line %q", ErrNotProven, line)
		}
		var nums [3]int
		for i := range nums {
			n, err := strconv.Atoi(fields[i])
			if err != nil {
				return nil, fmt.Errorf("%w: number in %q", ErrNotProven, line)
			}
			nums[i] = n
		}
		live = append(live, Proc{PID: nums[0], PPID: nums[1], PGID: nums[2], StartToken: strings.Join(fields[3:], " ")})
	}
	if len(live) == 0 {
		return nil, fmt.Errorf("%w: ps returned no processes", ErrNotProven)
	}
	return live, nil
}
