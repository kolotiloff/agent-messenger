package adapters

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// Limit states an agent reported (T-044).
const (
	LimitOK        = "ok"
	LimitWarning   = "warning"
	LimitExhausted = "exhausted"
)

// LimitState is the last usage-limit state an agent reported during a run.
// It is what the agent said at that moment, not a live reading.
type LimitState struct {
	State string
	// UsedPercent is the share of the window used, when the agent said it.
	UsedPercent float64
	HasPercent  bool
	// Window names the limit window as the agent reported it.
	Window  string
	ResetAt time.Time
	// Reported is what the agent sent about the limit, its plain fields
	// only, so its units can be learned from real runs (T-080).
	Reported string
}

// maxReported bounds Reported.
const maxReported = 480

// reported keeps the plain (string, number, bool) fields of a limit object
// as compact JSON, in key order, at most maxReported bytes.
func reported(m map[string]any) string {
	keys := make([]string, 0, len(m))
	for k, v := range m {
		switch v.(type) {
		case string, float64, bool:
			if len(k) <= 40 {
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	out := "{"
	for _, k := range keys {
		v := m[k]
		if s, ok := v.(string); ok && len(s) > 64 {
			v = s[:64]
		}
		kb, _ := json.Marshal(k)
		vb, _ := json.Marshal(v)
		part := string(kb) + ":" + string(vb)
		if len(out)+len(part)+2 > maxReported {
			break
		}
		if len(out) > 1 {
			out += ","
		}
		out += part
	}
	return out + "}"
}

func limitRank(state string) int {
	switch state {
	case LimitExhausted:
		return 2
	case LimitWarning:
		return 1
	}
	return 0
}

// noteLimit keeps the more severe state of one run; a later report of the
// same severity replaces an earlier one.
func (r *TaskEventResult) noteLimit(l LimitState) {
	if r.Limit == nil || limitRank(l.State) >= limitRank(r.Limit.State) {
		r.Limit = &l
	}
}

// claudeLimit reads Claude's rate_limit_event. Claude states the status and
// the reset time; a share used is not taken from it.
func claudeLimit(info map[string]any) (LimitState, bool) {
	status, _ := info["status"].(string)
	l := LimitState{ResetAt: unixReset(info["resetsAt"]), Reported: reported(info)}
	l.Window, _ = info["rateLimitType"].(string)
	switch status {
	case "allowed":
		l.State = LimitOK
	case "allowed_warning":
		l.State = LimitWarning
	case "rejected":
		l.State = LimitExhausted
	default:
		return l, false
	}
	return l, true
}

// codexLimit reads a "rate_limits" object with "primary" and "secondary"
// windows, at the top of an event or in its payload or msg. The window with
// the larger share used is kept.
func codexLimit(e map[string]any) (LimitState, bool) {
	limits, _ := e["rate_limits"].(map[string]any)
	for _, key := range []string{"payload", "msg"} {
		if limits != nil {
			break
		}
		if inner, ok := e[key].(map[string]any); ok {
			limits, _ = inner["rate_limits"].(map[string]any)
		}
	}
	var best LimitState
	found := false
	for _, key := range []string{"primary", "secondary"} {
		w, _ := limits[key].(map[string]any)
		used, ok := w["used_percent"].(float64)
		if !ok || used < 0 || used > 1000 {
			continue
		}
		if found && used <= best.UsedPercent {
			continue
		}
		l := LimitState{State: LimitOK, UsedPercent: used, HasPercent: true, ResetAt: unixReset(w["resets_at"])}
		if used >= 100 {
			l.State = LimitExhausted
		}
		l.Reported = reported(w)
		if minutes, ok := w["window_minutes"].(float64); ok && minutes > 0 {
			l.Window = windowName(int(minutes))
		}
		if secs, ok := w["resets_in_seconds"].(float64); ok && l.ResetAt.IsZero() && secs >= 0 && secs < 1e8 {
			l.ResetAt = taskNow().Add(time.Duration(secs) * time.Second).Truncate(time.Second)
		}
		best, found = l, true
	}
	return best, found
}

func windowName(minutes int) string {
	switch {
	case minutes%1440 == 0:
		return fmt.Sprintf("%dd", minutes/1440)
	case minutes%60 == 0:
		return fmt.Sprintf("%dh", minutes/60)
	}
	return fmt.Sprintf("%dm", minutes)
}
