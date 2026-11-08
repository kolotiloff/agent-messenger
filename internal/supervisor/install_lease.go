package supervisor

import (
	"bytes"
	"errors"
	"github.com/kolotiloff/agent-messenger/internal/platform"
	"os"
)

// WithInstallationLease refuses activation while any controller, pending startup
// or retained CLI reservation exists. It never reconciles or removes reservations.
func WithInstallationLease(install func() error) error {
	path, err := platform.SupervisorLockFile()
	if err != nil {
		return err
	}
	lock, err := platform.Acquire(path)
	if err != nil {
		return err
	}
	defer lock.Release()
	startup, err := acquireStartupLease()
	if err != nil {
		return err
	}
	defer startup.Release()
	registry, err := runsPath()
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(registry)
	if err == nil {
		trimmed := bytes.TrimSpace(raw)
		if len(trimmed) == 0 || trimmed[0] != '[' {
			return ErrRunsCorrupt
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	runs, err := loadRuns()
	if err != nil {
		return err
	}
	if len(runs) != 0 {
		return errors.New("installation blocked: retained CLI reservations require their documented recovery; no reservation was changed")
	}
	return install()
}

// ReservedRuns counts the CLI reservations in active-runs.json without
// changing anything; the Windows uninstaller refuses while there are any.
func ReservedRuns() (int, error) {
	runs, err := loadRuns()
	return len(runs), err
}
