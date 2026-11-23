package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// openSplitWorkerStore opens the verified project file for a registered
// run. It does not grant a receipt or start a process. Runtime activation
// must still use the machine's shared reservation registry.
func openSplitWorkerStore(ctx context.Context, computer *sql.DB, layout, project, run string) (*Store, error) {
	var file, identity string
	err := computer.QueryRowContext(ctx, `SELECT file,COALESCE((SELECT value FROM meta WHERE key='computer_id'),'')
 FROM project_storage_stage WHERE project=?`, project).Scan(&file, &identity)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: unknown project storage", ErrRejected)
	}
	if err != nil {
		return nil, err
	}
	if file != projectMigrationFile(project) || validComputer(identity) == "" {
		return nil, fmt.Errorf("%w: invalid worker storage catalog", ErrRejected)
	}

	db, err := sql.Open("sqlite", workerProjectDSN(filepath.Join(layout, file)))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	var task, owner, storedProject, savedIdentity string
	err = db.QueryRowContext(ctx, `SELECT t.id,t.computer,t.project,COALESCE((SELECT value FROM meta WHERE key='computer_id'),'')
 FROM task_runs r JOIN tasks t ON t.id=r.task_id
 WHERE r.id=? AND t.project=(SELECT project FROM project_snapshot)`, run).Scan(&task, &owner, &storedProject, &savedIdentity)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (storedProject != project || owner != identity || savedIdentity != identity)) {
		db.Close()
		return nil, fmt.Errorf("%w: run does not belong to this computer and project", ErrRejected)
	}
	if err != nil {
		db.Close()
		return nil, err
	}
	var registered bool
	err = computer.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM task_storage_routes WHERE id=? AND project=?)`, task, project).Scan(&registered)
	if err != nil || !registered {
		db.Close()
		if err == nil {
			err = fmt.Errorf("%w: task route was not committed", ErrRejected)
		}
		return nil, err
	}
	return &Store{db: db}, nil
}

// OpenTaskRunStore opens the worker's configured database. A project file
// requires a matching computer catalog and run route before it can serve
// task MCP. The legacy filename and worker receipt protocol stay valid.
func OpenTaskRunStore(ctx context.Context, path, run string) (*Store, error) {
	// Split filenames use an exact digest. A named snapshot must still
	// pass Open's normal rejection unless it is a routed project file.
	file := filepath.Base(path)
	if !strings.HasPrefix(file, "project-") || len(file) != len(projectMigrationFile("")) || !strings.HasSuffix(file, ".sqlite") {
		return Open(path)
	}
	layout := filepath.Dir(path)
	computer, err := sql.Open("sqlite", sqliteFileURL(filepath.Join(layout, migrationComputerFile), url.Values{
		"mode": {"ro"}, "_pragma": {sqliteBusyTimeout},
	}))
	if err != nil {
		return nil, err
	}
	defer computer.Close()
	var project string
	err = computer.QueryRowContext(ctx, "SELECT project FROM project_storage_stage WHERE file=?", file).Scan(&project)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: worker database is missing from the computer catalog", ErrRejected)
	}
	if err != nil {
		return nil, err
	}
	return openSplitWorkerStore(ctx, computer, layout, project, run)
}

func workerProjectDSN(path string) string {
	return sqliteFileURL(path, url.Values{
		"mode":    {"rw"},
		"_pragma": {sqliteBusyTimeout, "foreign_keys(1)", "synchronous(FULL)"},
		"_txlock": {"immediate"},
	})
}
