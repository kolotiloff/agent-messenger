package broker

import (
	"reflect"
	"sync"
	"testing"
	"time"
)

// Token usage travels with presence only when it changes, and the broker
// sends a computer the others' usage only when its copy is out of date
// (T-076).
func TestPresenceUsageOnlyWhenChanged(t *testing.T) {
	host := newService(t, t.TempDir())
	if err := host.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	code, err := host.Invite(host.Status().Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	member := newService(t, t.TempDir())
	if err := member.Connect(ctx(t), code.Code, "Ноутбук"); err != nil {
		t.Fatal(err)
	}
	today := time.Now().Format("2006-01-02")
	hostDays := []PresenceUsage{{Day: today, Agent: "claude", Tasks: 1, Input: 900, Cached: 100, Output: 90}}
	var mu sync.Mutex
	memberDays := []PresenceUsage{{Day: today, Agent: "codex", Tasks: 2, Input: 200, Cached: 50, Output: 20}}
	host.SetPresence(func() (Presence, error) {
		return Presence{Agents: []PresenceAgent{}, Usage: &PresenceUsageReport{Days: hostDays}}, nil
	})
	member.SetPresence(func() (Presence, error) {
		mu.Lock()
		defer mu.Unlock()
		return Presence{Agents: []PresenceAgent{}, Usage: &PresenceUsageReport{Days: append([]PresenceUsage(nil), memberDays...)}}, nil
	})
	var got []Presence
	olderHost = func(p Presence) bool { got = append(got, p); return false }
	t.Cleanup(func() { olderHost = nil })

	host.ReportPresence(ctx(t))
	member.ReportPresence(ctx(t))
	if len(got) != 1 || got[0].Usage == nil || !reflect.DeepEqual(got[0].Usage.Days, memberDays) || got[0].UsageSeen != nil {
		t.Fatalf("first report %+v", got)
	}
	peers := host.Status().Peers
	if len(peers) != 1 || peers[0].Usage == nil || !reflect.DeepEqual(peers[0].Usage.Days, memberDays) {
		t.Fatalf("host sees %+v", peers)
	}
	mine := member.Status().Peers
	if len(mine) != 1 || mine[0].Usage == nil || !reflect.DeepEqual(mine[0].Usage.Days, hostDays) {
		t.Fatalf("member sees %+v", mine)
	}

	// Nothing changed: the report carries no usage, only the versions the
	// member holds, and the broker answers without repeating the days.
	member.ReportPresence(ctx(t))
	if len(got) != 2 || got[1].Usage != nil || got[1].UsageSeen[HostPeer] == "" {
		t.Fatalf("second report %+v", got[1])
	}
	if peers := host.Status().Peers; peers[0].Usage == nil || !reflect.DeepEqual(peers[0].Usage.Days, memberDays) {
		t.Fatalf("the broker forgot the usage: %+v", peers[0].Usage)
	}
	if mine := member.Status().Peers; mine[0].Usage == nil || !reflect.DeepEqual(mine[0].Usage.Days, hostDays) {
		t.Fatalf("the member lost its copy: %+v", mine[0].Usage)
	}
	b := host.broker
	roster, err := b.Roster(got[1].UsageSeen)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range roster {
		if p.ID == HostPeer && (p.Usage == nil || p.Usage.Days != nil || p.Usage.Hash == "") {
			t.Fatalf("an up-to-date copy is sent again: %+v", p.Usage)
		}
	}

	// A new task changes the usage: it is sent again.
	mu.Lock()
	memberDays = append(memberDays, PresenceUsage{Day: today, Agent: "claude", Tasks: 1, Input: 10, Output: 1})
	mu.Unlock()
	member.ReportPresence(ctx(t))
	if len(got) != 3 || got[2].Usage == nil || len(got[2].Usage.Days) != 2 {
		t.Fatalf("changed usage not sent: %+v", got[2])
	}
	if peers := host.Status().Peers; len(peers[0].Usage.Days) != 2 {
		t.Fatalf("host sees %+v", peers[0].Usage)
	}

	// A host before usage refuses the fields; the member still reports its
	// agents and readiness.
	olderHost = func(p Presence) bool { return p.Usage != nil || p.UsageSeen != nil }
	ready := &PresenceReady{Accept: true, Projects: []string{"Game"}}
	member.SetPresence(func() (Presence, error) {
		return Presence{Agents: []PresenceAgent{{Agent: "codex", Running: 1}}, Ready: ready, Usage: &PresenceUsageReport{Days: []PresenceUsage{}}}, nil
	})
	member.ReportPresence(ctx(t))
	peers = host.Status().Peers
	if len(peers) != 1 || len(peers[0].Agents) != 1 || !reflect.DeepEqual(peers[0].Ready, ready) {
		t.Fatalf("older host sees %+v", peers)
	}
	if ms := member.Status().Member; !ms.Reachable || ms.Error != "" {
		t.Fatalf("member after fallback %+v", ms)
	}
}

func TestPresenceUsageValidation(t *testing.T) {
	ok := Presence{Agents: []PresenceAgent{}, Usage: &PresenceUsageReport{Days: []PresenceUsage{{Day: "2026-11-09", Agent: "codex", Tasks: 1, Input: 5}}}}
	if !ok.valid() {
		t.Fatal("a valid usage was refused")
	}
	for name, u := range map[string]PresenceUsage{
		"day":    {Day: "9.11.2026", Agent: "codex", Tasks: 1},
		"agent":  {Day: "2026-11-09", Agent: "Codex!", Tasks: 1},
		"tokens": {Day: "2026-11-09", Agent: "codex", Tasks: 1, Input: -1},
		"tasks":  {Day: "2026-11-09", Agent: "codex", Tasks: -1},
	} {
		p := Presence{Agents: []PresenceAgent{}, Usage: &PresenceUsageReport{Days: []PresenceUsage{u}}}
		if p.valid() {
			t.Fatalf("bad %s passed", name)
		}
	}
	twice := Presence{Agents: []PresenceAgent{}, Usage: &PresenceUsageReport{Days: []PresenceUsage{{Day: "2026-11-09", Agent: "codex"}, {Day: "2026-11-09", Agent: "codex"}}}}
	if twice.valid() {
		t.Fatal("a repeated day and agent passed")
	}
}
