package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// TaskContinuation selects settings for subsequent turns of a saved conversation.
type TaskContinuation struct {
	Model  string
	Access string
}

// Valid accepts an empty field to keep that setting and "default" to restore
// the executor's configured access.
func (o TaskContinuation) Valid() bool {
	return ValidTaskModel(o.Model) && (ValidTaskAccess(o.Access) || o.Access == "default")
}

// ContinueFromWindow explicitly resumes a finished conversation with changed
// settings. A working task still uses the ordinary owner-message path.
func (s *Store) ContinueFromWindow(task, opID, text string, options TaskContinuation, files []TaskAttachment) error {
	text = strings.TrimSpace(text)
	if opID == "" || len(opID) > 128 || text == "" || utf8.RuneCountInString(text) > MaxTaskText ||
		options == (TaskContinuation{}) || !options.Valid() {
		return fmt.Errorf("%w: invalid follow-up settings or message", ErrRejected)
	}
	return s.taskTx(task, func(machine, tx *sql.Tx) error {
		link, err := taskLink(tx, task)
		if err != nil {
			return err
		}
		return withMessageAttachments(tx, task, opID, "continue"+continuationOptionsValue(options), text, files, func() error {
			sender := MessageSender{Agent: AudienceOwner}
			if link.remote != nil {
				return continueRemoteTask(machine, tx, task, *link.remote, opID, text, sender, options)
			}
			gone := s.gone
			if gone == nil {
				gone = CheckWorkerGone
			}
			return continueTaskTx(machine, tx, task, opID, text, gone, false, sender, options)
		})
	})
}

func continuationOptionsValue(options TaskContinuation) string {
	if options == (TaskContinuation{}) {
		return ""
	}
	return options.Model + "\x00" + options.Access
}

func recordContinuationOptions(tx *sql.Tx, task, opID string, options TaskContinuation) error {
	return setMeta(tx, "continue_settings:"+opID, task+"\n"+continuationOptionsValue(options))
}

func checkContinuationOptions(tx *sql.Tx, task, opID string, options TaskContinuation) error {
	old, err := metaValue(tx, "continue_settings:"+opID)
	if err != nil {
		return err
	}
	want := ""
	if options != (TaskContinuation{}) {
		want = task + "\n" + continuationOptionsValue(options)
	}
	if old != want {
		return fmt.Errorf("%w: this operation id was used with different follow-up settings", ErrRejected)
	}
	return nil
}

func applyContinuationOptions(machine, tx *sql.Tx, task Task, opID string, options TaskContinuation) error {
	if options.Model != "" {
		var models string
		if err := machine.QueryRow(`SELECT models FROM computer_agents WHERE computer=? AND provider=?`, task.Computer, task.Provider).Scan(&models); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("%w: the agent is unavailable on this computer", ErrRejected)
			}
			return err
		}
		if err := modelAllowed(task.Provider, options.Model, splitModels(models)); err != nil {
			return err
		}
		task.Model = options.Model
	}
	if options.Access != "" {
		task.RequestedAccess = options.Access
		if options.Access == "default" {
			task.RequestedAccess = ""
		}
	}
	// The normal launch transaction still applies the executor's current
	// agent policy and its network-access limit.
	if _, err := tx.Exec(`UPDATE tasks SET model=?,requested_access=? WHERE id=?`, task.Model, task.RequestedAccess, task.ID); err != nil {
		return err
	}
	return recordContinuationOptions(tx, task.ID, opID, options)
}
