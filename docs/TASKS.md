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

| T-015 | First part done on `claude/t015-server-setup`: broker hosted by the client, invitations, joining, check, devices and revoke, the «Сервер» tab ([BROKER.md](BROKER.md), D-033–D-035). Left: tasks between computers, VPS package and wizard, Keychain (Q-013), Windows (Q-006). | agent + owner | `claude/t015-server-setup` |

## Next

Order of the task model (D-021), following the review's first slice ([RETHINK_REVIEW_V3.md](RETHINK_REVIEW_V3.md) section 5).

| ID | Task | Owner | Notes |
|---|---|---|---|
| T-028 | Close or migrate existing persistent sessions and state, then retire the persistent Claude stream and Codex app-server owners. | agent | blocked: Q-012 |

## Backlog

| ID | Task | Owner | Notes |
|---|---|---|---|
| T-010 | Safely terminate a CLI and release its session within the same Mac boot. This is an open research problem: release by timer, empty process group or one exited PID is not allowed. | agent | see [RELEASE_PLAN.md](RELEASE_PLAN.md) |
| T-011 | Switch the version of a running background client, plus full rollback. | agent | see [UPDATES.md](UPDATES.md) |
| T-012 | Persistent Cursor adapter. | agent | blocked: Q-004 |
| T-013 | Developer ID signing and notarization of the Mac app. | owner | blocked: Q-003 |
| T-014 | Intel Mac build and acceptance. | agent + owner | |
| T-016 | Windows and Linux runtime acceptance and packages. Stage 6. | agent + owner | Q-006 |
| T-017 | Coordinator role handover. Stage 6. | agent | |
| T-019 | Decide and implement the UI language. | owner → agent | blocked: Q-001 |
| T-030 | `/api/setup-codex` allows 100 s, but the window HTTP server cuts every reply at 25 s (`WriteTimeout`), so a slow successful setup shows as an error. Set a per-request write deadline for it. Persistent Codex path: moot if T-028 removes it. | agent | found in the T-015 audit |

## Done

| ID | Task | PR |
|---|---|---|
| T-033 | A resumed run's `read_task` returns the question and answer without the task text (the conversation already has it) and a note; `include_task: true` returns the text. | branch `claude/t033-resume-brief` |
| T-034 | Failure reasons (usage limit with reset time, rate limit, overload, sign-in, other) on the task, in the window and the coordinator's `failed` event; nothing is retried. Limit formats from the CLIs' docs, tested with fake CLIs only. | [PR #45](https://github.com/kolotiloff/agent-messenger/pull/45) |
| T-032 | `tasks`: open tasks by default, limit, no folder per row, `version` / `if_version` → `unchanged`. | [PR #45](https://github.com/kolotiloff/agent-messenger/pull/45) |
| T-031 | `wait`: at most 20 events and about 24,000 characters, `more: true`, cursor at the last returned event. | [PR #45](https://github.com/kolotiloff/agent-messenger/pull/45) |
| T-018 | Old branches already merged into `main` deleted by the owner on GitHub (2026-11-07). Branches with unmerged work were kept. | — |
| T-027 | Mac acceptance with real models. preview.23: Codex and Claude tasks, owner questions, reports through `wait` (with the owner). preview.24 with the fixes (D-028–D-032): owner tested it on the Mac and reported that everything works (2026-11-07); no separate written report of each case. | [PR #37](https://github.com/kolotiloff/agent-messenger/pull/37), [PR #38](https://github.com/kolotiloff/agent-messenger/pull/38) |
| T-029 | Task window: project switcher, computers with agents, three-line task list, task feed, owner answers, agent cards with trust. Mac app not yet checked. | [PR #36](https://github.com/kolotiloff/agent-messenger/pull/36) |
| T-026 | Worker profile, trust check, runs on real CLIs with fake models ([report](reports/2026-11-06-task-workers-linux.md)). | [PR #36](https://github.com/kolotiloff/agent-messenger/pull/36) |
| T-025 | Coordinator MCP with `wait` (`messenger coordinator --project`), skill. | [PR #36](https://github.com/kolotiloff/agent-messenger/pull/36) |
| T-024 | Task core on one computer: projects and folders, tasks, runs, worker tools, verification, events ([TASK_MODEL.md](TASK_MODEL.md)). | [PR #36](https://github.com/kolotiloff/agent-messenger/pull/36) |
| T-022 | Publish a reply only after verification (held replies and sends). | [PR #36](https://github.com/kolotiloff/agent-messenger/pull/36) |
| T-023 | Coordinator wait pilot ([experiments/wait_probe](https://github.com/kolotiloff/agent-messenger/tree/41e0e6e7e3f20275d480f6b064a881e7b5f4cf9e/experiments/wait_probe/README.md)): passed in the Claude app on the Mac ([report](reports/2026-11-06-wait-pilot-claude-mac.md)). Codex as coordinator deferred by the owner. | branch `claude/vigilant-darwin-34d88b` |
| T-021 | Rethink the product: [RETHINK.md](RETHINK.md) v1–v3, two Codex reviews, owner decision Q-008 → D-021–D-023. | branch `claude/vigilant-darwin-34d88b` |
| T-009 | Superseded by D-021: persistent Codex threads are no longer resumed for new work. | — |
| T-004 | First real Codex run on the Mac from the window: two messages answered and verified in one app-server ([report](reports/2026-11-06-codex-window-mac.md)). Exact resume moved to T-009. | [PR #33](https://github.com/kolotiloff/agent-messenger/pull/33) |
| T-006 | Codex in the window: setup dialog creates a Codex session and grants model calls for it only (`/api/setup-codex` → `setup-codex`). | [PR #32](https://github.com/kolotiloff/agent-messenger/pull/32) |
| T-020 | After the reboot incident: `recover-missing-root` (proven reboot, missing directory), the window refuses temporary data directories, and an acceptance-run rule in AGENTS.md. | [PR #31](https://github.com/kolotiloff/agent-messenger/pull/31) |
| T-008 | Several persistent Claude sessions at once: one controller per session, up to 4 at a time; `stop-job` and shutdown stop all. | [PR #31](https://github.com/kolotiloff/agent-messenger/pull/31) |
| T-003 | Codex config re-check before every turn (canonical fingerprint; unsent message returned to the queue on a change) and a recorded schema version that newer databases are refused by. | [PR #30](https://github.com/kolotiloff/agent-messenger/pull/30) |
| T-005 | Replies between agents: a reply to another local alias is queued for it in the same transaction, with a limit of 3 automatic replies per chain; Claude stream and Codex owner exchange messages side by side (fake CLIs). Several Claude streams moved to T-008. | [PR #30](https://github.com/kolotiloff/agent-messenger/pull/30) |
| T-007 | `recover-codex`: retire an uncertain Codex owner after a proven reboot; session closed without retry, history kept. | [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29) |
| T-002 | Deliver queued messages to the Codex owner: one turn per message, send intent before `turn/start`, turn ID bound to the attempt, bound tools through SQLite, success only on verified completion; verified on Linux with a fake app-server. | [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29) |
| T-001 | Create and persist a Codex session before the first message (`create-codex-session`); verified on Linux with a fake app-server and real Codex 0.155.1, no model. | [PR #28](https://github.com/kolotiloff/agent-messenger/pull/28) |
| T-100 | Translate documentation to English; shared agent rules in `AGENTS.md`; close stale draft PRs #2–#18. | [#26](https://github.com/kolotiloff/agent-messenger/pull/26) |
| T-101 | Fail-closed Codex app-server preflight and turn guards (components only, not wired into `serve`). | [#25](https://github.com/kolotiloff/agent-messenger/pull/25) |
| T-102 | preview.19: create and explicitly launch a separate test Claude session from the window; accepted on Mac. | see [report](reports/2026-11-06-window-launch-mac.md) |

Earlier work is summarized in [RELEASE_PLAN.md](RELEASE_PLAN.md) ("What is already done") and [LOG.md](LOG.md).
