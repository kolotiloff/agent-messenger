package adapters

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Failure reasons of a worker run (T-034). They only explain a failure to the
// owner and the coordinator; nothing is ever retried because of them.
const (
	ReasonUsageLimit = "usage_limit" // the account's usage or plan limit
	ReasonRateLimit  = "rate_limit"  // too many requests for now
	ReasonOverload   = "overload"    // the provider is overloaded
	ReasonSignIn     = "sign_in"     // the CLI is not signed in
	ReasonOther      = "other"
)

var (
	// Claude Code: "Claude AI usage limit reached|1762538400".
	claudeLimitEpoch = regexp.MustCompile(`(?i)limit reached\|(\d{9,11})`)
	// Codex: "... or try again at 6:12 PM." / "try again at 18:12".
	codexTryAgainAt = regexp.MustCompile(`(?i)try again at (\d{1,2}):(\d{2})\s*([ap]\.?m\.?)?`)
	// Cursor: "Your usage limits will reset when your monthly cycle ends on
	// 12/10/2026." The date is month/day/year.
	cursorCycleEnds = regexp.MustCompile(`(?i)reset[^.]*?\bon (\d{1,2})/(\d{1,2})/(\d{4})\b`)
	// HTTP status codes count only as whole numbers.
	statusCode = regexp.MustCompile(`\b(401|429|503|529)\b`)
)

// classifyFailure reads a provider's error text. A text it does not
// recognise is "other": an unknown failure is never called a limit.
func classifyFailure(text string) string {
	t := strings.ToLower(text)
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(t, w) {
				return true
			}
		}
		return false
	}
	code := ""
	if m := statusCode.FindStringSubmatch(t); m != nil {
		code = m[1]
	}
	switch {
	case has("usage limit", "limit reached", "hit your limit", "out of credits", "credit balance", "quota", "billing"):
		return ReasonUsageLimit
	case has("rate limit", "rate_limit", "too many requests") || code == "429":
		return ReasonRateLimit
	case has("overloaded", "overload", "service unavailable") || code == "529" || code == "503":
		return ReasonOverload
	case has("not logged in", "please log in", "please run /login", "sign in again", "authentication", "unauthorized", "invalid api key", "oauth token") || code == "401":
		return ReasonSignIn
	}
	return ReasonOther
}

// stronger keeps the more specific of two reasons, so a later generic error
// event does not hide an earlier limit.
func stronger(old, next string) string {
	if old == "" || old == ReasonOther {
		return next
	}
	return old
}

// resetFromText finds a reset time in a provider's error text, relative to
// now on this computer. Only exact forms are read; anything else is left
// empty rather than guessed.
func resetFromText(text string, now time.Time) time.Time {
	if m := claudeLimitEpoch.FindStringSubmatch(text); m != nil {
		if sec, err := strconv.ParseInt(m[1], 10, 64); err == nil {
			return time.Unix(sec, 0)
		}
	}
	if m := cursorCycleEnds.FindStringSubmatch(text); m != nil {
		month, _ := strconv.Atoi(m[1])
		day, _ := strconv.Atoi(m[2])
		year, _ := strconv.Atoi(m[3])
		at := time.Date(year, time.Month(month), day, 0, 0, 0, 0, now.Location())
		if month < 1 || month > 12 || at.Day() != day || !at.After(now) {
			return time.Time{}
		}
		return at
	}
	if m := codexTryAgainAt.FindStringSubmatch(text); m != nil {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		if ampm := strings.ToLower(strings.ReplaceAll(m[3], ".", "")); ampm != "" {
			if h < 1 || h > 12 {
				return time.Time{}
			}
			h %= 12
			if ampm == "pm" {
				h += 12
			}
		}
		if h > 23 || min > 59 {
			return time.Time{}
		}
		at := time.Date(now.Year(), now.Month(), now.Day(), h, min, 0, 0, now.Location())
		if !at.After(now) {
			at = at.AddDate(0, 0, 1)
		}
		return at
	}
	return time.Time{}
}

// unixReset reads a reset time given in seconds since 1970.
func unixReset(v any) time.Time {
	if f, ok := v.(float64); ok && f > 1e9 && f < 1e11 {
		return time.Unix(int64(f), 0)
	}
	return time.Time{}
}

// noteFailure records a failure text with its reason and reset time.
func (r *TaskEventResult) noteFailure(text string, now time.Time) {
	r.Reason = stronger(r.Reason, classifyFailure(text))
	if at := resetFromText(text, now); !at.IsZero() && r.ResetAt.IsZero() {
		r.ResetAt = at
	}
}

// withErrorText adds the CLI's own error words to a failure, shortened,
// so a run that failed says why (coordinator list row 32).
func withErrorText(failure, text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return failure
	}
	if r := []rune(text); len(r) > 300 {
		text = string(r[:300]) + "…"
	}
	return failure + ": " + text
}

// errorText collects the human-readable error of an event: Claude's result
// text and assistant message error, Codex's message or error.message.
func errorText(e map[string]any) string {
	var parts []string
	add := func(v any) {
		if s, ok := v.(string); ok && s != "" {
			if len(s) > 2000 {
				s = s[:2000]
			}
			parts = append(parts, s)
		}
	}
	add(e["result"])
	add(e["message"])
	add(e["error"])
	if m, ok := e["error"].(map[string]any); ok {
		add(m["message"])
		add(m["type"])
	}
	if m, ok := e["message"].(map[string]any); ok {
		if content, ok := m["content"].([]any); ok {
			for _, c := range content {
				if part, ok := c.(map[string]any); ok && part["type"] == "text" {
					add(part["text"])
				}
			}
		}
	}
	return strings.Join(parts, " ")
}

// StderrFailure reads what a worker CLI wrote to its error stream when its
// run failed, for a CLI that stops before it reports an error in its own
// output (coordinator feedback 2026-11-12: Cursor stopped with exit code 1
// and no text). It gives the shortened text for the failure detail, its
// reason and a reset time when the text names one. The text only explains
// the failure; nothing is retried because of it.
func StderrFailure(text string, now time.Time) (detail, reason string, reset time.Time) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", "", time.Time{}
	}
	return withErrorText("CLI error output", text), classifyFailure(text), resetFromText(text, now)
}
