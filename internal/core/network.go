package core

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// Tasks between computers (T-037, D-041). The computer whose coordinator
// starts a task (the origin) keeps a remote task: it has no runs of its own,
// only the state the executing computer reports. The executing computer
// keeps an ordinary task of its own (a mirror) that runs exactly like a local
// one. Messages travel through the broker's mailbox:
//
//	origin → executor   start, answer, cancel
//	executor → origin   accepted, refused, event, answer_refused
//
// Every message is written to net_outbox in the same transaction as the
// change it reports, so a change and its message are never separated. The
// supervisor sends the outbox in order; sending the same message again after
// a lost answer is a delivery retry, deduplicated by the broker and by the
// receiver (net_applied). It never starts, answers or cancels anything a
// second time. A message the broker refuses because the other computer left
// the network fails a start (it certainly never ran there) and blocks a task
// whose answer or cancel was not delivered (what happens there is unknown).
//
// The executing computer runs another computer's task only if its owner
// turned this on (Q-014, off by default), the project exists there with a
// folder, and the agent is set up.
const networkSchema = `
CREATE TABLE IF NOT EXISTS net_outbox (
 seq INTEGER PRIMARY KEY AUTOINCREMENT,
 broker TEXT NOT NULL,
 recipient TEXT NOT NULL,
 op_id TEXT NOT NULL UNIQUE,
 kind TEXT NOT NULL,
 task_id TEXT NOT NULL,
 body TEXT NOT NULL,
 created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS net_applied (
 broker TEXT NOT NULL,
 sender TEXT NOT NULL,
 op_id TEXT NOT NULL,
 applied_at TEXT NOT NULL,
 PRIMARY KEY (broker, sender, op_id)
);
CREATE TABLE IF NOT EXISTS net_peers (
 peer TEXT PRIMARY KEY,
 name TEXT NOT NULL,
 host INTEGER NOT NULL,
 age INTEGER NOT NULL,
 agents TEXT NOT NULL,
 updated_at TEXT NOT NULL
);
`

// Columns that link a task to another computer.
var networkTaskColumns = [][3]string{
	{"tasks", "remote_broker", "TEXT"},
	{"tasks", "remote_peer", "TEXT"},
	{"tasks", "remote_task", "TEXT"},
	{"tasks", "origin_broker", "TEXT"},
	{"tasks", "origin_peer", "TEXT"},
	{"tasks", "origin_task", "TEXT"},
}

// PeerFresh is how old, in seconds, a computer's last report may be for it
// to count as online.
const PeerFresh = 45

// NetAgent is one agent of another computer, as it reported it.
type NetAgent struct {
	Agent   string      `json:"agent"`
	Running int         `json:"running"`
	Waiting int         `json:"waiting_owner"`
	Models  []string    `json:"models,omitempty"`
	Limit   *AgentLimit `json:"limit,omitempty"`
}

// NetPeer is another computer of this computer's network.
type NetPeer struct {
	ID     string
	Name   string
	Host   bool
	Age    int // seconds since its last report; -1 if it never reported
	Agents []NetAgent
	// Ready is what the computer reported about taking tasks (T-070); nil
	// when it did not (an older version).
	Ready *NetReady
	// Usage is what the tasks that ran on it spent, by day and agent
	// (T-076); nil when it did not report it (an older version).
	Usage []UsageDayAgent
	// Reconnecting: the computer was in the network a moment ago and has
	// not reported since this computer or the server restarted (T-092). A
	// task for it waits in the server's mailbox until it is back.
	Reconnecting bool
}

// reconnectGrace is how long a computer that left the server's list stays
// listed as reconnecting.
const reconnectGrace = 15 * time.Minute

// NetReady says whether a computer runs tasks from other computers and for
// which projects it has a folder.
type NetReady struct {
	Accept   bool     `json:"accept"`
	Projects []string `json:"projects"`
}

// NetOutgoing is one message waiting to be sent.
type NetOutgoing struct {
	Seq  int64
	To   string
	OpID string
	Body []byte
}

// NetIncoming is one message from another computer.
type NetIncoming struct {
	Seq  int64
	From string
	OpID string
	Body []byte
}

// netLink is the other side of a task.
type netLink struct{ broker, peer, task string }

type taskLinks struct {
	remote *netLink // this computer started it; it runs on remote.peer
	origin *netLink // another computer started it; it runs here
}

func taskLink(tx *sql.Tx, task string) (taskLinks, error) {
	var l taskLinks
	var rb, rp, rt, ob, op, ot sql.NullString
	err := tx.QueryRow(`SELECT remote_broker,remote_peer,remote_task,origin_broker,origin_peer,origin_task FROM tasks WHERE id=?`, task).Scan(&rb, &rp, &rt, &ob, &op, &ot)
	if errors.Is(err, sql.ErrNoRows) {
		return l, fmt.Errorf("%w: unknown task", ErrRejected)
	}
	if err != nil {
		return l, err
	}
	if rp.Valid {
		l.remote = &netLink{rb.String, rp.String, rt.String}
	}
	if op.Valid {
		l.origin = &netLink{ob.String, op.String, ot.String}
	}
	return l, nil
}

// netBody is the content of a message. Task is always the origin's task ID.
type netBody struct {
	Kind     string `json:"kind"`
	Task     string `json:"task"`
	Project  string `json:"project,omitempty"`
	Agent    string `json:"agent,omitempty"`
	Title    string `json:"title,omitempty"`
	Text     string `json:"text,omitempty"`
	Model    string `json:"model,omitempty"`
	Folder   string `json:"folder,omitempty"`
	Mirror   string `json:"mirror,omitempty"`
	Reason   string `json:"reason,omitempty"`
	State    string `json:"state,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Audience string `json:"audience,omitempty"`
	Event    string `json:"event,omitempty"`
	Question string `json:"question,omitempty"`
	OpID     string `json:"op_id,omitempty"`
	By       string `json:"by,omitempty"`
	Failure  string `json:"failure_reason,omitempty"`
	Reset    string `json:"failure_reset,omitempty"`
	// Rules are the starting computer's worker rules (T-050). A computer
	// before preview.38 ignores them.
	Rules string `json:"rules,omitempty"`
	// Usage is the running computer's total for the task (T-058). A
	// computer before it ignores the field.
	Usage *TaskUsage `json:"usage,omitempty"`
	// Lock is the task's resource lock (T-072). A computer before it
	// ignores it.
	Lock string `json:"lock,omitempty"`
	// Lines is how many log lines a log request asks for (T-057).
	Lines int `json:"lines,omitempty"`
}

// netOpID is the broker's rule for an operation id. An id built from
// another computer's task or question id may break it; such an id is
// replaced by a stable hash so one message can never stop the outbox.
var netOpID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._-]{0,159}$`)

// netRef is what a task or question id from another computer may look like.
var netRef = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9:._-]{0,79}$`)

// remoteEventKinds are the task events another computer may forward; any
// other kind is ignored (audit 2026-11-09).
var remoteEventKinds = map[string]bool{"report": true, "question": true, "failed": true, "blocked": true, "interrupted": true, "cancelled": true, "answered": true, "continued": true, "started": true, "accepted": true}

func safeNetOp(opID string) string {
	if netOpID.MatchString(opID) {
		return opID
	}
	sum := sha256.Sum256([]byte(opID))
	return "h." + hex.EncodeToString(sum[:20])
}

func enqueueNet(tx *sql.Tx, broker, to, opID, task string, b netBody) error {
	opID = safeNetOp(opID)
	raw, err := json.Marshal(b)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO net_outbox(broker,recipient,op_id,kind,task_id,body,created_at) VALUES (?,?,?,?,?,?,?) ON CONFLICT(op_id) DO NOTHING`,
		broker, to, opID, b.Kind, task, string(raw), now())
	return err
}

func metaValue(tx *sql.Tx, key string) (string, error) {
	var v string
	err := tx.QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func setMeta(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec(`INSERT INTO meta(key,value) VALUES (?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// ---- the network and its computers ----

// SetNetwork records the current network and its other computers, so the
// coordinator (a separate process) can list them and start tasks there. An
// empty broker means this computer is in no network.
func (s *Store) SetNetwork(broker, self string, peers []NetPeer) error {
	return s.tx(func(tx *sql.Tx) error {
		// Shared task lists belong to one network (T-047).
		var was string
		if err := tx.QueryRow(`SELECT COALESCE((SELECT value FROM meta WHERE key='net_broker'),'')`).Scan(&was); err != nil {
			return err
		}
		if was != broker || broker == "" {
			if _, err := tx.Exec(`DELETE FROM net_tasks`); err != nil {
				return err
			}
		}
		if err := setMeta(tx, "net_broker", broker); err != nil {
			return err
		}
		if err := setMeta(tx, "net_self", self); err != nil {
			return err
		}
		// T-092: a computer of the same network that is missing or has not
		// reported since a restart stays listed as reconnecting for a while,
		// with what it reported before.
		type oldPeer struct {
			p     NetPeer
			since string
		}
		old := map[string]oldPeer{}
		if was == broker && broker != "" {
			list, err := netPeers(tx)
			if err != nil {
				return err
			}
			for _, p := range list {
				var since string
				if err := tx.QueryRow(`SELECT missing_since FROM net_peers WHERE peer=?`, p.ID).Scan(&since); err != nil {
					return err
				}
				old[p.ID] = oldPeer{p, since}
			}
		}
		if _, err := tx.Exec(`DELETE FROM net_peers`); err != nil {
			return err
		}
		at := now()
		cutoff := time.Now().UTC().Add(-reconnectGrace).Format(time.RFC3339Nano)
		listed := map[string]bool{}
		for i := range peers {
			listed[peers[i].ID] = true
		}
		missing := map[string]string{}
		for id, o := range old {
			if listed[id] {
				continue
			}
			if o.since != "" && o.since < cutoff {
				continue
			}
			p := o.p
			p.Reconnecting, p.Age = true, -1
			peers = append(peers, p)
			if o.since == "" {
				missing[id] = at
			} else {
				missing[id] = o.since
			}
		}
		for i := range peers {
			p := &peers[i]
			o, known := old[p.ID]
			if listed[p.ID] && p.Age < 0 && len(p.Agents) == 0 && known && len(o.p.Agents) > 0 {
				if o.since != "" && o.since < cutoff {
					continue
				}
				p.Agents, p.Reconnecting = o.p.Agents, true
				if p.Ready == nil {
					p.Ready = o.p.Ready
				}
				if p.Usage == nil {
					p.Usage = o.p.Usage
				}
				missing[p.ID] = o.since
				if o.since == "" {
					missing[p.ID] = at
				}
			}
		}
		for _, p := range peers {
			if p.Agents == nil {
				p.Agents = []NetAgent{}
			}
			raw, _ := json.Marshal(p.Agents)
			ready := ""
			if p.Ready != nil {
				r, _ := json.Marshal(p.Ready)
				ready = string(r)
			}
			usage := ""
			if p.Usage != nil {
				u, _ := json.Marshal(p.Usage)
				usage = string(u)
			}
			if _, err := tx.Exec(`INSERT INTO net_peers(peer,name,host,age,agents,ready,usage,updated_at,missing_since) VALUES (?,?,?,?,?,?,?,?,?)`, p.ID, p.Name, boolInt(p.Host), p.Age, string(raw), ready, usage, at, missing[p.ID]); err != nil {
				return err
			}
		}
		return nil
	})
}

// NetPeers lists the other computers of the network, with ages counted to
// now.
func (s *Store) NetPeers() ([]NetPeer, error) {
	var out []NetPeer
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		out, err = netPeers(tx)
		return err
	})
	return out, err
}

func netPeers(tx *sql.Tx) ([]NetPeer, error) {
	rows, err := tx.Query(`SELECT peer,name,host,age,agents,ready,usage,updated_at,missing_since!='' FROM net_peers ORDER BY host DESC, name, peer`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []NetPeer{}
	for rows.Next() {
		var p NetPeer
		var host int
		var agents, ready, usage, at string
		if err := rows.Scan(&p.ID, &p.Name, &host, &p.Age, &agents, &ready, &usage, &at, &p.Reconnecting); err != nil {
			return nil, err
		}
		p.Host = host != 0
		_ = json.Unmarshal([]byte(agents), &p.Agents)
		if ready != "" {
			var r NetReady
			if json.Unmarshal([]byte(ready), &r) == nil {
				p.Ready = &r
			}
		}
		if usage != "" {
			if json.Unmarshal([]byte(usage), &p.Usage) != nil || p.Usage == nil {
				p.Usage = []UsageDayAgent{}
			}
		}
		if when, err := time.Parse(time.RFC3339Nano, at); err == nil && p.Age >= 0 {
			p.Age += int(time.Since(when) / time.Second)
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func netPeer(tx *sql.Tx, id string) (NetPeer, bool, error) {
	peers, err := netPeers(tx)
	if err != nil {
		return NetPeer{}, false, err
	}
	for _, p := range peers {
		if p.ID == id {
			return p, true, nil
		}
	}
	return NetPeer{}, false, nil
}

// peerName names another computer for the window; its ID if it is not in
// the current list.
func peerName(tx *sql.Tx, id string) string {
	var name string
	if err := tx.QueryRow(`SELECT name FROM net_peers WHERE peer=?`, id).Scan(&name); err != nil || name == "" {
		return id
	}
	return name
}

// NetAccept reports whether this computer runs tasks that other computers
// of its network start. It is off until the owner turns it on (Q-014).
func (s *Store) NetAccept() (bool, error) {
	var on bool
	err := s.tx(func(tx *sql.Tx) error {
		v, err := metaValue(tx, "net_accept")
		// On unless the owner turned it off (D-056): every computer of the
		// network joined with the owner's invitation.
		on = v != "0"
		return err
	})
	return on, err
}

// SetNetAccept turns running other computers' tasks on or off. Turning it
// off refuses new tasks only; tasks already accepted continue.
func (s *Store) SetNetAccept(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return s.tx(func(tx *sql.Tx) error { return setMeta(tx, "net_accept", v) })
}

// NetFullAccess reports whether tasks of other computers may run with the
// agent's «Любые команды» (T-094). Off unless the owner turns it on: a task
// from the network runs with the agent's own permission checks.
func (s *Store) NetFullAccess() (bool, error) {
	var on bool
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		on, err = netFullAccess(tx)
		return err
	})
	return on, err
}

func netFullAccess(tx *sql.Tx) (bool, error) {
	v, err := metaValue(tx, "net_full_access")
	return v == "1", err
}

// SetNetFullAccess turns «Любые команды для задач других компьютеров» on or
// off. It applies to new runs; a run already started keeps what it had.
func (s *Store) SetNetFullAccess(on bool) error {
	v := "0"
	if on {
		v = "1"
	}
	return s.tx(func(tx *sql.Tx) error { return setMeta(tx, "net_full_access", v) })
}

// ---- origin side ----

// startRemoteTask records a task for another computer of the network and
// queues its start. Nothing runs here.
func startRemoteTask(tx *sql.Tx, r StartTaskRequest) (string, error) {
	broker, err := metaValue(tx, "net_broker")
	if err != nil {
		return "", err
	}
	if broker == "" {
		return "", fmt.Errorf("%w: this computer is in no network; computer must be this computer's id from list_agents", ErrRejected)
	}
	peer, ok, err := netPeer(tx, r.Computer)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("%w: unknown computer %q; list_agents shows the computers of the network", ErrRejected, r.Computer)
	}
	has := false
	for _, a := range peer.Agents {
		has = has || a.Agent == r.Provider
	}
	if !has {
		return "", fmt.Errorf("%w: agent %s is not set up on %s", ErrRejected, r.Provider, peer.Name)
	}
	id, err := randomID()
	if err != nil {
		return "", err
	}
	id = "task-" + id[:16]
	at := now()
	title := taskTitle(r.Title, r.Text)
	if _, err := tx.Exec(`INSERT INTO tasks(id,project,op_id,computer,provider,cli_path,environment,profile,folder,title,text,state,detail,created_at,updated_at,model,remote_broker,remote_peer,resource) VALUES (?,?,?,?,?,'','','','',?,?,'running',?,?,?,?,?,?,?)`,
		id, r.Project, r.OpID, r.Computer, r.Provider, title, r.Text, "sent to "+peer.Name+"; waiting for it to accept", at, at, r.Model, broker, r.Computer, r.Lock); err != nil {
		return "", err
	}
	if err := insertTaskEvent(tx, r.Project, id, AudienceLog, "started", title, ""); err != nil {
		return "", err
	}
	return id, enqueueNet(tx, broker, r.Computer, "start."+id, id, netBody{Kind: "start", Task: id, Project: r.Project, Agent: r.Provider, Title: title, Text: r.Text, Model: r.Model, Rules: r.Rules, Lock: r.Lock})
}

// answerRemoteTask sends an answer to the computer that runs the task.
func answerRemoteTask(tx *sql.Tx, t Task, remote netLink, question, opID, text, by string) error {
	if _, err := tx.Exec(`UPDATE task_questions SET state='answered',answer=?,answer_op=?,answered_by=? WHERE id=?`, text, opID, by, question); err != nil {
		return err
	}
	if err := setTaskState(tx, t.ID, "running", "answer sent to "+peerName(tx, remote.peer)); err != nil {
		return err
	}
	answeredFor := AudienceLog
	if by == AudienceOwner {
		answeredFor = AudienceOwner
	}
	if err := insertTaskEvent(tx, t.Project, t.ID, answeredFor, "answered", text, question); err != nil {
		return err
	}
	return enqueueNet(tx, remote.broker, remote.peer, "answer."+question, t.ID, netBody{Kind: "answer", Task: t.ID, Question: question, OpID: opID, Text: text, By: by})
}

// cancelRemoteTask asks the computer that runs the task to cancel it. A
// start that has not left this computer yet is withdrawn: the task never
// ran anywhere, so it is cancelled at once.
func cancelRemoteTask(tx *sql.Tx, t Task, remote netLink) (string, error) {
	switch t.State {
	case "running", "waiting_coordinator", "waiting_owner":
	default:
		return t.State, fmt.Errorf("%w: task is %s", ErrRejected, t.State)
	}
	res, err := tx.Exec(`DELETE FROM net_outbox WHERE op_id=?`, "start."+t.ID)
	if err != nil {
		return "", err
	}
	if n, _ := res.RowsAffected(); n == 1 {
		if err := setTaskState(tx, t.ID, "cancelled", "cancelled before it was sent to the other computer"); err != nil {
			return "", err
		}
		return "cancelled", insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "cancelled", "the task was cancelled before it reached the other computer", "")
	}
	if t.CancelRequested {
		return "cancelling", nil
	}
	if _, err := tx.Exec(`UPDATE tasks SET cancel_requested=1,updated_at=?,version=version+1 WHERE id=?`, now(), t.ID); err != nil {
		return "", err
	}
	return "cancelling", enqueueNet(tx, remote.broker, remote.peer, "cancel."+t.ID, t.ID, netBody{Kind: "cancel", Task: t.ID})
}

// ---- executing side ----

// forwardTaskEvent queues an event of a task that another computer started.
func forwardTaskEvent(tx *sql.Tx, o netLink, task, audience, kind, text, question string) error {
	var state, detail string
	var reason, reset sql.NullString
	if err := tx.QueryRow(`SELECT state,detail,failure_reason,failure_reset FROM tasks WHERE id=?`, task).Scan(&state, &detail, &reason, &reset); err != nil {
		return err
	}
	id, err := randomID()
	if err != nil {
		return err
	}
	usage, err := taskUsage(tx, task)
	if err != nil {
		return err
	}
	return enqueueNet(tx, o.broker, o.peer, "event."+id, task, netBody{Kind: "event", Task: o.task, State: state, Detail: clip(detail, 2000), Audience: audience, Event: kind, Text: text, Question: question, Failure: reason.String, Reset: reset.String, Usage: usage})
}

// ---- sending ----

// NetPending lists the messages waiting to be sent in the network, oldest
// first.
func (s *Store) NetPending(broker string, limit int) ([]NetOutgoing, error) {
	rows, err := s.db.Query(`SELECT seq,recipient,op_id,body FROM net_outbox WHERE broker=? ORDER BY seq LIMIT ?`, broker, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NetOutgoing
	for rows.Next() {
		var m NetOutgoing
		var body string
		if err := rows.Scan(&m.Seq, &m.To, &m.OpID, &body); err != nil {
			return nil, err
		}
		m.Body = []byte(body)
		out = append(out, m)
	}
	return out, rows.Err()
}

// NetSent removes a message the broker stored.
func (s *Store) NetSent(seq int64) error {
	_, err := s.db.Exec(`DELETE FROM net_outbox WHERE seq=?`, seq)
	return err
}

// NetUndeliverable handles a message the broker refused because its
// recipient is not in the network any more. A start that never left fails
// its task; an answer or cancel that never arrived blocks it.
func (s *Store) NetUndeliverable(seq int64) error {
	return s.tx(func(tx *sql.Tx) error {
		var kind, task, to string
		err := tx.QueryRow(`SELECT kind,task_id,recipient FROM net_outbox WHERE seq=?`, seq).Scan(&kind, &task, &to)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`DELETE FROM net_outbox WHERE seq=?`, seq); err != nil {
			return err
		}
		if kind == "log_request" || kind == "log" {
			return logUndeliverable(tx, kind, task)
		}
		var project, state string
		if err := tx.QueryRow(`SELECT project,state FROM tasks WHERE id=?`, task).Scan(&project, &state); err != nil {
			return err
		}
		if state == "done" || state == "failed" || state == "cancelled" {
			return nil
		}
		switch kind {
		case "start":
			const why = "the other computer is no longer in the network; the task never reached it"
			if err := setTaskState(tx, task, "failed", why); err != nil {
				return err
			}
			return insertTaskEvent(tx, project, task, AudienceCoordinator, "failed", why, "")
		case "answer", "cancel":
			what := map[string]string{"answer": "the answer", "cancel": "the cancel request"}[kind]
			why := what + " was not delivered: the other computer is no longer in the network; what happens there is unknown and nothing is retried"
			if err := setTaskState(tx, task, "blocked", why); err != nil {
				return err
			}
			return insertTaskEvent(tx, project, task, AudienceCoordinator, "blocked", why, "")
		}
		return nil
	})
}

// ---- receiving ----

// NetCursor is the last message applied from the network's broker.
func (s *Store) NetCursor(broker string) (int64, error) {
	var c int64
	err := s.db.QueryRow(`SELECT CAST(value AS INTEGER) FROM meta WHERE key=?`, "net_cursor:"+broker).Scan(&c)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return c, err
}

// ApplyNet applies one message in one transaction, together with the cursor
// and the record that it was applied, so a message read twice changes
// nothing the second time. A message that cannot be read or does not fit a
// task of this computer is recorded and ignored.
func (s *Store) ApplyNet(broker string, m NetIncoming) error {
	return s.tx(func(tx *sql.Tx) error {
		cursor := m.Seq
		if old, err := metaValue(tx, "net_cursor:"+broker); err != nil {
			return err
		} else if n := atoi64(old); n > cursor {
			cursor = n
		}
		if err := setMeta(tx, "net_cursor:"+broker, fmt.Sprint(cursor)); err != nil {
			return err
		}
		res, err := tx.Exec(`INSERT INTO net_applied(broker,sender,op_id,applied_at) VALUES (?,?,?,?) ON CONFLICT DO NOTHING`, broker, m.From, m.OpID, now())
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return nil
		}
		var b netBody
		// A message whose ids could not travel back is ignored, so a reply
		// to it can never block this computer's outbox.
		if json.Unmarshal(m.Body, &b) != nil || !netRef.MatchString(b.Task) || (b.Question != "" && !netRef.MatchString(b.Question)) {
			return nil
		}
		from := netLink{broker: broker, peer: m.From, task: b.Task}
		switch b.Kind {
		case "start":
			return applyStart(tx, from, b)
		case "answer":
			return applyAnswer(tx, from, m.OpID, b)
		case "cancel":
			return applyCancel(tx, from)
		case "accepted", "refused", "event", "answer_refused", "progress":
			return applyReport(tx, from, b)
		case "log_request":
			return applyLogRequest(tx, from, b)
		case "log":
			return applyLogReply(tx, from, b)
		case "output":
			return applyOutput(tx, from, b)
		}
		return nil
	})
}

func atoi64(s string) int64 {
	var n int64
	fmt.Sscan(s, &n)
	return n
}

// savepoint runs fn so that a refusal undoes only fn's own writes.
func savepoint(tx *sql.Tx, fn func() error) error {
	if _, err := tx.Exec(`SAVEPOINT net`); err != nil {
		return err
	}
	if err := fn(); err != nil {
		if _, rerr := tx.Exec(`ROLLBACK TO net`); rerr != nil {
			return rerr
		}
		_, _ = tx.Exec(`RELEASE net`)
		return err
	}
	_, err := tx.Exec(`RELEASE net`)
	return err
}

func mirrorOp(from netLink) string {
	return "net:" + from.broker + ":" + from.peer + ":" + from.task
}

func mirrorTask(tx *sql.Tx, from netLink) (Task, bool, error) {
	t, err := scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE origin_broker=? AND origin_peer=? AND origin_task=?`, from.broker, from.peer, from.task).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return t, false, nil
	}
	return t, err == nil, err
}

func refusalText(err error) string {
	return strings.TrimPrefix(err.Error(), ErrRejected.Error()+": ")
}

func applyStart(tx *sql.Tx, from netLink, b netBody) error {
	reply := func(kind string, body netBody) error {
		body.Kind, body.Task = kind, from.task
		return enqueueNet(tx, from.broker, from.peer, "reply."+from.peer+"."+from.task, from.task, body)
	}
	if _, ok, err := mirrorTask(tx, from); err != nil || ok {
		return err
	}
	accept, err := metaValue(tx, "net_accept")
	if err != nil {
		return err
	}
	if accept == "0" {
		return reply("refused", netBody{Reason: "this computer does not run tasks of other computers; its owner can turn this on in the Server tab"})
	}
	r := StartTaskRequest{Project: b.Project, OpID: mirrorOp(from), Provider: b.Agent, Title: b.Title, Text: strings.TrimSpace(b.Text), Model: strings.TrimSpace(b.Model), Rules: strings.TrimSpace(b.Rules)}
	lock, lockOK := normalizeLock(b.Lock)
	r.Lock = lock
	var id string
	err = savepoint(tx, func() error {
		switch {
		case !lockOK:
			return fmt.Errorf("%w: invalid lock name", ErrRejected)
		case !TaskProviders[r.Provider]:
			return fmt.Errorf("%w: unsupported agent %q", ErrRejected, r.Provider)
		case r.Text == "" || utf8.RuneCountInString(r.Text) > MaxTaskText:
			return fmt.Errorf("%w: the task text is empty or too long", ErrRejected)
		case !ValidTaskModel(r.Model):
			return fmt.Errorf("%w: invalid model name", ErrRejected)
		case utf8.RuneCountInString(r.Rules) > MaxWorkerRules || strings.ContainsRune(r.Rules, 0):
			return fmt.Errorf("%w: the worker rules are too long", ErrRejected)
		}
		var projects int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM projects WHERE name=? AND removed_at IS NULL`, r.Project).Scan(&projects); err != nil {
			return err
		}
		if projects == 0 {
			return fmt.Errorf("%w: this computer has no project %q", ErrRejected, r.Project)
		}
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		id, err = startLocalTask(tx, r, computer, &from)
		return err
	})
	if errors.Is(err, ErrRejected) {
		return reply("refused", netBody{Reason: refusalText(err)})
	}
	if err != nil {
		return err
	}
	var folder string
	if err := tx.QueryRow(`SELECT folder FROM tasks WHERE id=?`, id).Scan(&folder); err != nil {
		return err
	}
	return reply("accepted", netBody{Mirror: id, Folder: folder})
}

func applyAnswer(tx *sql.Tx, from netLink, opID string, b netBody) error {
	t, ok, err := mirrorTask(tx, from)
	if err != nil || !ok {
		return err
	}
	err = savepoint(tx, func() error {
		return answerTask(tx, t.ID, b.Question, "net:"+from.broker+":"+from.peer+":"+opID, b.Text, b.By, true)
	})
	if errors.Is(err, ErrRejected) {
		return enqueueNet(tx, from.broker, from.peer, "answer-refused."+b.Question, t.ID, netBody{Kind: "answer_refused", Task: from.task, Question: b.Question, Reason: refusalText(err)})
	}
	return err
}

func applyCancel(tx *sql.Tx, from netLink) error {
	t, ok, err := mirrorTask(tx, from)
	if err != nil || !ok {
		return err
	}
	err = savepoint(tx, func() error {
		_, err := cancelTask(tx, t.ID)
		return err
	})
	if errors.Is(err, ErrRejected) {
		// Already finished: its last event has been sent.
		return nil
	}
	return err
}

var taskStates = map[string]bool{"running": true, "waiting_coordinator": true, "waiting_owner": true, "done": true, "failed": true, "interrupted": true, "blocked": true, "cancelled": true}

// applyReport applies a message from the computer that runs one of this
// computer's tasks.
func applyReport(tx *sql.Tx, from netLink, b netBody) error {
	t, err := scanTask(tx.QueryRow(`SELECT `+taskColumns+` FROM tasks WHERE id=? AND remote_broker=? AND remote_peer=?`, from.task, from.broker, from.peer).Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	name := peerName(tx, from.peer)
	switch b.Kind {
	case "progress":
		// A line of the worker's progress, for the window only (D-055).
		if b.Event != "text" && b.Event != "tool" {
			return nil
		}
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM remote_task_items WHERE task_id=?`, t.ID).Scan(&n); err != nil {
			return err
		}
		if n >= maxRunItems {
			return nil
		}
		_, err := tx.Exec(`INSERT INTO remote_task_items(task_id,seq,kind,title,text,created_at) VALUES (?,?,?,?,?,?)`, t.ID, n+1, b.Event, clip(b.Title, 200), clip(b.Text, maxForwardedItemText), now())
		return err
	case "accepted":
		if _, err := tx.Exec(`UPDATE tasks SET folder=?,remote_task=?,detail=?,updated_at=?,version=version+1 WHERE id=?`, clip(b.Folder, 4096), clip(b.Mirror, 64), "accepted by "+name, now(), t.ID); err != nil {
			return err
		}
		return insertTaskEvent(tx, t.Project, t.ID, AudienceLog, "accepted", "accepted by "+name, "")
	case "refused":
		if t.State != "running" {
			return nil
		}
		why := name + " refused the task: " + clip(b.Reason, 2000)
		if err := setTaskState(tx, t.ID, "failed", why); err != nil {
			return err
		}
		return insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "failed", why, "")
	case "answer_refused":
		if t.State != "running" {
			return nil
		}
		why := name + " did not accept the answer: " + clip(b.Reason, 2000) + "; nothing is retried"
		if err := setTaskState(tx, t.ID, "blocked", why); err != nil {
			return err
		}
		return insertTaskEvent(tx, t.Project, t.ID, AudienceCoordinator, "blocked", why, "")
	}
	// An event of the run on the other computer.
	if !taskStates[b.State] || (b.Audience != AudienceCoordinator && b.Audience != AudienceOwner) || !remoteEventKinds[b.Event] {
		return nil
	}
	question := ""
	if b.Event == "question" && strings.HasPrefix(b.Question, "q-") && len(b.Question) <= 64 {
		var exists int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM task_questions WHERE id=?`, b.Question).Scan(&exists); err != nil {
			return err
		}
		if exists == 0 {
			if _, err := tx.Exec(`UPDATE task_questions SET state='superseded' WHERE task_id=? AND state='open'`, t.ID); err != nil {
				return err
			}
			// A question needs a run; this one only stands for the run on the
			// other computer and is never launched.
			var ordinal int
			if err := tx.QueryRow(`SELECT COALESCE(MAX(ordinal),0) FROM task_runs WHERE task_id=?`, t.ID).Scan(&ordinal); err != nil {
				return err
			}
			run, err := randomID()
			if err != nil {
				return err
			}
			receipt, err := randomID()
			if err != nil {
				return err
			}
			run = "run-" + run[:16]
			if _, err := tx.Exec(`INSERT INTO task_runs(id,task_id,ordinal,kind,input,receipt,state,detail,created_at) VALUES (?,?,?,'remote','',?,'remote',?,?)`, run, t.ID, ordinal+1, receipt, "runs on "+name, now()); err != nil {
				return err
			}
			if _, err := tx.Exec(`INSERT INTO task_questions(id,task_id,run_id,audience,text,state,created_at) VALUES (?,?,?,?,?,'open',?)`, b.Question, t.ID, run, b.Audience, clip(b.Text, MaxDispositionText+1000), now()); err != nil {
				return err
			}
			question = b.Question
		}
	}
	if !strings.HasPrefix(b.State, "waiting_") {
		if _, err := tx.Exec(`UPDATE task_questions SET state='superseded' WHERE task_id=? AND state='open'`, t.ID); err != nil {
			return err
		}
	}
	if err := setTaskState(tx, t.ID, b.State, clip(b.Detail, 2000)); err != nil {
		return err
	}
	if b.Failure != "" && failureReasons[b.Failure] {
		reset := sql.NullString{}
		if _, err := time.Parse(time.RFC3339, b.Reset); err == nil && b.Failure == "usage_limit" {
			reset = sql.NullString{String: b.Reset, Valid: true}
		}
		if _, err := tx.Exec(`UPDATE tasks SET failure_reason=?,failure_reset=? WHERE id=?`, b.Failure, nullString(reset), t.ID); err != nil {
			return err
		}
	}
	if err := setTaskUsage(tx, t.ID, b.Usage); err != nil {
		return err
	}
	text := clip(b.Text, MaxDispositionText+1000)
	if b.Event == "report" || b.Event == "failed" {
		var err error
		if text, err = withOutputNote(tx, t.ID, text); err != nil {
			return err
		}
	}
	return insertTaskEvent(tx, t.Project, t.ID, b.Audience, b.Event, text, question)
}
