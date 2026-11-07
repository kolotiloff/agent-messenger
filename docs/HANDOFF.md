# Handoff: current state

This file is the shared memory between tools (Claude Code, Cursor, Codex). Whoever stops working updates the **Current state** section; whoever starts reads it first. Tasks live in [TASKS.md](TASKS.md), decisions in [DECISIONS.md](DECISIONS.md), open questions in [QUESTIONS.md](QUESTIONS.md), the work history in [LOG.md](LOG.md).

## Owner's standing instruction for the night of 2026-11-07

The owner asked the agent to finish the program without them ("доделай бе меня полностью"), update the documentation on GitHub and clean out old code. Same rules as before: fake CLIs only, no real models, no Cursor, `active-runs.json` untouched, nothing retried on an unclear result, questions go to `QUESTIONS.md` with the most cautious option chosen. Removing the persistent paths (T-028) still needs the owner's explicit consent (Q-012).

## Current state

Updated 2026-11-07 (third night) by Claude Code.

- **Owner's instruction:** "доведи программу до релиза и сделай там настройку сервера", then an audit of the new work with all findings fixed.
- **preview.25 released** from `main` (PR #40): action errors as a chip above the input, a clear "old client in the background" message with the restart command, a fix for the task list status column.
- **T-015, first part** (branch `claude/t015-server-setup`): the background client can host a broker and a computer can join one by invitation. The window's «Сервер» tab enables it (local only by default, LAN on request), creates one-time invitations, lists and revokes devices, joins a broker, checks and drops the connection. `messenger status` shows the state. **Tasks do not travel between computers yet**; the tab says so. Details and file layout: [BROKER.md](BROKER.md); decisions D-033–D-035; Q-013 (device key in a 0600 file for now, not the Keychain).
- **Audit:** two independent reviews (security/protocol and window logic); every finding is fixed (list in [BROKER.md](BROKER.md) and [PR #41](https://github.com/kolotiloff/agent-messenger/pull/41)). One older issue outside this work went to the backlog as T-030.
- **Release:** PR #41 merged into `main` and released as preview.26 (see LOG for the outcome).
- **Verified here (Linux):** `check.sh`, `check-builds.sh`; tests in `internal/broker`, `internal/supervisor` (the client joins its own broker through the control command, revoke, check), `internal/localui`; the tab in headless Chromium in both themes, at 1100 and 600 px, against a real client.
- **Open PR #42** (`claude/t029-style-system`, not released): one set of style rules (D-036), the server note in a side column, «Запустить/Остановить сервер», Claude-style dropdowns, and a «Перезапустить» banner for an older background client (D-037). Per AGENTS.md it ships with the next functional release.
- **Not verified:** two real computers, a local network, the Mac app.
- **Next:**
  1. Owner: update, open «Сервер», turn the server on, invite a second computer when one is available.
  2. T-015 continued: tasks between computers through the broker, the VPS package and wizard, Keychain (Q-013).
  3. Q-012, then T-028 (T-027 is closed: the owner reported preview.24 works).

Previous update 2026-11-07 (second night) by Claude Code.

- **T-027 first run done with the owner on preview.23** (real models on the Mac): a Codex task and a Claude task with two owner questions, both verified and reported to the Claude-app coordinator through `wait`. Q-010 answered (ToolSearch works), Q-011 and Q-009 answered (D-028, D-029).
- **preview.24** (branch `claude/t027-window-fixes`, merged into `main`) contains everything found in that run:
  - Claude workers may edit files (`acceptEdits`, D-028);
  - the window's client allows tasks and resumes itself (D-029), so no terminal commands are needed;
  - a chat-like stop button (square button in the input field, Esc; D-031);
  - the coordinator may choose the worker's model (D-030), shown in the task header;
  - window fixes: Enter sends, no horizontal scroll, one line per program in the computer card, "Ждёт запуска" for unlaunched runs, duration from launch, code blocks and tool lines in worker text, specific error texts instead of "client unavailable";
  - a relative `--root` now works;
  - cleanup: unused code and `experiments/` removed (D-032).
- **Verified here (Linux):** `check.sh`, `check-builds.sh`, all tests with fake CLIs (including the model reaching both the first and the resumed run), the window in headless Chromium (stop by click and by Esc, light and dark), `check-desktop.py`. CI on Linux and macOS (includes the Mac app build).
- **Owner's Mac (2026-11-07 morning):** the owner tested preview.24 and reported that everything works. T-027 is closed on that report; the individual cases were not written up separately.
- **Next:** Q-012 (may the persistent Claude stream and Codex app-server be removed?), then T-028.

Previous update 2026-11-07 (first night) by Claude Code.

- **Built overnight** on `claude/task-core-night` ([PR #36](https://github.com/kolotiloff/agent-messenger/pull/36), into `claude/vigilant-darwin-34d88b`): T-022, T-024, T-025, T-026 and T-029. The task model now works end to end on one computer. Details are in [TASK_MODEL.md](TASK_MODEL.md).
  - projects and folders;
  - tasks with verified worker runs;
  - the coordinator MCP server with `wait`;
  - the worker profile;
  - the new window.
- **Verified on Linux:**
  - `check.sh` and `check-builds.sh`;
  - tests with fake CLIs;
  - real Codex 0.155.1 and Claude Code 2.1.319 as workers against local fake model APIs ([report](reports/2026-11-06-task-workers-linux.md));
  - the window in headless Chromium.
- **Not verified:**
  - real models;
  - the Mac app (WKWebView, the folder panel bridge, Swift build: CI only);
  - the Claude app as coordinator with this server (the `wait` contract was proven by the T-023 pilot).
- **Open questions for the owner:**
  - Q-009: how the window allows task launches. Today only `serve --allow-tasks`; the window's own client does not launch tasks.
  - Q-010: Claude Code hides MCP tools behind `ToolSearch`.
- **Release (2026-11-07):** the owner chose to merge PR #36 into `claude/vigilant-darwin-34d88b` and that into `main`, and to release preview.23 for T-027. Before that, macOS CI found a stream start race in the persistent Claude path (fixed: one stream process starts at a time) and two test-only path and timing issues.
- **Starting tasks for T-027:** the window's own client does not launch tasks (Q-009). Stop it with `messenger --root DATA shutdown`, start it with `messenger --root DATA start --claude-stream --allow-tasks`, then `messenger --root DATA resume`. `DATA` and the messenger path are shown in the window's coordinator command.
- **Next:**
  1. Review PR #36.
  2. Decide Q-009.
  3. Run T-027 on the Mac with real models: build a preview, create a project in the window, connect the Claude-app coordinator with the shown command, start the client with tasks allowed, then one Claude and one Codex task with a question round.
  4. Then T-028.
- **Persistent paths** (Claude stream, Codex app-server) are untouched and still work. Their replies are now published only after verification (T-022).
- **Schema version is 2.** Older clients refuse a data directory opened by this build.

Earlier state (kept for history):

Previous update 2026-11-06 by Claude Code (D-021–D-026 decided, window approved).

- **`main` ([PR #29](https://github.com/kolotiloff/agent-messenger/pull/29)):** T-001 and T-002 are done. `create-codex-session` creates a Codex thread owned by a live app-server, and queued messages to its alias are delivered one turn at a time in that same process. The turns use the bound `read_current`/`reply` tools. Details: [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md), [setup report](reports/2026-11-06-codex-setup-linux.md), [delivery report](reports/2026-11-06-codex-delivery-linux.md).
- **Verified:** only on Linux, with a fake app-server. No real Codex model turn has run yet.
- **`main` ([PR #29](https://github.com/kolotiloff/agent-messenger/pull/29)):** T-007 is done. `recover-codex` retires an uncertain Codex owner after a proven reboot. It is refused on Linux and Windows, which have no boot identity source yet.
- **`main` ([PR #30](https://github.com/kolotiloff/agent-messenger/pull/30)):** T-005 is done. Replies to another local agent are delivered as new messages, with a limit of 3 per chain ([ROUTING.md](ROUTING.md)). A persistent Claude session and a Codex owner exchange messages side by side; this was verified with fake CLIs.
- **`main` ([PR #30](https://github.com/kolotiloff/agent-messenger/pull/30)):** T-003 is done. The Codex config is checked again before every turn, and the database has a schema version.
- **`main` ([PR #31](https://github.com/kolotiloff/agent-messenger/pull/31)):** T-008 is done. Up to 4 persistent Claude sessions run side by side.
- **Releases:** Mac previews are now built, signed and published by the Release Mac preview workflow, using the `SPARKLE_ED_PRIVATE_KEY` secret ([UPDATES.md](UPDATES.md)). preview.20 was the first one released this way.
- **`main` ([PR #32](https://github.com/kolotiloff/agent-messenger/pull/32)):** T-006 is done. The window creates a Codex session and allows model calls only for it ([SESSION_SETUP.md](SESSION_SETUP.md)).
- **T-004 accepted on the Mac with preview.21:** real Codex answered two messages from the window, with both turns verified ([report](reports/2026-11-06-codex-window-mac.md)). A paused queue after setup was found and fixed in [PR #33](https://github.com/kolotiloff/agent-messenger/pull/33).
- **Direction decided (2026-11-06):** the product becomes "one task, one session" ([D-021–D-023](DECISIONS.md), [RETHINK.md](RETHINK.md)). The coordinator is the owner's Claude-app chat with Messenger over MCP and a blocking `wait`, confirmed by the [T-023 pilot](reports/2026-11-06-wait-pilot-claude-mac.md). Work so far is on branch `claude/vigilant-darwin-34d88b` (documentation and `experiments/wait_probe`); no product code changed yet.
- **Window and projects decided (2026-11-06):** one working folder per project per computer (D-025), several projects at once with one coordinator chat each (D-026), window approved as [docs/mockups/2026-11-06-task-window.html](mockups/2026-11-06-task-window.html) (D-024, details in T-029). PR #35 (T-009) is cancelled: do not merge or build on it.
- **Next:** T-022 (publish a reply only after verification), then T-024, T-025, T-026, T-029, then the Mac acceptance T-027 and T-028 in [TASKS.md](TASKS.md). Persistent paths stay until T-028. Claude ↔ Codex with real models happens in T-027.
- **Mac status:** after the freeze and reboot, preview.19 would not open. All reservations pointed to a test directory in `/private/tmp` that macOS had deleted. The owner moved `active-runs.json` aside after checking the evidence, and the window now works with the default data directory and empty history ([report](reports/2026-11-06-reboot-temp-root-mac.md)). The old Claude test session no longer exists. Fixes (`recover-missing-root`, refusing temporary data directories) are in PR #31 and ship with the next build.

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
