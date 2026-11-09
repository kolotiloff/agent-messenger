package supervisor

import (
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// A cancelled startup can still return from an OS call later. Its lease keeps
// every supervisor/recovery entry point away from the registry until that call
// has finished. The controller can stop without waiting for it.
func acquireStartupLease() (*platform.Lock, error) {
	path, err := platform.SupervisorLockFile()
	if err != nil {
		return nil, err
	}
	return platform.Acquire(path + ".startup")
}

// Called while holding the main supervisor lock, before reading the registry.
func checkStartupLease() error {
	lease, err := acquireStartupLease()
	if err != nil {
		return err
	}
	return lease.Release()
}
