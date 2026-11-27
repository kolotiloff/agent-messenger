package adapters

import (
	"fmt"
	"testing"
	"time"
)

func TestCodexLiveTokensSeparatesResumedTurns(t *testing.T) {
	launch := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	event := func(at time.Time, count int) string {
		return fmt.Sprintf(`{"timestamp":%q,"type":"event_msg","payload":{"type":"token_count","info":{"total_token_usage":{"total_tokens":%d}}}}`+"\n",
			at.Format(time.RFC3339), count)
	}
	old := event(launch.Add(-time.Minute), 100)
	current := event(launch.Add(time.Second), 145)
	for _, tc := range []struct {
		name, data string
		resumed    bool
		want       int64
	}{
		{"initial", current, false, 145},
		{"resumed", old + current + "{incomplete", true, 45},
		{"missing baseline", current, true, -1},
		{"missing turn", old, true, -1},
		{"reset counter", old + event(launch, 80), true, -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := readCodexLiveTokens([]byte(tc.data), launch, tc.resumed)
			if tc.want < 0 && got != nil || tc.want >= 0 && (got == nil || *got != tc.want) {
				t.Fatalf("live tokens = %v, want %d (-1 means unavailable)", got, tc.want)
			}
		})
	}
}
