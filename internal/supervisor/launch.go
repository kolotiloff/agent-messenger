package supervisor

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// AdapterLauncher builds the real Claude, Codex or Cursor command and starts it.
// AllowModels must be set by `serve --allow-model-calls`. Without that flag the
// process stays up and does not execute a CLI, so a status check cannot spend tokens.
type AdapterLauncher struct {
	AllowModels  bool
	ClaudeStream bool
	OnlySession  string
	// AlsoAllowed holds further sessions the window authorized in this run.
	AlsoAllowed []string
}

// Allows reports whether model calls may run for this alias.
func (l AdapterLauncher) Allows(alias string) bool {
	if !l.AllowModels {
		return false
	}
	if l.OnlySession == "" || alias == l.OnlySession {
		return true
	}
	for _, a := range l.AlsoAllowed {
		if a == alias {
			return true
		}
	}
	return false
}

// SessionAliases lists the scoped sessions, or nil when calls are unscoped.
func (l AdapterLauncher) SessionAliases() []string {
	if l.OnlySession == "" {
		return nil
	}
	return append([]string{l.OnlySession}, l.AlsoAllowed...)
}

func (l AdapterLauncher) SessionAlias() string { return l.OnlySession }

func (l AdapterLauncher) Enabled() bool { return l.AllowModels }

func (l AdapterLauncher) LaunchLabel() string {
	if !l.AllowModels {
		return "not-authorized"
	}
	if l.ClaudeStream {
		return "claude-stream"
	}
	return "adapters"
}

func (l AdapterLauncher) Start(spec Spec) (Started, error) {
	if !l.Allows(spec.Alias) {
		return Started{}, errModelsNotAuthorized
	}
	if spec.CLI == "" {
		return Started{}, errNoCLI
	}
	exe, err := os.Executable()
	if err != nil {
		return Started{}, err
	}
	args, err := commandArgs(spec, exe)
	if err != nil {
		return Started{}, err
	}
	if spec.Provider == "cursor" {
		if err := adapters.CheckCursorIsolation(os.Getenv("HOME"), os.Getenv("CURSOR_CONFIG_DIR")); err != nil {
			return Started{}, err
		}
		if err := adapters.PrepareCursorWorkspace(spec.Workspace, exe, spec.DB, spec.Token); err != nil {
			return Started{}, err
		}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = profileEnv(os.Environ(), spec.Provider, spec.Environment)
	if spec.Workspace != "" {
		cmd.Dir = spec.Workspace
	}
	if err := platform.SetNewGroup(cmd); err != nil {
		return Started{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Started{}, err
	}
	cmd.Stderr = io.Discard
	// Reserve before Start. A crash before the PID is recorded must still block another root.
	reservationID, err := newRunID()
	if err != nil {
		return Started{}, err
	}
	pending := recordedRun{
		ID: reservationID, Root: spec.DataRoot, Provider: spec.Provider,
		Environment: spec.Environment, ConversationID: spec.ConversationID,
	}
	if err := rememberRun(pending); err != nil {
		return Started{}, err
	}
	if err := cmd.Start(); err != nil {
		_ = forgetRunID(reservationID)
		return Started{}, err
	}
	obs, obsErr := platform.ReadObservation(cmd.Process.Pid)
	pending.PID = cmd.Process.Pid
	if obsErr == nil {
		pending.PGID = obs.PGID
		pending.StartToken = obs.StartToken
		pending.BootID = obs.BootID
	}
	if err := rememberRun(pending); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Started{}, err
	}
	if obsErr != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Started{}, obsErr
	}
	return Started{
		PID: obs.PID, PGID: obs.PGID, StartToken: obs.StartToken, BootID: obs.BootID, Stdout: stdout,
		Wait: func() (int, error) {
			waitErr := cmd.Wait()
			if cmd.ProcessState == nil {
				return -1, waitErr
			}
			return cmd.ProcessState.ExitCode(), nil
		},
	}, nil
}

func commandArgs(spec Spec, messenger string) ([]string, error) {
	if spec.NewSession {
		return nil, errString("new Claude session requires serve --claude-stream")
	}
	switch spec.Provider {
	case "claude":
		return adapters.ClaudeArgs(spec.CLI, messenger, spec.DB, spec.Token, spec.ConversationID)
	case "codex":
		return adapters.CodexArgs(spec.CLI, messenger, spec.DB, spec.Token, spec.ConversationID)
	case "cursor":
		return adapters.CursorArgs(spec.CLI, spec.ConversationID, spec.Workspace)
	default:
		return nil, errNoCLI
	}
}

func newRunID() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}

func profileVariable(provider string) string {
	switch provider {
	case "claude":
		return "CLAUDE_CONFIG_DIR"
	case "codex":
		return "CODEX_HOME"
	case "cursor":
		return "CURSOR_CONFIG_DIR"
	default:
		return ""
	}
}

// profileEnv binds the process to the registered profile.
// The standard profile is selected by removing the override: Claude stores the
// normal macOS login for ~/.claude and stops seeing it when CLAUDE_CONFIG_DIR is set.
// A different directory still gets the variable, and an inherited value is never kept.
func profileEnv(parent []string, provider, environment string) []string {
	return ProfileEnvironment(parent, provider, environment)
}

// ProfileEnvironment applies the same explicit profile selection to launches
// and diagnostic subcommands. It does not read or export stored credentials.
func ProfileEnvironment(parent []string, provider, environment string) []string {
	key := profileVariable(provider)
	out := make([]string, 0, len(parent)+1)
	for _, item := range parent {
		name, _, _ := strings.Cut(item, "=")
		if key != "" && name == key {
			continue
		}
		out = append(out, item)
	}
	if key != "" && environment != "" && !isDefaultProfile(provider, environment) {
		out = append(out, key+"="+environment)
	}
	return out
}

func isDefaultProfile(provider, environment string) bool {
	def, err := defaultProfileDir(provider)
	if err != nil || def == "" || environment == "" {
		return false
	}
	return filepath.Clean(environment) == filepath.Clean(def)
}

func defaultProfileDir(provider string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	var name string
	switch provider {
	case "claude":
		name = ".claude"
	case "codex":
		name = ".codex"
	case "cursor":
		name = ".cursor"
	default:
		return "", nil
	}
	path := filepath.Join(home, name)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path), nil
	}
	return resolved, nil
}

var errNoCLI = errString("session has no bound CLI")
var errModelsNotAuthorized = errString("model calls are not authorized; pass serve --allow-model-calls for a test session")

type errString string

func (e errString) Error() string { return string(e) }
