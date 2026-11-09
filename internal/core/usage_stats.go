package core

import (
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

// UsageTask is one of the largest tasks of the period.
type UsageTask struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Project  string `json:"project"`
	Agent    string `json:"agent"`
	Computer string `json:"computer"`
	At       string `json:"at"`
	Tokens   int64  `json:"tokens"`
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
	Since string `json:"since"`
	// Computer names the computer the page is limited to; empty for the
	// whole network.
	Computer  string       `json:"computer,omitempty"`
	Total     UsageTotal   `json:"total"`
	Agents    []UsageGroup `json:"agents"`
	Computers []UsageGroup `json:"computers"`
	Days      []UsageDay   `json:"days"`
	Top       []UsageTask  `json:"top"`
}

// MaxUsageTop is how many of the largest tasks the page lists.
const MaxUsageTop = 10

// UsageStats sums what tasks spent at or after since (T-068, T-076).
// Tasks that ran on this computer count from its own records; tasks that
// ran on another computer count from that computer's daily report, or from
// this computer's records of the tasks it gave there when that computer
// sent no report (an older version). computer limits the page to "local"
// or one peer; empty means the whole network. Days are local days of loc,
// oldest first, including days without tasks; the largest tasks are the
// ones this computer knows.
func (s *Store) UsageStats(since time.Time, loc *time.Location, computer string) (UsageStats, error) {
	out := UsageStats{Since: since.UTC().Format(time.RFC3339Nano), Agents: []UsageGroup{}, Computers: []UsageGroup{}, Days: []UsageDay{}, Top: []UsageTask{}}
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
		reported := map[string]NetPeer{}
		for _, p := range peers {
			if p.Usage != nil {
				reported[p.ID] = p
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
		rows, err := tx.Query(`SELECT t.id,t.title,t.project,t.provider,COALESCE(t.remote_peer,''),COALESCE(p.name,''),
 COALESCE(t.finished_at,t.updated_at),t.usage_input,t.usage_cached,t.usage_output
 FROM tasks t LEFT JOIN net_peers p ON p.peer=t.remote_peer
 WHERE t.usage_seen=1 AND COALESCE(t.finished_at,t.updated_at)>=?`, out.Since)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var task UsageTask
			var peer, peerName string
			var in, cached, outTokens int64
			if err := rows.Scan(&task.ID, &task.Title, &task.Project, &task.Agent, &peer, &peerName, &task.At, &in, &cached, &outTokens); err != nil {
				return err
			}
			task.Tokens = in + outTokens
			key, name, this := local, localName, true
			if peer != "" {
				key, name, this = peer, peerName, false
			}
			if !wanted(key) {
				continue
			}
			task.Computer = name
			out.Top = append(out.Top, task)
			if _, ok := reported[peer]; peer != "" && ok {
				continue // counted from that computer's own report
			}
			day := ""
			if at, err := time.Parse(time.RFC3339Nano, task.At); err == nil {
				day = at.In(loc).Format("2006-01-02")
			}
			count(key, name, this, task.Agent, day, 1, in, cached, outTokens)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		firstDay := since.In(loc).Format("2006-01-02")
		for _, p := range peers {
			if p.Usage == nil || !wanted(p.ID) {
				continue
			}
			for _, u := range p.Usage {
				if u.Day < firstDay || u.Tasks <= 0 {
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
		byTokens(out.Agents)
		byTokens(out.Computers)
		sort.Slice(out.Top, func(i, j int) bool {
			if out.Top[i].Tokens != out.Top[j].Tokens {
				return out.Top[i].Tokens > out.Top[j].Tokens
			}
			return out.Top[i].ID < out.Top[j].ID
		})
		if len(out.Top) > MaxUsageTop {
			out.Top = out.Top[:MaxUsageTop]
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
		rows, err := tx.Query(`SELECT provider,COALESCE(finished_at,updated_at),usage_input,usage_cached,usage_output FROM tasks
 WHERE usage_seen=1 AND COALESCE(remote_peer,'')='' AND COALESCE(finished_at,updated_at)>=?`, since.UTC().Format(time.RFC3339Nano))
		if err != nil {
			return err
		}
		defer rows.Close()
		sums := map[[2]string]*UsageDayAgent{}
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
		if err := rows.Err(); err != nil {
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
