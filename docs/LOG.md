# Work log

One entry per work session, newest first. Write what was done, which tool did it, the PR, and anything the next session must know. Keep entries to a few lines; details belong in PRs and reports.

## 2026-11-07 — Claude Code (preview.27 released)

- PR #42 merged into `main` (5881844) after green CI on Linux and macOS; released as preview.27 (run 42353801652). Assets: the app zip, `update-darwin-arm64.json` and its `.sig`.
- This update itself may still need the old background client restarted by hand (the copy-command button in the error chip): the blocking restart dialog arrives with this version and works from the next update on. Not yet checked on the Mac.

## 2026-11-07 — Claude Code (restart the background client from the window)

- Owner: after an update the window said «В фоне старая версия клиента, перезапустите его» and asked for a terminal. Now the client reports its version. While it is older than the window, a modal dialog blocks the window with «Перезапустить» or «Выйти» (the owner asked for a blocking dialog instead of a banner, since the window cannot work with an old client). It asks first if tasks are running (D-037).
- Checked: Go tests with fake clients (order of stop and start, kept pause, refused or lingering shutdown, lock wait, no downgrade, confirmation, one restart at a time). On Linux: an old client built from `main` was replaced from a preview.27 window by the button in headless Chromium; the PID changed and the client was not paused. In the browser the dialog stays open on Esc and covers the tabs, the confirmation step shows (with a stubbed answer), «Выйти» sends the quit message to the app and leaves the client running. Not checked on the Mac (the Swift quit handler is new).
- Owner: keep to the Claude app style, the buttons became too big. Controls are back to 32 px (compact 28), the dialog follows Claude's alerts (16 px padding, leaving choice on the left, main action on the right), and a destructive button has red text on a soft red fill. Measured in the browser: only 32 and 28 px controls.
- Owner: the text in buttons sat high. Measured by pixels: 9 px above the capitals, 13 below. Buttons, tabs, select fields and inputs now get a 4 px top padding (`--am-text-nudge`), which gives 11 and 11. Checked with the Linux font (Inter); the Mac font (SF) has the same kind of offset in the owner's screenshot, but it was not measured on the Mac.

## 2026-11-07 — Claude Code (window style rules)

- Owner: gaps between buttons differ between the sidebar and the agent cards; the style is scattered. The stylesheet was rewritten on one set of tokens (D-036, [UI_STYLE.md](UI_STYLE.md)); dead rules from the old chat screen were removed; agent cards and server sections share `.am-card`.
- Checked by screenshots of every tab, a dialog and a menu in both themes at 1100 and 600 px, and by measuring control heights (36 and 32 only). Not checked on the Mac.
- Owner follow-ups: the server note moved to a right column; the server is «Запустить»/«Остановить» (not «Включить»/«Выключить») in the window and in error texts; dropdown menus now look like the Claude app (4 px inset, 32 px options, background only on hover or keyboard focus, thin accent check on the chosen option). Checked in headless Chromium, both themes, mouse and keyboard.

## 2026-11-07 — Claude Code (third night: preview.25, server setup)

- Released preview.25 (PR #40: error chip, outdated-client message, task list fix).
- T-015 first part on `claude/t015-server-setup`: `internal/broker` (hosted broker, pinned TLS, one-time invitations, device keys, revoke), the control command `server`, `/api/server*`, the «Сервер» tab, `status` broker state. D-033–D-035, Q-013, [BROKER.md](BROKER.md).
- Audit by two independent reviewers; all findings fixed (connection cap, LAN peers, restart/lock race, shutdown order, settings rollback, joins, durable writes, rate limit, names, timeouts, and window state issues). T-030 recorded for an older timeout issue. PR #41, release preview.26.
- Verified on Linux with tests and headless Chromium; not with two computers, a real LAN or on the Mac.

## 2026-11-07 — Claude Code with the owner (T-027 closed)

- The owner tested preview.24 on the Mac and said everything works. T-027 moved to Done on that report. Next: Q-012, then T-028.

## 2026-11-07 — Claude Code (second night: preview.24)

- From the owner's T-027 feedback: stop button like a chat (square, Esc, D-031), model choice by the coordinator with guidance in its instructions and skill (D-030), window fixes listed in HANDOFF; the window's client now allows tasks and resumes itself (D-029); Claude workers accept edits (D-028).
- Found while testing: a relative `--root` failed with a SQLite error in the coordinator; it is now made absolute.
- Cleanup (D-032): removed code nothing calls (one was an unused helper that released a reservation by PID) and the `experiments/` folder; links now point to commit `41e0e6e`. Persistent paths kept: Q-012 asks the owner about T-028.
- Verified on Linux with fake CLIs and headless Chromium; CI on Linux and macOS. Not verified on the Mac or with real models.

## 2026-11-07 — Claude Code with the owner (first T-027 run, preview.23)

- After the update, the old client from preview.22 was gone and no client ran in the default data directory; `start --claude-stream --allow-tasks` and the window's "Продолжить" fixed it. The app lives in `~/Applications`, not `/Applications`.
- With real models on the Mac: the Claude-app coordinator started a Codex task; Codex created the file, the run was verified, and the coordinator got the report through `wait`. A Claude worker found the Messenger tools through ToolSearch (Q-010 answered: no change needed), asked the owner twice, and the same session resumed after each answer from the window and reported.
- Found: a Claude worker cannot edit without an allow rule (Q-011); the window showed "Работает" for an unlaunched task, two Claude lines in the computer card, a long title overflowing the task list, and answers needed Cmd+Enter. Fixed on `claude/t027-window-fixes`. Still open: code blocks in worker text are shown raw; "Команда" rows need their command in the title; duration should count from launch; the window says "client unavailable" for any error.

## 2026-11-07 — Claude Code (release for T-027)

- The owner asked for a build to test and chose to merge into `main` and release preview.23, so the installed app can update itself.
- macOS CI was red on PR #36. Two causes were in tests: `/tmp` is a link on macOS, and a reservation check ran a moment too early. One was a product bug in the persistent Claude path: the loop could claim a second stream before the first had taken the exclusive startup lease, so the second message became uncertain. Reproduced on Linux under CPU load; the loop now starts one stream process at a time.

## 2026-11-06/07 night — Claude Code (T-022, T-024–T-026, T-029)

- The owner asked for autonomous work overnight; PR #35 (T-009) was cancelled and left as is.
- T-022: replies and sends are held until the run is verified.
- T-024: task core (projects, folders, tasks, runs, worker tools, verification, events). T-025: coordinator MCP with `wait`. T-026: worker profile and trust check.
- Real Codex 0.155.1 and Claude Code 2.1.319 ran the worker path against local fake model APIs. Found that Claude 2.1.319 defers MCP tools behind ToolSearch (Q-010).
- T-029: the new window from the approved mockup, checked in headless Chromium. Q-009 (how the window allows task launches) and D-027 recorded.
- All on `claude/task-core-night`, [PR #36](https://github.com/kolotiloff/agent-messenger/pull/36). Next: T-027 on the Mac.

## 2026-11-06 — Claude Code with the owner (window direction)

- Agreed the future window with the owner on a mockup built from the real window files: an observation panel with a "Tasks" tab and coordinator status. Owner questions show as a yellow dot and inside the task's conversation, with an answer field only while the worker waits; the coordinator does not read that exchange. Recorded as D-024; added T-029.

## 2026-11-06 — Claude Code with the owner (Q-008 decided)

- The owner accepted the task model: D-021 (one task, one session; Claude-app coordinator with `wait`; clarifications resume the same task session; best-effort cleanup), D-022 (planning rules and parallelism live in each project's docs and the coordinator's judgement; no approval or worker limit in Messenger), D-023 (workers use their project's settings). Codex as coordinator deferred.
- D-004 partly, D-005 and D-016 superseded for new work; the first rule in `AGENTS.md` updated accordingly. Tasks T-024–T-028 added; T-009 superseded; T-021 and T-023 done.
- Next: T-022, then T-024.

## 2026-11-06 — Claude Code with the owner (T-023 on the Mac, Claude app)

- The owner ran the wait pilot in a local Claude Code-tab session. The coordinator waited 5 min in the foreground, woke by itself on each event, kept waiting after the owner wrote in between (the app moved the call to the background and it still woke the chat), lost nothing, and had no empty wake-ups. [Report](reports/2026-11-06-wait-pilot-claude-mac.md).
- Found: the Code tab did not load the project `.mcp.json`; `claude mcp add --scope local` in the test folder worked. README updated. Go was downloaded into `~/AgentMessengerTest/go-sdk` to build the probe.
- Next: Codex (app and CLI), then the owner decides Q-008.

## 2026-11-06 — Claude Code (T-023 wait pilot)

- Read the Codex review of RETHINK version 3 ([RETHINK_REVIEW_V3.md](RETHINK_REVIEW_V3.md)): better direction, pilot first. The owner agreed to start the pilot.
- Added `experiments/wait_probe`: a stdio MCP server with `wait` and `status`, a `send` command standing in for worker reports, a call log, progress notifications during long waits, and `setup.sh`, which prepares `~/AgentMessengerTest/pilot-coordinator` without touching global Claude or Codex settings.
- Verified on Linux without models (race tests, manual run). Codex config keys come from search results; the Codex docs were not reachable. Next: the owner runs the scenarios in the README on the Mac.

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

## 2026-11-06 — clickable window mockup

- Published a clickable mockup of the D-024 window (computers, task list with agent filter, task feed, answer field for an owner question). The owner approved it.
- Owner's note: the mockup is a layout reference only. T-029 builds on the program's existing styles and code, not on the mockup's code. Recorded in T-029 and D-024.

## 2026-11-06 — window mockup approved

- The owner approved the clickable window mockup after a series of changes: one working folder per computer (D-025) shown as a compact bar on the Agents tab with per-agent trust; sidebar agents with status dots and a "координатор" word; task rows in three lines (agent logo and name, title, coloured status with duration); agent filter as a dropdown; role tags next to names in the feed; no "·" separators; capitalised statuses; spacing from stylesheet tokens.
- The mockup is saved in [docs/mockups/2026-11-06-task-window.html](mockups/2026-11-06-task-window.html) so any agent can open it. It is a layout reference; T-029 builds on the program's own styles and code.

## 2026-11-06 — several projects (D-026)

- The owner wants to be able to run two games at once with two coordinator chats. Recorded as D-026: projects exist from the start (name, coordinator connection, folder per computer, tasks); agents are shared. The idea T-030 is folded into T-024, T-025 and T-029.
- The approved mockup now has a project switcher at the top of the sidebar ([docs/mockups/2026-11-06-task-window.html](mockups/2026-11-06-task-window.html)).
