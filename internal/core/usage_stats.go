package core

import (
	"context"
	"database/sql"
	"sort"
	"time"
)

// Token statistics (T-068): what the tasks this computer ran or gave to
// another computer spent over a period, by agent, by computer, by day and
// the largest tasks. A task counts on the day it finished, or was last
// updated while it still runs. Tokens are input plus output; the cached
// part is inside the input.

// UsageTotal is a sum of tokens over some tasks.
type UsageTotal struct {
	Tasks        int   `json:"tasks"`
	Tokens       int64 `json:"tokens"`
	InputTokens  int64 `json:"input_tokens"`
	CachedTokens int64 `json:"cached_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

func (t *UsageTotal) add(in, cached, out int64) { t.addN(1, in, cached, out) }

func (t *UsageTotal) addN(tasks int, in, cached, out int64) {
	t.Tasks += tasks
	t.InputTokens += in
	t.CachedTokens += cached
	t.OutputTokens += out
	t.Tokens += in + out
}

// UsageGroup is the total of one agent or one computer.
type UsageGroup struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	This bool   `json:"this,omitempty"`
	UsageTotal
}

// UsageDay is the total of one local day.
type UsageDay struct {
	Day string `json:"day"`
	UsageTotal
}

// UsageTask is a task in one of the period's ranked lists.
type UsageTask struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Project  string `json:"project"`
	Agent    string `json:"agent"`
	Computer string `json:"computer"`
	At       string `json:"at"`
	Tokens   int64  `json:"tokens"`
	Model    string `json:"model,omitempty"`
	// The parts of Tokens, shown on hover (T-197).
	Input             int64  `json:"input_tokens"`
	Cached            int64  `json:"cached_tokens"`
	Output            int64  `json:"output_tokens"`
	DurationMS        *int64 `json:"duration_ms,omitempty"`
	DurationEstimated bool   `json:"duration_estimated,omitempty"`
	UsageMissing      bool   `json:"usage_missing,omitempty"`
	atTime            time.Time
}

// UsageDayAgent is what one agent's tasks that ran on one computer spent
// on one of its local days (T-076). Computers report these to each other.
type UsageDayAgent struct {
	Day    string `json:"day"`
	Agent  string `json:"agent"`
	Tasks  int    `json:"tasks"`
	Input  int64  `json:"input_tokens"`
	Cached int64  `json:"cached_tokens"`
	Output int64  `json:"output_tokens"`
}

// UsageStats is the window's statistics page.
type UsageStats struct {
	Since   string `json:"since"`
	Project string `json:"project,omitempty"`
	// Computer names the computer the page is limited to; empty for the
	// whole network.
	Computer  string       `json:"computer,omitempty"`
	Total     UsageTotal   `json:"total"`
	Agents    []UsageGroup `json:"agents"`
	Computers []UsageGroup `json:"computers"`
	Days      []UsageDay   `json:"days"`
	Top       []UsageTask  `json:"top"`
	// Recent are the tasks that finished last, newest first (T-197).
	Recent []UsageTask `json:"recent"`
	// Other orders are ranked across the period, before applying the row limit.
	Ranked map[string][]UsageTask `json:"ranked"`
	// The filters the page was counted with (T-201), and the models of the
	// tasks this computer knows in the period, for the model list.
	Agent  string       `json:"agent,omitempty"`
	Model  string       `json:"model,omitempty"`
	Models []UsageGroup `json:"models"`
}

// UsageFilter limits statistics by project, agent and model. Peer summaries
// are per agent only, so project or model filters use known task records.
type UsageFilter struct {
	Project string
	Agent   string
	Model   string
}

// MaxUsageTop is how many tasks the page lists in each order.
const MaxUsageTop = 10

func taskDurationMS(started, finished string) *int64 {
	start, startErr := time.Parse(time.RFC3339Nano, started)
	end, endErr := time.Parse(time.RFC3339Nano, finished)
	if startErr != nil || endErr != nil || end.Before(start) {
		return nil
	}
	ms := end.Sub(start).Milliseconds()
	return &ms
}

func usageTaskDuration(local bool, finished, started, observedStart string, remote sql.NullInt64) (*int64, bool) {
	if finished == "" {
		return nil, false
	}
	if local {
		return taskDurationMS(started, finished), false
	}
	if remote.Valid {
		return &remote.Int64, false
	}
	// Older remote records may only have local receipt times.
	duration := taskDurationMS(observedStart, finished)
	return duration, duration != nil
}

// rankUsageTasks can reuse the input for the next order after copying the page.
func rankUsageTasks(tasks []UsageTask, less func(a, b UsageTask) bool) []UsageTask {
	sort.Slice(tasks, func(i, j int) bool { return less(tasks[i], tasks[j]) })
	page := tasks[:min(len(tasks), MaxUsageTop)]
	return append(make([]UsageTask, 0, len(page)), page...)
}

// UsageStats sums what tasks spent at or after since (T-068, T-076).
// Tasks that ran on this computer count from its own records; tasks that
// ran on another computer count from that computer's daily report, or from
// this computer's records of the tasks it gave there when that computer
// sent no report (an older version). computer limits the page to "local"
// or one peer; empty means the whole network. Days are local days of loc,
// oldest first, including days without tasks; the largest tasks are the
// ones this computer knows.
func (s *Store) UsageStats(since time.Time, loc *time.Location, computer string) (UsageStats, error) {
	return s.UsageStatsFiltered(since, loc, computer, UsageFilter{})
}

// UsageStatsFiltered applies the requested project, agent and model filters.
func (s *Store) UsageStatsFiltered(since time.Time, loc *time.Location, computer string, filter UsageFilter) (UsageStats, error) {
	out := UsageStats{Since: since.UTC().Format(time.RFC3339Nano), Project: filter.Project, Agents: []UsageGroup{}, Computers: []UsageGroup{}, Days: []UsageDay{}, Top: []UsageTask{}, Recent: []UsageTask{}, Models: []UsageGroup{}, Agent: filter.Agent, Model: filter.Model}
	err := s.tx(func(tx *sql.Tx) error {
		local, err := localComputer(tx)
		if err != nil {
			return err
		}
		localName, err := computerName(tx)
		if err != nil {
			return err
		}
		peers, err := netPeers(tx)
		if err != nil {
			return err
		}
		peerNames := make(map[string]string, len(peers))
		for _, peer := range peers {
			peerNames[peer.ID] = peer.Name
		}
		reported := map[string]NetPeer{}
		if filter.Project == "" && filter.Model == "" {
			for _, p := range peers {
				if p.Usage != nil {
					reported[p.ID] = p
				}
			}
		}
		wanted := func(key string) bool {
			return computer == "" || (computer == "local" && key == local) || computer == key
		}
		switch {
		case computer == "local":
			out.Computer = localName
		case computer != "":
			for _, p := range peers {
				if p.ID == computer {
					out.Computer = p.Name
				}
			}
		}
		agents := map[string]*UsageGroup{}
		computers := map[string]*UsageGroup{}
		days := map[string]*UsageDay{}
		count := func(key, name string, this bool, agent, day string, tasks int, in, cached, outTokens int64) {
			out.Total.addN(tasks, in, cached, outTokens)
			a := agents[agent]
			if a == nil {
				a = &UsageGroup{Key: agent, Name: agent}
				agents[agent] = a
			}
			a.addN(tasks, in, cached, outTokens)
			c := computers[key]
			if c == nil {
				c = &UsageGroup{Key: key, Name: name, This: this}
				computers[key] = c
			}
			c.addN(tasks, in, cached, outTokens)
			if day != "" {
				d := days[day]
				if d == nil {
					d = &UsageDay{Day: day}
					days[day] = d
				}
				d.addN(tasks, in, cached, outTokens)
			}
		}
		models := map[string]*UsageGroup{}
		var used, dated, timed []UsageTask
		broker, err := currentBroker(tx)
		if err != nil {
			return err
		}
		query := `SELECT t.id,t.title,t.project,t.provider,COALESCE(t.remote_peer,''),COALESCE(t.remote_broker,''),COALESCE(t.remote_name,''),
 COALESCE(t.finished_at,t.updated_at),t.usage_input,t.usage_cached,t.usage_output,
 COALESCE(NULLIF(t.ran_model,''),NULLIF(t.model,''),''),t.usage_seen,
 CASE WHEN t.state IN ('done','failed','cancelled') THEN COALESCE(t.finished_at,'') ELSE '' END,
 COALESCE((SELECT MIN(r.launched_at) FROM task_runs r WHERE r.task_id=t.id),''),
 t.remote_duration_ms,
 CASE WHEN t.remote_peer IS NOT NULL AND t.remote_duration_ms IS NULL AND t.state IN ('done','failed')
 THEN COALESCE((SELECT i.created_at FROM remote_task_items i WHERE i.task_id=t.id ORDER BY i.seq LIMIT 1),'') ELSE '' END
 FROM tasks t
 WHERE COALESCE(t.finished_at,t.updated_at)>=?
 AND (?='' OR t.project=?)
 AND (t.usage_seen=1 OR (t.finished_at IS NOT NULL AND t.state IN ('done','failed','cancelled')))`
		err = s.visitProjectTaskRows(tx, filter.Project, query, []any{out.Since, filter.Project, filter.Project}, func(rows *sql.Rows) error {
			for rows.Next() {
				var task UsageTask
				var peer, peerBroker, remoteName, finished, started, observedStart string
				var in, cached, outTokens int64
				var usageSeen bool
				var remoteDuration sql.NullInt64
				if err := rows.Scan(&task.ID, &task.Title, &task.Project, &task.Agent, &peer, &peerBroker, &remoteName, &task.At, &in, &cached, &outTokens,
					&task.Model, &usageSeen, &finished, &started, &remoteDuration, &observedStart); err != nil {
					return err
				}
				if filter.Agent != "" && task.Agent != filter.Agent {
					continue
				}
				task.Tokens = in + outTokens
				task.Input, task.Cached, task.Output = in, cached, outTokens
				key, name, this := local, localName, true
				if peer != "" {
					key, name, this = peer, peerNames[peer], false
					if broker == "" || peerBroker != broker {
						// Sent in an earlier network: that peer id may name
						// another computer now (linkComputer).
						key, peer = "old:"+peerBroker+"/"+peer, ""
						if name = remoteName; name == "" {
							name = formerComputer
						}
					}
				}
				if !wanted(key) {
					continue
				}
				task.DurationMS, task.DurationEstimated = usageTaskDuration(this, finished, started, observedStart, remoteDuration)
				if !usageSeen && task.DurationMS == nil {
					continue
				}
				// The model list counts every known task of the agent, whatever
				// model is chosen, so another model can be picked.
				if task.Model != "" {
					m := models[task.Model]
					if m == nil {
						m = &UsageGroup{Key: task.Model, Name: task.Model}
						models[task.Model] = m
					}
					if usageSeen {
						m.addN(1, in, cached, outTokens)
					}
				}
				if filter.Model != "" && task.Model != filter.Model {
					continue
				}
				task.Computer = name
				task.UsageMissing = !usageSeen
				task.atTime, _ = time.Parse(time.RFC3339Nano, task.At)
				dated = append(dated, task)
				if task.DurationMS != nil {
					timed = append(timed, task)
				}
				if !usageSeen {
					continue
				}
				used = append(used, task)
				if _, ok := reported[peer]; peer != "" && ok {
					continue // counted from that computer's own report
				}
				day := ""
				if !task.atTime.IsZero() {
					day = task.atTime.In(loc).Format("2006-01-02")
				}
				count(key, name, this, task.Agent, day, 1, in, cached, outTokens)
			}
			return rows.Err()
		})
		if err != nil {
			return err
		}
		firstDay := since.In(loc).Format("2006-01-02")
		for _, p := range reported {
			if !wanted(p.ID) {
				continue
			}
			for _, u := range p.Usage {
				if u.Day < firstDay || u.Tasks <= 0 || (filter.Agent != "" && u.Agent != filter.Agent) {
					continue
				}
				count(p.ID, p.Name, false, u.Agent, u.Day, u.Tasks, u.Input, u.Cached, u.Output)
			}
		}
		for _, a := range agents {
			out.Agents = append(out.Agents, *a)
		}
		for _, c := range computers {
			out.Computers = append(out.Computers, *c)
		}
		byTokens := func(g []UsageGroup) {
			sort.Slice(g, func(i, j int) bool {
				if g[i].Tokens != g[j].Tokens {
					return g[i].Tokens > g[j].Tokens
				}
				return g[i].Key < g[j].Key
			})
		}
		for _, m := range models {
			out.Models = append(out.Models, *m)
		}
		byTokens(out.Agents)
		byTokens(out.Computers)
		byTokens(out.Models)
		byDate := func(newest bool) func(a, b UsageTask) bool {
			return func(a, b UsageTask) bool {
				x, y := a.atTime, b.atTime
				if x.Equal(y) {
					return a.ID < b.ID
				}
				if x.IsZero() || y.IsZero() {
					return !x.IsZero()
				}
				if newest {
					return x.After(y)
				}
				return x.Before(y)
			}
		}
		byValue := func(value func(UsageTask) int64, largest bool) func(a, b UsageTask) bool {
			return func(a, b UsageTask) bool {
				x, y := value(a), value(b)
				if x == y {
					return a.ID < b.ID
				}
				if largest {
					return x > y
				}
				return x < y
			}
		}
		tokens := func(t UsageTask) int64 { return t.Tokens }
		duration := func(t UsageTask) int64 { return *t.DurationMS }
		out.Top = rankUsageTasks(used, byValue(tokens, true))
		out.Recent = rankUsageTasks(dated, byDate(true))
		out.Ranked = map[string][]UsageTask{
			"oldest":   rankUsageTasks(dated, byDate(false)),
			"fewest":   rankUsageTasks(used, byValue(tokens, false)),
			"longest":  rankUsageTasks(timed, byValue(duration, true)),
			"shortest": rankUsageTasks(timed, byValue(duration, false)),
		}
		start := since.In(loc)
		first := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, loc)
		last := time.Now().In(loc)
		for d := first; !d.After(last); d = d.AddDate(0, 0, 1) {
			day := d.Format("2006-01-02")
			if v := days[day]; v != nil {
				out.Days = append(out.Days, *v)
			} else {
				out.Days = append(out.Days, UsageDay{Day: day})
			}
		}
		return nil
	})
	return out, err
}

// MaxUsageDays is how many days a computer reports to the others.
const MaxUsageDays = 31

// UsageDaily sums, by local day of loc and agent, what the tasks that ran on
// this computer spent at or after since (T-076). Tasks this computer gave to
// another computer are left out: that computer reports them.
func (s *Store) UsageDaily(since time.Time, loc *time.Location) ([]UsageDayAgent, error) {
	out := []UsageDayAgent{}
	err := s.tx(func(tx *sql.Tx) error {
		query := `SELECT provider,COALESCE(finished_at,updated_at),usage_input,usage_cached,usage_output FROM tasks
 WHERE usage_seen=1 AND COALESCE(remote_peer,'')='' AND COALESCE(finished_at,updated_at)>=?`
		sums := map[[2]string]*UsageDayAgent{}
		err := s.visitProjectTaskRows(tx, "", query, []any{since.UTC().Format(time.RFC3339Nano)}, func(rows *sql.Rows) error {
			for rows.Next() {
				var agent, at string
				var in, cached, outTokens int64
				if err := rows.Scan(&agent, &at, &in, &cached, &outTokens); err != nil {
					return err
				}
				when, err := time.Parse(time.RFC3339Nano, at)
				if err != nil {
					continue
				}
				k := [2]string{when.In(loc).Format("2006-01-02"), agent}
				u := sums[k]
				if u == nil {
					u = &UsageDayAgent{Day: k[0], Agent: agent}
					sums[k] = u
				}
				u.Tasks++
				u.Input += in
				u.Cached += cached
				u.Output += outTokens
			}
			return rows.Err()
		})
		if err != nil {
			return err
		}
		for _, u := range sums {
			out = append(out, *u)
		}
		sort.Slice(out, func(i, j int) bool {
			if out[i].Day != out[j].Day {
				return out[i].Day < out[j].Day
			}
			return out[i].Agent < out[j].Agent
		})
		return nil
	})
	return out, err
}

// visitProjectTaskRows streams only the selected project's data when the caller
// has a scope. Peer labels and machine summaries stay in the computer DB.
func (s *Store) visitProjectTaskRows(machine *sql.Tx, project, query string, args []any, visit func(*sql.Rows) error) error {
	ctx := context.Background()
	read := func(source projectFileQuerier) error {
		rows, err := source.QueryContext(ctx, query, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		return visit(rows)
	}
	if s.split == nil {
		return read(machine)
	}
	if project != "" {
		file, err := splitTaskProjectFile(ctx, machine, project)
		if err != nil {
			return err
		}
		db, err := s.split.open(file)
		if err != nil {
			return err
		}
		return read(db)
	}
	files, err := splitProjectFiles(ctx, machine)
	if err != nil {
		return err
	}
	for _, file := range files {
		db, err := s.split.open(file)
		if err != nil {
			return err
		}
		if err := read(db); err != nil {
			return err
		}
	}
	return nil
}
