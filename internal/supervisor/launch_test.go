package supervisor

import "testing"

func TestProfileEnvReplacesInheritedProfile(t *testing.T) {
	parent := []string{"HOME=/users/me", "CODEX_HOME=/wrong", "CLAUDE_CONFIG_DIR=/also-wrong", "PATH=/bin"}
	env := profileEnv(parent, "codex", "/profiles/codex-a")
	if got := envValue(env, "CODEX_HOME"); got != "/profiles/codex-a" {
		t.Fatalf("CODEX_HOME = %s", got)
	}
	if got := envValue(env, "CLAUDE_CONFIG_DIR"); got != "/also-wrong" {
		t.Fatalf("unrelated profile changed: %s", got)
	}
	claude := profileEnv(parent, "claude", "/profiles/claude-a")
	if got := envValue(claude, "CLAUDE_CONFIG_DIR"); got != "/profiles/claude-a" {
		t.Fatalf("CLAUDE_CONFIG_DIR = %s", got)
	}
	cursor := profileEnv(parent, "cursor", "/profiles/cursor-a")
	if got := envValue(cursor, "CURSOR_CONFIG_DIR"); got != "/profiles/cursor-a" {
		t.Fatalf("CURSOR_CONFIG_DIR = %s", got)
	}
}

func envValue(env []string, key string) string {
	prefix := key + "="
	var found string
	for _, item := range env {
		if len(item) >= len(prefix) && item[:len(prefix)] == prefix {
			found = item[len(prefix):]
		}
	}
	return found
}
