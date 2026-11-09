// Package adapters builds the task worker and coordinator commands for Claude
// Code, Codex and Cursor and reads their events. It does not execute them.
package adapters

import (
	"errors"
	"fmt"
	"strings"
)

// ExplicitResume reports argv that would continue an unbound or latest session.
func ExplicitResume(args []string, conversationID string) error {
	if conversationID == "" {
		return errors.New("exact conversation id is required")
	}
	joined := "\n" + strings.Join(args, "\n") + "\n"
	for _, banned := range []string{"--last", "--continue", "--resume-last"} {
		if strings.Contains(joined, "\n"+banned+"\n") || strings.Contains(joined, banned+"=") {
			return fmt.Errorf("banned resume mode %s", banned)
		}
	}
	if !strings.Contains(joined, conversationID) {
		return errors.New("exact conversation id is absent")
	}
	return nil
}

func rejectLooseID(id string) error {
	switch strings.ToLower(strings.TrimSpace(id)) {
	case "", "last", "continue", "latest":
		return errors.New("exact conversation id is required")
	default:
		return nil
	}
}
