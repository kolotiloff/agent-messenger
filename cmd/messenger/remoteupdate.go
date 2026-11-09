package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// Updating this computer at another computer's request (T-078, D-100). The
// background client starts the hidden `_remote-update` command from the
// program file the owner installed. It installs the newest signed update
// with the window's updater, restarts the background client from the
// installed program (keeping a pause the owner set) and writes its result
// for the client to send. It never touches active-runs.json or a
// reservation, and never retries.

// remoteUpdateLimit bounds the helper's whole run.
const remoteUpdateLimit = 20 * time.Minute

// recordProgram writes the program file this window runs from, for a later
// remote update (Windows and Linux) and for the coordinator's connection in
// Claude, which must start the program file, not the client's copy (T-117).
func recordProgram(root string) {
	executable, err := os.Executable()
	if err == nil {
		executable, err = filepath.EvalSymlinks(executable)
	}
	if err != nil {
		return
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return
	}
	_ = supervisor.WriteProgramPath(root, executable)
}

// remoteUpdateStarter is the client's hook that starts the helper; nil
// where the program does not update this way.
func remoteUpdateStarter(root string) supervisor.UpdateStarter {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return nil
	}
	return func(program string, job supervisor.RemoteUpdate) error {
		absolute, err := filepath.Abs(root)
		if err != nil {
			return err
		}
		log, err := os.CreateTemp(absolute, "remote-update-*.log")
		if err != nil {
			return err
		}
		defer log.Close()
		return startDetached(program, []string{"--root", absolute, "_remote-update", "--broker", job.Broker, "--peer", job.Peer, "--id", job.ID}, log)
	}
}

// runRemoteUpdate is the helper command.
func runRemoteUpdate(root string, args []string) error {
	fs := flag.NewFlagSet("_remote-update", flag.ContinueOnError)
	brokerID := fs.String("broker", "", "network")
	peer := fs.String("peer", "", "computer that asked")
	id := fs.String("id", "", "request id")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *brokerID == "" || *peer == "" || *id == "" {
		return errors.New("_remote-update is started by the client only")
	}
	job := supervisor.RemoteUpdate{Broker: *brokerID, Peer: *peer, ID: *id}
	ctx, cancel := context.WithTimeout(context.Background(), remoteUpdateLimit)
	defer cancel()
	u, err := newRemoteUpdate(root)
	if err != nil {
		job.State, job.Text = core.UpdateFailed, "обновление не начато: "+err.Error()
	} else {
		job.State, job.Version, job.Text = u.run(ctx)
	}
	fmt.Printf("remote update %s: %s %s %s\n", job.ID, job.State, job.Version, job.Text)
	return supervisor.FinishRemoteUpdate(root, job)
}

// remoteUpdate is one update of this computer; its steps are replaced in
// tests.
type remoteUpdate struct {
	program string // the program file this helper runs from
	running string // its version
	updater *selfupdate.Updater
	// activeWork counts the tasks that are not finished.
	activeWork func() (int, error)
	// clientVersion is the running background client's version.
	clientVersion func(ctx context.Context) (string, error)
	// restart restarts the background client from program.
	restart func(ctx context.Context, program string) error
	// record remembers a new program file (an installation's new version).
	record func(program string) error
}

func newRemoteUpdate(root string) (*remoteUpdate, error) {
	executable, err := os.Executable()
	if err == nil {
		executable, err = filepath.EvalSymlinks(executable)
	}
	if err != nil {
		return nil, err
	}
	home, err := platform.DefaultRoot()
	if err != nil {
		return nil, err
	}
	selfupdate.RemoveLeftover(executable)
	return &remoteUpdate{
		program: executable,
		running: buildinfo.Version,
		updater: newUpdater(home, executable),
		activeWork: func() (int, error) {
			store, err := core.Open(filepath.Join(root, "queue.sqlite"))
			if err != nil {
				return 0, err
			}
			defer store.Close()
			return store.ActiveWork()
		},
		clientVersion: func(ctx context.Context) (string, error) {
			c, stop := context.WithTimeout(ctx, 3*time.Second)
			defer stop()
			r, err := control.CallContext(c, root, control.Request{Cmd: "status"})
			if err != nil {
				return "", err
			}
			if !r.OK || r.Status == nil {
				return "", errors.New("клиент не ответил")
			}
			return r.Status.Version, nil
		},
		restart: func(ctx context.Context, program string) error {
			return restartClient(ctx, root, func() error { return startClientFrom(root, program) }, time.Second)
		},
		record: func(program string) error { return supervisor.WriteProgramPath(root, program) },
	}, nil
}

// run installs the update and returns the final state, the version now
// installed and a note or reason.
func (r *remoteUpdate) run(ctx context.Context) (string, string, string) {
	// Tasks may have started since the client answered.
	if n, err := r.activeWork(); err != nil {
		return core.UpdateFailed, "", "задачи на компьютере не читаются: " + err.Error()
	} else if n > 0 {
		return core.UpdateRefused, "", fmt.Sprintf("На компьютере начались задачи (%d). Обновите, когда они закончатся.", n)
	}
	busy := false
	if sync := r.updater.Sync; sync != nil {
		r.updater.Sync = func(ctx context.Context) error {
			err := sync(ctx)
			if errors.Is(err, updatefeed.ErrDownloadBusy) {
				busy = true
			}
			return err
		}
	}
	s := r.updater.Check(ctx)
	switch s.State {
	case "unsupported":
		return core.UpdateFailed, "", "эта сборка программы не обновляется сама"
	case "failed":
		why := s.Error
		if why == "" {
			why = "не удалось проверить обновления"
		}
		return core.UpdateFailed, "", why
	case "available":
		s, err := r.updater.Apply(ctx)
		if err != nil {
			return core.UpdateFailed, "", "обновление не установлено: " + err.Error()
		}
		installed := r.updater.Installed()
		if installed != r.program {
			// A new version of an installation lives in its own folder.
			if err := r.record(installed); err != nil {
				return core.UpdateFailed, s.Version, "программа обновлена до " + s.Version + ", но её новый путь не записан: " + err.Error() + "; откройте окно программы на том компьютере"
			}
		}
		if err := r.restart(ctx, installed); err != nil {
			return core.UpdateFailed, s.Version, "программа обновлена до " + s.Version + ", но фоновый клиент не перезапущен: " + err.Error()
		}
		return core.UpdateDone, s.Version, ""
	}
	if busy {
		return core.UpdateFailed, "", "окно программы на том компьютере сейчас само скачивает обновление; повторите через несколько минут"
	}
	// No newer signed release. A client older than the program file (the
	// window was updated, the client was not restarted) is restarted.
	if client, err := r.clientVersion(ctx); err == nil && selfupdate.Build(client) < selfupdate.Build(r.running) {
		if err := r.restart(ctx, r.program); err != nil {
			return core.UpdateFailed, "", "фоновый клиент не перезапущен: " + err.Error()
		}
		return core.UpdateDone, r.running, "клиент перезапущен с установленной версией"
	}
	return core.UpdateDone, r.running, "уже последняя версия"
}
