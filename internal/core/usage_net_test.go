package core

import (
	"testing"
	"time"
)

// T-076: statistics of the whole network. Each computer reports, per day
// and agent, what the tasks that ran on it spent; a task another computer
// ran is counted from that computer's report, or from this computer's own
// record when that computer did not report (an older version).
func TestUsageStatsAcrossTheNetwork(t *testing.T) {
	f := newTaskFixture(t)
	finish := func(op, agent, title string, in, out int64) string {
		id := f.start(t, op, agent, title)
		session := "019a0000-0000-7000-8000-0000000000" + op[len(op)-2:]
		if agent == "claude" {
			session = f.sessionOf(t, id)
		}
		r := f.launch(t, id, session)
		f.dispose(t, r, DispositionReport, "Done")
		ev := cleanExit
		ev.Usage = &TaskUsage{InputTokens: in, OutputTokens: out}
		if _, err := f.st.FinishTaskRun(r.ID, ev); err != nil {
			t.Fatal(err)
		}
		return id
	}
	finish("op-01", "claude", "Here", 90, 10)
	onA := finish("op-02", "codex", "On A", 45, 5)
	onB := finish("op-03", "codex", "On B", 27, 3)
	for id, peer := range map[string]string{onA: "peer-a", onB: "peer-b"} {
		if _, err := f.st.db.Exec(`UPDATE tasks SET remote_peer=?,remote_broker='b',remote_task='x' WHERE id=?`, peer, id); err != nil {
			t.Fatal(err)
		}
	}
	// What this computer reports: only the task that ran here.
	since := time.Now().Add(-7 * 24 * time.Hour)
	own, err := f.st.UsageDaily(since, time.Local)
	if err != nil || len(own) != 1 || own[0].Agent != "claude" || own[0].Tasks != 1 || own[0].Input != 90 || own[0].Output != 10 {
		t.Fatalf("own %+v %v", own, err)
	}
	today := time.Now().In(time.Local).Format("2006-01-02")
	old := time.Now().AddDate(0, 0, -20).In(time.Local).Format("2006-01-02")
	// A reports both tasks it ran (one of them ours) and an old day; B is
	// an older version and reports nothing.
	peers := []NetPeer{
		{ID: "peer-a", Name: "PC-1", Age: 0, Usage: []UsageDayAgent{
			{Day: today, Agent: "codex", Tasks: 2, Input: 200, Cached: 50, Output: 20},
			{Day: old, Agent: "codex", Tasks: 9, Input: 9000, Output: 900},
		}},
		{ID: "peer-b", Name: "Laptop", Age: 0},
	}
	if err := f.st.SetNetwork("b", "self", peers); err != nil {
		t.Fatal(err)
	}
	all, err := f.st.UsageStats(since, time.Local, "")
	if err != nil {
		t.Fatal(err)
	}
	if all.Total.Tokens != 100+220+30 || all.Total.Tasks != 4 || all.Total.CachedTokens != 50 {
		t.Fatalf("total %+v", all.Total)
	}
	got := map[string]int64{}
	for _, c := range all.Computers {
		got[c.Key] = c.Tokens
	}
	if len(all.Computers) != 3 || got["peer-a"] != 220 || got["peer-b"] != 30 || got[all.Computers[len(all.Computers)-1].Key] == 0 {
		t.Fatalf("computers %+v", all.Computers)
	}
	if all.Days[len(all.Days)-1].Tokens != 350 {
		t.Fatalf("today %+v", all.Days[len(all.Days)-1])
	}
	// The largest tasks are the ones this computer knows, wherever they ran.
	if len(all.Top) != 3 {
		t.Fatalf("top %+v", all.Top)
	}
	// One computer only.
	a, err := f.st.UsageStats(since, time.Local, "peer-a")
	if err != nil || a.Total.Tokens != 220 || a.Total.Tasks != 2 || len(a.Computers) != 1 || len(a.Top) != 1 || a.Top[0].ID != onA || a.Computer != "PC-1" {
		t.Fatalf("peer-a %+v %v", a, err)
	}
	if len(a.Agents) != 1 || a.Agents[0].Key != "codex" || a.Agents[0].Tokens != 220 {
		t.Fatalf("peer-a agents %+v", a.Agents)
	}
	local, err := f.st.UsageStats(since, time.Local, "local")
	if err != nil || local.Total.Tokens != 100 || len(local.Top) != 1 || local.Top[0].Title != "Here" {
		t.Fatalf("local %+v %v", local, err)
	}
	// The old day of A is outside a 30-day window? No: inside 30 days, outside 7.
	month, _ := f.st.UsageStats(time.Now().AddDate(0, 0, -29), time.Local, "peer-a")
	if month.Total.Tokens != 220+9900 {
		t.Fatalf("month %+v", month.Total)
	}
}
