// Package supervisor is the single local client process.
// It owns task worker launches. Control commands reach it through the local
// socket.
package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/broker"
	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const dbName = "queue.sqlite"

// Launcher decides whether this client may launch task workers. Production
// serve uses AdapterLauncher; tests inject their own.
type Launcher interface {
	TasksEnabled() bool
}

// AdapterLauncher is the production launcher. AllowTasks is set by
// `serve --allow-tasks`; without it tasks are only recorded.
type AdapterLauncher struct {
	AllowTasks bool
}

// TasksEnabled is set by serve --allow-tasks.
func (l AdapterLauncher) TasksEnabled() bool { return l.AllowTasks }

// DisabledLauncher never launches a task worker.
type DisabledLauncher struct{}

func (DisabledLauncher) TasksEnabled() bool { return false }

type Supervisor struct {
	store    *core.Store
	root     string
	launcher Launcher
	cancel   context.CancelFunc
	ctx      context.Context

	dispatch sync.Mutex
	stopping atomic.Bool
	tasks    taskRuns
	server   *broker.Service
	net      netState
	// startUpdate starts the update helper for another computer's request
	// (T-078); nil when this client cannot be updated that way.
	startUpdate UpdateStarter
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
	store, err := core.Open(filepath.Join(root, dbName))
	if err != nil {
		return err
	}
	defer store.Close()
	// Reservations of another data directory, including old ones of the
	// removed persistent mode, keep this client from starting; old ones of
	// this data directory stay in the registry untouched (D-104).
	if err := reconcileRuns(root); err != nil {
		return err
	}
	if err := store.InterruptInheritedTaskRuns(); err != nil {
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
	}
}

func (s *Supervisor) currentLauncher() Launcher {
	if s.launcher == nil {
		return DisabledLauncher{}
	}
	return s.launcher
}

func (s *Supervisor) Status() (control.Status, error) {
	paused, err := s.store.Paused()
	if err != nil {
		return control.Status{}, err
	}
	status := control.Status{
		Paused:     paused,
		CLILaunch:  "disabled",
		Broker:     "not_configured",
		Supervisor: os.Getpid(),
		Version:    buildinfo.Version,
	}
	if s.server != nil {
		status.Broker = s.server.Summary()
	}
	if s.currentLauncher().TasksEnabled() {
		status.TasksEnabled = true
		status.CLILaunch = "tasks"
	}
	reservations := ReadReservations()
	status.Reservations = &reservations
	return status, nil
}

func (s *Supervisor) Pause() error {
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	return s.store.Pause()
}

func (s *Supervisor) Resume() error { return s.store.ResumeWork() }

func (s *Supervisor) Shutdown() error {
	s.dispatch.Lock()
	s.stopping.Store(true)
	s.dispatch.Unlock()
	// Delay only long enough for the command response to be written.
	// It is not a timer that proves a CLI process has stopped.
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.cancel()
	}()
	return nil
}
