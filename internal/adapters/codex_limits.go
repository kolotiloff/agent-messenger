package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"time"
)

// CodexLimitArgs starts Codex's app-server only to read the account's usage
// limits (T-191). It runs with the owner's normal Codex profile and sign-in,
// because the limits belong to that account. Messenger sends initialize and
// account/rateLimits/read and nothing else: no thread is created and no
// model is called. Tools, plugins and MCP servers are turned off anyway.
func CodexLimitArgs(cli string) ([]string, error) {
	if !filepath.IsAbs(cli) {
		return nil, errors.New("Codex CLI path must be absolute")
	}
	args := []string{cli, "app-server", "--listen", "stdio://"}
	for _, feature := range codexDisabledFeatures {
		args = append(args, "--disable", feature)
	}
	for _, override := range []string{
		`sandbox_mode="read-only"`, `approval_policy="never"`, `web_search="disabled"`, `mcp_servers={}`,
	} {
		args = append(args, "-c", override)
	}
	return args, nil
}

// ReadCodexLimits asks a started app-server for the account's limits: the
// handshake, then one account/rateLimits/read. It writes nothing else. The
// caller stops the process afterwards, whatever the result.
func ReadCodexLimits(ctx context.Context, w io.Writer, r io.Reader, version string) (LimitState, error) {
	rpc := NewCodexRPC(w, r)
	// Keep reading until the caller stops the process, so the reader never
	// blocks on a full queue.
	defer func() { go func() { _ = rpc.Drain(nil) }() }()
	if _, err := rpc.Call(ctx, "initialize", CodexInitializeParams(version)); err != nil {
		return LimitState{}, err
	}
	if err := rpc.Notify("initialized"); err != nil {
		return LimitState{}, err
	}
	raw, err := rpc.Call(ctx, "account/rateLimits/read", map[string]any{"excludeResetCreditDetails": true})
	if err != nil {
		return LimitState{}, err
	}
	return ParseCodexRateLimits(raw, time.Now())
}

// ParseCodexRateLimits reads an account/rateLimits/read result. Its
// rateLimits snapshot has a primary (about five hours) and a secondary
// (weekly) window, each with usedPercent, windowDurationMins and resetsAt
// (Unix seconds). The window used the most leads; both are kept.
func ParseCodexRateLimits(raw json.RawMessage, at time.Time) (LimitState, error) {
	type window struct {
		UsedPercent        *float64 `json:"usedPercent"`
		WindowDurationMins *float64 `json:"windowDurationMins"`
		ResetsAt           *float64 `json:"resetsAt"`
	}
	var result struct {
		RateLimits *struct {
			Primary              *window `json:"primary"`
			Secondary            *window `json:"secondary"`
			RateLimitReachedType *string `json:"rateLimitReachedType"`
		} `json:"rateLimits"`
	}
	if json.Unmarshal(raw, &result) != nil || result.RateLimits == nil {
		return LimitState{}, fmt.Errorf("%w: no rate limits in the answer", ErrCodexProtocol)
	}
	var l LimitState
	for _, w := range []*window{result.RateLimits.Primary, result.RateLimits.Secondary} {
		if w == nil || w.UsedPercent == nil || *w.UsedPercent < 0 || *w.UsedPercent > 1000 {
			continue
		}
		lw := LimitWindow{UsedPercent: *w.UsedPercent}
		if w.WindowDurationMins != nil && *w.WindowDurationMins > 0 && *w.WindowDurationMins < 1e7 {
			lw.Window = windowName(int(*w.WindowDurationMins))
		}
		if w.ResetsAt != nil && *w.ResetsAt > 0 && *w.ResetsAt < 1e11 {
			lw.ResetAt = time.Unix(int64(*w.ResetsAt), 0).UTC()
		}
		l.Windows = append(l.Windows, lw)
		if !l.HasPercent || lw.UsedPercent > l.UsedPercent {
			l.UsedPercent, l.HasPercent, l.Window, l.ResetAt = lw.UsedPercent, true, lw.Window, lw.ResetAt
		}
	}
	if !l.HasPercent {
		return LimitState{}, fmt.Errorf("%w: no usage window in the answer", ErrCodexProtocol)
	}
	l.State = LimitStateFor(l.UsedPercent)
	if t := result.RateLimits.RateLimitReachedType; t != nil && *t != "" {
		l.State = LimitExhausted
	}
	return l, nil
}

// LimitWarningPercent is the share used from which a checked limit counts
// as close (T-191).
const LimitWarningPercent = 90

// LimitStateFor is the state a share used means.
func LimitStateFor(used float64) string {
	switch {
	case used >= 100:
		return LimitExhausted
	case used >= LimitWarningPercent:
		return LimitWarning
	}
	return LimitOK
}
