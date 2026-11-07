package main

import (
	"path/filepath"
	"testing"
)

func TestInstallPrefix(t *testing.T) {
	prefix := filepath.Join(string(filepath.Separator)+"home", "a", ".local")
	installed := filepath.Join(prefix, "lib", "agent-messenger", "0.0.0-preview.33-abc", "messenger")
	if got := installPrefix(installed); got != prefix {
		t.Fatalf("installed: %q", got)
	}
	for _, plain := range []string{
		filepath.Join(prefix, "Downloads", "AgentMessenger-Linux-x64"),
		filepath.Join(prefix, "lib", "other", "v", "messenger"),
		filepath.Join(prefix, "bin", "agent-messenger", "v", "messenger"),
	} {
		if got := installPrefix(plain); got != "" {
			t.Fatalf("%s: %q", plain, got)
		}
	}
}
