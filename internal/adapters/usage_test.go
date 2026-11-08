package adapters

import "testing"

// Each provider's event stream says what the run spent; the window shows it
// on the task (T-058). Only Claude reports a price.
func TestTaskUsage(t *testing.T) {
	var c TaskEventResult
	ParseTaskEvent("claude", []byte(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","result":"ok","total_cost_usd":0.4213,"usage":{"input_tokens":12,"cache_creation_input_tokens":3000,"cache_read_input_tokens":40000,"output_tokens":900}}`), &c)
	if c.Usage == nil || c.Usage.Input != 43012 || c.Usage.Cached != 40000 || c.Usage.Output != 900 || !c.Usage.HasCost || c.Usage.Cost != 0.4213 {
		t.Fatalf("claude %+v", c.Usage)
	}

	var x TaskEventResult
	ParseTaskEvent("codex", []byte(`{"type":"turn.completed","usage":{"input_tokens":24763,"cached_input_tokens":24448,"output_tokens":122}}`), &x)
	ParseTaskEvent("codex", []byte(`{"type":"turn.completed","usage":{"input_tokens":1000,"cached_input_tokens":0,"output_tokens":10}}`), &x)
	if x.Usage == nil || x.Usage.Input != 25763 || x.Usage.Cached != 24448 || x.Usage.Output != 132 || x.Usage.HasCost {
		t.Fatalf("codex %+v", x.Usage)
	}

	var u TaskEventResult
	ParseTaskEvent("cursor", []byte(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","result":"ok","usage":{"inputTokens":500,"outputTokens":40,"cacheReadTokens":300,"cacheWriteTokens":20}}`), &u)
	if u.Usage == nil || u.Usage.Input != 820 || u.Usage.Cached != 300 || u.Usage.Output != 40 || u.Usage.HasCost {
		t.Fatalf("cursor %+v", u.Usage)
	}

	// No usage reported: nothing is shown, not zero.
	var none TaskEventResult
	ParseTaskEvent("cursor", []byte(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","result":"ok"}`), &none)
	ParseTaskEvent("claude", []byte(`{"type":"result","subtype":"success","is_error":false,"session_id":"s","result":"ok"}`), &none)
	if none.Usage != nil {
		t.Fatalf("invented usage %+v", none.Usage)
	}
	// Nonsense numbers are ignored.
	var bad TaskEventResult
	ParseTaskEvent("codex", []byte(`{"type":"turn.completed","usage":{"input_tokens":-5,"output_tokens":"x"}}`), &bad)
	if bad.Usage != nil {
		t.Fatalf("bad usage %+v", bad.Usage)
	}
}
