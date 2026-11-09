package supervisor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// recoveryEpoch reads this boot's identity; tests replace it.
var recoveryEpoch = platform.CurrentRecoveryEpoch

// RecoverMissingRoots retires registry entries whose data directory no longer
// exists, for example a test directory in a temporary folder that the OS
// removed on reboot. Each one needs the usual proof: the entry recorded a
// recovery epoch at launch, and the same host has booted again since then, so
// its processes are gone. Entries with an existing directory, without an epoch,
// from another host or from this boot are kept. The client must be stopped.
// Nothing is retried and no model is started; the missing database cannot be
// read, so its history cannot be recovered by this command.
func RecoverMissingRoots(confirmation string) (string, error) {
	if confirmation != core.RecoveryConfirmation {
		return "", core.ErrBadConfirmation
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
	current, epochErr := recoveryEpoch()
	var kept []recordedRun
	var retired, notes []string
	for _, run := range runs {
		_, statErr := os.Lstat(run.Root)
		if !errors.Is(statErr, fs.ErrNotExist) {
			kept = append(kept, run)
			continue
		}
		if epochErr != nil {
			kept = append(kept, run)
			notes = append(notes, run.Root+": "+epochErr.Error())
			continue
		}
		if _, err := platform.ProvePreviousBoot(run.Epoch, current); err != nil {
			kept = append(kept, run)
			notes = append(notes, run.Root+": "+err.Error())
			continue
		}
		retired = append(retired, run.Root)
	}
	if len(retired) == 0 {
		if len(notes) == 0 {
			return "", fmt.Errorf("%w: no reservation points to a missing data directory", core.ErrRejected)
		}
		return "", fmt.Errorf("%w: %s", core.ErrRecoveryBlocked, strings.Join(notes, "; "))
	}
	if err := saveRuns(kept); err != nil {
		return "", err
	}
	message := fmt.Sprintf("снято резервов: %d (каталоги удалены, процессы прежней загрузки ОС завершены): %s; модель не запускалась", len(retired), strings.Join(retired, ", "))
	if len(notes) != 0 {
		message += "; оставлены без доказательства: " + strings.Join(notes, "; ")
	}
	return message, nil
}
