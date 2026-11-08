package broker

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"testing"
	"time"

	"filippo.io/edwards25519"
)

// shortHost starts a hosting service whose discovery listens on a free port
// of this computer, and returns it with a new invitation.
func shortHost(t *testing.T) (*Service, InviteCode) {
	t.Helper()
	pc, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := pc.LocalAddr().(*net.UDPAddr).Port
	pc.Close()
	old := DiscoveryPort
	DiscoveryPort = port
	t.Cleanup(func() { DiscoveryPort = old })
	host := newService(t, t.TempDir())
	if err := host.Enable(ScopeLocal, freePort(t)); err != nil {
		t.Fatal(err)
	}
	code, err := host.Invite(host.Status().Host.Addresses[0])
	if err != nil {
		t.Fatal(err)
	}
	if !ValidShortCode(code.Short) || code.ShortExpiresAt == "" {
		t.Fatalf("invitation without a short code: %+v", code)
	}
	return host, code
}

func wrongCode(code string) string {
	n, _ := strconv.Atoi(code)
	return fmt.Sprintf("%06d", (n+1)%1000000)
}

// A short code typed on another computer finds the host and joins it; the
// long code is never typed. A wrong code is refused, and a used code stops
// answering.
func TestShortInvitation(t *testing.T) {
	host, code := shortHost(t)
	member := newService(t, t.TempDir())
	if err := member.Connect(ctx(t), wrongCode(code.Short), "Ноутбук"); !errors.Is(err, ErrShortCode) {
		t.Fatalf("wrong code: %v", err)
	}
	typed := code.Short[:3] + " " + code.Short[3:]
	if err := member.Connect(ctx(t), typed, "Ноутбук"); err != nil {
		t.Fatalf("right code: %v", err)
	}
	if st := member.Status(); !st.Member.Connected || st.Member.DeviceName != "Ноутбук" {
		t.Fatalf("member status %+v", st.Member)
	}
	if len(host.Status().Host.Devices) != 1 {
		t.Fatalf("host devices %+v", host.Status().Host.Devices)
	}
	// Used: the host no longer answers, and asked directly it refuses.
	other := newService(t, t.TempDir())
	if err := other.Connect(ctx(t), code.Short, "Второй"); !errors.Is(err, ErrNoHost) {
		t.Fatalf("used code: %v", err)
	}
	if _, err := PairShort(ctx(t), code.Address, code.Short); !errors.Is(err, ErrShortGone) {
		t.Fatalf("used code asked directly: %v", err)
	}
	// The long code of the same invitation was spent by the short one.
	if err := other.Connect(ctx(t), code.Code, "Второй"); !errors.Is(err, ErrInvitation) {
		t.Fatalf("long code after the short one: %v", err)
	}
}

// Three wrong tries burn the code, even the right one fails after them.
func TestShortInvitationBurnsAfterThreeTries(t *testing.T) {
	_, code := shortHost(t)
	for i := 0; i < ShortTries; i++ {
		if _, err := PairShort(ctx(t), code.Address, wrongCode(code.Short)); !errors.Is(err, ErrShortCode) {
			t.Fatalf("try %d: %v", i+1, err)
		}
	}
	if _, err := PairShort(ctx(t), code.Address, code.Short); !errors.Is(err, ErrShortGone) {
		t.Fatalf("right code after three tries: %v", err)
	}
	member := newService(t, t.TempDir())
	if err := member.Connect(ctx(t), code.Short, "Ноутбук"); !errors.Is(err, ErrNoHost) {
		t.Fatalf("burnt code still answered: %v", err)
	}
	// The long code still works: only the short one is burnt.
	if err := member.Connect(ctx(t), code.Code, "Ноутбук"); err != nil {
		t.Fatalf("long code: %v", err)
	}
}

func TestShortInvitationExpires(t *testing.T) {
	host, code := shortHost(t)
	host.mu.Lock()
	host.broker.now = func() time.Time { return time.Now().Add(ShortTTL + time.Second) }
	host.mu.Unlock()
	if _, err := PairShort(ctx(t), code.Address, code.Short); !errors.Is(err, ErrShortGone) {
		t.Fatalf("expired code: %v", err)
	}
}

// The exchange itself: both sides agree only with the same code and the
// same certificate; a message that hides the code's blinding is refused.
func TestPairExchange(t *testing.T) {
	run := func(joinCode, hostCode, joinFP, hostFP string) (pairKeys, pairKeys) {
		x, a, err := pairStart(joinCode, pairM)
		if err != nil {
			t.Fatal(err)
		}
		y, b, err := pairStart(hostCode, pairN)
		if err != nil {
			t.Fatal(err)
		}
		hs, err := pairShared(hostCode, y, a, pairM)
		if err != nil {
			t.Fatal(err)
		}
		js, err := pairShared(joinCode, x, b, pairN)
		if err != nil {
			t.Fatal(err)
		}
		return derivePairKeys(joinCode, "b1", joinFP, a, b, js), derivePairKeys(hostCode, "b1", hostFP, a, b, hs)
	}
	j, h := run("123456", "123456", "fp", "fp")
	if string(j.hostConfirm) != string(h.hostConfirm) || string(j.joinConfirm) != string(h.joinConfirm) || string(j.box) != string(h.box) {
		t.Fatal("same code, different keys")
	}
	if j, h := run("123456", "123457", "fp", "fp"); string(j.hostConfirm) == string(h.hostConfirm) {
		t.Fatal("different codes agreed")
	}
	if j, h := run("123456", "123456", "fp-seen", "fp-host"); string(j.hostConfirm) == string(h.hostConfirm) {
		t.Fatal("different certificates agreed")
	}
	// w·M sent as the message cancels to the identity point.
	forged := new(edwards25519.Point).ScalarMult(codeScalar("123456"), pairM).Bytes()
	y, _, _ := pairStart("123456", pairN)
	if _, err := pairShared("123456", y, forged, pairM); err == nil {
		t.Fatal("forged message accepted")
	}
	box, _ := seal(h.box, []byte("secret"))
	if plain, err := open(j.box, box); err != nil || string(plain) != "secret" {
		t.Fatalf("box %q %v", plain, err)
	}
}

func TestShortCodes(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c, err := newShortCode()
		if err != nil || !ValidShortCode(c) {
			t.Fatalf("code %q %v", c, err)
		}
		seen[c] = true
	}
	if len(seen) < 190 {
		t.Fatalf("codes repeat: %d distinct of 200", len(seen))
	}
	for in, want := range map[string]string{"123456": "123456", " 123 456 ": "123456", "123-456": "123456", "12345": "", "am1:abc": "", "12345a": ""} {
		if got := shortCodeOf(in); got != want {
			t.Fatalf("shortCodeOf(%q) = %q", in, got)
		}
	}
}
