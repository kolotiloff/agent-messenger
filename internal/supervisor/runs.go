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
		if runs[i].PID == run.PID && runs[i].StartToken == run.StartToken {
			runs[i] = run
			replaced = true
		}
	}
	if !replaced {
		runs = append(runs, run)
	}
	return saveRuns(runs)
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

// reconcileRuns drops dead records and refuses a data directory other than the one
// that still owns a live CLI. The caller must already hold the user supervisor lock.
func reconcileRuns(root string) error {
	runs, err := loadRuns()
	if err != nil {
		return err
	}
	var live []recordedRun
	for _, run := range runs {
		owned, ownErr := platform.OwnsObservation(platform.Observation{
			PID: run.PID, PGID: run.PGID, StartToken: run.StartToken, BootID: run.BootID,
		})
		if ownErr != nil {
			if errors.Is(ownErr, platform.ErrUnsupported) {
				return ownErr
			}
			owned = false
		}
		if owned {
			live = append(live, run)
		}
	}
	if err := saveRuns(live); err != nil {
		return err
	}
	for _, run := range live {
		if filepath.Clean(run.Root) != filepath.Clean(root) {
			return fmt.Errorf("%w: %s pid %d", ErrLiveCLI, run.Root, run.PID)
		}
	}
	return nil
}
