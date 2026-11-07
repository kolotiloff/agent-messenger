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
	"time"
)

type running struct {
	b    *Broker
	addr string
	stop func()
}

func serve(t *testing.T, dir string) running {
	t.Helper()
	b, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	ln, err := b.Listen("127.0.0.1:0")
	if err != nil {
		b.Close()
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- b.Serve(ctx, ln) }()
	stop := func() {
		cancel()
		<-done
		b.Close()
	}
	t.Cleanup(func() {
		select {
		case <-ctx.Done():
		default:
			stop()
		}
	})
	return running{b: b, addr: ln.Addr().String(), stop: stop}
}

func ctx(t *testing.T) context.Context {
	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return c
}

func TestIdentitySurvivesReopen(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "broker")
	b, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	id, fp := b.ID(), b.Fingerprint()
	b.Close()
	if !strings.HasPrefix(id, "brk-") || len(fp) != 64 {
		t.Fatalf("identity %q %q", id, fp)
	}
	b, err = Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if b.ID() != id || b.Fingerprint() != fp {
		t.Fatal("identity changed on reopen")
	}
}

func TestInvitationRegistersOnceAndPingWorks(t *testing.T) {
	r := serve(t, filepath.Join(t.TempDir(), "broker"))
	code, err := r.b.Invite(r.addr)
	if err != nil {
		t.Fatal(err)
	}
	inv, err := ParseInvitation(code.Code)
	if err != nil || inv.Address != r.addr || inv.BrokerID != r.b.ID() || inv.Fingerprint != r.b.Fingerprint() {
		t.Fatalf("invitation %+v %v", inv, err)
	}
	conn, err := Register(ctx(t), inv, "Mac Studio")
	if err != nil {
		t.Fatal(err)
	}
	if conn.BrokerID != r.b.ID() || conn.DeviceID == "" || conn.DeviceKey == "" || conn.DeviceName != "Mac Studio" {
		t.Fatalf("connection %+v", conn)
	}
	got, err := Check(ctx(t), conn)
	if err != nil || got.DeviceID != conn.DeviceID || got.BrokerID != r.b.ID() {
		t.Fatalf("check %+v %v", got, err)
	}
	if _, err := Register(ctx(t), inv, "Second"); !errors.Is(err, ErrInvitation) {
		t.Fatalf("invitation reused: %v", err)
	}
	devices, err := r.b.Devices()
	if err != nil || len(devices) != 1 || devices[0].Name != "Mac Studio" || devices[0].LastSeen == "" {
		t.Fatalf("devices %+v %v", devices, err)
	}
	// The device key is stored only as a hash.
	var stored int
	if err := r.b.db.QueryRow(`SELECT COUNT(*) FROM devices WHERE key_hash=?`, conn.DeviceKey).Scan(&stored); err != nil || stored != 0 {
		t.Fatalf("device key stored in clear: %d %v", stored, err)
	}
}

func TestExpiredInvitationIsRefused(t *testing.T) {
	r := serve(t, filepath.Join(t.TempDir(), "broker"))
	code, err := r.b.Invite(r.addr)
	if err != nil {
		t.Fatal(err)
	}
	r.b.now = func() time.Time { return time.Now().Add(InviteTTL + time.Minute) }
	inv, _ := ParseInvitation(code.Code)
	if _, err := Register(ctx(t), inv, "Late"); !errors.Is(err, ErrInvitation) {
		t.Fatalf("expired invitation accepted: %v", err)
	}
}

func TestRevokedDeviceIsRefused(t *testing.T) {
	r := serve(t, filepath.Join(t.TempDir(), "broker"))
	code, _ := r.b.Invite(r.addr)
	inv, _ := ParseInvitation(code.Code)
	conn, err := Register(ctx(t), inv, "Laptop")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.b.Revoke(conn.DeviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(ctx(t), conn); !errors.Is(err, ErrRevoked) {
		t.Fatalf("revoked device accepted: %v", err)
	}
	if err := r.b.Revoke("dev-unknown"); err == nil {
		t.Fatal("unknown device revoked")
	}
}

func TestPinnedCertificateAndIdentity(t *testing.T) {
	r := serve(t, filepath.Join(t.TempDir(), "broker"))
	code, _ := r.b.Invite(r.addr)
	inv, _ := ParseInvitation(code.Code)
	wrongPin := inv
	wrongPin.Fingerprint = strings.Repeat("0", 64)
	if _, err := Register(ctx(t), wrongPin, "X"); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("certificate not pinned: %v", err)
	}
	wrongID := inv
	wrongID.BrokerID = "brk-0000000000000000"
	if _, err := Register(ctx(t), wrongID, "X"); !errors.Is(err, ErrIdentity) {
		t.Fatalf("broker identity not checked: %v", err)
	}
	// Neither failed attempt used the invitation.
	if _, err := Register(ctx(t), inv, "Real"); err != nil {
		t.Fatalf("invitation burnt by a refused attempt: %v", err)
	}
	// Another broker on the same address is not the one a device joined.
	if _, err := Check(ctx(t), Connection{Address: r.addr, BrokerID: r.b.ID(), Fingerprint: strings.Repeat("1", 64), DeviceKey: "x"}); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("check without the pinned certificate: %v", err)
	}
	if _, err := Check(ctx(t), Connection{Address: r.addr, BrokerID: "brk-0000000000000000", Fingerprint: r.b.Fingerprint(), DeviceKey: "x"}); !errors.Is(err, ErrIdentity) {
		t.Fatalf("check against another broker id: %v", err)
	}
}

func TestOneServerPerDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "broker")
	serve(t, dir)
	b, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if _, err := b.Listen("127.0.0.1:0"); !errors.Is(err, ErrBusy) {
		t.Fatalf("second server on the same data: %v", err)
	}
}

func TestInvitationFormat(t *testing.T) {
	inv := Invitation{Address: "192.168.1.5:47100", BrokerID: "brk-0123456789abcdef", Fingerprint: strings.Repeat("ab", 32), Secret: strings.Repeat("cd", 32)}
	back, err := ParseInvitation(" " + inv.String() + "\n")
	if err != nil || back != inv {
		t.Fatalf("round trip %+v %v", back, err)
	}
	for _, bad := range []string{"", "am1:", "am1:!!!", "am2:" + inv.String()[4:], inv.String() + "x"} {
		if _, err := ParseInvitation(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	noPort := inv
	noPort.Address = "192.168.1.5"
	if _, err := ParseInvitation(noPort.String()); err == nil {
		t.Error("address without port accepted")
	}
	if _, _, err := net.SplitHostPort(inv.Address); err != nil {
		t.Fatal(err)
	}
}

func TestDeviceNamesAreOnePrintableLine(t *testing.T) {
	for _, name := range []string{"Mac\n", "a\tb", "x\x00y", "Mac‮gnp", "​", strings.Repeat("я", MaxDeviceName+1), ""} {
		if validDeviceName(name) {
			t.Fatalf("accepted %q", name)
		}
	}
	for _, name := range []string{"Mac Studio", "Ноутбук Алексея", strings.Repeat("я", MaxDeviceName)} {
		if !validDeviceName(name) {
			t.Fatalf("refused %q", name)
		}
	}
}

func TestWrongRegistrationsAreRateLimited(t *testing.T) {
	r := serve(t, filepath.Join(t.TempDir(), "broker"))
	code, err := r.b.Invite(r.addr)
	if err != nil {
		t.Fatal(err)
	}
	inv, _ := ParseInvitation(code.Code)
	wrong := inv
	wrong.Secret = strings.Repeat("0", 64)
	for i := 0; i < maxFailedRegistrations; i++ {
		if _, err := Register(ctx(t), wrong, "x"); !errors.Is(err, ErrInvitation) {
			t.Fatalf("attempt %d: %v", i, err)
		}
	}
	if _, err := Register(ctx(t), inv, "x"); err == nil || !strings.Contains(err.Error(), "слишком много") {
		t.Fatalf("flood not limited: %v", err)
	}
	// The real invitation was not spent by the refusal.
	var unused int
	if err := r.b.db.QueryRow(`SELECT COUNT(*) FROM invitations WHERE used_at IS NULL`).Scan(&unused); err != nil || unused != 1 {
		t.Fatalf("invitation %d %v", unused, err)
	}
}

func TestOnlyLocalPeersAreAccepted(t *testing.T) {
	for addr, want := range map[string]bool{"127.0.0.1": true, "::1": true, "192.168.1.5": true, "10.1.2.3": true, "172.16.0.9": true, "fd00::1": true, "::ffff:192.168.0.2": true, "8.8.8.8": false, "2001:db8::1": false, "100.64.0.1": false} {
		if got := localPeer(&net.TCPAddr{IP: net.ParseIP(addr), Port: 1}); got != want {
			t.Fatalf("%s: %v", addr, got)
		}
	}
}

func TestConnectionsAreBounded(t *testing.T) {
	r := serve(t, filepath.Join(t.TempDir(), "broker"))
	var conns []net.Conn
	defer func() {
		for _, c := range conns {
			c.Close()
		}
	}()
	// Idle connections that never finish TLS hold every slot.
	for i := 0; i < maxConnections; i++ {
		c, err := net.Dial("tcp", r.addr)
		if err != nil {
			t.Fatal(err)
		}
		conns = append(conns, c)
	}
	short, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	inv := Invitation{Address: r.addr, BrokerID: r.b.ID(), Fingerprint: r.b.Fingerprint()}
	if err := identify(short, pinnedClient(inv.Fingerprint), inv.Address, inv.BrokerID); err == nil {
		t.Fatal("a connection past the limit was served")
	}
	for _, c := range conns {
		c.Close()
	}
	conns = nil
	// Freed slots are served again (the handshake timeout drops idle ones too).
	deadline := time.Now().Add(10 * time.Second)
	for {
		if err := identify(ctx(t), pinnedClient(inv.Fingerprint), inv.Address, inv.BrokerID); err == nil {
			break
		} else if time.Now().After(deadline) {
			t.Fatalf("slots not released: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestWriteFileIgnoresAStaleTemporaryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.json")
	if err := os.WriteFile(path+".tmp", []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeFile(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
		t.Fatalf("mode %v %v", info, err)
	}
	if raw, _ := os.ReadFile(path); string(raw) != "new" {
		t.Fatalf("content %q", raw)
	}
}
