package supervisor

import "testing"

func TestAgentSandboxIsNoticed(t *testing.T) {
	env := map[string]string{}
	get := func(k string) string { return env[k] }
	if got := agentSandboxIn(get); got != "" {
		t.Fatalf("clean environment: %q", got)
	}
	env["CODEX_SANDBOX"] = "seatbelt"
	if got := agentSandboxIn(get); got != "CODEX_SANDBOX" {
		t.Fatalf("sandboxed: %q", got)
	}
}
