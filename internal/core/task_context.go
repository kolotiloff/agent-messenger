package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// A nil window invalidates a previous reading, for example after compaction.
// Ordinals order attempts. Sequence orders measurements within an attempt,
// including legitimate decreases in occupancy.
type contextSnapshot struct {
	Ordinal  int            `json:"ordinal"`
	Sequence int64          `json:"sequence"`
	Window   *ContextWindow `json:"window,omitempty"`
}

func (s contextSnapshot) valid() bool {
	return s.Ordinal > 0 && s.Sequence > 0 && (s.Window == nil || (s.Window.CapacityTokens > 0 && s.Window.UsedTokens >= 0))
}

func readTaskContext(q interface{ QueryRow(string, ...any) *sql.Row }, task string) (contextSnapshot, error) {
	var snapshot contextSnapshot
	var raw sql.NullString
	err := q.QueryRow("SELECT context_snapshot FROM tasks WHERE id=?", task).Scan(&raw)
	if err == nil && raw.Valid {
		err = json.Unmarshal([]byte(raw.String), &snapshot)
	}
	return snapshot, err
}

func writeTaskContext(tx *sql.Tx, task string, snapshot contextSnapshot) (json.RawMessage, error) {
	raw, err := json.Marshal(snapshot)
	if err == nil {
		_, err = tx.Exec("UPDATE tasks SET context_snapshot=? WHERE id=?", string(raw), task)
	}
	return raw, err
}

func applyTaskContext(tx *sql.Tx, task string, next contextSnapshot) error {
	old, err := readTaskContext(tx, task)
	if err != nil || old.Ordinal > next.Ordinal || (old.Ordinal == next.Ordinal && old.Sequence >= next.Sequence) {
		return err
	}
	_, err = writeTaskContext(tx, task, next)
	return err
}

// NoteTaskContext records and sends a measured context independently of totals.
func (s *Store) NoteTaskContext(run string, window *ContextWindow) error {
	st, err := s.storeForRun(run)
	if err != nil {
		return err
	}
	return st.tx(func(tx *sql.Tx) error {
		var task string
		next := contextSnapshot{Window: window, Sequence: 1}
		err := tx.QueryRow("SELECT task_id,ordinal FROM task_runs WHERE id=? AND state='running'", run).Scan(&task, &next.Ordinal)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil || !next.valid() {
			return err
		}
		old, err := readTaskContext(tx, task)
		if err != nil || old.Ordinal > next.Ordinal {
			return err
		}
		if old.Ordinal == next.Ordinal {
			if old.Window == nil && window == nil || old.Window != nil && window != nil && *old.Window == *window {
				return nil
			}
			next.Sequence = old.Sequence + 1
		}
		raw, err := writeTaskContext(tx, task, next)
		if err != nil {
			return err
		}
		link, err := taskLink(tx, task)
		if err != nil || link.origin == nil {
			return err
		}
		return enqueueNet(tx, link.origin.broker, link.origin.peer, fmt.Sprintf("context.%s.%d", run, next.Sequence), task,
			netBody{Kind: "progress", Task: link.origin.task, Event: "context_snapshot", ContextSnapshot: raw})
	})
}
