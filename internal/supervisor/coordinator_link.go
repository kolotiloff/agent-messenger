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

// Coordinator connection from the window (T-039, D-050, T-160, D-123). For
// Claude the window asks the client to run `claude mcp add --scope local` in
// the project folder. For Codex it writes only the messenger table in the
// project's .codex/config.toml and does not run the Codex CLI. The result is
// read back from that agent's own config; an unclear result is shown, never
// retried.

// coordinatorAddTimeout bounds one `claude mcp add` run.
const coordinatorAddTimeout = 30 * time.Second

// Window states of the coordinator connection besides the adapters.Link*
// ones.
const (
	linkNoFolder = "no_folder" // the project has no folder on this computer
	linkNoClaude = "no_claude" // Claude Code is not connected under «Агенты»
	linkNoCodex  = "no_codex"  // Codex is not connected under «Агенты»
	linkFailed   = "failed"    // the last attempt did not leave the entry
)

// CoordinatorLinkView is the coordinator connection as the window shows it.
type CoordinatorLinkView struct {
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

var coordinatorConnect sync.Mutex

type coordinatorTarget struct {
	folder, cli, codexCLI, messenger, config string
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
		switch a.Agent {
		case "claude":
			t.cli = a.CLI
		case "codex":
			t.codexCLI = a.CLI
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

// ConnectCoordinator is the window's «Подключить» (control command
// coordinator-connect). agent is "claude" (the default) or "codex".
func (s *Supervisor) ConnectCoordinator(project, agent string) (any, error) {
	switch agent {
	case "", "claude":
		return s.connectCoordinator(project), nil
	case "codex":
		return s.connectCodexCoordinator(project), nil
	default:
		return CoordinatorLinkView{State: linkFailed, Detail: "Координатором может быть Claude Code или Codex."}, nil
	}
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

func (s *Supervisor) readCodexLink(project string, t coordinatorTarget) CoordinatorLinkView {
	state, err := adapters.CodexCoordinatorLink(adapters.CodexProjectConfig(t.folder), t.messenger, s.root, project)
	view := CoordinatorLinkView{State: state}
	if err != nil {
		view.Detail = err.Error()
	}
	if state == adapters.LinkMissing && t.codexCLI == "" {
		view.State = linkNoCodex
	}
	return view
}

// CodexCoordinatorLinkState reports whether the project folder's Codex
// config starts this Messenger. It changes nothing.
func (s *Supervisor) CodexCoordinatorLinkState(project string) CoordinatorLinkView {
	t, view, ok := s.coordinatorTarget(project)
	if !ok {
		return view
	}
	return s.readCodexLink(project, t)
}

// connectCodexCoordinator writes the messenger table into the project
// folder's .codex/config.toml (D-123). It does not run Codex.
func (s *Supervisor) connectCodexCoordinator(project string) CoordinatorLinkView {
	coordinatorConnect.Lock()
	defer coordinatorConnect.Unlock()
	t, view, ok := s.coordinatorTarget(project)
	if !ok {
		return view
	}
	before := s.readCodexLink(project, t)
	if before.State != adapters.LinkMissing && before.State != adapters.LinkStale {
		return before
	}
	// A registered Codex program is required before writing, as Claude is
	// required before its own command runs. A missing file with no program
	// is already no_codex; a stale file is left as it is.
	if t.codexCLI == "" {
		if before.State == adapters.LinkMissing {
			return CoordinatorLinkView{State: linkNoCodex}
		}
		return before
	}
	if err := adapters.EnsureCodexCoordinator(adapters.CodexProjectConfig(t.folder), t.messenger, s.root, project); err != nil {
		detail := err.Error()
		if len(detail) > 600 {
			detail = detail[:600] + "…"
		}
		return CoordinatorLinkView{State: linkFailed, Detail: detail}
	}
	after := s.readCodexLink(project, t)
	if after.State != adapters.LinkConnected {
		return CoordinatorLinkView{State: linkFailed, Detail: "Codex не записал подключение."}
	}
	return after
}

// staleRepairs remembers the projects whose stale coordinator entry this
// client already tried to replace: one try per project and client start, so
// a failure is not repeated by itself.
var staleRepairs sync.Map

// repairStaleCoordinator replaces, in the background, the project's
// coordinator entry when it starts an earlier program file (T-117, T-160).
// Claude and Codex are each tried once per client start. A chat that is
// open keeps its running program until it reconnects; new chats and
// reconnects start the program file.
func (s *Supervisor) repairStaleCoordinator(project string) {
	if _, tried := staleRepairs.LoadOrStore(s.root+"\x00"+project, true); tried {
		return
	}
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		t, _, ok := s.coordinatorTarget(project)
		if !ok {
			return
		}
		if t.cli != "" && s.readCoordinatorLink(project, t).State == adapters.LinkStale {
			s.connectCoordinator(project)
		}
		if t.codexCLI != "" && s.readCodexLink(project, t).State == adapters.LinkStale {
			s.connectCodexCoordinator(project)
		}
	}()
}
