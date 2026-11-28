package core

import (
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// A follow-up goes to the computer that owns the saved worker conversation.
// netContinueSince is the first build that reads a "continue" message.
const netContinueSince = 60

// netContinueInterruptedSince is the first build that continues an
// interrupted task on request from another computer (T-159).
const netContinueInterruptedSince = 75

var previewBuild = regexp.MustCompile(`^(?:0\.0\.0|[1-9][0-9]*\.[0-9]+\.[0-9]+)-preview\.(\d+)$`)

// peerReadsContinue says whether a computer's reported version reads
// "continue" messages. A version this code cannot read is assumed newer.
func peerReadsContinue(version string) bool {
	return peerBuildAtLeast(version, netContinueSince)
}

// peerBuildAtLeast says whether a reported version is at least build since.
// An empty version is older; one this code cannot read is assumed newer.
func peerBuildAtLeast(version string, since int) bool {
	if version == "" {
		return false
	}
	m := previewBuild.FindStringSubmatch(version)
	if m == nil {
		return true
	}
	n, err := strconv.Atoi(m[1])
	return err == nil && n >= since
}

// A local close while the executor is unreachable does not establish that
// its worker stopped. The next terminal event from that executor settles it.
const remoteCloseUnconfirmedPrefix = "closed on this computer;"

func remoteCloseUnconfirmed(t Task) bool {
	return t.State == "cancelled" && strings.HasPrefix(t.Detail, remoteCloseUnconfirmedPrefix)
}

// continueRemoteTask sends the coordinator's follow-up to the computer that
// ran the task. opID makes it idempotent.
func continueRemoteTask(machine, tx *sql.Tx, task string, remote netLink, opID, text string, sender MessageSender) error {
	key := "net_continue:" + opID
	old, err := metaValue(tx, key)
	if err != nil {
		return err
	}
	if old != "" {
		if old == task+"\n"+text {
			return nil
		}
		return fmt.Errorf("%w: this operation id was used for another follow-up", ErrRejected)
	}
	t, err := scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=?`, task).Scan)
	if err != nil {
		return err
	}
	if t.archiveState != "" {
		return fmt.Errorf("%w: продолжить можно после восстановления задачи из архива", ErrRejected)
	}
	if !canResumeTask(t.State) {
		return fmt.Errorf("%w: this task cannot be continued while it is %s", ErrRejected, t.State)
	}
	if t.State == "cancelled" {
		var accepted bool
		if err := tx.QueryRow(`SELECT COALESCE(remote_task,'')<>'' FROM tasks WHERE id=?`, task).Scan(&accepted); err != nil {
			return err
		}
		if !accepted || remoteCloseUnconfirmed(t) {
			return fmt.Errorf("%w: the executor has not confirmed that this task stopped", ErrRejected)
		}
	}
	broker, err := metaValue(machine, "net_broker")
	if err != nil {
		return err
	}
	peer, found, err := netPeer(machine, remote.peer)
	if err != nil {
		return err
	}
	if broker == "" || broker != remote.broker || !found {
		return fmt.Errorf("%w: the computer that ran this task is no longer in this network; start a new task that names what is already done", ErrRejected)
	}
	if !peerReadsContinue(peer.Version) {
		return fmt.Errorf("%w: %s runs an older Messenger that cannot continue a task; update it, or start a new task that names what is already done", ErrRejected, peer.Name)
	}
	if t.State == "interrupted" && !peerBuildAtLeast(peer.Version, netContinueInterruptedSince) {
		return fmt.Errorf("%w: %s runs an older Messenger that cannot continue an interrupted task; update it, cancel the task, or start a new task that names what is already done", ErrRejected, peer.Name)
	}
	if err := setMeta(tx, key, task+"\n"+text); err != nil {
		return err
	}
	if err := setMeta(tx, "net_continue_prev:"+task, t.State); err != nil {
		return err
	}
	if err := setTaskState(tx, task, "running", "follow-up sent to "+peer.Name+"; the same conversation resumes there"); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE tasks SET finished_at=NULL,failure_reason=NULL,failure_reset=NULL,cancel_requested=0 WHERE id=?`, task); err != nil {
		return err
	}
	if err := insertTaskEvent(tx, t.Project, task, AudienceLog, "continued", text, "", sender); err != nil {
		return err
	}
	return enqueueNet(tx, remote.broker, remote.peer, "continue."+opID, task, netBody{Kind: "continue", Task: task, OpID: opID, Text: text, SenderAgent: sender.Agent, SenderModel: sender.Model})
}

// applyContinue resumes the mirror of another computer's task.
// An interrupted run is closed only when gone confirms its worker is gone
// (D-108); without gone it is refused, as before.
func applyContinue(machine, tx *sql.Tx, from netLink, b netBody, gone WorkerGone) error {
	t, ok, err := mirrorTask(tx, from)
	if err != nil || !ok {
		return err
	}
	err = savepoint(tx, func() error {
		return continueTaskTx(machine, tx, t.ID, "net:"+from.broker+":"+from.peer+":"+b.OpID, b.Text, gone, true, MessageSender{Agent: b.SenderAgent, Model: b.SenderModel})
	})
	if errors.Is(err, ErrRejected) {
		return enqueueNet(tx, from.broker, from.peer, "continue-refused."+b.OpID, t.ID, netBody{Kind: "continue_refused", Task: from.task, Reason: refusalText(err)})
	}
	return err
}

// continueNotRun puts a task back as it was before a follow-up that never
// ran on the other computer, and tells the coordinator why.
func continueNotRun(tx *sql.Tx, t Task, why string) error {
	if t.State != "running" {
		return nil
	}
	prev, err := metaValue(tx, "net_continue_prev:"+t.ID)
	if err != nil {
		return err
	}
	if !canResumeTask(prev) {
		prev = "failed"
	}
	if err := setTaskState(tx, t.ID, prev, why); err != nil {
		return err
	}
	return insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "continue_refused", why, "")
}
