package supervisor

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/broker"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

// networkInterval is how often the client sends its task messages and reads
// the messages of other computers (T-037).
var networkInterval = 3 * time.Second

// peersEvery is how many rounds pass between updates of the list of the
// network's computers that the coordinator reads.
const peersEvery = 5

type netState struct {
	mu      sync.Mutex
	problem string
	broker  string
	round   int
}

func (s *Supervisor) networkProblem() string {
	s.net.mu.Lock()
	defer s.net.mu.Unlock()
	return s.net.problem
}

func (s *Supervisor) setNetworkProblem(p string) {
	s.net.mu.Lock()
	s.net.problem = p
	s.net.mu.Unlock()
}

// runNetwork exchanges task messages with the network until ctx ends.
func (s *Supervisor) runNetwork(ctx context.Context) {
	ticker := time.NewTicker(networkInterval)
	defer ticker.Stop()
	for {
		s.networkRound(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// networkRound makes one exchange: it records the network's computers for
// the coordinator, sends the outbox in order and applies new messages. A
// message that could not be sent stays and goes first next round; that is
// a delivery retry the broker and the receiver deduplicate, never a second
// start, answer or cancel.
func (s *Supervisor) networkRound(ctx context.Context) {
	if s.server == nil {
		return
	}
	n, ok := s.server.Network()
	s.net.mu.Lock()
	changed := n.BrokerID != s.net.broker
	refresh := changed || s.net.round%peersEvery == 0
	s.net.broker = n.BrokerID
	s.net.round++
	s.net.mu.Unlock()
	if !ok {
		if changed {
			_ = s.store.SetNetwork("", "", nil)
		}
		s.setNetworkProblem("")
		return
	}
	if refresh {
		var peers []core.NetPeer
		for _, p := range s.server.Status().Peers {
			np := core.NetPeer{ID: p.ID, Name: p.Name, Host: p.Host, Age: p.Age}
			for _, a := range p.Agents {
				na := core.NetAgent{Agent: a.Agent, Running: a.Running, Waiting: a.Waiting, Models: a.Models}
				if l := a.Limit; l != nil {
					na.Limit = &core.AgentLimit{State: l.State, UsedPercent: l.UsedPercent, Window: l.Window, ResetsAt: l.ResetsAt, ObservedAt: l.ObservedAt}
				}
				np.Agents = append(np.Agents, na)
			}
			peers = append(peers, np)
		}
		if err := s.store.SetNetwork(n.BrokerID, n.Self, peers); err != nil {
			s.setNetworkProblem("список компьютеров сети не записан: " + err.Error())
			return
		}
	}
	// Send, read, then send what the read messages caused (an acceptance,
	// a refusal), so a reply does not wait for the next round.
	problem := s.sendOutbox(ctx, n)
	if p := s.readInbox(ctx, n); problem == "" {
		problem = p
	}
	if problem == "" {
		problem = s.sendOutbox(ctx, n)
	}
	if refresh {
		if p := s.shareTasks(ctx, n); problem == "" {
			problem = p
		}
	}
	s.setNetworkProblem(problem)
}

// sharedDays is how far back a computer shares the tasks it started.
const sharedDays = 7

// shareTasks sends the tasks this computer started and keeps the lists of
// the other computers (T-047). A broker older than shared lists is not a
// problem: the window then shows no lists.
func (s *Supervisor) shareTasks(ctx context.Context, n broker.Network) string {
	mine, err := s.store.SharedTasks(time.Now().AddDate(0, 0, -sharedDays))
	if err != nil {
		return "список задач не читается: " + err.Error()
	}
	list := make([]broker.SharedTask, 0, len(mine))
	for _, t := range mine {
		list = append(list, broker.SharedTask(t))
	}
	peers, err := s.server.ShareTasks(ctx, n, list)
	switch {
	case errors.Is(err, broker.ErrNoSharedTasks), errors.Is(err, broker.ErrNetworkChanged):
		return ""
	case err != nil:
		return "общий список задач не получен: " + err.Error()
	}
	out := make([]core.NetPeerTasks, 0, len(peers))
	for _, p := range peers {
		pt := core.NetPeerTasks{Peer: p.Peer, Name: p.Name, Age: p.Age, Tasks: make([]core.NetTask, 0, len(p.Tasks))}
		for _, t := range p.Tasks {
			pt.Tasks = append(pt.Tasks, core.NetTask(t))
		}
		out = append(out, pt)
	}
	if err := s.store.SetNetTasks(out); err != nil {
		return "общий список задач не записан: " + err.Error()
	}
	return ""
}

func (s *Supervisor) sendOutbox(ctx context.Context, n broker.Network) string {
	pending, err := s.store.NetPending(n.BrokerID, 50)
	if err != nil {
		return "очередь сообщений не читается: " + err.Error()
	}
	for _, m := range pending {
		err := s.server.Send(ctx, n, m.To, m.OpID, m.Body)
		switch {
		case err == nil:
			err = s.store.NetSent(m.Seq)
		case errors.Is(err, broker.ErrRecipient):
			err = s.store.NetUndeliverable(m.Seq)
		case errors.Is(err, broker.ErrNetworkChanged):
			return ""
		default:
			return "задачи не отправлены: " + err.Error()
		}
		if err != nil {
			return "очередь сообщений не обновлена: " + err.Error()
		}
	}
	return ""
}

func (s *Supervisor) readInbox(ctx context.Context, n broker.Network) string {
	for batch := 0; batch < 8; batch++ {
		cursor, err := s.store.NetCursor(n.BrokerID)
		if err != nil {
			return "сообщения сети не читаются: " + err.Error()
		}
		messages, err := s.server.Inbox(ctx, n, cursor)
		if errors.Is(err, broker.ErrNetworkChanged) {
			return ""
		}
		if err != nil {
			return "сообщения сети не получены: " + err.Error()
		}
		for _, m := range messages {
			if err := s.store.ApplyNet(n.BrokerID, core.NetIncoming{Seq: m.Seq, From: m.From, OpID: m.OpID, Body: m.Body}); err != nil {
				return "сообщение сети не применено: " + err.Error()
			}
		}
		if len(messages) < broker.InboxBatch {
			return ""
		}
	}
	return ""
}
