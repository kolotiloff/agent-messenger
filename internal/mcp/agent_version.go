package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/kolotiloff/agent-messenger/internal/cliprobe"
	"github.com/kolotiloff/agent-messenger/internal/core"
)

func (c *coordinator) agentVersion(ctx context.Context, computer, agent, requestID string) (any, error) {
	if requestID != "" {
		if computer != "" || agent != "" {
			return nil, fmt.Errorf("%w: pass only request_id to read an earlier version request", core.ErrRejected)
		}
		return c.store.AgentVersion(requestID)
	}
	if !core.TaskProviders[agent] {
		return nil, fmt.Errorf("%w: agent must be claude, codex or cursor", core.ErrRejected)
	}
	self, err := c.store.ComputerID()
	if err != nil {
		return nil, err
	}
	if computer == "" || computer == self {
		name, err := c.store.ComputerName()
		if err != nil {
			return nil, err
		}
		version, reason, err := cliprobe.RegisteredVersion(ctx, c.store, agent)
		if err != nil {
			return nil, err
		}
		state := "reported"
		if version == "" {
			state = "unknown"
		}
		return core.AgentVersion{
			Computer: name, Agent: agent, State: state, Version: version, Reason: reason,
			CheckedAt: time.Now().UTC().Format(time.RFC3339Nano),
		}, nil
	}
	id, err := c.store.RequestAgentVersion(computer, agent)
	if err != nil {
		return nil, err
	}
	wait := time.NewTimer(20 * time.Second)
	defer wait.Stop()
	ticker := time.NewTicker(coordinatorPoll)
	defer ticker.Stop()
	for {
		v, err := c.store.AgentVersion(id)
		if err != nil || v.State != "waiting" {
			return v, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-wait.C:
			return c.store.AgentVersion(id)
		case <-ticker.C:
		}
	}
}
