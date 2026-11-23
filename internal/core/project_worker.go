package core

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"modernc.org/sqlite"
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

	sqliteDriver := new(sqlite.Driver)
	sqliteDriver.RegisterConnectionHook(func(conn sqlite.ExecQuerierContext, _ string) error {
		// Task frames need the computer ID. Expose just that immutable
		// value on each connection. The worker cannot write, attach or
		// recreate the computer's persistent settings through this store.
		if _, err := conn.ExecContext(context.Background(), `CREATE TEMP TABLE meta(key TEXT PRIMARY KEY, value TEXT NOT NULL)`, nil); err != nil {
			return err
		}
		_, err := conn.ExecContext(context.Background(), `INSERT INTO temp.meta(key,value) VALUES ('computer_id',?)`,
			[]driver.NamedValue{{Ordinal: 1, Value: identity}})
		return err
	})
	source := workerProjectDSN(filepath.Join(layout, file))
	db := sql.OpenDB(&projectWorkerConnector{source: source, driver: sqliteDriver})
	db.SetMaxOpenConns(1)

	var task, owner, storedProject string
	err = db.QueryRowContext(ctx, `SELECT t.id,t.computer,t.project
 FROM task_runs r JOIN tasks t ON t.id=r.task_id
 WHERE r.id=? AND t.project=(SELECT project FROM project_snapshot)`, run).Scan(&task, &owner, &storedProject)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (storedProject != project || owner != identity)) {
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

func workerProjectDSN(path string) string {
	return sqliteFileURL(path, url.Values{
		"mode":    {"rw"},
		"_pragma": {sqliteBusyTimeout, "foreign_keys(1)", "synchronous(FULL)"},
		"_txlock": {"immediate"},
	})
}

type projectWorkerConnector struct {
	source string
	driver *sqlite.Driver
}

func (p *projectWorkerConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p.driver.Open(p.source)
}

func (p *projectWorkerConnector) Driver() driver.Driver {
	return p.driver
}
