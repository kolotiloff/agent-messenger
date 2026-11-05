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

var recoveryEpoch = platform.CurrentRecoveryEpoch

// RecoverStream never runs a provider or retries a message. The first supported
// proof is a new boot of the same host, recorded prospectively at launch.
func RecoverStream(root, alias, runID, confirmation string) (string, error) {
	if confirmation != core.StreamRecoveryConfirmation {
		return "", core.ErrBadConfirmation
	}
	if alias == "" || runID == "" {
		return "", fmt.Errorf("%w: exact alias and run id required", core.ErrRejected)
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
	target, err := store.StreamRecoveryTarget(alias, runID)
	if err != nil {
		return "", err
	}
	if target.Phase == "done" {
		return "восстановление этого запуска уже завершено; новых писем не создано", nil
	}
	var chosen recordedRun
	index := -1
	for i, r := range runs {
		if r.Provider == target.Provider && r.Environment == target.Environment && r.ConversationID == target.ConversationID {
			if index != -1 {
				return "", fmt.Errorf("%w: multiple reservations for the session", core.ErrRecoveryBlocked)
			}
			chosen = r
			index = i
		}
	}
	if index == -1 {
		if target.Phase != "pending" || json.Unmarshal([]byte(target.Reservation), &chosen) != nil {
			return "", fmt.Errorf("%w: matching reservation is missing", core.ErrRecoveryBlocked)
		}
	}
	for i, r := range runs {
		if i != index && (r.ID == chosen.ID || r.StreamRunID == runID) {
			return "", fmt.Errorf("%w: conflicting reservation identity", core.ErrRecoveryBlocked)
		}
	}
	reservedRoot, err := core.CanonicalEnvironment(chosen.Root)
	obs := target.Observation
	if err != nil || reservedRoot != canonical || chosen.ID == "" || chosen.StreamRunID != runID || chosen.Provider != target.Provider || chosen.Environment != target.Environment || chosen.ConversationID != target.ConversationID || chosen.PID != obs.PID || chosen.PGID != obs.PGID || chosen.StartToken != obs.StartToken || chosen.BootID != obs.BootID {
		return "", fmt.Errorf("%w: registry and database observation do not match", core.ErrRecoveryBlocked)
	}
	raw, err := json.Marshal(chosen)
	if err != nil {
		return "", err
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256(raw))
	if target.Phase == "pending" && (target.Fingerprint != fingerprint || target.Reservation != string(raw)) {
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
	if err := store.PrepareStreamRecovery(alias, runID, fingerprint, string(raw), proof); err != nil {
		return "", err
	}
	if index >= 0 {
		kept := append(runs[:index:index], runs[index+1:]...)
		if err := saveRuns(kept); err != nil {
			return "", fmt.Errorf("recovery pending; registry replacement failed: %w", err)
		}
	}
	if err := store.FinishStreamRecovery(alias, runID, fingerprint); err != nil {
		return "", fmt.Errorf("recovery pending; rerun the same command: %w", err)
	}
	return "владелец прежнего запуска освобождён после доказанной смены загрузки ОС; история сохранена, неопределённые письма не повторены; очередь на паузе; следующий запуск использует точный resume; модель не запускалась", nil
}
