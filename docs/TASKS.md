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
| — | Nothing in progress. Next without the Mac: T-008 or T-003; T-004 needs the Mac. | | |

## Next

| ID | Task | Owner | Notes |
|---|---|---|---|
| T-003 | Remaining Codex failure tests: managed profile changed between turns, and an old client refusing newer state. The fake app-server tests from T-002 already cover two sequential turns, boundary failures, repeated IDs, foreign turns and restart. | agent | |
| T-004 | Codex acceptance on Mac: two real messages in a new separate test Codex session, then exact resume of the saved session. HANDOFF step 7. | owner + agent | needs the Mac; blocked: Q-005, Q-007 |
| T-006 | Launch Codex sessions from the window. | agent | |
| T-008 | Several persistent Claude sessions at once. `Supervisor.current` and `runStream` hold one stream, so a second Claude session waits; Codex owners already run side by side. | agent | |

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
| T-005 | Replies between agents: a reply to another local alias is queued for it in the same transaction, with a limit of 3 automatic replies per chain; Claude stream and Codex owner exchange messages side by side (fake CLIs). Several Claude streams moved to T-008. | PR5_LINK |
| T-007 | `recover-codex`: retire an uncertain Codex owner after a proven reboot; session closed without retry, history kept. | [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29) |
| T-002 | Deliver queued messages to the Codex owner: one turn per message, send intent before `turn/start`, turn ID bound to the attempt, bound tools through SQLite, success only on verified completion; verified on Linux with a fake app-server. | [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29) |
| T-001 | Create and persist a Codex session before the first message (`create-codex-session`); verified on Linux with a fake app-server and real Codex 0.155.1, no model. | [PR #28](https://github.com/kolotiloff/agent-messenger/pull/28) |
| T-100 | Translate documentation to English; shared agent rules in `AGENTS.md`; close stale draft PRs #2–#18. | [#26](https://github.com/kolotiloff/agent-messenger/pull/26) |
| T-101 | Fail-closed Codex app-server preflight and turn guards (components only, not wired into `serve`). | [#25](https://github.com/kolotiloff/agent-messenger/pull/25) |
| T-102 | preview.19: create and explicitly launch a separate test Claude session from the window; accepted on Mac. | see [report](reports/2026-11-06-window-launch-mac.md) |

Earlier work is summarized in [RELEASE_PLAN.md](RELEASE_PLAN.md) ("What is already done") and [LOG.md](LOG.md).
