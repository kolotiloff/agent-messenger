package supervisor

import (
	"context"
	"os"
	"sync"

	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/control"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// storageStartup responds to the same readiness handshake on all supported
// platforms, including when the previous version launches the new client.
// A resume from that launcher applies once the selected store is open.
type storageStartup struct {
	mu      sync.Mutex
	paused  bool
	ready   bool
	stopped bool
	cancel  context.CancelFunc
}

func (s *storageStartup) control(req control.Request) (control.Response, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ready {
		return control.Response{}, false
	}
	if s.stopped {
		return control.Response{Error: "Клиент останавливается"}, true
	}
	switch req.Cmd {
	case "status":
		return control.Response{OK: true, Status: &control.Status{
			Paused: true, Supervisor: os.Getpid(), Version: buildinfo.Version,
			ProjectStorage: "preparing", Detail: "Подготовка баз данных проектов",
		}}, true
	case "pause", "resume":
		s.paused = req.Cmd == "pause"
	case "shutdown":
		s.stopped = true
		s.cancel()
	default:
		return control.Response{Error: "Подготовка баз данных проектов"}, true
	}
	return control.Response{OK: true}, true
}

func (s *storageStartup) finish(ctx context.Context, store *core.Store) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	var err error
	if s.paused {
		err = store.Pause()
	} else {
		err = store.ResumeWork()
	}
	s.ready = err == nil
	return err
}
