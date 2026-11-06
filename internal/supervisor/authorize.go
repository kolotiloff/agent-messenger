package supervisor

import "github.com/kolotiloff/agent-messenger/internal/core"

func (s *Supervisor) currentLauncher() Launcher {
	s.launcherMu.RLock()
	defer s.launcherMu.RUnlock()
	return s.launcher
}

// AuthorizeSetup grants this process permission for one newly created test
// session. It cannot replace a grant, attach a working chat, or survive restart.
func (s *Supervisor) AuthorizeSetup(alias string) error {
	s.dispatch.Lock()
	defer s.dispatch.Unlock()
	if s.stopping.Load() {
		return core.ErrRejected
	}
	launcher, ok := s.currentLauncher().(AdapterLauncher)
	if !ok || !launcher.ClaudeStream {
		return core.ErrRejected
	}
	if launcher.AllowModels {
		if launcher.OnlySession == alias && alias != "" {
			return nil
		}
		return core.ErrRejected
	}
	busy := len(s.jobList()) != 0
	if busy || s.starting.Load() {
		return core.ErrBusy
	}
	if err := s.store.AuthorizeSetup(alias); err != nil {
		return err
	}
	launcher.AllowModels, launcher.OnlySession = true, alias
	s.launcherMu.Lock()
	s.launcher = launcher
	s.launcherMu.Unlock()
	return nil
}
