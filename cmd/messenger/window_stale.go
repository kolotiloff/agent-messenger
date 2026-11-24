package main

import (
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

// T-209: an old window kept running in the tray after an update and the
// menu brought it back instead of the new version. A window started from an
// older installed version is stale: the new one closes it and opens itself.

var previewVersion = regexp.MustCompile(`^(?:0\.0\.0|[1-9][0-9]*\.[0-9]+\.[0-9]+)-preview\.(\d+)$`)

func previewNumber(version string) (int, bool) {
	m := previewVersion.FindStringSubmatch(version)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	return n, err == nil
}

// olderWindowProgram says whether the program file of a running window,
// other, is an installed version older than this one. A file that is not an
// installation's version, or a version this code cannot read, is not older.
func olderWindowProgram(other, own, ownVersion string) bool {
	other = strings.TrimSuffix(other, " (deleted)")
	if other == "" || filepath.Clean(other) == filepath.Clean(own) || supervisor.InstallPrefix(other) == "" {
		return false
	}
	theirs, ok := previewNumber(filepath.Base(filepath.Dir(other)))
	if !ok {
		return false
	}
	ours, ok := previewNumber(ownVersion)
	return ok && theirs < ours
}
