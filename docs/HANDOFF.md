# Handoff: current state

This file is the shared memory between tools (Claude Code, Cursor, Codex). Whoever stops working updates the **Current state** section; whoever starts reads it first. Tasks live in [TASKS.md](TASKS.md), decisions in [DECISIONS.md](DECISIONS.md), open questions in [QUESTIONS.md](QUESTIONS.md), the work history in [LOG.md](LOG.md).

## Owner's standing instruction for the night of 2026-11-07

The owner asked the agent to finish the program without them ("доделай бе меня полностью"), update the documentation on GitHub and clean out old code. Same rules as before: fake CLIs only, no real models, `active-runs.json` untouched, nothing retried on an unclear result, questions go to `QUESTIONS.md` with the most cautious option chosen. Removing the persistent paths (T-028) still needs the owner's explicit consent (Q-012).

## Current state

Updated 2026-11-09 (day) by Claude Code.

- T-077 (D-091): automatic restart of the old client after an update. All collected changes go to `main` and a preview release. T-078 (remote update) waits for Q-025.
- On `claude/ui-tweaks`, not released: T-083 running tasks on another computer's agent cards, T-084 reply box height. T-085 write to a working worker (D-096). The security review's four reports are in; fixes of clear bugs follow, design choices go to QUESTIONS.md.
- preview.55 released (PR #86 merged) with: T-079 start screen (D-092), T-080 limit window and reset on the cards, the agent's raw limit fields in `list_agents` (D-093), T-081 limit warning in `start_task` (D-094), T-082 no system title bar on Mac and Linux (D-095; Mac: check dragging and double click by the top). After one real worker task of each agent, read `limit.reported` to learn whether a percent can be shown (Q-015).

- **Collected, not released** on `claude/ui-tweaks` (owner: collect changes, release later): «Пока пусто» alignment in statistics; T-074 coordinator catches up after an interrupted wait, wait 120 s by default (D-088); T-075 workers commit or ask and stay open (D-089). Language and theme lists of the same width; T-076 statistics of the whole network, filtered by the sidebar computer, another computer's agents view only, usage sent only when it changes (D-090). Not verified between real computers yet: check it on the Mac and ПК-1 after the release.

Updated 2026-11-09 (end of night) by Claude Code.

- **Released tonight:** preview.50 (T-057 log of another computer), preview.51 (T-071 last action and tokens per task, #81), preview.52 (T-070 readiness in `list_agents`, #82), preview.53 (T-072 resource locks, #83). T-073 decided without code (D-087). T-049 is finished except item 6 (Q-024).
- Backlog tidied: T-030 was already fixed with T-029 (per-route write deadline); T-019 is done by T-059 (Q-001 answered).
- **Owner's tests (not done here):** with two computers, ask the coordinator for `list_agents` (accepts tasks, project folder), start two tasks with the same `lock` on one computer and watch the second wait, and look at `tasks` for the last action and tokens. Everything was checked only with fake CLIs.
- **Waiting on the owner:** Q-024 (T-049 item 6), Q-023, Q-022, Q-018 default, Q-012 (T-028).
- **Next:** nothing in the owner's order is left. Open backlog items need the owner (T-013 signing, T-014 Intel Mac, T-028 consent) or are optional (T-017, T-036).

Updated 2026-11-08 (night, T-057) by Claude Code.

- **Released:** preview.50 (T-057 remote log), preview.51 (T-071 last action and tokens, #81), preview.52 (T-070 readiness, #82). T-072 resource locks goes out as preview.53.
- preview.48 is out (T-052 short invitation). **T-053** Windows installation merged (#79) after its CI step found and we fixed a folder that stayed on «Удалить»; goes out as preview.49.
- **T-057** log of another computer on `claude/t057-remote-log` (D-083): coordinator tool `get_log`, switch «Отдавать журнал по сети» on «Сервер», off by default. Owner's test: turn it on on one computer, ask the coordinator on another to read that computer's log.
- **T-049** split into T-070 (readiness in `list_agents`), T-071 (last action and tokens in `tasks`, done on `claude/t071-task-activity`, D-084), T-072 (resource locks), T-073 (results off the worker's computer) and Q-024 (item 6, unclear).
- **T-070** on `claude/t070-readiness` (D-085): `list_agents` shows `accepts_tasks` and `has_project` for another computer.
- **T-072** on `claude/t072-resource-locks` (D-086): `start_task` takes `lock`; the later task waits to start.
- **T-073** decided without code (D-087): reports already come back whole, files go through the project's version control.
- **Waiting on the owner:** Q-024 (T-049 item 6), Q-023, Q-018 default.

Updated 2026-11-08 (night, T-053) by Claude Code. The owner is asleep and asked to keep working alone, deciding by simplicity.

- **PR #73** (`claude/t061-agent-cards`): T-061 agent cards with the on/off switch, T-062 Windows window without the title bar; goes out as preview.43.
- **T-068** token statistics on `claude/t068-token-stats` (D-077, PR #74), goes out as preview.44.
- **T-069** Linux window on `claude/t069-linux-window` (D-078, built on T-068), goes out as preview.45. Owner's test on the Linux laptop: the window, its title bar in light and dark, the dock icon.
- **T-059** window language on `claude/t059-language` (D-079, built on T-069), goes out as preview.46.
- **T-047** shared task list on `claude/t047-shared-tasks` (D-080, built on T-059), goes out as preview.47. Owner's test: two computers in one network, tasks of the same project on both; each shows the other's under «На компьютере «…»» within about 15 seconds.
- **T-052** short invitation on `claude/t052-short-invite` (D-081, built on T-047), goes out as preview.48. Owner's test: invite on one computer, type the 6 digits on another in the same network. Windows may ask to allow Messenger in the firewall (UDP 47100); if the host is not found, the long code still works.
- **T-053** Windows installation on `claude/t053-windows-install` (D-082, built on T-052), goes out as preview.49. Owner's test on Windows: double-click the download; Agent Messenger appears in the Start menu and in «Приложения»; «Удалить» removes it and keeps `%LOCALAPPDATA%\AgentMessenger`.
- **Next, in the owner's order:** T-057 remote log, T-049.

Updated 2026-11-08 (before preview.42) by Claude Code.

- **PR #72** (`claude/t058-task-cost`) goes out as **preview.42** (owner asked to release everything done today): T-058 usage in statuses, T-060 blue dot only while running, T-063 resizable panes and empty-click, T-064 «Проект» tab with auto-save, T-065 project rename (shown name only, D-072, Q-019), T-066 update icon and «v0.41» (D-073, Q-020), T-067 Server tab variant Б, «Любые команды» text. All window changes were checked in Chromium with stubbed APIs only, not in the native windows.
- **Approved, not built yet:** T-061 agent cards (D-070), the settings dialog and the per-card switch; T-062 Windows window without the system title bar.
- **Owner's next asks:** Linux as a real program window (Q-016, now requested: WebKitGTK through cgo); status words for the server connection («Подключено / Нет связи / Не подключено» proposed, not answered); dollars in task statuses (not answered).

Updated 2026-11-08 (T-058) by Claude Code.

- **T-058** on `claude/t058-task-cost`: each task in the list shows what it spent (D-068). preview.41 carries T-056.
- Owner declined, no task: per-task worktrees, trigger automations, a file browser.

Updated 2026-11-08 (T-056, after preview.40) by Claude Code.

- Released today: preview.38 (#68), preview.39 (#69: T-054 full access, T-055 remove a project), preview.40 (#70: removal popup, «Настроить проект…», folder bar removed). T-050 and T-051 are on `main` (#66, #67).
- **T-056** on `claude/t056-access-all`: one «Любые команды» switch per computer, answers and follow-ups use the current setting, «Войти»/«Выйти» on cards (D-067). Next: PR, CI, release preview.41.
- **Waiting on the owner:** what the Linux update button does (it does not update); whether the coordinator should not block in `wait` for 10 minutes; Q-017 (invitation code).
- **Next owner requests:** a native Linux window (Q-016, WebKitGTK needs cgo); T-052 6-digit invitation; T-053 Windows installer; T-047 shared task list; T-049 coordinator wishlist.

Updated 2026-11-08 (T-048, before preview.37) by Claude Code.

- **PR #65** (`claude/t044-agent-limits`, same commits as `claude/t046-ui-feedback`): T-044 limits, T-045 icons and the Windows restart fix, T-046 agent cards, T-048 computer rename (D-061). T-043 (models) is already on `main` (#64). Next: CI, merge, release **preview.37**; every computer must update, the Mac (server) first: an older server refuses reports with models or limits and ignores renames.
- **Owner's requests not done yet, in order:** T-047 shared task list across computers (7-day retention, read-only on other computers); T-049 the coordinator's wishlist (computer readiness in `list_agents`, progress and last output, result files, resource locks such as the Unity editor, tokens per task); Q-016 a Linux window without Chrome/Chromium (today the browser opens when neither is installed).
- **Owner's Linux laptop:** a Codex worker failed because `~/.local/bin/codex-code-mode-host` is missing next to `codex`, i.e. Codex is installed incompletely; the owner was advised to reinstall Codex the official way. Not a Messenger fault; not verified here (no Codex in this container).
- **Answered, no change:** switching between computers like tabs existed only in the approved mockup, never in the program.

Updated 2026-11-08 (T-043) by Claude Code.

- **preview.36** released from `main` 722c51f (PR #63, T-041/T-042); check the assets.
- **T-043 on `claude/t043-agent-models`:** model names per agent, owner-listed, for the coordinator (D-058). Verified with tests only.
- **T-044 on `claude/t044-agent-limits` (on top of T-043):** each agent's last reported limit as a `limit` field (D-059). Real CLI fields not checked (Q-015).
- **T-045 on `claude/t045-linux-icon` (on top of T-044):** icons on Windows and Linux; the Windows window closes and reopens after «Обновиться».
- **T-046 on `claude/t046-ui-feedback` (on top of T-045):** agent cards check by themselves, simpler buttons, dialog actions on the right (D-060); Q-016 about the Linux window.
- **Next:** merge T-043 to T-046, release preview.37; the owner tests models and limits with real agents.

Updated 2026-11-08 (T-042) by Claude Code.

- **T-042 on `claude/t041-sparkle-retry` (PR #63):** after the owner's Mac → Windows test: progress and «Принято» on the starting computer (D-055), grouped tool actions, a computer name for every computer (host included), tasks of other computers accepted by default (D-056), Codex trust read from all `config.toml` forms, «Войти в аккаунт» / «Установить …» buttons, a frameless Windows window with a themed title bar (D-057).
- **Verified here:** `check.sh`, Windows vet, tests with fakes, headless Chromium screenshots. **Not verified:** real Windows, the real two-computer relay, the Codex/Cursor install page addresses.
- **Next:** CI, merge, preview.36; the owner tests it. Open idea from the owner: show every agent's usage limits (not started; see LOG).

Previous state:

Updated 2026-11-08 (preview.35) by Claude Code.

- **preview.35 released** from `main` 48b883e (PR #62, T-041; run 42470255420, the Mac build re-run once after GitHub's download server returned 504), 21 assets. Own window on Windows (WebView2) and Linux (app-mode browser window), the agent check on every OS, Codex and Cursor found on Windows, «Выбрать…» buttons (D-051–D-054). Also fixed: a worker's last output lines could be lost when it exited, so a finished task was marked unverified.
- **Checked here:** the Linux x64 program from the release matches the SHA-256 list and reports preview.35 at 48b883e.
- **Not checked:** anything on a real Windows computer or a Linux desktop (window, console, dialogs, Codex/Cursor discovery, the check); the Mac app.
- **Next:** the owner tests preview.35 on Windows. Branch `claude/t041-sparkle-retry` waits up to 15 min for the Sparkle download (GitHub 504s); merge it with this docs update. Open owner decisions: version 0.0.1 and Codex/Cursor as coordinator.

Previous state (2026-11-07, evening, later):

Updated 2026-11-07 (evening, later) by Claude Code.

- **T-039 / D-050 (branch `claude/t039-connect-coordinator`):** the window connects the coordinator chat to Claude by itself. It runs Claude's own `claude mcp add --scope local` in the project folder, right after the project is created and on «Подключить к Claude», and reads `~/.claude.json` to confirm. Worker runs are marked (`AGENT_MESSENGER_WORKER_RUN`), and the coordinator server gives them no tools.
- **Verified here (Linux):** fake Claude CLI tests; a browser run of the built program (connected, error shown, button); the written entry starts the coordinator, which answers `list_agents`. **Not verified:** the real Claude Code (where it writes local-scope servers, especially on Windows), Mac, Windows.
- **Next:** PR, CI, merge; ship in preview.34 together with the Cursor name fix (PR #58).

Updated 2026-11-07 (evening, preview.34) by Claude Code.

- **preview.34 released** from `main` 7617420 (PR #58 Cursor names, PR #59 T-039; run 42396560953), 21 assets. The window connects Claude to a project by itself (D-050).
- **Checked here:** the Linux x64 program from `/releases/latest/download/` matches the SHA-256 list and reports preview.34 at 7617420.
- **Not checked:** the real Claude Code connection (Mac, Windows, Linux); the Mac app; the in-window update preview.33 → preview.34 on Windows and Linux.
- **Open:** PR #60 (T-040) runs the release's Mac checks next to the Windows and Linux job; timing to confirm on preview.35.
- **Next:** the owner installs preview.34 (or updates from 33), creates a project, and checks that a new Claude chat in its folder sees the agents.

Updated 2026-11-07 (evening, release) by Claude Code.

- **preview.33 released** from `main` 3ae2058 (PR #54 with #55, and PR #56; run 42388079985). It is the latest release, with 21 assets: the Mac app zip and its update files; plain Windows and Linux programs (x64, ARM64) with a SHA-256 list; and signed update files for Windows and Linux (`update-<os>-<arch>.json`, `.sig`, `<sha256>.zip`).
- **PR #56:** the first release run stopped on a failing test. A refused window grant opened model calls for a moment before the store refused it. Now the store is checked first.
- **Checked here:** the README links (`/releases/latest/download/...`) download the Linux x64 and Windows x64 programs, and their SHA-256 values match the list. `version` of the Linux x64 program reports preview.33 at 3ae2058. The Linux amd64 update manifest and its archive are signed by the app's update key, and that key is built into the program.
- **Not checked:** the Mac app from this release; real Windows and Linux desktops; the real Cursor CLI; a real update from GitHub, possible only from preview.33 to preview.34.
- **Next:** the owner tests preview.33 on Mac, Windows and Linux, and Cursor with the real CLI. Then preview.34 tests the in-window update on Windows and Linux. Still waiting for the owner: Q-012/T-028, Q-013.

Updated 2026-11-07 (evening, later) by Claude Code.

- **The owner:** there is no rule against Cursor; set it up. The AGENTS.md line was removed, D-011 superseded, Q-004 answered.
- **T-012 (branch `claude/t012-cursor-tasks`, stacked on `claude/t016-download-buttons`):** Cursor runs tasks like Claude and Codex (D-049). Messenger keeps one `messenger_task` entry in `~/.cursor/mcp.json`; the run reaches its tools through `AGENT_MESSENGER_TASK` and `${env:...}`. The «Агенты» card and setup dialog work for Cursor; the coordinator can pick `cursor`.
- **Verified here (Linux):** a fake Cursor CLI that reads `~/.cursor/mcp.json` and fills `${env:...}` like Cursor: report, question and resumed run with `--resume=<chat>`, model, usage limit; a broken `mcp.json` is left alone and the run does not start; the window card and dialog in headless Chromium. **Not verified:** the real Cursor CLI (that `${env:...}` reaches the server, the current event format), Cursor on Windows (not supported: `.cmd`).
- **Next:** the owner's test with the real `cursor-agent` on Mac or Linux: connect Cursor in «Агенты», ask the coordinator to give Cursor a small task.

Updated 2026-11-07 (evening) by Claude Code.

- **Owner's requests:** plain «Download» buttons for Mac, Windows and Linux instead of archives, one heading above them; signed updates from the app on Windows and Linux.
- **D-047 (branch `claude/t016-download-buttons`):** each release publishes plain programs under fixed names and is marked latest; README buttons link to `/releases/latest/download/<name>`; [DOWNLOAD.md](DOWNLOAD.md) explains starting them.
- **T-038 / D-048 (same branch):** `internal/selfupdate` verifies and installs a signed update; the browser window checks every 5 minutes and shows «Доступно обновление» / «Обновиться»; after the install the window reopens from the new program. The release signs `update-<os>-<arch>` archives for Windows and Linux on the macOS job.
- **Verified here (Linux):** `check.sh`; the update package tests; an update preview.900 → preview.901 with a test key through the real window (the program replaced, the window reopened, the old client offered for restart); the signing script with a throwaway key. Windows CI runs the update package tests (including replacing a running program). **Not verified:** a real update from GitHub (only possible from preview.33 to preview.34), a real Windows desktop, the release signing step itself (it runs only in a release).
- **Next:** PR for this branch, green CI, merge, release preview.33; then the owner tests downloads on all three systems, and an update after preview.34.

Updated 2026-11-07 (afternoon) by Claude Code.

- **Owner's instruction:** "довести до релиза сейчас программу, все везде сбилдить, выложить на GitHub" — the owner tests on Windows, Linux and Mac later; decisions that need them are taken cautiously or postponed.
- **T-016 (PR #52, `claude/t016-windows-linux`):** Windows and Linux builds. Windows job objects as process groups (D-043; the client keeps each job's handle for the whole run, a job it cannot reach is not proven), `.exe` agents only (D-044), the window in the browser via `messenger open` (D-045), zip/tar.gz archives in every release, a Windows CI job. Guide: [WINDOWS_LINUX.md](WINDOWS_LINUX.md).
- **Verified:** see "What is verified" in WINDOWS_LINUX.md and the PR. **Not verified:** a real Windows 10/11 desktop, a browser opening on Windows, real Claude Code or Codex on Windows or Linux, ARM builds (compiled only).
- **preview.32 released** from `main` 11d938c (PR #52; run 42380342750): the Mac app zip, `update-darwin-arm64.json` and its `.sig`, and zip/tar.gz archives for Windows and Linux (amd64, arm64) with a SHA-256 list. Checked here: the checksums of the Linux amd64 and Windows amd64 archives, and `messenger version` from the Linux archive.
- **Next:** the owner tests preview.32 on Mac, Windows and Linux: start, the window in the browser, a task with a real agent, a task between two computers. Still waiting for the owner: Q-004 (Cursor), Q-012/T-028, Q-013.

Updated 2026-11-07 (night) by Claude Code.

- **preview.30 released** from `main` 598437c (PR #49, T-035: computers and their agents in the sidebar).
- **preview.31 released** from `main` 019f30a (PR #50, T-037; run 42371886488).
- **T-037 (PR #50, merged):** tasks between computers. The broker carries messages (`/v1/send`, `/v1/inbox`); a coordinator's `start_task` with another computer's ID records a remote task here and the other computer runs it as its own task, sending questions, the report and the outcome back. Answers and cancel go the other way. A computer runs other computers' tasks only when «Задачи других компьютеров» is on («Сервер» tab; off by default, Q-014). Schema version 3. Design: [TASK_MODEL.md](TASK_MODEL.md#tasks-between-computers-t-037), D-041, D-042.
- **Verified here (Linux):** `check.sh`; store tests for the whole exchange (refusal, question → owner answer → report, cancel both ways, a duplicate message, an undeliverable start or cancel); a supervisor test through a real broker over TLS; two separate clients on one machine joined by the window API: start from the coordinator, acceptance with the folder, cancel, refusal while not accepting; window screenshots in headless Chromium. **Not verified:** two real Macs, a real worker run started from another computer (the run itself is the same code as a local task), WebKit.
- **Next:** the owner tests preview.31 on two Macs: join, turn on «Задачи других компьютеров» on the second, ask the coordinator on the first to give a task to the second. Q-014 to confirm. Cursor (T-012) waits for Q-004.

Updated 2026-11-07 (late evening) by Claude Code.

- **T-035 (branch `claude/t035-network-roster`):** every computer reports its agents to the broker every 15 s; the sidebar shows all computers of the network with their agents and statuses, «Был на связи …» after 45 s without a report (D-039). VPS made optional (D-040, T-036 in the backlog).
- **Verified here (Linux):** `check.sh`; broker tests; headless Chromium with a helper program joining as a second computer. **Not verified:** two real Macs.
- **Next:** T-037 tasks between computers — design first in TASK_MODEL.md, with the cautious defaults recorded in QUESTIONS.md.

Updated 2026-11-07 (evening) by Claude Code.

- **«Сервер» tab and sidebar polish (T-015), approved by the owner on a live preview, going into preview.29:** statuses are a dot and plain text; Доступ, Порт and one start/stop button in one row, fields locked while the server runs, «Применить» removed; explanations moved into status tooltips so cards keep their height; computers that joined this server show in the sidebar «Компьютеры» (name, dot, last seen) with «Отозвать» in a right-click menu; invitation dialog copies with an icon; the sidebar shows only «Доступно обновление» and «Обновиться»; control text is centred by measuring the font at start.
- **Verified here (Linux):** `check.sh`; headless Chromium: start/stop, locked fields, focus, join a device to its own server, sidebar card, right-click menu, revoke. **Not verified:** the Mac window (WebKit, SF font centring), two real computers — the owner tests preview.29 on two Macs.
- **Next:** devices send their agents and statuses to the server so the sidebar cards show them (T-015 continued); Cursor adapter (T-012) waits for the owner to allow running Cursor CLI (Q-004).

Updated 2026-11-07 (day) by Claude Code.

- **Done and merged (PR #45):** T-034 failure reasons with the limit's reset time; T-031 bounded `wait`; T-032 lean `tasks` with `if_version`; window tweaks (menus stand apart from cards, green «Работает», the invitation in its own dialog).
- **T-033** (branch `claude/t033-resume-brief`): a resumed run's `read_task` leaves out the task text unless `include_task: true`.
- **Other products are not named anywhere** (owner's request): the comparison document was removed, the rule is in AGENTS.md. The owner deleted old merged branches (T-018).
- **Verified here (Linux):** `check.sh`, new tests with fake Claude and Codex CLIs, window screenshots in headless Chromium. **Not verified:** real usage limits (formats come from the CLIs' docs), the Mac app.
- **preview.28 released** from `main` at 98d346e (PR #45 and PR #46; run 42360015163): the app zip, `update-darwin-arm64.json` and its `.sig`. Not yet installed on the Mac.
- **Next:** owner updates to preview.28; then T-015 continued and Q-012 / T-028.

Updated 2026-11-07 (third night) by Claude Code.

- **Owner's instruction:** "доведи программу до релиза и сделай там настройку сервера", then an audit of the new work with all findings fixed.
- **preview.25 released** from `main` (PR #40): action errors as a chip above the input, a clear "old client in the background" message with the restart command, a fix for the task list status column.
- **T-015, first part** (branch `claude/t015-server-setup`): the background client can host a broker and a computer can join one by invitation. The window's «Сервер» tab enables it (local only by default, LAN on request), creates one-time invitations, lists and revokes devices, joins a broker, checks and drops the connection. `messenger status` shows the state. **Tasks do not travel between computers yet**; the tab says so. Details and file layout: [BROKER.md](BROKER.md); decisions D-033–D-035; Q-013 (device key in a 0600 file for now, not the Keychain).
- **Audit:** two independent reviews (security/protocol and window logic); every finding is fixed (list in [BROKER.md](BROKER.md) and [PR #41](https://github.com/kolotiloff/agent-messenger/pull/41)). One older issue outside this work went to the backlog as T-030.
- **Release:** PR #41 merged into `main` and released as preview.26 (see LOG for the outcome).
- **Verified here (Linux):** `check.sh`, `check-builds.sh`; tests in `internal/broker`, `internal/supervisor` (the client joins its own broker through the control command, revoke, check), `internal/localui`; the tab in headless Chromium in both themes, at 1100 and 600 px, against a real client.
- **preview.27 released** from PR #42: one set of style rules (D-036), the server note in a side column, «Запустить/Остановить сервер», Claude-style dropdowns, and a blocking «Перезапустить / Выйти» dialog for an older background client (D-037).
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

No user session IDs, tokens, auth files, private SQLite dumps or full logs in GitHub. The repository is public. Do not restart the Mac automatically. Do not use working chats as test chats.

## Getting started and verification

Work in a regular clone of the [repository](https://github.com/kolotiloff/agent-messenger) from the current `main`, in a separate branch. The previous developer's local copy of the sources may have been a mirror without `.git`; do not automatically treat it as a full checkout.

```sh
bash scripts/check.sh
```

Go 1.22.2 or later is required. If needed, point to the installed Go with `GO_BIN`. The script itself selects the external linker for race tests on Mac. An additional model-free check of the installed Codex is described in the report; it is skipped by default. Real models are not run in the regular tests.

Deliverable of the next stage: a PR with the implementation, failure tests and updated documentation; list separately what actually passed and what remains. For audit, hand over the PR link and the exact SHA. Publish a new application package after the functional step is finished and verified, not for the sake of an intermediate library.
