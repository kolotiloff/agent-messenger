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
	s, err := f.st.UsageStats(since, time.Local)
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
	// A period with nothing in it.
	empty, err := f.st.UsageStats(time.Now().Add(time.Hour), time.Local)
	if err != nil || empty.Total.Tasks != 0 || len(empty.Agents) != 0 || len(empty.Top) != 0 {
		t.Fatalf("empty %+v %v", empty, err)
	}
}
