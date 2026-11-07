package core

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
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
`

// Bounds on what the window keeps of a worker's event stream.
const (
	maxRunItems    = 400
	maxRunItemText = 4000
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
		_, err := tx.Exec(`INSERT INTO task_run_items(run_id,seq,kind,title,text,created_at) VALUES (?,?,?,?,?,?)`, run, n+1, kind, clip(title, 200), clip(text, maxRunItemText), now())
		return err
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
		v.Name, _ = os.Hostname()
		rows, err := tx.Query(`SELECT a.provider,a.cli_path,a.profile,a.environment,
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='running'),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='waiting_owner'),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='running' AND t.project=?),
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.project=? AND t.created_at>=?)
 FROM computer_agents a WHERE a.computer=? ORDER BY a.provider`, project, project, today, computer)
		if err != nil {
			return err
		}
		for rows.Next() {
			var a WindowAgent
			if err := rows.Scan(&a.Agent, &a.CLI, &a.Profile, &a.Env, &a.Running, &a.Waiting, &a.ProjectRunning, &a.ProjectToday); err != nil {
				rows.Close()
				return err
			}
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
	f.Items = []FeedItem{{Kind: "task", Author: AudienceCoordinator, Text: t.Text, At: t.CreatedAt, Verified: true}}
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
 (SELECT COUNT(*) FROM tasks t WHERE t.computer=a.computer AND t.provider=a.provider AND t.state='waiting_owner')
 FROM computer_agents a WHERE a.computer=? ORDER BY a.provider`, computer)
		if err != nil {
			return err
		}
		for rows.Next() {
			var a AgentPresence
			if err := rows.Scan(&a.Agent, &a.Running, &a.Waiting); err != nil {
				rows.Close()
				return err
			}
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
