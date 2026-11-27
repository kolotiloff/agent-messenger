package core

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"time"
)

type distribution struct {
	Samples int     `json:"samples"`
	Mean    float64 `json:"mean,omitempty"`
	Median  float64 `json:"median,omitempty"`
	P90     float64 `json:"p90,omitempty"`
}

func summarizeSamples(values []float64) distribution {
	n := len(values)
	if n == 0 {
		return distribution{}
	}
	slices.Sort(values)
	var total float64
	for _, value := range values {
		total += value
	}
	return distribution{
		Samples: n,
		Mean:    total / float64(n),
		Median:  (values[(n-1)/2] + values[n/2]) / 2,
		P90:     values[n-1-n/10],
	}
}

type efficiencyCohort struct {
	Agent      string `json:"agent"`
	Model      string `json:"model"`
	Type       string `json:"task_type"`
	Difficulty string `json:"difficulty"`
}

type efficiencyTokens struct {
	NewInput   float64 `json:"new_input_tokens"`
	CacheRead  float64 `json:"cache_read_tokens"`
	CacheWrite float64 `json:"cache_write_tokens"`
	Output     float64 `json:"output_tokens"`
}

func (t *efficiencyTokens) add(u *TaskUsage) {
	t.NewInput += float64(u.InputTokens - u.CachedTokens - u.CacheWriteTokens)
	t.CacheRead += float64(u.CachedTokens)
	t.CacheWrite += float64(u.CacheWriteTokens)
	t.Output += float64(u.OutputTokens)
}

func (t *efficiencyTokens) merge(u efficiencyTokens) {
	t.NewInput += u.NewInput
	t.CacheRead += u.CacheRead
	t.CacheWrite += u.CacheWrite
	t.Output += u.Output
}

type modelEfficiency struct {
	efficiencyCohort
	Project                  string           `json:"project,omitempty"`
	InitialModel             *exportModel     `json:"initial_model"`
	Tasks                    int              `json:"tasks"`
	Reviewed                 int              `json:"reviewed"`
	Accepted                 int              `json:"accepted"`
	Returned                 int              `json:"returned"`
	Unreviewed               int              `json:"unreviewed"`
	Open                     int              `json:"open"`
	FailedOrCancelled        int              `json:"failed_or_cancelled"`
	FirstPassKnown           int              `json:"first_pass_known"`
	FirstPassAccepted        int              `json:"first_pass_accepted"`
	ReturnReasons            map[string]int   `json:"return_reasons,omitempty"`
	AcceptedTokens           distribution     `json:"accepted_new_input_plus_output"`
	AcceptedUsage            efficiencyTokens `json:"accepted_usage_totals"`
	AcceptedRuntimeMS        distribution     `json:"accepted_active_runtime_ms"`
	TimeToAcceptanceMS       distribution     `json:"time_to_acceptance_ms"`
	tokens, runtime, elapsed []float64
}

// UsageEfficiency is computed on request from committed project records.
// Filters match the initial assignment so retries on other models or
// computers still contribute to the cost of that assignment.
func (s *Store) UsageEfficiency(ctx context.Context, queue string, since time.Time, computer string, filter UsageFilter) ([]modelEfficiency, error) {
	if !ValidUsageProjects(filter.Project, filter.Projects, false) {
		return nil, fmt.Errorf("invalid project scope")
	}
	groups := []modelEfficiency{}
	for _, project := range filter.projectNames() {
		scope := UsageFilter{Project: project, Agent: filter.Agent, Model: filter.Model}
		err := s.readUsageProject(ctx, queue, project, func(tx *sql.Tx, broker string) error {
			batch, err := scopedModelEfficiency(ctx, tx, since, computer, broker, scope, len(filter.Projects) != 0)
			groups = append(groups, batch...)
			return err
		})
		if err != nil {
			return nil, err
		}
	}
	return groups, nil
}

func scopedModelEfficiency(ctx context.Context, db *sql.Tx, since time.Time, computer, broker string, filter UsageFilter, includeProject bool) ([]modelEfficiency, error) {
	groups, err := readModelEfficiency(ctx, db, since, computer, broker, filter)
	if includeProject {
		for i := range groups {
			groups[i].Project = filter.Project
		}
	}
	return groups, err
}

type efficiencyIssue struct {
	root  *usageExportTask
	tasks []usageExportTask
}

func readModelEfficiency(ctx context.Context, db *sql.Tx, since time.Time, computer, broker string, filter UsageFilter) ([]modelEfficiency, error) {
	issues := map[string]*efficiencyIssue{}
	err := visitUsageExportTasks(ctx, db, since, computer, broker, filter, true, func(task usageExportTask) error {
		id := task.ID
		if task.Assignment != nil && task.Assignment.OriginalID != "" {
			id = task.Assignment.OriginalID
		}
		issue := issues[id]
		if issue == nil {
			issue = &efficiencyIssue{}
			issues[id] = issue
		}
		if id == task.ID {
			issue.root = &task
		}
		issue.tasks = append(issue.tasks, task)
		return nil
	})
	if err != nil {
		return nil, err
	}
	groups := map[efficiencyCohort]*modelEfficiency{}
	for _, issue := range issues {
		root := issue.root
		if root == nil || (filter.Agent != "" && filter.Agent != root.Agent) ||
			(computer != "" && !(computer == "local" && root.local) && computer != root.Computer) {
			continue
		}
		key := efficiencyCohort{Agent: root.Agent}
		if len(root.Attempts) > 0 && root.Attempts[0].Ordinal == 1 {
			key.Model = root.Attempts[0].Model
		}
		// With only one recorded local run, the last observed model also
		// describes the first. Older databases may lack attempt records.
		if key.Model == "" && root.singleLocalRun && root.Reported != nil {
			key.Model = root.Reported.ID
			if root.Reported.Effort != "" {
				key.Model += "[effort=" + root.Reported.Effort + "]"
			}
		}
		selection := key.Model
		if selection == "" && root.Requested != nil {
			selection = root.Requested.ID
			if effort := root.Requested.Effort; effort != "" {
				selection += "[effort=" + effort + "]"
			}
		}
		if filter.Model != "" && filter.Model != selection {
			continue
		}
		if root.Assignment != nil {
			key.Type, key.Difficulty = root.Assignment.Type, root.Assignment.Difficulty
		}
		g := groups[key]
		if g == nil {
			g = &modelEfficiency{efficiencyCohort: key, InitialModel: usageExportModel(key.Model)}
			groups[key] = g
		}
		g.recordIssue(root, issue.tasks)
	}
	out := make([]modelEfficiency, 0, len(groups))
	for _, g := range groups {
		g.AcceptedTokens = summarizeSamples(g.tokens)
		g.AcceptedRuntimeMS = summarizeSamples(g.runtime)
		g.TimeToAcceptanceMS = summarizeSamples(g.elapsed)
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Agent != b.Agent {
			return a.Agent < b.Agent
		}
		if a.Model != b.Model {
			return a.Model < b.Model
		}
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		return a.Difficulty < b.Difficulty
	})
	return out, nil
}

func (g *modelEfficiency) recordIssue(root *usageExportTask, tasks []usageExportTask) {
	g.Tasks++
	var latest time.Time
	var review TaskReview
	var reviewedAt time.Time
	closed, failed := true, false
	for _, task := range tasks {
		finished, _ := time.Parse(time.RFC3339Nano, task.Finished)
		if task.State != "done" && task.State != "failed" && task.State != "cancelled" || finished.IsZero() {
			closed = false
		}
		failed = failed || task.State == "failed" || task.State == "cancelled"
		if finished.After(latest) {
			latest = finished
		}
		for _, r := range task.Reviews {
			if r.Result == "returned" {
				if g.ReturnReasons == nil {
					g.ReturnReasons = map[string]int{}
				}
				g.ReturnReasons[r.Reason]++
			}
			when, _ := time.Parse(time.RFC3339Nano, r.ReviewedAt)
			if r.ReportAt == task.Finished && when.After(reviewedAt) {
				review, reviewedAt = r, when
			}
		}
	}
	if len(root.Reviews) > 0 {
		r := root.Reviews[0]
		for i, attempt := range root.Attempts {
			// A continuation before the first explicit review is ambiguous.
			if attempt.Kind == "continue" || attempt.Ordinal != i+1 {
				break
			}
			if attempt.OutcomeAt != "" && attempt.OutcomeAt == r.ReportAt {
				g.FirstPassKnown++
				if r.Result == "accepted" {
					g.FirstPassAccepted++
				}
				break
			}
		}
	}
	switch {
	case !closed:
		g.Open++
	case review.Result == "" || reviewedAt.Before(latest):
		if failed {
			g.FailedOrCancelled++
		} else {
			g.Unreviewed++
		}
	case review.Result == "returned":
		g.Reviewed++
		g.Returned++
	case review.Result == "accepted":
		g.Reviewed++
		g.Accepted++
		g.recordAccepted(root, tasks, review.ReviewedAt)
	}
}

func (g *modelEfficiency) recordAccepted(root *usageExportTask, tasks []usageExportTask, reviewedAt string) {
	if ms := taskDurationMS(root.Created, reviewedAt); ms != nil {
		g.elapsed = append(g.elapsed, float64(*ms))
	}
	var usage efficiencyTokens
	var runtime float64
	hasUsage, hasRuntime := true, true
	for _, task := range tasks {
		attempts := task.Attempts
		// A terminal marker ties the last attempt to this exact outcome.
		// Old clients and restarts may leave only the aggregate total.
		if len(attempts) == 0 || attempts[len(attempts)-1].OutcomeAt != task.Finished {
			return
		}
		var taskUsage efficiencyTokens
		for i, attempt := range attempts {
			if attempt.Ordinal != i+1 {
				return
			}
			if !attempt.Usage.valid() {
				hasUsage = false
			} else {
				taskUsage.add(attempt.Usage)
			}
			if ms := attempt.DurationMS; ms != nil {
				runtime += float64(*ms)
			} else {
				hasRuntime = false
			}
		}
		var total efficiencyTokens
		if task.Usage != nil {
			total.add(&task.Usage.TaskUsage)
		}
		hasUsage = hasUsage && task.Usage != nil && total == taskUsage
		usage.merge(taskUsage)
	}
	if hasUsage {
		g.tokens = append(g.tokens, usage.NewInput+usage.Output)
		g.AcceptedUsage.merge(usage)
	}
	if hasRuntime {
		g.runtime = append(g.runtime, runtime)
	}
}
