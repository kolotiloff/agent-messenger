package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// Read models for the window (T-029). They only read; nothing here launches
// or changes a task.

const taskRunItemsSchema = `
CREATE TABLE IF NOT EXISTS task_run_items (
 run_id TEXT NOT NULL REFERENCES task_runs(id),
 seq INTEGER NOT NULL,
 kind TEXT NOT NULL,
 title TEXT NOT NULL,
 text TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 PRIMARY KEY (run_id, seq)
);
CREATE TABLE IF NOT EXISTS remote_task_items (
 task_id TEXT NOT NULL REFERENCES tasks(id),
 seq INTEGER NOT NULL,
 kind TEXT NOT NULL,
 title TEXT NOT NULL,
 text TEXT NOT NULL,
 description TEXT NOT NULL DEFAULT '',
 created_at TEXT NOT NULL,
 PRIMARY KEY (task_id, seq)
);
`

// Bounds on what the window keeps of a worker's event stream.
const (
	maxRunItems    = 400
	maxRunItemText = 4000
	// What goes to the computer that started a task (D-055).
	maxForwardedItems    = 200
	maxForwardedItemText = 1000
)

func clip(text string, n int) string {
	if utf8.RuneCountInString(text) <= n {
		return text
	}
	return string([]rune(text)[:n-1]) + "…"
}

func isCommandActivity(kind string) bool {
	return kind == "command_start" || kind == "command_end"
}

// AppendTaskRunItem stores one sanitized item of a worker's event stream
// (its text or a tool action) for the window. Items past the bound are
// dropped; this never affects verification.
func (s *Store) AppendTaskRunItem(run, kind, title, text, description string) error {
	if s.split != nil {
		project, err := s.storeForRun(run)
		if err != nil {
			return err
		}
		return project.AppendTaskRunItem(run, kind, title, text, description)
	}
	activity := isCommandActivity(kind)
	if kind != "text" && kind != "tool" && !activity {
		return fmt.Errorf("%w: unknown item kind", ErrRejected)
	}
	description = clip(strings.Join(strings.Fields(description), " "), 200)
	return s.tx(func(tx *sql.Tx) error {
		var seq, n int
		if err := tx.QueryRow(`SELECT COALESCE(MAX(seq),0), COUNT(CASE WHEN kind IN ('text','tool') THEN 1 END) FROM task_run_items WHERE run_id=?`, run).Scan(&seq, &n); err != nil {
			return err
		}
		// Timing records have their own allowance and never displace visible items.
		if (!activity && n >= maxRunItems) || (activity && seq-n >= 2*maxRunItems) {
			return nil
		}
		if _, err := tx.Exec(`INSERT INTO task_run_items(run_id,seq,kind,title,text,description,created_at) VALUES (?,?,?,?,?,?,?)`, run, seq+1, kind, clip(title, 200), clip(text, maxRunItemText), description, now()); err != nil {
			return err
		}
		if (!activity && n >= maxForwardedItems) || (activity && seq-n >= 2*maxForwardedItems) {
			return nil
		}
		// A task another computer started shows its progress there too
		// (D-055), in the same transaction.
		var task string
		if err := tx.QueryRow(`SELECT task_id FROM task_runs WHERE id=?`, run).Scan(&task); err != nil {
			return err
		}
		link, err := taskLink(tx, task)
		if err != nil || link.origin == nil {
			return err
		}
		return enqueueNet(tx, link.origin.broker, link.origin.peer, fmt.Sprintf("progress.%s.%d", run, seq+1), task,
			netBody{Kind: "progress", Task: link.origin.task, Event: kind, Title: clip(title, 200), Text: clip(text, maxForwardedItemText), Description: description})
	})
}

// TouchCoordinatorClient records the coordinator's client name (from MCP
// initialize) as well as the time.
func (s *Store) TouchCoordinatorClient(project, client string) error {
	if err := s.TouchCoordinator(project); err != nil {
		return err
	}
	_, err := s.db.Exec(`UPDATE coordinator_connections SET client=? WHERE project=?`, clip(client, 64), project)
	return err
}

// WindowTask is one row of the task list.
type WindowTask struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Agent  string `json:"agent"`
	State  string `json:"state"`
	Folder string `json:"folder"`
	Model  string `json:"model,omitempty"`
	// RanModel is the model the worker's CLI reported at its last start
	// (T-108); Model is the one the coordinator asked for.
	RanModel   string `json:"ran_model,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	// OwnerQuestion is set while the worker waits for the owner.
	OwnerQuestion *TaskQuestion `json:"owner_question,omitempty"`
	// PendingLaunch is set while the task's latest run is recorded but not
	// launched (paused queue or tasks not allowed).
	PendingLaunch bool `json:"pending_launch,omitempty"`
	// WaitsForLock is the lock a pending task waits for (T-072).
	WaitsForLock string `json:"waits_for_lock,omitempty"`
	// StartedAt is when the first run was launched; durations count from it.
	StartedAt string `json:"started_at,omitempty"`
	// FailureReason and FailureReset explain a failed task (T-034).
	FailureReason string `json:"failure_reason,omitempty"`
	FailureReset  string `json:"failure_reset,omitempty"`
	// Remote names the other computer of a task between computers: the one
	// that runs it, or (From) the one that started it.
	Remote *RemoteComputer `json:"remote,omitempty"`
	// Usage is what the task's runs spent so far (T-058).
	Usage *TaskUsage `json:"usage,omitempty"`
	// FullAccess: the worker runs with «Любые команды» (T-115). Only for
	// tasks that run on this computer.
	FullAccess bool `json:"full_access,omitempty"`
	// Quiet describes the live run of a running task of this computer:
	// its last output and, when its final message is out but the run has
	// not ended, the processes still running (T-172).
	Quiet *RunQuiet `json:"quiet,omitempty"`
}

// RemoteComputer is the other computer of a task (T-037).
type RemoteComputer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	From bool   `json:"from,omitempty"`
	// Lost: this computer has no contact with the computer that runs the
	// task (T-126). It is offline, left the network, or the network was
	// made again. The task's outcome there is unknown; LastSeen is when it
	// last reported, if known.
	Lost     bool   `json:"lost,omitempty"`
	LastSeen string `json:"last_seen,omitempty"`
	// Unaccepted: the computer that should run the task has not said it
	// accepted it (T-137). Until it does, nothing is known to run there.
	Unaccepted bool `json:"unaccepted,omitempty"`
	// Silent: that computer is in reach but has not answered: the task was
	// not accepted within unacceptedAfter, or a cancel went unanswered for
	// cancelAnswerWait. It can be closed here at once (owner, 2026-11-12:
	// such a task stayed among the active ones).
	Silent bool `json:"silent,omitempty"`
	// Delayed: that computer reports, but its oldest unsent message has
	// waited this many minutes (T-150): its replies are late, not lost.
	Delayed int `json:"outbox_delayed_minutes,omitempty"`
	// Worker is whether a worker process runs the task there now, as that
	// computer last reported (T-149): "running" or "none"; empty when that
	// computer does not say (before 0.67) or the task is not working.
	// WorkerOutputAt is when that worker last wrote output.
	Worker         string `json:"worker,omitempty"`
	WorkerOutputAt string `json:"worker_output_at,omitempty"`
}

// TaskRemote names the other computer of a task, or nil for a task of this
// computer only.
func (s *Store) TaskRemote(task string) (*RemoteComputer, error) { return s.remoteOf(task) }

// remoteComputer describes the computer that runs a task this computer
// started.
func remoteComputer(machine, tx *sql.Tx, task string, link netLink) (*RemoteComputer, error) {
	id, name, err := linkComputer(machine, link)
	if err != nil {
		return nil, err
	}
	out := &RemoteComputer{ID: id, Name: name}
	return out, remoteContact(machine, tx, task, link, out)
}

// remoteContact fills in whether the computer that runs a task is out of
// reach (T-126).
func remoteContact(machine, tx *sql.Tx, task string, link netLink, out *RemoteComputer) error {
	var name, remoteTask sql.NullString
	var created, updated string
	var cancel int
	if err := tx.QueryRow(`SELECT remote_name,remote_task,created_at,updated_at,cancel_requested FROM tasks WHERE id=?`, task).Scan(&name, &remoteTask, &created, &updated, &cancel); err != nil {
		return err
	}
	out.Unaccepted = remoteTask.String == ""
	older := func(at string, d time.Duration) bool {
		t, err := time.Parse(time.RFC3339Nano, at)
		return err == nil && time.Since(t) > d
	}
	out.Silent = (out.Unaccepted && older(created, unacceptedAfter)) || (cancel != 0 && older(updated, cancelAnswerWait))
	var broker string
	if err := machine.QueryRow(`SELECT COALESCE((SELECT value FROM meta WHERE key='net_broker'),'')`).Scan(&broker); err != nil {
		return err
	}
	peer, found, err := netPeer(machine, link.peer)
	if err != nil {
		return err
	}
	if (out.Name == "" || out.Name == link.peer) && name.String != "" {
		out.Name = name.String
	}
	if out.Name == "" {
		out.Name = formerComputer
	}
	if found && peer.Age >= 0 {
		out.LastSeen = time.Now().UTC().Add(-time.Duration(peer.Age) * time.Second).Format(time.RFC3339)
	}
	out.Lost = broker == "" || broker != link.broker || !found || peer.Age < 0 || peer.Age > PeerFresh
	if !out.Lost {
		out.Delayed = peer.OutboxDelayedMinutes()
		var state string
		if err := tx.QueryRow(`SELECT state FROM tasks WHERE id=?`, task).Scan(&state); err != nil {
			return err
		}
		if peer.Workers != nil && state == "running" && !out.Unaccepted {
			out.Worker = "none"
			for _, r := range peer.Workers.Runs {
				if r.Task == task {
					out.Worker = "running"
					out.WorkerOutputAt = time.Now().UTC().Add(-time.Duration(r.OutputAge+peer.Age) * time.Second).Format(time.RFC3339)
				}
			}
		}
	}
	return nil
}

func (s *Store) remoteOf(task string) (*RemoteComputer, error) {
	var out *RemoteComputer
	err := s.taskTx(task, func(machine, tx *sql.Tx) error {
		link, err := taskLink(tx, task)
		if err != nil {
			return err
		}
		switch {
		case link.remote != nil:
			out, err = remoteComputer(machine, tx, task, *link.remote)
			return err
		case link.origin != nil:
			id, name, err := linkComputer(machine, *link.origin)
			if name == "" {
				name = formerComputer
			}
			out = &RemoteComputer{ID: id, Name: name, From: true}
			return err
		}
		return nil
	})
	return out, err
}

func (s *Store) ranModel(task string) string {
	var m sql.NullString
	_ = s.db.QueryRow(`SELECT ran_model FROM tasks WHERE id=?`, task).Scan(&m)
	return m.String
}

func (s *Store) startedAt(task string) (string, error) {
	return taskStartedAt(s.db, task)
}

func taskStartedAt(q interface{ QueryRow(string, ...any) *sql.Row }, task string) (string, error) {
	var at sql.NullString
	err := q.QueryRow(`SELECT MIN(launched_at) FROM task_runs WHERE task_id=?`, task).Scan(&at)
	return at.String, err
}

func (s *Store) pendingLaunch(task string) (bool, error) {
	var state string
	err := s.db.QueryRow(`SELECT state FROM task_runs WHERE task_id=? ORDER BY ordinal DESC LIMIT 1`, task).Scan(&state)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return state == "intent", err
}

// WindowAgent is one agent program of this computer, with its state across
// all projects (D-026).
type WindowAgent struct {
	Agent   string `json:"agent"`
	CLI     string `json:"cli"`
	Profile string `json:"profile"`
	Env     string `json:"environment"`
	Running int    `json:"running"`
	Waiting int    `json:"waiting_owner"`
	// TasksToday counts this project's tasks of this agent started today
	// (local time of the client), for the agent card.
	ProjectRunning int `json:"project_running"`
	ProjectWaiting int `json:"project_waiting"`
	ProjectToday   int `json:"project_today"`
	// Models are the names the owner listed for the coordinator (T-043).
	Models []string `json:"models"`
	// FullAccess: «Разрешить любые команды» is on (T-054).
	FullAccess bool `json:"full_access"`
	// Limit is the last usage-limit state the agent reported (T-044).
	Limit *AgentLimit `json:"limit,omitempty"`
	// Paused: the owner turned the agent off on its card (T-061).
	Paused bool `json:"paused"`
	// TodayTokens are the tokens of this agent's tasks on this computer
	// started today, in every project; LastDone is when its last task here
	// finished well, the proof that it works with Messenger (D-070).
	TodayTokens int64  `json:"today_tokens"`
	LastDone    string `json:"last_done,omitempty"`
}

// WindowView is what the window shows for one project.
type WindowView struct {
	Projects []Project `json:"projects"`
	Project  string    `json:"project"`
	// ProjectTitle is the name the window shows for the project (T-065).
	ProjectTitle string `json:"project_title"`
	Folder       string `json:"folder"`
	// WorkerRules are the owner's rules for this project's workers (T-050).
	WorkerRules string `json:"worker_rules"`
	Computer    string `json:"computer"`
	Name        string `json:"computer_name"`
	Coordinator struct {
		Connected bool   `json:"connected"`
		Active    bool   `json:"active,omitempty"`
		Client    string `json:"client,omitempty"`
		LastSeen  string `json:"last_seen,omitempty"`
	} `json:"coordinator"`
	CoordinatorModels map[string][]string `json:"coordinator_models"`
	Agents            []WindowAgent       `json:"agents"`
	Tasks             []WindowTask        `json:"tasks"`
	// HistoryNext identifies the last completed row of the first page.
	HistoryNext      string `json:"history_next,omitempty"`
	HistoryVersion   string `json:"history_version,omitempty"`
	HistoryUnchanged bool   `json:"history_unchanged,omitempty"`
	HistoryPaged     bool   `json:"history_paged,omitempty"`
	// Shared are the other computers' tasks of this project (T-047).
	Shared []NetPeerTasks `json:"shared"`
	// Pending counts this project's runs that are recorded but not launched.
	Pending int `json:"pending_launch"`
}

// WindowView builds the window's view of a project. An empty project picks
// the first one.
func (s *Store) WindowView(project, today string) (WindowView, error) {
	return s.WindowViewFiltered(project, today, "", "", "")
}

// WindowViewFiltered keeps every unfinished row available for activity and
// limits the completed part of the list to the current filter's first page.
func (s *Store) WindowViewFiltered(project, today, agent, computer, historyVersion string) (WindowView, error) {
	var v WindowView
	projects, err := s.Projects()
	if err != nil {
		return v, err
	}
	v.Projects = projects
	if v.Projects == nil {
		v.Projects = []Project{}
	}
	v.Agents, v.Tasks, v.Shared = []WindowAgent{}, []WindowTask{}, []NetPeerTasks{}
	if project == "" && len(projects) > 0 {
		project = projects[0].Name
	}
	found := false
	for _, p := range projects {
		if p.Name == project {
			found, v.Project, v.ProjectTitle, v.Folder = true, p.Name, p.Title, p.Folder
		}
	}
	var coordinator platform.Observation
	err = s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		v.Computer = computer
		v.CoordinatorModels, err = coordinatorModelLists(tx, computer)
		if err != nil {
			return err
		}
		if found {
			if err := tx.QueryRow(`SELECT worker_rules FROM projects WHERE name=?`, v.Project).Scan(&v.WorkerRules); err != nil {
				return err
			}
		}
		if v.Name, err = computerName(tx); err != nil {
			return err
		}
		if s.split != nil {
			v.Agents, err = s.splitWindowAgents(tx, v.Project, today, computer)
			if err != nil {
				return err
			}
		} else {
			rows, err := tx.Query(`SELECT a.provider,a.cli_path,a.profile,a.environment,
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='running'),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='waiting_owner'),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='running' AND t.project=?),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='waiting_owner' AND t.project=?),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.project=? AND t.created_at>=?),
 a.models, a.limit_state, a.full_access, a.paused,
 (SELECT COALESCE(SUM(t.usage_input+t.usage_output),0) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.created_at>=?),
 (SELECT COALESCE(MAX(t.finished_at),'') FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='done')
 FROM computer_agents a WHERE a.computer=? ORDER BY a.provider`, project, project, project, today, today, computer)
			if err != nil {
				return err
			}
			for rows.Next() {
				var a WindowAgent
				var models, limit string
				if err := rows.Scan(&a.Agent, &a.CLI, &a.Profile, &a.Env, &a.Running, &a.Waiting, &a.ProjectRunning, &a.ProjectWaiting, &a.ProjectToday, &models, &limit, &a.FullAccess, &a.Paused, &a.TodayTokens, &a.LastDone); err != nil {
					rows.Close()
					return err
				}
				a.Models = splitModels(models)
				a.Limit = parseAgentLimit(limit)
				v.Agents = append(v.Agents, a)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}
		}
		if !found {
			return nil
		}
		var client sql.NullString
		err = tx.QueryRow(`SELECT c.last_seen,c.client,COALESCE(a.pid,0),COALESCE(a.start_token,''),COALESCE(a.boot_id,'')
 FROM coordinator_connections c LEFT JOIN coordinator_chats a ON a.project=c.project WHERE c.project=?`, project).
			Scan(&v.Coordinator.LastSeen, &client, &coordinator.PID, &coordinator.StartToken, &coordinator.BootID)
		if err == nil {
			v.Coordinator.Connected, v.Coordinator.Client = true, client.String
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		v.Shared, err = projectNetTasks(tx, project, s.split != nil)
		return err
	})
	if err != nil || !found {
		return v, err
	}
	// A saved connection alone does not mean its chat is still open.
	if coordinator.PID > 0 && coordinator.StartToken != "" && coordinator.BootID != "" {
		live, err := platform.ReadObservation(coordinator.PID)
		v.Coordinator.Active = err == nil && live.StartToken == coordinator.StartToken && live.BootID == coordinator.BootID
	}
	scoped, err := s.projectStore(project)
	if err != nil {
		return v, err
	}
	if err := scoped.db.QueryRow(`SELECT COUNT(*) FROM task_runs r JOIN tasks t ON t.id=r.task_id WHERE t.project=? AND r.state='intent' AND t.state='running'`, project).Scan(&v.Pending); err != nil {
		return v, err
	}
	active, err := scoped.windowTaskRows(`WHERE project=? AND hidden_at IS NULL AND state NOT IN `+finishedWindowStates+`
 ORDER BY created_at DESC,rowid DESC`, project)
	if err != nil {
		return v, err
	}
	v.Tasks, v.HistoryPaged = active, true
	broker, err := currentBroker(s.db)
	if err != nil {
		return v, err
	}
	v.HistoryVersion, err = scoped.windowHistoryVersion(project, agent, computer, broker)
	if err != nil {
		return v, err
	}
	if v.HistoryVersion == historyVersion {
		v.HistoryUnchanged = true
		return v, nil
	}
	history, err := scoped.windowTaskHistory(project, "", agent, computer, broker)
	if err != nil {
		return v, err
	}
	v.Tasks = append(v.Tasks, history.Tasks...)
	v.HistoryNext = history.Next
	return v, nil
}

func (s *Store) windowTaskRows(selection string, args ...any) ([]WindowTask, error) {
	rows, err := s.db.Query(`SELECT `+taskColumns+` FROM tasks `+selection, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []Task
	for rows.Next() {
		task, err := scanTask(rows.Scan)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	out := make([]WindowTask, 0, len(tasks))
	for _, t := range tasks {
		w := WindowTask{ID: t.ID, Title: t.Title, Agent: t.Provider, State: t.State, Folder: t.Folder, Model: t.Model, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, FinishedAt: t.FinishedAt, FailureReason: t.FailureReason, FailureReset: t.FailureReset, FullAccess: t.FullAccess}
		// The row shows the model the worker started on (T-202).
		w.RanModel = s.ranModel(t.ID)
		if t.State == "waiting_owner" {
			question, err := openQuestion(s.db, t.ID)
			if err != nil {
				return nil, err
			}
			if question != nil && question.Audience == AudienceOwner {
				w.OwnerQuestion = question
			}
		}
		if t.State == "running" {
			if w.PendingLaunch, err = s.pendingLaunch(t.ID); err != nil {
				return nil, err
			}
			if w.PendingLaunch {
				if w.WaitsForLock, err = s.waitingLock(t.ID); err != nil {
					return nil, err
				}
			} else if w.Quiet, err = s.TaskRunQuiet(t.ID); err != nil {
				return nil, err
			}
		}
		if w.StartedAt, err = s.startedAt(t.ID); err != nil {
			return nil, err
		}
		if w.Remote, err = s.remoteOf(t.ID); err != nil {
			return nil, err
		}
		if w.Usage, err = s.taskUsage(t.ID); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

// FeedItem is one entry of a task's feed, rendered like a session in the
// Claude app: messages to the worker, the worker's text and tool actions,
// its questions and report, and the outcome.
type FeedItem struct {
	// Kind: task, answer, note, text, tool, command_start, command_end, question, report, outcome.
	Kind string `json:"kind"`
	// Author: coordinator, owner, worker or messenger.
	Author      string `json:"author"`
	SenderAgent string `json:"sender_agent,omitempty"`
	SenderModel string `json:"sender_model,omitempty"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Text        string `json:"text"`
	At          string `json:"at"`
	Run         int    `json:"run,omitempty"`
	// Verified is false for a worker's report or question that was not
	// published because its run failed verification.
	Verified bool   `json:"verified"`
	Audience string `json:"audience,omitempty"`
	State    string `json:"state,omitempty"`
	// Images are the pictures the worker attached to this report or
	// question (T-175).
	Images []TaskImage `json:"images,omitempty"`
}

// TaskFeed is a task with its feed.
type TaskFeed struct {
	Task       WindowTask `json:"task"`
	Items      []FeedItem `json:"items"`
	Cursor     string     `json:"cursor,omitempty"`
	AppendFrom int        `json:"append_from,omitempty"`
}

func (s *Store) taskFeedHeader(id string) (TaskFeed, Task, error) {
	var f TaskFeed
	t, err := s.TaskByID(id)
	if err != nil {
		return f, t, err
	}
	f.Task = WindowTask{ID: t.ID, Title: t.Title, Agent: t.Provider, State: t.State, Folder: t.Folder, Model: t.Model, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, FinishedAt: t.FinishedAt, RanModel: t.RanModel, FullAccess: t.FullAccess}
	if t.OpenQuestion != nil && t.OpenQuestion.Audience == AudienceOwner {
		f.Task.OwnerQuestion = t.OpenQuestion
	}
	if t.State == "running" {
		if f.Task.PendingLaunch, err = s.pendingLaunch(t.ID); err != nil {
			return f, t, err
		}
		if f.Task.Quiet, err = s.TaskRunQuiet(t.ID); err != nil {
			return f, t, err
		}
	}
	if f.Task.StartedAt, err = s.startedAt(t.ID); err != nil {
		return f, t, err
	}
	if f.Task.Remote, err = s.remoteOf(t.ID); err != nil {
		return f, t, err
	}
	if f.Task.Usage, err = s.taskUsage(t.ID); err != nil {
		return f, t, err
	}
	return f, t, nil
}

// TaskFeed builds the feed of one task.
func (s *Store) TaskFeed(id string) (TaskFeed, error) {
	if s.split != nil {
		project, err := s.storeForTask(id)
		if err != nil {
			return TaskFeed{}, err
		}
		return project.TaskFeed(id)
	}
	f, t, err := s.taskFeedHeader(id)
	if err != nil {
		return f, err
	}
	f.Items = []FeedItem{{Kind: "task", Author: AudienceCoordinator, Text: t.Text, At: t.CreatedAt, Verified: true, SenderAgent: t.SenderAgent, SenderModel: t.SenderModel}}
	if f.Task.Remote != nil && !f.Task.Remote.From {
		err = s.remoteFeed(&f)
		return f, err
	}
	err = s.tx(func(tx *sql.Tx) error {
		runs, err := tx.Query(`SELECT r.id,r.ordinal,r.kind,r.input,r.state,COALESCE(r.disposition,''),COALESCE(r.disposition_text,''),r.created_at,COALESCE(r.finished_at,''),r.detail,
 CASE WHEN r.kind='answer' AND r.sender_agent='' THEN COALESCE((SELECT answered_by FROM task_questions q WHERE q.task_id=r.task_id AND q.answer=r.input AND q.state='answered' ORDER BY q.rowid DESC LIMIT 1),'coordinator') ELSE 'coordinator' END, r.sender_agent,r.sender_model
 FROM task_runs r WHERE r.task_id=? ORDER BY r.ordinal`, id)
		if err != nil {
			return err
		}
		type runRow struct {
			id, kind, input, state, disposition, text, created, finished, detail, by string
			senderAgent, senderModel                                                 string
			ordinal                                                                  int
		}
		var list []runRow
		for runs.Next() {
			var r runRow
			if err := runs.Scan(&r.id, &r.ordinal, &r.kind, &r.input, &r.state, &r.disposition, &r.text, &r.created, &r.finished, &r.detail, &r.by, &r.senderAgent, &r.senderModel); err != nil {
				runs.Close()
				return err
			}
			list = append(list, r)
		}
		runs.Close()
		if err := runs.Err(); err != nil {
			return err
		}
		for i, r := range list {
			if r.kind == "answer" || r.kind == "continue" {
				f.Items = append(f.Items, FeedItem{Kind: "answer", Author: r.by, Text: r.input, At: r.created, Run: r.ordinal, Verified: true, SenderAgent: r.senderAgent, SenderModel: r.senderModel})
			}
			items, err := tx.Query(`SELECT kind,title,text,description,created_at FROM task_run_items WHERE run_id=? ORDER BY seq`, r.id)
			if err != nil {
				return err
			}
			for items.Next() {
				var it FeedItem
				if err := items.Scan(&it.Kind, &it.Title, &it.Text, &it.Description, &it.At); err != nil {
					items.Close()
					return err
				}
				it.Author, it.Run, it.Verified = "worker", r.ordinal, true
				f.Items = append(f.Items, it)
			}
			items.Close()
			if r.disposition != "" {
				kind, audience := "report", ""
				if r.disposition != DispositionReport {
					kind, audience = "question", AudienceCoordinator
					if r.disposition == DispositionAskOwner {
						audience = AudienceOwner
					}
				}
				at := r.finished
				if at == "" {
					at = r.created
				}
				// A report kept back because the owner wrote meanwhile (T-085).
				if kind == "report" && i+1 < len(list) && list[i+1].kind == "note" {
					audience = "held"
				}
				images, err := queryImages(tx, `SELECT id,name,type,length(data) FROM task_images WHERE run_id=? ORDER BY seq`, r.id)
				if err != nil {
					return err
				}
				f.Items = append(f.Items, FeedItem{Kind: kind, Author: "worker", Text: r.text, At: at, Run: r.ordinal, Verified: r.state == "finished", Audience: audience, State: r.state, Images: images})
			}
			switch r.state {
			case "failed", "blocked", "interrupted", "cancelled":
				f.Items = append(f.Items, FeedItem{Kind: "outcome", Author: "messenger", Text: r.detail, At: r.finished, Run: r.ordinal, State: r.state, Verified: true})
			}
		}
		notes, err := ownerNoteItems(tx, id)
		if err != nil {
			return err
		}
		f.Items = mergeByTime(f.Items, notes)
		return nil
	})
	return f, err
}

// remoteFeed builds the feed of a task that runs on another computer from
// the events it reported: its questions, the answers sent from here, its
// report and outcome. The worker's own text and tool actions stay on the
// computer that ran it.
func (s *Store) remoteFeed(f *TaskFeed) error {
	if err := s.remoteEvents(f); err != nil {
		return err
	}
	rows, err := s.db.Query(`SELECT kind,title,text,description,created_at FROM remote_task_items WHERE task_id=? ORDER BY seq`, f.Task.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		it := FeedItem{Author: "worker", Verified: true}
		if err := rows.Scan(&it.Kind, &it.Title, &it.Text, &it.Description, &it.At); err != nil {
			return err
		}
		f.Items = append(f.Items, it)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// T-149: the coordinator's notes sent to the worker there.
	if err := s.tx(func(tx *sql.Tx) error {
		notes, err := ownerNoteItems(tx, f.Task.ID)
		f.Items = append(f.Items, notes...)
		return err
	}); err != nil {
		return err
	}
	// Events, notes and progress lines come from three tables: show them in
	// time order. The task itself stays first.
	at := func(i int) time.Time { v, _ := time.Parse(time.RFC3339Nano, f.Items[i].At); return v }
	rest := f.Items[1:]
	sort.SliceStable(rest, func(i, j int) bool { return at(i + 1).Before(at(j + 1)) })
	return nil
}

func (s *Store) remoteEvents(f *TaskFeed) error {
	// T-175: pictures sent along belong to the report or question that
	// followed them.
	pics := map[int][]TaskImage{}
	picRows, err := s.db.Query(`SELECT event_seq,id,name,type,length(data) FROM task_images WHERE task_id=? AND run_id='' ORDER BY seq`, f.Task.ID)
	if err != nil {
		return err
	}
	for picRows.Next() {
		var n int
		var img TaskImage
		if err := picRows.Scan(&n, &img.ID, &img.Name, &img.Type, &img.Size); err != nil {
			picRows.Close()
			return err
		}
		pics[n] = append(pics[n], img)
	}
	picRows.Close()
	if err := picRows.Err(); err != nil {
		return err
	}
	rows, err := s.db.Query(`SELECT audience,kind,text,created_at,sender_agent,sender_model FROM task_events WHERE task_id=? ORDER BY seq`, f.Task.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	disposed := 0
	for rows.Next() {
		var audience, kind, text, at, senderAgent, senderModel string
		if err := rows.Scan(&audience, &kind, &text, &at, &senderAgent, &senderModel); err != nil {
			return err
		}
		switch kind {
		case "question", "report", "late_report":
			disposed++
		}
		switch kind {
		case "question":
			f.Items = append(f.Items, FeedItem{Kind: "question", Author: "worker", Text: text, At: at, Verified: true, Audience: audience, State: "finished", Images: pics[disposed]})
		case "answered", "continued":
			by := AudienceCoordinator
			if audience == AudienceOwner {
				by = AudienceOwner
			}
			f.Items = append(f.Items, FeedItem{Kind: "answer", Author: by, Text: text, At: at, Verified: true, SenderAgent: senderAgent, SenderModel: senderModel})
		case "accepted":
			f.Items = append(f.Items, FeedItem{Kind: "accepted", Author: "messenger", Text: f.Task.Remote.Name, At: at, Verified: true})
		case "report", "late_report":
			f.Items = append(f.Items, FeedItem{Kind: "report", Author: "worker", Text: text, At: at, Verified: true, State: "finished", Images: pics[disposed]})
		case "failed", "blocked", "interrupted", "cancelled":
			f.Items = append(f.Items, FeedItem{Kind: "outcome", Author: "messenger", Text: text, At: at, State: kind, Verified: true})
		}
	}
	return rows.Err()
}

// WindowAnswer is the owner's answer from the window: only to an open owner
// question of the task (D-024).
func (s *Store) WindowAnswer(task, question, opID, text string) error {
	if strings.TrimSpace(question) == "" {
		return fmt.Errorf("%w: no question", ErrRejected)
	}
	return s.AnswerTask(task, question, opID, text, AudienceOwner)
}

// AgentPresence is one agent program of this computer as other computers
// see it: how many of its tasks run and how many wait for the owner.
type AgentPresence struct {
	Agent   string
	Running int
	Waiting int
	Models  []string
	Limit   *AgentLimit
}

// Presence summarizes this computer's agents for the server's list of
// computers. Coordinator is the program of the most recently seen
// coordinator connection, if any.
func (s *Store) Presence() (agents []AgentPresence, coordinator string, err error) {
	agents = []AgentPresence{}
	err = s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		var activity map[string]taskAgentActivity
		query := `SELECT a.provider,
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='running'),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='waiting_owner'),
 a.models, a.limit_state
 FROM computer_agents a WHERE a.computer=? AND a.paused=0 ORDER BY a.provider`
		if s.split != nil {
			activity, err = s.splitAgentActivity(tx, "", computer)
			if err != nil {
				return err
			}
			query = `SELECT provider,0,0,models,limit_state FROM computer_agents WHERE computer=? AND paused=0 ORDER BY provider`
		}
		rows, err := tx.Query(query, computer)
		if err != nil {
			return err
		}
		for rows.Next() {
			var a AgentPresence
			var models, limit string
			if err := rows.Scan(&a.Agent, &a.Running, &a.Waiting, &models, &limit); err != nil {
				rows.Close()
				return err
			}
			if s.split != nil {
				a.Running, a.Waiting = activity[a.Agent].Running, activity[a.Agent].WaitingOwner
			}
			a.Models = splitModels(models)
			a.Limit = parseAgentLimit(limit)
			agents = append(agents, a)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		var client sql.NullString
		err = tx.QueryRow(`SELECT client FROM coordinator_connections ORDER BY last_seen DESC LIMIT 1`).Scan(&client)
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		// A connection whose program is not known yet is still a coordinator.
		coordinator = client.String
		if coordinator == "" {
			coordinator = "coordinator"
		}
		return nil
	})
	return agents, coordinator, err
}

// MaxComputerName bounds the name of this computer in the network, as the
// broker bounds device names.
const MaxComputerName = 64

func computerName(tx *sql.Tx) (string, error) {
	name, err := metaValue(tx, "computer_name")
	if err != nil || name != "" {
		return name, err
	}
	name, _ = os.Hostname()
	return name, nil
}

// ComputerName is the name other computers of the network see: the owner's
// choice, or the system's name.
func (s *Store) ComputerName() (string, error) {
	var name string
	err := s.tx(func(tx *sql.Tx) (err error) { name, err = computerName(tx); return })
	return name, err
}

// ComputerID is this computer's lasting id, the same in every network
// (T-132).
func (s *Store) ComputerID() (string, error) {
	var id string
	err := s.tx(func(tx *sql.Tx) (err error) { id, err = localComputer(tx); return })
	return id, err
}

// AdoptComputerName keeps name as this computer's name unless the owner
// already chose one. A computer that joined a network before the window
// remembered the joining name (D-061) still uses the name the others see.
func (s *Store) AdoptComputerName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" || utf8.RuneCountInString(name) > MaxComputerName || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return nil
	}
	return s.tx(func(tx *sql.Tx) error {
		current, err := metaValue(tx, "computer_name")
		if err != nil || current != "" {
			return err
		}
		return setMeta(tx, "computer_name", name)
	})
}

// SetComputerName names this computer for its network; an empty name goes
// back to the system's name.
func (s *Store) SetComputerName(name string) error {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) > MaxComputerName || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return fmt.Errorf("%w: имя компьютера — до %d символов, в одну строку", ErrRejected, MaxComputerName)
	}
	return s.tx(func(tx *sql.Tx) error {
		if name == "" {
			_, err := tx.Exec(`DELETE FROM meta WHERE key='computer_name'`)
			return err
		}
		return setMeta(tx, "computer_name", name)
	})
}
