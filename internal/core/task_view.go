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
 created_at TEXT NOT NULL,
 PRIMARY KEY (run_id, seq)
);
CREATE TABLE IF NOT EXISTS remote_task_items (
 task_id TEXT NOT NULL REFERENCES tasks(id),
 seq INTEGER NOT NULL,
 kind TEXT NOT NULL,
 title TEXT NOT NULL,
 text TEXT NOT NULL,
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

// AppendTaskRunItem stores one sanitized item of a worker's event stream
// (its text or a tool action) for the window. Items past the bound are
// dropped; this never affects verification.
func (s *Store) AppendTaskRunItem(run, kind, title, text string) error {
	if kind != "text" && kind != "tool" {
		return fmt.Errorf("%w: unknown item kind", ErrRejected)
	}
	return s.tx(func(tx *sql.Tx) error {
		var n int
		if err := tx.QueryRow(`SELECT COUNT(*) FROM task_run_items WHERE run_id=?`, run).Scan(&n); err != nil {
			return err
		}
		if n >= maxRunItems {
			return nil
		}
		if _, err := tx.Exec(`INSERT INTO task_run_items(run_id,seq,kind,title,text,created_at) VALUES (?,?,?,?,?,?)`, run, n+1, kind, clip(title, 200), clip(text, maxRunItemText), now()); err != nil {
			return err
		}
		if n >= maxForwardedItems {
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
		return enqueueNet(tx, link.origin.broker, link.origin.peer, fmt.Sprintf("progress.%s.%d", run, n+1), task,
			netBody{Kind: "progress", Task: link.origin.task, Event: kind, Title: clip(title, 200), Text: clip(text, maxForwardedItemText)})
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
	ID         string `json:"id"`
	Title      string `json:"title"`
	Agent      string `json:"agent"`
	State      string `json:"state"`
	Folder     string `json:"folder"`
	Model      string `json:"model,omitempty"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	// OwnerQuestion is set while the worker waits for the owner.
	OwnerQuestion *TaskQuestion `json:"owner_question,omitempty"`
	// PendingLaunch is set while the task's latest run is recorded but not
	// launched (paused queue or tasks not allowed).
	PendingLaunch bool `json:"pending_launch,omitempty"`
	// StartedAt is when the first run was launched; durations count from it.
	StartedAt string `json:"started_at,omitempty"`
	// FailureReason and FailureReset explain a failed task (T-034).
	FailureReason string `json:"failure_reason,omitempty"`
	FailureReset  string `json:"failure_reset,omitempty"`
	// Remote names the other computer of a task between computers: the one
	// that runs it, or (From) the one that started it.
	Remote *RemoteComputer `json:"remote,omitempty"`
}

// RemoteComputer is the other computer of a task (T-037).
type RemoteComputer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	From bool   `json:"from,omitempty"`
}

func (s *Store) remoteOf(task string) (*RemoteComputer, error) {
	var out *RemoteComputer
	err := s.tx(func(tx *sql.Tx) error {
		link, err := taskLink(tx, task)
		if err != nil {
			return err
		}
		switch {
		case link.remote != nil:
			out = &RemoteComputer{ID: link.remote.peer, Name: peerName(tx, link.remote.peer)}
		case link.origin != nil:
			out = &RemoteComputer{ID: link.origin.peer, Name: peerName(tx, link.origin.peer), From: true}
		}
		return nil
	})
	return out, err
}

func (s *Store) startedAt(task string) (string, error) {
	var at sql.NullString
	err := s.db.QueryRow(`SELECT MIN(launched_at) FROM task_runs WHERE task_id=?`, task).Scan(&at)
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
	ProjectToday   int `json:"project_today"`
	// Models are the names the owner listed for the coordinator (T-043).
	Models []string `json:"models"`
	// Limit is the last usage-limit state the agent reported (T-044).
	Limit *AgentLimit `json:"limit,omitempty"`
}

// WindowView is what the window shows for one project.
type WindowView struct {
	Projects    []Project `json:"projects"`
	Project     string    `json:"project"`
	Folder      string    `json:"folder"`
	Computer    string    `json:"computer"`
	Name        string    `json:"computer_name"`
	Coordinator struct {
		Connected bool   `json:"connected"`
		Client    string `json:"client,omitempty"`
		LastSeen  string `json:"last_seen,omitempty"`
	} `json:"coordinator"`
	Agents []WindowAgent `json:"agents"`
	Tasks  []WindowTask  `json:"tasks"`
	// Pending counts this project's runs that are recorded but not launched.
	Pending int `json:"pending_launch"`
}

// WindowView builds the window's view of a project. An empty project picks
// the first one.
func (s *Store) WindowView(project, today string) (WindowView, error) {
	var v WindowView
	projects, err := s.Projects()
	if err != nil {
		return v, err
	}
	v.Projects = projects
	if v.Projects == nil {
		v.Projects = []Project{}
	}
	v.Agents, v.Tasks = []WindowAgent{}, []WindowTask{}
	if project == "" && len(projects) > 0 {
		project = projects[0].Name
	}
	found := false
	for _, p := range projects {
		if p.Name == project {
			found, v.Project, v.Folder = true, p.Name, p.Folder
		}
	}
	err = s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		v.Computer = computer
		if v.Name, err = computerName(tx); err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT a.provider,a.cli_path,a.profile,a.environment,
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='running'),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='waiting_owner'),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='running' AND t.project=?),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.project=? AND t.created_at>=?),
 a.models, a.limit_state
 FROM computer_agents a WHERE a.computer=? ORDER BY a.provider`, project, project, today, computer)
		if err != nil {
			return err
		}
		for rows.Next() {
			var a WindowAgent
			var models, limit string
			if err := rows.Scan(&a.Agent, &a.CLI, &a.Profile, &a.Env, &a.Running, &a.Waiting, &a.ProjectRunning, &a.ProjectToday, &models, &limit); err != nil {
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
		if !found {
			return nil
		}
		var client sql.NullString
		err = tx.QueryRow(`SELECT last_seen,client FROM coordinator_connections WHERE project=?`, project).Scan(&v.Coordinator.LastSeen, &client)
		if err == nil {
			v.Coordinator.Connected, v.Coordinator.Client = true, client.String
		} else if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err := tx.QueryRow(`SELECT COUNT(*) FROM task_runs r JOIN tasks t ON t.id=r.task_id WHERE t.project=? AND r.state='intent' AND t.state='running'`, project).Scan(&v.Pending); err != nil {
			return err
		}
		return nil
	})
	if err != nil || !found {
		return v, err
	}
	tasks, err := s.Tasks(project, 300)
	if err != nil {
		return v, err
	}
	for _, t := range tasks {
		w := WindowTask{ID: t.ID, Title: t.Title, Agent: t.Provider, State: t.State, Folder: t.Folder, Model: t.Model, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, FinishedAt: t.FinishedAt, FailureReason: t.FailureReason, FailureReset: t.FailureReset}
		if t.OpenQuestion != nil && t.OpenQuestion.Audience == AudienceOwner {
			w.OwnerQuestion = t.OpenQuestion
		}
		if t.State == "running" {
			if w.PendingLaunch, err = s.pendingLaunch(t.ID); err != nil {
				return v, err
			}
		}
		if w.StartedAt, err = s.startedAt(t.ID); err != nil {
			return v, err
		}
		if w.Remote, err = s.remoteOf(t.ID); err != nil {
			return v, err
		}
		v.Tasks = append(v.Tasks, w)
	}
	return v, nil
}

// FeedItem is one entry of a task's feed, rendered like a session in the
// Claude app: messages to the worker, the worker's text and tool actions,
// its questions and report, and the outcome.
type FeedItem struct {
	// Kind: task, answer, text, tool, question, report, outcome.
	Kind string `json:"kind"`
	// Author: coordinator, owner, worker or messenger.
	Author string `json:"author"`
	Title  string `json:"title,omitempty"`
	Text   string `json:"text"`
	At     string `json:"at"`
	Run    int    `json:"run,omitempty"`
	// Verified is false for a worker's report or question that was not
	// published because its run failed verification.
	Verified bool   `json:"verified"`
	Audience string `json:"audience,omitempty"`
	State    string `json:"state,omitempty"`
}

// TaskFeed is a task with its feed.
type TaskFeed struct {
	Task  WindowTask `json:"task"`
	Items []FeedItem `json:"items"`
}

// TaskFeed builds the feed of one task.
func (s *Store) TaskFeed(id string) (TaskFeed, error) {
	var f TaskFeed
	t, err := s.TaskByID(id)
	if err != nil {
		return f, err
	}
	f.Task = WindowTask{ID: t.ID, Title: t.Title, Agent: t.Provider, State: t.State, Folder: t.Folder, Model: t.Model, CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt, FinishedAt: t.FinishedAt}
	if t.OpenQuestion != nil && t.OpenQuestion.Audience == AudienceOwner {
		f.Task.OwnerQuestion = t.OpenQuestion
	}
	if t.State == "running" {
		if f.Task.PendingLaunch, err = s.pendingLaunch(t.ID); err != nil {
			return f, err
		}
	}
	if f.Task.StartedAt, err = s.startedAt(t.ID); err != nil {
		return f, err
	}
	if f.Task.Remote, err = s.remoteOf(t.ID); err != nil {
		return f, err
	}
	f.Items = []FeedItem{{Kind: "task", Author: AudienceCoordinator, Text: t.Text, At: t.CreatedAt, Verified: true}}
	if f.Task.Remote != nil && !f.Task.Remote.From {
		err = s.remoteFeed(&f)
		return f, err
	}
	err = s.tx(func(tx *sql.Tx) error {
		runs, err := tx.Query(`SELECT r.id,r.ordinal,r.kind,r.input,r.state,COALESCE(r.disposition,''),COALESCE(r.disposition_text,''),r.created_at,COALESCE(r.finished_at,''),r.detail,
 COALESCE((SELECT answered_by FROM task_questions q WHERE q.task_id=r.task_id AND q.answer=r.input AND q.state='answered' ORDER BY q.rowid DESC LIMIT 1),'coordinator')
 FROM task_runs r WHERE r.task_id=? ORDER BY r.ordinal`, id)
		if err != nil {
			return err
		}
		type runRow struct {
			id, kind, input, state, disposition, text, created, finished, detail, by string
			ordinal                                                                  int
		}
		var list []runRow
		for runs.Next() {
			var r runRow
			if err := runs.Scan(&r.id, &r.ordinal, &r.kind, &r.input, &r.state, &r.disposition, &r.text, &r.created, &r.finished, &r.detail, &r.by); err != nil {
				runs.Close()
				return err
			}
			list = append(list, r)
		}
		runs.Close()
		if err := runs.Err(); err != nil {
			return err
		}
		for _, r := range list {
			if r.kind == "answer" {
				f.Items = append(f.Items, FeedItem{Kind: "answer", Author: r.by, Text: r.input, At: r.created, Run: r.ordinal, Verified: true})
			}
			items, err := tx.Query(`SELECT kind,title,text,created_at FROM task_run_items WHERE run_id=? ORDER BY seq`, r.id)
			if err != nil {
				return err
			}
			for items.Next() {
				var it FeedItem
				if err := items.Scan(&it.Kind, &it.Title, &it.Text, &it.At); err != nil {
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
				f.Items = append(f.Items, FeedItem{Kind: kind, Author: "worker", Text: r.text, At: at, Run: r.ordinal, Verified: r.state == "finished", Audience: audience, State: r.state})
			}
			switch r.state {
			case "failed", "blocked", "interrupted", "cancelled":
				f.Items = append(f.Items, FeedItem{Kind: "outcome", Author: "messenger", Text: r.detail, At: r.finished, Run: r.ordinal, State: r.state, Verified: true})
			}
		}
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
	rows, err := s.db.Query(`SELECT kind,title,text,created_at FROM remote_task_items WHERE task_id=? ORDER BY seq`, f.Task.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		it := FeedItem{Author: "worker", Verified: true}
		if err := rows.Scan(&it.Kind, &it.Title, &it.Text, &it.At); err != nil {
			return err
		}
		f.Items = append(f.Items, it)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Events and progress lines come from two tables: show them in time
	// order. The task itself stays first.
	at := func(i int) time.Time { v, _ := time.Parse(time.RFC3339Nano, f.Items[i].At); return v }
	rest := f.Items[1:]
	sort.SliceStable(rest, func(i, j int) bool { return at(i + 1).Before(at(j + 1)) })
	return nil
}

func (s *Store) remoteEvents(f *TaskFeed) error {
	rows, err := s.db.Query(`SELECT audience,kind,text,created_at FROM task_events WHERE task_id=? ORDER BY seq`, f.Task.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var audience, kind, text, at string
		if err := rows.Scan(&audience, &kind, &text, &at); err != nil {
			return err
		}
		switch kind {
		case "question":
			f.Items = append(f.Items, FeedItem{Kind: "question", Author: "worker", Text: text, At: at, Verified: true, Audience: audience, State: "finished"})
		case "answered":
			by := AudienceCoordinator
			if audience == AudienceOwner {
				by = AudienceOwner
			}
			f.Items = append(f.Items, FeedItem{Kind: "answer", Author: by, Text: text, At: at, Verified: true})
		case "accepted":
			f.Items = append(f.Items, FeedItem{Kind: "accepted", Author: "messenger", Text: f.Task.Remote.Name, At: at, Verified: true})
		case "report":
			f.Items = append(f.Items, FeedItem{Kind: "report", Author: "worker", Text: text, At: at, Verified: true, State: "finished"})
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
		rows, err := tx.Query(`SELECT a.provider,
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='running'),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='waiting_owner'),
 a.models, a.limit_state
 FROM computer_agents a WHERE a.computer=? ORDER BY a.provider`, computer)
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
