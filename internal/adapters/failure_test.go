package adapters

import (
	"testing"
	"time"
)

func TestClassifyFailure(t *testing.T) {
	for text, want := range map[string]string{
		"Claude AI usage limit reached|1762538400":                             ReasonUsageLimit,
		"You've hit your usage limit. Upgrade to Pro or try again at 6:12 PM.": ReasonUsageLimit,
		"Quota exceeded. Check your plan and billing details.":                 ReasonUsageLimit,
		"429 Too Many Requests":                                                ReasonRateLimit,
		"API Error: Repeated 529 Overloaded errors":                            ReasonOverload,
		"503 Service Unavailable":                                              ReasonOverload,
		"Invalid API key · Please run /login":                                  ReasonSignIn,
		"stream error: 401 Unauthorized":                                       ReasonSignIn,
		"error_max_turns":                                                      ReasonOther,
		"the tests failed in file 4290_limits_test.go":                         ReasonOther,
		"": ReasonOther,
	} {
		if got := classifyFailure(text); got != want {
			t.Errorf("%q: %s, want %s", text, got, want)
		}
	}
}

func TestResetFromText(t *testing.T) {
	now := time.Date(2026, 11, 7, 15, 0, 0, 0, time.Local)
	for text, want := range map[string]time.Time{
		"Claude AI usage limit reached|1762538400": time.Unix(1762538400, 0),
		"try again at 6:12 PM.":                    time.Date(2026, 11, 7, 18, 12, 0, 0, time.Local),
		"try again at 12:05 a.m.":                  time.Date(2026, 11, 8, 0, 5, 0, 0, time.Local),
		"try again at 9:30":                        time.Date(2026, 11, 8, 9, 30, 0, 0, time.Local),
		"try again at 18:12":                       time.Date(2026, 11, 7, 18, 12, 0, 0, time.Local),
		"try again at 13:00 PM":                    {},
		"try again later":                          {},
		"resets soon":                              {},
	} {
		if got := resetFromText(text, now); !got.Equal(want) {
			t.Errorf("%q: %v, want %v", text, got, want)
		}
	}
}

func TestParseTaskEventFailureReasons(t *testing.T) {
	defer func(old func() time.Time) { taskNow = old }(taskNow)
	taskNow = func() time.Time { return time.Date(2026, 11, 7, 15, 0, 0, 0, time.Local) }
	feed := func(provider string, lines ...string) TaskEventResult {
		var r TaskEventResult
		for _, line := range lines {
			ParseTaskEvent(provider, []byte(line), &r)
		}
		return r
	}
	// Claude: a rejected plan limit with its reset time, then a failed result.
	r := feed("claude",
		`{"type":"rate_limit_event","session_id":"s","rate_limit_info":{"status":"allowed_warning","resetsAt":1762538000}}`,
		`{"type":"rate_limit_event","session_id":"s","rate_limit_info":{"status":"rejected","resetsAt":1762538400,"rateLimitType":"five_hour"}}`,
		`{"type":"assistant","session_id":"s","error":"rate_limit","message":{"content":[{"type":"text","text":"Claude AI usage limit reached|1762538400"}]}}`,
		`{"type":"result","subtype":"success","is_error":true,"session_id":"s","result":"Claude AI usage limit reached|1762538400"}`)
	if r.Completed || r.Failure == "" || r.Reason != ReasonUsageLimit || !r.ResetAt.Equal(time.Unix(1762538400, 0)) {
		t.Fatalf("claude limit %+v", r)
	}
	// A warning alone is not a failure.
	r = feed("claude", `{"type":"rate_limit_event","session_id":"s","rate_limit_info":{"status":"allowed_warning","resetsAt":1762538000}}`,
		`{"type":"result","subtype":"success","is_error":false,"session_id":"s"}`)
	if !r.Completed || r.Reason != "" || !r.ResetAt.IsZero() {
		t.Fatalf("claude warning %+v", r)
	}
	// Assistant error codes.
	for code, want := range map[string]string{"authentication_failed": ReasonSignIn, "billing_error": ReasonUsageLimit, "rate_limit": ReasonRateLimit, "server_error": ReasonOther} {
		r = feed("claude", `{"type":"assistant","session_id":"s","error":"`+code+`","message":{"content":[{"type":"text","text":"API Error"}]}}`,
			`{"type":"result","subtype":"success","is_error":true,"session_id":"s","result":"API Error"}`)
		if r.Reason != want {
			t.Errorf("claude %s: %q, want %q", code, r.Reason, want)
		}
	}
	// A later generic error does not hide the limit.
	r = feed("claude", `{"type":"assistant","session_id":"s","error":"billing_error","message":{"content":[]}}`,
		`{"type":"result","subtype":"error_during_execution","is_error":true,"session_id":"s"}`)
	if r.Reason != ReasonUsageLimit {
		t.Fatalf("claude kept %+v", r)
	}
	// An unknown failure stays unexplained.
	r = feed("claude", `{"type":"result","subtype":"error_max_turns","is_error":true,"session_id":"s"}`)
	if r.Reason != ReasonOther || !r.ResetAt.IsZero() {
		t.Fatalf("claude unknown %+v", r)
	}
	// Codex: the limit message with a clock time.
	r = feed("codex", `{"type":"thread.started","thread_id":"t"}`,
		`{"type":"error","message":"You've hit your usage limit. Upgrade to Pro or try again at 6:12 PM."}`,
		`{"type":"turn.failed","error":{"message":"You've hit your usage limit. Upgrade to Pro or try again at 6:12 PM."}}`)
	if r.Completed || r.Reason != ReasonUsageLimit || !r.ResetAt.Equal(time.Date(2026, 11, 7, 18, 12, 0, 0, time.Local)) {
		t.Fatalf("codex limit %+v", r)
	}
	r = feed("codex", `{"type":"turn.failed","error":{"message":"stream disconnected before completion"}}`)
	if r.Reason != ReasonOther || r.Failure != "Codex turn.failed: stream disconnected before completion" {
		t.Fatalf("codex unknown %+v", r)
	}
}
