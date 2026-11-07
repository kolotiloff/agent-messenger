package supervisor

import "github.com/kolotiloff/agent-messenger/internal/core"

func (s *Supervisor) currentLauncher() Launcher {
	s.launcherMu.RLock()
	defer s.launcherMu.RUnlock()
	return s.launcher
}

// AuthorizeSetup grants this process permission for a newly created test
// session. Several sessions can be granted in one run (up to maxWindowSessions),
// so a Claude and a Codex test session can talk. A grant cannot attach a
// working chat or survive a restart.
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
	if launcher.AllowModels && launcher.OnlySession == "" {
		return core.ErrRejected
	}
	if launcher.Allows(alias) {
		return nil
	}
	// Check before granting: the loop claims work every 30 ms, so a grant
	// that is withdrawn afterwards could start a refused session.
	if err := s.store.CheckAuthorizeSetup(alias); err != nil {
		return err
	}
	granted, err := s.grantWindowAliasLocked(launcher, alias)
	if err != nil {
		return err
	}
	if err := s.store.AuthorizeSetup(alias); err != nil {
		if granted {
			s.revokeWindowAlias(alias)
		}
		return err
	}
	return nil
}
