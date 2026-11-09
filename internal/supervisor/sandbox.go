package supervisor

import "os"

// agentSandboxMarkers are environment variables a coding agent sets for the
// commands it runs inside its sandbox (T-129). A client started from such a
// command inherits the sandbox, and so does every worker it launches: no
// network, only some folders.
var agentSandboxMarkers = []string{"CODEX_SANDBOX", "CODEX_SANDBOX_NETWORK_DISABLED"}

// AgentSandbox names the marker found in this process's environment, or ""
// when the client was not started from inside an agent's sandbox.
func AgentSandbox() string { return agentSandboxIn(os.Getenv) }

func agentSandboxIn(getenv func(string) string) string {
	for _, name := range agentSandboxMarkers {
		if getenv(name) != "" {
			return name
		}
	}
	return ""
}
