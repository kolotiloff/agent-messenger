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
		pending, err := pendingNetMessages(ctx, db, broker, limit, skip)
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

func splitProjectFiles(ctx context.Context, computer *sql.DB) ([]string, error) {
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
	db := computer
	if message.file != migrationComputerFile {
		// The filename was resolved from the catalog when selecting the
		// message. It must remain a member before opening it again.
		var project string
		err := computer.QueryRowContext(ctx, "SELECT project FROM project_storage_stage WHERE file=?", message.file).Scan(&project)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: the outbox is no longer in the project catalog", ErrRejected)
		}
		if err != nil {
			return err
		}
		if message.file != projectMigrationFile(project) {
			return fmt.Errorf("%w: invalid project database route", ErrRejected)
		}
		db, err = openProject(message.file)
		if err != nil {
			return err
		}
	}
	_, err := db.ExecContext(ctx, "DELETE FROM net_outbox WHERE seq=? AND op_id=?", message.Seq, message.OpID)
	return err
}
