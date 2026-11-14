package supervisor

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/buildinfo"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// Window read models and owner actions for the task window (T-029). None of
// them launches anything: tasks are launched by the client's own loop.

type windowAgent struct {
	core.WindowAgent
	Trust adapters.FolderTrust `json:"trust"`
}

type windowView struct {
	core.WindowView
	Agents       []windowAgent `json:"agents"`
	TasksEnabled bool          `json:"tasks_enabled"`
	Paused       bool          `json:"paused"`
	// Messenger and DataRoot build the coordinator connection command.
	Messenger string `json:"messenger"`
	DataRoot  string `json:"data_root"`
	// CoordinatorLink says whether Claude has the coordinator for the
	// project folder. CodexLink is the same for Codex (T-160).
	CoordinatorLink *CoordinatorLinkView `json:"coordinator_link,omitempty"`
	CodexLink       *CoordinatorLinkView `json:"codex_link,omitempty"`
	CursorLink      *CoordinatorLinkView `json:"cursor_link,omitempty"`
	// FeedbackCount is how many remarks the feedback journal holds (T-135).
	FeedbackCount int `json:"feedback_count"`
}

// WindowView returns the selected project's view with folder trust per agent.
func (s *Supervisor) WindowView(project string) (any, error) {
	y, m, d := time.Now().Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.Local).UTC().Format(time.RFC3339Nano)
	v, err := s.store.WindowView(project, today)
	if err != nil {
		return nil, err
	}
	out := windowView{WindowView: v, Agents: []windowAgent{}}
	home, _ := os.UserHomeDir()
	for _, a := range v.Agents {
		trust := adapters.FolderTrust{State: adapters.TrustUnknown}
		if v.Folder != "" {
			trust = adapters.CheckFolderTrust(a.Agent, v.Folder, a.Env, home)
		}
		out.Agents = append(out.Agents, windowAgent{WindowAgent: a, Trust: trust})
	}
	if l, ok := s.currentLauncher().(TaskLauncher); ok {
		out.TasksEnabled = l.TasksEnabled()
	}
	out.Paused, _ = s.store.Paused()
	out.Messenger, _ = workerProgram()
	out.DataRoot = s.root
	out.FeedbackCount, _ = s.store.FeedbackCount()
	if v.Project != "" {
		s.repairStaleCoordinator(v.Project)
		claude := s.CoordinatorLinkState(v.Project)
		codex := s.CodexCoordinatorLinkState(v.Project)
		out.CoordinatorLink = &claude
		out.CodexLink = &codex
		cursor := s.CursorCoordinatorLinkState(v.Project)
		out.CursorLink = &cursor
	}
	return out, nil
}

// TaskFeed returns one task's feed.
func (s *Supervisor) TaskFeed(id string) (any, error) { return s.store.TaskFeed(id) }

// CreateProject creates a project with its folder on this computer.
func (s *Supervisor) CreateProject(name, folder string) error {
	return s.store.CreateProject(name, folder)
}

// SetProjectFolder changes the project folder on this computer.
func (s *Supervisor) SetProjectFolder(name, folder string) error {
	return s.store.SetProjectFolder(name, folder)
}

// OwnerAnswer answers the owner's question of a task.
func (s *Supervisor) OwnerAnswer(task, question, op, text string) error {
	return s.store.WindowAnswer(task, question, op, text)
}

// OwnerNote keeps the owner's message to a working task; the worker gets it
// when its current run ends (T-085, T-148).
func (s *Supervisor) OwnerNote(task, op, text string) error {
	return s.store.AddOwnerNote(task, op, text)
}

// SetAgent registers an agent program for tasks with the worker profile.
func (s *Supervisor) SetAgent(provider, cli, environment string) error {
	return s.store.SetComputerAgent(provider, cli, environment, "worker")
}

// SetProjectWorkerRules stores the owner's rules for a project's workers (T-050).
func (s *Supervisor) SetProjectWorkerRules(name, rules string) error {
	return s.store.SetProjectWorkerRules(name, rules)
}

// SetProjectTitle renames a project in the window only (T-065).
func (s *Supervisor) SetProjectTitle(name, title string) error {
	return s.store.SetProjectTitle(name, title)
}

// RemoveProject takes a project out of the window (T-055).
func (s *Supervisor) RemoveProject(name string) error {
	return s.store.RemoveProject(name)
}

// SetAgentFullAccess turns «Разрешить любые команды» on or off (T-054).
// The provider "all" sets every agent of this computer at once (T-056).
// UsageStats is the window's token statistics for a period (T-068): "day"
// is today, "week" the last 7 days and "month" the last 30, in local days.
// computer is "" for every computer of the network, "local" or a peer's ID
// (T-076).
func (s *Supervisor) UsageStats(period, computer string) (any, error) {
	days := map[string]int{"day": 1, "week": 7, "month": 30}[period]
	if days == 0 {
		return nil, fmt.Errorf("unknown period %q", period)
	}
	y, m, d := time.Now().Date()
	since := time.Date(y, m, d-(days-1), 0, 0, 0, 0, time.Local)
	return s.store.UsageStats(since, time.Local, computer)
}

// SetAgentPaused turns an agent off or on from its card (T-061).
func (s *Supervisor) SetAgentPaused(provider string, paused bool) error {
	return s.store.SetAgentPaused(provider, paused)
}

func (s *Supervisor) SetAgentFullAccess(provider string, on bool) error {
	if provider == "all" {
		return s.store.SetAllAgentsFullAccess(on)
	}
	return s.store.SetAgentFullAccess(provider, on)
}

// SetAgentModels records the model names the owner listed for an agent (T-043).
func (s *Supervisor) SetAgentModels(provider string, models []string) error {
	return s.store.SetAgentModels(provider, models)
}

// RemoveAgent disconnects an agent from the window (T-046).
func (s *Supervisor) RemoveAgent(provider string) error {
	return s.store.RemoveComputerAgent(provider)
}

// HideNetTask removes a task from the window's list (T-131): another
// computer's task when peer is set, else a finished task of this computer.
func (s *Supervisor) HideNetTask(peer, task string) error {
	if peer == "" {
		return s.store.HideTask(task)
	}
	return s.store.HideNetTask(peer, task)
}

// CancelTask stops a task from the window's stop button. The running worker
// is signalled by the client's cancel watcher and verified like any run.
func (s *Supervisor) CancelTask(id string) error {
	_, err := s.store.CancelTask(id)
	if errors.Is(err, core.ErrWorkerMayRun) {
		return errors.New("Задача не закрыта: процесс исполнителя на этом компьютере, возможно, ещё работает или о нём нет данных. Задача осталась как была.")
	}
	return err
}

// AddFeedback records the owner's remark about the program (T-135).
func (s *Supervisor) AddFeedback(project, text, imageType string, image []byte) error {
	_, err := s.store.AddFeedback(core.AudienceOwner, project, text, buildinfo.Version, imageType, image)
	return err
}

// ExportFeedback writes the feedback journal into one zip file (T-135).
func (s *Supervisor) ExportFeedback() (string, int, error) {
	return s.store.ExportFeedback(core.FeedbackDir(s.root), time.Now())
}
