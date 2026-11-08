package supervisor

import (
	"bufio"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// TaskLauncher reports whether this client may launch task workers. It is
// separate from per-session model permission: `serve --allow-tasks` enables
// it, and a paused queue still launches nothing.
type TaskLauncher interface {
	TasksEnabled() bool
}

// TasksEnabled is set by serve --allow-tasks.
func (l AdapterLauncher) TasksEnabled() bool { return l.AllowTasks }

// taskGroupWait bounds how long the runner waits for a signalled process
// group to empty after the worker exits. A group that is still not empty
// blocks the task; it is never released by this timeout.
var taskGroupWait = 3 * time.Second

// taskCancelPoll is how often a running worker checks for a cancel request.
var taskCancelPoll = 200 * time.Millisecond

// taskOutputGrace is how long the output may stay open after the worker
// exits.
var taskOutputGrace = 5 * time.Second

type taskRuns struct {
	mu      sync.Mutex
	running map[string]bool
	wg      sync.WaitGroup
}

func (t *taskRuns) claim(id string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.running == nil {
		t.running = map[string]bool{}
	}
	if t.running[id] {
		return false
	}
	t.running[id] = true
	return true
}

func (t *taskRuns) release(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.running, id)
}

// dispatchTasks launches every recorded run intent. There is no limit on
// parallel workers (D-022); one task never has two runs at once, because a
// run is recorded only when the previous one has finished.
func (s *Supervisor) dispatchTasks(ctx context.Context) {
	launcher, ok := s.currentLauncher().(TaskLauncher)
	if !ok || !launcher.TasksEnabled() || s.stopping.Load() {
		return
	}
	if paused, err := s.store.Paused(); err != nil || paused {
		return
	}
	intents, err := s.store.NextTaskRunIntents(50)
	if err != nil {
		return
	}
	for _, id := range intents {
		if !s.tasks.claim(id) {
			continue
		}
		s.tasks.wg.Add(1)
		go func(id string) {
			defer s.tasks.wg.Done()
			defer s.tasks.release(id)
			s.runTask(ctx, id)
		}(id)
	}
}

// runTask runs one worker run: launch intent, reservation, exec, identity
// binding, then the verification in FinishTaskRun. Nothing is retried. The
// reservation is retired only when the exit and an empty process group (after
// signalling it) were observed; otherwise it stays and the task is blocked.
func (s *Supervisor) runTask(ctx context.Context, runID string) {
	reservation, err := newRunID()
	if err != nil {
		return
	}
	run, task, err := s.store.BeginTaskRun(runID, reservation)
	if err != nil {
		return // paused, cancelled or no longer pending: nothing was launched
	}
	exe, err := os.Executable()
	if err != nil {
		_ = s.store.TaskRunNotStarted(runID, "messenger executable not found")
		return
	}
	spec := adapters.TaskRunSpec{
		Provider: task.Provider, CLI: task.CLI, Messenger: exe, DB: filepath.Join(s.root, dbName),
		RunID: run.ID, Receipt: run.Receipt, Session: task.SessionID, Resume: run.Kind == "answer" || run.Kind == "continue", Profile: task.Profile, Model: task.Model, FullAccess: task.FullAccess,
	}
	args, err := adapters.TaskArgs(spec)
	if err != nil {
		_ = s.store.TaskRunNotStarted(runID, err.Error())
		return
	}
	if task.Provider == "cursor" {
		if err := ensureCursorTaskServer(exe, task.Environment); err != nil {
			_ = s.store.TaskRunNotStarted(runID, "Cursor MCP settings: "+err.Error())
			return
		}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(ProfileEnvironment(os.Environ(), task.Provider, task.Environment), adapters.TaskEnv(spec)...)
	cmd.Env = append(cmd.Env, adapters.WorkerRunVariable+"="+spec.RunID)
	cmd.Dir = task.Folder
	cmd.Stderr = io.Discard
	if err := platform.SetNewGroup(cmd); err != nil {
		_ = s.store.TaskRunNotStarted(runID, err.Error())
		return
	}
	// A plain pipe, not StdoutPipe: Wait would close StdoutPipe's read end
	// as soon as the process exits and could drop its last lines.
	stdout, stdoutW, err := os.Pipe()
	if err != nil {
		_ = s.store.TaskRunNotStarted(runID, err.Error())
		return
	}
	cmd.Stdout = stdoutW
	started := false
	defer func() {
		if !started {
			_ = stdout.Close()
		}
	}()
	record := recordedRun{ID: reservation, Root: s.root, Provider: task.Provider, Environment: task.Environment, ConversationID: task.SessionID, TaskRunID: run.ID}
	record.Epoch, _ = recoveryEpoch()
	if err := rememberRun(record); err != nil {
		_ = s.store.TaskRunNotStarted(runID, "reservation not written: "+err.Error())
		return
	}
	err = platform.StartGroup(cmd)
	_ = stdoutW.Close()
	if err != nil {
		if platform.NeverRan(cmd, err) {
			_ = forgetRunID(reservation)
			_ = s.store.TaskRunNotStarted(runID, err.Error())
			return
		}
		// Unknown whether anything ran: keep the reservation and block.
		_, _ = s.store.FinishTaskRun(runID, core.RunEvidence{Detail: "start outcome unknown: " + err.Error()})
		return
	}
	obs, obsErr := platform.ReadObservation(cmd.Process.Pid)
	if obsErr == nil {
		record.PID, record.PGID, record.StartToken, record.BootID = obs.PID, obs.PGID, obs.StartToken, obs.BootID
		obsErr = rememberRun(record)
	}
	if obsErr == nil {
		obsErr = s.store.RecordTaskRunProcess(runID, obs)
	}
	if obsErr != nil {
		// Without a recorded process the run cannot be verified. Stop it and
		// block the task; the reservation stays.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_, _ = s.store.FinishTaskRun(runID, core.RunEvidence{Detail: "process not recorded: " + obsErr.Error()})
		return
	}
	started = true

	type readOutcome struct {
		result adapters.TaskEventResult
		err    error
	}
	readDone := make(chan readOutcome, 1)
	go func() {
		var result adapters.TaskEventResult
		reader := bufio.NewReader(stdout)
		bound := false
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				adapters.ParseTaskEvent(task.Provider, line, &result)
				for _, item := range adapters.TaskFeedEntries(task.Provider, line) {
					_ = s.store.AppendTaskRunItem(runID, item.Kind, item.Title, item.Text)
				}
				if !bound && result.Session != "" {
					bound = true
					if s.store.BindTaskSession(runID, result.Session) != nil {
						// A different conversation: its tools stay closed, and
						// the run will fail verification. Stop it early.
						_ = platform.SignalGroup(obs.PGID)
					}
				}
			}
			if err != nil {
				if errors.Is(err, io.EOF) {
					err = nil
				}
				readDone <- readOutcome{result, err}
				return
			}
		}
	}()

	cancelled := false
	stopWatch := make(chan struct{})
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		ticker := time.NewTicker(taskCancelPoll)
		defer ticker.Stop()
		for {
			select {
			case <-stopWatch:
				return
			case <-ticker.C:
				if t, err := s.store.TaskByID(task.ID); err == nil && t.CancelRequested && !cancelled {
					cancelled = true
					_ = platform.SignalGroup(obs.PGID)
				}
			}
		}
	}()
	exitCode, waitErr, detached := waitUnlessStopped(ctx, func() (int, error) {
		err := cmd.Wait()
		if cmd.ProcessState == nil {
			return -1, err
		}
		return cmd.ProcessState.ExitCode(), nil
	})
	close(stopWatch)
	<-watchDone
	if detached {
		// The client is stopping. The worker keeps running; the next start
		// marks the run interrupted and keeps its reservation.
		return
	}
	// The output ends when every holder of the pipe exits. A process the
	// worker left behind may keep it open: then the output is not trusted.
	var read readOutcome
	select {
	case read = <-readDone:
	case <-time.After(taskOutputGrace):
		_ = stdout.Close()
		select {
		case read = <-readDone:
		case <-time.After(taskOutputGrace):
		}
		read.err = errors.New("output still open after the worker exited")
	}
	_ = stdout.Close()
	result, readErr := read.result, read.err
	ev := core.RunEvidence{ExitObserved: waitErr == nil, ExitCode: exitCode, Cancelled: cancelled, LastText: result.LastText}
	if u := result.Usage; u != nil {
		ev.Usage = &core.TaskUsage{InputTokens: u.Input, CachedTokens: u.Cached, OutputTokens: u.Output}
		if u.HasCost {
			cost := u.Cost
			ev.Usage.CostUSD = &cost
		}
	}
	ev.ProviderOK = result.Completed && !result.Conflicting && readErr == nil && result.Failure == ""
	if result.Failure != "" {
		ev.Detail = result.Failure
	}
	// T-034: the failure class and reset time only explain the failure.
	if result.Reason != "" {
		ev.Reason = result.Reason
		if !result.ResetAt.IsZero() {
			ev.ResetAt = result.ResetAt.UTC().Format(time.RFC3339)
		}
	}
	if result.Conflicting {
		ev.Detail = "the CLI reported two conversation ids"
	}
	// T-044: keep the agent's last reported limit for the coordinator. It
	// only informs; it never changes the run's outcome.
	if limit, ok := agentLimit(result); ok {
		_ = s.store.RecordAgentLimit(task.Provider, limit)
	}
	// D-021: after the exit, signal the group and require it to be empty.
	_ = platform.SignalGroup(obs.PGID)
	deadline := time.Now().Add(taskGroupWait)
	for {
		empty, err := platform.GroupEmpty(obs.PGID)
		if err == nil && empty {
			ev.GroupEmpty = true
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	final, err := s.store.FinishTaskRun(runID, ev)
	if err != nil {
		return
	}
	if final == "finished" || final == "failed" || final == "cancelled" {
		if forgetRunID(reservation) == nil {
			_ = s.store.TaskRunReservationRetired(runID)
		}
	}
}

// ensureCursorTaskServer keeps Messenger's worker tools in the Cursor
// mcp.json files a run reads (D-049). Only Messenger's own entry is written.
func ensureCursorTaskServer(messenger, profile string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	for _, path := range adapters.CursorConfigFiles(home, profile) {
		if err := adapters.EnsureCursorTaskServer(path, messenger); err != nil {
			return err
		}
	}
	return nil
}

// agentLimit is the limit state a run showed: what the agent reported, or
// "exhausted" when the run failed on its usage limit.
func agentLimit(r adapters.TaskEventResult) (core.AgentLimit, bool) {
	var l core.AgentLimit
	switch {
	case r.Limit != nil:
		l.State, l.Window = r.Limit.State, r.Limit.Window
		if r.Limit.HasPercent {
			p := r.Limit.UsedPercent
			l.UsedPercent = &p
		}
		if !r.Limit.ResetAt.IsZero() {
			l.ResetsAt = r.Limit.ResetAt.UTC().Format(time.RFC3339)
		}
	case r.Reason == adapters.ReasonUsageLimit:
		l.State = adapters.LimitExhausted
	default:
		return l, false
	}
	if r.Reason == adapters.ReasonUsageLimit {
		l.State = adapters.LimitExhausted
		if l.ResetsAt == "" && !r.ResetAt.IsZero() {
			l.ResetsAt = r.ResetAt.UTC().Format(time.RFC3339)
		}
	}
	return l, true
}
