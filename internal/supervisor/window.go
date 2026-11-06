package supervisor

import (
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
