# Tasks

The task list shared by all tools. Stages and exit conditions are in [RELEASE_PLAN.md](RELEASE_PLAN.md); this file is the working breakdown.

How to use it:

- Each task has an ID (`T-NNN`) that never changes or gets reused. Use it in branch names, commit messages and PR titles, e.g. `T-001: persist Codex setup intent`.
- When you start a task, move it to **Now** and write the branch name. When it is done, move it to **Done** with the PR link.
- "Owner" means who has to act: `agent` for any coding agent, `owner` for the project owner (Mac runs, accounts, decisions).
- Questions that block a task go to [QUESTIONS.md](QUESTIONS.md), with the task marked `blocked: Q-NNN`.

## Now

| ID | Task | Owner | Branch / notes |
|---|---|---|---|
| T-001 | Create and persist a Codex session before the first message: persist the setup intent before launching the process, write the shared reservation before launching the CLI, check the effective config before `thread/start`, then atomically store the server-issued thread ID and owner. Lost confirmation or restart leaves an uncertain state; no automatic `thread/start` retry. See [HANDOFF.md](HANDOFF.md) steps 1–3. | agent | WIP on `claude/quirky-cannon-u0n3qm`; see HANDOFF |

## Next

| ID | Task | Owner | Notes |
|---|---|---|---|
| T-002 | Persistent Codex transport: bind the confirmed turn ID to an attempt in SQLite, buffer early events within a bound, write send intent before stdin, connect `CodexTurnGuard` to the existing read/receipt/reply checks. HANDOFF steps 4–5. | agent | after T-001 |
| T-003 | Codex failure tests on a fake app-server: two sequential turns, failures at every boundary, repeated RPC/call IDs, foreign turns, profile change, restart, no model calls while idle, old client refusing newer state. HANDOFF step 6. | agent | with T-002 |
| T-004 | Codex acceptance on Mac: two real messages in a new separate test Codex session, then exact resume of the saved session. HANDOFF step 7. | owner + agent | needs the Mac; blocked: Q-005, Q-007 |
| T-005 | Several independent owners and Claude ↔ Codex routing with a limit on automatic replies. `Supervisor.current` and `runStream` assume one owner today. HANDOFF step 8. | agent | after T-004 |
| T-006 | Launch Codex sessions from the window. | agent | after T-005 |

## Backlog

| ID | Task | Owner | Notes |
|---|---|---|---|
| T-010 | Safely terminate a CLI and release its session within the same Mac boot. This is an open research problem: release by timer, empty process group or one exited PID is not allowed. | agent | see [RELEASE_PLAN.md](RELEASE_PLAN.md) |
| T-011 | Switch the version of a running background client, plus full rollback. | agent | see [UPDATES.md](UPDATES.md) |
| T-012 | Persistent Cursor adapter. | agent | blocked: Q-004 |
| T-013 | Developer ID signing and notarization of the Mac app. | owner | blocked: Q-003 |
| T-014 | Intel Mac build and acceptance. | agent + owner | |
| T-015 | Network broker, device registration, Mac ↔ Windows exchange. Stage 5. | agent + owner | needs a Windows PC; Q-006 |
| T-016 | Windows and Linux runtime acceptance and packages. Stage 6. | agent + owner | Q-006 |
| T-017 | Coordinator role handover. Stage 6. | agent | |
| T-018 | Delete old remote branches whose content is already in `main`. | agent | blocked: Q-002 |
| T-019 | Decide and implement the UI language. | owner → agent | blocked: Q-001 |

## Done

| ID | Task | PR |
|---|---|---|
| T-100 | Translate documentation to English; shared agent rules in `AGENTS.md`; close stale draft PRs #2–#18. | [#26](https://github.com/kolotiloff/agent-messenger/pull/26) |
| T-101 | Fail-closed Codex app-server preflight and turn guards (components only, not wired into `serve`). | [#25](https://github.com/kolotiloff/agent-messenger/pull/25) |
| T-102 | preview.19: create and explicitly launch a separate test Claude session from the window; accepted on Mac. | see [report](reports/2026-11-06-window-launch-mac.md) |

Earlier work is summarized in [RELEASE_PLAN.md](RELEASE_PLAN.md) ("What is already done") and [LOG.md](LOG.md).
