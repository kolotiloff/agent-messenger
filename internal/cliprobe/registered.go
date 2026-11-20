package cliprobe

import (
	"context"
	"os"
	"path/filepath"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// RegisteredVersion runs only --version, using the same registered executable
// (including the current symlink target or Cursor install) as workers.
func RegisteredVersion(ctx context.Context, store *core.Store, provider string) (version, reason string, err error) {
	agents, err := store.ComputerAgents()
	if err != nil {
		return "", "", err
	}
	for _, a := range agents {
		if a.Provider != provider || a.Paused {
			continue
		}
		path, err := filepath.EvalSymlinks(a.CLI)
		if err != nil || !filepath.IsAbs(path) {
			return "", "executable_unavailable", nil
		}
		if info, err := os.Stat(path); err != nil || !platform.IsExecutable(path, info) {
			return "", "executable_unavailable", nil
		}
		dir, err := os.MkdirTemp("", "messenger-version-")
		if err != nil {
			return "", "probe_directory_unavailable", nil
		}
		defer os.RemoveAll(dir)
		program := []string{path}
		env := os.Environ()
		if provider == "cursor" {
			program = adapters.CursorProgram(path)
			env = append(env, adapters.CursorProgramEnv(path)...)
		}
		version, reason := Version(provider, Run(ctx, program[0], dir, env, append(program[1:], "--version")))
		return version, reason, nil
	}
	return "", "agent_unavailable", nil
}
