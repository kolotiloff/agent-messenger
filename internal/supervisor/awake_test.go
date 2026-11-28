package supervisor

import "testing"

func TestSleepPreferenceRoundTrip(t *testing.T) {
	root := t.TempDir()
	s := &Supervisor{root: root, awake: initialAwakePolicy(root)}
	if s.awake.status.Mode != "off" || s.awake.status.Active {
		t.Fatalf("unexpected initial power state: %#v", s.awake.status)
	}
	if _, err := s.SleepMode("working"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SleepMode("invalid"); err == nil {
		t.Fatal("invalid sleep mode accepted")
	}
	if mode := initialAwakePolicy(root).status.Mode; mode != "working" {
		t.Fatalf("incorrect persisted mode: %q", mode)
	}
	if _, err := s.SleepMode("off"); err != nil {
		t.Fatal(err)
	}
	if mode := initialAwakePolicy(root).status.Mode; mode != "off" {
		t.Fatalf("incorrect persisted mode: %q", mode)
	}
}
