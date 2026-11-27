package adapters

import (
	"bytes"
	"encoding/json"
	"time"
)

// CodexUsage separates cumulative work from the last provider context snapshot.
type CodexUsage struct {
	ObservedContext bool
	TurnTokens      *int64
	UsedTokens      int64
	CapacityTokens  int64
}

func CodexLiveUsage(profile, thread string, launched, taskCreated time.Time, resumed bool) CodexUsage {
	return codexRollout(profile, thread, launched, taskCreated, func(path string) CodexUsage {
		return readCodexLiveUsage(readCodexRolloutTail(path), launched, resumed)
	})
}

func readCodexLiveUsage(raw []byte, launched time.Time, resumed bool) CodexUsage {
	var latest, baseline *int64
	var usage CodexUsage
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if !bytes.Contains(line, []byte(`"token_count"`)) && !bytes.Contains(line, []byte(`"compacted"`)) && !bytes.Contains(line, []byte(`"turn_context"`)) {
			continue
		}
		var event struct {
			Type    string    `json:"type"`
			At      time.Time `json:"timestamp"`
			Payload struct {
				Type string `json:"type"`
				Info struct {
					Capacity int64 `json:"model_context_window"`
					Last     struct {
						Tokens *int64 `json:"total_tokens"`
					} `json:"last_token_usage"`
					Total struct {
						Tokens *int64 `json:"total_tokens"`
					} `json:"total_token_usage"`
				} `json:"info"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &event) != nil || event.At.IsZero() {
			continue
		}
		if (event.Type == "compacted" || event.Type == "turn_context") && !event.At.Before(launched) {
			usage.ObservedContext = true
			usage.UsedTokens, usage.CapacityTokens = 0, 0
			continue
		}
		if event.Type != "event_msg" ||
			event.Payload.Type != "token_count" {
			continue
		}
		info := event.Payload.Info
		if !event.At.Before(launched) {
			usage.ObservedContext = true
			usage.UsedTokens, usage.CapacityTokens = 0, 0
			if used := info.Last.Tokens; used != nil && *used >= 0 && info.Capacity > 0 {
				usage.UsedTokens, usage.CapacityTokens = *used, info.Capacity
			}
		}
		n := info.Total.Tokens
		if n == nil || *n < 0 {
			continue
		}
		if event.At.Before(launched) {
			baseline = n
		} else {
			latest = n
		}
	}
	if latest == nil || (resumed && baseline == nil) {
		return usage
	}
	if baseline != nil {
		if *latest < *baseline {
			return usage
		}
		current := *latest - *baseline
		usage.TurnTokens = &current
		return usage
	}
	usage.TurnTokens = latest
	return usage
}
