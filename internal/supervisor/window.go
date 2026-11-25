package supervisor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	ChangeWatch  bool          `json:"change_watch"`
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
	// FeedbackNew is how many of them were in no export yet: the next export
	// takes only those (owner, 2026-11-16).
	FeedbackNew int `json:"feedback_new"`
	// PendingUpdate is this computer's queued or installing Messenger
	// update (T-155): "queued" or "updating".
	PendingUpdate   string `json:"pending_update,omitempty"`
	PendingUpdateID string `json:"pending_update_id,omitempty"`
	// LocalUpdate is this computer's last update of itself, for the
	// window notice (T-155).
	LocalUpdate *core.NetUpdate `json:"local_update,omitempty"`
}

// WindowView returns the selected project's view with folder trust per agent.
func (s *Supervisor) WindowView(project string) (any, error) {
	return s.WindowViewFiltered(project, "", "", "")
}

func (s *Supervisor) WindowViewFiltered(project, agent, computer, version string) (any, error) {
	y, m, d := time.Now().Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.Local).UTC().Format(time.RFC3339Nano)
	v, err := s.store.WindowViewFiltered(project, today, agent, computer, version)
	if err != nil {
		return nil, err
	}
	out := windowView{WindowView: v, ChangeWatch: true, Agents: []windowAgent{}}
	home, _ := os.UserHomeDir()
	target := coordinatorTarget{folder: v.Folder}
	for _, a := range v.Agents {
		target.setCLI(a.Agent, a.CLI)
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
	out.FeedbackCount, out.FeedbackNew, _ = s.store.FeedbackCount()
	if pending, ok, err := s.store.PendingUpdate(); err == nil && ok {
		out.PendingUpdateID = pending.ID
		out.PendingUpdate = core.UpdateQueued
		if s.updateBusy() && pending.ID != "" {
			if _, err := os.Lstat(filepath.Join(s.root, remoteUpdateMarker)); err == nil {
				out.PendingUpdate = core.UpdateUpdating
			}
		}
	}
	if u, ok, err := s.store.LocalUpdate(); err == nil && ok {
		out.LocalUpdate = &u
	}
	if v.Project != "" {
		s.repairStaleCoordinator(v.Project)
		t, state, ok := s.resolveCoordinatorTarget(target)
		claude, codex, cursor := state, state, state
		if ok {
			claude = s.readCoordinatorLink(v.Project, t)
			codex = s.readCodexLink(v.Project, t)
			cursor = s.readCursorLink(v.Project, t)
		}
		out.CoordinatorLink = &claude
		out.CodexLink = &codex
		out.CursorLink = &cursor
	}
	return out, nil
}

func (s *Supervisor) WindowTaskHistory(project, before, agent, computer string) (any, error) {
	return s.store.WindowTaskHistory(project, before, agent, computer)
}

func (s *Supervisor) HideWindowHistory(project, agent, computer string) error {
	return s.store.HideWindowHistory(project, agent, computer)
}

// TaskFeed returns one task's feed.
func (s *Supervisor) TaskFeed(id string) (any, error) { return s.store.TaskFeed(id) }

func (s *Supervisor) TaskFeedDelta(id, cursor string) (any, error) {
	return s.store.TaskFeedDelta(id, cursor)
}

const windowWatchInterval = 250 * time.Millisecond

// WaitWindowChanges holds a lightweight request open until a committed
// change is visible. Writers may be independent MCP processes.
func (s *Supervisor) WaitWindowChanges(request context.Context, project, task, window, feed string) (any, error) {
	ctx, cancel := context.WithTimeout(request, 12*time.Second)
	defer cancel()
	stop := context.AfterFunc(s.ctx, cancel)
	defer stop()
	ticker := time.NewTicker(windowWatchInterval)
	defer ticker.Stop()
	current := core.WindowChanges{Window: window, Feed: feed}
	for {
		change, err := s.store.WindowChanges(ctx, project, task)
		if ctx.Err() != nil {
			return current, nil
		}
		if err != nil || change.Window != window || change.Feed != feed {
			return change, err
		}
		current = change
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return change, nil
		}
	}
}

// TaskImage is a worker's picture for the window (T-175).
func (s *Supervisor) TaskImage(id string) (any, error) {
	kind, data, err := s.store.TaskImageData(id)
	if err != nil {
		return nil, err
	}
	return struct {
		Type string `json:"type"`
		Data []byte `json:"data"`
	}{kind, data}, nil
}

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
	if err := s.store.WindowAnswer(task, question, op, text); err != nil {
		return err
	}
	s.wakeTaskDispatch()
	return nil
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
	return s.UsageStatsFiltered(period, computer, "", "")
}

// UsageStatsFiltered is UsageStats for one agent and one model (T-201).
func (s *Supervisor) UsageStatsFiltered(period, computer, agent, model string) (any, error) {
	return s.UsageStatsForProject("", period, computer, agent, model)
}

// UsageStatsForProject scopes the window statistics when a project is selected.
func (s *Supervisor) UsageStatsForProject(project, period, computer, agent, model string) (any, error) {
	if project != "" && !core.ValidProjectName(project) {
		return nil, fmt.Errorf("invalid project")
	}
	since, err := usagePeriodStart(period, time.Now())
	if err != nil {
		return nil, err
	}
	return s.store.UsageStatsFiltered(since, time.Local, computer, core.UsageFilter{Project: project, Agent: agent, Model: model})
}

// ExportUsage writes one project's filtered task history to a local JSON file.
func (s *Supervisor) ExportUsage(ctx context.Context, project, period, computer, agent, model string) (string, int, error) {
	at := time.Now()
	since, err := usagePeriodStart(period, at)
	if err != nil {
		return "", 0, err
	}
	return s.store.WriteUsageExport(ctx, filepath.Join(s.root, dbName), core.FeedbackDir(s.root), since, at, computer,
		core.UsageFilter{Project: project, Agent: agent, Model: model})
}

// UsageEfficiency returns comparable model groups on the user's request.
func (s *Supervisor) UsageEfficiency(ctx context.Context, project, period, computer, agent, model string) (any, error) {
	since, err := usagePeriodStart(period, time.Now())
	if err != nil {
		return nil, err
	}
	groups, err := s.store.UsageEfficiency(ctx, filepath.Join(s.root, dbName), since, computer,
		core.UsageFilter{Project: project, Agent: agent, Model: model})
	if err != nil {
		return nil, err
	}
	return map[string]any{"project": project, "groups": groups[:min(len(groups), 100)], "total_groups": len(groups)}, nil
}

func usagePeriodStart(period string, at time.Time) (time.Time, error) {
	days := map[string]int{"day": 1, "week": 7, "month": 30}[period]
	if days == 0 {
		return time.Time{}, fmt.Errorf("unknown period %q", period)
	}
	y, m, d := at.Date()
	return time.Date(y, m, d-(days-1), 0, 0, 0, 0, at.Location()), nil
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

// ClearFeedback deletes the remarks already exported (owner, 2026-11-16).
func (s *Supervisor) ClearFeedback() (int, int, error) {
	return s.store.ClearExportedFeedback()
}

// ExportFeedback writes the feedback journal into one zip file (T-135).
func (s *Supervisor) ExportFeedback() (string, int, error) {
	path, n, _, err := s.store.ExportFeedback(core.FeedbackDir(s.root), time.Now())
	return path, n, err
}
