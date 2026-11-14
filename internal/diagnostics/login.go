package diagnostics

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

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

// DefaultProfile is the agent's standard profile folder (T-056).
func DefaultProfile(provider string) (string, error) {
	return supervisor.DefaultProfileDir(provider)
}

// logoutArgs is each agent's own sign-out command (T-056).
var logoutArgs = map[string][]string{
	"claude": {"auth", "logout"},
	"codex":  {"logout"},
	"cursor": {"logout"},
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
	return agentCommand(loginArgs, provider, cli, environment)
}

// LogoutCommand is the agent's sign-out for the selected program and
// profile, built like LoginCommand. Nothing is run here.
func LogoutCommand(provider, cli, environment string) (argv []string, env []string, err error) {
	return agentCommand(logoutArgs, provider, cli, environment)
}

func agentCommand(table map[string][]string, provider, cli, environment string) (argv []string, env []string, err error) {
	args, ok := table[provider]
	if !ok {
		return nil, nil, fmt.Errorf("Неизвестный агент. Выберите Claude Code, Codex или Cursor.")
	}
	if !filepath.IsAbs(cli) {
		return nil, nil, fmt.Errorf("Не указан полный путь к CLI. Найдите программу в настройках агента.")
	}
	if info, err := os.Stat(cli); err != nil || info.IsDir() {
		return nil, nil, fmt.Errorf("Программа агента недоступна: %s. Нажмите «Найти» в настройках агента.", cli)
	}
	program := []string{cli}
	if provider == "cursor" {
		program = adapters.CursorProgram(cli)
		env = append(env, adapters.CursorProgramEnv(cli)...)
	}
	if environment != "" {
		canonical, err := core.CanonicalEnvironment(environment)
		if err != nil {
			// Discovery offers the default profile even before the first login.
			// Let the CLI create its own default storage; never create or switch
			// an explicit custom profile on a failed validation.
			def, defErr := DefaultProfile(provider)
			same := filepath.Clean(environment) == filepath.Clean(def)
			if runtime.GOOS == "windows" {
				same = strings.EqualFold(filepath.Clean(environment), filepath.Clean(def))
			}
			_, statErr := os.Lstat(environment)
			if defErr == nil && def != "" && same && os.IsNotExist(statErr) {
				return append(program, args...), env, nil
			}
			return nil, nil, fmt.Errorf("Каталог профиля агента недоступен: %s. Проверьте путь к профилю в настройках агента.", environment)
		}
		// Only the profile variable, when it is needed: the rest of the
		// environment is the terminal's own.
		env = append(env, supervisor.ProfileEnvironment(nil, provider, canonical)...)
	}
	return append(program, args...), env, nil
}

// InteractiveCommand opens a new CLI at its own trust prompt, with no task or
// model prompt. The owner confirms in that terminal; Messenger does not reply.
func InteractiveCommand(provider, cli, environment string) ([]string, []string, error) {
	return agentCommand(map[string][]string{"claude": {}, "codex": {}, "cursor": {}}, provider, cli, environment)
}
