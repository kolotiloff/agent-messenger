package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// DefaultRoot is the per-user client directory. Tests pass an explicit root instead.
func DefaultRoot() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "AgentMessenger"), nil
	case "linux":
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return filepath.Join(dir, "agent-messenger"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "agent-messenger"), nil
	case "windows":
		dir := os.Getenv("AppData")
		if dir == "" {
			return "", fmt.Errorf("%w: AppData is not set", ErrUnsupported)
		}
		return filepath.Join(dir, "AgentMessenger"), nil
	default:
		return "", fmt.Errorf("%w: no default data directory for %s", ErrUnsupported, runtime.GOOS)
	}
}
