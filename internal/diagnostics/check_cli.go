package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
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

type probeOutput struct {
	stdout, stderr []byte
	exit           int
	failure        string
}

type probeRunner func(context.Context, string, string, []string, []string) probeOutput

// cappedOutput bounds memory while continuing to drain provider output. Nothing
// from these buffers is logged or copied to the report except validated fields.
type cappedOutput struct {
	data     bytes.Buffer
	overflow bool
}

func (b *cappedOutput) Bytes() []byte { return b.data.Bytes() }

func (b *cappedOutput) Write(p []byte) (int, error) {
	n := len(p)
	left := 65536 - b.data.Len()
	if len(p) > left {
		b.overflow = true
		p = p[:left]
	}
	_, _ = b.data.Write(p)
	return n, nil
}

func runProbe(ctx context.Context, cli, dir string, env, args []string) probeOutput {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, cli, args...)
	cmd.Dir, cmd.Env = dir, env
	// Bound inherited pipes after direct-child exit/cancellation. This does not
	// claim to stop every detached descendant and is never session-unlock proof.
	cmd.WaitDelay = 500 * time.Millisecond
	platform.HideConsole(cmd)
	var stdout, stderr cappedOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	out := probeOutput{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exit: -1}
	if cmd.ProcessState != nil {
		out.exit = cmd.ProcessState.ExitCode()
	}
	switch {
	case ctx.Err() != nil:
		out.failure = "timeout_or_cancelled"
	case stdout.overflow || stderr.overflow:
		out.failure = "output_limit"
	case errors.Is(err, exec.ErrWaitDelay):
		out.failure = "inherited_pipe_not_closed"
	case err != nil && out.exit < 0:
		out.failure = "execution_failed"
	}
	return out
}

var versionPatterns = map[string]*regexp.Regexp{
	"claude": regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+) \(Claude Code\)$`),
	"codex":  regexp.MustCompile(`^codex-cli ([0-9]+\.[0-9]+\.[0-9]+)$`),
	"cursor": regexp.MustCompile(`^([0-9]{4}\.[0-9]{2}\.[0-9]{2}-[a-f0-9]{7,40})$`),
}

func CheckCLI(ctx context.Context, provider, cli, environment string) (CLICheck, error) {
	return checkCLI(ctx, provider, cli, environment, runtime.GOOS, runProbe)
}

func checkCLI(ctx context.Context, provider, cli, environment, goos string, runner probeRunner) (CLICheck, error) {
	r := CLICheck{SchemaVersion: 1, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano), Provider: provider, OS: goos,
		Version: CheckState{State: "unknown"}, Authentication: CheckState{State: "not_checked"},
		Session: "not_checked", Compatibility: "not_checked", AuthScope: "selected_profile_and_inherited_environment"}
	if versionPatterns[provider] == nil || !filepath.IsAbs(cli) || !filepath.IsAbs(environment) {
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
	probe := func(args ...string) probeOutput {
		return runner(ctx, program[0], dir, env, append(append([]string(nil), program[1:]...), args...))
	}
	r.ProbeAttempts++
	v := probe("--version")
	if v.failure != "" {
		r.Version.Reason = v.failure
		return r, nil
	}
	match := versionPatterns[provider].FindStringSubmatch(strings.TrimSpace(string(v.stdout)))
	if v.exit != 0 || len(match) != 2 {
		r.Version.Reason = "unrecognized_version_output"
		return r, nil
	}
	r.Version = CheckState{State: "reported", Value: match[1]}
	if r.Tested = Tested(provider, match[1]); r.Tested != "" {
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

func parseAuthentication(provider string, out probeOutput) CheckState {
	unknown := CheckState{State: "unknown", Reason: "unrecognized_auth_output"}
	if out.failure != "" {
		return CheckState{State: "unknown", Reason: out.failure}
	}
	if provider == "codex" {
		// Codex writes status to stderr. Never publish the rest of that stream:
		// API-key mode may include a masked credential in its status line.
		text := strings.TrimSpace(string(out.stdout) + string(out.stderr))
		if out.exit == 0 && text == "Logged in using ChatGPT" {
			return CheckState{State: "authenticated", Reason: "cli_reports_credentials_present", Value: "chatgpt"}
		}
		if out.exit == 0 && strings.HasPrefix(text, "Logged in using an API key - ") && !strings.ContainsAny(text, "\r\n") {
			return CheckState{State: "authenticated", Reason: "cli_reports_credentials_present", Value: "api_key"}
		}
		if out.exit == 1 && text == "Not logged in" {
			return CheckState{State: "not_authenticated", Reason: "cli_reports_no_login"}
		}
		return unknown
	}
	var value map[string]json.RawMessage
	if json.Unmarshal(out.stdout, &value) != nil {
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
	if string(raw) == "true" && out.exit == 0 {
		return CheckState{State: "authenticated", Reason: "cli_reports_authenticated"}
	}
	// Claude documents exit 1 for missing login. Cursor's negative JSON/exit
	// contract has not been observed; do not label arbitrary failures logout.
	if string(raw) == "false" && provider == "claude" && out.exit == 1 {
		return CheckState{State: "not_authenticated", Reason: "cli_reports_no_login"}
	}
	return unknown
}
