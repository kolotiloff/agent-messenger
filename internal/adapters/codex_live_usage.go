package adapters

import (
	"bytes"
	"encoding/json"
	"time"
)

// CodexLiveTokens reads the cumulative provider counter for a worker's
// current turn. Resumed sessions require a prior counter to subtract.
func CodexLiveTokens(profile, thread string, launched, taskCreated time.Time, resumed bool) *int64 {
	return codexRollout(profile, thread, launched, taskCreated, func(path string) *int64 {
		return readCodexLiveTokens(readCodexRolloutTail(path), launched, resumed)
	})
}

func readCodexLiveTokens(raw []byte, launched time.Time, resumed bool) *int64 {
	var latest, baseline *int64
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if !bytes.Contains(line, []byte(`"total_token_usage"`)) {
			continue
		}
		var event struct {
			Type    string    `json:"type"`
			At      time.Time `json:"timestamp"`
			Payload struct {
				Type string `json:"type"`
				Info struct {
					Total struct {
						Tokens *int64 `json:"total_tokens"`
					} `json:"total_token_usage"`
				} `json:"info"`
			} `json:"payload"`
		}
		if json.Unmarshal(line, &event) != nil || event.Type != "event_msg" ||
			event.Payload.Type != "token_count" || event.At.IsZero() {
			continue
		}
		n := event.Payload.Info.Total.Tokens
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
		return nil
	}
	if baseline != nil {
		if *latest < *baseline {
			return nil
		}
		current := *latest - *baseline
		return &current
	}
	return latest
}
