package core

import (
	"database/sql"
	"fmt"
	"strings"
)

const uiOutboxSchema = `CREATE TABLE IF NOT EXISTS ui_outbox (
 id TEXT PRIMARY KEY,
 session_key TEXT NOT NULL REFERENCES sessions(key),
 alias TEXT NOT NULL,
 text TEXT NOT NULL,
 state TEXT NOT NULL CHECK(state IN ('prepared','queued','acknowledged')),
 created_at TEXT NOT NULL
);`

const UIOutboxLimit = 100

type UIEnvelope struct {
	ID        string `json:"message_id"`
	Alias     string `json:"alias"`
	Text      string `json:"text"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
}

type UIOutbox struct {
	SchemaVersion int          `json:"schema_version"`
	Items         []UIEnvelope `json:"items"`
}

// PrepareUIMessage durably binds an immutable envelope to its registered session.
// It never inserts a dispatchable request. Reopening a window cannot launch it.
func (s *Store) PrepareUIMessage(id, alias, text string) error {
	if !ValidUIMessageID(id) || strings.TrimSpace(alias) == "" || len(alias) > 4096 || strings.TrimSpace(text) == "" || len(text) > 16000 {
		return fmt.Errorf("%w: invalid UI envelope", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		key, decision, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		var oldKey, oldAlias, oldText string
		err = tx.QueryRow(`SELECT session_key,alias,text FROM ui_outbox WHERE id=?`, id).Scan(&oldKey, &oldAlias, &oldText)
		if err == nil {
			if oldKey != key || oldAlias != alias || oldText != text {
				return fmt.Errorf("%w: outbox ID conflict", ErrRejected)
			}
			return nil
		}
		if err != sql.ErrNoRows {
			return err
		}
		if decision == "closed_without_retry" {
			return ErrClosed
		}
		var count int
		if err = tx.QueryRow(`SELECT count(*) FROM ui_outbox WHERE state!='acknowledged'`).Scan(&count); err != nil {
			return err
		}
		if count >= UIOutboxLimit {
			return fmt.Errorf("%w: review existing outbox messages first", ErrRejected)
		}
		// Old UI requests can be adopted only when their exact route/content match.
		var requestKey, sender, requestText string
		state := "prepared"
		err = tx.QueryRow(`SELECT session_key,sender,text FROM requests WHERE id=?`, id).Scan(&requestKey, &sender, &requestText)
		if err == nil {
			if requestKey != key || sender != "local-user" || requestText != text {
				return ErrRejected
			}
			state = "queued"
		} else if err != sql.ErrNoRows {
			return err
		}
		if _, err = tx.Exec(`INSERT INTO ui_outbox(id,session_key,alias,text,state,created_at) VALUES(?,?,?,?,?,?)`, id, key, alias, text, state, now()); err != nil {
			return err
		}
		return insertEvent(tx, "ui-prepare", key, "", id)
	})
}

// SubmitUIMessage atomically inserts the request and marks its durable envelope.
// Repeating an already-queued ID remains a no-op even if the session later closes.
func (s *Store) SubmitUIMessage(id string) (bool, error) {
	created := false
	err := s.tx(func(tx *sql.Tx) error {
		var key, alias, text, state string
		if err := tx.QueryRow(`SELECT session_key,alias,text,state FROM ui_outbox WHERE id=?`, id).Scan(&key, &alias, &text, &state); err != nil {
			return err
		}
		actual, decision, err := sessionByAlias(tx, alias)
		if err != nil {
			return err
		}
		if actual != key {
			return ErrRejected
		}
		var oldKey, oldSender, oldText string
		err = tx.QueryRow(`SELECT session_key,sender,text FROM requests WHERE id=?`, id).Scan(&oldKey, &oldSender, &oldText)
		if err == nil {
			if oldKey != key || oldSender != "local-user" || oldText != text {
				return ErrRejected
			}
			if state == "prepared" {
				_, err = tx.Exec(`UPDATE ui_outbox SET state='queued' WHERE id=?`, id)
			}
			return err
		}
		if err != sql.ErrNoRows {
			return err
		}
		// A queued marker with a missing request is damaged data, never permission to
		// replay external work. There is no automatic repair or fresh ID here.
		if state != "prepared" {
			return ErrRejected
		}
		if decision == "closed_without_retry" {
			return ErrClosed
		}
		if _, err = tx.Exec(`INSERT INTO requests(id,session_key,sender,text,state,created_at) VALUES(?,?, 'local-user',?,'queued',?)`, id, key, text, now()); err != nil {
			return err
		}
		if _, err = tx.Exec(`UPDATE ui_outbox SET state='queued' WHERE id=?`, id); err != nil {
			return err
		}
		created = true
		return insertEvent(tx, "enqueue", key, "", id)
	})
	return created, err
}

// AcknowledgeUIMessage only hides an already-queued envelope from recovery UI.
// Queue, attempts, answers and the immutable outbox record are preserved.
func (s *Store) AcknowledgeUIMessage(id string) error {
	return s.tx(func(tx *sql.Tx) error {
		result, err := tx.Exec(`UPDATE ui_outbox SET state='acknowledged' WHERE id=? AND state IN ('queued','acknowledged') AND EXISTS (SELECT 1 FROM requests r WHERE r.id=ui_outbox.id AND r.session_key=ui_outbox.session_key AND r.sender='local-user' AND r.text=ui_outbox.text)`, id)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return ErrRejected
		}
		return nil
	})
}

func (s *Store) UIOutbox() (UIOutbox, error) {
	out := UIOutbox{SchemaVersion: 1, Items: []UIEnvelope{}}
	rows, err := s.db.Query(`SELECT o.id,o.alias,o.text,o.state,o.created_at,COALESCE(r.id IS NOT NULL AND r.session_key=o.session_key AND r.sender='local-user' AND r.text=o.text,0) FROM ui_outbox o LEFT JOIN requests r ON r.id=o.id WHERE o.state!='acknowledged' ORDER BY o.rowid LIMIT ?`, UIOutboxLimit+1)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var item UIEnvelope
		var recorded bool
		if err = rows.Scan(&item.ID, &item.Alias, &item.Text, &item.State, &item.CreatedAt, &recorded); err != nil {
			return out, err
		}
		if item.State == "queued" && !recorded {
			return out, ErrRejected
		}
		out.Items = append(out.Items, item)
	}
	if err = rows.Err(); err != nil {
		return out, err
	}
	if len(out.Items) > UIOutboxLimit {
		return out, fmt.Errorf("%w: outbox exceeds limit", ErrRejected)
	}
	return out, nil
}

func ValidUIMessageID(id string) bool {
	if len(id) != 39 || !strings.HasPrefix(id, "ui-") {
		return false
	}
	for i, c := range id[3:] {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
