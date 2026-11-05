// Package supervisor is the single local client process.
// It owns CLI launches. Control commands reach it through the local socket.
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
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const (
	dbName   = "queue.sqlite"
	lockName = "supervisor.lock"
)

// Launcher starts a bound CLI. The product binary uses DisabledLauncher so tests
// and an ordinary serve do not call real models.
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

	mu      sync.Mutex
	current *activeJob
}

type activeJob struct {
	token string
	pgid  int
	alias string
	wait  func() (int, error)
}

func Run(parent context.Context, root string, launcher Launcher) error {
	if launcher == nil {
		launcher = DisabledLauncher{}
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return err
	}
	lock, err := platform.Acquire(filepath.Join(root, lockName))
	if err != nil {
		return err
	}
	defer lock.Release()
	store, err := core.Open(filepath.Join(root, dbName))
	if err != nil {
		return err
	}
	defer store.Close()
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
	go sup.loop(ctx)
	err = control.Serve(ln, sup)
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
	reader := bufio.NewReader(started.Stdout)
	line, err := reader.ReadString('\n')
	if err != nil {
		_ = s.store.MarkUncertain(token, "identity line was not read")
		_, _ = started.Wait()
		return
	}
	var reported string
	if _, err := fmt.Sscanf(line, "session %s", &reported); err != nil || reported != spec.ConversationID {
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
	detail := "group checked by the waiting supervisor"
	if groupErr != nil {
		groupEmpty = false
		detail = groupErr.Error()
	}
	_, _ = s.store.FinishObserved(token, exitCode, groupEmpty, detail)
	_ = ctx
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
				Provider:       fmt.Sprint(session["provider"]),
				Environment:    fmt.Sprint(session["environment"]),
				ConversationID: fmt.Sprint(session["conversation_id"]),
				Alias:          alias,
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
	if s.launcher.Enabled() {
		status.CLILaunch = "test-launcher"
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
	if obs.PGID > 1 {
		if err := platform.SignalGroup(obs.PGID); err != nil {
			if errors.Is(err, platform.ErrUnsupported) {
				result.Signal = "unsupported"
			} else {
				result.Signal = "error"
			}
			result.Note = err.Error() + "; " + result.Note
		} else {
			result.Signal = "sent"
		}
	}
	if err := s.store.MarkUncertain(token, "stop-job"); err != nil {
		return result, err
	}
	return result, nil
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
