package broker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// A pinned server that answers with a redirect is not followed: the device
// key never leaves for another address, and the redirect is an error.
func TestPinnedClientsDoNotFollowRedirects(t *testing.T) {
	var hit atomic.Int32
	var leaked atomic.Value
	elsewhere := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit.Add(1)
		leaked.Store(r.Header.Get("Authorization"))
		writeJSON(w, http.StatusOK, hello{BrokerID: "brk-0000000000000000", Protocol: Protocol})
	}))
	defer elsewhere.Close()
	pinned := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, elsewhere.URL+r.URL.Path, http.StatusTemporaryRedirect)
	}))
	defer pinned.Close()
	sum := sha256.Sum256(pinned.Certificate().Raw)
	fp := hex.EncodeToString(sum[:])
	address := strings.TrimPrefix(pinned.URL, "https://")
	key := strings.Repeat("a", 64)

	var out PingResult
	if _, err := call(ctx(t), pinnedClient(fp), http.MethodGet, address, "/v1/ping", key, nil, &out); err == nil {
		t.Fatal("a redirect was not an error")
	}
	c := Connection{Address: address, BrokerID: "brk-0000000000000000", Fingerprint: fp, DeviceID: "dev-0000000000000000", DeviceKey: key}
	if _, err := Report(ctx(t), c, Presence{}); err == nil {
		t.Fatal("report through a redirect succeeded")
	}
	if err := SendMessage(ctx(t), c, HostPeer, "op", []byte(`{}`)); err == nil {
		t.Fatal("send through a redirect succeeded")
	}
	client, _ := observingClient()
	if _, err := call(ctx(t), client, http.MethodGet, address, "/v1/hello", "", nil, &hello{}); err == nil {
		t.Fatal("the pairing client followed a redirect")
	}
	if _, err := PairShort(ctx(t), address, "123456"); err == nil {
		t.Fatal("pairing through a redirect succeeded")
	}
	if n := hit.Load(); n != 0 {
		t.Fatalf("redirect followed %d times, authorization %q", n, leaked.Load())
	}
}

// fakeListener hands out prepared connections.
type fakeListener struct {
	conns chan net.Conn
}

func (l *fakeListener) Accept() (net.Conn, error) {
	c, ok := <-l.conns
	if !ok {
		return nil, net.ErrClosed
	}
	return c, nil
}
func (l *fakeListener) Close() error   { return nil }
func (l *fakeListener) Addr() net.Addr { return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)} }

type fakeConn struct {
	net.Conn
	remote net.Addr
	closed atomic.Bool
}

func (c *fakeConn) RemoteAddr() net.Addr { return c.remote }
func (c *fakeConn) Close() error         { c.closed.Store(true); return nil }

func fakeFrom(ip string) *fakeConn {
	return &fakeConn{remote: &net.TCPAddr{IP: net.ParseIP(ip), Port: 40000}}
}

// One computer of the network cannot take every connection slot.
func TestConnectionsPerSourceAreBounded(t *testing.T) {
	fake := &fakeListener{conns: make(chan net.Conn, 4*maxConnections)}
	ln := newGuardedListener(fake)
	var greedy []*fakeConn
	for i := 0; i <= maxConnectionsPerSource; i++ {
		c := fakeFrom("192.168.1.5")
		greedy = append(greedy, c)
		fake.conns <- c
	}
	// The same /64 counts as one source.
	v6 := []*fakeConn{fakeFrom("fd00::1"), fakeFrom("fd00::2")}
	other := fakeFrom("192.168.1.6")
	fake.conns <- other
	var accepted []net.Conn
	for i := 0; i < maxConnectionsPerSource+1; i++ {
		c, err := ln.Accept()
		if err != nil {
			t.Fatal(err)
		}
		accepted = append(accepted, c)
	}
	if got := accepted[maxConnectionsPerSource].RemoteAddr().String(); !strings.HasPrefix(got, "192.168.1.6") {
		t.Fatalf("connection past the source limit served: %s", got)
	}
	if !greedy[maxConnectionsPerSource].closed.Load() {
		t.Fatal("the extra connection was not closed")
	}
	// A closed connection frees its source's slot.
	accepted[0].Close()
	again := fakeFrom("192.168.1.5")
	fake.conns <- again
	if c, err := ln.Accept(); err != nil || c.(*slotConn).Conn != again {
		t.Fatalf("freed slot not reused: %v", err)
	}
	for i := 0; i < maxConnectionsPerSource; i++ {
		c := fakeFrom(fmt.Sprintf("fd00::%x", i+1))
		if i < 2 {
			c = v6[i]
		}
		fake.conns <- c
	}
	fake.conns <- fakeFrom("fd00::ffff")
	fake.conns <- fakeFrom("fd01::1")
	for i := 0; i < maxConnectionsPerSource; i++ {
		if _, err := ln.Accept(); err != nil {
			t.Fatal(err)
		}
	}
	if c, err := ln.Accept(); err != nil || !strings.HasPrefix(c.RemoteAddr().String(), "[fd01::1]") {
		t.Fatalf("one /64 took more than its share: %v %v", c, err)
	}
	// This computer itself is not limited per source.
	for i := 0; i < maxConnectionsPerSource+2; i++ {
		fake.conns <- fakeFrom("127.0.0.1")
	}
	for i := 0; i < maxConnectionsPerSource+2; i++ {
		if _, err := ln.Accept(); err != nil {
			t.Fatal(err)
		}
	}
}

func registerFrom(t *testing.T, h http.Handler, from, secret string) int {
	t.Helper()
	raw, _ := json.Marshal(registerRequest{Secret: secret, Name: "Ноутбук"})
	req := httptest.NewRequest(http.MethodPost, "/v1/register", strings.NewReader(string(raw)))
	req.RemoteAddr = from
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// Junk registrations from one computer do not lock out a real join from
// another one.
func TestRegistrationLimitIsPerSource(t *testing.T) {
	b, err := Open(filepath.Join(t.TempDir(), "broker"))
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	code, err := b.Invite("192.168.1.2:47100")
	if err != nil {
		t.Fatal(err)
	}
	inv, _ := ParseInvitation(code.Code)
	h := b.Handler()
	for i := 0; i < maxFailedRegistrations; i++ {
		if got := registerFrom(t, h, "192.168.1.66:5000", strings.Repeat("0", 64)); got != http.StatusForbidden {
			t.Fatalf("junk %d: %d", i, got)
		}
	}
	if got := registerFrom(t, h, "192.168.1.66:5001", inv.Secret); got != http.StatusTooManyRequests {
		t.Fatalf("flooding source not limited: %d", got)
	}
	if got := registerFrom(t, h, "[fd00::1:2]:5000", strings.Repeat("0", 64)); got != http.StatusForbidden {
		t.Fatalf("another source limited: %d", got)
	}
	if got := registerFrom(t, h, "192.168.1.7:5000", inv.Secret); got != http.StatusOK {
		t.Fatalf("real join from another computer: %d", got)
	}
}

// A device cannot take the host's name or another computer's name.
func TestRenameRefusesTakenNames(t *testing.T) {
	host, a, b := joined(t)
	host.SetPresence(func() (Presence, error) { return Presence{Name: "Mac Studio"}, nil })
	host.ReportPresence(ctx(t))
	names := func() map[string]string {
		out := map[string]string{}
		roster, err := host.broker.Roster(nil)
		if err != nil {
			t.Fatal(err)
		}
		for _, p := range roster {
			out[p.ID] = p.Name
		}
		return out
	}
	an, _ := a.Network()
	bn, _ := b.Network()
	for _, taken := range []string{"Mac Studio", " mac studio ", "СТУДИЯ", "студия "} {
		a.SetPresence(func() (Presence, error) { return Presence{Name: taken}, nil })
		a.ReportPresence(ctx(t))
		if got := names()[an.Self]; got != "Ноутбук" {
			t.Fatalf("renamed to taken %q: %q", taken, got)
		}
		if got := a.MemberName(); got != "Ноутбук" {
			t.Fatalf("member keeps a refused name %q: %q", taken, got)
		}
		if a.Status().Member.Error != "" {
			t.Fatalf("a refused name stopped reports: %s", a.Status().Member.Error)
		}
	}
	// Changing only the case of one's own name is allowed.
	a.SetPresence(func() (Presence, error) { return Presence{Name: "НОУТБУК"}, nil })
	a.ReportPresence(ctx(t))
	if got := names()[an.Self]; got != "НОУТБУК" {
		t.Fatalf("own name: %q", got)
	}
	// A host that never named itself is still «Сервер».
	host.SetPresence(func() (Presence, error) { return Presence{}, nil })
	host.ReportPresence(ctx(t))
	a.SetPresence(func() (Presence, error) { return Presence{Name: "сервер"}, nil })
	a.ReportPresence(ctx(t))
	if got := names()[an.Self]; got != "НОУТБУК" {
		t.Fatalf("took the default host name: %q", got)
	}
	// A revoked computer's name is free again.
	if err := host.Revoke(bn.Self); err != nil {
		t.Fatal(err)
	}
	a.SetPresence(func() (Presence, error) { return Presence{Name: "Студия"}, nil })
	a.ReportPresence(ctx(t))
	if got := names()[an.Self]; got != "Студия" || a.MemberName() != "Студия" {
		t.Fatalf("free name refused: %q %q", got, a.MemberName())
	}
}

// One sender cannot fill another computer's mailbox for everybody.
func TestMailboxQuotaPerSender(t *testing.T) {
	host, a, b := joined(t)
	an, _ := a.Network()
	bn, _ := b.Network()
	for i := 0; i < maxPendingPerSender; i++ {
		if err := a.Send(ctx(t), an, bn.Self, fmt.Sprintf("flood-%d", i), []byte(`{}`)); err != nil {
			t.Fatal(err)
		}
	}
	if err := a.Send(ctx(t), an, bn.Self, "one-more", []byte(`{}`)); !errors.Is(err, ErrMailboxFull) {
		t.Fatalf("sender over its quota: %v", err)
	}
	// Sending again what is already stored is still fine.
	if err := a.Send(ctx(t), an, bn.Self, "flood-0", []byte(`{}`)); err != nil {
		t.Fatalf("resend: %v", err)
	}
	hn, _ := host.Network()
	if err := host.Send(ctx(t), hn, bn.Self, "from-host", []byte(`{}`)); err != nil {
		t.Fatalf("another sender blocked: %v", err)
	}
	if err := a.Send(ctx(t), an, HostPeer, "to-host", []byte(`{}`)); err != nil {
		t.Fatalf("another recipient blocked: %v", err)
	}
}

// fakePacketConn plays questions from given sources and records answers.
type fakePacketConn struct {
	mu      sync.Mutex
	from    []net.Addr
	answers []net.Addr
	done    chan struct{}
}

func (c *fakePacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	c.mu.Lock()
	if len(c.from) == 0 {
		c.mu.Unlock()
		<-c.done
		return 0, nil, net.ErrClosed
	}
	from := c.from[0]
	c.from = c.from[1:]
	c.mu.Unlock()
	return copy(p, discoverQuestion), from, nil
}

func (c *fakePacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	c.mu.Lock()
	c.answers = append(c.answers, addr)
	c.mu.Unlock()
	return len(p), nil
}

func (c *fakePacketConn) Close() error {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
	return nil
}
func (c *fakePacketConn) LocalAddr() net.Addr              { return &net.UDPAddr{} }
func (c *fakePacketConn) SetDeadline(time.Time) error      { return nil }
func (c *fakePacketConn) SetReadDeadline(time.Time) error  { return nil }
func (c *fakePacketConn) SetWriteDeadline(time.Time) error { return nil }

// Discovery answers only this computer and private networks.
func TestDiscoveryAnswersOnlyLocalSources(t *testing.T) {
	conn := &fakePacketConn{done: make(chan struct{}), from: []net.Addr{
		&net.UDPAddr{IP: net.ParseIP("8.8.8.8"), Port: 1},
		&net.UDPAddr{IP: net.ParseIP("192.168.1.9"), Port: 2},
		&net.UDPAddr{IP: net.ParseIP("100.64.0.1"), Port: 3},
		&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4},
	}}
	c, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		serveDiscovery(c, conn, func() (discoverAnswer, bool) { return discoverAnswer{V: 1}, true })
		close(stopped)
	}()
	deadline := time.Now().Add(5 * time.Second)
	for {
		conn.mu.Lock()
		n, left := len(conn.answers), len(conn.from)
		conn.mu.Unlock()
		if left == 0 && n >= 2 || time.Now().After(deadline) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-stopped
	conn.mu.Lock()
	defer conn.mu.Unlock()
	if len(conn.answers) != 2 || conn.answers[0].(*net.UDPAddr).Port != 2 || conn.answers[1].(*net.UDPAddr).Port != 4 {
		t.Fatalf("answered %v", conn.answers)
	}
}

// Answers are kept per broker and address, so a first answer cannot hide
// the real host, and one address cannot fill the list.
func TestDiscoveryCandidates(t *testing.T) {
	var c candidates
	real := discoverAnswer{V: 1, Broker: "brk-1111111111111111", Port: 47100}
	add := func(ip string, a discoverAnswer) { c.add(&net.UDPAddr{IP: net.ParseIP(ip), Port: 9}, a) }
	add("192.168.1.66", real) // a rogue claims the real broker ID first
	add("192.168.1.2", real)
	add("192.168.1.2", real)
	for i := 0; i < 5; i++ {
		add("192.168.1.66", discoverAnswer{V: 1, Broker: fmt.Sprintf("brk-%016x", i), Port: 47100})
	}
	add("192.168.1.3", discoverAnswer{V: 2, Broker: real.Broker, Port: 47100})
	add("192.168.1.4", discoverAnswer{V: 1, Broker: "nope", Port: 47100})
	add("8.8.8.8", discoverAnswer{V: 1, Broker: "brk-2222222222222222", Port: 47100})
	want := []string{"192.168.1.66:47100", "192.168.1.2:47100", "192.168.1.66:47100"}
	if fmt.Sprint(c.out) != fmt.Sprint(want) {
		t.Fatalf("candidates %v", c.out)
	}
}

// A candidate that cannot prove the code does not end the search, and the
// real host is asked only once for a wrong code.
func TestPairingSkipsHostsThatCannotProveTheCode(t *testing.T) {
	host, code := shortHost(t)
	rogue := serve(t, filepath.Join(t.TempDir(), "rogue"))
	if _, err := rogue.b.Invite(rogue.addr); err != nil {
		t.Fatal(err)
	}
	silent := serve(t, filepath.Join(t.TempDir(), "silent"))
	rogue.b.pair.mu.Lock()
	rogueCode := rogue.b.pair.code
	rogue.b.pair.mu.Unlock()
	if rogueCode == code.Short || rogueCode == wrongCode(code.Short) {
		t.Skip("the rogue drew the same code")
	}

	// A wrong code: the real host listed twice is asked once.
	if _, err := pairCandidates(ctx(t), []string{rogue.addr, silent.addr, code.Address, code.Address}, wrongCode(code.Short)); !errors.Is(err, ErrShortCode) {
		t.Fatalf("wrong code: %v", err)
	}
	host.broker.pair.mu.Lock()
	tries := host.broker.pair.tries
	host.broker.pair.mu.Unlock()
	if tries != 1 {
		t.Fatalf("the real host was asked %d times", tries)
	}

	inv, err := pairCandidates(ctx(t), []string{rogue.addr, silent.addr, code.Address}, code.Short)
	if err != nil {
		t.Fatalf("real host behind a rogue: %v", err)
	}
	if inv.BrokerID != host.broker.ID() || inv.Fingerprint != host.broker.Fingerprint() {
		t.Fatalf("paired with %s", inv.BrokerID)
	}
	if _, err := pairCandidates(ctx(t), []string{silent.addr}, code.Short); !errors.Is(err, ErrShortGone) {
		t.Fatalf("only a host without invitation: %v", err)
	}
}
