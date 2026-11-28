package platform

import (
	"context"
	"os/exec"
)

// HoldAwake holds a logind inhibitor until the client releases it.
func HoldAwake(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "systemd-inhibit", "--what=idle", "--mode=block",
		"--who=Kotel", "--why=Kotel is keeping this computer awake", "cat")
	pipe, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer pipe.Close()
	return cmd.Run()
}
