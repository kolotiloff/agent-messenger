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
	// PPID is filled only by ProcessTree; Snapshot leaves it zero.
	PPID int
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

// OwnsObservation reports whether the recorded pid, start token, boot id and process
// group still refer to the same live process. A recycled group number is not owned.
func OwnsObservation(stored Observation) (bool, error) {
	if stored.PID <= 0 || stored.PGID <= 1 || stored.StartToken == "" || stored.BootID == "" {
		return false, fmt.Errorf("%w: incomplete observation", ErrNotProven)
	}
	boot, live, err := Snapshot()
	if err != nil {
		return false, err
	}
	if boot != stored.BootID {
		return false, nil
	}
	for _, proc := range live {
		if proc.PID == stored.PID && proc.StartToken == stored.StartToken && proc.PGID == stored.PGID {
			return true, nil
		}
	}
	return false, nil
}

// Descendants returns the live processes that descend from root through
// parent links, plus every other live member of root's process group. It is
// taken just before a graceful stop, so their later absence can be checked. A
// process that had already detached and been reparented is not visible here.
func Descendants(root Observation) ([]Proc, error) {
	boot, live, err := ProcessTree()
	if err != nil {
		return nil, err
	}
	if boot != root.BootID {
		return nil, fmt.Errorf("%w: boot changed", ErrNotProven)
	}
	return descendantsOf(root, live), nil
}

func descendantsOf(root Observation, live []Proc) []Proc {
	children := map[int][]Proc{}
	for _, p := range live {
		children[p.PPID] = append(children[p.PPID], p)
	}
	seen := map[int]bool{root.PID: true}
	var out []Proc
	queue := []int{root.PID}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, child := range children[pid] {
			if !seen[child.PID] {
				seen[child.PID] = true
				out = append(out, child)
				queue = append(queue, child.PID)
			}
		}
	}
	for _, p := range live {
		if p.PGID == root.PGID && !seen[p.PID] {
			seen[p.PID] = true
			out = append(out, p)
		}
	}
	return out
}

// AnyAlive reports whether any recorded process (same pid and start token) or
// any member of the group is still alive. An error is never "none alive".
func AnyAlive(boot string, pgid int, recorded []Proc) (bool, error) {
	current, live, err := ProcessTree()
	if err != nil {
		return true, err
	}
	if current != boot {
		return false, nil
	}
	for _, p := range live {
		if pgid > 1 && p.PGID == pgid {
			return true, nil
		}
		for _, r := range recorded {
			if p.PID == r.PID && p.StartToken == r.StartToken {
				return true, nil
			}
		}
	}
	return false, nil
}
