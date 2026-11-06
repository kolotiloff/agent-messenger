package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

var errNotStarted = errors.New("CLI definitely did not start")

type StreamProcess struct {
	Observation platform.Observation
	Input       *os.File
	Output      io.ReadCloser
	Wait        func() (int, error)
}

type StreamLauncher interface {
	StreamingEnabled() bool
	StartStream(context.Context, Spec, string, string) (StreamProcess, error)
}

func (l AdapterLauncher) StreamingEnabled() bool { return l.ClaudeStream }

func (l AdapterLauncher) StartStream(ctx context.Context, spec Spec, url, secret string) (StreamProcess, error) {
	if !l.AllowModels || !l.ClaudeStream || (l.OnlySession != "" && spec.Alias != l.OnlySession) {
		return StreamProcess{}, errModelsNotAuthorized
	}
	if spec.Provider != "claude" || spec.CLI == "" {
		return StreamProcess{}, errNoCLI
	}
	config, _ := json.Marshal(map[string]any{"mcpServers": map[string]any{"messenger_bound": map[string]any{"type": "http", "url": url, "headers": map[string]string{"Authorization": "Bearer " + secret}}}})
	args, err := adapters.ClaudeStreamArgs(spec.CLI, spec.ConversationID, string(config), spec.NewSession)
	if err != nil {
		return StreamProcess{}, err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = profileEnv(os.Environ(), spec.Provider, spec.Environment)
	cmd.Dir = spec.Workspace
	cmd.Stderr = io.Discard
	if err := platform.SetNewGroup(cmd); err != nil {
		return StreamProcess{}, err
	}
	inputRead, input, err := os.Pipe()
	if err != nil {
		return StreamProcess{}, err
	}
	output, outputWrite, err := os.Pipe()
	if err != nil {
		inputRead.Close()
		input.Close()
		return StreamProcess{}, err
	}
	cmd.Stdin = inputRead
	cmd.Stdout = outputWrite
	started := false
	defer func() {
		inputRead.Close()
		outputWrite.Close()
		if !started {
			input.Close()
			output.Close()
		}
	}()
	id, err := newRunID()
	if err != nil {
		return StreamProcess{}, err
	}
	run := recordedRun{ID: id, Root: spec.DataRoot, Provider: spec.Provider, Environment: spec.Environment, ConversationID: spec.ConversationID}
	run.StreamRunID = spec.Token
	// Failure to read the epoch does not relax a reservation. Such a run will
	// remain non-recoverable by the strict reboot recovery command.
	run.Epoch, _ = platform.CurrentRecoveryEpoch()
	if err := rememberRun(run); err != nil {
		return StreamProcess{}, err
	}
	if err := ctx.Err(); err != nil {
		return StreamProcess{}, errors.Join(err, forgetRunID(id))
	}
	if err := cmd.Start(); err != nil {
		// Only these definite OS refusals establish that the target executable
		// never ran. Unknown failures keep their incomplete reservation.
		if cmd.Process == nil && (errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOEXEC) || errors.Is(err, syscall.ENOTDIR)) {
			if cleanupErr := forgetRunID(id); cleanupErr != nil {
				return StreamProcess{}, errors.Join(err, cleanupErr)
			}
			return StreamProcess{}, errors.Join(errNotStarted, err)
		}
		return StreamProcess{}, err
	}
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return StreamProcess{}, err
	}
	run.PID = obs.PID
	run.PGID = obs.PGID
	run.StartToken = obs.StartToken
	run.BootID = obs.BootID
	if err := rememberRun(run); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return StreamProcess{}, err
	}
	started = true
	return StreamProcess{Observation: obs, Input: input, Output: output, Wait: func() (int, error) {
		err := cmd.Wait()
		if cmd.ProcessState == nil {
			return -1, fmt.Errorf("no wait status: %w", err)
		}
		return cmd.ProcessState.ExitCode(), nil
	}}, nil
}
