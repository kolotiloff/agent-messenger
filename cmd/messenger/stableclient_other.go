//go:build !windows

package main

// startPinnedClient starts the client from its pinned copy. Only Windows
// needs a stable path (D-118).
func startPinnedClient(root, pinned, _ string) error {
	_, _, err := startBackgroundAt(root, []string{"--claude-stream", "--allow-tasks"}, pinned)
	return err
}
