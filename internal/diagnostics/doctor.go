// Package diagnostics separates passive discovery from explicitly requested
// provider status subcommands. Neither operation opens a model conversation.
package diagnostics

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/control"
)

type CLI struct {
	State    string `json:"state"`
	Path     string `json:"path,omitempty"`
	Command  string `json:"command,omitempty"`
	Source   string `json:"source"`
	Identity string `json:"identity"`
	Version  string `json:"version"`
}

type Agent struct {
	Provider       string `json:"provider"`
	Name           string `json:"name"`
	CLI            CLI    `json:"cli"`
	Authentication string `json:"authentication"`
	Session        string `json:"session"`
}

type Controller struct {
	State  string          `json:"state"`
	Reason string          `json:"reason,omitempty"`
	Status *control.Status `json:"status,omitempty"`
}

type Data struct {
	Root  string `json:"root"`
	State string `json:"state"`
}

type Report struct {
	SchemaVersion int        `json:"schema_version"`
	CheckedAt     string     `json:"checked_at"`
	OS            string     `json:"os"`
	Arch          string     `json:"arch"`
	Data          Data       `json:"data"`
	Controller    Controller `json:"controller"`
	Agents        []Agent    `json:"agents"`
	// This counts only CLI processes started by this diagnostic operation.
	ProbeCLIStarts int `json:"probe_cli_starts"`
}

var providers = []struct {
	id, name string
	commands []string
}{
	{"claude", "Claude Code", []string{"claude"}},
	{"codex", "Codex CLI", []string{"codex"}},
	{"cursor", "Cursor Agent CLI", []string{"cursor-agent", "agent"}},
}

// Collect only consults PATH, file metadata and the existing local status API.
// Finding a candidate executable does not confirm its identity or login state.
func Collect(ctx context.Context, root string) Report {
	absolute, err := filepath.Abs(root)
	if err != nil {
		absolute = root
	}
	r := Report{SchemaVersion: 1, CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
		OS: runtime.GOOS, Arch: runtime.GOARCH, Data: Data{Root: absolute}, Agents: []Agent{}}
	info, statErr := os.Stat(absolute)
	switch {
	case os.IsNotExist(statErr):
		r.Data.State = "missing"
	case statErr != nil:
		r.Data.State = "unreadable"
	case !info.IsDir():
		r.Data.State = "not_directory"
	default:
		r.Data.State = "exists"
	}
	for _, p := range providers {
		agent := Agent{Provider: p.id, Name: p.name, Authentication: "not_checked", Session: "not_checked",
			CLI: CLI{State: "not_found", Source: "PATH", Identity: "not_checked", Version: "not_checked"}}
		for _, command := range p.commands {
			path, err := exec.LookPath(command)
			if err != nil {
				if errors.Is(err, exec.ErrDot) {
					agent.CLI.State = "unsafe_relative_path"
				}
				continue
			}
			path, err = filepath.Abs(path)
			if err != nil {
				agent.CLI.State = "path_error"
				continue
			}
			agent.CLI.State, agent.CLI.Path, agent.CLI.Command = "candidate_found", path, command
			break
		}
		r.Agents = append(r.Agents, agent)
	}
	// Even a caller with no deadline cannot leave doctor blocked indefinitely.
	probe, cancel := context.WithTimeout(ctx, 750*time.Millisecond)
	defer cancel()
	response, callErr := control.CallContext(probe, absolute, control.Request{Cmd: "status"})
	switch {
	case callErr != nil:
		r.Controller = Controller{State: "unreachable", Reason: "connection_failed"}
		if probe.Err() != nil {
			r.Controller.Reason = "deadline_or_cancellation"
		} else if os.IsNotExist(callErr) {
			r.Controller.Reason = "socket_missing"
		}
	case !response.OK || response.Status == nil:
		r.Controller = Controller{State: "unknown", Reason: "invalid_or_failed_status"}
	default:
		// Copy only status fields, never Inspect, provider output or error text.
		status := *response.Status
		status.Detail = ""
		r.Controller = Controller{State: "reachable", Status: &status}
	}
	return r
}

func WriteText(w io.Writer, r Report) {
	fmt.Fprintf(w, "Диагностика Messenger (%s/%s)\n", r.OS, r.Arch)
	if r.Controller.State == "reachable" {
		s := r.Controller.Status
		fmt.Fprintf(w, "Клиент доступен: pid=%d, пауза=%t, вызовы CLI=%s, очередь=%d, занятых сессий=%d.\n", s.Supervisor, s.Paused, s.CLILaunch, s.Queued, s.Held)
	} else {
		fmt.Fprintln(w, "Клиент не ответил достоверным статусом. Это не доказывает остановку CLI и не снимает блокировки.")
	}
	fmt.Fprintf(w, "Каталог данных: %s (%s).\n", r.Data.Root, r.Data.State)
	for _, a := range r.Agents {
		if a.CLI.State == "candidate_found" {
			fmt.Fprintf(w, "%s: найден исполняемый файл %s; подлинность/версия и вход не проверены.\n", a.Name, a.CLI.Path)
		} else {
			fmt.Fprintf(w, "%s: исполняемый файл в PATH не подтверждён (%s).\n", a.Name, a.CLI.State)
		}
	}
	fmt.Fprintln(w, "Эта проверка не запускает CLI, не читает учётные данные и не подтверждает готовность сессий. Для отдельной проверки версии и входа выбранного CLI используйте check-cli.")
}
