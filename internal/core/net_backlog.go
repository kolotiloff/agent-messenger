package core

import (
	"database/sql"
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
