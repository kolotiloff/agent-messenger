package mcp

import "runtime"

// exeName names a fake agent program so the store accepts it as a program:
// on Windows only a .exe file is one.
func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
