package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// These messages refer to a task this computer already assigned. A new
// start or a resource-changing request takes the machine-wide admission
// path and cannot be dispatched through the project response handler.
func isProjectResponse(kind string) bool {
	switch kind {
	case "accepted", "refused", "event", "answer_refused", "continue_refused",
		"note_refused", "progress", "ran_model", "output", "image":
		return true
	default:
		return false
	}
}

func applyProjectResponseTx(tx *sql.Tx, from netLink, body netBody, peer string) error {
	switch body.Kind {
	case "output":
		return applyOutput(tx, from, body)
	case "image":
		return applyImage(tx, from, body)
	default:
		return applyReport(tx, from, body, peer)
	}
}

// applySplitProjectResponse commits the task change and a receipt in its
// project DB before acknowledging the broker sequence in the computer DB.
// The caller supplies the DBs of one verified layout and delivers messages
// in broker order. A retry after a crash between commits sees the project's
// receipt and only advances the cursor. It never repeats the task change.
func applySplitProjectResponse(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), broker string, m NetIncoming) error {
	body, valid := decodeNetIncoming(m)
	if !valid || !isProjectResponse(body.Kind) {
		return fmt.Errorf("%w: this delivery requires the computer's inbox handler", ErrRejected)
	}
	common, err := computer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer common.Rollback()
	apply, err := claimNetDelivery(common, broker, m)
	if err != nil {
		return err
	}
	if apply {
		var project, file string
		err := common.QueryRowContext(ctx, `SELECT r.project,s.file
 FROM task_storage_routes r JOIN project_storage_stage s ON s.project=r.project
 WHERE r.id=? AND r.remote_broker=? AND r.remote_peer=?`, body.Task, broker, m.From).Scan(&project, &file)
		if errors.Is(err, sql.ErrNoRows) {
			// The route may still need to be restored or rebound. Do not
			// acknowledge the message on a guess.
			return fmt.Errorf("%w: no confirmed project route for this delivery", ErrRejected)
		}
		if err != nil {
			return err
		}
		if file != projectMigrationFile(project) {
			return fmt.Errorf("%w: invalid project database route", ErrRejected)
		}
		db, err := openProject(file)
		if err != nil {
			return err
		}
		peer := peerName(common, m.From)
		from := netLink{broker: broker, peer: m.From, task: body.Task}
		if err := commitProjectResponse(ctx, db, project, from, m, body, peer); err != nil {
			return err
		}
	}
	if err := advanceNetCursor(common, broker, m.Seq); err != nil {
		return err
	}
	return common.Commit()
}

func commitProjectResponse(ctx context.Context, db *sql.DB, project string, from netLink, m NetIncoming, body netBody, peer string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var known bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tasks
 WHERE id=? AND project=? AND remote_broker=? AND remote_peer=?)`, from.task, project, from.broker, from.peer).Scan(&known); err != nil {
		return err
	}
	if !known {
		return fmt.Errorf("%w: the project database does not contain the routed task", ErrRejected)
	}
	apply, err := claimNetDelivery(tx, from.broker, m)
	if err != nil {
		return err
	}
	if apply {
		if err := applyProjectResponseTx(tx, from, body, peer); err != nil {
			return err
		}
	}
	return tx.Commit()
}
