package supervisor

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// codexTurnTimeout bounds one turn. A timeout is uncertain, never success.
var codexTurnTimeout = 15 * time.Minute

// codexPoll is how often an idle owner checks the queue. Checking the queue
// never calls the model.
var codexPoll = 30 * time.Millisecond

// codexSetupTimeout bounds each setup RPC. Tests shorten it. A timeout is
// never treated as proof that the server did nothing.
var codexSetupTimeout = 30 * time.Second

// CodexServerSpec describes one managed app-server process.
type CodexServerSpec struct {
	SetupID     string
	Alias       string
	CLI         string
	Environment string // CODEX_HOME of the managed profile
	Home        string // private HOME for the process
	Workspace   string
	DataRoot    string
}

// CodexProcess is a started app-server with its reservation already recorded.
type CodexProcess struct {
	Observation platform.Observation
	Input       io.WriteCloser
	Output      io.ReadCloser
	Wait        func() (int, error)
	run         recordedRun
}

// CodexSetupLauncher starts managed Codex app-server processes.
type CodexSetupLauncher interface {
	CodexSetupEnabled(alias string) bool
	StartCodexServer(context.Context, CodexServerSpec) (CodexProcess, error)
}

// CodexSetupEnabled requires the same explicit authorization as any CLI launch.
func (l AdapterLauncher) CodexSetupEnabled(alias string) bool {
	return l.Allows(alias)
}

// codexEnv is an explicit environment: the managed profile, a private HOME and
// TMPDIR, and PATH. Nothing else is inherited, including credentials and
// profile overrides from the parent.
func codexEnv(spec CodexServerSpec) []string {
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/bin:/bin:/usr/sbin:/sbin"
	}
	return []string{
		"HOME=" + spec.Home,
		"CODEX_HOME=" + spec.Environment,
		"TMPDIR=" + filepath.Join(spec.Home, "tmp"),
		"PATH=" + path,
	}
}

// StartCodexServer records the reservation before exec, like StartStream. Only
// a definite OS refusal before the executable ran removes it again.
func (l AdapterLauncher) StartCodexServer(ctx context.Context, spec CodexServerSpec) (CodexProcess, error) {
	if !l.CodexSetupEnabled(spec.Alias) {
		return CodexProcess{}, errModelsNotAuthorized
	}
	args, err := adapters.CodexAppServerArgs(spec.CLI)
	if err != nil {
		return CodexProcess{}, err
	}
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = codexEnv(spec)
	cmd.Dir = spec.Workspace
	cmd.Stderr = io.Discard
	if err := platform.SetNewGroup(cmd); err != nil {
		return CodexProcess{}, err
	}
	inputRead, input, err := os.Pipe()
	if err != nil {
		return CodexProcess{}, err
	}
	output, outputWrite, err := os.Pipe()
	if err != nil {
		inputRead.Close()
		input.Close()
		return CodexProcess{}, err
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
		return CodexProcess{}, err
	}
	run := recordedRun{ID: id, Root: spec.DataRoot, Provider: "codex", Environment: spec.Environment, CodexSetupID: spec.SetupID}
	// Failure to read the epoch does not relax the reservation; such a setup
	// can then never be recovered by recover-codex.
	run.Epoch, _ = recoveryEpoch()
	if err := rememberRun(run); err != nil {
		return CodexProcess{}, errors.Join(errNotStarted, err)
	}
	if err := ctx.Err(); err != nil {
		if cleanupErr := forgetRunID(id); cleanupErr != nil {
			return CodexProcess{}, errors.Join(err, cleanupErr)
		}
		return CodexProcess{}, errors.Join(errNotStarted, err)
	}
	if err := cmd.Start(); err != nil {
		if cmd.Process == nil && (errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ENOEXEC) || errors.Is(err, syscall.ENOTDIR)) {
			if cleanupErr := forgetRunID(id); cleanupErr != nil {
				return CodexProcess{}, errors.Join(err, cleanupErr)
			}
			return CodexProcess{}, errors.Join(errNotStarted, err)
		}
		return CodexProcess{}, err
	}
	obs, err := platform.ReadObservation(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return CodexProcess{}, err
	}
	run.PID, run.PGID, run.StartToken, run.BootID = obs.PID, obs.PGID, obs.StartToken, obs.BootID
	if err := rememberRun(run); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return CodexProcess{}, err
	}
	started = true
	return CodexProcess{Observation: obs, Input: input, Output: output, run: run, Wait: func() (int, error) {
		err := cmd.Wait()
		if cmd.ProcessState == nil {
			return -1, fmt.Errorf("no wait status: %w", err)
		}
		return cmd.ProcessState.ExitCode(), nil
	}}, nil
}

// codexOwner is a live app-server that owns a created thread.
type codexOwner struct {
	setupID     string
	fingerprint string
	proc        CodexProcess
	rpc         *adapters.CodexRPC
}

type codexOwners struct {
	mu     sync.Mutex
	owners map[string]*codexOwner
	closed bool
}

// whileOpen runs a late store write from a watcher goroutine only while the
// supervisor still has its store open. After closeAll nothing is written; the
// next start marks such setups uncertain anyway.
func (o *codexOwners) whileOpen(fn func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.closed {
		fn()
	}
}

func (o *codexOwners) add(owner *codexOwner) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		_ = owner.proc.Input.Close()
		return
	}
	if o.owners == nil {
		o.owners = map[string]*codexOwner{}
	}
	o.owners[owner.setupID] = owner
}

func (o *codexOwners) remove(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.owners, id)
}

// closeAll closes the owners' stdin when the supervisor exits, which is what
// process exit does in production. The servers then exit; their setups stay
// uncertain on the next start. Nothing is signalled or killed.
// write runs a store write unless the supervisor is closing its store.
func (o *codexOwners) write(fn func() error) error {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed {
		return errors.New("client is shutting down")
	}
	return fn()
}

func (o *codexOwners) closeAll() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
	for _, owner := range o.owners {
		_ = owner.proc.Input.Close()
	}
}

func (o *codexOwners) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return len(o.owners)
}

func newSetupID() (string, error) {
	id, err := newRunID()
	if err != nil {
		return "", err
	}
	// Same UUIDv4 layout as CreateClaudeSession.
	return id[:8] + "-" + id[8:12] + "-4" + id[13:16] + "-8" + id[17:20] + "-" + id[20:], nil
}

func privateDir(path string) error {
	if err := os.Mkdir(path, 0o700); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return core.ErrRejected
	}
	return nil
}

// CreateCodexSession launches a managed app-server, checks its effective
// config, creates one persistent thread with the Messenger tools, and stores the
// server-issued thread ID. No model turn is started. The live process remains
// the thread's owner: an empty thread is not saved to disk, so it could not be
// resumed by another process.
//
// Message delivery to this owner is not implemented yet, so the alias cannot
// receive messages. Every failure after launching began leaves the setup
// uncertain with its alias and process reservation kept; nothing is retried.
func (s *Supervisor) CreateCodexSession(environment, alias, cli string) (string, error) {
	s.dispatch.Lock()
	launcher, ok := s.currentLauncher().(CodexSetupLauncher)
	if !ok || !launcher.CodexSetupEnabled(alias) {
		s.dispatch.Unlock()
		return "", errors.New("Codex session setup requires serve --allow-model-calls for this alias")
	}
	if s.stopping.Load() {
		s.dispatch.Unlock()
		return "", errors.New("client is shutting down")
	}
	profile, err := core.CanonicalEnvironment(environment)
	if err != nil {
		s.dispatch.Unlock()
		return "", err
	}
	if isDefaultProfile("codex", profile) {
		s.dispatch.Unlock()
		return "", errors.New("use a separate managed Codex profile, not the default one")
	}
	id, err := newSetupID()
	if err != nil {
		s.dispatch.Unlock()
		return "", err
	}
	workspace := filepath.Join(s.root, "workspaces", id)
	home := filepath.Join(s.root, "codex-home", id)
	for _, dir := range []string{filepath.Join(s.root, "workspaces"), workspace, filepath.Join(s.root, "codex-home"), home, filepath.Join(home, "tmp")} {
		if err := privateDir(dir); err != nil {
			s.dispatch.Unlock()
			return "", err
		}
	}
	if err := s.store.BeginCodexSetup(id, alias, profile, cli, workspace); err != nil {
		s.dispatch.Unlock()
		return "", err
	}
	setup, err := s.store.CodexSetup(id)
	if err != nil {
		s.dispatch.Unlock()
		return "", err
	}
	if err := s.store.MarkCodexSetupLaunching(id); err != nil {
		s.dispatch.Unlock()
		return "", err
	}
	spec := CodexServerSpec{SetupID: id, Alias: alias, CLI: setup.CLI, Environment: setup.Environment, Home: home, Workspace: setup.Workspace, DataRoot: s.root}
	proc, err := launcher.StartCodexServer(context.Background(), spec)
	s.dispatch.Unlock()
	if err != nil {
		if errors.Is(err, errNotStarted) {
			_ = s.store.CodexSetupNotStarted(id, "executable did not start")
			return "", fmt.Errorf("Codex app-server did not start: %w", err)
		}
		_ = s.store.MarkCodexSetupUncertain(id, "start outcome unknown")
		return "", fmt.Errorf("Codex app-server start uncertain; reservation kept: %w", err)
	}
	rpc := adapters.NewCodexRPC(proc.Input, proc.Output)
	fail := func(reason string, cause error) (string, error) {
		_ = s.codex.write(func() error { return s.store.MarkCodexSetupUncertain(id, reason) })
		// Closing stdin asks the server to exit. Its exit is recorded, but the
		// reservation stays: an exit does not prove descendants stopped.
		_ = proc.Input.Close()
		go func() { _ = rpc.Drain(nil) }()
		go func() {
			code, _ := proc.Wait()
			s.codex.whileOpen(func() { _ = s.store.RecordCodexSetupExit(id, code) })
		}()
		return "", fmt.Errorf("Codex setup uncertain (%s); alias and reservation kept; no retry: %w", reason, cause)
	}
	if err := s.codex.write(func() error { return s.store.RecordCodexSetupProcess(id, proc.Observation) }); err != nil {
		return fail("process not recorded", err)
	}
	call := func(method string, params any) (json.RawMessage, error) {
		ctx, cancel := context.WithTimeout(context.Background(), codexSetupTimeout)
		defer cancel()
		return rpc.Call(ctx, method, params)
	}
	if _, err := call("initialize", adapters.CodexInitializeParams(buildinfo.Version)); err != nil {
		return fail("initialize failed", err)
	}
	if err := rpc.Notify("initialized"); err != nil {
		return fail("initialized not sent", err)
	}
	result, err := call("config/read", map[string]any{"includeLayers": false, "cwd": setup.Workspace})
	if err != nil {
		return fail("config/read failed", err)
	}
	var report struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(result, &report); err != nil {
		return fail("config report invalid", err)
	}
	if err := adapters.ValidateCodexAppServerConfig(report.Config); err != nil {
		return fail("effective config rejected", err)
	}
	fingerprint, err := adapters.CodexConfigFingerprint(report.Config)
	if err != nil {
		return fail("config report invalid", err)
	}
	if err := s.codex.write(func() error { return s.store.ConfirmCodexSetupConfig(id, fingerprint) }); err != nil {
		return fail("config check not recorded", err)
	}
	if err := s.codex.write(func() error { return s.store.PrepareCodexThreadStart(id) }); err != nil {
		return fail("thread intent not recorded", err)
	}
	result, err = call("thread/start", adapters.CodexThreadStartParams(setup.Workspace))
	if err != nil {
		return fail("thread/start outcome unknown", err)
	}
	thread, err := adapters.ParseCodexThreadStart(result, setup.Workspace)
	if err != nil {
		return fail("thread/start result rejected", err)
	}
	if err := s.codex.write(func() error { return s.store.CompleteCodexSetup(id, thread) }); err != nil {
		return fail("thread id not recorded", err)
	}
	run := proc.run
	run.ConversationID = thread
	registryErr := rememberRun(run)
	owner := &codexOwner{setupID: id, proc: proc, rpc: rpc}
	s.codex.add(owner)
	go s.serveCodex(owner, alias, thread)
	go func() {
		code, _ := proc.Wait()
		s.codex.whileOpen(func() { _ = s.store.RecordCodexSetupExit(id, code) })
		s.codex.remove(id)
	}()
	detail := "Codex session created; model not called; queued messages are delivered while model calls are allowed"
	if registryErr != nil {
		detail += "; warning: reservation kept without thread id"
	}
	return detail, nil
}

// serveCodex is the single controller of one owner. While idle it drains the
// server output and claims the next queued message for its session; then it
// runs exactly one turn for it. Any turn that is not verified, and any server
// activity that is not allowed while idle, makes the owner uncertain: the
// attempt and reservation are kept, stdin is closed, and nothing is retried.
func (s *Supervisor) serveCodex(owner *codexOwner, alias, thread string) {
	ctx := s.ctx
	var workspace, fingerprint string
	if err := s.codex.write(func() error {
		setup, err := s.store.CodexSetup(owner.setupID)
		if err != nil {
			return err
		}
		workspace = setup.Workspace
		fingerprint, err = s.store.CodexConfigFingerprint(owner.setupID)
		return err
	}); err != nil {
		_ = owner.proc.Input.Close()
		return
	}
	owner.fingerprint = fingerprint
	fail := func(token, reason string) {
		_ = s.codex.write(func() error {
			if token != "" {
				return s.store.FailCodexTurn(token, reason)
			}
			return s.store.MarkCodexSetupUncertain(owner.setupID, reason)
		})
		_ = owner.proc.Input.Close()
	}
	for {
		var token string
		err := owner.rpc.WaitIdle(ctx, codexPoll, func() bool {
			launcher, ok := s.currentLauncher().(CodexSetupLauncher)
			if !ok || !launcher.CodexSetupEnabled(alias) || s.stopping.Load() {
				return false
			}
			var claimErr error
			_ = s.codex.write(func() error {
				token, claimErr = s.store.ClaimCodexTurn(owner.setupID)
				return nil
			})
			return claimErr == nil && token != ""
		})
		if err != nil {
			if ctx.Err() == nil {
				fail("", "app-server stopped or sent a request while idle")
			}
			return
		}
		// The managed profile must still have the checked config. config/read
		// does not call the model. On any doubt the turn is not sent.
		if err := s.recheckCodexConfig(owner, workspace); err != nil {
			_ = s.codex.write(func() error {
				return s.store.AbortCodexTurnUnsent(token, "Codex config not confirmed before the turn: "+err.Error())
			})
			_ = owner.proc.Input.Close()
			return
		}
		turnCtx, cancel := context.WithTimeout(ctx, codexTurnTimeout)
		err = owner.rpc.RunTurn(turnCtx, thread, adapters.CodexTurnHooks{
			Prepare: func() error {
				return s.codex.write(func() error { return s.store.PrepareCodexTurnSend(token) })
			},
			Bind: func(turn string) error {
				return s.codex.write(func() error { return s.store.BindCodexTurn(token, turn) })
			},
			Tools: adapters.CodexBoundTools{
				Read: func() (string, string, error) {
					var text, receipt string
					err := s.codex.write(func() error {
						var err error
						text, receipt, err = s.store.CodexRead(token)
						return err
					})
					return text, receipt, err
				},
				Reply: func(receipt, text string) error {
					return s.codex.write(func() error { return s.store.CodexReply(token, receipt, text) })
				},
			},
		})
		cancel()
		if err == nil {
			err = s.codex.write(func() error { return s.store.CompleteCodexTurn(token) })
		}
		if err != nil {
			fail(token, "Codex turn not verified; no retry: "+err.Error())
			return
		}
	}
}

// recheckCodexConfig reads the effective config again and requires the same
// validated content that was checked at setup.
func (s *Supervisor) recheckCodexConfig(owner *codexOwner, workspace string) error {
	ctx, cancel := context.WithTimeout(s.ctx, codexSetupTimeout)
	defer cancel()
	result, err := owner.rpc.Call(ctx, "config/read", map[string]any{"includeLayers": false, "cwd": workspace})
	if err != nil {
		return err
	}
	var report struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(result, &report); err != nil {
		return err
	}
	if err := adapters.ValidateCodexAppServerConfig(report.Config); err != nil {
		return err
	}
	fingerprint, err := adapters.CodexConfigFingerprint(report.Config)
	if err != nil {
		return err
	}
	if fingerprint != owner.fingerprint {
		return errors.New("effective config changed since setup")
	}
	return nil
}

// maxWindowSessions bounds how many sessions the window may authorize in one
// client run, for example one Claude and one Codex session that talk.
const maxWindowSessions = 4

// grantWindowAliasLocked adds alias to the window's model-call scope. The
// caller holds s.dispatch. It returns false when the alias was already allowed.
// The first grant needs an idle client; later grants only extend the list.
func (s *Supervisor) grantWindowAliasLocked(launcher AdapterLauncher, alias string) (bool, error) {
	if launcher.AllowModels && launcher.OnlySession == "" {
		return false, nil // unscoped serve: everything is already allowed
	}
	if launcher.Allows(alias) {
		return false, nil
	}
	if !launcher.AllowModels {
		if len(s.jobList()) != 0 || s.starting.Load() {
			return false, core.ErrBusy
		}
		launcher.AllowModels, launcher.OnlySession, launcher.AlsoAllowed = true, alias, nil
	} else {
		if len(launcher.SessionAliases()) >= maxWindowSessions {
			return false, errors.New("too many sessions allowed in this client run")
		}
		launcher.AlsoAllowed = append(append([]string(nil), launcher.AlsoAllowed...), alias)
	}
	s.launcherMu.Lock()
	s.launcher = launcher
	s.launcherMu.Unlock()
	return true, nil
}

// revokeWindowAlias withdraws a grant that turned out not to be used.
func (s *Supervisor) revokeWindowAlias(alias string) {
	s.launcherMu.Lock()
	defer s.launcherMu.Unlock()
	launcher, ok := s.launcher.(AdapterLauncher)
	if !ok {
		return
	}
	if launcher.OnlySession == alias {
		if len(launcher.AlsoAllowed) == 0 {
			launcher.AllowModels, launcher.OnlySession = false, ""
		} else {
			launcher.OnlySession, launcher.AlsoAllowed = launcher.AlsoAllowed[0], launcher.AlsoAllowed[1:]
		}
	} else {
		var kept []string
		for _, a := range launcher.AlsoAllowed {
			if a != alias {
				kept = append(kept, a)
			}
		}
		launcher.AlsoAllowed = kept
	}
	s.launcher = launcher
}

// SetupCodexFromWindow is the window's one-step "create a Codex session and
// allow model calls for it". The client started by the window runs without
// model permission; this grants it for exactly this new alias, like
// AuthorizeSetup does for a new Claude session, and then creates the session.
// It refuses when another session already holds the permission in this run.
// If nothing was launched, the grant is withdrawn again.
func (s *Supervisor) SetupCodexFromWindow(environment, alias, cli, confirmation string) (string, error) {
	if confirmation != "allow-model-calls" {
		return "", errors.New("explicit confirmation required")
	}
	s.dispatch.Lock()
	if s.stopping.Load() {
		s.dispatch.Unlock()
		return "", core.ErrRejected
	}
	launcher, ok := s.currentLauncher().(AdapterLauncher)
	if !ok {
		s.dispatch.Unlock()
		return "", core.ErrRejected
	}
	granted, grantErr := s.grantWindowAliasLocked(launcher, alias)
	if grantErr != nil {
		s.dispatch.Unlock()
		return "", grantErr
	}
	s.dispatch.Unlock()
	detail, err := s.CreateCodexSession(environment, alias, cli)
	if err == nil && granted {
		// Like the Claude test-session launch: the explicit window action also
		// opens the queue. Model calls stay limited to this one alias.
		if resumeErr := s.store.ResumeWork(); resumeErr != nil {
			return detail, fmt.Errorf("session created, but the queue is still paused: %w", resumeErr)
		}
	}
	if err != nil && granted {
		setup, lookupErr := s.store.CodexSetupByAlias(alias)
		if errors.Is(lookupErr, sql.ErrNoRows) || (lookupErr == nil && setup.State == "not_started") {
			s.revokeWindowAlias(alias)
		}
	}
	return detail, err
}
