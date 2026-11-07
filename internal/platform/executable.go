package platform

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
)

// IsExecutable reports whether a file can be started directly as an agent
// program. On Windows only .exe files qualify: a .cmd or .bat script runs
// through cmd.exe, whose own parsing of arguments would change the JSON and
// paths Messenger passes (D-044).
func IsExecutable(path string, info fs.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Ext(path), ".exe")
	}
	return info.Mode().Perm()&0o111 != 0
}
