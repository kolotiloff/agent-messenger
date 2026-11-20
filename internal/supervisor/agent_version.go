package supervisor

import (
	"context"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/cliprobe"
	"github.com/kolotiloff/agent-messenger/internal/journal"
)

// Version probes run serially outside the mailbox loop, so a slow CLI does
// not hold up tasks, answers or reports.
func (s *Supervisor) runAgentVersionChecks(ctx context.Context) {
	ticker := time.NewTicker(networkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if net, ok := s.server.Network(); ok {
				s.answerAgentVersion(ctx, net.BrokerID)
			}
		}
	}
}

func (s *Supervisor) answerAgentVersion(ctx context.Context, broker string) {
	r, err := s.store.ClaimAgentVersionRequest(broker)
	if err != nil {
		journal.Printf("agent version request could not be read")
		return
	}
	if r.ID == "" {
		return
	}
	version, reason, err := cliprobe.RegisteredVersion(ctx, s.store, r.Agent)
	if err != nil {
		reason = "probe_failed"
	}
	if err := s.store.ReplyAgentVersion(r, version, reason); err != nil {
		journal.Printf("agent version reply could not be queued")
	}
}
