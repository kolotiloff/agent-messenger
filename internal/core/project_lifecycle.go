package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ensureSplitProject publishes a new database before admitting its catalog
// route. An interrupted catalog commit can reuse the unmodified file on the
// next explicit creation. Removed projects keep their existing file.
func (s *Store) ensureSplitProject(tx *sql.Tx, name, folder, identity string) error {
	ctx := context.Background()
	file := projectMigrationFile(name)
	path := filepath.Join(s.split.root, file)
	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		var registered bool
		if err := tx.QueryRow("SELECT EXISTS(SELECT 1 FROM project_storage_stage WHERE project=?)", name).Scan(&registered); err != nil {
			return err
		}
		if registered {
			return fmt.Errorf("%w: the existing project file is missing", ErrRejected)
		}
		if err := publishEmptyProject(ctx, s.split.root, path, name, folder, identity); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	db, err := s.split.open(file)
	if err != nil {
		return err
	}
	var saved, owner string
	if err := db.QueryRowContext(ctx, `SELECT project,(SELECT value FROM meta WHERE key='computer_id')
 FROM project_snapshot`).Scan(&saved, &owner); err != nil {
		return err
	}
	if saved != name || owner != identity {
		return fmt.Errorf("%w: an existing project file belongs to another project or computer", ErrRejected)
	}
	_, err = tx.Exec("INSERT INTO project_storage_stage(project,file) VALUES (?,?) ON CONFLICT(project) DO NOTHING", name, file)
	return err
}

func publishEmptyProject(ctx context.Context, root, dest, name, folder, identity string) error {
	scratch, err := os.MkdirTemp(root, ".messenger-new-project-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	legacyPath := filepath.Join(scratch, "empty.sqlite")
	empty, err := Open(legacyPath)
	if err != nil {
		return err
	}
	defer empty.Close()
	if err := empty.tx(func(tx *sql.Tx) error { return setMeta(tx, "computer_id", identity) }); err != nil {
		return err
	}
	if err := empty.CreateProject(name, folder); err != nil {
		return err
	}
	if err := empty.Close(); err != nil {
		return err
	}
	project := filepath.Join(scratch, "project.sqlite")
	if err := snapshotProjectDatabase(ctx, legacyPath, name, project, true, ""); err != nil {
		return err
	}
	if err := stageProjectRevisions(project); err != nil {
		return err
	}
	return os.Link(project, dest)
}
