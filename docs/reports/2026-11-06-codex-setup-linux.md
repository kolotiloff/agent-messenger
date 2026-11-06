# Codex session setup: Linux checks

2026-11-06, cloud session (Linux x86-64), Go 1.24.7, Codex CLI 0.155.1 (`@openai/codex` from npm, linux-x64 musl binary). This is not a Mac acceptance run.

## Protocol facts confirmed on the real binary

The schema came from `codex app-server generate-json-schema --experimental`. A model-free probe used an empty temporary HOME/CODEX_HOME with no login:

- `thread/start` takes `dynamicTools` with a `namespace` spec (`messenger_bound` with `read_current` and `reply`), and it works without a login. It returned an idle, non-ephemeral thread with `turns: []`, `sandbox: {"type":"readOnly",...}` and `approvalPolicy: "never"`.
- Notifications arrive before responses, for example `configWarning`, `remoteControl/status/changed` and `thread/started`. On Linux without bubblewrap, `configWarning` reports the missing sandbox helper.
- After stdin closed, the server exited with code 0 and left no rollout file. This matches the earlier Mac finding: an empty thread is not saved.

Thread IDs from these probes stayed in temporary directories and are not recorded.

## Automated checks

`bash scripts/check.sh` passed: gofmt, `go mod verify`, `go vet` and all race tests. `scripts/check-builds.sh` passed for darwin/arm64, darwin/amd64, linux/amd64 and windows/amd64 (compile only).

New tests:

- `internal/adapters/codex_rpc_test.go` (RPC client):
  - Notifications are skipped and the response ID is matched.
  - A server request, wrong or string ID, error response, result plus error, null result, response without ID, malformed frame and EOF all fail. None is retried, and server error text does not leak.
  - `turn/start` and `thread/resume` are refused before writing.
  - After a timeout the client is closed.
- `internal/adapters/codex_rpc_test.go` (thread result): the tool declaration is checked. Ephemeral, missing ID, other workspace, active thread, existing turns, writable sandbox and other approval policies are rejected.
- `internal/core/codex_setup_test.go`:
  - The state machine order is enforced, and a fingerprint is required.
  - `begin` is idempotent only when the call is identical.
  - The alias is exclusive against `register-session`, Claude setup and a second setup.
  - An owned thread cannot be registered.
  - `not_started` is allowed only without a recorded process.
  - An uncertain setup keeps its alias.
  - On restart, active setups become `uncertain` and a bare intent becomes `not_started`.
- `internal/supervisor/codex_setup_test.go`, full client with a fake app-server:
  - The success case sends `initialize`, `initialized`, `config/read` and `thread/start` exactly once each, and no `turn/start`. The process environment is exactly `CODEX_HOME,HOME,PATH,TMPDIR`. The reservation names the thread and PID.
  - In the success case, enqueue and a second registration are refused, and so is a second setup with the same alias.
  - After a restart the setup is uncertain, and the reservation still blocks another data directory.
  - Inherited MCP, a server request during setup, no thread response, exit on `thread/start` and an ephemeral thread each end uncertain: exit observed, no thread stored, reservation kept, alias kept, `thread/start` sent at most once.
  - A server request after creation makes the setup uncertain.
  - Without `--allow-model-calls`, outside `--only-session`, and with the default profile, nothing is launched or recorded.
- Opt-in `TestInstalledCodexCreateSession` (`MESSENGER_CODEX_PROBE=/path/to/codex`): the full `create-codex-session` path against real Codex 0.155.1 in an empty profile reached `created`, with the thread ID in the reservation. No login, no model call. The existing `TestInstalledCodexAppServerPreflight` also passed on Linux.

A test-only issue was found and fixed: with check.sh's TMPDIR, long subtest names pushed the control socket path past the 108-byte Linux limit. The harness now uses a short root.

## Not verified

- macOS, where CI runs the same tests but the installed-CLI check is not run.
- A managed profile with a real login (Q-007).
- Any model turn.
- Delivering messages to the owner (T-002).
- Resolving uncertain setups (T-007).
