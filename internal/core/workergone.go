package core

import (
	"errors"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// CheckWorkerGone confirms that a run's recorded worker is gone (D-108,
// D-120): the computer restarted since, or neither its process (same id and
// start time) nor any process of its group is running. Anything it cannot
// confirm is an error, and the run stays as it is.
func CheckWorkerGone(o platform.Observation) (string, error) {
	if o.PID <= 0 || o.PGID <= 0 || o.StartToken == "" || o.BootID == "" {
		return "", errors.New("its process was never recorded")
	}
	boot, live, err := platform.Snapshot()
	if err != nil {
		return "", err
	}
	if boot == "" {
		return "", errors.New("this computer's start could not be read")
	}
	if boot != o.BootID {
		return "the computer restarted since", nil
	}
	for _, p := range live {
		if p.PGID == o.PGID || (p.PID == o.PID && p.StartToken == o.StartToken) {
			return "", errors.New("its process or a process of its group is still running")
		}
	}
	return "its process and its process group are gone", nil
}

// SetWorkerCheck lets an explicit cancel close a blocked or interrupted
// task of this computer once check confirms its worker is gone (D-120).
// Without it such a cancel is refused, as before. Call it before the store
// is shared.
func (s *Store) SetWorkerCheck(check WorkerGone) { s.gone = check }
