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

func (t *UsageTotal) add(in, cached, out int64) {
	t.Tasks++
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

// UsageStats is the window's statistics page.
type UsageStats struct {
	Since     string       `json:"since"`
	Total     UsageTotal   `json:"total"`
	Agents    []UsageGroup `json:"agents"`
	Computers []UsageGroup `json:"computers"`
	Days      []UsageDay   `json:"days"`
	Top       []UsageTask  `json:"top"`
}

// MaxUsageTop is how many of the largest tasks the page lists.
const MaxUsageTop = 10

// UsageStats sums the tasks with reported usage that finished (or were last
// updated) at or after since. Days are local days of loc, oldest first,
// including days without tasks.
func (s *Store) UsageStats(since time.Time, loc *time.Location) (UsageStats, error) {
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
		rows, err := tx.Query(`SELECT t.id,t.title,t.project,t.provider,COALESCE(t.remote_peer,''),COALESCE(p.name,''),
 COALESCE(t.finished_at,t.updated_at),t.usage_input,t.usage_cached,t.usage_output
 FROM tasks t LEFT JOIN net_peers p ON p.peer=t.remote_peer
 WHERE t.usage_seen=1 AND COALESCE(t.finished_at,t.updated_at)>=?`, out.Since)
		if err != nil {
			return err
		}
		defer rows.Close()
		agents := map[string]*UsageGroup{}
		computers := map[string]*UsageGroup{}
		days := map[string]*UsageDay{}
		for rows.Next() {
			var task UsageTask
			var peer, peerName string
			var in, cached, outTokens int64
			if err := rows.Scan(&task.ID, &task.Title, &task.Project, &task.Agent, &peer, &peerName, &task.At, &in, &cached, &outTokens); err != nil {
				return err
			}
			task.Tokens = in + outTokens
			out.Total.add(in, cached, outTokens)
			a := agents[task.Agent]
			if a == nil {
				a = &UsageGroup{Key: task.Agent, Name: task.Agent}
				agents[task.Agent] = a
			}
			a.add(in, cached, outTokens)
			key, name, this := local, localName, true
			if peer != "" {
				key, name, this = peer, peerName, false
			}
			c := computers[key]
			if c == nil {
				c = &UsageGroup{Key: key, Name: name, This: this}
				computers[key] = c
			}
			c.add(in, cached, outTokens)
			task.Computer = name
			if at, err := time.Parse(time.RFC3339Nano, task.At); err == nil {
				day := at.In(loc).Format("2006-01-02")
				d := days[day]
				if d == nil {
					d = &UsageDay{Day: day}
					days[day] = d
				}
				d.add(in, cached, outTokens)
			}
			out.Top = append(out.Top, task)
		}
		if err := rows.Err(); err != nil {
			return err
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
