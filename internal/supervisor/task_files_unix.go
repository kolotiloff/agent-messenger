//go:build darwin || linux

package supervisor

import (
	"os"

	"github.com/google/safeopen"
	"golang.org/x/sys/unix"
)

func openTaskFile(root, path string) (*os.File, error) {
	// A file may have become a named pipe after the regular-file check.
	return safeopen.OpenFileBeneath(root, path, unix.O_RDONLY|unix.O_NONBLOCK, 0)
}
