package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

func (s *Store) refreshSplitPeers(computer *sql.Tx, broker string, peers []NetPeer) error {
	if broker == "" {
		return nil
	}
	ctx := context.Background()
	var ids []string
	for _, peer := range peers {
		if peer.Reconnecting {
			continue
		}
		ids = append(ids, peer.ID)
		if peer.Age >= 0 && peer.Age <= PeerFresh {
			if err := rebindSplitPeer(ctx, computer, s.split.open, broker, peer); err != nil {
				return err
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	files, err := splitProjectFiles(ctx, computer)
	if err != nil {
		return err
	}
	for _, file := range files {
		db, err := s.split.open(file)
		if err != nil {
			return err
		}
		var waiting bool
		if err := db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM net_outbox WHERE broker=?)", parkedPrefix+broker).Scan(&waiting); err != nil {
			return err
		}
		if waiting {
			store := &Store{db: db}
			if err := store.tx(func(tx *sql.Tx) error { return unparkFor(tx, broker, ids) }); err != nil {
				return err
			}
		}
	}
	return nil
}

// rebindSplitPeer uses the catalog to find changed routes only. Project
// rows and their outbox are committed first. If the catalog commit is lost,
// its old route is still selected on the next presence report or message.
func rebindSplitPeer(ctx context.Context, computer *sql.Tx, openProject func(string) (*sql.DB, error), broker string, peer NetPeer) error {
	id := validComputer(peer.Computer)
	if id == "" || broker == "" || peer.ID == "" || peer.Reconnecting {
		return nil
	}
	self, err := localComputer(computer)
	if err != nil || self == id {
		return err
	}
	rows, err := computer.QueryContext(ctx, `SELECT r.project,p.file,r.id FROM task_storage_routes r
 JOIN project_storage_stage p ON p.project=r.project
 WHERE r.remote_computer=? AND (r.remote_broker IS NOT ? OR r.remote_peer IS NOT ?)
 OR COALESCE(r.remote_computer,'')='' AND r.remote_broker=? AND r.remote_peer=?
 OR r.origin_computer=? AND (r.origin_broker IS NOT ? OR r.origin_peer IS NOT ?)
 OR COALESCE(r.origin_computer,'')='' AND r.origin_broker=? AND r.origin_peer=?
 ORDER BY r.project,r.id`, id, broker, peer.ID, broker, peer.ID, id, broker, peer.ID, broker, peer.ID)
	if err != nil {
		return err
	}
	type routeGroup struct {
		project, file string
		ids           []string
	}
	var groups []routeGroup
	for rows.Next() {
		var project, file, task string
		if err := rows.Scan(&project, &file, &task); err != nil {
			rows.Close()
			return err
		}
		if file != projectMigrationFile(project) {
			rows.Close()
			return fmt.Errorf("%w: invalid project database route", ErrRejected)
		}
		if len(groups) == 0 || groups[len(groups)-1].project != project {
			groups = append(groups, routeGroup{project: project, file: file})
		}
		last := &groups[len(groups)-1]
		last.ids = append(last.ids, task)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, group := range groups {
		db, err := openProject(group.file)
		if err != nil {
			return err
		}
		if err := commitSplitPeerRoutes(ctx, computer, db, group.project, group.ids, broker, peer); err != nil {
			return err
		}
	}
	return nil
}

func commitSplitPeerRoutes(ctx context.Context, computer *sql.Tx, db *sql.DB, project string, tasks []string, broker string, peer NetPeer) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := checkSplitProject(ctx, tx, project); err != nil {
		return err
	}
	if err := fillComputers(tx, broker, []NetPeer{peer}); err != nil {
		return err
	}
	if err := rebindComputer(tx, peer.Computer, broker, peer.ID); err != nil {
		return err
	}
	for _, task := range tasks {
		var rb, rp, rc, ob, op, oc sql.NullString
		err := tx.QueryRowContext(ctx, `SELECT remote_broker,remote_peer,remote_computer,origin_broker,origin_peer,origin_computer
 FROM tasks WHERE id=? AND project=?`, task, project).Scan(&rb, &rp, &rc, &ob, &op, &oc)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: rebound task is missing from its project", ErrRejected)
		}
		if err != nil {
			return err
		}
		// The start acknowledgement was published with the catalog route.
		// Move it with that route, using the old origin and task together.
		if ob.Valid && op.Valid {
			if _, err := computer.ExecContext(ctx, `UPDATE net_outbox SET broker=?,recipient=?
 WHERE kind IN ('accepted','refused') AND EXISTS(SELECT 1 FROM task_storage_routes r
 WHERE r.id=? AND r.project=? AND r.origin_task=net_outbox.task_id AND r.origin_peer=net_outbox.recipient
 AND (r.origin_broker=net_outbox.broker OR ('parked:'||r.origin_broker)=net_outbox.broker))`, ob, op, task, project); err != nil {
				return err
			}
		}
		if _, err := computer.ExecContext(ctx, `UPDATE task_storage_routes
 SET remote_broker=?,remote_peer=?,remote_computer=?,origin_broker=?,origin_peer=?,origin_computer=?
 WHERE id=? AND project=?`, rb, rp, rc, ob, op, oc, task, project); err != nil {
			return err
		}
	}
	return tx.Commit()
}
