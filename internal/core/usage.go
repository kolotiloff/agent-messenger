package core

import (
	"database/sql"
	"fmt"
	"math"
	"strings"
)

// TaskUsage is what a task's runs spent, as their providers reported it
// (T-058). InputTokens counts every input token, CachedTokens included.
// CostUSD is set only when a provider reported a price (Claude does; Codex
// and Cursor report tokens only). It only informs the owner.
type TaskUsage struct {
	InputTokens  int64    `json:"input_tokens"`
	CachedTokens int64    `json:"cached_tokens"`
	OutputTokens int64    `json:"output_tokens"`
	// CacheWriteTokens is the part of InputTokens written to the cache
	// (T-088); 0 when the agent does not report it.
	CacheWriteTokens int64    `json:"cache_write_tokens,omitempty"`
	CostUSD          *float64 `json:"cost_usd,omitempty"`
}

// valid rejects negative or impossible numbers from a provider or a peer.
func (u *TaskUsage) valid() bool {
	if u == nil || u.InputTokens < 0 || u.CachedTokens < 0 || u.OutputTokens < 0 || u.CachedTokens > u.InputTokens || u.CacheWriteTokens < 0 || u.CachedTokens+u.CacheWriteTokens > u.InputTokens {
		return false
	}
	return u.CostUSD == nil || (*u.CostUSD >= 0 && !math.IsInf(*u.CostUSD, 0) && !math.IsNaN(*u.CostUSD))
}

// addTaskUsage adds one run's usage to its task.
func addTaskUsage(tx *sql.Tx, task string, u *TaskUsage) error {
	if !u.valid() {
		return nil
	}
	var cost any
	if u.CostUSD != nil {
		cost = *u.CostUSD
	}
	_, err := tx.Exec(`UPDATE tasks SET usage_seen=1,usage_input=usage_input+?,usage_cached=usage_cached+?,usage_output=usage_output+?,usage_cache_write=usage_cache_write+?,
 usage_cost=CASE WHEN ? IS NULL THEN usage_cost ELSE COALESCE(usage_cost,0)+? END WHERE id=?`,
		u.InputTokens, u.CachedTokens, u.OutputTokens, u.CacheWriteTokens, cost, cost, task)
	return err
}

// setTaskUsage keeps the total that the computer running a task reported.
func setTaskUsage(tx *sql.Tx, task string, u *TaskUsage) error {
	if !u.valid() {
		return nil
	}
	var cost any
	if u.CostUSD != nil {
		cost = *u.CostUSD
	}
	_, err := tx.Exec(`UPDATE tasks SET usage_seen=1,usage_input=?,usage_cached=?,usage_output=?,usage_cache_write=?,usage_cost=? WHERE id=?`,
		u.InputTokens, u.CachedTokens, u.OutputTokens, u.CacheWriteTokens, cost, task)
	return err
}

// taskUsage reads a task's total, or nil when no run reported any.
func taskUsage(tx *sql.Tx, task string) (*TaskUsage, error) {
	var seen int
	var u TaskUsage
	var cost sql.NullFloat64
	if err := tx.QueryRow(`SELECT usage_seen,usage_input,usage_cached,usage_output,usage_cache_write,usage_cost FROM tasks WHERE id=?`, task).
		Scan(&seen, &u.InputTokens, &u.CachedTokens, &u.OutputTokens, &u.CacheWriteTokens, &cost); err != nil {
		return nil, err
	}
	if seen == 0 {
		return nil, nil
	}
	if cost.Valid {
		u.CostUSD = &cost.Float64
	}
	return &u, nil
}

func (s *Store) taskUsage(task string) (*TaskUsage, error) {
	var u *TaskUsage
	err := s.tx(func(tx *sql.Tx) error {
		var err error
		u, err = taskUsage(tx, task)
		return err
	})
	return u, err
}

// usageLine is the coordinator's line about what a task spent (T-088): new
// input, cache write, cache read and output tokens.
func usageLine(u *TaskUsage) string {
	if u == nil {
		return ""
	}
	fresh := u.InputTokens - u.CachedTokens - u.CacheWriteTokens
	write := fmt.Sprint(u.CacheWriteTokens)
	if u.CacheWriteTokens == 0 {
		write = "0 or not reported"
	}
	line := fmt.Sprintf("[Messenger] Tokens of this task so far: new input %d, cache write %s, cache read %d, output %d", fresh, write, u.CachedTokens, u.OutputTokens)
	if u.CostUSD != nil {
		line += fmt.Sprintf(", price the agent reported $%.4f", *u.CostUSD)
	}
	return line + "."
}

// withUsageNote adds the task's usage line to an event text.
func withUsageNote(tx *sql.Tx, task, text string) (string, error) {
	if strings.Contains(text, "[Messenger] Tokens of this task") {
		return text, nil
	}
	u, err := taskUsage(tx, task)
	if err != nil || u == nil {
		return text, err
	}
	return strings.TrimSpace(text + "\n" + usageLine(u)), nil
}
