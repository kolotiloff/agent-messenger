# Work log

One entry per work session, newest first. Write what was done, which tool did it, the PR, and anything the next session must know. Keep entries to a few lines; details belong in PRs and reports.

## 2026-11-06 — Codex (T-021 version 3 review)

- Reviewed `RETHINK.md` at `7d02b62` against the unchanged product code and official Claude Code/Codex documentation. Saved findings and all seven answers in [RETHINK_REVIEW_V3.md](RETHINK_REVIEW_V3.md), including convenience, illustrative token costs and effectiveness.
- Recommend a bounded task pilot, server-enforced approved plans, idempotent operations and replayable events. Within-task ask/answer still needs an explicit resume policy; actual desktop wait behavior is an acceptance gate. Q-008 remains pending.
- Native Mac arm64 `scripts/check.sh` passed on identical product code with Go 1.27.3, including all race tests; an initial restricted run could not access test sockets/process observations. Documentation only; no real models, product changes or releases.

## 2026-11-06 — Claude Code (RETHINK version 3)

- With the owner, replaced the proposal with version 3: the coordinator is the owner's own Claude/Codex chat with Messenger as MCP and a skill; one task = one new worker session; the coordinator waits with a blocking `wait` tool, which spends no tokens while open.
- Checked the Claude Code docs for waking an idle chat: cross-session messages, `asyncRewake` hooks and channels (research preview, terminal CLI) exist; none is needed for the first version.
- Q-008 rewritten with four owner decisions. Next: Codex review of version 3. No code changed.

## 2026-11-06 — Claude Code (RETHINK version 2)

- Checked the Codex review against the code: R2 (reply routed before verification), R3 (one request per attempt) and R4 (strict tool-sequence validators) are confirmed.
- Rewrote [RETHINK.md](RETHINK.md) as version 2: weaker ownership guarantee named plainly, provisional reports, one message per run, profile-aware validators, migration before removal, owner-only budget reset. Section 9 maps each finding to its change.
- Q-008 now lists the three decisions for the owner. Added T-022 (publish reply after verification), which is useful whatever Q-008 decides. No code changed.

## 2026-11-06 — Codex (T-021 review)

- Reviewed `RETHINK.md` against `59d58eb` on `claude/vigilant-darwin-34d88b`; findings and all five section 9 answers are in [RETHINK_REVIEW.md](RETHINK_REVIEW.md).
- Recommendation: accept direction with changes. Main gaps: escaped-descendant ownership, reports routed before verification, batching, worker validators and migration of existing owners. Q-008 remains pending; existing safety rules are unchanged.
- Mac arm64 `scripts/check.sh` passed with Go 1.27.3, including all race tests. Documentation only; no product changes, real-model runs or release.

## 2026-11-06 — Claude Code (rethink)

- Discussed the concept with the owner. Wrote [RETHINK.md](RETHINK.md): one-shot runs that resume a saved conversation instead of persistent processes, real tools for workers, questions to the owner. No code changed.
- Found while reading the code: the one-shot path already exists but keeps every session reserved after its first run; and every launch path strips the agent to `read_current`/`reply`, so no path can yet run a worker that edits files.
- Added Q-008 and T-021. Next: Codex review, then the owner's decision.

## 2026-11-06 — Claude Code (T-004 on the Mac)

- Released preview.21 from CI. The owner updated, logged in to a separate Codex profile and created a Codex session from the window. Real Codex answered both messages, with both turns verified.
- Found and fixed: the Codex setup from the window did not resume the paused queue. [PR #33](https://github.com/kolotiloff/agent-messenger/pull/33).

## 2026-11-06 — Claude Code (releases from CI, T-006)

- The owner added the `SPARKLE_ED_PRIVATE_KEY` secret. Merged [PR #31](https://github.com/kolotiloff/agent-messenger/pull/31) and started the first CI release, preview.20.
- T-006: a Codex session can be created from the window in one confirmed step. [PR #32](https://github.com/kolotiloff/agent-messenger/pull/32).

## 2026-11-06 — Claude Code (incident: app would not open after reboot)

- preview.19 window failed with "environment must be an existing directory". The reservations pointed to `/private/tmp/am-claude-nee2aviw`, which macOS had deleted on reboot. The owner verified the evidence and moved the registry aside; the window works again with empty history.
- Added `recover-missing-root`, the window's refusal of temporary data directories, and a rule against `/tmp` data directories in acceptance runs. Fixed a CI race in the test helper that waited for the control socket. All of this is in [PR #31](https://github.com/kolotiloff/agent-messenger/pull/31).

## 2026-11-06 — Claude Code (T-008)

- Persistent Claude streams now run in their own goroutines; the main loop no longer blocks on one. `current` became a job map, `stop-job` and shutdown stop every stream, and the limit is 4 concurrent streams. Test: two sessions, two processes, two turns each. [PR #31](https://github.com/kolotiloff/agent-messenger/pull/31).

## 2026-11-06 — Claude Code (T-003)

- Found on real Codex 0.155.1 that `config/read` returns config keys in a different order on every call. Fingerprints are now taken over canonical JSON. Without this, every re-check would have refused the turn.
- The config is re-checked before every turn; a mismatch returns the unsent message to the queue. Added a schema version with refusal of newer databases. [PR #30](https://github.com/kolotiloff/agent-messenger/pull/30).

## 2026-11-06 — Claude Code (T-005)

- Merged [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29) (T-002, T-007) after green CI.
- Reply routing between local agents with `MaxAutoReplyHops` = 3. The end-to-end test with fake Claude and fake Codex stops at the limit, with both owners still alive. [PR #30](https://github.com/kolotiloff/agent-messenger/pull/30). Several Claude streams were split off as T-008.

## 2026-11-06 — Claude Code (T-007)

- `recover-codex`: two-phase retirement of an uncertain Codex setup after a proven reboot (same rule as `recover-stream`). Tests cover refusal on the same boot, with a live client, with a wrong confirmation, after a finished recovery, and resuming a half-done recovery. [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29).

## 2026-11-06 — Claude Code (T-002)

- Codex message delivery: owner controller, `codex_turns`, `RunTurn` with an early-event buffer and server-request handling, bound tools through SQLite. Tests with a fake app-server cover two turns, pause, and six failure modes. No model was called. [PR #29](https://github.com/kolotiloff/agent-messenger/pull/29).
- Codex sessions are stored without a CLI path, so older clients cannot launch them.
- The owner reported a Mac freeze and reboot; no Mac action is needed for this work.

## 2026-11-06 — Claude Code (T-001)

- Installed Codex 0.155.1 from npm in the cloud session. Generated the app-server schema. Confirmed on the real binary that `thread/start` declares dynamic tools, needs no login and leaves no rollout for an empty thread.
- Implemented Codex session setup: an RPC client, the `codex_setups` table and `CreateCodexSession`, with tests. Fixed the windows test build and a socket-path limit in the test harness. `check.sh` and `check-builds.sh` pass. [PR #28](https://github.com/kolotiloff/agent-messenger/pull/28).
- Next: T-002.

## 2026-11-06 — Claude Code

- Took over from Codex. Read the project; `scripts/check.sh` passes on Linux.
- Translated all documentation to English; added `AGENTS.md` (shared rules) and `CLAUDE.md` (imports it). [PR #26](https://github.com/kolotiloff/agent-messenger/pull/26), merged.
- Closed draft PRs #2–#18 and gave the Russian ones English titles; their content is in `main`.
- Added the working documents: `docs/README.md`, `HANDOFF.md` (was `HANDOFF_CLAUDE.md`), `TASKS.md`, `DECISIONS.md`, `QUESTIONS.md`, this log.
- Next: T-001.

## 2026-11-05 – 2026-11-06 — Codex (summary)

Built the product from the first Go client to preview.19 in about 50 commits:

- persistent Claude with pause/resume, exact resume and recovery after a reboot;
- the background client;
- diagnostics (`doctor`, `check-cli`);
- the overview and timeline read APIs;
- the local UI with sending and a durable outbox;
- Mac preview packaging and install;
- CI on Mac and Linux;
- the native Mac shell and Sparkle updates from GitHub Releases;
- session setup and scoped launch from the window;
- Codex app-server preflight and turn guards.

Reports for each step are in [reports/](reports/). The handoff is in [HANDOFF.md](HANDOFF.md).
