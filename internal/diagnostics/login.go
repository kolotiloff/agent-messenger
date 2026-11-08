package diagnostics

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

// loginArgs is each agent's own sign-in command. It opens the browser and
// stores the sign-in in the agent's profile.
var loginArgs = map[string][]string{
	"claude": {"auth", "login"},
	"codex":  {"login"},
	"cursor": {"login"},
}

// InstallPages are the agents' official installation pages.
var InstallPages = map[string]string{
	"claude": "https://code.claude.com/docs/en/setup",
	"codex":  "https://developers.openai.com/codex/cli",
	"cursor": "https://cursor.com/cli",
}

// LoginCommand is the agent's sign-in for the selected program and profile:
// the program and its arguments, and the profile variable when the profile
// is not the standard one. Nothing is run here.
func LoginCommand(provider, cli, environment string) (argv []string, env []string, err error) {
	args, ok := loginArgs[provider]
	if !ok || !filepath.IsAbs(cli) {
		return nil, nil, fmt.Errorf("unknown agent or relative program path")
	}
	if info, err := os.Stat(cli); err != nil || info.IsDir() {
		return nil, nil, fmt.Errorf("program not found")
	}
	program := []string{cli}
	if provider == "cursor" {
		program = adapters.CursorProgram(cli)
		env = append(env, adapters.CursorProgramEnv(cli)...)
	}
	if environment != "" {
		canonical, err := core.CanonicalEnvironment(environment)
		if err != nil {
			return nil, nil, err
		}
		// Only the profile variable, when it is needed: the rest of the
		// environment is the terminal's own.
		env = append(env, supervisor.ProfileEnvironment(nil, provider, canonical)...)
	}
	return append(program, args...), env, nil
}
