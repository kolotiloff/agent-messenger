package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
)

// Notes to a worker on another computer (T-149). The coordinator's note
// travels as a "note" message; that computer keeps it for the mirror task
// like a local note, so the worker reads it when its current run ends. A
// refusal there comes back as note_refused. Only a computer whose version
// reads the message gets one: an older one would drop it.

// netNoteSince is the first build that reads a "note" message.
const netNoteSince = 67

// peerAtLeast says whether a reported version is the given preview build or
// newer. A version this code cannot read is assumed newer; none is older.
func peerAtLeast(version string, build int) bool {
	if version == "" {
		return false
	}
	m := previewBuild.FindStringSubmatch(version)
	if m == nil {
		return true
	}
	n, err := strconv.Atoi(m[1])
	return err == nil && n >= build
}

// noteRemoteTask sends the coordinator's note to the computer that runs the
// task. The note is kept here too, marked sent, for the window and for a
// repeated call with the same op id.
func noteRemoteTask(tx *sql.Tx, t Task, remote netLink, opID, text string) error {
	switch {
	case t.CancelRequested:
		return fmt.Errorf("%w: the task is being cancelled", ErrRejected)
	case t.State == "waiting_coordinator":
		return fmt.Errorf("%w: the worker asked you a question; answer it with answer", ErrRejected)
	case t.State == "waiting_owner":
		return fmt.Errorf("%w: the worker asked the owner a question and waits for the answer", ErrRejected)
	case t.State != "running":
		return fmt.Errorf("%w: only a working task takes messages; this one is %s", ErrRejected, t.State)
	}
	broker, err := metaValue(tx, "net_broker")
	if err != nil {
		return err
	}
	peer, found, err := netPeer(tx, remote.peer)
	if err != nil {
		return err
	}
	if broker == "" || broker != remote.broker || !found || peer.Age < 0 || peer.Age > PeerFresh {
		return fmt.Errorf("%w: no contact with the computer that runs this task; the note was not sent", ErrRejected)
	}
	if !peerAtLeast(peer.Version, netNoteSince) {
		return fmt.Errorf("%w: %s runs an older Messenger that does not take notes for its workers; update it", ErrRejected, peer.Name)
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO task_owner_notes(id,task_id,op_id,text,state,created_at,author) VALUES (?,?,?,?,'sent',?,?)`, "note-"+id[:16], t.ID, opID, text, now(), AudienceCoordinator); err != nil {
		return err
	}
	return enqueueNet(tx, remote.broker, remote.peer, "note."+opID, t.ID, netBody{Kind: "note", Task: t.ID, OpID: opID, Text: text})
}

// applyNote keeps another computer's coordinator note for the mirror of its
// task, or tells that computer why not.
func applyNote(tx *sql.Tx, from netLink, b netBody) error {
	t, ok, err := mirrorTask(tx, from)
	if err != nil || !ok {
		return err
	}
	err = savepoint(tx, func() error {
		return addNoteTx(tx, t.ID, "", "net:"+from.broker+":"+from.peer+":"+b.OpID, b.Text, AudienceCoordinator, true)
	})
	if errors.Is(err, ErrRejected) {
		return enqueueNet(tx, from.broker, from.peer, "note-refused."+b.OpID, t.ID, netBody{Kind: "note_refused", Task: from.task, OpID: b.OpID, Reason: refusalText(err)})
	}
	return err
}

// noteRefused marks the coordinator's note undelivered and tells it why.
func noteRefused(tx *sql.Tx, t Task, name string, b netBody) error {
	if _, err := tx.Exec(`UPDATE task_owner_notes SET state='undelivered' WHERE task_id=? AND op_id=? AND state='sent'`, t.ID, b.OpID); err != nil {
		return err
	}
	return insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "note_refused", name+" did not pass your note to the worker: "+clip(b.Reason, 2000), "")
}
