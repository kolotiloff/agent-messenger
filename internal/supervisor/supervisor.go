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
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
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
	store    *core.Store
	root     string
	launcher Launcher
	cancel   context.CancelFunc

	mu           sync.Mutex
	current      *activeJob
	beforeSignal func()
}

type activeJob struct {
	token string
	pgid  int
	alias string
	wait  func() (int, error)
}

func Run(parent context.Context, root string, launcher Launcher) error {
	if launcher == nil {
		launcher = AdapterLauncher{}
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
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
	store, err := core.Open(filepath.Join(root, dbName))
	if err != nil {
		return err
	}
	defer store.Close()
	if _, err := store.RevokeInherited(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	sup := &Supervisor{store: store, root: root, launcher: launcher, cancel: cancel}
	ln, err := control.Listen(root)
	if err != nil {
		return err
	}
	defer ln.Close()
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sup.loop(ctx)
	}()
	err = control.Serve(ln, sup)
	cancel()
	wg.Wait()
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
		if !s.launcher.Enabled() {
			continue
		}
		token, err := s.store.ClaimNext()
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
	started, err := s.launcher.Start(spec)
	if err != nil {
		_ = s.store.MarkUncertain(token, err.Error())
		return
	}
	s.mu.Lock()
	s.current = &activeJob{token: token, pgid: started.PGID, alias: spec.Alias, wait: started.Wait}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.current = nil
		s.mu.Unlock()
	}()
	obs := platform.Observation{PID: started.PID, PGID: started.PGID, StartToken: started.StartToken, BootID: started.BootID}
	if err := s.store.RecordProcess(token, obs); err != nil {
		_ = s.store.MarkUncertain(token, err.Error())
		_, _ = started.Wait()
		return
	}
	reported, err := readStartupID(started.Stdout, spec.Provider)
	if err != nil || reported == "" || reported != spec.ConversationID {
		if reported == "" {
			reported = "missing"
		}
		_ = s.store.ConfirmIdentity(token, spec.Provider, spec.Environment, reported)
		_, _ = started.Wait()
		return
	}
	if err := s.store.ConfirmIdentity(token, spec.Provider, spec.Environment, reported); err != nil {
		_, _ = started.Wait()
		return
	}
	exitCode, err := started.Wait()
	if err != nil {
		_ = s.store.MarkUncertain(token, "wait failed")
		return
	}
	groupEmpty, groupErr := platform.GroupEmpty(started.PGID)
	detail := "empty process group is not descendant proof"
	if groupErr != nil {
		groupEmpty = false
		detail = groupErr.Error()
	}
	// groupEmpty is recorded and does not unlock the session.
	_, _ = s.store.FinishObserved(token, exitCode, groupEmpty, detail)
	_ = ctx
}

func readStartupID(r io.Reader, provider string) (string, error) {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadString('\n')
		if strings.TrimSpace(line) != "" {
			var event map[string]any
			if json.Unmarshal([]byte(line), &event) == nil {
				id, idErr := adapters.StartupID(provider, event)
				if idErr != nil {
					return "", idErr
				}
				if id != "" {
					return id, nil
				}
			}
		}
		if err != nil {
			return "", err
		}
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
			}, nil
		}
	}
	return Spec{}, errors.New("claimed session disappeared")
}

func (s *Supervisor) Status() (control.Status, error) {
	snap, err := s.store.Snapshot()
	if err != nil {
		return control.Status{}, err
	}
	status := control.Status{
		Paused:     snap.Paused,
		CLILaunch:  "disabled",
		Broker:     "not_configured",
		Supervisor: os.Getpid(),
	}
	if labeled, ok := s.launcher.(interface{ LaunchLabel() string }); ok {
		status.CLILaunch = labeled.LaunchLabel()
	} else if s.launcher.Enabled() {
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
	return status, nil
}

func (s *Supervisor) Pause() error  { return s.store.Pause() }
func (s *Supervisor) Resume() error { return s.store.ResumeWork() }

func (s *Supervisor) StopJob() (control.StopResult, error) {
	s.mu.Lock()
	job := s.current
	s.mu.Unlock()
	result := control.StopResult{State: "uncertain", Signal: "not_recorded", Note: "сигнал не доказывает остановку потомков"}
	token := ""
	if job != nil {
		token = job.token
	} else {
		snap, err := s.store.Snapshot()
		if err != nil {
			return result, err
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

func (s *Supervisor) Resolve(alias, action, confirmation string) (string, error) {
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
