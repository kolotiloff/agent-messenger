package supervisor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/journal"
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
	// live are the worker processes started and not yet exited, by run
	// (T-149): their task and when they last wrote output.
	live map[string]*liveRun
}

type liveRun struct {
	contextWindow  atomic.Pointer[core.ContextWindow]
	task           string
	lastOutput     atomic.Int64 // unix nanoseconds
	activeCommands atomic.Int32
	turnEnded      atomic.Bool
}

func (t *taskRuns) started(run, task string) *liveRun {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.live == nil {
		t.live = map[string]*liveRun{}
	}
	l := &liveRun{task: task}
	l.lastOutput.Store(time.Now().UnixNano())
	t.live[run] = l
	return l
}

func (t *taskRuns) exited(run string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.live, run)
}

// workers are the live workers: task and seconds since their last output.
func (t *taskRuns) workers() map[string]int {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := map[string]int{}
	for _, l := range t.live {
		age := max(0, int(time.Since(time.Unix(0, l.lastOutput.Load()))/time.Second))
		if old, ok := out[l.task]; !ok || age < old {
			out[l.task] = age
		}
	}
	return out
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

// A write by a different local process is found by the periodic queue check.
// Writes handled here can wake the dispatcher immediately.
func (s *Supervisor) wakeTaskDispatch() {
	select {
	case s.taskWake <- struct{}{}:
	default:
	}
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
	if s.server != nil {
		s.server.NotifyPresence()
		defer s.server.NotifyPresence()
	}
	journal.Printf("run %s of %s: launching %s (model %q, %s run)", runID, task.ID, task.Provider, task.Model, run.Kind)
	exe, err := workerProgram()
	if err != nil {
		s.runNotStarted(runID, task.ID, "messenger executable not found")
		return
	}
	taskDB, err := s.store.WorkerTaskDatabase(filepath.Join(s.root, dbName), task.ID)
	if err != nil {
		s.runNotStarted(runID, task.ID, "worker database not available: "+err.Error())
		return
	}
	spec := adapters.TaskRunSpec{
		Provider: task.Provider, CLI: task.CLI, Messenger: exe, DB: taskDB,
		RunID: run.ID, Receipt: run.Receipt, Session: task.SessionID, Resume: run.Kind == "answer" || run.Kind == "continue" || run.Kind == "note", Profile: task.Profile, Model: task.Model, FullAccess: task.FullAccess,
	}
	args, err := adapters.TaskArgs(spec)
	if err != nil {
		s.runNotStarted(runID, task.ID, err.Error())
		return
	}
	if task.Provider == "cursor" {
		if err := ensureCursorTaskServer(exe, task.Environment); err != nil {
			s.runNotStarted(runID, task.ID, "Cursor MCP settings: "+err.Error())
			return
		}
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(ProfileEnvironment(os.Environ(), task.Provider, task.Environment), adapters.TaskEnv(spec)...)
	cmd.Env = append(cmd.Env, adapters.WorkerRunVariable+"="+spec.RunID)
	cmd.Dir = task.Folder
	// Keep only the end of the CLI's error output: it explains a run that
	// failed before the CLI reported anything (coordinator, 2026-11-12).
	stderr := &tailBuffer{max: stderrTail}
	cmd.Stderr = stderr
	// D-147: a process the worker left behind inherits the error output.
	// Without a limit Wait would wait for it too, after the worker exited.
	cmd.WaitDelay = taskOutputGrace
	if err := platform.SetNewGroup(cmd); err != nil {
		s.runNotStarted(runID, task.ID, err.Error())
		return
	}
	// A plain pipe, not StdoutPipe: Wait would close StdoutPipe's read end
	// as soon as the process exits and could drop its last lines.
	stdout, stdoutW, err := os.Pipe()
	if err != nil {
		s.runNotStarted(runID, task.ID, err.Error())
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
		s.runNotStarted(runID, task.ID, "reservation not written: "+err.Error())
		return
	}
	launched := time.Now()
	err = platform.StartGroup(cmd)
	_ = stdoutW.Close()
	if err != nil {
		if platform.NeverRan(cmd, err) {
			_ = forgetRunID(reservation)
			s.runNotStarted(runID, task.ID, err.Error())
			return
		}
		// Unknown whether anything ran: keep the reservation and block.
		s.finishUnclear(runID, task.ID, core.RunEvidence{Detail: "start outcome unknown: " + err.Error()})
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
		s.finishUnclear(runID, task.ID, core.RunEvidence{Detail: "process not recorded: " + obsErr.Error()})
		return
	}
	started = true
	if spec.Resume {
		_ = s.store.ClearTaskRunModel(runID)
	}
	journal.Printf("run %s of %s: worker started, process %d", runID, task.ID, obs.PID)
	live := s.tasks.started(runID, task.ID)
	lingerDone := make(chan struct{})
	defer close(lingerDone)

	type readOutcome struct {
		result adapters.TaskEventResult
		err    error
	}
	readDone := make(chan readOutcome, 1)
	go func() {
		var result adapters.TaskEventResult
		reader := bufio.NewReader(stdout)
		bound := false
		var outputNoted time.Time
		finalNoted := false
		activeCommands := make(map[string]bool)
		for {
			line, err := reader.ReadBytes('\n')
			if len(line) > 0 {
				live.lastOutput.Store(time.Now().UnixNano())
				if time.Since(outputNoted) >= core.RunOutputStep() {
					outputNoted = time.Now()
					_ = s.store.NoteRunOutput(runID)
				}
				model := result.Model
				adapters.ParseTaskEvent(task.Provider, line, &result)
				if result.Completed && !finalNoted {
					// T-172: the worker's turn ended. If its process does not
					// end too, say what keeps it; nothing is closed here.
					finalNoted = true
					live.turnEnded.Store(true)
					_ = s.store.NoteRunOutput(runID)
					_ = s.store.NoteRunFinal(runID)
					if usage := result.Usage; usage != nil {
						_ = s.store.NoteLiveTokens(runID, usage.Input+usage.Output)
					}
					go s.watchLinger(runID, task.ID, task.Provider, obs, lingerDone)
				}
				if result.Model != model {
					_ = s.store.SetTaskRunModel(runID, result.Model)
				}
				for _, item := range adapters.TaskFeedEntries(task.Provider, line) {
					switch item.Kind {
					case "command_start":
						if activeCommands[item.Title] {
							continue
						}
						activeCommands[item.Title] = true
						live.activeCommands.Add(1)
						item.Title = runID + "/" + item.Title
					case "command_end":
						if !activeCommands[item.Title] {
							continue
						}
						delete(activeCommands, item.Title)
						live.activeCommands.Add(-1)
						item.Title = runID + "/" + item.Title
					}
					_ = s.store.AppendTaskRunItem(runID, item.Kind, item.Title, item.Text, item.Description)
				}
				if !bound && result.Session != "" {
					bound = true
					if s.store.BindTaskSession(runID, result.Session) != nil {
						// A different conversation: its tools stay closed, and
						// the run will fail verification. Stop it early.
						_ = platform.SignalGroup(obs.PGID)
					} else if task.Provider == "codex" {
						go s.watchCodexTask(ctx, lingerDone, live, runID, task, result.Session, launched, spec.Resume, result.Model != "")
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
	if task.Provider == "codex" {
		go s.watchCodexSilence(runID, live, obs, stopWatch)
	}
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
	if !detached {
		s.tasks.exited(runID)
	}
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
	if readErr != nil && result.Completed && waitErr == nil {
		// D-147: the worker's turn ended and its own process exited; a
		// process it left keeps the output open, which is no failure.
		readErr = nil
	}
	ev := core.RunEvidence{ExitObserved: waitErr == nil, ExitCode: exitCode, Cancelled: cancelled, LastText: result.LastText, FinalText: result.FinalText}
	if u := result.Usage; u != nil {
		ev.Usage = &core.TaskUsage{InputTokens: u.Input, CachedTokens: u.Cached, OutputTokens: u.Output, CacheWriteTokens: u.CacheWrite}
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
	if !ev.ProviderOK || exitCode != 0 {
		// The error output is cleaned like a shared log: it reaches the
		// coordinator and may travel to another computer.
		if detail, reason, reset := adapters.StderrFailure(redactLog(stderr.String()), time.Now()); detail != "" {
			if result.Failure == "" && !result.Conflicting {
				ev.Detail = detail
			} else {
				ev.Detail += "; " + detail
			}
			if ev.Reason == "" || ev.Reason == adapters.ReasonOther {
				ev.Reason = reason
				if !reset.IsZero() && ev.ResetAt == "" {
					ev.ResetAt = reset.UTC().Format(time.RFC3339)
				}
				// The agent's limit in list_agents follows the same text.
				result.Reason = reason
				if result.ResetAt.IsZero() {
					result.ResetAt = reset
				}
			}
		}
	}
	// T-044: keep the agent's last reported limit for the coordinator. It
	// only informs; it never changes the run's outcome.
	if limit, ok := agentLimit(result); ok {
		_ = s.store.RecordAgentLimit(task.Provider, limit)
	} else if ev.ProviderOK {
		// A run that completed shows the limit was not exhausted.
		_ = s.store.AgentLimitCleared(task.Provider, time.Now())
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
	if !ev.GroupEmpty && ev.ExitObserved {
		ev.Leftover = leftoverNames(obs.PID, obs.PGID)
	}
	final, err := s.store.FinishTaskRun(runID, ev)
	journal.Printf("run %s of %s: ended %s (exit %d observed %t, empty group %t, provider %t, reason %q, cancelled %t)%s", runID, task.ID, final, ev.ExitCode, ev.ExitObserved, ev.GroupEmpty, ev.ProviderOK, ev.Reason, ev.Cancelled, journalDetail(ev.Detail, err))
	if err != nil {
		return
	}
	if final == "finished" || final == "failed" || final == "cancelled" {
		if forgetRunID(reservation) == nil {
			_ = s.store.TaskRunReservationRetired(runID)
		}
	}
}

func (s *Supervisor) watchCodexTask(ctx context.Context, done <-chan struct{}, live *liveRun, runID string, task core.Task, thread string, launched time.Time, resumed, modelKnown bool) {
	profile := task.Environment
	if profile == "" {
		profile, _ = defaultProfileDir("codex")
	}
	if !filepath.IsAbs(profile) {
		return
	}
	created, _ := time.Parse(time.RFC3339Nano, task.CreatedAt)
	var lastTokens int64 = -1
	read := func() {
		if !modelKnown {
			if model := adapters.CodexTurnModel(profile, thread, launched, created); model != "" {
				modelKnown = s.store.SetTaskRunModel(runID, model) == nil
			}
		}
		usage := adapters.CodexLiveUsage(profile, thread, launched, created, resumed)
		if usage.CapacityTokens > 0 {
			live.contextWindow.Store(&core.ContextWindow{UsedTokens: usage.UsedTokens, CapacityTokens: usage.CapacityTokens})
		} else {
			live.contextWindow.Store(nil)
		}
		if tokens := usage.TurnTokens; tokens != nil && *tokens > lastTokens {
			if s.store.NoteLiveTokens(runID, *tokens) == nil {
				lastTokens = *tokens
			}
		}
	}
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		read()
		select {
		case <-tick.C:
		case <-ctx.Done():
			return
		case <-done:
			read()
			return
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
		l.State, l.Window, l.Reported = r.Limit.State, r.Limit.Window, r.Limit.Reported
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

// runNotStarted records a run that never started, also in the journal.
func (s *Supervisor) runNotStarted(runID, task, why string) {
	journal.Printf("run %s of %s: not started: %s", runID, task, why)
	_ = s.store.TaskRunNotStarted(runID, why)
}

// finishUnclear closes a run whose start or process is unclear; the store
// blocks its task.
func (s *Supervisor) finishUnclear(runID, task string, ev core.RunEvidence) {
	final, err := s.store.FinishTaskRun(runID, ev)
	journal.Printf("run %s of %s: ended %s%s", runID, task, final, journalDetail(ev.Detail, err))
}

// journalDetail adds a run's detail and a store error to a journal line.
func journalDetail(detail string, err error) string {
	out := ""
	if detail != "" {
		out += "; " + detail
	}
	if err != nil {
		out += "; not recorded: " + err.Error()
	}
	return out
}

// taskLingerAfter is how long a worker may stay up after its final message
// before its group's processes are recorded, and how often they are
// recorded again while it stays (T-172).
// It is atomic so tests can shorten it while watches run.
var taskLingerAfter atomic.Int64

func init() { taskLingerAfter.Store(int64(time.Minute)) }

// taskQuietAfter is separate from the grace period for an ended turn.
const taskQuietAfter = 10 * time.Minute

func (s *Supervisor) watchCodexSilence(run string, live *liveRun, obs platform.Observation, done <-chan struct{}) {
	tick := time.NewTicker(core.RunOutputStep())
	defer tick.Stop()
	var notified int64
	for {
		select {
		case <-done:
			return
		case at := <-tick.C:
			s.observeCodexSilence(run, live, obs, at, &notified)
		}
	}
}

func (s *Supervisor) observeCodexSilence(run string, live *liveRun, obs platform.Observation, at time.Time, notified *int64) {
	last := live.lastOutput.Load()
	elapsed := at.Sub(time.Unix(0, last))
	if last == *notified || elapsed < taskQuietAfter || live.turnEnded.Load() || live.activeCommands.Load() != 0 {
		return
	}
	current, err := platform.ReadObservation(obs.PID)
	if err != nil || current != obs || live.lastOutput.Load() != last || live.turnEnded.Load() || live.activeCommands.Load() != 0 {
		return
	}
	// Each uninterrupted quiet episode has one advisory. It has no effect
	// on the task phase or the worker's reservation.
	*notified = last
	if err := s.store.NotifyCodexQuiet(run, elapsed); err != nil {
		journal.Printf("run %s: cannot record worker silence: %v", run, err)
	}
}

// watchLinger records which processes keep a run going after the worker's
// final message. It only describes the run: by D-021 and Q-030 the run still
// ends only on an observed exit and an empty group.
// leftoverNames lists what still runs in a run's group after its worker
// exited (D-147). It is empty when the list is unclear: it could not be read,
// it is empty, or the worker's own process is still among them.
func leftoverNames(pid, pgid int) string {
	members, err := groupMembers(pgid)
	if err != nil || len(members) == 0 {
		return ""
	}
	names := make([]string, 0, len(members))
	for _, m := range members {
		if m.PID == pid {
			return ""
		}
		names = append(names, fmt.Sprintf("%s (%d)", m.Name, m.PID))
	}
	out := strings.Join(names, ", ")
	if len(out) > 300 {
		out = out[:300] + "…"
	}
	return out
}

func (s *Supervisor) watchLinger(runID, task, provider string, obs platform.Observation, done <-chan struct{}) {
	logged, notified := false, false
	for {
		select {
		case <-done:
			return
		case <-time.After(time.Duration(taskLingerAfter.Load())):
		}
		members, err := platform.GroupMembers(obs.PGID)
		if err != nil {
			continue
		}
		procs := make([]core.RunProcess, 0, len(members))
		names := ""
		workerPresent := false
		for _, m := range members {
			worker := m.PID == obs.PID
			workerPresent = workerPresent || worker
			procs = append(procs, core.RunProcess{PID: m.PID, Name: m.Name, Worker: worker})
			if len(names) < 300 {
				names += fmt.Sprintf(" %s(%d)", m.Name, m.PID)
			}
		}
		select {
		case <-done:
			return
		default:
		}
		_ = s.store.NoteRunProcesses(runID, procs)
		if !logged {
			logged = true
			journal.Printf("run %s of %s: final message written, worker still running; group:%s", runID, task, names)
		}
		if provider == "codex" && !notified && workerPresent {
			current, err := platform.ReadObservation(obs.PID)
			if err == nil && current == obs {
				if err := s.store.NotifyCodexStillRunning(runID); err != nil {
					journal.Printf("run %s of %s: cannot record worker exit delay: %v", runID, task, err)
				} else {
					notified = true
				}
			}
		}
	}
}
