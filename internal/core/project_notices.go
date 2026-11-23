package core

import (
	"context"
	"database/sql"
)

// drainProjectNotices delivers machine events to one project's cursor. The
// state change and notice were queued together in the computer database.
// A project receipt survives a failed shared acknowledgement.
func (s *Store) drainProjectNotices(project string, scoped *Store) error {
	if s.split == nil {
		return nil
	}
	ctx := context.Background()
	var pending bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM project_notices WHERE project=?)`, project).Scan(&pending); err != nil || !pending {
		return err
	}
	computer, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer computer.Rollback()
	rows, err := computer.QueryContext(ctx, `SELECT seq,kind,text,created_at FROM project_notices
 WHERE project=? ORDER BY seq LIMIT 100`, project)
	if err != nil {
		return err
	}
	type notice struct {
		seq                 int64
		kind, text, created string
	}
	var batch []notice
	for rows.Next() {
		var n notice
		if err := rows.Scan(&n.seq, &n.kind, &n.text, &n.created); err != nil {
			rows.Close()
			return err
		}
		batch = append(batch, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(batch) == 0 {
		return err
	}
	if err := scoped.tx(func(tx *sql.Tx) error {
		for _, n := range batch {
			result, err := tx.ExecContext(ctx, "INSERT INTO project_notice_receipts(seq) VALUES (?) ON CONFLICT DO NOTHING", n.seq)
			if err != nil {
				return err
			}
			added, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if added == 1 {
				if err := insertProjectNoticeEvent(tx, project, n.kind, n.text, n.created); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	if _, err := computer.ExecContext(ctx, "DELETE FROM project_notices WHERE project=? AND seq<=?", project, batch[len(batch)-1].seq); err != nil {
		return err
	}
	return computer.Commit()
}
