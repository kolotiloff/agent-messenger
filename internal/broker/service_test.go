package broker

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func newService(t *testing.T, root string) *Service {
	t.Helper()
	s := NewService(context.Background(), root)
	t.Cleanup(s.Close)
	return s
}

func TestHostedBrokerAndMembership(t *testing.T) {
	hostRoot, memberRoot := t.TempDir(), t.TempDir()
	host := newService(t, hostRoot)
	port := freePort(t)
	if err := host.Enable(ScopeLocal, port); err != nil {
		t.Fatal(err)
	}
	st := host.Status()
	if !st.Host.Running || len(st.Host.Addresses) != 1 || st.Host.BrokerID == "" {
		t.Fatalf("host %+v", st)
	}
	if _, err := host.Invite("10.0.0.1:1"); err == nil {
		t.Fatal("invitation for a foreign address")
	}
	code, err := host.Invite(st.Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	if open := host.Status().Host.OpenInvitations; len(open) != 1 || open[0] != code.ID {
		t.Fatalf("open invitations %v, code %s", open, code.ID)
	}
	member := newService(t, memberRoot)
	if err := member.Connect(ctx(t), code.Code, "Ноутбук"); err != nil {
		t.Fatal(err)
	}
	if open := host.Status().Host.OpenInvitations; len(open) != 0 {
		t.Fatalf("used invitation still open %v", open)
	}
	ms := member.Status().Member
	if !ms.Connected || !ms.Reachable || ms.BrokerID != st.Host.BrokerID || ms.DeviceName != "Ноутбук" {
		t.Fatalf("member %+v", ms)
	}
	// The connection file is private and the status never shows the key.
	info, err := os.Stat(filepath.Join(memberRoot, "server-connection.json"))
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("connection file %v %v", info, err)
	}
	if raw, _ := os.ReadFile(filepath.Join(memberRoot, "server-connection.json")); !strings.Contains(string(raw), "device_key") {
		t.Fatal("key not stored")
	}
	if err := member.Connect(ctx(t), code.Code, "Снова"); err == nil || !strings.Contains(err.Error(), "уже подключён") {
		t.Fatalf("second broker joined silently: %v", err)
	}
	if err := member.CheckConnection(ctx(t)); err != nil {
		t.Fatal(err)
	}
	// The connection survives a restart of the client.
	again := newService(t, memberRoot)
	if !again.Status().Member.Connected {
		t.Fatal("connection lost on restart")
	}
	devices := host.Status().Host.Devices
	if len(devices) != 1 {
		t.Fatalf("devices %+v", devices)
	}
	if err := host.Revoke(devices[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := member.CheckConnection(ctx(t)); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked device still accepted: %v", err)
	}
	if ms := member.Status().Member; ms.Reachable || ms.Error == "" {
		t.Fatalf("revocation not shown %+v", ms)
	}
	if err := member.Disconnect(); err != nil || member.Status().Member.Connected {
		t.Fatalf("disconnect %v", err)
	}
	if _, err := os.Stat(filepath.Join(memberRoot, "server-connection.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("connection file kept")
	}
}

func TestHostSettingsPersistAndBusyPortIsRefused(t *testing.T) {
	root := t.TempDir()
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer busy.Close()
	s := newService(t, root)
	if err := s.Enable(ScopeLocal, busy.Addr().(*net.TCPAddr).Port); err == nil || !strings.Contains(err.Error(), "занят") {
		t.Fatalf("busy port: %v", err)
	}
	if st := s.Status().Host; st.Enabled || st.Running {
		t.Fatalf("failed start saved as enabled %+v", st)
	}
	port := freePort(t)
	if err := s.Enable(ScopeLocal, port); err != nil {
		t.Fatal(err)
	}
	id := s.Status().Host.BrokerID
	if err := s.Enable(ScopeLocal, 80); err == nil {
		t.Fatal("privileged port accepted")
	}
	if !s.Status().Host.Running {
		t.Fatal("a refused change stopped the running server")
	}
	s.Close()
	// The next client start brings the server back with the same identity.
	s2 := newService(t, root)
	s2.Start()
	st := s2.Status().Host
	if !st.Running || st.Port != port || st.BrokerID != id {
		t.Fatalf("restart %+v", st)
	}
	if err := s2.Disable(); err != nil {
		t.Fatal(err)
	}
	s3 := newService(t, root)
	s3.Start()
	if st := s3.Status().Host; st.Running || st.Enabled {
		t.Fatalf("disabled server started %+v", st)
	}
}

func TestLANScopeListsOnlyRealAddresses(t *testing.T) {
	s := newService(t, t.TempDir())
	_, private, _ := net.ParseCIDR("192.168.1.20/24")
	private.IP = net.ParseIP("192.168.1.20").To4()
	_, linkLocal, _ := net.ParseCIDR("169.254.3.4/16")
	linkLocal.IP = net.ParseIP("169.254.3.4").To4()
	s.interfaces = func() ([]net.Addr, error) {
		return []net.Addr{&net.IPNet{IP: net.ParseIP("127.0.0.1"), Mask: net.CIDRMask(8, 32)}, private, linkLocal, &net.IPNet{IP: net.ParseIP("fe80::1"), Mask: net.CIDRMask(64, 128)}}, nil
	}
	s.settings = Settings{Scope: ScopeLAN, Port: 47100}
	got := s.addressesLocked()
	if len(got) != 1 || got[0] != "192.168.1.20:47100" {
		t.Fatalf("addresses %v", got)
	}
}

func TestDamagedFilesDoNotStopTheClient(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "broker"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broker", "settings.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "server-connection.json"), []byte(`{"address":"x"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newService(t, root)
	s.Start()
	st := s.Status()
	if st.Host.Running || st.Host.Error == "" || st.Member.Connected || st.Member.Error == "" {
		t.Fatalf("damaged files %+v", st)
	}
	// The damaged connection is kept for the owner to look at.
	if _, err := os.Stat(filepath.Join(root, "server-connection.json")); err != nil {
		t.Fatal(err)
	}
}

func TestDamagedConnectionBlocksConnectUntilDisconnect(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "server-connection.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newService(t, root)
	if err := s.Connect(ctx(t), Invitation{Address: "127.0.0.1:1", BrokerID: "brk-0000000000000000", Fingerprint: strings.Repeat("a", 64), Secret: strings.Repeat("b", 64)}.String(), "x"); err == nil || !strings.Contains(err.Error(), "повреждён") {
		t.Fatalf("damaged file overwritten: %v", err)
	}
	if raw, _ := os.ReadFile(filepath.Join(root, "server-connection.json")); string(raw) != "{" {
		t.Fatal("damaged file changed")
	}
	if err := s.Disconnect(); err != nil || s.Status().Member.Error != "" {
		t.Fatalf("disconnect %v %+v", err, s.Status().Member)
	}
}

func TestStopReleasesTheLockBeforeRestart(t *testing.T) {
	s := newService(t, t.TempDir())
	port := freePort(t)
	for i := 0; i < 5; i++ {
		if err := s.Enable(ScopeLocal, port); err != nil {
			t.Fatalf("enable %d: %v", i, err)
		}
		if err := s.Disable(); err != nil {
			t.Fatal(err)
		}
	}
	// A broker whose base context ends releases its data before it is
	// marked stopped, so Close does not race it.
	base, cancel := context.WithCancel(context.Background())
	s2 := NewService(base, t.TempDir())
	if err := s2.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	cancel()
	s2.Close()
	if s2.Status().Host.Running {
		t.Fatal("still running after close")
	}
}
