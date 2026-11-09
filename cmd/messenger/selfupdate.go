package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/distribution"
	"github.com/kolotiloff/agent-messenger/internal/localui"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
	"github.com/kolotiloff/agent-messenger/internal/updatefeed"
)

// selfUpdateEvery is how often the window looks for a new signed release.
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

// installPrefix returns the prefix of an installation made by
// `messenger install` (<prefix>/lib/agent-messenger/<version>/messenger),
// or "" for a plain program file.
func installPrefix(executable string) string {
	version := filepath.Dir(executable)
	library := filepath.Dir(version)
	lib := filepath.Dir(library)
	if filepath.Base(executable) != "messenger" || filepath.Base(library) != "agent-messenger" || filepath.Base(lib) != "lib" {
		return ""
	}
	return filepath.Dir(lib)
}

// windowSelfUpdate prepares the window's update on Windows and Linux (D-048)
// and starts the background check. The running background client is not
// touched: it keeps its version until the window offers to restart it.
func windowSelfUpdate(ctx context.Context, home string, stop context.CancelFunc, next *relaunch) *localui.SelfUpdate {
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
	if u.Status().State != "unsupported" {
		go func() {
			for {
				check, cancel := context.WithTimeout(ctx, 5*time.Minute)
				u.Check(check)
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
			next.set(program)
			stop()
		}
	}}
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
	}
	if prefix := installPrefix(executable); prefix != "" && runtime.GOOS != "windows" {
		u.Install = func(program string, info buildinfo.Info) (string, error) {
			return distribution.InstallUpdate(program, prefix, info)
		}
	}
	return u
}
