package core

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestUsageRanksBeforeLimitingTheList(t *testing.T) {
	f := newTaskFixture(t)
	end := time.Now().UTC()
	ids := make([]string, MaxUsageTop+3)
	for i := range ids {
		ids[i] = f.start(t, fmt.Sprintf("rank-%d", i), "codex", "Ranked task")
		finished := end.Add(-time.Duration(i) * time.Minute)
		started := finished.Add(-time.Duration(i+1) * time.Second)
		if _, err := f.st.db.Exec(`UPDATE tasks SET state='done',finished_at=?,usage_seen=?,usage_input=? WHERE id=?`,
			finished.Format(time.RFC3339Nano), i != len(ids)-1, 100-i, ids[i]); err != nil {
			t.Fatal(err)
		}
		if _, err := f.st.db.Exec(`UPDATE task_runs SET launched_at=? WHERE task_id=?`, started.Format(time.RFC3339Nano), ids[i]); err != nil {
			t.Fatal(err)
		}
	}
	s, err := f.st.UsageStats(end.Add(-time.Hour), time.Local, "local")
	if err != nil {
		t.Fatal(err)
	}
	last, lastWithUsage := len(ids)-1, len(ids)-2
	for _, tc := range []struct {
		name  string
		rows  []UsageTask
		first int
	}{
		{"recent", s.Recent, 0},
		{"oldest", s.Ranked["oldest"], last},
		{"tokens", s.Top, 0},
		{"fewest", s.Ranked["fewest"], lastWithUsage},
		{"longest", s.Ranked["longest"], last},
		{"shortest", s.Ranked["shortest"], 0},
	} {
		if len(tc.rows) != MaxUsageTop || tc.rows[0].ID != ids[tc.first] {
			t.Fatalf("%s: %+v", tc.name, tc.rows)
		}
		row := tc.rows[0]
		if row.DurationMS == nil || *row.DurationMS != int64(tc.first+1)*1000 || row.UsageMissing != (tc.first == last) {
			t.Fatalf("%s: incorrect duration or usage flag: %+v", tc.name, row)
		}
	}
	if s.Total.Tasks != len(ids)-1 {
		t.Fatalf("duration-only task changed token totals: %+v", s.Total)
	}
}

func TestUsageRecoversDurationForAnOlderRemoteResult(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "long-remote", "codex", "Long task")
	p.carry(t, true)
	p.carry(t, false)
	apply := func(op string, b netBody) {
		t.Helper()
		b.Task = id
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		p.seq++
		if err := p.a.st.ApplyNet(testBroker, NetIncoming{Seq: p.seq, From: "dev-b", OpID: op, Body: raw}); err != nil {
			t.Fatal(err)
		}
	}
	apply("long-progress", netBody{Kind: "progress", Event: "text", Text: "Working"})
	start := time.Now().UTC().Add(-3 * time.Hour).Format(time.RFC3339Nano)
	if _, err := p.a.st.db.Exec(`UPDATE remote_task_items SET created_at=? WHERE task_id=?`, start, id); err != nil {
		t.Fatal(err)
	}
	result := netBody{Kind: "event", Event: "failed", Audience: AudienceCoordinator, State: "failed"}
	apply("long-result", result)
	verify := func(want time.Duration, estimated bool) {
		t.Helper()
		s, err := p.a.st.UsageStats(time.Now().Add(-time.Hour), time.Local, "dev-b")
		if err != nil {
			t.Fatal(err)
		}
		for _, rows := range [][]UsageTask{s.Recent, s.Ranked["longest"]} {
			if len(rows) != 1 || rows[0].ID != id || rows[0].DurationMS == nil || rows[0].DurationEstimated != estimated || !rows[0].UsageMissing {
				t.Fatalf("expected timed task without token totals: %+v", rows)
			}
			if got := *rows[0].DurationMS; got < want.Milliseconds() || got > (want+time.Second).Milliseconds() {
				t.Fatalf("duration %d ms, want %s", got, want)
			}
		}
		if s.Total.Tasks != 0 || len(s.Top) != 0 {
			t.Fatalf("duration changed token statistics: %+v", s.Total)
		}
	}
	verify(3*time.Hour, true)
	exact := (3*time.Hour + time.Minute).Milliseconds()
	result.DurationMS = &exact
	apply("long-result-exact", result)
	verify(3*time.Hour+time.Minute, false)
}

// Token statistics (T-068): sums by agent, by computer, by day and the
// largest tasks. Token totals omit tasks without reported usage.
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
	withoutUsage := finish("op-03", "codex", "Nothing reported", 0, 0, 0)
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
	if len(s.Recent) != 4 || s.Recent[0].ID != withoutUsage || s.Recent[1].ID != remote || s.Recent[2].ID != a || s.Recent[3].ID != b || s.Top[0].ID != b {
		t.Fatalf("recent %+v top %+v", s.Recent, s.Top)
	}
	if r := s.Recent[2]; r.Input != 1000 || r.Cached != 800 || r.Output != 100 || r.Tokens != 1100 {
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
