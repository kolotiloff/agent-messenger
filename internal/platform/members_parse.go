package platform

import (
	"path/filepath"
	"strconv"
	"strings"
)

// parseMembers reads `ps -o pid=,pgid=,comm=` lines and keeps one group.
// The command may hold spaces; only its last path element is kept.
func parseMembers(out string, pgid int) []Member {
	var members []Member
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		group, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil || group != pgid {
			continue
		}
		members = append(members, Member{PID: pid, Name: filepath.Base(strings.Join(fields[2:], " "))})
	}
	return members
}
