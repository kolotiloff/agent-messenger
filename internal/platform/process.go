package platform

import "fmt"

// Proof is a positive demonstration that the recorded execution's processes are gone.
// An empty process group is not such a demonstration: a descendant may have left the group.
type Proof struct {
	Proven bool
	Detail string
}

// Proc is one live process visible to the platform snapshot.
type Proc struct {
	PID        int
	PGID       int
	StartToken string
}

// Observation is what the supervisor recorded at launch.
type Observation struct {
	PID        int
	PGID       int
	StartToken string
	BootID     string
}

// EvaluateStop decides whether a later snapshot proves the old execution stopped.
// A matching live pid or process group is still running.
// A changed boot id means the previous boot's processes are gone.
// An empty group does not prove that detached descendants stopped.
func EvaluateStop(stored Observation, live []Proc, bootID string, snapshotErr error) (Proof, error) {
	if snapshotErr != nil {
		return Proof{}, fmt.Errorf("%w: %v", ErrNotProven, snapshotErr)
	}
	if stored.BootID != "" && bootID != "" && stored.BootID != bootID {
		return Proof{Proven: true, Detail: "boot id changed; processes from the previous boot are gone"}, nil
	}
	if stored.PID <= 0 || stored.PGID <= 0 || stored.StartToken == "" || stored.BootID == "" {
		return Proof{}, fmt.Errorf("%w: launch observation is incomplete", ErrNotProven)
	}
	for _, proc := range live {
		if proc.PGID == stored.PGID || (proc.PID == stored.PID && proc.StartToken == stored.StartToken) {
			return Proof{}, fmt.Errorf("%w: recorded process or its group is still present", ErrStillRunning)
		}
	}
	return Proof{}, fmt.Errorf("%w: process group is absent; detached descendants are not disproven", ErrNotProven)
}

// CurrentProof reads the OS snapshot. It does not treat a lone PID or an empty group as success.
func CurrentProof(stored Observation) (Proof, error) {
	boot, live, err := Snapshot()
	return EvaluateStop(stored, live, boot, err)
}
