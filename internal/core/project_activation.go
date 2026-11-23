package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

const (
	projectGenerationKey    = "project_storage_generation"
	projectGenerationPrefix = "project-storage-"
)

var ErrProjectStorageBusy = fmt.Errorf("%w: the legacy queue is still in use", ErrRejected)

// UsesProjectStorage reports which layout this open connection is using.
func (s *Store) UsesProjectStorage() bool { return s.split != nil }

func readProjectGeneration(db projectRouteQuerier) (string, error) {
	ctx := context.Background()
	var hasMeta bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM sqlite_schema WHERE type='table' AND name='meta')`).Scan(&hasMeta); err != nil || !hasMeta {
		return "", err
	}
	var generation string
	err := db.QueryRowContext(ctx, "SELECT value FROM meta WHERE key=?", projectGenerationKey).Scan(&generation)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	suffix := strings.TrimPrefix(generation, projectGenerationPrefix)
	if suffix == generation || len(suffix) != 32 || strings.Trim(suffix, "0123456789abcdef") != "" {
		return "", fmt.Errorf("%w: invalid project storage generation", ErrRejected)
	}
	return generation, nil
}

// ActivateProjectStorage is called while the caller exclusively owns the
// supervisor lock and has checked the shared process registry. It leaves
// the legacy queue intact as a sealed source and keeps a separate backup.
// The committed selector and the fence against old writers are atomic.
func ActivateProjectStorage(ctx context.Context, legacy string) error {
	err := activateProjectStorage(ctx, legacy)
	var kept *legacyKept
	if errors.As(err, &kept) {
		return fmt.Errorf("%w: %w", ErrProjectStorageNotActivated, kept.err)
	}
	return err
}

// ErrProjectStorageNotActivated means activation stopped before it changed
// the single database, which stays complete and in use (D-188).
var ErrProjectStorageNotActivated = errors.New("project storage was not activated")

type legacyKept struct{ err error }

func (k *legacyKept) Error() string { return k.err.Error() }

func activateProjectStorage(ctx context.Context, legacy string) (err error) {
	// Until the selector is written, a failure leaves the single database
	// as it was. An already selected generation, or an uncertain commit,
	// is not marked: the client must not fall back to the old copy.
	selected := false
	defer func() {
		if err != nil && !selected && !errors.Is(err, ErrProjectStorageBusy) {
			err = &legacyKept{err}
		}
	}()
	root := filepath.Dir(legacy)
	db, err := sql.Open("sqlite", sqliteFileURL(legacy, url.Values{
		"mode": {"rw"}, "_pragma": {sqliteBusyTimeout, "synchronous(FULL)"}, "_txlock": {"immediate"},
	}))
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	generation, err := readProjectGeneration(tx)
	if err != nil {
		return err
	}
	if generation != "" {
		selected = true
		// Verify the installed generation can actually be opened. Its
		// staging checksums no longer describe files intentionally in use.
		current, err := openSplitStore(ctx, filepath.Join(root, generation))
		if err != nil {
			return err
		}
		return current.Close()
	}
	var busy bool
	if err := tx.QueryRowContext(ctx, `SELECT
 EXISTS(SELECT 1 FROM task_runs WHERE state IN ('launching','running'))
 OR EXISTS(SELECT 1 FROM sessions WHERE active_token IS NOT NULL)`).Scan(&busy); err != nil {
		return err
	}
	if busy {
		return ErrProjectStorageBusy
	}
	busy, err = legacyCoordinatorRunning(ctx, tx)
	if err != nil {
		return err
	}
	if busy {
		return ErrProjectStorageBusy
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	generation = projectGenerationPrefix + id
	path := filepath.Join(root, generation)
	// The legacy writer lock survives the entire backup. A reader sees
	// the last committed SQLite image, while old writers cannot race it.
	if err := StageProjectMigration(ctx, legacy, path); err != nil {
		return err
	}
	committing := false
	defer func() {
		if !committing {
			_ = os.RemoveAll(path)
		}
	}()
	if err := VerifyProjectMigration(ctx, path); err != nil {
		return err
	}
	if err := syncProjectGeneration(root, path); err != nil {
		return err
	}
	if err := setMeta(tx, projectGenerationKey, generation); err != nil {
		return err
	}
	// Version 3 cannot understand the selector. Guard its next startup,
	// and fence connections it might already have held open.
	if err := setMeta(tx, "schema_version", "4"); err != nil {
		return err
	}
	if err := fenceLegacyQueue(ctx, tx); err != nil {
		return err
	}
	// If Commit returns an uncertain result the generation stays on disk.
	committing = true
	selected = true
	return tx.Commit()
}

// A coordinator is not owned by the supervisor and may keep its database
// connection after the client restarts. Allow those chats to close normally.
func legacyCoordinatorRunning(ctx context.Context, tx *sql.Tx) (bool, error) {
	rows, err := tx.QueryContext(ctx, "SELECT pid,start_token,boot_id FROM coordinator_chats WHERE pid>0")
	if err != nil {
		return false, err
	}
	defer rows.Close()
	var owners []platform.Observation
	for rows.Next() {
		var owner platform.Observation
		if err := rows.Scan(&owner.PID, &owner.StartToken, &owner.BootID); err != nil {
			return false, err
		}
		owners = append(owners, owner)
	}
	if err := rows.Err(); err != nil || len(owners) == 0 {
		return false, err
	}
	boot, procs, err := platform.Snapshot()
	if err != nil {
		return true, nil
	}
	for _, owner := range owners {
		if owner.BootID != "" && owner.BootID != boot {
			continue
		}
		for _, proc := range procs {
			if proc.PID == owner.PID && (proc.StartToken == owner.StartToken || proc.StartToken == "" || owner.StartToken == "") {
				return true, nil
			}
		}
	}
	return false, nil
}

func syncProjectGeneration(root, generation string) error {
	files, err := os.ReadDir(generation)
	if err != nil {
		return err
	}
	for _, entry := range files {
		f, err := os.OpenFile(filepath.Join(generation, entry.Name()), os.O_RDWR, 0)
		if err != nil {
			return err
		}
		if err := errors.Join(f.Sync(), f.Close()); err != nil {
			return err
		}
	}
	if err := platform.SyncDir(generation); err != nil {
		return err
	}
	return platform.SyncDir(root)
}

func fenceLegacyQueue(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, `SELECT name FROM sqlite_schema WHERE type='table' AND name NOT GLOB 'sqlite_*'`)
	if err != nil {
		return err
	}
	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			rows.Close()
			return err
		}
		tables = append(tables, table)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for i, table := range tables {
		quoted := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
		for _, action := range []string{"INSERT", "UPDATE", "DELETE"} {
			query := fmt.Sprintf(`CREATE TRIGGER project_storage_seal_%d_%s BEFORE %s ON %s
 BEGIN SELECT RAISE(ABORT,'the legacy queue was sealed during project storage activation'); END`, i, action, action, quoted)
			if _, err := tx.ExecContext(ctx, query); err != nil {
				return err
			}
		}
	}
	return nil
}
