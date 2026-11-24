package supervisor

import (
	"context"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/journal"
)

// Automatic updates of this computer (T-277, D-191). While the owner has
// them on, the client looks for a newer signed release at most every
// autoUpdateEvery and, for each newer release once, queues an update of
// this computer. The queue is the one a coordinator's or another computer's
// request uses: it waits until no task runs here, then the helper installs
// the update and restarts the client. A Mac is updated by its app.

// UpdateFinder returns the version of a newer signed release than the
// running program, or "" when there is none. It downloads nothing.
type UpdateFinder func(context.Context) (string, error)

// autoUpdateEvery matches the window's look: GitHub allows 60 checks an
// hour per address, shared by every computer behind it.
var autoUpdateEvery = 30 * time.Minute

type autoUpdateState struct {
	mu      sync.Mutex
	looking bool
	looked  time.Time
	found   string
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
	if !on {
		return ""
	}
	a := &s.auto
	a.mu.Lock()
	found := a.found
	if !a.looking && (a.looked.IsZero() || time.Since(a.looked) >= autoUpdateEvery) {
		a.looking, a.looked = true, time.Now()
		s.bg.Add(1)
		go func() {
			defer s.bg.Done()
			ctx, cancel := context.WithTimeout(s.ctx, 5*time.Minute)
			version, err := s.findUpdate(ctx)
			cancel()
			a.mu.Lock()
			defer a.mu.Unlock()
			a.looking = false
			if err == nil {
				a.found = version
			}
		}()
	}
	a.mu.Unlock()
	if found == "" || ProgramPath(s.root) == "" || s.updateBusy() {
		return ""
	}
	queued, err := s.store.QueueAutoUpdate(found)
	if err != nil {
		return "автообновление не поставлено в очередь: " + err.Error()
	}
	if queued {
		journal.Printf("update: automatic update queued for %s", found)
	}
	return ""
}
