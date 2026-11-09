package broker

import (
	"strings"
	"testing"
	"time"
)

// A computer hosts its network or joins another one, never both.
func TestHostingAndJoiningExcludeEachOther(t *testing.T) {
	host := newService(t, t.TempDir())
	if err := host.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	code, err := host.Invite(host.Status().Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	other := newService(t, t.TempDir())
	if err := other.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	if err := other.Connect(ctx(t), code.Code, "Второй"); err == nil || !strings.Contains(err.Error(), "остановите") {
		t.Fatalf("a hosting computer joined another server: %v", err)
	}
	if err := other.Disable(); err != nil {
		t.Fatal(err)
	}
	if err := other.Connect(ctx(t), code.Code, "Второй"); err != nil {
		t.Fatal(err)
	}
	if err := other.Enable(ScopeLocal, freePort(t)); err == nil || !strings.Contains(err.Error(), "отключите") {
		t.Fatalf("a member started its own server: %v", err)
	}
	if other.Status().Host.Running || other.Summary() != "connected" {
		t.Fatalf("summary %s", other.Summary())
	}
}

// A computer that disconnects leaves the server, and one that joins again
// does not show its earlier device as another computer (T-127).
func TestRejoinedComputerIsListedOnce(t *testing.T) {
	host := newService(t, t.TempDir())
	if err := host.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	invite := func() string {
		code, err := host.Invite(host.Status().Host.Addresses[0])
		if err != nil {
			t.Fatal(err)
		}
		return code.Code
	}
	mac := newService(t, t.TempDir())
	if err := mac.Connect(ctx(t), invite(), "Мак"); err != nil {
		t.Fatal(err)
	}
	if err := mac.Disconnect(ctx(t)); err != nil {
		t.Fatal(err)
	}
	if devices := host.Status().Host.Devices; len(devices) != 1 || devices[0].RevokedAt == "" {
		t.Fatalf("the device did not leave: %+v", devices)
	}
	if peers := host.Status().Peers; len(peers) != 0 {
		t.Fatalf("a left device is listed: %+v", peers)
	}

	// An earlier device the server still keeps (it was unreachable, or
	// older): the new one replaces it once it reports.
	clock := func(d time.Duration) {
		host.mu.Lock()
		host.broker.now = func() time.Time { return time.Now().Add(d) }
		host.mu.Unlock()
	}
	quiet := func() (Presence, error) { return Presence{Agents: []PresenceAgent{}}, nil }
	if err := mac.Connect(ctx(t), invite(), "Мак"); err != nil {
		t.Fatal(err)
	}
	mac.SetPresence(quiet)
	mac.ReportPresence(ctx(t))
	clock(time.Minute)
	mac2 := newService(t, t.TempDir())
	if err := mac2.Connect(ctx(t), invite(), "Мак"); err != nil {
		t.Fatal(err)
	}
	pc := newService(t, t.TempDir())
	if err := pc.Connect(ctx(t), invite(), "ПК-1"); err != nil {
		t.Fatal(err)
	}
	if peers := host.Status().Peers; len(peers) != 3 {
		t.Fatalf("before the new device reports: %+v", peers)
	}
	mac2.SetPresence(quiet)
	pc.SetPresence(quiet)
	mac2.ReportPresence(ctx(t))
	pc.ReportPresence(ctx(t))
	peers := host.Status().Peers
	names := map[string]string{}
	for _, p := range peers {
		names[p.Name] = p.ID
	}
	if len(peers) != 2 || names["Мак"] != mac2.Status().Member.DeviceID || names["ПК-1"] == "" {
		t.Fatalf("host lists %+v", peers)
	}
	// The member does not list a device under its own name.
	for _, p := range pc.Status().Peers {
		if p.Name == "ПК-1" {
			t.Fatalf("own name listed: %+v", p)
		}
	}
	if got := mac2.Status().Peers; len(got) != 1 || got[0].Name != "ПК-1" {
		t.Fatalf("mac sees %+v", got)
	}
}
