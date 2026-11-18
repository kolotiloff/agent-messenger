package core

import (
	"testing"
	"time"
)

// Token statistics (T-068): sums by agent, by computer, by day and the
// largest tasks; tasks without reported usage and tasks before the period
// are left out.
func TestUsageStats(t *testing.T) {
	f := newTaskFixture(t)
	finish := func(op, agent, title string, in, cached, out int64) string {
		id := f.start(t, op, agent, title)
		session := "019a0000-0000-7000-8000-0000000000" + op[len(op)-2:]
		if agent == "claude" {
			session = f.sessionOf(t, id)
		}
		r := f.launch(t, id, session)
		f.dispose(t, r, DispositionReport, "Done")
		ev := cleanExit
		if in+out > 0 {
			ev.Usage = &TaskUsage{InputTokens: in, CachedTokens: cached, OutputTokens: out}
		}
		if _, err := f.st.FinishTaskRun(r.ID, ev); err != nil {
			t.Fatal(err)
		}
		return id
	}
	a := finish("op-01", "claude", "Menu", 1000, 800, 100)
	finish("op-02", "codex", "Build", 5000, 0, 500)
	finish("op-03", "codex", "Nothing reported", 0, 0, 0)
	old := finish("op-04", "claude", "Old", 9999, 0, 1)
	remote := finish("op-05", "codex", "On the Mac", 300, 0, 30)
	if _, err := f.st.db.Exec(`UPDATE tasks SET finished_at='2000-01-01T00:00:00Z' WHERE id=?`, old); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.db.Exec(`INSERT INTO net_peers(peer,name,host,age,agents,updated_at) VALUES ('peer-mac','Mac',0,0,'[]','2026-01-01T00:00:00Z')`); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.db.Exec(`UPDATE tasks SET remote_peer='peer-mac',remote_broker='b',remote_task='x' WHERE id=?`, remote); err != nil {
		t.Fatal(err)
	}
	since := time.Now().Add(-7 * 24 * time.Hour)
	s, err := f.st.UsageStats(since, time.Local, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.Total.Tasks != 3 || s.Total.Tokens != 1100+5500+330 || s.Total.CachedTokens != 800 {
		t.Fatalf("total %+v", s.Total)
	}
	if len(s.Agents) != 2 || s.Agents[0].Key != "codex" || s.Agents[0].Tokens != 5830 || s.Agents[0].Tasks != 2 || s.Agents[1].Tokens != 1100 {
		t.Fatalf("agents %+v", s.Agents)
	}
	if len(s.Computers) != 2 || !s.Computers[0].This || s.Computers[0].Tokens != 6600 || s.Computers[1].Name != "Mac" || s.Computers[1].Tokens != 330 {
		t.Fatalf("computers %+v", s.Computers)
	}
	if len(s.Days) != 8 || s.Days[7].Tokens != 6930 || s.Days[0].Tokens != 0 {
		t.Fatalf("days %d %+v", len(s.Days), s.Days[len(s.Days)-1])
	}
	if len(s.Top) != 3 || s.Top[0].Title != "Build" || s.Top[2].ID != remote || s.Top[2].Computer != "Mac" {
		t.Fatalf("top %+v", s.Top)
	}
	for _, task := range s.Top {
		if task.ID == a && task.Project != "Game" {
			t.Fatalf("project %+v", task)
		}
	}
	// T-197: the same tasks, newest first, with their token parts.
	b := s.Top[0].ID
	now := time.Now().UTC()
	for i, id := range []string{remote, a, b} { // remote newest, then a, then b
		at := now.Add(-time.Duration(i+1) * time.Minute).Format(time.RFC3339Nano)
		if _, err := f.st.db.Exec(`UPDATE tasks SET finished_at=? WHERE id=?`, at, id); err != nil {
			t.Fatal(err)
		}
	}
	s, err = f.st.UsageStats(since, time.Local, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Recent) != 3 || s.Recent[0].ID != remote || s.Recent[1].ID != a || s.Recent[2].ID != b || s.Top[0].ID != b {
		t.Fatalf("recent %+v top %+v", s.Recent, s.Top)
	}
	if r := s.Recent[1]; r.Input != 1000 || r.Cached != 800 || r.Output != 100 || r.Tokens != 1100 {
		t.Fatalf("parts %+v", r)
	}
	// T-201: one agent, then one model of it.
	if _, err := f.st.db.Exec(`UPDATE tasks SET model='gpt-6.2-sol[effort=low]' WHERE id=?`, b); err != nil {
		t.Fatal(err)
	}
	if _, err := f.st.db.Exec(`UPDATE tasks SET model='gpt-6.2-sol[effort=low]',ran_model='gpt-6.2-astra' WHERE id=?`, remote); err != nil {
		t.Fatal(err)
	}
	codex, err := f.st.UsageStatsFiltered(since, time.Local, "", UsageFilter{Agent: "codex"})
	if err != nil || codex.Total.Tokens != 5830 || len(codex.Agents) != 1 || len(codex.Top) != 2 || codex.Agent != "codex" || len(codex.Models) != 2 {
		t.Fatalf("codex %+v %v", codex, err)
	}
	sol, err := f.st.UsageStatsFiltered(since, time.Local, "", UsageFilter{Agent: "codex", Model: "gpt-6.2-sol[effort=low]"})
	if err != nil || sol.Total.Tokens != 5500 || len(sol.Top) != 1 || sol.Top[0].ID != b || sol.Top[0].Model != "gpt-6.2-sol[effort=low]" || len(sol.Models) != 2 {
		t.Fatalf("sol %+v %v", sol, err)
	}
	// A period with nothing in it.
	empty, err := f.st.UsageStats(time.Now().Add(time.Hour), time.Local, "")
	if err != nil || empty.Total.Tasks != 0 || len(empty.Agents) != 0 || len(empty.Top) != 0 || len(empty.Recent) != 0 {
		t.Fatalf("empty %+v %v", empty, err)
	}
}
