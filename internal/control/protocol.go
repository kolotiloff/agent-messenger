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
	SessionSetup bool   `json:"session_setup,omitempty"`
	StreamState  string `json:"stream_state,omitempty"`
	Paused       bool   `json:"paused"`
	CLILaunch    string `json:"cli_launch"`
	Broker       string `json:"broker"`
	Held         int    `json:"held_sessions"`
	Queued       int    `json:"queued"`
	Supervisor   int    `json:"supervisor_pid"`
	Detail       string `json:"detail,omitempty"`
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
	if err := os.Chmod(path, 0o600); err != nil {
		ln.Close()
		return nil, err
	}
	return ln, nil
}
