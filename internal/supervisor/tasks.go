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
	"syscall"
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

func definiteNotStarted(err error) bool {
	return errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOEXEC) || errors.Is(err, syscall.ENOTDIR) || errors.Is(err, exec.ErrNotFound)
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
		RunID: run.ID, Receipt: run.Receipt, Session: task.SessionID, Resume: run.Kind == "answer", Profile: task.Profile,
	}
	args, err := adapters.TaskArgs(spec)
	if err != nil {
		_ = s.store.TaskRunNotStarted(runID, err.Error())
		return
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = ProfileEnvironment(os.Environ(), task.Provider, task.Environment)
	cmd.Dir = task.Folder
	cmd.Stderr = io.Discard
	if err := platform.SetNewGroup(cmd); err != nil {
		_ = s.store.TaskRunNotStarted(runID, err.Error())
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = s.store.TaskRunNotStarted(runID, err.Error())
		return
	}
	record := recordedRun{ID: reservation, Root: s.root, Provider: task.Provider, Environment: task.Environment, ConversationID: task.SessionID, TaskRunID: run.ID}
	record.Epoch, _ = recoveryEpoch()
	if err := rememberRun(record); err != nil {
		_ = s.store.TaskRunNotStarted(runID, "reservation not written: "+err.Error())
		return
	}
	if err := cmd.Start(); err != nil {
		if cmd.Process == nil && definiteNotStarted(err) {
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

	var result adapters.TaskEventResult
	readDone := make(chan error, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		bound := false
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				adapters.ParseTaskEvent(task.Provider, line, &result)
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
				readDone <- err
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
	readErr := <-readDone
	ev := core.RunEvidence{ExitObserved: waitErr == nil, ExitCode: exitCode, Cancelled: cancelled}
	ev.ProviderOK = result.Completed && !result.Conflicting && readErr == nil && result.Failure == ""
	if result.Failure != "" {
		ev.Detail = result.Failure
	}
	if result.Conflicting {
		ev.Detail = "the CLI reported two conversation ids"
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
