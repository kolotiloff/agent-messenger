package core

import (
	"os"
	"reflect"
	"testing"
)

func TestScopedClaimRejectsWrongProviderWithoutReserving(t *testing.T) {
	s, env := openTest(t)
	for _, p := range []string{"claude", "codex"} {
		if _, err := s.Register(p, env, p+"-test", p); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Enqueue(p+"-message", p, "user", "payload"); err != nil {
			t.Fatal(err)
		}
	}
	before, _ := s.Snapshot()
	for _, alias := range []string{"missing", "codex", "claude"} {
		if _, err := s.ClaimProvider(alias, "claude"); err == nil {
			t.Fatal("invalid or unconfigured scope reserved", alias)
		}
	}
	after, _ := s.Snapshot()
	if !reflect.DeepEqual(before, after) {
		t.Fatal("failed scoped claim changed the queue")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigureLaunch("claude", exe, env); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimProvider("claude", "claude"); err != nil {
		t.Fatal(err)
	}
	after, _ = s.Snapshot()
	if len(after.Attempts) != 1 || after.Attempts[0]["request_id"] != "claude-message" {
		t.Fatal("wrong mailbox claimed")
	}
}
