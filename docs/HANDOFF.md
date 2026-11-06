# Handoff: current state

This file is the shared memory between tools (Claude Code, Cursor, Codex). Whoever stops working updates the **Current state** section; whoever starts reads it first. Tasks live in [TASKS.md](TASKS.md), decisions in [DECISIONS.md](DECISIONS.md), open questions in [QUESTIONS.md](QUESTIONS.md), the work history in [LOG.md](LOG.md).

## Current state

Updated 2026-11-06 by Claude Code.

- **`main` ([PR #29](https://github.com/kolotiloff/agent-messenger/pull/29)):** T-001 and T-002 are done. `create-codex-session` creates a Codex thread owned by a live app-server, and queued messages to its alias are delivered one turn at a time in that same process. The turns use the bound `read_current`/`reply` tools. Details: [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md), [setup report](reports/2026-11-06-codex-setup-linux.md), [delivery report](reports/2026-11-06-codex-delivery-linux.md).
- **Verified:** only on Linux, with a fake app-server. No real Codex model turn has run yet.
- **No work in progress on any branch.**
- **`main` ([PR #29](https://github.com/kolotiloff/agent-messenger/pull/29)):** T-007 is done. `recover-codex` retires an uncertain Codex owner after a proven reboot. It is refused on Linux and Windows, which have no boot identity source yet.
- **`main` ([PR #30](https://github.com/kolotiloff/agent-messenger/pull/30)):** T-005 is done. Replies to another local agent are delivered as new messages, with a limit of 3 per chain ([ROUTING.md](ROUTING.md)). A persistent Claude session and a Codex owner exchange messages side by side; this was verified with fake CLIs.
- **Next:** T-004, the first real run on the Mac, needs the owner (Q-005, Q-007). Without the Mac: T-008 (several persistent Claude sessions) or T-003 (remaining Codex failure tests).
- **Mac status:** the Mac froze and rebooted on 2026-11-06 after preview.19. The live Claude test owner from that acceptance is gone. Its reservation is kept until `recover-stream` is run on the Mac; this is not needed for current work.

Codex CLI for model-free checks in a cloud session: `npm install @openai/codex@0.155.1` in a scratch directory. Then use the native binary under `node_modules/@openai/codex-linux-x64/vendor/*/bin/codex` with `MESSENGER_CODEX_PROBE=...`.

The sections below are the original handoff from Codex (2026-11-06, base `fcec2d88cc000ca9afad5427e46bcf5701c7fb15`). They still describe the product state and the plan for the Codex adapter.

## Product and way of working

Agent Messenger is messaging between agents on macOS, Windows and Linux. Any supported agent can be the coordinator. The main client is written in Go: `cmd/messenger`, `internal`. The Python in `experiments` is research, not a direction for rewriting the product. The native Mac window lives in `desktop`; the interface lives in `internal/localui/assets`.

In the future the broker runs on one of the computers or on a VPS. The program is not tied to the user's projects, game engines, or the systems that store their files. Overall plan: [RELEASE_PLAN.md](RELEASE_PLAN.md).

Continue with the functional stages. Fold interface feedback into the same release; do not ship a separate build for every small item. The already chosen light/dark themes and layout are kept. Do not present preparatory components as a finished window feature. When a stage is finished, update the sources, the documentation and the verification results together.

## What actually works

- The installed user build is preview.19. The signed shell update to it through the updater has been verified.
- A separate new test session of real Claude Code 2.1.318 was created through the window. Two messages got `ACK:WINDOW-ONE` and `ACK:WINDOW-TWO` in one persistent process. The replies appeared automatically; three old messages of other sessions were not processed.
- After the check the queue was paused and the live Claude owner was kept. As of the handoff, the new developer must not stop it, release its reservation, or take over that conversation as a working development session. Check the actual state first with the standard tools, not by PIDs from old reports.
- Waiting and reading state do not send model requests. No timer-driven model prompts are needed.
- The background client and the shell update are separate processes. Automatic replacement of a running client and full rollback are not yet accepted.

Details: [preview.19 report](reports/2026-11-06-window-launch-mac.md), [scoped launch](SCOPED_LAUNCH.md), [updates](UPDATES.md).

## Current stage: persistent Codex

The installed CLI was studied at version 0.155.1. Persistent Codex is **not yet connected to serve or the window**. No real Codex model turns have been run with this new adapter.

Added and verified:

- `internal/adapters/codex_appserver.go`: arguments for a separate stdio app-server for the managed profile, and a check of the actual `config/read`. Missing fields and `null` are not treated as disabled capabilities.
- `internal/adapters/codex_turn.go`: checks of the exact thread/turn ID, the `read_current` → `reply` sequence, the receipt, and the reply write through handlers bound by the controller. A refusal, a repeat, an old receipt, `failed`/`interrupted`, and completion without a reply do not become success. This is not an RPC transport and not process management.
- Three checks of the real app-server without a model: a clean profile was accepted; an enabled inherited MCP was rejected; an explicitly disabled MCP was accepted. None of these checks call `thread/start` or `turn/start`.
- Full local Mac checks passed on Go 1.27.3 and 1.22.2. All four jobs of [CI 42206000660](https://github.com/kolotiloff/agent-messenger/actions/runs/42206000660) passed on Mac/Linux. Cross-compilation is not Windows acceptance.

Read [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md) and the [verification report](reports/2026-11-06-codex-preflight-mac.md).

Confirmed Codex specifics:

1. `app-server` does not accept `--ignore-user-config`; do not blindly carry over flags from one-shot `exec`.
2. `-c 'mcp_servers={}'` does not remove servers inherited from the file. Launch arguments alone are not enough to prove isolation.
3. `thread/start` issues the ID itself. In the investigation, an empty new conversation had no rollout for `thread/resume` after the process exited. Do not automatically replace a session that was not found with a new conversation.
4. Dynamic tools are experimental; check the format against the installed CLI and the [official App Server](https://learn.chatgpt.com/docs/app-server).

## Next task

Create and persist a Codex session before the first message, then connect the persistent transport to the client's existing guarantees. No changes for this were made in the last pass: only code reading and design were done.

Start by checking the current schema constraints:

- `SetupNewClaude` uses a UUID known in advance; Codex's server-issued ID needs a different creation order.
- `stream_runs.id` has a foreign key to `attempts(token)`. Currently a process owner cannot exist before the first message attempt. Do not create a fake message or a fake ID to work around this link.
- `ConfirmStreamIdentity` explicitly selects `provider='claude'`. You cannot simply drop the condition without checking the provider, the conversation and the specific turn.
- `Supervisor.current` and `runStream` are designed for a single persistent owner. Multiple agents require a separate controller change; one successful Codex reply does not yet prove Claude ↔ Codex.

Proposed implementation sequence, which may be refined with justification after reading the code:

1. Persist the creation intent before starting the process: a random setup ID, a reserved alias, the canonical managed profile, the verified CLI/workspace, and a configuration fingerprint. Separate creating the owner from a message attempt.
2. Write the shared reservation before launching the CLI. Check the actual configuration before creating the conversation; separately work out authentication for the managed profile without publishing or logging credentials.
3. Match the RPC of the creation response. Atomically persist the issued ID and the owner before the first message. A lost confirmation, a write error or a restart leaves an uncertain result; `thread/start` is not retried automatically.
4. Bind the confirmed turn ID to a specific attempt in SQLite. Buffer events that arrive before the RPC response, within a bounded buffer; tools are unavailable until identity is confirmed. Record the send intent before writing to stdin.
5. Connect `CodexTurnGuard` to the existing read/receipt/reply checks in the database. Success is only an exact `turn/completed` with status `completed` plus a persisted linked reply. EOF, timeout and stop do not release the owner and do not trigger a retry.
6. Test with a fake CLI: two consecutive processings, failures at every boundary, repeated RPC/call IDs, foreign turns, profile changes, restart, and no model calls while waiting. Check compatibility and that an old client refuses unsupported new state.
7. After that, two real messages in a separate new Codex test session, then an exact resume of the persisted session. Do not use working chats or GUI chats.
8. The next functional stage is independent owners and Claude ↔ Codex routing with a limit on automatic replies, then connection from the window.

## Constraints that must not be relaxed

Do not delete `active-runs.json`, do not empty it, and do not release a reservation based on a timer, a single PID, or an empty process group. Do not bypass the shared registry to launch from a different database. A signal to a process does not prove that all its descendants have exited. When the result is uncertain, do not repeat actions automatically.

No user session IDs, tokens, auth files, private SQLite dumps or full logs in GitHub. The repository is public. Do not run Cursor CLI: the user previously asked to keep it disabled. Do not restart the Mac automatically. Do not use working chats as test chats.

## Getting started and verification

Work in a regular clone of the [repository](https://github.com/kolotiloff/agent-messenger) from the current `main`, in a separate branch. The previous developer's local copy of the sources may have been a mirror without `.git`; do not automatically treat it as a full checkout.

```sh
bash scripts/check.sh
```

Go 1.22.2 or later is required. If needed, point to the installed Go with `GO_BIN`. The script itself selects the external linker for race tests on Mac. An additional model-free check of the installed Codex is described in the report; it is skipped by default. Real models are not run in the regular tests.

Deliverable of the next stage: a PR with the implementation, failure tests and updated documentation; list separately what actually passed and what remains. For audit, hand over the PR link and the exact SHA. Publish a new application package after the functional step is finished and verified, not for the sake of an intermediate library.
