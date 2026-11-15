package broker

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// T-182: the owner's external address is an IP or a domain, with the
// server's port when none is given.
func TestPublicAddress(t *testing.T) {
	good := map[string]string{
		"203.0.113.7":             "203.0.113.7:47100",
		" 203.0.113.7:50000 ":     "203.0.113.7:50000",
		"my-home.example.com":     "my-home.example.com:47100",
		"my-home.example.com:443": "my-home.example.com:443",
		"2001:db8::7":             "[2001:db8::7]:47100",
		"[2001:db8::7]:50000":     "[2001:db8::7]:50000",
	}
	for in, want := range good {
		if got, err := PublicAddress(in, 47100); err != nil || got != want {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for _, in := range []string{"", "localhost", "127.0.0.1", "0.0.0.0", "https://203.0.113.7", "203.0.113.7:0", "203.0.113.7:70000", "user@host.example.com", "a b.example.com", "-bad.example.com", "224.0.0.1"} {
		if got, err := PublicAddress(in, 47100); err == nil {
			t.Errorf("%q accepted as %q", in, got)
		}
	}
}

// T-182: only the internet scope lets public peers in, and then within their
// own limits, so the internet cannot take the slots of the local network.
func TestInternetScopeLimitsPublicPeers(t *testing.T) {
	// Local network scope: a public peer is dropped before TLS.
	fake := &fakeListener{conns: make(chan net.Conn, 8)}
	ln := newGuardedListener(fake)
	outsider, local := fakeFrom("203.0.113.9"), fakeFrom("192.168.1.5")
	fake.conns <- outsider
	fake.conns <- local
	if c, err := ln.Accept(); err != nil || c.(*slotConn).Conn != local || !outsider.closed.Load() {
		t.Fatalf("lan scope let a public peer in: %v", err)
	}

	fake = &fakeListener{conns: make(chan net.Conn, 4*maxConnections)}
	ln = newGuardedListener(fake)
	ln.public = true
	// Special addresses stay out even here.
	for _, ip := range []string{"0.0.0.0", "224.0.0.1", "169.254.1.1"} {
		fake.conns <- fakeFrom(ip)
	}
	// One public source gets maxConnectionsPerPublicSource.
	var one []*fakeConn
	for i := 0; i <= maxConnectionsPerPublicSource; i++ {
		c := fakeFrom("203.0.113.9")
		one = append(one, c)
		fake.conns <- c
	}
	next := fakeFrom("198.51.100.1")
	fake.conns <- next
	var held []net.Conn
	for i := 0; i < maxConnectionsPerPublicSource; i++ {
		c, err := ln.Accept()
		if err != nil || c.(*slotConn).Conn != one[i] {
			t.Fatalf("public source %d: %v", i, err)
		}
		held = append(held, c)
	}
	if c, err := ln.Accept(); err != nil || c.(*slotConn).Conn != next || !one[maxConnectionsPerPublicSource].closed.Load() {
		t.Fatalf("past the public source limit: %v", err)
	}
	// All public sources together get maxPublicConnections.
	for i := 0; len(held)+1 < maxPublicConnections; i++ {
		c := fakeFrom(fmt.Sprintf("198.51.100.%d", i+2))
		fake.conns <- c
		got, err := ln.Accept()
		if err != nil || got.(*slotConn).Conn != c {
			t.Fatalf("public %d: %v", i, err)
		}
		held = append(held, got)
	}
	extra, private := fakeFrom("198.51.100.200"), fakeFrom("10.0.0.5")
	fake.conns <- extra
	fake.conns <- private
	if c, err := ln.Accept(); err != nil || c.(*slotConn).Conn != private || !extra.closed.Load() {
		t.Fatalf("past the public total, or the local network kept out: %v", err)
	}
	// A closed public connection frees a public slot.
	held[0].Close()
	again := fakeFrom("198.51.100.201")
	fake.conns <- again
	if c, err := ln.Accept(); err != nil || c.(*slotConn).Conn != again {
		t.Fatalf("freed public slot not reused: %v", err)
	}
}

// T-182: the six-digit code works only from this computer and private
// networks; from the internet only the long invitation does.
func TestPairingIsLocalOnly(t *testing.T) {
	b, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { b.Close() })
	h := b.Handler()
	for _, path := range []string{"/v1/pair/start", "/v1/pair/finish"} {
		for from, forbidden := range map[string]bool{"203.0.113.9:40000": true, "[2001:db8::1]:40000": true, "192.168.1.5:40000": false, "127.0.0.1:40000": false} {
			req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{}`))
			req.RemoteAddr = from
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if (rec.Code == http.StatusForbidden) != forbidden {
				t.Errorf("%s from %s: %d", path, from, rec.Code)
			}
		}
	}
}

// T-182: the internet scope saves the external address, offers it first for
// invitations, and still serves this computer and the local network.
func TestInternetScopeService(t *testing.T) {
	root := t.TempDir()
	host := newService(t, root)
	port := freePort(t)
	if err := host.EnableWith(Settings{Scope: ScopeInternet, Port: port}); err == nil {
		t.Fatal("internet scope without an external address")
	}
	if host.Status().Host.Running {
		t.Fatal("started without an external address")
	}
	if err := host.EnableWith(Settings{Scope: ScopeInternet, Port: port, PublicAddress: "my-home.example.com"}); err != nil {
		t.Fatal(err)
	}
	st := host.Status().Host
	want := "my-home.example.com:" + strconv.Itoa(port)
	if !st.Running || st.Scope != ScopeInternet || st.PublicAddress != want || len(st.Addresses) == 0 || st.Addresses[0] != want {
		t.Fatalf("host %+v", st)
	}
	code, err := host.Invite(want)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := ParseInvitation(code.Code)
	if err != nil || inv.Address != want {
		t.Fatalf("invitation %+v %v", inv, err)
	}
	raw, err := os.ReadFile(filepath.Join(root, "broker", "settings.json"))
	var saved Settings
	if err != nil || json.Unmarshal(raw, &saved) != nil || saved.PublicAddress != want || saved.Scope != ScopeInternet {
		t.Fatalf("saved %s %v", raw, err)
	}
	// It listens on every interface: this computer reaches it too.
	if err := identify(ctx(t), pinnedClient(inv.Fingerprint), net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), inv.BrokerID); err != nil {
		t.Fatalf("not reachable on this computer: %v", err)
	}
	// Going back to the local network forgets the external address.
	if err := host.Disable(); err != nil {
		t.Fatal(err)
	}
	if err := host.Enable(ScopeLocal, port); err != nil {
		t.Fatal(err)
	}
	if st := host.Status().Host; st.PublicAddress != "" {
		t.Fatalf("external address kept %+v", st)
	}
}
