package supervisor

import (
	"io"
	"os"
	"os/exec"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// AdapterLauncher builds the real Claude, Codex or Cursor command and starts it.
// It does not decide that a model may be called: that happens only after register-session
// stored a CLI path and a message is queued. Tests pass a fake executable as that path.
type AdapterLauncher struct{}

func (AdapterLauncher) Enabled() bool { return true }

func (AdapterLauncher) LaunchLabel() string { return "adapters" }

func (AdapterLauncher) Start(spec Spec) (Started, error) {
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
	}
	cmd := exec.Command(args[0], args[1:]...)
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
	if err := cmd.Start(); err != nil {
		return Started{}, err
	}
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Started{}, err
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

var errNoCLI = errString("session has no bound CLI")

type errString string

func (e errString) Error() string { return string(e) }
