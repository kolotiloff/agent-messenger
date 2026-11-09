// Package control is the local command API between the window, the command
// line and the background client. It does not launch CLI processes itself.
package control

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const SocketName = "control.sock"

// MaxRequestBytes bounds one request line. The largest legitimate request
// carries one message text (at most 256 KiB in the broker); the rest is
// small fields, so 4 MiB leaves ample room for JSON escaping.
const MaxRequestBytes = 4 << 20

type Request struct {
	Cmd            string `json:"cmd"`
	Alias          string `json:"alias,omitempty"`
	MessageID      string `json:"message_id,omitempty"`
	Sender         string `json:"sender,omitempty"`
	Text           string `json:"text,omitempty"`
	Action         string `json:"action,omitempty"`
	Confirmation   string `json:"confirmation,omitempty"`
	Provider       string `json:"provider,omitempty"`
	Environment    string `json:"environment,omitempty"`
	ConversationID string `json:"conversation_id,omitempty"`
	CLI            string `json:"cli,omitempty"`
	Workspace      string `json:"workspace,omitempty"`
}

type Status struct {
	Paused bool `json:"paused"`
	// CLILaunch is "tasks" when this client starts task workers and
	// "disabled" when it only records tasks.
	CLILaunch    string `json:"cli_launch"`
	Broker       string `json:"broker"`
	Supervisor   int    `json:"supervisor_pid"`
	Detail       string `json:"detail,omitempty"`
	TasksEnabled bool   `json:"tasks_enabled,omitempty"`
	// Version is the client build; clients before preview.27 leave it empty.
	Version string `json:"version,omitempty"`
	// Reservations are the CLI reservations in the user's registry
	// (active-runs.json), read without changing it.
	Reservations *Reservations `json:"reservations,omitempty"`
}

// Reservations summarises active-runs.json. A task run releases its own
// reservation once the run is proven finished (D-021). Old reservations were
// left by the removed persistent mode (T-028, D-104); nothing in this version
// releases them, so they are listed for the owner to see.
type Reservations struct {
	Tasks int              `json:"tasks"`
	Old   []OldReservation `json:"old,omitempty"`
	// Error is set when the registry could not be read; the counts are then
	// unknown, not zero.
	Error string `json:"error,omitempty"`
}

// OldReservation is one reservation of the removed persistent mode.
type OldReservation struct {
	// Kind is "claude-stream" (a persistent Claude process), "codex-app-server"
	// (a Codex owner process) or "session" (a run of a registered session).
	Kind     string `json:"kind"`
	Provider string `json:"provider,omitempty"`
	Root     string `json:"root"`
	PID      int    `json:"pid,omitempty"`
}

type Response struct {
	OK       bool            `json:"ok"`
	Error    string          `json:"error,omitempty"`
	Status   *Status         `json:"status,omitempty"`
	Overview json.RawMessage `json:"overview,omitempty"`
}

type Handler interface {
	Status() (Status, error)
	Pause() error
	Resume() error
	Shutdown() error
}

// SocketPath keeps the shorter lexical spelling for Unix socket length limits.
// It does not resolve symlinks or define ownership; the supervisor does that.
func SocketPath(root string) string {
	path := filepath.Join(root, SocketName)
	if absolute, err := filepath.Abs(path); err == nil && len(absolute) < len(path) {
		return absolute
	}
	return path
}

// Call sends one command to the running client. It never launches a CLI.
func Call(root string, request Request) (Response, error) {
	response, err := CallContext(context.Background(), root, request)
	return response, err
}

// CallContext lets diagnostic/front-end callers bound an unresponsive socket.
// It never turns a timeout into proof that the supervisor or a CLI stopped.
func CallContext(ctx context.Context, root string, request Request) (Response, error) {
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", SocketPath(root))
	if err != nil {
		return Response{}, fmt.Errorf("клиент недоступен: %w", err)
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	defer stop()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return Response{}, err
		}
	}
	return exchange(conn, request)
}

func Serve(ln net.Listener, handler Handler) error {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		go func(conn net.Conn) {
			defer conn.Close()
			// One byte past the limit is enough to tell a long line apart.
			reader := bufio.NewReader(io.LimitReader(conn, MaxRequestBytes+1))
			line, err := reader.ReadBytes('\n')
			if err != nil && !errors.Is(err, io.EOF) {
				return
			}
			if len(bytes.TrimSuffix(line, []byte("\n"))) > MaxRequestBytes {
				writeResponse(conn, Response{OK: false, Error: "команда слишком длинная"})
				return
			}
			var request Request
			if err := json.Unmarshal(line, &request); err != nil {
				writeResponse(conn, Response{OK: false, Error: "некорректная команда"})
				return
			}
			writeResponse(conn, dispatch(handler, request))
		}(conn)
	}
}

func dispatch(handler Handler, request Request) Response {
	switch request.Cmd {
	case "window-view", "task-feed":
		reader, ok := handler.(interface {
			WindowView(string) (any, error)
			TaskFeed(string) (any, error)
		})
		if !ok {
			return Response{Error: "tasks unsupported; update the background client"}
		}
		var value any
		var err error
		if request.Cmd == "window-view" {
			value, err = reader.WindowView(request.Alias)
		} else {
			value, err = reader.TaskFeed(request.MessageID)
		}
		if err != nil {
			return Response{Error: err.Error()}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return Response{Error: "view unavailable"}
		}
		return Response{OK: true, Overview: raw}
	case "usage-stats":
		h, ok := handler.(interface {
			UsageStats(period, computer string) (any, error)
		})
		if !ok {
			return Response{Error: "statistics unsupported; update the background client"}
		}
		value, err := h.UsageStats(request.Alias, request.Text)
		if err != nil {
			return Response{Error: err.Error()}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return Response{Error: "statistics unavailable"}
		}
		return Response{OK: true, Overview: raw}
	case "coordinator-connect":
		h, ok := handler.(interface {
			ConnectCoordinator(string) (any, error)
		})
		if !ok {
			return Response{Error: "coordinator connection unsupported; update the background client"}
		}
		value, err := h.ConnectCoordinator(request.Alias)
		if err != nil {
			return Response{Error: err.Error()}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return Response{Error: "result unavailable"}
		}
		return Response{OK: true, Overview: raw}
	case "project-create", "project-folder", "task-owner-answer", "agent-set", "task-cancel":
		h, ok := handler.(interface {
			CreateProject(string, string) error
			SetProjectFolder(string, string) error
			OwnerAnswer(string, string, string, string) error
			SetAgent(string, string, string) error
			CancelTask(string) error
		})
		if !ok {
			return Response{Error: "tasks unsupported; update the background client"}
		}
		var err error
		switch request.Cmd {
		case "project-create":
			err = h.CreateProject(request.Alias, request.Workspace)
		case "project-folder":
			err = h.SetProjectFolder(request.Alias, request.Workspace)
		case "task-owner-answer":
			err = h.OwnerAnswer(request.MessageID, request.ConversationID, request.Confirmation, request.Text)
		case "agent-set":
			err = h.SetAgent(request.Provider, request.CLI, request.Environment)
		case "task-cancel":
			err = h.CancelTask(request.MessageID)
		}
		if err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	case "task-owner-note":
		h, ok := handler.(interface {
			OwnerNote(string, string, string) error
		})
		if !ok {
			return Response{Error: "messages to working tasks unsupported; update the background client"}
		}
		if err := h.OwnerNote(request.MessageID, request.Confirmation, request.Text); err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	case "agent-remove":
		h, ok := handler.(interface{ RemoveAgent(string) error })
		if !ok {
			return Response{Error: "disconnecting agents unsupported; update the background client"}
		}
		if err := h.RemoveAgent(request.Provider); err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	case "project-rules":
		h, ok := handler.(interface {
			SetProjectWorkerRules(string, string) error
		})
		if !ok {
			return Response{Error: "worker rules unsupported; update the background client"}
		}
		if err := h.SetProjectWorkerRules(request.Alias, request.Text); err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	case "project-title":
		h, ok := handler.(interface {
			SetProjectTitle(string, string) error
		})
		if !ok {
			return Response{Error: "project rename unsupported; update the background client"}
		}
		if err := h.SetProjectTitle(request.Alias, request.Text); err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	case "project-remove":
		h, ok := handler.(interface {
			RemoveProject(string) error
		})
		if !ok {
			return Response{Error: "project removal unsupported; update the background client"}
		}
		if err := h.RemoveProject(request.Alias); err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	case "agent-pause":
		h, ok := handler.(interface {
			SetAgentPaused(string, bool) error
		})
		if !ok {
			return Response{Error: "agent switch unsupported; update the background client"}
		}
		if request.Text != "0" && request.Text != "1" {
			return Response{Error: "agent-pause needs 0 or 1"}
		}
		if err := h.SetAgentPaused(request.Provider, request.Text == "1"); err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	case "agent-access":
		h, ok := handler.(interface {
			SetAgentFullAccess(string, bool) error
		})
		if !ok {
			return Response{Error: "full access unsupported; update the background client"}
		}
		if request.Text != "0" && request.Text != "1" {
			return Response{Error: "agent-access needs 0 or 1"}
		}
		if err := h.SetAgentFullAccess(request.Provider, request.Text == "1"); err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	case "agent-models":
		h, ok := handler.(interface {
			SetAgentModels(string, []string) error
		})
		if !ok {
			return Response{Error: "model lists unsupported; update the background client"}
		}
		var models []string
		if request.Text != "" {
			models = strings.Split(request.Text, "\n")
		}
		if err := h.SetAgentModels(request.Provider, models); err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true}
	case "server":
		h, ok := handler.(interface {
			Server(context.Context, string, json.RawMessage) (any, error)
		})
		if !ok {
			return Response{Error: "неизвестная команда"}
		}
		var payload json.RawMessage
		if request.Text != "" {
			payload = json.RawMessage(request.Text)
		}
		value, err := h.Server(context.Background(), request.Action, payload)
		if err != nil {
			return Response{Error: err.Error()}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return Response{Error: "server status unavailable"}
		}
		return Response{OK: true, Overview: raw}
	case "status":
		status, err := handler.Status()
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Status: &status}
	case "pause":
		if err := handler.Pause(); err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true}
	case "resume":
		if err := handler.Resume(); err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true}
	case "shutdown":
		if err := handler.Shutdown(); err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true}
	default:
		return Response{OK: false, Error: "неизвестная команда"}
	}
}

func exchange(conn net.Conn, request Request) (Response, error) {
	encoded, err := json.Marshal(request)
	if err != nil {
		return Response{}, err
	}
	if _, err := fmt.Fprintf(conn, "%s\n", encoded); err != nil {
		return Response{}, err
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return Response{}, err
	}
	var response Response
	if err := json.Unmarshal(line, &response); err != nil {
		return Response{}, err
	}
	return response, nil
}

func writeResponse(conn net.Conn, response Response) {
	encoded, _ := json.Marshal(response)
	fmt.Fprintf(conn, "%s\n", encoded)
}

// Listen creates the command socket. The caller must already hold the supervisor lock.
func Listen(root string) (net.Listener, error) {
	path := SocketPath(root)
	_ = os.Remove(path)
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	// On Windows the socket file takes the data directory's access list (the
	// user's own folder); file modes do not apply there.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o600); err != nil {
			ln.Close()
			return nil, err
		}
	}
	return ln, nil
}
