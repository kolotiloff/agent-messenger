package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
)

type projectLayout struct {
	root  string
	mu    sync.Mutex
	files map[string]*sql.DB
	runs  map[string]string
}

// OpenVerifiedProjectStorage opens a freshly prepared generation after its
// integrity check. The caller must keep it isolated from the live client
// until the activation transaction selects this generation.
func OpenVerifiedProjectStorage(ctx context.Context, root string) (*Store, error) {
	if err := VerifyProjectMigration(ctx, root); err != nil {
		return nil, err
	}
	return openSplitStore(ctx, root)
}

// openSplitStore uses an already verified storage generation. The upgrade
// coordinator must select the active generation before ordinary startup
// calls this. A staged copy on disk does not select itself.
func openSplitStore(ctx context.Context, root string) (*Store, error) {
	db, err := sql.Open("sqlite", workerProjectDSN(filepath.Join(root, migrationComputerFile)))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(ctx, "SELECT seq FROM project_notices LIMIT 0"); err != nil {
		db.Close()
		return nil, fmt.Errorf("project storage lacks the current notice queue: %w", err)
	}
	if _, err := splitProjectFiles(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureUpdateCancellation(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := ensureColumn(db, "net_peers", "contact_age", "INTEGER"); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, split: &projectLayout{root: root, files: make(map[string]*sql.DB), runs: make(map[string]string)}}, nil
}

// open takes only catalog-validated filenames. It never creates a database.
func (p *projectLayout) open(file string) (*sql.DB, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.files == nil {
		return nil, sql.ErrConnDone
	}
	if db := p.files[file]; db != nil {
		return db, nil
	}
	db, err := sql.Open("sqlite", workerProjectDSN(filepath.Join(p.root, file)))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := ensureRemoteLiveUsage(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("upgrade project task usage: %w", err)
	}
	if _, err := db.Exec(taskAnalyticsSchema + taskAttachmentsSchema + messageAttachmentsSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("initialize project statistics: %w", err)
	}
	// A project database made before T-272 gets the state indexes once.
	// They only speed up the work count, so a busy database is not a
	// reason to refuse it; the next start tries again.
	_, _ = db.Exec(workStateIndexes + taskLaunchEventsIndex)
	p.files[file] = db
	return db, nil
}

func (p *projectLayout) close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	var err error
	for _, db := range p.files {
		err = errors.Join(err, db.Close())
	}
	p.files = nil
	p.runs = nil
	return err
}

func (p *projectLayout) forgetRun(run string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.runs, run)
}

type projectRouteQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func taskProjectFile(ctx context.Context, computer projectRouteQuerier, task string) (string, string, error) {
	var project, file string
	err := computer.QueryRowContext(ctx, `SELECT r.project,p.file
 FROM task_storage_routes r JOIN project_storage_stage p ON p.project=r.project WHERE r.id=?`, task).Scan(&project, &file)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", fmt.Errorf("%w: unknown task", ErrRejected)
	}
	if err == nil && file != projectMigrationFile(project) {
		err = fmt.Errorf("%w: invalid project database route", ErrRejected)
	}
	return project, file, err
}

func (s *Store) projectStore(project string) (*Store, error) {
	if s.split == nil {
		return s, nil
	}
	ctx := context.Background()
	var file string
	err := s.db.QueryRowContext(ctx, "SELECT file FROM project_storage_stage WHERE project=?", project).Scan(&file)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: unknown project", ErrRejected)
	}
	if err != nil {
		return nil, err
	}
	if file != projectMigrationFile(project) {
		return nil, fmt.Errorf("%w: invalid project database route", ErrRejected)
	}
	return s.splitProjectStore(project, file)
}

func (s *Store) splitProjectStore(project, file string) (*Store, error) {
	db, err := s.split.open(file)
	if err != nil {
		return nil, err
	}
	var saved string
	if err := db.QueryRow("SELECT project FROM project_snapshot").Scan(&saved); err != nil {
		return nil, err
	}
	if saved != project {
		return nil, fmt.Errorf("%w: the database belongs to another project", ErrRejected)
	}
	return &Store{db: db, gone: s.gone, machine: s.db}, nil
}

func (s *Store) storeForTask(task string) (*Store, error) {
	if s.split == nil {
		return s, nil
	}
	project, file, err := taskProjectFile(context.Background(), s.db, task)
	if err != nil {
		return nil, err
	}
	return s.splitProjectStore(project, file)
}

func (s *Store) eachProjectStore(read func(*Store) error) error {
	files, err := splitProjectFiles(context.Background(), s.db)
	if err != nil {
		return err
	}
	for _, file := range files {
		db, err := s.split.open(file)
		if err != nil {
			return err
		}
		if err := read(&Store{db: db, gone: s.gone, machine: s.db}); err != nil {
			return err
		}
	}
	return nil
}

// WorkerTaskDatabase is the file handed to the worker's MCP process. It
// keeps the data root (and therefore the process registry) unchanged.
func (s *Store) WorkerTaskDatabase(legacyPath, task string) (string, error) {
	if s.split == nil {
		return legacyPath, nil
	}
	_, file, err := taskProjectFile(context.Background(), s.db, task)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.split.root, file), nil
}

func (s *Store) storeForRun(run string) (*Store, error) {
	if s.split == nil {
		return s, nil
	}
	s.split.mu.Lock()
	file := s.split.runs[run]
	s.split.mu.Unlock()
	if file != "" {
		db, err := s.split.open(file)
		return &Store{db: db, gone: s.gone, machine: s.db}, err
	}
	ctx := context.Background()
	files, err := splitProjectFiles(ctx, s.db)
	if err != nil {
		return nil, err
	}
	for _, name := range files {
		db, err := s.split.open(name)
		if err != nil {
			return nil, err
		}
		var task string
		err = db.QueryRowContext(ctx, "SELECT task_id FROM task_runs WHERE id=?", run).Scan(&task)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		project, registered, err := taskProjectFile(ctx, s.db, task)
		if err != nil {
			return nil, err
		}
		if registered != name || file != "" {
			return nil, fmt.Errorf("%w: conflicting project route for run", ErrRejected)
		}
		if _, err := s.splitProjectStore(project, registered); err != nil {
			return nil, err
		}
		file = registered
	}
	if file == "" {
		return nil, fmt.Errorf("%w: unknown run", ErrRejected)
	}
	// Only launched runs are cached, so reading old runs cannot grow this
	// map. They can be found again from the route after a process restart.
	db, err := s.split.open(file)
	return &Store{db: db, gone: s.gone, machine: s.db}, err
}

// taskTx is for operations that alter one routed task. Machine policy is
// read before the project's write. The operation changes only the project
// transaction, so no successful task write depends on another commit.
func (s *Store) taskTx(task string, apply func(machine, project *sql.Tx) error) error {
	if s.split == nil {
		if s.machine == nil {
			return s.tx(func(tx *sql.Tx) error { return apply(tx, tx) })
		}
		machine, err := s.machine.BeginTx(context.Background(), nil)
		if err != nil {
			return err
		}
		defer machine.Rollback()
		return s.tx(func(tx *sql.Tx) error { return apply(machine, tx) })
	}
	ctx := context.Background()
	common, err := beginSplitAdmission(ctx, s.db)
	if err != nil {
		return err
	}
	defer common.Rollback()
	project, file, err := taskProjectFile(ctx, common, task)
	if err != nil {
		return err
	}
	db, err := s.split.open(file)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := checkSplitProject(ctx, tx, project); err != nil {
		return err
	}
	if err := apply(common, tx); err != nil {
		return err
	}
	return tx.Commit()
}
