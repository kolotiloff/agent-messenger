package supervisor

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// RecoverCodex retires the reservation of an uncertain Codex setup. The only
// accepted proof is a new boot of the same host since the app-server was
// launched, recorded in the reservation at launch. A missing PID, an empty
// process group or a timeout is never enough. The client must be stopped.
//
// The session is closed without retry. Attempts, replies and uncertain
// messages are kept; nothing is resent and no model is started. The thread is
// not resumed: an empty thread is not saved, and a resumed thread would need a
// new owner, which is not implemented.
func RecoverCodex(root, alias, confirmation string) (string, error) {
	if confirmation != core.StreamRecoveryConfirmation {
		return "", core.ErrBadConfirmation
	}
	if alias == "" {
		return "", fmt.Errorf("%w: exact alias required", core.ErrRejected)
	}
	canonical, err := core.CanonicalEnvironment(root)
	if err != nil {
		return "", err
	}
	dbPath := filepath.Join(canonical, dbName)
	if _, err := os.Stat(dbPath); err != nil {
		return "", err
	}
	lockPath, err := platform.SupervisorLockFile()
	if err != nil {
		return "", err
	}
	lock, err := platform.Acquire(lockPath)
	if err != nil {
		return "", err
	}
	defer lock.Release()
	if err := checkStartupLease(); err != nil {
		return "", err
	}
	runs, err := loadRuns()
	if err != nil {
		return "", err
	}
	store, err := core.Open(dbPath)
	if err != nil {
		return "", err
	}
	defer store.Close()
	setup, err := store.CodexSetupForRecovery(alias)
	if err != nil {
		return "", err
	}
	var chosen recordedRun
	index := -1
	for i, r := range runs {
		if r.CodexSetupID == setup.ID {
			if index != -1 {
				return "", fmt.Errorf("%w: multiple reservations for the setup", core.ErrRecoveryBlocked)
			}
			chosen, index = r, i
		}
	}
	if index == -1 {
		// Only a pending recovery may find its reservation already removed.
		if setup.State != "retiring" || json.Unmarshal([]byte(setup.RecoveryReservation), &chosen) != nil {
			return "", fmt.Errorf("%w: matching reservation is missing", core.ErrRecoveryBlocked)
		}
	}
	reservedRoot, err := core.CanonicalEnvironment(chosen.Root)
	obs := setup.Observation
	if err != nil || reservedRoot != canonical || chosen.ID == "" || chosen.CodexSetupID != setup.ID || chosen.Provider != "codex" || chosen.Environment != setup.Environment || chosen.ConversationID != setup.ThreadID || chosen.PID != obs.PID || chosen.PGID != obs.PGID || chosen.StartToken != obs.StartToken || chosen.BootID != obs.BootID {
		return "", fmt.Errorf("%w: registry and database do not match", core.ErrRecoveryBlocked)
	}
	raw, err := json.Marshal(chosen)
	if err != nil {
		return "", err
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(raw))
	if setup.State == "retiring" && (setup.RecoveryFingerprint != fingerprint || setup.RecoveryReservation != string(raw)) {
		return "", fmt.Errorf("%w: pending recovery reservation changed", core.ErrRecoveryBlocked)
	}
	current, err := recoveryEpoch()
	if err != nil {
		return "", fmt.Errorf("%w: %v", core.ErrRecoveryBlocked, err)
	}
	proof, err := platform.ProvePreviousBoot(chosen.Epoch, current)
	if err != nil {
		return "", fmt.Errorf("%w: %v", core.ErrRecoveryBlocked, err)
	}
	if err := store.PrepareCodexRecovery(setup.ID, fingerprint, string(raw), proof.Detail); err != nil {
		return "", err
	}
	if index >= 0 {
		kept := append(runs[:index:index], runs[index+1:]...)
		if err := saveRuns(kept); err != nil {
			return "", fmt.Errorf("recovery pending; registry replacement failed: %w", err)
		}
	}
	if err := store.FinishCodexRecovery(setup.ID, fingerprint); err != nil {
		return "", fmt.Errorf("recovery pending; rerun the same command: %w", err)
	}
	return "резерв сессии Codex снят после доказанной перезагрузки; сессия закрыта без повтора; история сохранена; модель не запускалась", nil
}
