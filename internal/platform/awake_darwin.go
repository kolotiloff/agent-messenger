package platform

import (
	"context"
	"os"
	"os/exec"
	"strconv"
)

// HoldAwake inhibits automatic idle sleep for as long as ctx is active.
func HoldAwake(ctx context.Context) error {
	return exec.CommandContext(ctx, "/usr/bin/caffeinate", "-i", "-w", strconv.Itoa(os.Getpid())).Run()
}
