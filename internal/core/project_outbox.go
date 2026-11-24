package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
)

// splitNetMessage keeps the database identity with a selected message.
// A sequence number alone is no longer unique once the stores are split.
type splitNetMessage struct {
	NetOutgoing
	file string
}

// splitNetPending uses the same queue selection as the current sender.
// It preserves the original sequence order of migrated frames and the
// per-database FIFO order of new ones. The opener owns the connections.
func splitNetPending(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), broker string, limit int, skip []string) ([]splitNetMessage, error) {
	files, err := splitProjectFiles(ctx, computer)
	if err != nil {
		return nil, err
	}
	out := []splitNetMessage{}
	for _, file := range append([]string{migrationComputerFile}, files...) {
		db := computer
		if file != migrationComputerFile {
			db, err = openProject(file)
			if err != nil {
				return nil, err
			}
		}
		pending, err := splitPendingInFile(ctx, computer, db, file, broker, limit, skip)
		if err != nil {
			return nil, err
		}
		for _, message := range pending {
			out = append(out, splitNetMessage{NetOutgoing: message, file: file})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	if limit >= 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func splitPendingInFile(ctx context.Context, computer, db *sql.DB, file, broker string, limit int, skip []string) ([]NetOutgoing, error) {
	if file == migrationComputerFile {
		return pendingNetMessages(ctx, db, broker, limit, skip)
	}
	var out []NetOutgoing
	var after sql.NullInt64
	for limit < 0 || len(out) < limit {
		batch := limit
		if limit >= 0 {
			batch = limit - len(out)
		}
		pending, err := pendingNetMessagesAfter(ctx, db, broker, batch, skip, after)
		if err != nil {
			return nil, err
		}
		for _, m := range pending {
			if m.kind == "start" {
				// The project can commit before the shared route. Only a
				// published route makes this assignment eligible to send.
				var registered bool
				if err := computer.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1
 FROM task_storage_routes r JOIN project_storage_stage p ON p.project=r.project
 WHERE r.id=? AND r.remote_broker=? AND r.remote_peer=? AND p.file=?)`, m.task, broker, m.To, file).Scan(&registered); err != nil {
					return nil, err
				}
				if !registered {
					continue
				}
			}
			out = append(out, m)
		}
		if batch < 0 || len(pending) < batch {
			break
		}
		after = sql.NullInt64{Int64: pending[len(pending)-1].Seq, Valid: true}
	}
	return out, nil
}

type projectFileQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func splitProjectFiles(ctx context.Context, computer projectFileQuerier) ([]string, error) {
	rows, err := computer.QueryContext(ctx, "SELECT project,file FROM project_storage_stage ORDER BY project")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var files []string
	for rows.Next() {
		var name, file string
		if err := rows.Scan(&name, &file); err != nil {
			return nil, err
		}
		if file != projectMigrationFile(name) {
			return nil, fmt.Errorf("%w: invalid project database route", ErrRejected)
		}
		files = append(files, file)
	}
	return files, rows.Err()
}

// splitNetSent acknowledges the precise outbox row the broker stored.
// It cannot remove an equally numbered message from another database.
func splitNetSent(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), message splitNetMessage) error {
	db, err := splitOutboxDB(ctx, computer, openProject, message.file)
	if err != nil {
		return err
	}
	return acknowledgeNetDelivery(ctx, db, message.NetOutgoing)
}

func splitOutboxDB(ctx context.Context, computer *sql.DB, openProject func(string) (*sql.DB, error), file string) (*sql.DB, error) {
	if file == migrationComputerFile {
		return computer, nil
	}
	// A file selected for sending must still belong to the catalog.
	var project string
	err := computer.QueryRowContext(ctx, "SELECT project FROM project_storage_stage WHERE file=?", file).Scan(&project)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: the outbox is no longer in the project catalog", ErrRejected)
	}
	if err != nil {
		return nil, err
	}
	if file != projectMigrationFile(project) {
		return nil, fmt.Errorf("%w: invalid project database route", ErrRejected)
	}
	return openProject(file)
}
