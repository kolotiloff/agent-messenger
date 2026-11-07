package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/broker"
)

// serverNetworkTimeout bounds a connection attempt or check from the window.
// It stays below the window route deadline (22 s) and the HTTP server write
// timeout (25 s), so the window always gets an answer.
const serverNetworkTimeout = 18 * time.Second

// Server handles the window's server settings: the broker hosted by this
// client and this computer's connection to a broker.
func (s *Supervisor) Server(_ context.Context, action string, payload json.RawMessage) (any, error) {
	if s.server == nil {
		return nil, errors.New("сервер недоступен в этом клиенте")
	}
	var in struct {
		Scope   string `json:"scope"`
		Port    int    `json:"port"`
		Address string `json:"address"`
		Device  string `json:"device"`
		Code    string `json:"code"`
		Name    string `json:"name"`
	}
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &in); err != nil {
			return nil, errors.New("некорректный запрос")
		}
	}
	ctx, cancel := context.WithTimeout(s.ctx, serverNetworkTimeout)
	defer cancel()
	switch action {
	case "status":
	case "enable":
		if err := s.server.Enable(in.Scope, in.Port); err != nil {
			return nil, err
		}
	case "disable":
		if err := s.server.Disable(); err != nil {
			return nil, err
		}
	case "invite":
		return s.server.Invite(in.Address)
	case "revoke":
		if err := s.server.Revoke(in.Device); err != nil {
			return nil, err
		}
	case "connect":
		if err := s.server.Connect(ctx, in.Code, in.Name); err != nil {
			return nil, err
		}
	case "check":
		// The result is part of the status; a failed check is not an error
		// of the request.
		_ = s.server.CheckConnection(ctx)
	case "disconnect":
		if err := s.server.Disconnect(); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("неизвестное действие сервера")
	}
	return s.server.Status(), nil
}

// presence describes this computer's agents for its network: the agent
// programs with their running and waiting tasks, and the coordinator.
func (s *Supervisor) presence() (broker.Presence, error) {
	agents, coordinator, err := s.store.Presence()
	if err != nil {
		return broker.Presence{}, err
	}
	p := broker.Presence{Agents: []broker.PresenceAgent{}, Coordinator: clipName(coordinator)}
	for _, a := range agents {
		p.Agents = append(p.Agents, broker.PresenceAgent{Agent: a.Agent, Running: a.Running, Waiting: a.Waiting})
	}
	if name, err := os.Hostname(); err == nil {
		p.Name = clipName(name)
	}
	return p, nil
}

// clipName keeps a name within the broker's limit for device names.
func clipName(name string) string {
	runes := []rune(name)
	if len(runes) > broker.MaxDeviceName {
		runes = runes[:broker.MaxDeviceName]
	}
	return string(runes)
}
