package platform

import (
	"errors"
	"strings"
	"testing"
)

func TestRecoveryEpochRequiresSameHostAndProspectiveBootUUID(t *testing.T) {
	old := RecoveryEpoch{Version: 1, Host: strings.Repeat("a", 64), Boot: "11111111-1111-1111-1111-111111111111"}
	current := old
	current.Boot = "22222222-2222-2222-2222-222222222222"
	if p, err := ProvePreviousBoot(old, current); err != nil || !p.Proven {
		t.Fatal(p, err)
	}
	for _, change := range []func(*RecoveryEpoch){
		func(e *RecoveryEpoch) { e.Boot = old.Boot },
		func(e *RecoveryEpoch) { e.Host = strings.Repeat("b", 64) },
		func(e *RecoveryEpoch) { e.Version = 0 },
		func(e *RecoveryEpoch) { e.Boot = "{ sec = 1234, usec = 0 }" },
		func(e *RecoveryEpoch) { e.Boot = "00000000-0000-0000-0000-000000000000" },
	} {
		e := current
		change(&e)
		if p, err := ProvePreviousBoot(old, e); err == nil || p.Proven {
			t.Fatal("unsafe proof", p, err)
		}
		if p, err := ProvePreviousBoot(e, old); err == nil || p.Proven {
			t.Fatal("unsafe stored proof", p, err)
		}
	}
}

func TestNativeRecoveryEpochIsStableAndWellFormed(t *testing.T) {
	a, err := CurrentRecoveryEpoch()
	if errors.Is(err, ErrUnsupported) {
		t.Skip("epoch provider not implemented on this OS")
	}
	if err != nil || !a.Valid() {
		t.Fatal("native epoch unavailable or invalid", err)
	}
	b, err := CurrentRecoveryEpoch()
	if err != nil || a != b {
		t.Fatal("epoch changed without reboot", err)
	}
	if p, err := ProvePreviousBoot(a, b); err == nil || p.Proven {
		t.Fatal("live boot accepted")
	}
}
