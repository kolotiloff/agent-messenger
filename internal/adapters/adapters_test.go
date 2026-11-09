package adapters

import "testing"

func TestExplicitResumeNeedsTheExactID(t *testing.T) {
	if err := ExplicitResume([]string{"claude", "--resume", "conversation-1"}, "conversation-1"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"claude", "--continue", "conversation-1"},
		{"codex", "resume", "--last", "conversation-1"},
		{"claude", "--resume-last=conversation-1"},
		{"claude", "--resume", "other"},
	} {
		if err := ExplicitResume(args, "conversation-1"); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
	if err := ExplicitResume([]string{"claude"}, ""); err == nil {
		t.Fatal("accepted an empty id")
	}
	for _, id := range []string{"", "last", "Continue", " latest "} {
		if rejectLooseID(id) == nil {
			t.Fatalf("accepted %q as an id", id)
		}
	}
	if rejectLooseID("conversation-1") != nil {
		t.Fatal("refused an exact id")
	}
}
