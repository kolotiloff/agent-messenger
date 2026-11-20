package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/cliprobe"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
)

type CheckState struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	Value  string `json:"value,omitempty"`
}

type CLICheck struct {
	SchemaVersion  int        `json:"schema_version"`
	CheckedAt      string     `json:"checked_at"`
	Provider       string     `json:"provider"`
	CLI            string     `json:"cli"`
	Environment    string     `json:"environment"`
	OS             string     `json:"os"`
	Version        CheckState `json:"version"`
	Authentication CheckState `json:"authentication"`
	Session        string     `json:"session"`
	Compatibility  string     `json:"messenger_compatibility"`
	AuthScope      string     `json:"auth_scope"`
	ProbeAttempts  int        `json:"probe_command_attempts"`
	// T-139: the version this release was checked against and how the
	// installed one compares ("tested", "newer", "older"; empty: unknown).
	TestedVersion string `json:"tested_version,omitempty"`
	Tested        string `json:"tested,omitempty"`
}

type probeRunner func(context.Context, string, string, []string, []string) cliprobe.Output

func CheckCLI(ctx context.Context, provider, cli, environment string) (CLICheck, error) {
	return checkCLI(ctx, provider, cli, environment, runtime.GOOS, cliprobe.Run)
}

func checkCLI(ctx context.Context, provider, cli, environment, goos string, runner probeRunner) (CLICheck, error) {
	r := CLICheck{SchemaVersion: 1, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), Provider: provider, OS: goos,
		Version: CheckState{State: "unknown"}, Authentication: CheckState{State: "not_checked"},
		Session: "not_checked", Compatibility: "not_checked", AuthScope: "selected_profile_and_inherited_environment"}
	if !core.TaskProviders[provider] || !filepath.IsAbs(cli) || !filepath.IsAbs(environment) {
		return r, fmt.Errorf("нужны provider claude|codex|cursor и явные абсолютные пути --cli и --environment")
	}
	canonical, err := core.CanonicalEnvironment(environment)
	if err != nil {
		return r, fmt.Errorf("каталог профиля должен существовать")
	}
	resolved, err := filepath.EvalSymlinks(cli)
	if err != nil {
		return r, fmt.Errorf("выбранный CLI недоступен")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() {
		return r, fmt.Errorf("выбранный CLI должен быть обычным файлом")
	}
	r.CLI, r.Environment = resolved, canonical
	// The neutral temporary cwd avoids loading instructions from the caller's
	// project. Credentials remain in the explicitly selected CLI environment.
	dir, err := os.MkdirTemp("", "messenger-check-")
	if err != nil {
		return r, fmt.Errorf("не удалось создать временный каталог проверки")
	}
	defer os.RemoveAll(dir)
	env := supervisor.ProfileEnvironment(os.Environ(), provider, canonical)
	program := []string{resolved}
	if provider == "cursor" {
		program = adapters.CursorProgram(resolved) // node.exe and index.js on Windows (D-052)
	}
	probe := func(args ...string) cliprobe.Output {
		return runner(ctx, program[0], dir, env, append(append([]string(nil), program[1:]...), args...))
	}
	r.ProbeAttempts++
	version, reason := cliprobe.Version(provider, probe("--version"))
	if reason != "" {
		r.Version.Reason = reason
		return r, nil
	}
	r.Version = CheckState{State: "reported", Value: version}
	if r.Tested = Tested(provider, version); r.Tested != "" {
		r.TestedVersion = TestedCLI[provider]
	}
	// The login request runs for any recognized version (D-051): it only
	// reads the CLI's own status, and anything but the exact known answer is
	// reported as unknown.
	current, err := os.Stat(resolved)
	if err != nil || !os.SameFile(info, current) || info.Size() != current.Size() || !info.ModTime().Equal(current.ModTime()) {
		r.Authentication.Reason = "executable_changed"
		return r, nil
	}
	var args []string
	switch provider {
	case "claude":
		args = []string{"auth", "status", "--json"}
	case "codex":
		args = []string{"login", "status"}
	case "cursor":
		args = []string{"status", "--format", "json"}
	}
	if ctx.Err() != nil {
		r.Authentication.Reason = "timeout_or_cancelled"
		return r, nil
	}
	r.ProbeAttempts++
	r.Authentication = parseAuthentication(provider, probe(args...))
	return r, nil
}

func parseAuthentication(provider string, out cliprobe.Output) CheckState {
	unknown := CheckState{State: "unknown", Reason: "unrecognized_auth_output"}
	if out.Failure != "" {
		return CheckState{State: "unknown", Reason: out.Failure}
	}
	if provider == "codex" {
		// Codex writes status to stderr. Never publish the rest of that stream:
		// API-key mode may include a masked credential in its status line.
		text := strings.TrimSpace(string(out.Stdout) + string(out.Stderr))
		if out.Exit == 0 && text == "Logged in using ChatGPT" {
			return CheckState{State: "authenticated", Reason: "cli_reports_credentials_present", Value: "chatgpt"}
		}
		if out.Exit == 0 && strings.HasPrefix(text, "Logged in using an API key - ") && !strings.ContainsAny(text, "\r\n") {
			return CheckState{State: "authenticated", Reason: "cli_reports_credentials_present", Value: "api_key"}
		}
		if out.Exit == 1 && text == "Not logged in" {
			return CheckState{State: "not_authenticated", Reason: "cli_reports_no_login"}
		}
		return unknown
	}
	var value map[string]json.RawMessage
	if json.Unmarshal(out.Stdout, &value) != nil {
		return unknown
	}
	key := "loggedIn"
	if provider == "cursor" {
		key = "isAuthenticated"
	}
	raw, ok := value[key]
	if !ok || (string(raw) != "true" && string(raw) != "false") {
		return unknown
	}
	if string(raw) == "true" && out.Exit == 0 {
		return CheckState{State: "authenticated", Reason: "cli_reports_authenticated"}
	}
	// Claude documents exit 1 for missing login. Cursor's negative JSON/exit
	// contract has not been observed; do not label arbitrary failures logout.
	if string(raw) == "false" && provider == "claude" && out.Exit == 1 {
		return CheckState{State: "not_authenticated", Reason: "cli_reports_no_login"}
	}
	return unknown
}
