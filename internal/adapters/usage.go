package adapters

// Usage is what a worker run's provider reported it spent (T-058). Input
// counts every input token, cached ones included; Cached is the part read
// from the provider's cache. Only Claude reports a price; HasCost says one
// was reported. It only informs the owner and never changes a run's outcome.
type Usage struct {
	Input, Cached, Output int64
	// CacheWrite is the part of Input written to the cache (T-088); Codex
	// reports none.
	CacheWrite int64
	Cost       float64
	HasCost    bool
}

// count reads a non-negative whole number of tokens.
func count(v any) (int64, bool) {
	f, ok := v.(float64)
	if !ok || f < 0 || f != float64(int64(f)) {
		return 0, false
	}
	return int64(f), true
}

// noteUsage adds one report to the run's total. A report without any
// readable number adds nothing.
func (r *TaskEventResult) noteUsage(input, cached, output []any, cost any, write ...any) {
	var add Usage
	seen := false
	sum := func(dst *int64, values []any) {
		for _, v := range values {
			if n, ok := count(v); ok {
				*dst += n
				seen = true
			}
		}
	}
	sum(&add.Input, input)
	sum(&add.Cached, cached)
	sum(&add.Output, output)
	sum(&add.CacheWrite, write)
	if c, ok := cost.(float64); ok && c >= 0 {
		add.Cost, add.HasCost, seen = c, true, true
	}
	if !seen {
		return
	}
	if r.Usage == nil {
		r.Usage = &Usage{}
	}
	r.Usage.Input += add.Input
	r.Usage.Cached += add.Cached
	r.Usage.Output += add.Output
	r.Usage.CacheWrite += add.CacheWrite
	if add.HasCost {
		r.Usage.Cost += add.Cost
		r.Usage.HasCost = true
	}
}

// Claude's result: input_tokens excludes the cache, so cache writes and reads
// are added to it.
func claudeUsage(e map[string]any, r *TaskEventResult) {
	u, _ := e["usage"].(map[string]any)
	if u == nil && e["total_cost_usd"] == nil {
		return
	}
	read := u["cache_read_input_tokens"]
	r.noteUsage([]any{u["input_tokens"], u["cache_creation_input_tokens"], read}, []any{read}, []any{u["output_tokens"]}, e["total_cost_usd"], u["cache_creation_input_tokens"])
}

// Codex's turn.completed: input_tokens already includes the cached part.
func codexUsage(e map[string]any, r *TaskEventResult) {
	u, _ := e["usage"].(map[string]any)
	if u == nil {
		return
	}
	r.noteUsage([]any{u["input_tokens"]}, []any{u["cached_input_tokens"]}, []any{u["output_tokens"]}, nil)
}

// Cursor's result may carry usage in camel case; its format is not
// documented, so a missing or different block just shows nothing.
func cursorUsage(e map[string]any, r *TaskEventResult) {
	u, _ := e["usage"].(map[string]any)
	if u == nil {
		return
	}
	read := u["cacheReadTokens"]
	r.noteUsage([]any{u["inputTokens"], u["cacheWriteTokens"], read}, []any{read}, []any{u["outputTokens"]}, nil, u["cacheWriteTokens"])
}

// Claude reports a model's capacity on the result. Match it to the latest
// top-level assistant request; aggregate result.usage spans multiple calls.
func (r *TaskEventResult) noteClaudeContext(e map[string]any) {
	if e["parent_tool_use_id"] != nil {
		return
	}
	switch e["type"] {
	case "system":
		if e["subtype"] == "compact_boundary" {
			r.contextModel = ""
			r.ContextCapacity = 0
		}
	case "assistant":
		r.contextModel = ""
		r.ContextCapacity = 0
		m, _ := e["message"].(map[string]any)
		u, _ := m["usage"].(map[string]any)
		input, ok := count(u["input_tokens"])
		if !ok {
			return
		}
		for _, key := range []string{"cache_read_input_tokens", "cache_creation_input_tokens"} {
			if v, present := u[key]; present {
				n, ok := count(v)
				if !ok || n > 1<<53 || input > 1<<53-n {
					return
				}
				input += n
			}
		}
		r.contextModel, _ = m["model"].(string)
		r.ContextUsed = input
	case "result":
		models, _ := e["modelUsage"].(map[string]any)
		usage, _ := models[r.contextModel].(map[string]any)
		if limit, ok := count(usage["contextWindow"]); ok && limit > 0 && r.contextModel != "" {
			r.ContextCapacity = limit
		}
	}
}
