package supervisor

import (
	"fmt"
	"os"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
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
	// project folder; filled only while no coordinator chat is connected.
	CoordinatorLink *CoordinatorLinkView `json:"coordinator_link,omitempty"`
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
	out.Messenger, _ = os.Executable()
	out.DataRoot = s.root
	if v.Project != "" && !v.Coordinator.Connected {
		link := s.CoordinatorLinkState(v.Project)
		out.CoordinatorLink = &link
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
func (s *Supervisor) UsageStats(period string) (any, error) {
	days := map[string]int{"day": 1, "week": 7, "month": 30}[period]
	if days == 0 {
		return nil, fmt.Errorf("unknown period %q", period)
	}
	y, m, d := time.Now().Date()
	since := time.Date(y, m, d-(days-1), 0, 0, 0, 0, time.Local)
	return s.store.UsageStats(since, time.Local)
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

// CancelTask stops a task from the window's stop button. The running worker
// is signalled by the client's cancel watcher and verified like any run.
func (s *Supervisor) CancelTask(id string) error {
	_, err := s.store.CancelTask(id)
	return err
}
