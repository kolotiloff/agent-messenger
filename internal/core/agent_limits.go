package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// AgentLimit is the last usage-limit state an agent reported during a task
// run (T-044). It is what the agent said then, not a live reading.
type AgentLimit struct {
	// State is "ok", "warning" (close to the limit) or "exhausted".
	State string `json:"state"`
	// UsedPercent is the share of the window used, when the agent said it.
	UsedPercent *float64 `json:"used_percent,omitempty"`
	// Window names the limit window as the agent reported it.
	Window string `json:"window,omitempty"`
	// ResetsAt is when the agent said the limit resets (RFC 3339, UTC).
	ResetsAt string `json:"resets_at,omitempty"`
	// ObservedAt is when Messenger read it (RFC 3339, UTC).
	ObservedAt string `json:"observed_at"`
	// Reported is what the agent sent about the limit (its plain fields as
	// JSON), kept so its units can be learned from real runs (T-080).
	Reported string `json:"reported,omitempty"`
	// ResetPassed is set when the limit is listed after its reset time
	// (T-081): the state above is then likely out of date. Never stored.
	ResetPassed bool `json:"reset_passed,omitempty"`
}

// limitView is a copy of l for the coordinator, with ResetPassed set.
func limitView(l *AgentLimit, at time.Time) *AgentLimit {
	if l == nil {
		return nil
	}
	c := *l
	c.ResetPassed = false
	if t, err := time.Parse(time.RFC3339, c.ResetsAt); c.ResetsAt != "" && err == nil && !t.After(at) {
		c.ResetPassed = true
	}
	return &c
}

// LimitNote is a warning for the coordinator when an agent last reported
// that it is close to or at its limit and the reset time has not passed
// (T-081). It is only advice: nothing is refused or retried because of it.
func LimitNote(l *AgentLimit) string {
	if l == nil || l.ResetPassed {
		return ""
	}
	resets := ""
	if l.ResetsAt != "" {
		resets = ", resets at " + l.ResetsAt
	}
	switch l.State {
	case "exhausted":
		return "this agent reported its usage limit exhausted at " + l.ObservedAt + resets + ": the task will likely fail until then. Tell the owner, or start it on another agent or computer."
	case "warning":
		return "this agent reported it is close to its usage limit at " + l.ObservedAt + resets + ". Tell the owner if the task is long."
	}
	return ""
}

// AgentLimitFor is the limit list_agents shows for an agent on a computer
// of the project ("" is this computer), or nil when none is known.
func (s *Store) AgentLimitFor(project, computer, provider string) (*AgentLimit, error) {
	out, err := s.ProjectAgents(project)
	if err != nil {
		return nil, err
	}
	list, _ := out["computers"].([]ComputerSummary)
	for _, c := range list {
		if !(computer == "" && c.This || computer != "" && c.Computer == computer) {
			continue
		}
		for _, a := range c.Agents {
			if a.Agent == provider {
				return a.Limit, nil
			}
		}
	}
	return nil, nil
}

var agentLimitStates = map[string]bool{"ok": true, "warning": true, "exhausted": true}

func (l AgentLimit) valid() bool {
	if !agentLimitStates[l.State] || len(l.Window) > 32 || len(l.Reported) > 512 {
		return false
	}
	if l.UsedPercent != nil && (*l.UsedPercent < 0 || *l.UsedPercent > 1000) {
		return false
	}
	for _, at := range []string{l.ResetsAt, l.ObservedAt} {
		if at == "" {
			continue
		}
		if _, err := time.Parse(time.RFC3339, at); err != nil {
			return false
		}
	}
	return l.ObservedAt != ""
}

// RecordAgentLimit keeps the limit state an agent of this computer reported.
func (s *Store) RecordAgentLimit(provider string, l AgentLimit) error {
	if l.ObservedAt == "" {
		l.ObservedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if !l.valid() {
		return fmt.Errorf("%w: invalid limit state", ErrRejected)
	}
	raw, _ := json.Marshal(l)
	return s.tx(func(tx *sql.Tx) error {
		computer, err := localComputer(tx)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`UPDATE computer_agents SET limit_state=? WHERE computer=? AND provider=?`, string(raw), computer, provider)
		return err
	})
}

// parseAgentLimit reads a stored or reported limit; anything unreadable is
// no limit.
func parseAgentLimit(raw string) *AgentLimit {
	if raw == "" {
		return nil
	}
	var l AgentLimit
	if json.Unmarshal([]byte(raw), &l) != nil || !l.valid() {
		return nil
	}
	return &l
}
