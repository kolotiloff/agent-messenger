package core

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// NetOutbox is what a computer has not handed to the server yet (T-150): how
// many messages and how many seconds the oldest has waited, by that
// computer's clock, when it last reported.
type NetOutbox struct {
	Waiting   int `json:"waiting"`
	OldestAge int `json:"oldest_age"`
}

// OutboxDelayedAfter is how long a computer's oldest unsent message may wait
// before the coordinator and the window are told (T-150). Messages normally
// leave within a round of a few seconds.
const OutboxDelayedAfter = 3 * time.Minute

// OutboxBacklog is this computer's unsent messages for computers listed in
// its network now: messages for a computer that is not listed, or not
// reporting, wait for it and are not a delay of this computer.
func (s *Store) OutboxBacklog() (NetOutbox, error) {
	if s.split != nil {
		return s.splitOutboxBacklog()
	}
	var out NetOutbox
	var oldest sql.NullString
	err := s.db.QueryRow(`SELECT COUNT(*),MIN(o.created_at) FROM net_outbox o
 WHERE o.broker=(SELECT value FROM meta WHERE key='net_broker')
 AND EXISTS (SELECT 1 FROM net_peers p WHERE p.peer=o.recipient AND p.missing_since='' AND p.age>=0)`).Scan(&out.Waiting, &oldest)
	if err != nil {
		return NetOutbox{}, err
	}
	if t, err := time.Parse(time.RFC3339Nano, oldest.String); err == nil && out.Waiting > 0 {
		out.OldestAge = max(0, int(time.Since(t)/time.Second))
	}
	// Within what the server accepts in a report.
	out.Waiting, out.OldestAge = min(out.Waiting, 1_000_000), min(out.OldestAge, 400*24*3600)
	return out, nil
}

func (s *Store) splitOutboxBacklog() (NetOutbox, error) {
	var out NetOutbox
	err := s.tx(func(machine *sql.Tx) error {
		broker, err := metaValue(machine, "net_broker")
		if err != nil || broker == "" {
			return err
		}
		rows, err := machine.Query("SELECT peer FROM net_peers WHERE missing_since='' AND age>=0")
		if err != nil {
			return err
		}
		args := []any{broker}
		for rows.Next() {
			var peer string
			if err := rows.Scan(&peer); err != nil {
				rows.Close()
				return err
			}
			args = append(args, peer)
		}
		rows.Close()
		if err := rows.Err(); err != nil || len(args) == 1 {
			return err
		}
		query := "SELECT COUNT(*),MIN(created_at) FROM net_outbox WHERE broker=? AND recipient IN (?" + strings.Repeat(",?", len(args)-2) + ")"
		ctx := context.Background()
		add := func(source projectRouteQuerier) error {
			var count int
			var oldest sql.NullString
			if err := source.QueryRowContext(ctx, query, args...).Scan(&count, &oldest); err != nil {
				return err
			}
			out.Waiting += count
			if t, err := time.Parse(time.RFC3339Nano, oldest.String); err == nil {
				out.OldestAge = max(out.OldestAge, int(time.Since(t)/time.Second))
			}
			return nil
		}
		if err := add(machine); err != nil {
			return err
		}
		files, err := splitProjectFiles(ctx, machine)
		if err != nil {
			return err
		}
		for _, file := range files {
			db, err := s.split.open(file)
			if err != nil {
				return err
			}
			if err := add(db); err != nil {
				return err
			}
		}
		return nil
	})
	out.Waiting, out.OldestAge = min(out.Waiting, 1_000_000), min(out.OldestAge, 400*24*3600)
	return out, err
}

// outboxDelay is how many seconds the oldest unsent message of a computer
// has waited by now: its age at the report plus the report's age. Zero when
// it reported nothing waiting or never said.
func (p NetPeer) outboxDelay() int {
	if p.Outbox == nil || p.Outbox.Waiting == 0 || p.Age < 0 {
		return 0
	}
	return p.Outbox.OldestAge + p.Age
}

// OutboxDelayedMinutes is how many whole minutes the computer's oldest
// unsent message has waited, when that is OutboxDelayedAfter or more.
func (p NetPeer) OutboxDelayedMinutes() int {
	if d := p.outboxDelay(); time.Duration(d)*time.Second >= OutboxDelayedAfter {
		return d / 60
	}
	return 0
}

// NetWorkers is what a computer ran for other computers' tasks at its last
// report (T-149): the giver's task id and the seconds since its worker last
// wrote output.
type NetWorkers struct {
	Runs []NetWorkerRun `json:"runs"`
}

// NetWorkerRun is one running worker of another computer's task.
type NetWorkerRun struct {
	Task      string `json:"task"`
	OutputAge int    `json:"output_age"`
}

// OriginTasks maps tasks of this computer that another computer of the
// current network gave it to that computer's task ids, for the running
// workers it reports (T-149).
func (s *Store) OriginTasks(ids []string) (map[string]string, error) {
	out := map[string]string{}
	routes := "tasks"
	if s.split != nil {
		routes = "task_storage_routes"
	}
	for _, id := range ids {
		var origin string
		err := s.db.QueryRow(`SELECT COALESCE(origin_task,'') FROM `+routes+` WHERE id=? AND origin_peer IS NOT NULL
 AND origin_broker=(SELECT value FROM meta WHERE key='net_broker')`, id).Scan(&origin)
		if errors.Is(err, sql.ErrNoRows) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if netRef.MatchString(origin) {
			out[id] = origin
		}
	}
	return out, nil
}
