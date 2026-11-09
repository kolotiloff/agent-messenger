package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/broker"
	"github.com/kolotiloff/agent-messenger/internal/core"
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
		On      bool   `json:"on"`
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
		// The name given when joining is this computer's name in the network.
		if strings.TrimSpace(in.Name) == "" {
			break
		}
		if err := s.store.SetComputerName(in.Name); err != nil {
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
	case "rename":
		if err := s.store.SetComputerName(in.Name); err != nil {
			return nil, err
		}
	case "accept":
		if err := s.store.SetNetAccept(in.On); err != nil {
			return nil, err
		}
	case "log_share":
		if err := s.store.SetNetLogShare(in.On); err != nil {
			return nil, err
		}
	default:
		return nil, errors.New("неизвестное действие сервера")
	}
	st := s.server.Status()
	accept, err := s.store.NetAccept()
	if err != nil {
		return nil, err
	}
	share, err := s.store.NetLogShare()
	if err != nil {
		return nil, err
	}
	st.Accept, st.LogShare, st.Network = accept, share, s.networkProblem()
	return st, nil
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
		pa := broker.PresenceAgent{Agent: a.Agent, Running: a.Running, Waiting: a.Waiting}
		if len(a.Models) > 0 {
			pa.Models = a.Models
		}
		if l := a.Limit; l != nil {
			pa.Limit = &broker.PresenceLimit{State: l.State, UsedPercent: l.UsedPercent, Window: l.Window, ResetsAt: l.ResetsAt, ObservedAt: l.ObservedAt}
		}
		p.Agents = append(p.Agents, pa)
	}
	// A computer that joined before its window kept the joining name takes
	// it now, instead of reporting the system's name over it (D-061).
	if s.server != nil {
		if joined := s.server.MemberName(); joined != "" {
			_ = s.store.AdoptComputerName(joined)
		}
	}
	if name, err := s.store.ComputerName(); err == nil {
		p.Name = clipName(name)
	}
	// Readiness for other computers' tasks (T-070): left out if it cannot
	// be read, so the others see it as unknown rather than wrong.
	if accept, err := s.store.NetAccept(); err == nil {
		if projects, err := s.store.Projects(); err == nil {
			ready := &broker.PresenceReady{Accept: accept, Projects: []string{}}
			for _, pr := range projects {
				if pr.Folder != "" && len(ready.Projects) < 64 && len([]rune(pr.Name)) <= broker.MaxDeviceName {
					ready.Projects = append(ready.Projects, pr.Name)
				}
			}
			p.Ready = ready
		}
	}
	// Token usage of the last days, for the others' statistics (T-076);
	// the broker client sends it only when it changed.
	y, m, d := time.Now().Date()
	if days, err := s.store.UsageDaily(time.Date(y, m, d-(core.MaxUsageDays-1), 0, 0, 0, 0, time.Local), time.Local); err == nil {
		u := &broker.PresenceUsageReport{Days: make([]broker.PresenceUsage, 0, len(days))}
		for _, day := range days {
			u.Days = append(u.Days, broker.PresenceUsage(day))
		}
		p.Usage = u
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
