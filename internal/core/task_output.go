package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Worker outputs (T-087): the worker's last message of each run, kept in
// full, so a long final answer is never lost when its report is short. The
// coordinator reads it with task_output. A task of another computer sends
// it back before its event, so the computer that started the task has it.

const taskOutputSchema = `
CREATE TABLE IF NOT EXISTS task_outputs (
 task_id TEXT NOT NULL REFERENCES tasks(id),
 ordinal INTEGER NOT NULL,
 text TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY (task_id, ordinal)
);
`

// MaxTaskOutput bounds a kept output in characters.
const MaxTaskOutput = 60001

// TaskOutputChunk is how much task_output returns at once.
const TaskOutputChunk = 20000

// outputMarker starts the note the coordinator gets with a long output.
const outputMarker = "[Messenger] The worker's full final message"

func saveTaskOutput(tx *sql.Tx, task string, ordinal int, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	text = clip(text, MaxTaskOutput)
	if _, err := tx.Exec(`INSERT INTO task_outputs(task_id,ordinal,text,created_at) VALUES (?,?,?,?) ON CONFLICT DO UPDATE SET text=excluded.text`, task, ordinal, text, now()); err != nil {
		return err
	}
	link, err := taskLink(tx, task)
	if err != nil || link.origin == nil {
		return err
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	return enqueueNet(tx, link.origin.broker, link.origin.peer, "output."+id, task, netBody{Kind: "output", Task: link.origin.task, Text: clipBytes(text, maxOutputBytes), Lines: ordinal})
}

// maxOutputBytes keeps an output message well under the broker's limit.
const maxOutputBytes = 180000

// clipBytes shortens text to at most n bytes on a character boundary.
func clipBytes(text string, n int) string {
	if len(text) <= n {
		return text
	}
	cut := n
	for cut > 0 && !utf8.RuneStart(text[cut]) {
		cut--
	}
	return text[:cut] + "…"
}

// applyOutput keeps the output another computer sent for a task this
// computer started there.
func applyOutput(tx *sql.Tx, from netLink, b netBody) error {
	var id string
	err := tx.QueryRow(`SELECT id FROM tasks WHERE id=? AND remote_broker=? AND remote_peer=?`, from.task, from.broker, from.peer).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) || b.Lines < 1 || b.Lines > 10000 {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO task_outputs(task_id,ordinal,text,created_at) VALUES (?,?,?,?) ON CONFLICT DO UPDATE SET text=excluded.text`, id, b.Lines, clip(strings.TrimSpace(b.Text), MaxTaskOutput), now())
	return err
}

// withOutputNote adds a line about the task's latest output to an event
// text when the output says more than the text itself.
func withOutputNote(tx *sql.Tx, task, text string) (string, error) {
	if strings.Contains(text, outputMarker) {
		return text, nil
	}
	var out string
	err := tx.QueryRow(`SELECT text FROM task_outputs WHERE task_id=? ORDER BY ordinal DESC LIMIT 1`, task).Scan(&out)
	if errors.Is(err, sql.ErrNoRows) {
		return text, nil
	}
	if err != nil {
		return "", err
	}
	n := utf8.RuneCountInString(out)
	if n <= utf8.RuneCountInString(text)+200 || strings.Contains(text, out) {
		return text, nil
	}
	return strings.TrimSpace(text + "\n" + fmt.Sprintf("%s has %d characters; read it with task_output before asking the worker again.", outputMarker, n)), nil
}

// TaskOutput is one part of a task's kept worker output.
type TaskOutput struct {
	TaskID string `json:"task_id"`
	Run    int    `json:"run"`
	Total  int    `json:"total_characters"`
	Offset int    `json:"offset"`
	Text   string `json:"text"`
	Next   int    `json:"next_offset,omitempty"`
	Done   bool   `json:"done"`
}

// TaskOutputOf returns up to TaskOutputChunk characters of the task's
// output from offset; run 0 is the latest run that left one.
func (s *Store) TaskOutputOf(task string, run, offset int) (TaskOutput, error) {
	if offset < 0 || run < 0 {
		return TaskOutput{}, fmt.Errorf("%w: offset and run start at 0", ErrRejected)
	}
	var text string
	var err error
	if run == 0 {
		err = s.db.QueryRow(`SELECT ordinal,text FROM task_outputs WHERE task_id=? ORDER BY ordinal DESC LIMIT 1`, task).Scan(&run, &text)
	} else {
		err = s.db.QueryRow(`SELECT text FROM task_outputs WHERE task_id=? AND ordinal=?`, task, run).Scan(&text)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return TaskOutput{}, fmt.Errorf("%w: no worker output is kept for this task (run %d)", ErrRejected, run)
	}
	if err != nil {
		return TaskOutput{}, err
	}
	runes := []rune(text)
	out := TaskOutput{TaskID: task, Run: run, Total: len(runes), Offset: offset}
	if offset > len(runes) {
		offset = len(runes)
	}
	end := offset + TaskOutputChunk
	if end >= len(runes) {
		end, out.Done = len(runes), true
	} else {
		out.Next = end
	}
	out.Text = string(runes[offset:end])
	return out, nil
}
