package core

import (
	"testing"
	"time"
)

// T-278, D-192: after the network was made again, a peer id of the old one
// (here "dev-b") may name another computer. A finished task sent there in the
// old network must not show as this computer's task, in the history filter
// of the new one, in the shared list or in the usage page.
func TestOldNetworkTaskBelongsToNoCurrentComputer(t *testing.T) {
	p := newNetPair(t)
	id := p.startRemote(t, "old-net", "codex", "Build")
	if _, err := p.a.st.db.Exec(`UPDATE tasks SET state='cancelled',finished_at=?,updated_at=?,usage_seen=1,usage_input=10 WHERE id=?`, now(), now(), id); err != nil {
		t.Fatal(err)
	}
	if err := p.a.st.SetNetwork("brk-00000000000000cc", HostPeerID, []NetPeer{{ID: "dev-b", Name: "VPS", Age: 1, Agents: []NetAgent{{Agent: "codex"}}}}); err != nil {
		t.Fatal(err)
	}
	all, err := p.a.st.WindowViewFiltered("Game", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var found *WindowTask
	for i := range all.Tasks {
		if all.Tasks[i].ID == id {
			found = &all.Tasks[i]
		}
	}
	if found == nil || found.Remote == nil || found.Remote.ID != "" || found.Remote.Name != "Ноутбук" || !found.Remote.Lost {
		t.Fatalf("old network task %+v", found)
	}
	vps, err := p.a.st.WindowViewFiltered("Game", "", "", "dev-b", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range vps.Tasks {
		if task.ID == id {
			t.Fatal("the new computer under the old peer id shows the old task")
		}
	}
	if err := p.a.st.HideWindowHistory("Game", "", "dev-b"); err != nil {
		t.Fatal(err)
	}
	if page, err := p.a.st.WindowTaskHistory("Game", "", "", ""); err != nil || len(page.Tasks) != 1 || page.Tasks[0].ID != id {
		t.Fatalf("clearing the new computer's list hid the old task: %+v %v", page.Tasks, err)
	}
	shared, err := p.a.st.SharedTasks(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, task := range shared {
		if task.ID == id {
			t.Fatalf("old network task shared as running on %q", task.RunsOn)
		}
	}
	usage, err := p.a.st.UsageStats(time.Now().Add(-time.Hour), time.UTC, "dev-b")
	if err != nil || usage.Total.Tasks != 0 {
		t.Fatalf("the new computer is charged for the old task: %+v %v", usage.Total, err)
	}
	usage, err = p.a.st.UsageStats(time.Now().Add(-time.Hour), time.UTC, "")
	if err != nil || usage.Total.Tasks != 1 {
		t.Fatalf("the old task is missing from the whole network: %+v %v", usage.Total, err)
	}
	for _, c := range usage.Computers {
		if c.Key == "dev-b" {
			t.Fatalf("usage groups the old task under the new computer: %+v", usage.Computers)
		}
	}
}
