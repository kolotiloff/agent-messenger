package supervisor

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/adapters"
	"github.com/kolotiloff/agent-messenger/internal/platform"
)

// Coordinator connection from the window (T-039, D-050). The window asks the
// client to run Claude's own `claude mcp add --scope local` in the project
// folder, so the owner never types a command. The result is read back from
// Claude's settings; an unclear result is shown, never retried.

// coordinatorAddTimeout bounds one `claude mcp add` run.
const coordinatorAddTimeout = 30 * time.Second

// Window states of the coordinator connection besides the adapters.Link*
// ones.
const (
	linkNoFolder = "no_folder" // the project has no folder on this computer
	linkNoClaude = "no_claude" // Claude Code is not connected under «Агенты»
	linkFailed   = "failed"    // the last attempt did not leave the entry
)

// CoordinatorLinkView is the coordinator connection as the window shows it.
type CoordinatorLinkView struct {
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

var coordinatorConnect sync.Mutex

type coordinatorTarget struct {
	folder, cli, messenger, config string
}

func (s *Supervisor) coordinatorTarget(project string) (coordinatorTarget, CoordinatorLinkView, bool) {
	v, err := s.store.WindowView(project, "")
	if err != nil {
		return coordinatorTarget{}, CoordinatorLinkView{State: adapters.LinkUnknown, Detail: err.Error()}, false
	}
	if v.Folder == "" {
		return coordinatorTarget{}, CoordinatorLinkView{State: linkNoFolder}, false
	}
	t := coordinatorTarget{folder: v.Folder}
	for _, a := range v.Agents {
		if a.Agent == "claude" {
			t.cli = a.CLI
		}
	}
	t.messenger, err = CoordinatorProgram(s.root)
	if err != nil {
		return t, CoordinatorLinkView{State: adapters.LinkUnknown, Detail: err.Error()}, false
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return t, CoordinatorLinkView{State: adapters.LinkUnknown, Detail: err.Error()}, false
	}
	t.config = adapters.ClaudeUserConfig(home, os.Environ())
	return t, CoordinatorLinkView{}, true
}

func (s *Supervisor) readCoordinatorLink(project string, t coordinatorTarget) CoordinatorLinkView {
	state, err := adapters.CoordinatorLink(t.config, t.folder, t.messenger, s.root, project)
	view := CoordinatorLinkView{State: state}
	if err != nil {
		view.Detail = err.Error()
	}
	if state == adapters.LinkMissing && t.cli == "" {
		view.State = linkNoClaude
	}
	return view
}

// CoordinatorLinkState reports, without changing anything, whether the
// project's folder is connected to this Messenger in Claude.
func (s *Supervisor) CoordinatorLinkState(project string) CoordinatorLinkView {
	t, view, ok := s.coordinatorTarget(project)
	if !ok {
		return view
	}
	return s.readCoordinatorLink(project, t)
}

// ConnectCoordinator is the window's «Подключить к Claude» (control
// command coordinator-connect).
func (s *Supervisor) ConnectCoordinator(project string) (any, error) {
	return s.connectCoordinator(project), nil
}

// connectCoordinator connects the project's folder in Claude. It does
// nothing when the entry is already there, and it never replaces a
// "messenger" entry that starts something else.
func (s *Supervisor) connectCoordinator(project string) CoordinatorLinkView {
	coordinatorConnect.Lock()
	defer coordinatorConnect.Unlock()
	t, view, ok := s.coordinatorTarget(project)
	if !ok {
		return view
	}
	before := s.readCoordinatorLink(project, t)
	if before.State != adapters.LinkMissing && before.State != adapters.LinkStale {
		return before
	}
	ctx, cancel := context.WithTimeout(context.Background(), coordinatorAddTimeout)
	defer cancel()
	var out bytes.Buffer
	run := func(args []string) error {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		cmd.Dir = t.folder
		cmd.WaitDelay = time.Second
		platform.HideConsole(cmd)
		cmd.Stdout, cmd.Stderr = &out, &out
		return cmd.Run()
	}
	var runErr error
	if before.State == adapters.LinkStale {
		// Our own entry for an earlier program file (T-117): Claude's add
		// does not replace an entry, so it is removed first.
		runErr = run(adapters.ClaudeRemoveCoordinatorArgs(t.cli))
	}
	if runErr == nil {
		runErr = run(adapters.ClaudeAddCoordinatorArgs(t.cli, t.messenger, s.root, project))
	}
	after := s.readCoordinatorLink(project, t)
	if after.State == adapters.LinkConnected {
		return after
	}
	// Unknown is not success, and nothing is retried: the window shows what
	// happened and offers the manual command.
	detail := strings.TrimSpace(out.String())
	if len(detail) > 600 {
		detail = detail[:600] + "…"
	}
	switch {
	case ctx.Err() != nil:
		detail = "Claude не ответил за 30 секунд. " + detail
	case runErr != nil && !errors.As(runErr, new(*exec.ExitError)):
		detail = runErr.Error()
	}
	if detail == "" {
		detail = "Claude не сообщил об ошибке, но подключения в его настройках нет."
	}
	return CoordinatorLinkView{State: linkFailed, Detail: detail}
}

// staleRepairs remembers the projects whose stale coordinator entry this
// client already tried to replace: one try per project and client start, so
// a failure is not repeated by itself.
var staleRepairs sync.Map

// repairStaleCoordinator replaces, in the background, the project's
// coordinator entry when it starts an earlier program file (T-117). A chat
// that is open keeps its running program until it reconnects; new chats
// and reconnects start the program file.
func (s *Supervisor) repairStaleCoordinator(project string) {
	if _, tried := staleRepairs.LoadOrStore(s.root+"\x00"+project, true); tried {
		return
	}
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		t, _, ok := s.coordinatorTarget(project)
		if !ok || t.cli == "" || s.readCoordinatorLink(project, t).State != adapters.LinkStale {
			return
		}
		s.connectCoordinator(project)
	}()
}
