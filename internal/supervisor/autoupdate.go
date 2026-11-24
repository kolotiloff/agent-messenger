package supervisor

import (
	"context"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/journal"
	"github.com/kolotiloff/agent-messenger/internal/selfupdate"
)

// Automatic updates of this computer (T-277, D-191). While the owner has
// them on, the client looks for a newer signed release at most every
// autoUpdateEvery and, for each newer release once, queues an update of
// this computer. The queue is the one a coordinator's or another computer's
// request uses: it waits until no task runs here, then the helper installs
// the update and restarts the client. A Mac is updated by its app.

// UpdateFinder returns the newest verified release, including the running
// version when it is current. An empty version proves nothing.
type UpdateFinder func(context.Context) (string, error)

// autoUpdateEvery matches the window's look: GitHub allows 60 checks an
// hour per address, shared by every computer behind it.
var autoUpdateEvery = 30 * time.Minute

type autoUpdateState struct {
	mu         sync.Mutex
	looking    bool
	looked     time.Time
	found      string
	lookedFor  string
	checkedFor string
}

// autoUpdateSupported reports whether this computer can update itself.
func (s *Supervisor) autoUpdateSupported() bool {
	return s.findUpdate != nil && s.updateHardRefusal() == ""
}

// autoUpdate looks for a newer release when one is due and queues an
// update for a release found. It never waits for the look itself.
func (s *Supervisor) autoUpdate() string {
	if !s.autoUpdateSupported() {
		return ""
	}
	on, err := s.store.AutoUpdate()
	if err != nil {
		return "настройка автообновления не читается: " + err.Error()
	}
	pending, hasPending, err := s.store.PendingUpdate()
	if err != nil {
		return "очередь обновления не читается: " + err.Error()
	}
	if (!on && !hasPending) || s.updateBusy() || (hasPending && pending.Target != "") {
		return ""
	}
	a := &s.auto
	a.mu.Lock()
	found, checkedFor := a.found, a.checkedFor
	newRequest := hasPending && a.lookedFor != pending.ID
	if !a.looking && (newRequest || a.looked.IsZero() || time.Since(a.looked) >= autoUpdateEvery) {
		a.looking, a.looked = true, time.Now()
		a.lookedFor = pending.ID
		// Bind the answer to the request present when the check started.
		// A result received later must not finish a new request.
		s.bg.Add(1)
		go func() {
			defer s.bg.Done()
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
			version, err := s.findUpdate(ctx)
			cancel()
			a.mu.Lock()
			defer a.mu.Unlock()
			a.looking = false
			a.found, a.checkedFor = "", ""
			if err == nil {
				a.found, a.checkedFor = version, pending.ID
			}
		}()
	}
	a.mu.Unlock()
	build, running := selfupdate.Build(found), selfupdate.Build(buildinfo.Version)
	if hasPending {
		if checkedFor == pending.ID && build > 0 && build <= running {
			if err := s.store.FinishPendingUpdate(pending.ID, core.UpdateDone, buildinfo.Version, "уже последняя версия"); err != nil {
				return "результат обновления не записан: " + err.Error()
			}
		}
		return ""
	}
	if !on || build <= running || ProgramPath(s.root) == "" {
		return ""
	}
	queued, err := s.store.QueueAutoUpdate(found)
	if err != nil {
		return "автообновление не поставлено в очередь: " + err.Error()
	}
	if queued {
		if s.server != nil {
			s.server.NotifyPresence()
		}
		journal.Printf("update: automatic update queued for %s", found)
	}
	return ""
}
