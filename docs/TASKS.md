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
| — | Nothing in progress. Next: T-022, then T-024. | | |

## Next

Order of the task model (D-021), following the review's first slice ([RETHINK_REVIEW_V3.md](RETHINK_REVIEW_V3.md) section 5).

| ID | Task | Owner | Notes |
|---|---|---|---|
| T-022 | Publish a final reply to its recipient only after the run is verified (identity, completion, exit status), in the transaction that marks the attempt complete. Today `replyTx` routes it when it is stored. Applies to the one-shot, Claude stream and Codex paths; does not depend on Q-008. | agent | [RETHINK.md](RETHINK.md) 4.5, review R2 |
| T-024 | Task core on one computer: `tasks` table and lifecycle (start, report, ask → answer → exact resume of the same session, failed, interrupted, blocked, cancel); one message per run; durable launch intent; idempotent start/answer operations; provisional report published after verification; best-effort cleanup (D-021); persistent paths untouched. Per-computer project list (D-025): a task names a project, the worker runs in its folder, unknown names are refused. Fake CLIs, `messenger-only` profile. | agent | D-021 |
| T-025 | Coordinator MCP: `list_agents`, `start_task`, `wait` (replayable cursor, progress, cancellable, asynchronous), `tasks`, `answer`, `cancel`; idempotent operations, no worker limit (D-022); a Messenger skill that explains only the tools. Provider-neutral. | agent | D-021, D-022; T-023 probe |
| T-026 | Worker profile: launch with the project's own settings and normal tools (D-023), worker tools `read_task`/`report`/`ask`/`ask_owner`, profile-aware validators and prompt, bounded report size. | agent | D-023 |
| T-029 | Window for the task model (D-024): sidebar of computers with the coordinator marked; "Tasks" tab with the selected computer's tasks as a chat list (active, finished, filter by agent); each task shown as its worker session in one feed like the Claude app (messages, worker text, collapsible tool actions from the CLI event stream), owner questions as a yellow dot plus an answer field inside the task only while the worker waits, agent cards with role and task counts; a project list per computer with add/remove and a trust warning (D-025), and the project shown on each task; remove the general composer, the queue panel and the manual refresh button (tasks, statuses and the open feed update by themselves). Keep design and themes. The mockup shows layout and behaviour only: build with the existing `app.css` components and `app.js` structure, do not copy the mockup's code or add parallel styles. No "·" separators in UI text (the owner dislikes them): use commas, "на", labels or separate lines; replace the existing ones in `app.js` and the Mac shell too. | agent | after T-024, T-025 |
| T-027 | Mac acceptance with real models in `~/AgentMessengerTest/`: a Claude-app coordinator, one Claude and one Codex worker, a clarification round, two full tasks. | agent + owner | after T-024–T-026 |
| T-028 | Close or migrate existing persistent sessions and state, then retire the persistent Claude stream and Codex app-server owners. | agent | after T-027 |

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
| T-023 | Coordinator wait pilot ([experiments/wait_probe](../experiments/wait_probe/README.md)): passed in the Claude app on the Mac ([report](reports/2026-11-06-wait-pilot-claude-mac.md)). Codex as coordinator deferred by the owner. | branch `claude/vigilant-darwin-34d88b` |
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
