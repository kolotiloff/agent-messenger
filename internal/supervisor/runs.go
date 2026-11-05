package supervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// ErrLiveCLI is returned when another data directory still has a running CLI.
var ErrLiveCLI = errors.New("a CLI from another data directory is still running")

type recordedRun struct {
	ID             string `json:"id"`
	Root           string `json:"root"`
	PID            int    `json:"pid"`
	PGID           int    `json:"pgid"`
	StartToken     string `json:"start_token"`
	BootID         string `json:"boot_id"`
	Provider       string `json:"provider"`
	Environment    string `json:"environment"`
	ConversationID string `json:"conversation_id"`
}

func runsPath() (string, error) {
	lockPath, err := platform.SupervisorLockFile()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(lockPath), "active-runs.json"), nil
}

func loadRuns() ([]recordedRun, error) {
	path, err := runsPath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var runs []recordedRun
	if len(raw) == 0 {
		return nil, nil
	}
	if err := json.Unmarshal(raw, &runs); err != nil {
		return nil, err
	}
	return runs, nil
}

func saveRuns(runs []recordedRun) error {
	path, err := runsPath()
	if err != nil {
		return err
	}
	raw, err := json.Marshal(runs)
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o600)
}

func rememberRun(run recordedRun) error {
	runs, err := loadRuns()
	if err != nil {
		return err
	}
	replaced := false
	for i := range runs {
		if run.ID != "" && runs[i].ID == run.ID {
			runs[i] = run
			replaced = true
		}
	}
	if !replaced {
		runs = append(runs, run)
	}
	return saveRuns(runs)
}

func forgetRunID(id string) error {
	if id == "" {
		return nil
	}
	runs, err := loadRuns()
	if err != nil {
		return err
	}
	kept := make([]recordedRun, 0, len(runs))
	for _, run := range runs {
		if run.ID != id {
			kept = append(kept, run)
		}
	}
	return saveRuns(kept)
}

func forgetRun(pid int) error {
	runs, err := loadRuns()
	if err != nil {
		return err
	}
	kept := runs[:0]
	for _, run := range runs {
		if run.PID != pid {
			kept = append(kept, run)
		}
	}
	return saveRuns(kept)
}

func rootRun(root string, spec Spec, obs platform.Observation) recordedRun {
	return recordedRun{
		Root: root, PID: obs.PID, PGID: obs.PGID, StartToken: obs.StartToken, BootID: obs.BootID,
		Provider: spec.Provider, Environment: spec.Environment, ConversationID: spec.ConversationID,
	}
}

// runBlocks reports whether a reservation must keep other data directories from starting.
// A missing PID, an observation error, or a dead parent with a live process group all block.
// An empty process group is still not proof that a detached descendant has stopped.
func runBlocks(run recordedRun) (bool, error) {
	if run.PID <= 0 || run.PGID <= 1 || run.StartToken == "" || run.BootID == "" {
		return true, nil
	}
	owned, err := platform.OwnsObservation(platform.Observation{
		PID: run.PID, PGID: run.PGID, StartToken: run.StartToken, BootID: run.BootID,
	})
	if err != nil {
		return true, err
	}
	if owned {
		return true, nil
	}
	empty, err := platform.GroupEmpty(run.PGID)
	if err != nil || !empty {
		return true, nil
	}
	return true, nil
}

// reconcileRuns refuses a different data directory while any reservation remains.
// It does not delete a reservation when the original PID is gone or observation fails.
// The caller must already hold the user supervisor lock.
func reconcileRuns(root string) error {
	runs, err := loadRuns()
	if err != nil {
		return err
	}
	for _, run := range runs {
		blocks, blockErr := runBlocks(run)
		if blockErr != nil && !errors.Is(blockErr, platform.ErrUnsupported) {
			blocks = true
		}
		if errors.Is(blockErr, platform.ErrUnsupported) {
			return blockErr
		}
		if !blocks {
			continue
		}
		if filepath.Clean(run.Root) != filepath.Clean(root) {
			return fmt.Errorf("%w: %s pid %d", ErrLiveCLI, run.Root, run.PID)
		}
	}
	return nil
}
