package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// applySplitRemoteStart admits a new network assignment from an ordered
// broker delivery. It commits the task and intent before publishing the
// route, start reply and cursor in a single computer transaction.
func applySplitRemoteStart(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), broker string, m NetIncoming) error {
	body, valid := decodeNetIncoming(m)
	if !valid || body.Kind != "start" {
		return fmt.Errorf("%w: this delivery is not a remote start", ErrRejected)
	}
	common, err := beginSplitAdmission(ctx, computer)
	if err != nil {
		return err
	}
	defer common.Rollback()
	apply, err := claimNetDelivery(common, broker, m)
	if err != nil {
		return err
	}
	if apply {
		from := netLink{broker: broker, peer: m.From, task: body.Task}
		if err := rebindSplitPeer(ctx, common, openProject, broker, NetPeer{ID: m.From, Computer: body.Computer}); err != nil {
			return err
		}
		if err := applySplitStart(ctx, common, openProject, from, body); err != nil {
			return err
		}
	}
	if err := advanceNetCursor(common, broker, m.Seq); err != nil {
		return err
	}
	return common.Commit()
}

func applySplitStart(ctx context.Context, common *sql.Tx, openProject func(string) (*sql.DB, error), from netLink, body netBody) error {
	var routed bool
	if err := common.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM task_storage_routes
 WHERE origin_broker=? AND origin_peer=? AND origin_task=?)`, from.broker, from.peer, from.task).Scan(&routed); err != nil || routed {
		return err
	}
	var file string
	err := common.QueryRowContext(ctx, "SELECT file FROM project_storage_stage WHERE project=?", body.Project).Scan(&file)
	if errors.Is(err, sql.ErrNoRows) {
		if _, err := remoteStartRequest(common, from, body); errors.Is(err, ErrRejected) {
			return enqueueStartReply(common, from, "refused", netBody{Reason: refusalText(err)})
		} else if err != nil {
			return err
		}
		// The project row and its catalog entry are written together, so
		// this does not heal by waiting. Refusing keeps the inbox moving
		// and tells the computer that gave the task why (D-188).
		return refuseSplitStart(common, from, fmt.Errorf("%w: visible project is missing from the storage catalog", ErrRejected))
	}
	if err != nil {
		return err
	}
	if file != projectMigrationFile(body.Project) {
		return refuseSplitStart(common, from, fmt.Errorf("%w: invalid project database route", ErrRejected))
	}
	db, err := openProject(file)
	if err != nil {
		return err
	}
	project, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer project.Rollback()
	if err := checkSplitProject(ctx, project, body.Project); errors.Is(err, ErrRejected) {
		return refuseSplitStart(common, from, err)
	} else if err != nil {
		return err
	}
	task, known, err := mirrorTask(project, from)
	if err != nil {
		return err
	}
	if !known {
		task, err = recordSplitRemoteTask(ctx, common, project, from, body)
		if errors.Is(err, ErrRejected) {
			return enqueueStartReply(common, from, "refused", netBody{Reason: refusalText(err)})
		}
		if err != nil {
			return err
		}
	}
	var originComputer, resource string
	if err := project.QueryRowContext(ctx, "SELECT origin_computer,project,computer,created_at,resource FROM tasks WHERE id=?", task.ID).
		Scan(&originComputer, &task.Project, &task.Computer, &task.CreatedAt, &resource); err != nil {
		return err
	}
	pending, err := runPendingLaunch(project, task.ID)
	if err != nil {
		return err
	}
	if err := project.Commit(); err != nil {
		return err
	}
	if _, err := common.ExecContext(ctx, `INSERT INTO task_storage_routes(id,project,origin_broker,origin_peer,origin_task,origin_computer)
 VALUES (?,?,?,?,?,?)`, task.ID, body.Project, from.broker, from.peer, from.task, originComputer); err != nil {
		return err
	}
	reply := netBody{Mirror: task.ID, Folder: task.Folder, LaunchPending: pending}
	if pending && resource != "" {
		// This is an optional explanation. The dispatcher independently
		// checks the lock before starting a process.
		if order, err := splitRouteOrder(ctx, common, task); err == nil {
			if holder, err := splitTaskResourceHolder(ctx, common, db, file, openProject, task, resource, order); err == nil && holder.ID != "" {
				reply.Lock = resource
			}
		}
	}
	return enqueueStartReply(common, from, "accepted", reply)
}

func refuseSplitStart(common *sql.Tx, from netLink, err error) error {
	return enqueueStartReply(common, from, "refused", netBody{Reason: refusalText(err)})
}

func recordSplitRemoteTask(ctx context.Context, common, project *sql.Tx, from netLink, body netBody) (Task, error) {
	var r StartTaskRequest
	var cfg localTaskConfig
	if err := savepoint(common, func() error {
		var err error
		r, err = remoteStartRequest(common, from, body)
		if err == nil {
			cfg, err = prepareLocalTask(common, r, r.Computer, true, body.CoordinatorModel)
		}
		return err
	}); err != nil {
		return Task{}, err
	}
	id, err := recordLocalTask(project, r, r.Computer, &from, cfg)
	if err != nil {
		return Task{}, err
	}
	_, err = project.ExecContext(ctx, "UPDATE tasks SET origin_computer=? WHERE id=?", validComputer(body.Computer), id)
	return Task{ID: id, Folder: cfg.folder}, err
}
