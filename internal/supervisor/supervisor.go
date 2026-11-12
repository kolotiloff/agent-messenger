// Package supervisor is the single local client process.
// It owns CLI launches. Control commands reach it through the local socket.
package supervisor

import (
	"bufio"
	"context"
	"encoding/json"
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
	"github.com/kolotiloff/agent-messenger/internal/broker"
	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/journal"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const dbName = "queue.sqlite"

// Launcher starts a bound CLI. Production serve uses AdapterLauncher.
// Tests inject a launcher that runs a synthetic process instead of a model.
type Launcher interface {
	Enabled() bool
	Start(spec Spec) (Started, error)
}

type Spec struct {
	Token          string
	DB             string
	Provider       string
	Environment    string
	ConversationID string
	Alias          string
	CLI            string
	Workspace      string
	DataRoot       string
	NewSession     bool
}

type Started struct {
	PID        int
	PGID       int
	StartToken string
	BootID     string
	Stdout     io.ReadCloser
	Wait       func() (int, error)
}

// DisabledLauncher refuses to execute an external CLI.
type DisabledLauncher struct{}

func (DisabledLauncher) Enabled() bool { return false }

func (DisabledLauncher) Start(Spec) (Started, error) {
	return Started{}, errors.New("запуск реальных CLI выключен")
}

type Supervisor struct {
	// bg counts background work that reads the store, so it ends before
	// the store is closed.
	bg         sync.WaitGroup
	store      *core.Store
	root       string
	launcher   Launcher
	launcherMu sync.RWMutex
	cancel     context.CancelFunc
	ctx        context.Context

	mu          sync.Mutex
	jobs        map[string]*activeJob
	streams     sync.WaitGroup
	liveStreams atomic.Int32
	// streamOpening counts stream controllers that have not finished starting
	// their process. Only one may start at a time: the startup lease is
	// exclusive, and a second start would fail and leave its message
	// uncertain.
	streamOpening atomic.Int32
	beforeSignal  func()
	dispatch      sync.Mutex
	stopping      atomic.Bool
	starting      atomic.Bool
	codex         codexOwners
	tasks         taskRuns
	server        *broker.Service
	net           netState
	// startUpdate starts the update helper for another computer's request
	// (T-078); nil when this client cannot be updated that way.
	startUpdate UpdateStarter
}

type activeJob struct {
	token    string
	pgid     int
	alias    string
	wait     func() (int, error)
	streamID string
	cancel   context.CancelFunc
}

func Run(parent context.Context, root string, launcher Launcher) error {
	return RunWithOptions(parent, root, launcher, RunOptions{})
}

// RunOptions controls supervisor startup, never provider recovery. A detached
// client starts paused and acknowledges readiness before its dispatch loop runs.
type RunOptions struct {
	StartPaused bool
	Ready       func() error
	// StartUpdate starts the update helper when another computer of the
	// network asks this one to update (T-078); nil refuses such requests.
	StartUpdate UpdateStarter
}

func RunWithOptions(parent context.Context, root string, launcher Launcher, options RunOptions) error {
	if launcher == nil {
		launcher = AdapterLauncher{}
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	// Keep the original spelling for the Unix socket: resolving /tmp to
	// /private/tmp can exceed macOS socket-path limits. Ownership uses the
	// physical path below; both socket spellings still address the same file.
	socketRoot := root
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root, err = core.CanonicalEnvironment(absoluteRoot)
	if err != nil {
		return err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return err
	}
	lockPath, err := platform.SupervisorLockFile()
	if err != nil {
		return err
	}
	lock, err := platform.Acquire(lockPath)
	if err != nil {
		return err
	}
	defer lock.Release()
	if err := checkStartupLease(); err != nil {
		return err
	}
	// T-142: only the one running client writes the journal.
	journal.Open(root)
	defer journal.Close()
	journal.Printf("client started, version %s", buildinfo.Current().Version)
	defer journal.Printf("client stopped")
	store, err := core.Open(filepath.Join(root, dbName))
	if err != nil {
		return err
	}
	defer store.Close()
	if err := store.CheckPendingStreamRecovery(); err != nil {
		return err
	}
	if err := store.CheckPendingCodexRecovery(); err != nil {
		return err
	}
	if err := reconcileRuns(root); err != nil {
		return err
	}
	if _, err := store.RevokeInherited(); err != nil {
		return err
	}
	if options.StartPaused {
		if err := store.Pause(); err != nil {
			return err
		}
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	sup := &Supervisor{store: store, root: root, launcher: launcher, cancel: cancel, ctx: ctx, startUpdate: options.StartUpdate}
	defer sup.bg.Wait()
	defer sup.codex.closeAll()
	server := broker.NewService(ctx, root)
	sup.server = server
	defer server.Close()
	ln, err := control.Listen(socketRoot)
	if err != nil {
		return err
	}
	defer ln.Close()
	if options.Ready != nil {
		if err := options.Ready(); err != nil {
			return err
		}
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	// The hosted broker starts only if the owner turned it on; a failure is
	// shown in the window and does not stop the client.
	server.Start()
	server.SetPresence(sup.presence)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		server.RunPresence(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		sup.runNetwork(ctx)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		sup.loop(ctx)
	}()
	err = control.Serve(ln, sup)
	cancel()
	wg.Wait()
	sup.streams.Wait()
	sup.tasks.wg.Wait()
	if ctx.Err() != nil {
		return nil
	}
	return err
}

func (s *Supervisor) loop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		s.dispatchTasks(ctx)
		launcher := s.currentLauncher()
		if s.stopping.Load() || s.starting.Load() || !launcher.Enabled() {
			continue
		}
		if stream, ok := launcher.(StreamLauncher); ok && stream.StreamingEnabled() {
			if s.liveStreams.Load() >= maxClaudeStreams || s.streamOpening.Load() > 0 {
				continue
			}
			var token string
			var err error
			if aliases := scopedAliases(launcher); aliases != nil {
				token, err = claimFirst(aliases, func(alias string) (string, error) { return s.store.ClaimProvider(alias, "claude") })
			} else {
				token, err = s.store.ClaimNextProvider("claude")
			}
			if err != nil {
				continue
			}
			// Each persistent Claude session has its own controller, like Codex
			// owners, so one live stream does not starve another session.
			s.streams.Add(1)
			s.liveStreams.Add(1)
			s.streamOpening.Add(1)
			go func() {
				defer s.streams.Done()
				defer s.liveStreams.Add(-1)
				s.runStream(ctx, token, stream)
			}()
			continue
		}
		var token string
		var err error
		if aliases := scopedAliases(launcher); aliases != nil {
			token, err = claimFirst(aliases, s.store.Claim)
		} else {
			token, err = s.store.ClaimNext()
		}
		if err != nil {
			continue
		}
		s.runClaimed(ctx, token)
	}
}

func (s *Supervisor) runClaimed(ctx context.Context, token string) {
	if err := s.store.BeginLaunch(token); err != nil {
		_ = s.store.MarkUncertain(token, err.Error())
		return
	}
	spec, err := s.spec(token)
	if err != nil {
		_ = s.store.MarkUncertain(token, err.Error())
		return
	}
	started, err := s.currentLauncher().Start(spec)
	if err != nil {
		_ = s.store.MarkUncertain(token, err.Error())
		return
	}
	s.addJob(&activeJob{token: token, pgid: started.PGID, alias: spec.Alias, wait: started.Wait})
	defer s.removeJob(token)
	obs := platform.Observation{PID: started.PID, PGID: started.PGID, StartToken: started.StartToken, BootID: started.BootID}
	if err := s.store.RecordProcess(token, obs); err != nil {
		_ = s.store.MarkUncertain(token, err.Error())
		_, _ = started.Wait()
		return
	}
	type collected struct {
		events []map[string]any
		err    error
	}
	done := make(chan collected, 1)
	go func() {
		events, err := s.collectEvents(ctx, started.Stdout, spec)
		done <- collected{events, err}
	}()
	exitCode, waitErr, detached := waitUnlessStopped(ctx, started.Wait)
	if detached {
		// The CLI keeps running. Shutdown must not wait for it or hold the lock.
		// The user-level reservation stays, including when only a descendant remains.
		return
	}
	got := <-done
	events, readErr := got.events, got.err
	if waitErr != nil && readErr == nil {
		readErr = waitErr
	}
	detail := "empty process group is not descendant proof"
	if readErr != nil {
		detail = readErr.Error()
	}
	providerErr := validateProvider(spec.Provider, events, spec.ConversationID)
	if providerErr != nil {
		detail = providerErr.Error() + "; " + detail
	}
	groupEmpty, groupErr := platform.GroupEmpty(started.PGID)
	if groupErr != nil {
		groupEmpty = false
		detail = groupErr.Error() + "; " + detail
	}
	providerOK := providerErr == nil && readErr == nil && exitCode == 0
	// groupEmpty is recorded and does not unlock the session.
	_, _ = s.store.FinishObserved(token, exitCode, groupEmpty, providerOK, detail)
}

func waitUnlessStopped(ctx context.Context, wait func() (int, error)) (int, error, bool) {
	finished := make(chan struct{})
	var code int
	var err error
	go func() {
		code, err = wait()
		close(finished)
	}()
	select {
	case <-finished:
		return code, err, false
	case <-ctx.Done():
		go func() { <-finished }()
		return 0, ctx.Err(), true
	}
}

func (s *Supervisor) collectEvents(ctx context.Context, r io.Reader, spec Spec) ([]map[string]any, error) {
	reader := bufio.NewReader(r)
	var events []map[string]any
	seen := false
	var firstErr error
	for {
		line, err := reader.ReadString('\n')
		if strings.TrimSpace(line) != "" {
			var event map[string]any
			if json.Unmarshal([]byte(line), &event) == nil {
				events = append(events, event)
				if !seen {
					id, idErr := adapters.StartupID(spec.Provider, event)
					if idErr != nil {
						firstErr = idErr
					}
					if id != "" {
						seen = true
						if ctx.Err() != nil {
							break
						}
						if id != spec.ConversationID {
							id = "mismatch"
						}
						if callErr := s.store.ConfirmIdentity(spec.Token, spec.Provider, spec.Environment, id); callErr != nil && firstErr == nil {
							firstErr = callErr
						}
					}
				}
			}
		}
		if err != nil {
			if !seen && firstErr == nil {
				firstErr = err
			}
			break
		}
	}
	if !seen && firstErr == nil {
		firstErr = errors.New("startup id was not observed")
	}
	return events, firstErr
}

func validateProvider(provider string, events []map[string]any, conversationID string) error {
	switch provider {
	case "claude":
		return adapters.ValidateClaude(events, conversationID)
	case "codex":
		return adapters.ValidateCodex(events, conversationID)
	case "cursor":
		return adapters.ValidateCursor(events, conversationID)
	default:
		return nil
	}
}

func (s *Supervisor) spec(token string) (Spec, error) {
	snap, err := s.store.Snapshot()
	if err != nil {
		return Spec{}, err
	}
	var sessionKey, alias string
	for _, attempt := range snap.Attempts {
		if fmt.Sprint(attempt["token"]) == token {
			sessionKey = fmt.Sprint(attempt["session_key"])
			alias = fmt.Sprint(attempt["alias"])
		}
	}
	for _, session := range snap.Sessions {
		if fmt.Sprint(session["key"]) == sessionKey {
			return Spec{
				Token:          token,
				DB:             filepath.Join(s.root, dbName),
				Provider:       text(session["provider"]),
				Environment:    text(session["environment"]),
				ConversationID: text(session["conversation_id"]),
				Alias:          alias,
				CLI:            text(session["cli_path"]),
				Workspace:      text(session["workspace"]),
				DataRoot:       s.root,
				NewSession:     text(session["start_mode"]) == "new",
			}, nil
		}
	}
	return Spec{}, errors.New("claimed session disappeared")
}

func (s *Supervisor) Status() (control.Status, error) {
	launcher := s.currentLauncher()
	snap, err := s.store.Snapshot()
	if err != nil {
		return control.Status{}, err
	}
	status := control.Status{
		Paused:     snap.Paused,
		CLILaunch:  "disabled",
		Broker:     "not_configured",
		Supervisor: os.Getpid(),
		Version:    buildinfo.Version,
		Sandbox:    AgentSandbox(),
	}
	if s.server != nil {
		status.Broker = s.server.Summary()
	}
	if aliases := scopedAliases(launcher); len(aliases) != 0 {
		status.LaunchSession = aliases[0]
		status.LaunchSessions = aliases
	}
	if stream, ok := launcher.(StreamLauncher); ok {
		status.SessionSetup = stream.StreamingEnabled()
	}
	if adapter, ok := launcher.(AdapterLauncher); ok {
		status.SessionLaunch = adapter.ClaudeStream && (!adapter.AllowModels || adapter.OnlySession != "")
	}
	if tasks, ok := launcher.(TaskLauncher); ok {
		status.TasksEnabled = tasks.TasksEnabled()
	}
	if labeled, ok := launcher.(interface{ LaunchLabel() string }); ok {
		status.CLILaunch = labeled.LaunchLabel()
	} else if launcher.Enabled() {
		status.CLILaunch = "enabled"
	}
	for _, session := range snap.Sessions {
		if session["active_token"] != nil {
			status.Held++
		}
	}
	for _, request := range snap.Requests {
		if request["state"] == "queued" {
			status.Queued++
		}
	}
	if len(snap.StreamRuns) > 0 {
		status.StreamState = text(snap.StreamRuns[len(snap.StreamRuns)-1]["state"])
	}
	return status, nil
}

func (s *Supervisor) Pause() error {
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	return s.store.Pause()
}
func (s *Supervisor) Resume() error { return s.store.ResumeWork() }

func (s *Supervisor) StopJob() (control.StopResult, error) {
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	jobs := s.jobList()
	result := control.StopResult{State: "uncertain", Signal: "not_recorded", Note: "сигнал не доказывает остановку потомков"}
	// Every live persistent stream is stopped; one-shot jobs are handled below.
	var job *activeJob
	streams := 0
	for _, j := range jobs {
		if j.streamID == "" {
			job = j
			continue
		}
		streams++
		signal, err := s.stopStreamJob(j)
		if err != nil {
			result.Signal = signal
			return result, err
		}
		if result.Signal != "sent" {
			result.Signal = signal
		}
	}
	if streams > 0 {
		return result, nil
	}
	token := ""
	if job != nil {
		token = job.token
	} else {
		snap, err := s.store.Snapshot()
		if err != nil {
			return result, err
		}
		for _, run := range snap.StreamRuns {
			id := text(run["id"])
			obs, err := s.store.Observation(id)
			if err != nil {
				continue
			}
			if owns, err := platform.OwnsObservation(obs); err == nil && owns {
				if err := s.store.StopStream(id, "stop-job after restart"); err != nil {
					return result, err
				}
				if err := platform.SignalGroup(obs.PGID); err != nil {
					return result, err
				}
				result.Signal = "sent"
				return result, nil
			}
		}
		for _, session := range snap.Sessions {
			if session["active_token"] != nil {
				token = fmt.Sprint(session["active_token"])
			}
		}
		if token == "" {
			return result, errors.New("нет активного задания")
		}
	}
	obs, err := s.store.Observation(token)
	if err != nil {
		return result, err
	}
	owned, ownErr := platform.OwnsObservation(obs)
	if ownErr != nil {
		owned = false
		if errors.Is(ownErr, platform.ErrUnsupported) {
			result.Signal = "unsupported"
		} else {
			result.Signal = "not_owner"
		}
		result.Note = ownErr.Error() + "; номер группы без совпадения владельца не используется"
	}
	// Revoke MCP before any signal so the process cannot store a reply after stop.
	if err := s.store.MarkUncertain(token, "stop-job"); err != nil {
		return result, err
	}
	if s.beforeSignal != nil {
		s.beforeSignal()
	}
	if !owned {
		if result.Signal == "not_recorded" {
			result.Signal = "not_owner"
		}
		return result, nil
	}
	if err := platform.SignalGroup(obs.PGID); err != nil {
		if errors.Is(err, platform.ErrUnsupported) {
			result.Signal = "unsupported"
		} else {
			result.Signal = "error"
		}
		result.Note = err.Error() + "; " + result.Note
		return result, nil
	}
	result.Signal = "sent"
	return result, nil
}

func (s *Supervisor) RegisterSession(provider, environment, conversationID, alias, cliPath, workspace string) (string, error) {
	key, err := s.store.Register(provider, environment, conversationID, alias)
	if err != nil {
		return "", err
	}
	if err := s.store.ConfigureLaunch(alias, cliPath, workspace); err != nil {
		return "", err
	}
	return key, nil
}

func text(value any) string {
	if value == nil {
		return ""
	}
	s := fmt.Sprint(value)
	if s == "<nil>" {
		return ""
	}
	return s
}

func (s *Supervisor) Shutdown() error {
	s.dispatch.Lock()
	s.stopping.Store(true)
	for _, job := range s.jobList() {
		if job.streamID == "" {
			continue
		}
		err := s.store.StopStream(job.streamID, "shutdown")
		job.cancel()
		if err != nil {
			s.dispatch.Unlock()
			return err
		}
	}
	s.dispatch.Unlock()
	// Delay only long enough for the command response to be written.
	// It is not a timer that proves a CLI process has stopped.
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.cancel()
	}()
	return nil
}

func (s *Supervisor) Enqueue(alias, id, sender, text string) (bool, error) {
	return s.store.Enqueue(id, alias, sender, text)
}

func (s *Supervisor) Inspect() (any, error) { return s.store.Snapshot() }

func (s *Supervisor) Overview() (any, error) { return s.store.Overview() }

func (s *Supervisor) Timeline(alias string, limit int, before string) (any, error) {
	return s.store.Timeline(alias, limit, before)
}

func (s *Supervisor) Resolve(alias, action, confirmation string) (string, error) {
	if run, err := s.store.StreamRunForAlias(alias); err != nil {
		return "", err
	} else if run != "" {
		return "", fmt.Errorf("%w: persistent process remains reserved; no automatic retry or unlock", core.ErrRecoveryBlocked)
	}
	if owned, err := s.store.CodexOwnedAlias(alias); err == nil && owned {
		return "", fmt.Errorf("%w: a Codex owner process holds this session; resolution is not implemented", core.ErrRecoveryBlocked)
	}
	switch action {
	case "fail":
		if err := s.store.FailClosed(alias); err != nil {
			return "", err
		}
		return "сессия закрыта без новой попытки; история сохранена", nil
	case "accept", "retry":
		token, err := s.store.ActiveToken(alias)
		if err != nil {
			return "", err
		}
		obs, err := s.store.Observation(token)
		if err != nil {
			return "", err
		}
		proof, err := platform.CurrentProof(obs)
		if action == "accept" {
			if err := s.store.Accept(alias, proof); err != nil {
				return "", err
			}
			return proof.Detail, nil
		}
		id, err := s.store.Retry(alias, confirmation, proof)
		if err != nil {
			return "", err
		}
		return "новая попытка " + id + "; действия могут повториться", nil
	default:
		return "", errors.New("неизвестное действие разбора")
	}
}

// PrepareCommand is a helper for tests that build a process in its own group
// without executing a model CLI.
func PrepareCommand(cmd *exec.Cmd) error { return platform.SetNewGroup(cmd) }

// SetupNewClaude creates an isolated workspace and atomically records the exact
// idempotent setup operation. It never queues a message or changes permissions.
func (s *Supervisor) SetupNewClaude(environment, id, alias, cli, workspace string) error {
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	stream, ok := s.currentLauncher().(StreamLauncher)
	if !ok || !stream.StreamingEnabled() || s.stopping.Load() {
		return errors.New("new-session setup requires an available streaming client")
	}
	if !core.ValidSetupID(id) || workspace != "" {
		return core.ErrRejected
	}
	parent := filepath.Join(s.root, "workspaces")
	workspace = filepath.Join(parent, id)
	for _, dir := range []string{parent, workspace} {
		if err := os.Mkdir(dir, 0700); err != nil && !os.IsExist(err) {
			return err
		}
		info, err := os.Lstat(dir)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return core.ErrRejected
		}
	}
	return s.store.SetupNewClaude(environment, id, alias, cli, workspace)
}

// CreateClaudeSession allocates a new exact ID without calling a model.
func (s *Supervisor) CreateClaudeSession(environment, alias, cli, workspace string) (string, error) {
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	stream, ok := s.currentLauncher().(StreamLauncher)
	if !ok || !stream.StreamingEnabled() {
		return "", errors.New("create-claude-session requires serve --claude-stream")
	}
	if s.stopping.Load() {
		return "", errors.New("client is shutting down")
	}
	if workspace == "" {
		return "", errors.New("new Claude session requires an isolated workspace")
	}
	id, err := newRunID()
	if err != nil {
		return "", err
	}
	id = id[:8] + "-" + id[8:12] + "-4" + id[13:16] + "-8" + id[17:20] + "-" + id[20:]
	if _, err := s.RegisterSession("claude", environment, id, alias, cli, workspace); err != nil {
		return "", err
	}
	if err := s.store.SetNewClaudeSession(alias); err != nil {
		return "", err
	}
	return "new Claude session registered; model not called", nil
}

func (s *Supervisor) PrepareUIMessage(id, alias, text string) error {
	return s.store.PrepareUIMessage(id, alias, text)
}
func (s *Supervisor) SubmitUIMessage(id string) (bool, error) { return s.store.SubmitUIMessage(id) }
func (s *Supervisor) AcknowledgeUIMessage(id string) error    { return s.store.AcknowledgeUIMessage(id) }
func (s *Supervisor) UIOutbox() (any, error)                  { return s.store.UIOutbox() }

// maxClaudeStreams bounds how many persistent Claude processes run at once.
const maxClaudeStreams = 4

func (s *Supervisor) addJob(job *activeJob) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobs == nil {
		s.jobs = map[string]*activeJob{}
	}
	s.jobs[job.token] = job
}

func (s *Supervisor) removeJob(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.jobs, token)
}

func (s *Supervisor) jobList() []*activeJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]*activeJob, 0, len(s.jobs))
	for _, job := range s.jobs {
		out = append(out, job)
	}
	return out
}

// stopStreamJob revokes the stream's tools first, then signals its group only
// if the recorded process still owns it. A signal does not prove descendants
// stopped; the reservation stays.
func (s *Supervisor) stopStreamJob(job *activeJob) (string, error) {
	if err := s.store.StopStream(job.streamID, "stop-job"); err != nil {
		return "not_recorded", err
	}
	job.cancel()
	obs, err := s.store.Observation(job.streamID)
	if err != nil {
		return "not_recorded", err
	}
	owned, err := platform.OwnsObservation(obs)
	if err != nil || !owned {
		return "not_owner", nil
	}
	if err := platform.SignalGroup(obs.PGID); err != nil {
		return "error", err
	}
	return "sent", nil
}

// scopedAliases returns the sessions a scoped launcher may run, or nil when it
// is not scoped.
func scopedAliases(launcher Launcher) []string {
	if scoped, ok := launcher.(interface{ SessionAliases() []string }); ok {
		return scoped.SessionAliases()
	}
	if scoped, ok := launcher.(interface{ SessionAlias() string }); ok && scoped.SessionAlias() != "" {
		return []string{scoped.SessionAlias()}
	}
	return nil
}

// claimFirst claims the next message of the first scoped session that has one.
func claimFirst(aliases []string, claim func(string) (string, error)) (string, error) {
	var last error = core.ErrEmpty
	for _, alias := range aliases {
		token, err := claim(alias)
		if err == nil {
			return token, nil
		}
		last = err
	}
	return "", last
}
