package adapters

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// T-044: the last limit state an agent reported during a run.
func TestLimitStateFromEvents(t *testing.T) {
	var r TaskEventResult
	for _, line := range []string{
		`{"type":"rate_limit_event","session_id":"s","rate_limit_info":{"status":"allowed","resetsAt":1762538000,"rateLimitType":"five_hour"}}`,
		`{"type":"rate_limit_event","session_id":"s","rate_limit_info":{"status":"allowed_warning","resetsAt":1762900000,"rateLimitType":"seven_day"}}`,
		`{"type":"rate_limit_event","session_id":"s","rate_limit_info":{"status":"allowed","resetsAt":1762538000,"rateLimitType":"five_hour"}}`,
	} {
		ParseTaskEvent("claude", []byte(line), &r)
	}
	if r.Limit == nil || r.Limit.State != LimitWarning || r.Limit.Window != "seven_day" || !r.Limit.ResetAt.Equal(time.Unix(1762900000, 0)) || r.Limit.HasPercent {
		t.Fatalf("claude limit %+v", r.Limit)
	}
	ParseTaskEvent("claude", []byte(`{"type":"rate_limit_event","session_id":"s","rate_limit_info":{"status":"rejected","resetsAt":1762538400,"rateLimitType":"five_hour"}}`), &r)
	if r.Limit.State != LimitExhausted || r.Limit.Window != "five_hour" {
		t.Fatalf("rejected %+v", r.Limit)
	}

	// Codex: the window with the larger share used.
	var c TaskEventResult
	ParseTaskEvent("codex", []byte(`{"type":"token_count","rate_limits":{"primary":{"used_percent":62.5,"window_minutes":300,"resets_at":1762538400},"secondary":{"used_percent":20,"window_minutes":10080,"resets_in_seconds":3600}}}`), &c)
	if c.Limit == nil || c.Limit.State != LimitOK || !c.Limit.HasPercent || c.Limit.UsedPercent != 62.5 || c.Limit.Window != "5h" || !c.Limit.ResetAt.Equal(time.Unix(1762538400, 0)) {
		t.Fatalf("codex limit %+v", c.Limit)
	}
	var full TaskEventResult
	ParseTaskEvent("codex", []byte(`{"type":"event_msg","payload":{"type":"token_count","rate_limits":{"primary":{"used_percent":100,"window_minutes":10080}}}}`), &full)
	if full.Limit == nil || full.Limit.State != LimitExhausted || full.Limit.Window != "7d" {
		t.Fatalf("codex full %+v", full.Limit)
	}
	// Nothing about limits: no state.
	var none TaskEventResult
	ParseTaskEvent("codex", []byte(`{"type":"turn.completed","usage":{"input_tokens":1}}`), &none)
	ParseTaskEvent("codex", []byte(`{"type":"x","rate_limits":{"primary":{"used_percent":"lots"}}}`), &none)
	if none.Limit != nil {
		t.Fatalf("no limit expected: %+v", none.Limit)
	}
}

// T-080: the plain fields the agent sent about its limit are kept, so the
// units of a share can be learned from a real run; objects and long texts
// are left out.
func TestLimitKeepsWhatTheAgentReported(t *testing.T) {
	var r TaskEventResult
	ParseTaskEvent("claude", []byte(`{"type":"rate_limit_event","session_id":"s","rate_limit_info":{"status":"allowed_warning","resetsAt":1762900000,"rateLimitType":"seven_day","utilization":0.83,"isUsingOverage":false,"nested":{"a":1}}}`), &r)
	want := `{"isUsingOverage":false,"rateLimitType":"seven_day","resetsAt":1762900000,"status":"allowed_warning","utilization":0.83}`
	if r.Limit == nil || r.Limit.Reported != want || r.Limit.HasPercent {
		t.Fatalf("reported %+v", r.Limit)
	}
	long := map[string]any{"status": strings.Repeat("x", 300)}
	for i := 0; i < 40; i++ {
		long[fmt.Sprintf("field_%02d", i)] = float64(i)
	}
	if got := reported(long); len(got) > maxReported || !strings.HasSuffix(got, "}") {
		t.Fatalf("long %d %q", len(got), got)
	}
}
