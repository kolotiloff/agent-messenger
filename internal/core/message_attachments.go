package core

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const messageAttachmentsSince = 143

const messageAttachmentsSchema = `
CREATE TABLE IF NOT EXISTS message_attachments (
 op_id TEXT PRIMARY KEY,
 task_id TEXT NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
 fingerprint TEXT NOT NULL,
 payload BLOB NOT NULL
);
`

// AddOwnerMessageWithAttachments uses the normal note or answer transaction.
func (s *Store) AddOwnerMessageWithAttachments(task, question, opID, text string, files []TaskAttachment) error {
	if opID == "" || len(opID) > 128 || len(files) == 0 {
		return fmt.Errorf("%w: the message needs an operation id and attachments", ErrRejected)
	}
	// Use a separate operation namespace so a text-only retry cannot acknowledge
	// an earlier attachment send or silently turn it into a text-only message.
	opID = fmt.Sprintf("files-%x", sha256.Sum256([]byte(opID)))
	return s.taskTx(task, func(machine, tx *sql.Tx) error {
		return withMessageAttachments(tx, task, opID, question, text, files, func() error {
			if question != "" {
				return answerTask(machine, tx, task, question, opID, text, AudienceOwner, false, false)
			}
			gone := s.gone
			if gone == nil {
				gone = CheckWorkerGone
			}
			return addNoteTx(machine, tx, task, "", opID, text, AudienceOwner, false, gone)
		})
	})
}

// Store the files and accept the message in the same transaction. The caller
// rolls both back on refusal, including when applying a remote message.
func withMessageAttachments(tx *sql.Tx, task, opID, kind, text string, input []TaskAttachment, apply func() error) error {
	if len(input) == 0 {
		return apply()
	}
	files, err := normalizeTaskAttachments(input)
	if err != nil {
		return err
	}
	body, err := json.Marshal(files)
	if err != nil {
		return err
	}
	fingerprint := fmt.Sprintf("%x", sha256.Sum256([]byte(kind+"\x00"+strings.TrimSpace(text))))
	var oldTask, oldFingerprint string
	var oldBody []byte
	err = tx.QueryRow(`SELECT task_id,fingerprint,payload FROM message_attachments WHERE op_id=?`, opID).Scan(&oldTask, &oldFingerprint, &oldBody)
	if err == nil {
		if oldTask == task && oldFingerprint == fingerprint && string(oldBody) == string(body) {
			// The record was committed together with the message.
			return nil
		}
		return fmt.Errorf("%w: this operation id was used for another message", ErrRejected)
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// The message handlers validate the task. An INSERT with an unknown task
	// must preserve the same rejection rather than returning a database error.
	result, err := tx.Exec(`INSERT INTO message_attachments SELECT ?,id,?,? FROM tasks WHERE id=?`, opID, fingerprint, body, task)
	if err != nil {
		return err
	}
	if n, err := result.RowsAffected(); err != nil || n == 0 {
		if err != nil {
			return err
		}
		return fmt.Errorf("%w: unknown task", ErrRejected)
	}
	return apply()
}

func enqueueTaskMessage(machine, tx *sql.Tx, remote netLink, messageID, task, opID string, body netBody) error {
	var raw []byte
	err := tx.QueryRow(`SELECT payload FROM message_attachments WHERE task_id=? AND op_id=?`, task, opID).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if len(raw) > 0 {
		peer, found, err := netPeer(machine, remote.peer)
		if err != nil {
			return err
		}
		if !found || !previewBuild.MatchString(peer.Version) || !peerBuildAtLeast(peer.Version, messageAttachmentsSince) {
			return fmt.Errorf("%w: update the executor's Messenger to send attachments with a message", ErrRejected)
		}
		if err := json.Unmarshal(raw, &body.Attachments); err != nil {
			return err
		}
	}
	return enqueueNet(tx, remote.broker, remote.peer, messageID, task, body)
}

// Deliver a message's files only to the run that receives it, through the same
// receipt-scoped worker tool that materializes the original task's files.
func appendRunAttachments(tx *sql.Tx, run TaskRun, initial []TaskAttachment) ([]TaskAttachment, error) {
	rows, err := tx.Query(`SELECT payload FROM message_attachments AS a WHERE a.task_id=? AND (
   a.op_id=(SELECT op_id FROM task_runs WHERE id=?) OR
   a.op_id IN (SELECT op_id FROM task_owner_notes WHERE run_id=?)
 ) ORDER BY a.rowid`, run.TaskID, run.ID, run.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var prefix string
	group := 0
	for rows.Next() {
		if group == 0 {
			prefix = "message-attachments"
			for attachmentRootUsed(initial, prefix) {
				prefix += "-"
			}
		}
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var files []TaskAttachment
		if err := json.Unmarshal(raw, &files); err != nil {
			return nil, err
		}
		group++
		for _, file := range files {
			file.Path = prefix + "/" + strconv.Itoa(group) + "/" + file.Path
			initial = append(initial, file)
		}
	}
	return initial, rows.Err()
}

func attachmentRootUsed(files []TaskAttachment, root string) bool {
	for _, file := range files {
		name, _, _ := strings.Cut(file.Path, "/")
		if strings.EqualFold(name, root) {
			return true
		}
	}
	return false
}
