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
		"note_refused", "progress", "ran_model", "output", "image", "run_usage":
		return true
	default:
		return false
	}
}

func isProjectRequest(kind string) bool {
	switch kind {
	case "answer", "continue", continueWithSettingsKind, "note", "cancel", "rename_task", "archive_task":
		return true
	default:
		return false
	}
}

func applyProjectResponseTx(tx *sql.Tx, from netLink, body netBody, peer string) error {
	switch body.Kind {
	case "run_usage":
		return applyTaskAttempt(tx, from, body.Analytics)
	case "output":
		return applyOutput(tx, from, body)
	case "image":
		return applyImage(tx, from, body)
	default:
		return applyReport(tx, from, body, peer)
	}
}

func applyProjectRequestTx(machine, tx *sql.Tx, from netLink, m NetIncoming, body netBody, gone WorkerGone) error {
	switch body.Kind {
	case "answer":
		return applyAnswer(machine, tx, from, m.OpID, body)
	case "continue", continueWithSettingsKind:
		return applyContinue(machine, tx, from, body, gone)
	case "note":
		return applyNote(machine, tx, from, body)
	case "cancel":
		return applyCancel(machine, tx, from, gone)
	case "rename_task", "archive_task":
		return applyTaskMetadata(tx, from, body)
	default:
		return fmt.Errorf("%w: the delivery is not a task request", ErrRejected)
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
	return applySplitProjectMessage(ctx, computer, openProject, broker, m, body, nil)
}

// applySplitProjectMessage applies a routed response or request. Requests
// are sent by a task's coordinator and must match its recorded origin.
// A continuation uses the current machine policy while the common writer
// lock is held. It only records an intent. Dispatch still verifies it.
func applySplitProjectMessage(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), broker string, m NetIncoming, body netBody, gone WorkerGone) error {
	request := isProjectRequest(body.Kind)
	common, err := computer.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer common.Rollback()
	apply := true
	if !taskMetadataUpdate(body.Kind) {
		apply, err = claimNetDelivery(common, broker, m)
		if err != nil {
			return err
		}
	}
	if apply {
		var task, project, file string
		where := "r.id=? AND r.remote_broker=? AND r.remote_peer=?"
		if request {
			where = "r.origin_task=? AND r.origin_broker=? AND r.origin_peer=?"
		}
		resolve := func() error {
			return common.QueryRowContext(ctx, `SELECT r.id,r.project,s.file
 FROM task_storage_routes r JOIN project_storage_stage s ON s.project=r.project WHERE `+where, body.Task, broker, m.From).Scan(&task, &project, &file)
		}
		err := resolve()
		if errors.Is(err, sql.ErrNoRows) && !taskMetadataUpdate(body.Kind) && validComputer(body.Computer) != "" {
			if err := rebindSplitPeer(ctx, common, openProject, broker, NetPeer{ID: m.From, Computer: body.Computer}); err != nil {
				return err
			}
			err = resolve()
		}
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// No task here matches this delivery, even after the peer was
			// rebound: a reply for a task this computer never published,
			// or a message the other computer should not have sent. Like
			// the single database, skip it. Holding the cursor would stop
			// every later message from every computer (D-188).
		case err != nil:
			return err
		case file != projectMigrationFile(project):
			return fmt.Errorf("%w: invalid project database route", ErrRejected)
		default:
			db, err := openProject(file)
			if err != nil {
				return err
			}
			peer := peerName(common, m.From)
			from := netLink{broker: broker, peer: m.From, task: body.Task}
			if err := commitProjectDelivery(ctx, common, db, task, project, from, m, body, peer, gone); err != nil {
				return err
			}
		}
	}
	if err := advanceNetCursor(common, broker, m.Seq); err != nil {
		return err
	}
	return common.Commit()
}

func commitProjectDelivery(ctx context.Context, common *sql.Tx, db *sql.DB, task, project string, from netLink, m NetIncoming, body netBody, peer string, gone WorkerGone) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	link := "remote_broker=? AND remote_peer=? AND id=?"
	if isProjectRequest(body.Kind) {
		link = "origin_broker=? AND origin_peer=? AND origin_task=?"
	}
	var known bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM tasks
 WHERE id=? AND project=? AND `+link+`)`, task, project, from.broker, from.peer, from.task).Scan(&known); err != nil {
		return err
	}
	if !known {
		// The route names a task the project database lacks. Skip the
		// message rather than stall the inbox (D-188).
		return nil
	}
	apply := true
	if !taskMetadataUpdate(body.Kind) {
		apply, err = claimNetDelivery(tx, from.broker, m)
		if err != nil {
			return err
		}
	}
	if apply {
		if isProjectRequest(body.Kind) {
			err = applyProjectRequestTx(common, tx, from, m, body, gone)
		} else {
			err = applyProjectResponseTx(tx, from, body, peer)
		}
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
