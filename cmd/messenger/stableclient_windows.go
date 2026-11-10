//go:build windows

package main

import (
	"errors"

	"github.com/kolotiloff/agent-messenger/internal/distribution"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

// startPinnedClient starts the client from the stable runtime file (D-118):
// one path for every version, so the firewall permission the owner gave
// once stays valid after updates. The client's workers still start the
// pinned copy. When another start holds the stable file, this one starts
// the pinned copy, as before D-118.
func startPinnedClient(root, pinned, cache string) error {
	stable, release, err := distribution.StableRuntime(pinned, cache)
	if errors.Is(err, distribution.ErrStableBusy) {
		_, _, err = startBackgroundAt(root, []string{"--claude-stream", "--allow-tasks"}, pinned)
		return err
	}
	if err != nil {
		return err
	}
	defer release()
	_, _, err = startBackgroundEnv(root, []string{"--claude-stream", "--allow-tasks"}, stable, []string{supervisor.PinnedRuntimeVariable + "=" + pinned})
	return err
}
