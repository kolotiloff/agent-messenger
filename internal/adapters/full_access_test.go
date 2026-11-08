package adapters

import (
	"strings"
	"testing"
)

// The owner's «Разрешить любые команды» switch (T-054, D-065) lifts each
// agent's own permission checks and sandbox; without it nothing changes.
func TestFullAccessTaskArgs(t *testing.T) {
	cases := []struct {
		provider, session string
		want, banned      []string
	}{
		{"claude", testUUID, []string{"\x00--permission-mode\x00bypassPermissions\x00"}, []string{"acceptEdits"}},
		{"codex", "", []string{"\x00--dangerously-bypass-approvals-and-sandbox\x00"}, []string{"--sandbox"}},
		{"cursor", "", []string{"\x00--force\x00", "\x00--sandbox\x00disabled\x00"}, []string{"\x00--sandbox\x00enabled\x00"}},
	}
	for _, c := range cases {
		s := taskSpec(c.provider, ProfileWorker, c.session, false)
		s.FullAccess = true
		args, err := TaskArgs(s)
		if err != nil {
			t.Fatal(err)
		}
		j := joined(args)
		for _, w := range c.want {
			if !strings.Contains(j, w) {
				t.Errorf("%s: missing %q in %v", c.provider, w, args)
			}
		}
		for _, b := range c.banned {
			if strings.Contains(j, b) {
				t.Errorf("%s: has %q in %v", c.provider, b, args)
			}
		}
		// The test profile never runs with full access.
		s.Profile = ProfileMessengerOnly
		args, err = TaskArgs(s)
		if err != nil {
			t.Fatal(err)
		}
		j = joined(args)
		for _, b := range []string{"bypassPermissions", "dangerously", "\x00--sandbox\x00disabled\x00"} {
			if strings.Contains(j, b) {
				t.Errorf("%s messenger-only: has %q", c.provider, b)
			}
		}
	}
}
