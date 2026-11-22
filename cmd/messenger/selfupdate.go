package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/distribution"
	"github.com/kolotiloff/agent-messenger/internal/localui"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
	"github.com/kolotiloff/agent-messenger/internal/supervisor"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// selfUpdateEvery is how often the window looks for a new signed release. It
// only looks: the archive is downloaded when the owner presses the button.
// GitHub allows 60 checks an hour per address, shared by every computer
// behind it, so the window checks rarely (owner, 2026-11-09).
const selfUpdateEvery = 30 * time.Minute

// relaunch remembers the program to start once the window has closed.
type relaunch struct {
	mu      sync.Mutex
	program string
}

func (r *relaunch) set(program string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.program = program
}

func (r *relaunch) get() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.program
}

// windowSelfUpdate prepares the window's update on Windows and Linux (D-048)
// and starts the background check. The running background client is not
// touched: it keeps its version until the window offers to restart it.
func windowSelfUpdate(ctx context.Context, root, home string, stop context.CancelFunc, next *relaunch) *localui.SelfUpdate {
	if runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		return nil
	}
	executable, err := os.Executable()
	if err == nil {
		executable, err = filepath.EvalSymlinks(executable)
	}
	if err != nil {
		return nil
	}
	selfupdate.RemoveLeftover(executable)
	u := newUpdater(home, executable)
	reopen := func(program string) {
		next.set(program)
		stop()
	}
	if u.Status().State != "unsupported" {
		go watchWindowUpdate(ctx, root, u, reopen)
		go func() {
			for {
				check, cancel := context.WithTimeout(ctx, 5*time.Minute)
				u.Look(check)
				cancel()
				select {
				case <-ctx.Done():
					return
				case <-time.After(selfUpdateEvery):
				}
			}
		}()
	}
	return &localui.SelfUpdate{Updater: u, Relaunch: func() {
		if program := u.Installed(); program != "" {
			reopen(program)
		}
	}}
}

// A remote update runs outside the desktop session. The open window polls
// locally so it can replace itself even when hidden in the tray.
func watchWindowUpdate(ctx context.Context, root string, updater *selfupdate.Updater, reopen func(string)) {
	ticks := time.NewTicker(5 * time.Second)
	defer ticks.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks.C:
			if program := updatedWindowProgram(ctx, root, updater); program != "" && ctx.Err() == nil {
				reopen(program)
				return
			}
		}
	}
}

func updatedWindowProgram(ctx context.Context, root string, updater *selfupdate.Updater) string {
	// Status reads the client's queue, so ask only when a newer signed
	// update is present locally. This check never contacts GitHub.
	manifest, err := updater.Channel.Load()
	if err != nil || manifest.Build <= updater.Running {
		return ""
	}
	call, cancel := context.WithTimeout(ctx, 2*time.Second)
	response, err := control.CallContext(call, root, control.Request{Cmd: "status"})
	cancel()
	if err != nil || !response.OK || response.Status == nil || response.Status.Version != manifest.Version {
		return ""
	}
	program := supervisor.ProgramPath(root)
	if program != "" && updater.MatchesInstalledUpdate(program, response.Status.Version) {
		return program
	}
	return ""
}

// newUpdater builds the updater of the program file executable from the
// signed channel in home: the window's and the remote update's (T-078).
func newUpdater(home, executable string) *selfupdate.Updater {
	channel := updatefeed.Channel{Directory: filepath.Join(home, "updates"), PublicKey: buildinfo.UpdatePublicKey}
	running := selfupdate.Build(buildinfo.Version)
	u := &selfupdate.Updater{
		Channel:    channel,
		Executable: executable,
		Running:    running,
		Sync: func(ctx context.Context) error {
			_, err := channel.SyncGitHub(ctx, running)
			return err
		},
		Find: func(ctx context.Context) (string, error) {
			r, err := channel.FindGitHub(ctx, running)
			if r.State != "found" {
				return "", err
			}
			return r.Version, err
		},
	}
	if prefix := distribution.InstallPrefix(executable); prefix != "" && runtime.GOOS != "windows" {
		u.Install = func(program string, info buildinfo.Info) (string, error) {
			return distribution.InstallUpdate(program, prefix, info)
		}
	}
	return u
}
