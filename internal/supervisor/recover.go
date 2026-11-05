package supervisor

import (
	"fmt"
	"path/filepath"

	"github.com/kolotiloff/agent-messenger/internal/core"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// RecoverFailed retires a failed attempt after an explicit confirmation.
// It does not start a CLI. A live process, a live descendant, an incomplete
// observation, or a valid provider result leaves the reservation unchanged.
func RecoverFailed(confirm string) (string, error) {
	if confirm != core.RetryConfirmation {
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
	if len(runs) == 0 {
		return "", fmt.Errorf("%w: no reservation is stored", core.ErrRejected)
	}
	for _, run := range runs {
		if err := reservationReadyToRetire(run); err != nil {
			return "", err
		}
		if err := inspectFailedAttempt(run); err != nil {
			return "", err
		}
	}
	var created []string
	for _, run := range runs {
		id, err := retireFailedAttempt(run)
		if err != nil {
			return "", err
		}
		created = append(created, id)
	}
	if err := saveRuns([]recordedRun{}); err != nil {
		return "", err
	}
	return fmt.Sprintf("восстановление записано; каталог %s; новое письмо %s; действия могут повториться; отделившиеся потомки вне группы не доказаны; модель не запускалась", joinRoots(runs), joinIDs(created)), nil
}

func inspectFailedAttempt(run recordedRun) error {
	return withRunStore(run, func(store *core.Store) error {
		return store.FailedAttemptRecoverable(run.Provider, run.Environment, run.ConversationID)
	})
}

func retireFailedAttempt(run recordedRun) (string, error) {
	var id string
	err := withRunStore(run, func(store *core.Store) error {
		var err error
		id, err = store.RecoverFailedAttempt(run.Provider, run.Environment, run.ConversationID)
		return err
	})
	return id, err
}

func withRunStore(run recordedRun, fn func(*core.Store) error) error {
	if run.Root == "" {
		return fmt.Errorf("%w: reservation has no data directory", core.ErrRecoveryBlocked)
	}
	store, err := core.Open(filepath.Join(run.Root, dbName))
	if err != nil {
		return err
	}
	defer store.Close()
	return fn(store)
}

func joinRoots(runs []recordedRun) string {
	seen := map[string]bool{}
	var roots []string
	for _, run := range runs {
		if run.Root == "" || seen[run.Root] {
			continue
		}
		seen[run.Root] = true
		roots = append(roots, run.Root)
	}
	return joinIDs(roots)
}

func joinIDs(ids []string) string {
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	return out
}
