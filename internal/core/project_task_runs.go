package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

// startSplitTask records the task and its launch intent or start message
// together in the project database. The computer route gates launch or
// delivery. Repeating the explicit operation can repair a missing route.
func startSplitTask(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), r StartTaskRequest) (string, bool, error) {
	r, err := normalizeStartTask(r)
	if err != nil {
		return "", false, err
	}
	common, err := beginSplitAdmission(ctx, computer)
	if err != nil {
		return "", false, err
	}
	defer common.Rollback()
	self, err := localComputer(common)
	if err != nil {
		return "", false, err
	}
	if r.Computer == "" {
		r.Computer = self
	}
	file, err := splitTaskProjectFile(ctx, common, r.Project)
	if err != nil {
		return "", false, err
	}
	db, err := openProject(file)
	if err != nil {
		return "", false, err
	}
	project, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", false, err
	}
	defer project.Rollback()
	if err := checkSplitProject(ctx, project, r.Project); err != nil {
		return "", false, err
	}
	id, err := existingStartTask(project, r)
	created := errors.Is(err, sql.ErrNoRows)
	if created {
		r.Rules, err = startProjectRules(common, r.Project)
		if err != nil {
			return "", false, err
		}
		if r.Computer == self {
			cfg, prepareErr := prepareLocalTask(common, r, self, false, false)
			if prepareErr != nil {
				return "", false, prepareErr
			}
			id, err = recordLocalTask(project, r, self, nil, cfg)
		} else {
			cfg, prepareErr := prepareRemoteTask(common, r)
			if prepareErr != nil {
				return "", false, prepareErr
			}
			id, err = recordRemoteTask(project, r, self, cfg)
		}
		if err != nil {
			return "", false, err
		}
	} else if err != nil {
		return "", false, err
	}
	var broker, peer, machine sql.NullString
	if err := project.QueryRowContext(ctx, `SELECT remote_broker,remote_peer,remote_computer FROM tasks WHERE id=?`, id).Scan(&broker, &peer, &machine); err != nil {
		return "", false, err
	}
	if err := project.Commit(); err != nil {
		return "", false, err
	}
	if _, err := common.ExecContext(ctx, `INSERT INTO task_storage_routes(id,project,remote_broker,remote_peer,remote_computer)
 VALUES (?,?,?,?,?) ON CONFLICT(id) DO NOTHING`, id, r.Project, broker, peer, machine); err != nil {
		return "", false, err
	}
	var registered bool
	if err := common.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM task_storage_routes
 WHERE id=? AND project=? AND remote_broker IS ? AND remote_peer IS ?)`, id, r.Project, broker, peer).Scan(&registered); err != nil {
		return "", false, err
	}
	if !registered {
		return "", false, fmt.Errorf("%w: registered task does not match the project route", ErrRejected)
	}
	if err := common.Commit(); err != nil {
		return "", false, err
	}
	return id, created, nil
}

// beginSplitLocalRun uses the shared admission transaction to serialize
// resource checks with other project launches. The project run's durable
// state is authoritative, even if the process stops after it changes.
// This records a launch reservation in SQLite. Its caller still has to use
// the existing process registry before any exec.
func beginSplitLocalRun(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), projectName, run, reservation string) (TaskRun, Task, error) {
	var r TaskRun
	var t Task
	common, err := beginSplitAdmission(ctx, computer)
	if err != nil {
		return r, t, err
	}
	defer common.Rollback()
	file, err := splitTaskProjectFile(ctx, common, projectName)
	if err != nil {
		return r, t, err
	}
	db, err := openProject(file)
	if err != nil {
		return r, t, err
	}
	project, err := db.BeginTx(ctx, nil)
	if err != nil {
		return r, t, err
	}
	defer project.Rollback()
	r, t, err = runTask(project, run)
	if err != nil {
		return r, t, err
	}
	self, err := localComputer(common)
	if err != nil {
		return r, t, err
	}
	if t.Project != projectName || t.Computer != self || r.State != "intent" {
		return r, t, fmt.Errorf("%w: the task does not have a local launch intent in this project", ErrRejected)
	}
	order, err := splitRouteOrder(ctx, common, t)
	if err != nil {
		return r, t, err
	}
	var resource string
	if err := project.QueryRowContext(ctx, "SELECT resource FROM tasks WHERE id=?", t.ID).Scan(&resource); err != nil {
		return r, t, err
	}
	if resource != "" {
		if err := checkSplitTaskResource(ctx, common, project, file, openProject, t, resource, order); err != nil {
			return r, t, err
		}
	}
	if err := beginTaskRunTx(common, project, &r, &t, reservation); err != nil {
		return r, t, err
	}
	if err := project.Commit(); err != nil {
		return r, t, err
	}
	return r, t, common.Commit()
}

func checkSplitProject(ctx context.Context, project *sql.Tx, name string) error {
	var saved string
	if err := project.QueryRowContext(ctx, "SELECT project FROM project_snapshot").Scan(&saved); err != nil {
		return err
	}
	if saved != name {
		return fmt.Errorf("%w: the database belongs to another project", ErrRejected)
	}
	return nil
}

func beginSplitAdmission(ctx context.Context, computer *sql.DB) (*sql.Tx, error) {
	tx, err := computer.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	// Take SQLite's computer-wide writer lock before any project writes,
	// even when the connection was opened with deferred transactions.
	if _, err := tx.ExecContext(ctx, "UPDATE project_storage_stage SET file=file WHERE 0"); err != nil {
		tx.Rollback()
		return nil, err
	}
	return tx, nil
}

func splitTaskProjectFile(ctx context.Context, common *sql.Tx, name string) (string, error) {
	var file string
	err := common.QueryRowContext(ctx, "SELECT file FROM project_storage_stage WHERE project=?", name).Scan(&file)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%w: project is missing from the storage catalog", ErrRejected)
	}
	if err != nil {
		return "", err
	}
	if file != projectMigrationFile(name) {
		return "", fmt.Errorf("%w: invalid project database route", ErrRejected)
	}
	return file, nil
}

func splitRouteOrder(ctx context.Context, common projectRouteQuerier, task Task) (int64, error) {
	var order int64
	err := common.QueryRowContext(ctx, "SELECT rowid FROM task_storage_routes WHERE id=? AND project=?", task.ID, task.Project).Scan(&order)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("%w: task route was not committed", ErrRejected)
	}
	return order, err
}

func checkSplitTaskResource(ctx context.Context, common interface {
	projectFileQuerier
	projectRouteQuerier
}, project projectFileQuerier, currentFile string, openProject func(string) (*sql.DB, error), task Task, resource string, order int64) error {
	holder, err := splitTaskResourceHolder(ctx, common, project, currentFile, openProject, task, resource, order)
	if err == nil && holder.ID != "" {
		return fmt.Errorf("%w: resource %q is already assigned", ErrRejected, resource)
	}
	return err
}

func splitTaskResourceHolder(ctx context.Context, common interface {
	projectFileQuerier
	projectRouteQuerier
}, project projectFileQuerier, currentFile string, openProject func(string) (*sql.DB, error), task Task, resource string, order int64) (Task, error) {
	files, err := splitProjectFiles(ctx, common)
	if err != nil {
		return Task{}, err
	}
	for _, file := range files {
		var read projectFileQuerier = project
		if file != currentFile {
			db, err := openProject(file)
			if err != nil {
				return Task{}, err
			}
			read = db
		}
		holder, err := resourceHolderInProject(ctx, common, read, task, resource, order)
		if err != nil || holder.ID != "" {
			return holder, err
		}
	}
	return Task{}, nil
}

func (s *Store) nextSplitRunIntents(limit int) ([]string, error) {
	ctx := context.Background()
	// Candidate selection is a read. BeginTaskRun rechecks the route and
	// resource under the shared writer lock before it reserves anything.
	common := s.db
	files, err := splitProjectFiles(ctx, common)
	if err != nil {
		return nil, err
	}
	var self string
	err = common.QueryRowContext(ctx, "SELECT value FROM meta WHERE key='computer_id'").Scan(&self)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type candidate struct {
		run, at, resource string
		task              Task
		order             int64
	}
	var eligible []candidate
	for _, file := range files {
		db, err := s.split.open(file)
		if err != nil {
			return nil, err
		}
		rows, err := db.QueryContext(ctx, `SELECT r.id,r.created_at,t.id,t.project,t.created_at,t.resource
 FROM task_runs r JOIN tasks t ON t.id=r.task_id WHERE r.state='intent' AND t.state='running' AND t.computer=?`, self)
		if err != nil {
			return nil, err
		}
		var candidates []candidate
		for rows.Next() {
			var c candidate
			c.task.Computer = self
			if err := rows.Scan(&c.run, &c.at, &c.task.ID, &c.task.Project, &c.task.CreatedAt, &c.resource); err != nil {
				rows.Close()
				return nil, err
			}
			candidates = append(candidates, c)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return nil, err
		}
		for _, c := range candidates {
			c.order, err = splitRouteOrder(ctx, common, c.task)
			if errors.Is(err, ErrRejected) {
				continue
			}
			if err != nil {
				return nil, err
			}
			if c.resource != "" {
				err := checkSplitTaskResource(ctx, common, db, file, s.split.open, c.task, c.resource, c.order)
				if errors.Is(err, ErrRejected) {
					continue
				}
				if err != nil {
					return nil, err
				}
			}
			eligible = append(eligible, c)
		}
	}
	sort.SliceStable(eligible, func(i, j int) bool {
		if eligible[i].at != eligible[j].at {
			return eligible[i].at < eligible[j].at
		}
		return eligible[i].order < eligible[j].order
	})
	if limit >= 0 {
		eligible = eligible[:min(len(eligible), limit)]
	}
	out := make([]string, 0, len(eligible))
	for _, c := range eligible {
		out = append(out, c.run)
	}
	return out, nil
}

func resourceHolderInProject(ctx context.Context, common projectRouteQuerier, read projectFileQuerier, task Task, resource string, order int64) (Task, error) {
	rows, err := read.QueryContext(ctx, `SELECT id,project,created_at,active FROM (
 SELECT t.id,t.project,t.created_at,t.state,EXISTS(SELECT 1 FROM task_runs r
 WHERE r.task_id=t.id AND r.state IN ('launching','running','blocked','interrupted')) AS active
 FROM tasks t WHERE t.computer=? AND t.resource=? AND t.id<>?)
 WHERE active OR state NOT IN ('done','failed','cancelled')`, task.Computer, resource, task.ID)
	if err != nil {
		return Task{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var other Task
		var active bool
		if err := rows.Scan(&other.ID, &other.Project, &other.CreatedAt, &active); err != nil {
			return Task{}, err
		}
		otherOrder, err := splitRouteOrder(ctx, common, other)
		if err != nil {
			return Task{}, err
		}
		if active || other.CreatedAt < task.CreatedAt || other.CreatedAt == task.CreatedAt && otherOrder < order {
			return other, nil
		}
	}
	return Task{}, rows.Err()
}
