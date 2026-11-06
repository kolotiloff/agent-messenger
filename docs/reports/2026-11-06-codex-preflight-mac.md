# First components of persistent Codex on Mac

November 6, 2026. macOS arm64, installed Codex CLI 0.155.1, Go 1.27.3.

## Checking the real app-server without a model

```sh
MESSENGER_CODEX_PROBE=/absolute/path/to/codex \
  go test -race -count=1 -ldflags=-linkmode=external ./internal/adapters
```

The variable enables only the additional check of the installed CLI. A regular `go test` skips it. Each case creates separate temporary HOME/CODEX_HOME and a working folder, the environment is set by an explicit list, credentials are not copied. The auth storage is set to `file` inside the empty profile. A separate stdio process is launched, not the user's daemon.

Only `initialize`, the `initialized` notification and `config/read` are sent. The test transport has no permission for `thread/start` or `turn/start`; a forbidden method is rejected before it is written. Three checks passed:

| Profile | Go check result |
|---|---|
| Empty managed profile | Accepted |
| Artificial MCP in the file + empty MCP table in the flags | Rejected: the server stayed enabled |
| The same MCP with explicit `enabled=false` | Accepted |

In all three cases the app-server exited normally after stdin was closed. No model turns. Working profiles and GUI sessions were not used.

## Automated Go checks

- Incomplete settings, `null` instead of disabling, implicitly enabled MCP, enabled hooks/goals, write permission and web search are rejected.
- A wrong/empty RPC ID, a server request instead of a response, simultaneous result/error, corrupted JSON and EOF do not lead to the request being retried. The content of a server error does not end up in the test transport's message.
- Linked reading and writing of a reply with exact thread/turn IDs were verified. The caller must bind the handlers to a specific attempt in the database.
- Another conversation/turn, an old receipt, a model-chosen recipient, a repeated call ID and an unrelated tool do not trigger writing a reply. After a refusal, the next correct call does not reopen the tools either.
- The reply is written before the tool returns successfully. A write error leaves the result uncertain; turn completion does not mask it.
- `completed` without a linked reply, `failed`, `interrupted`, someone else's completion, a call after `Stop` and a repeated completion are rejected.

The full `scripts/check.sh` passed on Mac on Go 1.27.3 and Go 1.22.2: formatting, dependencies, vet, all race tests. After tightening the test RPC decoder, the adapters package with the real non-model checks of the installed Codex and the full suite on Go 1.22.2 passed again.

All four GitHub jobs for `b3aa4439691e6d2fa5b38f69483673ca5b2c1110` passed in [run 42206000660](https://github.com/kolotiloff/agent-messenger/actions/runs/42206000660): Mac/Linux, Go 1.22.2/stable, compiling target platforms and Mac package checks. [PR #25](https://github.com/kolotiloff/agent-messenger/pull/25) is merged into the main branch. The installed Codex was not run in CI; its three non-model probes were performed separately on the local Mac.

## Acceptance boundaries

These are preparatory components, not yet launching Codex from the product. Real model turns, issuing and saving the ID of a new session, resume, a persistent transport, handling events before the RPC response, binding to SQLite and the global reservations, protection against configuration changes after the check, and existing authorization in the managed profile remain the next steps. The current Claude and its queue were not changed. No new user build was released from this intermediate work.
