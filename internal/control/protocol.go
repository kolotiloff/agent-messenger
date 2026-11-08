// Package control is the local command API. A future window uses this socket
// and does not launch CLI processes itself.
package control

import (
	"bufio"
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
	Limit          int    `json:"limit,omitempty"`
	Before         string `json:"before,omitempty"`
}

type Status struct {
	SessionLaunch  bool     `json:"session_launch,omitempty"`
	LaunchSession  string   `json:"launch_session,omitempty"`
	LaunchSessions []string `json:"launch_sessions,omitempty"`
	SessionSetup   bool     `json:"session_setup,omitempty"`
	StreamState    string   `json:"stream_state,omitempty"`
	Paused         bool     `json:"paused"`
	CLILaunch      string   `json:"cli_launch"`
	Broker         string   `json:"broker"`
	Held           int      `json:"held_sessions"`
	Queued         int      `json:"queued"`
	Supervisor     int      `json:"supervisor_pid"`
	Detail         string   `json:"detail,omitempty"`
	TasksEnabled   bool     `json:"tasks_enabled,omitempty"`
	// Version is the client build; clients before preview.27 leave it empty.
	Version string `json:"version,omitempty"`
}

type StopResult struct {
	State  string `json:"state"`
	Signal string `json:"signal"`
	Note   string `json:"note"`
}

type Response struct {
	Outbox   json.RawMessage `json:"outbox,omitempty"`
	OK       bool            `json:"ok"`
	Error    string          `json:"error,omitempty"`
	Status   *Status         `json:"status,omitempty"`
	Stop     *StopResult     `json:"stop,omitempty"`
	Created  *bool           `json:"created,omitempty"`
	Inspect  json.RawMessage `json:"inspect,omitempty"`
	Overview json.RawMessage `json:"overview,omitempty"`
	Timeline json.RawMessage `json:"timeline,omitempty"`
}

type Handler interface {
	Status() (Status, error)
	Pause() error
	Resume() error
	StopJob() (StopResult, error)
	Shutdown() error
	Enqueue(alias, id, sender, text string) (bool, error)
	Inspect() (any, error)
	Resolve(alias, action, confirmation string) (string, error)
	RegisterSession(provider, environment, conversationID, alias, cliPath, workspace string) (string, error)
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
			reader := bufio.NewReader(conn)
			line, err := reader.ReadBytes('\n')
			if err != nil && !errors.Is(err, io.EOF) {
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
	case "authorize-setup":
		h, ok := handler.(interface{ AuthorizeSetup(string) error })
		if !ok || request.Confirmation != "allow-model-calls" {
			return Response{Error: "explicit test-session authorization required"}
		}
		if err := h.AuthorizeSetup(request.Alias); err != nil {
			return Response{Error: "test-session authorization rejected"}
		}
		return Response{OK: true}
	case "setup-new-claude":
		h, ok := handler.(interface {
			SetupNewClaude(string, string, string, string, string) error
		})
		if !ok {
			return Response{Error: "setup unsupported; update the background client"}
		}
		if err := h.SetupNewClaude(request.Environment, request.ConversationID, request.Alias, request.CLI, request.Workspace); err != nil {
			return Response{Error: "session setup rejected; existing sessions unchanged"}
		}
		return Response{OK: true}
	case "ui-prepare", "ui-submit", "ui-ack", "ui-outbox":
		h, ok := handler.(interface {
			PrepareUIMessage(string, string, string) error
			SubmitUIMessage(string) (bool, error)
			AcknowledgeUIMessage(string) error
			UIOutbox() (any, error)
		})
		if !ok {
			return Response{Error: "UI outbox unsupported"}
		}
		var err error
		switch request.Cmd {
		case "ui-prepare":
			err = h.PrepareUIMessage(request.MessageID, request.Alias, request.Text)
		case "ui-submit":
			var created bool
			created, err = h.SubmitUIMessage(request.MessageID)
			if err == nil {
				return Response{OK: true, Created: &created}
			}
		case "ui-ack":
			err = h.AcknowledgeUIMessage(request.MessageID)
		case "ui-outbox":
			var value any
			value, err = h.UIOutbox()
			if err == nil {
				var raw []byte
				raw, err = json.Marshal(value)
				if err == nil {
					return Response{OK: true, Outbox: raw}
				}
			}
		}
		if err != nil {
			return Response{Error: "UI outbox operation rejected"}
		}
		return Response{OK: true}
	case "timeline":
		reader, ok := handler.(interface {
			Timeline(string, int, string) (any, error)
		})
		if !ok {
			return Response{Error: "timeline unsupported"}
		}
		limit := request.Limit
		if limit == 0 {
			limit = 50
		}
		if request.Alias == "" || limit < 1 || limit > 100 {
			return Response{Error: "timeline requires alias and limit 1..100"}
		}
		value, err := reader.Timeline(request.Alias, limit, request.Before)
		if err != nil {
			return Response{Error: "timeline unavailable; check alias and cursor"}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return Response{Error: "timeline unavailable"}
		}
		return Response{OK: true, Timeline: raw}
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
	case "overview":
		reader, ok := handler.(interface{ Overview() (any, error) })
		if !ok {
			return Response{Error: "overview unsupported"}
		}
		value, err := reader.Overview()
		if err != nil {
			return Response{Error: "overview unavailable"}
		}
		raw, err := json.Marshal(value)
		if err != nil {
			return Response{Error: "overview unavailable"}
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
	case "stop-job":
		result, err := handler.StopJob()
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Stop: &result}
	case "shutdown":
		if err := handler.Shutdown(); err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true}
	case "enqueue":
		created, err := handler.Enqueue(request.Alias, request.MessageID, request.Sender, request.Text)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Created: &created}
	case "inspect":
		value, err := handler.Inspect()
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		raw, _ := json.Marshal(value)
		return Response{OK: true, Inspect: raw}
	case "resolve":
		detail, err := handler.Resolve(request.Alias, request.Action, request.Confirmation)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Status: &Status{Detail: detail}}
	case "create-claude-session":
		creator, ok := handler.(interface {
			CreateClaudeSession(string, string, string, string) (string, error)
		})
		if !ok {
			return Response{OK: false, Error: "unsupported"}
		}
		detail, err := creator.CreateClaudeSession(request.Environment, request.Alias, request.CLI, request.Workspace)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Status: &Status{Detail: detail}}
	case "setup-codex":
		h, ok := handler.(interface {
			SetupCodexFromWindow(string, string, string, string) (string, error)
		})
		if !ok {
			return Response{Error: "Codex setup unsupported; update the background client"}
		}
		detail, err := h.SetupCodexFromWindow(request.Environment, request.Alias, request.CLI, request.Confirmation)
		if err != nil {
			return Response{Error: err.Error()}
		}
		return Response{OK: true, Status: &Status{Detail: detail}}
	case "create-codex-session":
		creator, ok := handler.(interface {
			CreateCodexSession(string, string, string) (string, error)
		})
		if !ok {
			return Response{OK: false, Error: "unsupported; update the background client"}
		}
		detail, err := creator.CreateCodexSession(request.Environment, request.Alias, request.CLI)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Status: &Status{Detail: detail}}
	case "register-session":
		key, err := handler.RegisterSession(request.Provider, request.Environment, request.ConversationID, request.Alias, request.CLI, request.Workspace)
		if err != nil {
			return Response{OK: false, Error: err.Error()}
		}
		return Response{OK: true, Status: &Status{Detail: key}}
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
