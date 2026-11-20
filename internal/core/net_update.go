package core

import (
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Updating another computer of the network (T-078, D-100, T-155, D-133).
// The owner asks from the window, or the coordinator with update_messenger.
// The request and the replies travel through the mailbox like the log
// requests (T-057). The other computer never interrupts running tasks: when
// work is still live it answers "queued", takes no new tasks, and installs
// as soon as none runs. A Mac refuses (it updates in its own app). It
// replies at once (queued, updating, or refused) and, after its update
// helper finished, with the final result (done or failed). Nothing is
// retried on its own: an unclear result is reported as failed, with the
// reason. The coordinator of the project that asked gets a messenger_update
// event for each answer.

// UpdateWait is how long a request may stay unanswered before the owner may
// ask again.
const UpdateWait = 15 * time.Minute

// Update states.
const (
	UpdateWaiting  = "waiting"
	UpdateQueued   = "queued"
	UpdateUpdating = "updating"
	UpdateDone     = "done"
	UpdateRefused  = "refused"
	UpdateFailed   = "failed"
)

// MessengerUpdateEvent is the coordinator event for an update this
// computer asked (T-155).
const MessengerUpdateEvent = "messenger_update"

// maxUpdateText bounds a reason or a version that came from another
// computer.
const (
	maxUpdateText    = 2000
	maxUpdateVersion = 64
)

var updateReplyStates = map[string]bool{UpdateQueued: true, UpdateUpdating: true, UpdateDone: true, UpdateRefused: true, UpdateFailed: true}

const netUpdatesSchema = `
CREATE TABLE IF NOT EXISTS net_updates (
 peer TEXT PRIMARY KEY,
 id TEXT NOT NULL,
 broker TEXT NOT NULL,
 state TEXT NOT NULL,
 version TEXT NOT NULL,
 text TEXT NOT NULL,
 created_at TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS net_update_requests (
 broker TEXT NOT NULL,
 peer TEXT NOT NULL,
 id TEXT NOT NULL,
 created_at TEXT NOT NULL,
 PRIMARY KEY (broker, peer, id)
);
CREATE TABLE IF NOT EXISTS pending_updates (
 id TEXT PRIMARY KEY,
 broker TEXT NOT NULL,
 peer TEXT NOT NULL,
 project TEXT NOT NULL,
 created_at TEXT NOT NULL
);`

// NetUpdate is the last update this computer asked of another computer.
// Stale: no final answer came within UpdateWait; the owner may ask again.
type NetUpdate struct {
	Computer  string `json:"computer"`
	ID        string `json:"request_id"`
	State     string `json:"state"`
	Version   string `json:"version,omitempty"`
	Text      string `json:"text,omitempty"`
	Stale     bool   `json:"stale,omitempty"`
	UpdatedAt string `json:"updated_at"`
}

// NetUpdateRequest is another computer's request to update this one.
type NetUpdateRequest struct {
	Broker, Peer, ID, Project string
}

// PendingUpdate is an accepted update this computer has not finished yet
// (queued until live work ends, or the helper still running).
type PendingUpdate struct {
	Broker, Peer, ID, Project string
}

// RequestNetUpdate asks another computer of the network to install the
// newest signed update, and returns the request's id. Project is the
// coordinator project that should receive messenger_update events, or
// empty when the owner asked from the window. A second request for the
// same computer is refused while the first one waits, is queued, or is
// installing.
func (s *Store) RequestNetUpdate(computer, project string) (string, error) {
	if project != "" && !ValidProjectName(project) {
		return "", fmt.Errorf("%w: unknown project", ErrRejected)
	}
	var id string
	err := s.tx(func(tx *sql.Tx) error {
		self, err := localComputer(tx)
		if err != nil {
			return err
		}
		if computer == "" || computer == self {
			id, err = queueLocalUpdate(tx, project)
			return err
		}
		broker, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
		}
		if broker == "" {
			return fmt.Errorf("%w: этот компьютер не в сети", ErrRejected)
		}
		p, ok, err := netPeer(tx, computer)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("%w: такого компьютера нет в сети", ErrRejected)
		}
		if p.Age < 0 || p.Age > PeerFresh || p.Reconnecting {
			return fmt.Errorf("%w: компьютер «%s» не на связи; обновите его, когда он вернётся", ErrRejected, p.Name)
		}
		// A computer that reports no version is older than remote updates
		// and would never answer.
		if p.Version == "" {
			return fmt.Errorf("%w: на компьютере «%s» программа старше обновления по сети; обновите её там вручную", ErrRejected, p.Name)
		}
		if p.UpdatePending {
			return fmt.Errorf("%w: компьютер «%s» уже ждёт обновления и не берёт новых задач", ErrRejected, p.Name)
		}
		var state, created string
		err = tx.QueryRow(`SELECT state,created_at FROM net_updates WHERE peer=? AND broker=?`, computer, broker).Scan(&state, &created)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && updateInFlight(state, created) {
			return fmt.Errorf("%w: обновление этого компьютера уже запрошено; дождитесь ответа", ErrRejected)
		}
		r, err := randomID()
		if err != nil {
			return err
		}
		id = "upd-" + r[:16]
		at := now()
		if _, err := tx.Exec(`INSERT INTO net_updates(peer,id,broker,state,version,text,project,created_at,updated_at) VALUES (?,?,?,?,'','',?,?,?)
 ON CONFLICT(peer) DO UPDATE SET id=excluded.id,broker=excluded.broker,state=excluded.state,version='',text='',project=excluded.project,created_at=excluded.created_at,updated_at=excluded.updated_at`,
			computer, id, broker, UpdateWaiting, project, at, at); err != nil {
			return err
		}
		return enqueueNet(tx, broker, computer, "update."+id, id, netBody{Kind: "update_request", Task: id, Project: project})
	})
	return id, err
}

func updateInFlight(state, created string) bool {
	switch state {
	case UpdateQueued, UpdateUpdating:
		return true
	case UpdateWaiting:
		return !updateStale(created)
	}
	return false
}

func updateStale(created string) bool {
	when, err := time.Parse(time.RFC3339Nano, created)
	return err != nil || time.Since(when) > UpdateWait
}

func queueLocalUpdate(tx *sql.Tx, project string) (string, error) {
	var existing string
	err := tx.QueryRow(`SELECT id FROM pending_updates LIMIT 1`).Scan(&existing)
	if err == nil {
		return "", fmt.Errorf("%w: обновление этого компьютера уже запрошено; дождитесь результата", ErrRejected)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	r, err := randomID()
	if err != nil {
		return "", err
	}
	id := "upd-" + r[:16]
	if _, err := tx.Exec(`INSERT INTO pending_updates(id,broker,peer,project,created_at) VALUES (?,?,?,?,?)`, id, "", "", project, now()); err != nil {
		return "", err
	}
	self, err := localComputer(tx)
	if err != nil {
		return "", err
	}
	name, err := computerName(tx)
	if err != nil {
		return "", err
	}
	n, err := liveWork(tx)
	if err != nil {
		return "", err
	}
	text := ""
	if n > 0 {
		tasks, _ := liveWorkTasks(tx, 3)
		text = updateQueuedText(n, tasks)
	}
	if err := writeLocalUpdate(tx, self, id, UpdateQueued, "", text, project); err != nil {
		return "", err
	}
	if err := insertCoordinatorNotice(tx, project, MessengerUpdateEvent, messengerUpdateEvent(name, UpdateQueued, "", text)); err != nil {
		return "", err
	}
	return id, nil
}

func writeLocalUpdate(tx *sql.Tx, peer, id, state, version, text, project string) error {
	at := now()
	_, err := tx.Exec(`INSERT INTO net_updates(peer,id,broker,state,version,text,project,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?)
 ON CONFLICT(peer) DO UPDATE SET id=excluded.id,broker='',state=excluded.state,version=excluded.version,text=excluded.text,project=excluded.project,created_at=excluded.created_at,updated_at=excluded.updated_at`,
		peer, id, "", state, clip(version, maxUpdateVersion), clip(text, maxUpdateText), project, at, at)
	return err
}

func touchLocalUpdate(tx *sql.Tx, id, state, version, text string) error {
	if id == "" {
		_, err := tx.Exec(`UPDATE net_updates SET state=?,version=?,text=?,updated_at=? WHERE broker=''`,
			state, clip(version, maxUpdateVersion), clip(text, maxUpdateText), now())
		return err
	}
	_, err := tx.Exec(`UPDATE net_updates SET state=?,version=?,text=?,updated_at=? WHERE id=? AND broker=''`,
		state, clip(version, maxUpdateVersion), clip(text, maxUpdateText), now(), id)
	return err
}

// LocalUpdate is this computer's last update of itself (T-155), for the
// window notice. Empty when this computer has never been asked.
func (s *Store) LocalUpdate() (NetUpdate, bool, error) {
	var u NetUpdate
	var created string
	err := s.db.QueryRow(`SELECT peer,id,state,version,text,created_at,updated_at FROM net_updates WHERE broker='' ORDER BY updated_at DESC LIMIT 1`).Scan(&u.Computer, &u.ID, &u.State, &u.Version, &u.Text, &created, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return NetUpdate{}, false, nil
	}
	if err != nil {
		return NetUpdate{}, false, err
	}
	u.Stale = u.State == UpdateWaiting && updateStale(created)
	return u, true, nil
}

// NetUpdates lists the last update request to each computer of the current
// network.
func (s *Store) NetUpdates() ([]NetUpdate, error) {
	var updates []NetUpdate
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		updates, err = netUpdates(tx)
		return err
	})
	return updates, err
}

func netUpdates(tx *sql.Tx) ([]NetUpdate, error) {
	out := []NetUpdate{}
	rows, err := tx.Query(`SELECT peer,id,state,version,text,created_at,updated_at FROM net_updates
 WHERE broker=(SELECT value FROM meta WHERE key='net_broker') ORDER BY peer`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var u NetUpdate
		var created string
		if err := rows.Scan(&u.Computer, &u.ID, &u.State, &u.Version, &u.Text, &created, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.Stale = u.State == UpdateWaiting && updateStale(created)
		out = append(out, u)
	}
	return out, rows.Err()
}

// updateResultFresh bounds how long an update result outweighs old presence.
const updateResultFresh = 10 * time.Minute

// PeerUpdateState reconciles a computer's version and pending flag with the
// last result for that computer. The same result drives notices and the roster.
func (u NetUpdate) PeerUpdateState(version string, pending bool, presenceAge int, now time.Time) (string, bool) {
	if u.State != UpdateDone && u.State != UpdateFailed && u.State != UpdateRefused {
		return version, pending
	}
	at, err := time.Parse(time.RFC3339Nano, u.UpdatedAt)
	if err != nil || now.Sub(at) >= updateResultFresh {
		return version, pending
	}
	before := presenceAge < 0 || now.Add(-time.Duration(presenceAge)*time.Second).Before(at)
	if u.State == UpdateDone {
		newBuild, oldBuild := updateBuild(u.Version), updateBuild(version)
		switch {
		case newBuild != 0 && oldBuild > newBuild:
			return version, pending
		case newBuild != 0 && (oldBuild < newBuild && oldBuild != 0 || version == "" || before && oldBuild == 0):
			return u.Version, false
		}
	}
	return version, pending && !before
}

func updateBuild(version string) int {
	m := previewBuild.FindStringSubmatch(version)
	if m == nil && strings.HasPrefix(version, "0.") {
		m = previewBuild.FindStringSubmatch("0.0.0-preview." + strings.TrimPrefix(version, "0."))
	}
	if m == nil {
		return 0
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0
	}
	return n
}

// NetUpdateRequests lists the other computers' requests to update this
// computer, oldest first.
func (s *Store) NetUpdateRequests(broker string) ([]NetUpdateRequest, error) {
	rows, err := s.db.Query(`SELECT broker,peer,id,COALESCE(project,'') FROM net_update_requests WHERE broker=? ORDER BY created_at, id LIMIT 20`, broker)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NetUpdateRequest
	for rows.Next() {
		var r NetUpdateRequest
		if err := rows.Scan(&r.Broker, &r.Peer, &r.ID, &r.Project); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// PendingUpdate returns this computer's unfinished update, if any.
func (s *Store) PendingUpdate() (PendingUpdate, bool, error) {
	var p PendingUpdate
	err := s.db.QueryRow(`SELECT id,broker,peer,project FROM pending_updates ORDER BY created_at, id LIMIT 1`).Scan(&p.ID, &p.Broker, &p.Peer, &p.Project)
	if errors.Is(err, sql.ErrNoRows) {
		return PendingUpdate{}, false, nil
	}
	return p, err == nil, err
}

// HasPendingUpdate reports whether this computer is waiting to install an
// update and therefore takes no new tasks.
func (s *Store) HasPendingUpdate() (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pending_updates`).Scan(&n)
	return n > 0, err
}

func hasPendingUpdate(tx *sql.Tx) (bool, error) {
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM pending_updates`).Scan(&n)
	return n > 0, err
}

// RememberPendingUpdate keeps an accepted request until live work ends and
// the helper has reported. A second request is refused.
func (s *Store) RememberPendingUpdate(r NetUpdateRequest) error {
	if r.ID == "" || !netRef.MatchString(r.ID) {
		return fmt.Errorf("%w: invalid update request", ErrRejected)
	}
	project := r.Project
	if project != "" && !ValidProjectName(project) {
		project = ""
	}
	return s.tx(func(tx *sql.Tx) error {
		var existing string
		err := tx.QueryRow(`SELECT id FROM pending_updates LIMIT 1`).Scan(&existing)
		if err == nil {
			if existing == r.ID {
				return nil
			}
			return fmt.Errorf("%w: обновление на том компьютере уже идёт; дождитесь его результата", ErrRejected)
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		_, err = tx.Exec(`INSERT INTO pending_updates(id,broker,peer,project,created_at) VALUES (?,?,?,?,?)`, r.ID, r.Broker, r.Peer, project, now())
		return err
	})
}

func (s *Store) ClearPendingUpdate(id string) error {
	_, err := s.db.Exec(`DELETE FROM pending_updates WHERE id=?`, id)
	return err
}

// FinishPendingUpdate forgets a local queued update and tells the
// coordinator. Used when this computer asked itself (T-155).
func (s *Store) FinishPendingUpdate(id, state, version, text string) error {
	if state == UpdateUpdating || state == UpdateQueued || !updateReplyStates[state] {
		return fmt.Errorf("%w: unknown final update state %q", ErrRejected, state)
	}
	return s.tx(func(tx *sql.Tx) error {
		var project string
		err := tx.QueryRow(`SELECT project FROM pending_updates WHERE id=?`, id).Scan(&project)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM pending_updates WHERE id=?`, id); err != nil {
			return err
		}
		if err := touchLocalUpdate(tx, id, state, version, text); err != nil {
			return err
		}
		name, err := computerName(tx)
		if err != nil {
			return err
		}
		return insertCoordinatorNotice(tx, project, MessengerUpdateEvent, messengerUpdateEvent(name, state, version, text))
	})
}

// RecordMessengerUpdate writes a messenger_update event for a project on
// this computer, without changing the pending row.
func (s *Store) RecordMessengerUpdate(project, state, version, text string) error {
	return s.tx(func(tx *sql.Tx) error {
		if err := touchLocalUpdate(tx, "", state, version, text); err != nil {
			return err
		}
		name, err := computerName(tx)
		if err != nil {
			return err
		}
		return insertCoordinatorNotice(tx, project, MessengerUpdateEvent, messengerUpdateEvent(name, state, version, text))
	})
}

// ReplyNetUpdate gives the first answer to a request (updating, or refused
// or failed with the reason) and forgets the request.
func (s *Store) ReplyNetUpdate(r NetUpdateRequest, state, version, text string) error {
	if !updateReplyStates[state] {
		return fmt.Errorf("unknown update state %q", state)
	}
	return s.tx(func(tx *sql.Tx) error {
		res, err := tx.Exec(`DELETE FROM net_update_requests WHERE broker=? AND peer=? AND id=?`, r.Broker, r.Peer, r.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		return enqueueNet(tx, r.Broker, r.Peer, "update-reply."+r.ID, r.ID, netBody{Kind: "update", Task: r.ID, State: state, Detail: clip(version, maxUpdateVersion), Text: clip(text, maxUpdateText)})
	})
}

// SendUpdateReply sends an update answer that is not the first one, such as
// "updating" after the computer had already answered "queued".
func (s *Store) SendUpdateReply(r NetUpdateRequest, state, version, text string) error {
	if r.Broker == "" || !updateReplyStates[state] {
		return nil
	}
	if !netRef.MatchString(r.ID) || r.Peer == "" {
		return fmt.Errorf("%w: invalid update request", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		return enqueueNet(tx, r.Broker, r.Peer, "update-progress."+r.ID+"."+state, r.ID, netBody{Kind: "update", Task: r.ID, State: state, Detail: clip(version, maxUpdateVersion), Text: clip(text, maxUpdateText)})
	})
}

// FinishNetUpdate sends the final result of an update this computer
// started for another computer: done, refused or failed. A result for
// another network than the current one is dropped: its recipient cannot be
// reached any more.
func (s *Store) FinishNetUpdate(broker, peer, id, state, version, text string) error {
	if state == UpdateUpdating || state == UpdateQueued || !updateReplyStates[state] {
		return fmt.Errorf("%w: unknown final update state %q", ErrRejected, state)
	}
	if !netRef.MatchString(id) || peer == "" || len(peer) > 80 {
		return fmt.Errorf("%w: invalid update request", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		current, err := metaValue(tx, "net_broker")
		if err != nil {
			return err
		}
		if current == "" || current != broker {
			return nil
		}
		return enqueueNet(tx, broker, peer, "update-final."+id, id, netBody{Kind: "update", Task: id, State: state, Detail: clip(version, maxUpdateVersion), Text: clip(text, maxUpdateText)})
	})
}

// ActiveWork counts the tasks that are not finished, or have a run that is
// not closed, in every project, stuck ones included. LiveWork counts only
// tasks that still run or wait, which an update waits for (T-155): blocked
// and interrupted tasks survive a restart and do not hold it.
func (s *Store) ActiveWorkTasks(limit int) ([]ActiveTask, error) {
	return listWorkTasks(s, limit, activeWorkWhere)
}

func (s *Store) LiveWorkTasks(limit int) ([]ActiveTask, error) {
	return listWorkTasks(s, limit, liveWorkWhere)
}

const (
	activeWorkWhere = `t.state IN ('running','waiting_coordinator','waiting_owner','blocked','interrupted')
 OR EXISTS (SELECT 1 FROM task_runs r WHERE r.task_id=t.id AND r.state IN ('intent','launching','running','blocked','interrupted'))`
	liveWorkWhere = `t.state IN ('running','waiting_coordinator','waiting_owner')
 OR EXISTS (SELECT 1 FROM task_runs r WHERE r.task_id=t.id AND r.state IN ('intent','launching','running'))`
)

func listWorkTasks(s *Store, limit int, where string) ([]ActiveTask, error) {
	rows, err := s.db.Query(`SELECT t.title,t.state FROM tasks t WHERE `+where+` ORDER BY t.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActiveTask
	for rows.Next() {
		var a ActiveTask
		if err := rows.Scan(&a.Title, &a.State); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ActiveTask is a task that keeps a computer from updating.
type ActiveTask struct{ Title, State string }

func (s *Store) ActiveWork() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM tasks t WHERE ` + activeWorkWhere).Scan(&n)
	return n, err
}

func (s *Store) LiveWork() (int, error) {
	var n int
	err := s.tx(func(tx *sql.Tx) (err error) {
		n, err = liveWork(tx)
		return
	})
	return n, err
}

func liveWork(tx *sql.Tx) (int, error) {
	var n int
	err := tx.QueryRow(`SELECT COUNT(*) FROM tasks t WHERE ` + liveWorkWhere).Scan(&n)
	return n, err
}

func liveWorkTasks(tx *sql.Tx, limit int) ([]ActiveTask, error) {
	rows, err := tx.Query(`SELECT t.title,t.state FROM tasks t WHERE `+liveWorkWhere+` ORDER BY t.created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ActiveTask
	for rows.Next() {
		var a ActiveTask
		if err := rows.Scan(&a.Title, &a.State); err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func updateQueuedText(n int, tasks []ActiveTask) string {
	text := fmt.Sprintf("На компьютере идут задачи (%d). Программа обновится, когда они закончатся; до тех пор он не берёт новых задач.", n)
	if len(tasks) == 0 {
		return text
	}
	names := make([]string, 0, len(tasks))
	for _, t := range tasks {
		title := []rune(t.Title)
		if len(title) > 60 {
			title = append(title[:59], '…')
		}
		names = append(names, "«"+string(title)+"»")
	}
	text += " " + joinNames(names)
	if n > len(tasks) {
		text += fmt.Sprintf(" и ещё %d", n-len(tasks))
	}
	text += "."
	return text
}

func joinNames(names []string) string {
	if len(names) == 0 {
		return ""
	}
	if len(names) == 1 {
		return names[0]
	}
	return names[0] + ", " + joinNames(names[1:])
}

func messengerUpdateEvent(name, state, version, text string) string {
	switch state {
	case UpdateQueued:
		msg := "Messenger update of computer «" + name + "» is waiting until its running tasks end. It takes no new tasks."
		if text != "" {
			msg += " " + text
		}
		return msg + " Nothing is retried. The result comes as a later messenger_update event."
	case UpdateUpdating:
		return "Messenger update of computer «" + name + "» has started. Nothing is retried."
	case UpdateDone:
		if version != "" && text == "" {
			return "Messenger on computer «" + name + "» is updated to " + version + "."
		}
		if text != "" {
			return "Messenger update of computer «" + name + "»: " + text
		}
		return "Messenger on computer «" + name + "» is updated."
	case UpdateRefused:
		return "Messenger update of computer «" + name + "» was refused: " + text + " Nothing is retried."
	case UpdateFailed:
		return "Messenger update of computer «" + name + "» failed: " + text + " Nothing is retried."
	}
	return "Messenger update of computer «" + name + "»: " + state
}

func insertCoordinatorNotice(tx *sql.Tx, project, kind, text string) error {
	if project == "" || !ValidProjectName(project) {
		return nil
	}
	_, err := tx.Exec(`INSERT INTO task_events(project,task_id,audience,kind,text,created_at) VALUES (?,'',?,?,?,?)`,
		project, AudienceCoordinator, kind, text, now())
	return err
}

// applyUpdateRequest records another computer's request; the client
// answers it.
func applyUpdateRequest(tx *sql.Tx, from netLink, b netBody) error {
	project := ""
	if ValidProjectName(b.Project) {
		project = b.Project
	}
	_, err := tx.Exec(`INSERT INTO net_update_requests(broker,peer,id,project,created_at) VALUES (?,?,?,?,?) ON CONFLICT DO NOTHING`,
		from.broker, from.peer, from.task, project, now())
	return err
}

// applyUpdateReply records an answer to this computer's request, only from
// the computer that was asked and only for its current request. A final
// answer is not replaced by a later one. The coordinator of the project
// that asked gets a messenger_update event.
func applyUpdateReply(tx *sql.Tx, from netLink, b netBody) error {
	if !updateReplyStates[b.State] {
		return nil
	}
	cond := `state IN ('waiting','queued','updating')`
	switch b.State {
	case UpdateUpdating:
		cond = `state IN ('waiting','queued')`
	case UpdateQueued:
		cond = `state='waiting'`
	}
	res, err := tx.Exec(`UPDATE net_updates SET state=?,version=?,text=?,updated_at=? WHERE id=? AND broker=? AND peer=? AND `+cond,
		b.State, clip(b.Detail, maxUpdateVersion), clip(b.Text, maxUpdateText), now(), from.task, from.broker, from.peer)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil
	}
	var project, name string
	if err := tx.QueryRow(`SELECT COALESCE(project,''),peer FROM net_updates WHERE id=? AND broker=? AND peer=?`, from.task, from.broker, from.peer).Scan(&project, &name); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if p, ok, err := netPeer(tx, from.peer); err == nil && ok {
		name = p.Name
	}
	return insertCoordinatorNotice(tx, project, MessengerUpdateEvent, messengerUpdateEvent(name, b.State, clip(b.Detail, maxUpdateVersion), clip(b.Text, maxUpdateText)))
}

// updateUndeliverable handles an update message the broker refused because
// its recipient left the network: the request fails; a reply is dropped.
func updateUndeliverable(tx *sql.Tx, kind, id string) error {
	if kind != "update_request" {
		return nil
	}
	var project, peer string
	_ = tx.QueryRow(`SELECT COALESCE(project,''),peer FROM net_updates WHERE id=?`, id).Scan(&project, &peer)
	if _, err := tx.Exec(`UPDATE net_updates SET state='failed',text=?,updated_at=? WHERE id=? AND state IN ('waiting','queued','updating')`,
		"компьютер больше не в сети", now(), id); err != nil {
		return err
	}
	name := peer
	if p, ok, err := netPeer(tx, peer); err == nil && ok {
		name = p.Name
	}
	return insertCoordinatorNotice(tx, project, MessengerUpdateEvent, messengerUpdateEvent(name, UpdateFailed, "", "the computer is no longer in the network"))
}
