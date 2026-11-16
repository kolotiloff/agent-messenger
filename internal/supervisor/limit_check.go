package supervisor

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/journal"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// LimitReader reads an agent's account limits outside a task (T-191). It
// never calls a model.
type LimitReader func(ctx context.Context, cli, environment, dir string) (adapters.LimitState, error)

// How often the limits are read, how long the first read waits after the
// start, and how long one read may take.
var (
	limitCheckEvery   = 10 * time.Minute
	limitCheckFirst   = time.Minute
	limitCheckTimeout = 30 * time.Second
)

// checkLimits reads the Codex account limits of this computer now and then
// and keeps them for list_agents and the other computers (T-191). A failed
// read keeps the last stored limit and is tried again only at the next
// round: nothing is retried at once.
func (s *Supervisor) checkLimits(ctx context.Context, read LimitReader) {
	timer := time.NewTimer(limitCheckFirst)
	defer timer.Stop()
	failed := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}
		if why := s.checkLimitsOnce(ctx, read); why != failed {
			if why != "" {
				journal.Printf("codex limits not read: %s", why)
			} else if failed != "" {
				journal.Printf("codex limits read again")
			}
			failed = why
		}
		timer.Reset(limitCheckEvery)
	}
}

// checkLimitsOnce reads and stores the limits once; it returns why it did
// not, or "".
func (s *Supervisor) checkLimitsOnce(ctx context.Context, read LimitReader) string {
	agents, err := s.store.ComputerAgents()
	if err != nil {
		return "agents not read: " + err.Error()
	}
	for _, a := range agents {
		if a.Provider != "codex" || a.CLI == "" || a.Paused {
			continue
		}
		call, cancel := context.WithTimeout(ctx, limitCheckTimeout)
		l, err := read(call, a.CLI, a.Environment, s.root)
		cancel()
		if err != nil {
			return err.Error()
		}
		if err := s.store.RecordAgentLimit("codex", checkedLimit(l, time.Now())); err != nil {
			return "not stored: " + err.Error()
		}
	}
	return ""
}

// checkedLimit is what a check read, in the stored form.
func checkedLimit(l adapters.LimitState, at time.Time) core.AgentLimit {
	out := core.AgentLimit{State: l.State, Window: l.Window, Source: core.LimitSourceCheck, ObservedAt: at.UTC().Format(time.RFC3339)}
	if l.HasPercent {
		p := l.UsedPercent
		out.UsedPercent = &p
	}
	if !l.ResetAt.IsZero() {
		out.ResetsAt = l.ResetAt.UTC().Format(time.RFC3339)
	}
	for _, w := range l.Windows {
		cw := core.LimitWindow{Window: w.Window, UsedPercent: w.UsedPercent}
		if !w.ResetAt.IsZero() {
			cw.ResetsAt = w.ResetAt.UTC().Format(time.RFC3339)
		}
		out.Windows = append(out.Windows, cw)
	}
	return out
}

// readCodexLimits starts `codex app-server` with the owner's Codex profile,
// asks it for the account limits and stops it with its whole process group.
// It sends no request that creates a thread or calls a model.
func readCodexLimits(ctx context.Context, cli, environment, dir string) (adapters.LimitState, error) {
	args, err := adapters.CodexLimitArgs(cli)
	if err != nil {
		return adapters.LimitState{}, err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = ProfileEnvironment(os.Environ(), "codex", environment)
	cmd.Dir = dir
	if err := platform.SetNewGroup(cmd); err != nil {
		return adapters.LimitState{}, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return adapters.LimitState{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return adapters.LimitState{}, err
	}
	if err := platform.StartGroup(cmd); err != nil {
		return adapters.LimitState{}, err
	}
	pgid := cmd.Process.Pid
	if obs, err := platform.ReadObservation(cmd.Process.Pid); err == nil {
		pgid = obs.PGID
	}
	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = stdin.Close()
			_ = platform.SignalGroup(pgid)
			done := make(chan struct{})
			go func() { _ = cmd.Wait(); close(done) }()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
		})
	}
	defer stop()
	l, err := adapters.ReadCodexLimits(ctx, stdin, stdout, buildinfo.Version)
	if err != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return l, errors.New("Codex did not answer in time")
	}
	return l, err
}
