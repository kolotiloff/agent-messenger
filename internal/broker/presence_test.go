package broker

import (
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestPresenceRoster(t *testing.T) {
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
	host.SetPresence(func() (Presence, error) {
		return Presence{Name: "Mac Studio", Agents: []PresenceAgent{{Agent: "claude", Running: 1}, {Agent: "codex"}}, Coordinator: "claude-code"}, nil
	})
	member.SetPresence(func() (Presence, error) {
		return Presence{Agents: []PresenceAgent{{Agent: "codex", Waiting: 2, Models: []string{"gpt-5.5", "gpt-5.5-mini"}}}}, nil
	})
	host.ReportPresence(ctx(t))
	member.ReportPresence(ctx(t))

	peers := member.Status().Peers
	if len(peers) != 1 || !peers[0].Host || peers[0].Name != "Mac Studio" || peers[0].Coordinator != "claude-code" || peers[0].Age < 0 || peers[0].Age > 5 {
		t.Fatalf("member sees %+v", peers)
	}
	if len(peers[0].Agents) != 2 || !reflect.DeepEqual(peers[0].Agents[0], PresenceAgent{Agent: "claude", Running: 1}) {
		t.Fatalf("host agents %+v", peers[0].Agents)
	}
	if ms := member.Status().Member; !ms.Reachable || ms.CheckedAt == "" {
		t.Fatalf("a report is a successful check: %+v", ms)
	}
	peers = host.Status().Peers
	if len(peers) != 1 || peers[0].Host || peers[0].Name != "Ноутбук" || len(peers[0].Agents) != 1 || peers[0].Agents[0].Waiting != 2 || !reflect.DeepEqual(peers[0].Agents[0].Models, []string{"gpt-5.5", "gpt-5.5-mini"}) {
		t.Fatalf("host sees %+v", peers)
	}

	// A device renames itself through its report, and every computer sees
	// the new name (D-061).
	member.SetPresence(func() (Presence, error) {
		return Presence{Name: "Ноут-Linux", Agents: []PresenceAgent{{Agent: "codex", Waiting: 2}}}, nil
	})
	member.ReportPresence(ctx(t))
	if peers = host.Status().Peers; len(peers) != 1 || peers[0].Name != "Ноут-Linux" {
		t.Fatalf("renamed device: %+v", peers)
	}
	if devices := host.Status().Host.Devices; len(devices) != 1 || devices[0].Name != "Ноут-Linux" {
		t.Fatalf("host's devices: %+v", devices)
	}
	if name := member.Status().Member.DeviceName; name != "Ноут-Linux" || member.MemberName() != name {
		t.Fatalf("member's own name: %q", name)
	}
	if reopened := newService(t, member.root); reopened.MemberName() != "Ноут-Linux" {
		t.Fatalf("the connection file keeps the name: %q", reopened.MemberName())
	}

	// A malformed report is refused and changes nothing.
	c := *member.member
	for _, body := range []any{
		map[string]any{"agents": []map[string]any{{"agent": "Not A Name"}}},
		map[string]any{"agents": []map[string]any{{"agent": "codex", "models": []string{"--yolo"}}}},
		map[string]any{"agents": []map[string]any{{"agent": "codex", "running": -1}}},
		map[string]any{"agents": []map[string]any{{"agent": "codex"}, {"agent": "codex"}}},
		map[string]any{"agents": []any{}, "extra": true},
	} {
		code, err := call(ctx(t), pinnedClient(c.Fingerprint), http.MethodPost, c.Address, "/v1/presence", c.DeviceKey, body, nil)
		if err != nil || code != http.StatusBadRequest {
			t.Fatalf("bad report %v: %d %v", body, code, err)
		}
	}
	if code, _ := call(ctx(t), pinnedClient(c.Fingerprint), http.MethodPost, c.Address, "/v1/presence", strings.Repeat("0", 64), Presence{}, nil); code != http.StatusUnauthorized {
		t.Fatalf("unknown key: %d", code)
	}
	if got := host.Status().Peers[0].Agents[0].Waiting; got != 2 {
		t.Fatalf("refused report changed the roster: %d", got)
	}

	// A revoked device is told so and leaves the roster.
	if err := host.Revoke(c.DeviceID); err != nil {
		t.Fatal(err)
	}
	if len(host.Status().Peers) != 0 {
		t.Fatal("revoked device still listed")
	}
	member.ReportPresence(ctx(t))
	if ms := member.Status().Member; ms.Reachable || ms.Error != ErrRevoked.Error() {
		t.Fatalf("revoked member %+v", ms)
	}
	if err := member.Disconnect(ctx(t)); err != nil {
		t.Fatal(err)
	}
	if len(member.Status().Peers) != 0 {
		t.Fatal("peers kept after disconnect")
	}
}

func TestPresenceOfUnreachableBrokerAges(t *testing.T) {
	host := newService(t, t.TempDir())
	if err := host.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	code, _ := host.Invite(host.Status().Host.Addresses[0])
	member := newService(t, t.TempDir())
	if err := member.Connect(ctx(t), code.Code, "Ноутбук"); err != nil {
		t.Fatal(err)
	}
	host.SetPresence(func() (Presence, error) { return Presence{Name: "Mac", Agents: []PresenceAgent{}}, nil })
	member.SetPresence(func() (Presence, error) { return Presence{Agents: []PresenceAgent{}}, nil })
	host.ReportPresence(ctx(t))
	member.ReportPresence(ctx(t))
	if len(member.Status().Peers) != 1 {
		t.Fatal("no roster")
	}
	if err := host.Disable(); err != nil {
		t.Fatal(err)
	}
	member.ReportPresence(ctx(t))
	st := member.Status()
	if st.Member.Reachable || st.Member.Error == "" {
		t.Fatalf("unreachable broker not shown: %+v", st.Member)
	}
	// The last list is kept and keeps ageing; it is not refreshed.
	member.mu.Lock()
	member.peersAt = member.peersAt.Add(-PresenceInterval * 10)
	member.mu.Unlock()
	if p := member.Status().Peers; len(p) != 1 || p[0].Age < 150 {
		t.Fatalf("stale roster %+v", p)
	}
}

func TestPresenceReadiness(t *testing.T) {
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
	ready := &PresenceReady{Accept: true, Projects: []string{"Game"}}
	member.SetPresence(func() (Presence, error) {
		return Presence{Agents: []PresenceAgent{{Agent: "codex"}}, Ready: ready}, nil
	})
	member.ReportPresence(ctx(t))
	if peers := host.Status().Peers; len(peers) != 1 || !reflect.DeepEqual(peers[0].Ready, ready) {
		t.Fatalf("host sees %+v", peers)
	}

	// An older host refuses the field; the member still reports its agents.
	olderHost = func(p Presence) bool { return p.Ready != nil }
	t.Cleanup(func() { olderHost = nil })
	member.SetPresence(func() (Presence, error) {
		return Presence{Agents: []PresenceAgent{{Agent: "codex", Running: 1}}, Ready: ready}, nil
	})
	member.ReportPresence(ctx(t))
	peers := host.Status().Peers
	if len(peers) != 1 || peers[0].Ready != nil || len(peers[0].Agents) != 1 || peers[0].Agents[0].Running != 1 {
		t.Fatalf("older host sees %+v", peers)
	}
	if ms := member.Status().Member; !ms.Reachable || ms.Error != "" {
		t.Fatalf("member after fallback %+v", ms)
	}

	bad := Presence{Agents: []PresenceAgent{}, Ready: &PresenceReady{Projects: []string{"a\nb"}}}
	if bad.valid() {
		t.Fatal("a project name with a line break passed")
	}
}

// T-078: a computer reports its program version; an older host refuses
// the field and the member reports without it, keeping its other fields.
func TestPresenceVersion(t *testing.T) {
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
	ready := &PresenceReady{Accept: true, Projects: []string{"Game"}}
	member.SetPresence(func() (Presence, error) {
		return Presence{Agents: []PresenceAgent{{Agent: "codex"}}, Ready: ready, Version: "0.0.0-preview.57"}, nil
	})
	host.SetPresence(func() (Presence, error) {
		return Presence{Agents: []PresenceAgent{}, Version: "0.0.0-preview.56"}, nil
	})
	host.ReportPresence(ctx(t))
	member.ReportPresence(ctx(t))
	if peers := host.Status().Peers; len(peers) != 1 || peers[0].Version != "0.0.0-preview.57" {
		t.Fatalf("host sees %+v", peers)
	}
	if peers := member.Status().Peers; len(peers) != 1 || peers[0].Version != "0.0.0-preview.56" {
		t.Fatalf("member sees %+v", peers)
	}

	olderHost = func(p Presence) bool { return p.Version != "" }
	t.Cleanup(func() { olderHost = nil })
	member.SetPresence(func() (Presence, error) {
		return Presence{Agents: []PresenceAgent{{Agent: "codex", Running: 1}}, Ready: ready, Version: "0.0.0-preview.58"}, nil
	})
	member.ReportPresence(ctx(t))
	peers := host.Status().Peers
	if len(peers) != 1 || peers[0].Version != "" || !reflect.DeepEqual(peers[0].Ready, ready) || peers[0].Agents[0].Running != 1 {
		t.Fatalf("older host sees %+v", peers)
	}
	if ms := member.Status().Member; !ms.Reachable || ms.Error != "" {
		t.Fatalf("member after fallback %+v", ms)
	}

	for _, bad := range []string{"0.1 beta", strings.Repeat("9", 65), "-1", "v1\n"} {
		if (Presence{Agents: []PresenceAgent{}, Version: bad}).valid() {
			t.Errorf("version %q passed", bad)
		}
	}
}
