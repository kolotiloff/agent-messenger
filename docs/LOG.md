# Work log

One entry per work session, newest first. Write what was done, which tool did it, the PR, and anything the next session must know. Keep entries to a few lines; details belong in PRs and reports.

## 2026-11-08 — Claude Code (T-043 models per agent)

- PR #63 merged (722c51f); preview.36 release started. The owner's coordinator asked for exact model names for Codex and Cursor: it may not guess them. Now the owner lists them on the agent card, `list_agents` returns them, and Codex and Cursor refuse an unlisted name (D-058). `list_agents` also uses the computer's own name now.
- The owner also wants each agent's remaining limit known to the coordinator, as a separate field (T-044, not started).
- Verified here: `check.sh`, Windows vet, new core, broker, window-route tests. Not verified: real CLIs with listed names; a mixed network of preview.36 and newer computers.

## 2026-11-08 — Claude Code (T-042 owner's two-computer test, follow-ups)

- The owner's test passed: the Claude coordinator on the Mac gave Codex on Windows an audit and got the report. Follow-ups done: tool actions in the feed are grouped into one collapsible line; the computer has its own name on the «Сервер» tab, the host included; tasks of other computers are accepted by default (D-056); Codex trust is read from all `config.toml` forms; «Войти в аккаунт» and «Установить …» buttons (D-057); on Windows no inner frame and a title bar in the theme's colour.
- Verified here: `check.sh`, `GOOS=windows go vet`, new tests (trust forms, names, login commands per OS, routes), headless Chromium screenshots of the grouping and of the Windows layout. Not verified: real Windows (title bar, terminal login, install page), the real Codex config, the Codex and Cursor install page addresses (not reachable from here).

## 2026-11-08 — Claude Code (T-042 progress between computers)

- The owner ran preview.35 on a Mac and a Windows computer: Windows hosted the server, the Mac joined and gave Codex on Windows a task. It was first refused because «Задачи других компьютеров» was off (Q-014, by design); the refusal showed as an English note in the middle of the feed. After turning it on, Codex worked on Windows, but the Mac showed nothing until the end.
- Now the Mac shows «Принято на компьютере …» and the worker's progress lines (D-055), and known refusals are explained in Russian. Tested with core tests (two stores, messages carried by hand); the window change is checked for syntax only. Not tested on two real computers.

## 2026-11-08 — Claude Code (T-041 agents on Windows)

- The owner ran preview.34 on Windows: the window opened in the browser (as designed, D-045). Claude Code was found but «Проверить версию и вход» showed nothing: the check ran only on macOS and asked for login only for one exact version. Fixed (D-051). Codex was not found: newer npm packages keep `codex.exe` in `vendor/<target>/bin`, not `vendor/<target>/codex`; both are searched now. Cursor is `%LOCALAPPDATA%\cursor-agent\agent.cmd`, a script that runs `versions\<newest>\node.exe index.js` through PowerShell; Messenger now starts that pair directly (D-052). Verified with tests on Linux; not on a real Windows computer.
- The owner did not want the browser tab and console on Windows: the window is now the program's own (WebView2) on Windows and an app-mode window on Linux (D-054). Linux was run here with a fake browser (window file, page answered, cleanup on close); WebView2 is compiled only, not run.
- The owner asked for a button to choose paths instead of typing them: «Выбрать…» opens the system's dialog (D-053). Tested with a fake dialog; the real dialogs (Windows, macOS, Linux) are not checked here.
- PR #62 merged and preview.35 released (48b883e). CI found two problems on the way: GitHub's download server returned 504 for Sparkle (the download now retries, up to 15 min on `claude/t041-sparkle-retry`), and a supervisor test caught a race: `Wait` closed `StdoutPipe` before the last lines were read, so a finished task could be marked unverified. Now a plain pipe is read to the end, and output still open 5 s after the exit is not trusted.

## 2026-11-07 — Claude Code (owner's Mac results)

- The owner's Mac: the Claude coordinator, connected by the window itself in preview.34 (T-039, no Terminal), gave «Посчитать 5 + 5» to the real Cursor CLI; Cursor read the task, reported «5 + 5 = 10» and its model, and the session closed. Earlier Codex and Claude Code tasks in the same project also finished. This is the first check of T-039 with the real Claude Code.
- Noticed: Cursor's progress lines in English («I'll start by reading…») are shown in the report. Possible follow-up: show only the final report.

## 2026-11-07 — Claude Code (T-040 faster release)

- The owner found a release too slow (about 15 minutes). The Mac checks ran only after the Windows and Linux job. Branch `claude/t040-faster-release`: a separate «Mac checks» job runs next to it, and «Build, sign and publish» waits for both. The checks themselves are kept. Expected: about 10 minutes. Not verified until the next release run.
- PR #59 (T-039) merged; preview.34 released (run 42396560953, 21 assets). Checked here: the Linux x64 download, its SHA-256 and `version`.

## 2026-11-07 — Claude Code (T-039 coordinator without Terminal)

- The owner asked not to type the `claude mcp add` command. Branch `claude/t039-connect-coordinator`: the window connects the coordinator by itself after a project is created and on «Подключить к Claude» (D-050). Every worker run is marked so the coordinator server gives it no tools.
- Verified on Linux with a fake Claude CLI: unit tests (connect, already connected, another entry kept, failure shown once and not retried, no Claude agent), and an end-to-end run of the built program in a browser: a new project was connected, the entry in `~/.claude.json` started the coordinator, which answered `list_agents`; an error from Claude was shown with its message. Not verified: the real Claude Code, Windows, Mac.

## 2026-11-07 — Claude Code (T-012 Cursor names)

- The owner saw «cursor» in lower case under a computer in the sidebar. Fixed: the sidebar's short names, the task list's agent filter (it had no Cursor, so the Cursor card's «Задачи» link selected nothing), the `agent-set` help, and the `check-cli` line, which now prints «Claude Code», «Codex», «Cursor». A new test requires a proper name for every task agent in each of the window's name lists and in the filter.

## 2026-11-07 — Claude Code (release preview.33, window grant fix)

- PR #55 (Cursor) merged into PR #54; PR #54 squash-merged to `main` (56ee506); PR #53 closed (its commit was already in #54).
- The first preview.33 release run (42386214790) stopped in `check.sh`: `TestWindowAuthorizationIsScopedIdempotentAndNotRestored` saw a model start for a refused session. Cause: the window's grant (Claude test session and Codex setup) opened model calls before the store refused the alias, and withdrew them after; the 30 ms loop could claim the refused session's queued message in between. Now the store is checked first (`CheckAuthorizeSetup`, `AliasInUse`) and only then the grant is made. A new test repeats refused grants with the loop running and failed every time before the fix.
- PR #56 merged (3ae2058); preview.33 released (run 42388079985, 21 assets). Checked here: downloads via the README links, SHA-256, `version`, and the Linux amd64 update signatures against the app's key.

## 2026-11-07 — Claude Code (T-012 Cursor tasks)

- The owner said there is no rule against Cursor. Branch `claude/t012-cursor-tasks`: Cursor as a task worker (D-049), tested with a fake Cursor CLI only; no real Cursor or model calls.

## 2026-11-07 — Claude Code (D-047 downloads, T-038 updates on Windows and Linux)

- Branch `claude/t016-download-buttons`: plain downloads behind README buttons (D-047), and signed in-window updates on Windows and Linux (D-048): `internal/selfupdate`, `/api/self-update`, the window's button, relaunch, signing in the release (`scripts/sign-other-updates.py`).
- Verified on Linux here with a test key (preview.900 → preview.901 through the window); Windows-only replace tests run on the Windows CI runner. No real model calls; Cursor not run.

## 2026-11-07 — Claude Code (T-016 Windows and Linux)

- Branch `claude/t016-windows-linux`, PR #52: Windows platform layer (job objects, LockFileEx, process snapshot, boot identity, background start), `.exe`-only agents, `messenger open` for a browser window, archives for Windows and Linux in CI and in the release. D-043 to D-046; Q-014 settled (off by default).
- Windows CI found that a job's name disappears when its last handle closes; the client now keeps the handle for the run and treats an unreachable job as not proven. SQLite path on Windows fixed (drive letter).
- Windows CI also found that a rename over `active-runs.json` fails while a reader has it open; the replace and the read now retry for up to 2 s (a refused rename changes nothing).
- PR #52 merged (squash, 11d938c) after two green CI runs on 1ce5622; preview.32 released (run 42380342750).
- No real model calls; Cursor not run; no real Windows desktop used.

## 2026-11-07 — Claude Code (T-037 tasks between computers)

- PR #49 merged, preview.30 released (run 42370126728). PR #50 merged after green CI on Linux and macOS, preview.31 released from 019f30a (run 42371886488).
- Branch `claude/t037-network-tasks`: broker mailbox, remote and mirror tasks in the store (schema 3), the client's 3-second exchange, `list_agents`/`start_task` across computers, the accept switch on «Сервер» (off by default, Q-014). D-041, D-042.
- No real model calls; Cursor not run; Mac app not run. Two clients on one Linux machine exchanged a task through the window API and the coordinator.

## 2026-11-07 — Claude Code (T-015 «Сервер» tab polish)

- Branch `claude/t015-status-style`: one status style, one-row server controls with locked fields, tooltips instead of hint lines, joined devices in the sidebar with a right-click «Отозвать», icon copy in the invitation, quiet update block, font-measured text centring. The owner approved it on a live preview; released as preview.29.
- No real model calls; Mac app not run.

## 2026-11-07 — Claude Code (T-035 computers and agents in the sidebar)

- preview.29 released from df7ade8 (PR #48). Owner asked to continue without them.
- T-035: presence reports (`POST /v1/presence`), the roster in the server status, sidebar cards for other computers. VPS optional (D-040). No real model calls; a helper program played the second computer.

## 2026-11-07 — Claude Code (preview.28 released)

- PR #46 (T-033) merged after green CI on Linux and macOS; released as preview.28 from 98d346e (run 42360015163). Assets: the app zip, `update-darwin-arm64.json` and its `.sig`. Not yet checked on the Mac.

## 2026-11-07 — Claude Code (T-031–T-034)

- PR #45 merged: T-034 failure reasons and reset time, T-031 bounded `wait`, T-032 lean `tasks` (`if_version` instead of `if_cursor`, because starting or cancelling a task changes the list without an event), window tweaks.
- T-033 on `claude/t033-resume-brief`: resumed `read_task` without the task text. Fake-CLI resume test checks both the short and the `include_task` answer.
- No real model calls; Mac app not run.

## 2026-11-07 — Claude Code (no references to other products)

- At the owner's request the docs no longer name or link other products: the earlier comparison document was removed, its accepted directions moved to RELEASE_PLAN.md "Planned directions" without sources. Keep it that way: no product names, links or comparisons in code, docs, commits or PRs.

## 2026-11-07 — Claude Code (token and reliability review)

- Owner asked to change only what saves tokens, simplifies work or adds reliability. Written up in [TOKEN_REVIEW.md](TOKEN_REVIEW.md), D-038; new tasks T-031–T-034 in Next.
- Measured our fixed prompt cost (~1.2k tokens for the coordinator, ~130 for a worker); the cost is in `wait`, `tasks` and resumed `read_task` answers. No code changed.

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
