package supervisor

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// ErrLiveCLI is returned when another data directory still has a running CLI.
var ErrLiveCLI = errors.New("a CLI from another data directory is still running")

// ErrRunsCorrupt means active-runs.json exists but is empty or unreadable.
// That is not the same as "no reservations".
var ErrRunsCorrupt = errors.New("active-runs.json is empty or damaged")

// beforeRenameHook, when set by tests, runs after the temp file is synced and before rename.
var beforeRenameHook func() error

// ownsObservation and groupEmpty are the process checks used before a reservation
// may be retired. Tests replace them to simulate an observation failure.
var ownsObservation = platform.OwnsObservation
var groupEmpty = platform.GroupEmpty

type recordedRun struct {
	StreamRunID    string                 `json:"stream_run_id,omitempty"`
	CodexSetupID   string                 `json:"codex_setup_id,omitempty"`
	TaskRunID      string                 `json:"task_run_id,omitempty"`
	Epoch          platform.RecoveryEpoch `json:"recovery_epoch,omitempty"`
	ID             string                 `json:"id"`
	Root           string                 `json:"root"`
	PID            int                    `json:"pid"`
	PGID           int                    `json:"pgid"`
	StartToken     string                 `json:"start_token"`
	BootID         string                 `json:"boot_id"`
	Provider       string                 `json:"provider"`
	Environment    string                 `json:"environment"`
	ConversationID string                 `json:"conversation_id"`
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
	if len(bytesTrimSpace(raw)) == 0 {
		return nil, ErrRunsCorrupt
	}
	var runs []recordedRun
	if err := json.Unmarshal(raw, &runs); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRunsCorrupt, err)
	}
	// JSON null decodes successfully into a nil slice, but the persisted
	// registry must be an array. Treat it as damaged, not as "no owners".
	if runs == nil {
		return nil, ErrRunsCorrupt
	}
	for _, run := range runs {
		// A relative path can name a different database after the working directory
		// changes. Missing roots also clean to "." and must not pass a same-root check.
		if !filepath.IsAbs(run.Root) {
			return nil, ErrRunsCorrupt
		}
	}
	return runs, nil
}

func bytesTrimSpace(raw []byte) []byte {
	start, end := 0, len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\n' || raw[start] == '\t' || raw[start] == '\r') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\n' || raw[end-1] == '\t' || raw[end-1] == '\r') {
		end--
	}
	return raw[start:end]
}

func saveRuns(runs []recordedRun) error {
	path, err := runsPath()
	if err != nil {
		return err
	}
	if runs == nil {
		runs = []recordedRun{}
	}
	raw, err := json.Marshal(runs)
	if err != nil {
		return err
	}
	return writeAtomic(path, raw)
}

// writeAtomic puts the new contents on disk before replacing active-runs.json.
// A crash while writing the temporary file leaves the previous file intact.
func writeAtomic(path string, raw []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".active-runs-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if beforeRenameHook != nil {
		if err := beforeRenameHook(); err != nil {
			return err
		}
	}
	if err := os.Rename(tmpName, path); err != nil {
		return err
	}
	cleanup = false
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer dirFile.Close()
	return dirFile.Sync()
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

// reservationReadyToRetire reports whether this reservation may be retired.
// A missing PID, a live process, a live group, or an observation error refuses.
func reservationReadyToRetire(run recordedRun) error {
	if run.PID <= 0 || run.PGID <= 1 || run.StartToken == "" || run.BootID == "" {
		return fmt.Errorf("%w: observation is incomplete", core.ErrRecoveryBlocked)
	}
	owned, err := ownsObservation(platform.Observation{
		PID: run.PID, PGID: run.PGID, StartToken: run.StartToken, BootID: run.BootID,
	})
	if err != nil {
		return fmt.Errorf("%w: %v", core.ErrRecoveryBlocked, err)
	}
	if owned {
		return fmt.Errorf("%w: recorded process is still running", core.ErrRecoveryBlocked)
	}
	empty, err := groupEmpty(run.PGID)
	if err != nil {
		return fmt.Errorf("%w: %v", core.ErrRecoveryBlocked, err)
	}
	if !empty {
		return fmt.Errorf("%w: a descendant is still in the process group", core.ErrRecoveryBlocked)
	}
	return nil
}

// reconcileRuns refuses a different data directory while any reservation remains.
// It does not delete a reservation when the original PID is gone or observation fails.
// The caller must already hold the user supervisor lock.
func reconcileRuns(root string) error {
	runs, err := loadRuns()
	if err != nil {
		if errors.Is(err, ErrRunsCorrupt) {
			return fmt.Errorf("%w: %v", ErrLiveCLI, err)
		}
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
		reservedRoot, rootErr := core.CanonicalEnvironment(run.Root)
		currentRoot, currentErr := core.CanonicalEnvironment(root)
		if rootErr != nil || currentErr != nil || reservedRoot != currentRoot {
			return fmt.Errorf("%w: %s pid %d", ErrLiveCLI, run.Root, run.PID)
		}
	}
	return nil
}
