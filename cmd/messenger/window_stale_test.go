package main

import "testing"

func TestOlderWindowProgram(t *testing.T) {
	own := "/home/u/.local/lib/agent-messenger/0.0.0-preview.85/messenger"
	cases := []struct {
		other string
		want  bool
	}{
		{"/home/u/.local/lib/agent-messenger/0.0.0-preview.83/messenger", true},
		{"/home/u/.local/lib/agent-messenger/0.0.0-preview.62/messenger (deleted)", true},
		{own, false},
		{"/home/u/.local/lib/agent-messenger/0.0.0-preview.86/messenger", false},
		{"/home/u/Downloads/messenger", false},
		{"/home/u/.local/lib/agent-messenger/custom/messenger", false},
		{"", false},
	}
	for _, c := range cases {
		if got := olderWindowProgram(c.other, own, "0.0.0-preview.85"); got != c.want {
			t.Errorf("%q: %t", c.other, got)
		}
	}
	if olderWindowProgram(cases[0].other, own, "dev") {
		t.Error("a dev build closed an installed window")
	}
}
