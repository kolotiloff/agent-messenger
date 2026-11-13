package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Owner notes (T-085, Q-026 a): the owner writes to a worker while it works.
// The note waits; as soon as the current run ends, the same conversation
// continues with it. A report that ends that run is held, not sent to the
// coordinator; a question goes out as usual and the note travels with its
// answer. A task that stops keeps its waiting notes as undelivered: nothing
// sends them again by itself.

// AddOwnerNote records the owner's note to a working task of this computer.
// opID makes it idempotent. A worker that asked the coordinator has ended
// its step and waits: the owner's message answers that question at once,
// instead of waiting for the coordinator (T-125).
func (s *Store) AddOwnerNote(task, opID, text string) error {
	return s.addNote(task, "", opID, text, AudienceOwner)
}

// AddCoordinatorNote records the coordinator's message to a working task of
// its project on this computer (T-114). It travels like the owner's note,
// marked as the coordinator's. A worker that waits for the coordinator gets
// an answer instead; one that asked the owner waits for the owner.
func (s *Store) AddCoordinatorNote(project, task, opID, text string) error {
	if project == "" {
		return fmt.Errorf("%w: no project", ErrRejected)
	}
	return s.addNote(task, project, opID, text, AudienceCoordinator)
}

func (s *Store) addNote(task, project, opID, text, author string) error {
	text = strings.TrimSpace(text)
	if opID == "" || len(opID) > 128 || text == "" || utf8.RuneCountInString(text) > MaxTaskText {
		return fmt.Errorf("%w: a message needs an operation id and 1..%d characters", ErrRejected, MaxTaskText)
	}
	return s.tx(func(tx *sql.Tx) error {
		var oldTask, oldText string
		err := tx.QueryRow(`SELECT task_id,text FROM task_owner_notes WHERE op_id=?`, opID).Scan(&oldTask, &oldText)
		if err == nil {
			if oldTask == task && oldText == text {
				return nil
			}
			return fmt.Errorf("%w: this operation id was used for another message", ErrRejected)
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		var answered int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM task_questions WHERE task_id=? AND answer_op=? AND answer=?`, task, opID, text).Scan(&answered); err != nil {
			return err
		}
		if answered > 0 {
			return nil
		}
		t, err := scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=?`, task).Scan)
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("%w: unknown task", ErrRejected)
		}
		if err != nil {
			return err
		}
		if project != "" && t.Project != project {
			return fmt.Errorf("%w: unknown task", ErrRejected)
		}
		link, err := taskLink(tx, task)
		if err != nil {
			return err
		}
		if link.origin != nil || link.remote != nil {
			return fmt.Errorf("%w: a task of another computer takes no messages yet", ErrRejected)
		}
		if t.State == "waiting_owner" {
			if author == AudienceCoordinator {
				return fmt.Errorf("%w: the worker asked the owner a question and waits for the answer", ErrRejected)
			}
			return fmt.Errorf("%w: the worker asked you a question; answer it instead", ErrRejected)
		}
		if author == AudienceCoordinator && t.State == "waiting_coordinator" && !t.CancelRequested {
			return fmt.Errorf("%w: the worker asked you a question; answer it with answer", ErrRejected)
		}
		if t.State == "waiting_coordinator" && !t.CancelRequested {
			var question string
			err := tx.QueryRow(`SELECT id FROM task_questions WHERE task_id=? AND audience=? AND state='open' ORDER BY rowid DESC LIMIT 1`, task, AudienceCoordinator).Scan(&question)
			if err == nil {
				return answerTask(tx, task, question, opID, text, AudienceOwner, false, true)
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
		}
		if (t.State != "running" && t.State != "waiting_coordinator") || t.CancelRequested {
			return fmt.Errorf("%w: only a working task takes messages; this one is %s", ErrRejected, t.State)
		}
		var waiting int
		if err := tx.QueryRow(`SELECT COALESCE(SUM(LENGTH(text)),0) FROM task_owner_notes WHERE task_id=? AND state='pending'`, task).Scan(&waiting); err != nil {
			return err
		}
		if waiting+len(text) > 4*MaxTaskText {
			return fmt.Errorf("%w: too much is already waiting for the worker", ErrRejected)
		}
		id, err := randomID()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO task_owner_notes(id,task_id,op_id,text,state,created_at,author) VALUES (?,?,?,?,'pending',?,?)`, "note-"+id[:16], task, opID, text, now(), author); err != nil {
			return err
		}
		if author == AudienceCoordinator {
			return nil
		}
		return insertTaskEvent(tx, t.Project, task, AudienceCoordinator, "owner_note",
			"The owner wrote to this worker; it reads this only when its current run ends (a worker CLI takes no input while it works) and then continues the same task:\n"+text, "")
	})
}

// holdForOwnerNotes keeps a run's report back when the owner wrote while it
// worked, and records the next run of the same conversation instead.
func holdForOwnerNotes(tx *sql.Tx, t Task, r TaskRun) (bool, error) {
	if r.Disposition != DispositionReport || t.SessionID == "" {
		return false, nil
	}
	var waiting int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM task_owner_notes WHERE task_id=? AND state='pending'`, t.ID).Scan(&waiting); err != nil || waiting == 0 {
		return false, err
	}
	if err := insertTaskRun(tx, t.ID, r.Ordinal+1, "note", ""); err != nil {
		return false, err
	}
	return true, setTaskState(tx, t.ID, "running", "the owner wrote while it worked; the same conversation continues")
}

// ownerNotesOf is the owner's notes delivered with a run, oldest first.
func ownerNotesOf(tx *sql.Tx, run string) (string, error) {
	rows, err := tx.Query(`SELECT text,author FROM task_owner_notes WHERE run_id=? ORDER BY created_at,rowid`, run)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var parts []string
	for rows.Next() {
		var text, author string
		if err := rows.Scan(&text, &author); err != nil {
			return "", err
		}
		if author == AudienceCoordinator {
			text = "From the coordinator: " + text
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n"), rows.Err()
}

// ownerNoteItems are the task's notes for the window, in time order.
func ownerNoteItems(tx *sql.Tx, task string) ([]FeedItem, error) {
	rows, err := tx.Query(`SELECT text,state,created_at,author FROM task_owner_notes WHERE task_id=? ORDER BY created_at,rowid`, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FeedItem
	for rows.Next() {
		it := FeedItem{Kind: "note", Author: AudienceOwner, Verified: true}
		if err := rows.Scan(&it.Text, &it.State, &it.At, &it.Author); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// mergeByTime puts each note before the first item that came after it.
func mergeByTime(items, notes []FeedItem) []FeedItem {
	for _, n := range notes {
		at := len(items)
		for i, it := range items {
			if i > 0 && it.At > n.At {
				at = i
				break
			}
		}
		items = append(items[:at], append([]FeedItem{n}, items[at:]...)...)
	}
	return items
}
