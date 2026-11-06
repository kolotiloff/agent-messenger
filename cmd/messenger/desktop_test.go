package main

import "testing"

func TestDesktopCannotAcceptModelFlags(t *testing.T) {
	for _, args := range [][]string{{"--allow-model-calls"}, {"--claude-stream"}, {"anything"}} {
		if err := runDesktop("", args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
