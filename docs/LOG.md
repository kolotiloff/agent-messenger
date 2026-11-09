# Work log

One entry per work session, newest first. Write what was done, which tool did it, the PR, and anything the next session must know. Keep entries to a few lines; details belong in PRs and reports.

## 2026-11-09 — Claude Code (release preview.55)

- Merged #86 (T-079 to T-082) and released preview.55; all files checked (Mac, Windows x64/ARM64, Linux x64/ARM64, checksums, 5 signed update feeds and archives).

## 2026-11-09 — Claude Code (T-082 no title bar on Mac and Linux)

- T-082 (D-095): the page runs to the top edge on Mac (transparent title bar, system buttons at the top left) and Linux (empty GTK title bar, page buttons). Verified: Linux window under Xvfb, page in Chromium with a stubbed Mac bridge. The Swift app is checked by CI and the owner. On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-081 limit advice)

- T-081 (D-094): `start_task` warns the coordinator (`limit_note`) when the chosen agent was last close to or at its limit; `list_agents` marks a passed reset. Advice only. On `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-080 limits)

- The owner sees no limits on the Mac and no percent elsewhere. Causes: limits come only from worker tasks on that computer; Claude sends a state, not a share we can read; Codex prints none in `exec` mode. T-080 (D-093): cards name the window and reset, clearer empty text, and the agent's own limit fields are kept for the coordinator (`limit.reported`). On `claude/ui-tweaks`, not released; not verified with real agents.

## 2026-11-09 — Claude Code (T-077, release)

- T-077 (D-091): the window restarts the old background client by itself after an update when no task runs; asks only otherwise. The owner's wish to update another computer remotely is T-078, Q-025.
- Released preview.54 (PR #85) and checked its files. T-079 (D-092): a start screen instead of the restart popup, chosen by the owner from mockups; on `claude/ui-tweaks`, not released.

## 2026-11-09 — Claude Code (T-076, UI tweak)

- T-076 (D-090): statistics of the whole network and filtered by the computer chosen in the sidebar; another computer's agents with their limits, view only. Usage travels with presence only when it changes. The owner approved the look from screenshots with test data. Collected on `claude/ui-tweaks`, not released.
- Language and theme lists have the same width in both languages.

## 2026-11-09 — Claude Code (T-074, T-075, UI tweak)

- Owner's first working days with the program: the coordinator stops reading the owner while it waits and forgets workers' events after an interruption (T-074, D-088); workers that could not commit finished with a cut report (T-075, D-089). Both changed on `claude/ui-tweaks` with tests; not released, the owner asked to collect changes first.
- The empty «Самые затратные задачи» card: «Пока пусто» aligned with the rows.

## 2026-11-09 — Claude Code (releases 50–53, backlog tidy)

- Merged #81 (T-071), #82 (T-070), #83 (T-072 + T-073 decision); released preview.51, .52, .53 and checked every release has all its files. preview.50 (T-057) checked too.
- T-030 and T-019 moved to Done: both were already done (T-029, T-059); Q-001 marked answered.

## 2026-11-08 — Claude Code (T-072 resource locks)

- `start_task` takes an optional `lock`; a later unfinished task with the same lock on the same computer waits to start (D-086). The window says «Ждёт ресурс «…»», the coordinator sees `waiting_for_lock`.
- Verified: core and MCP tests, `check.sh`. Not verified with real workers.

## 2026-11-08 — Claude Code (T-070 readiness)

- Each computer's presence report says whether it takes other computers' tasks and which projects have a folder; `list_agents` shows `accepts_tasks` and `has_project` (D-085). An older host refuses the field; the report then goes without it for 10 minutes.
- Verified: broker, core, supervisor tests, `check.sh`. Not verified between real computers.

## 2026-11-08 — Claude Code (T-071 task activity, T-049 split)

- T-049 split into T-070…T-073 and Q-024. T-071: the coordinator's `tasks` shows an open task's last worker action with its time and every task's tokens (D-084).
- Verified: core and MCP tests, `check.sh`. Not verified with a real coordinator.

## 2026-11-08 — Claude Code (T-057 remote log)

- Coordinator tool `get_log(computer, lines)`: request and reply go through the mailbox; the other computer answers only with «Отдавать журнал по сети» on (off by default) and removes ids, codes and keys (D-083). Screenshot: docs/mockups/log-share-built.png (stubbed).
- Verified: core, supervisor (two computers, real broker), MCP tests, `check.sh`. Not verified between real computers.

## 2026-11-08 — Claude Code (T-053 Windows installation)

- A double click installs the program for the user (D-082): programs folder, Start menu, «Приложения» with «Удалить»; data always kept (Q-023). `install` and `uninstall` commands on Windows.
- Verified: plan tests, `check.sh`, `check-builds.sh`, a new CI step that installs and removes on the Windows runner. Not verified by hand on Windows.

## 2026-11-08 — Claude Code (T-052 short invitation)

- 6-digit code next to the long one (D-081): found on the local network over UDP 47100, proved with SPAKE2, 3 tries, 10 minutes. New dependency `filippo.io/edwards25519` v1.1.0. Screenshot: docs/mockups/short-invite-built.png (stubbed).
- Verified: pairing tests on this computer (loopback), `check.sh`. Not verified: two real computers, firewalls, broadcast on Wi-Fi.

## 2026-11-08 — Claude Code (T-047 shared task list)

- Each computer shares the tasks it started (7 days, up to 50, start of the report) through the broker's `POST /v1/tasks`; the task list shows the other computers' tasks of the project read-only (D-080). Screenshot: docs/mockups/shared-tasks-built.png (stubbed data).
- Verified: broker test (limits, revoked, replacement), store test (what is shared, duplicates hidden, dropped on leaving), two-computer supervisor test, `check.sh`, headless Chromium. Not verified between real computers.

## 2026-11-08 — Claude Code (T-059 window language)

- «Язык» in the sidebar: Russian (default) or English (D-079), on top of T-069. Client messages, CLI and Mac menus stay Russian.
- Verified: translation coverage test, `check.sh`, headless Chromium in English and Russian. Not verified: native windows.

## 2026-11-08 — Claude Code (T-069 Linux window)

- Linux window with the system's WebKitGTK through `purego` (D-078), on top of T-068. Browser path kept as fallback.
- Verified in this container under Xvfb: page shown, Ctrl+C closes, no display falls back. Not verified on a real Linux desktop.

## 2026-11-08 — Claude Code (T-068 token statistics)

- «Статистика» tab (D-077): today / 7 days / 30 days, total, bar per day with a tooltip, by agent, by computer, ten most expensive tasks (click opens the task). Backend: `Store.UsageStats`, control `usage-stats`, `GET /api/usage-stats?period=`.
- Verified: core and route tests, `check.sh`, headless Chromium with stubs in both themes. Not verified: native windows, real usage.

## 2026-11-08 — Claude Code (T-062 Windows window without title bar)

- Windows window without the system title bar (D-076): own minimise, maximise, close; drag by the tab row and the sidebar top; double click maximises. Built on `claude/t061-agent-cards`, released together with T-061.
- Verified: Windows cross-build with vet (x64, ARM64), page controls in headless Chromium with a stubbed bridge. Not verified on a real Windows computer.

## 2026-11-08 — Claude Code (T-061 agent cards)

- Agent cards and the settings dialog built as approved (D-070, D-075): check, gear and on/off switch per card; four steps; one «Задачи» button. The switch keeps the agent's settings and hides it from the coordinator and other computers. Claude lists `haiku, sonnet, opus` from the start; models are one comma-separated field. Screenshot: docs/mockups/agent-cards-built.png (stubbed data).
- Cautious choices (Q-022): «Удалить» opens the official page; «CLI неисправен» not shown yet.
- Verified: core, window route, protocol and diagnostics tests, `check.sh`, headless Chromium with stubs in both themes. Not verified: native windows, real agents.

## 2026-11-08 — Claude Code (before preview.42: owner's answers)

- Owner's answers: no dollars, tokens only (D-074); server status «Подключено / Нет связи / Не подключено» (Q-021); project rename stays local (Q-019); «v0.42» display with unchanged tags (Q-020).
- Owner's order of work recorded in TASKS: design (T-061, T-062), then statistics (T-068), Linux window (T-069), language, shared list, short invitation, Windows installation, remote log, coordinator wishlist. Owner: release an update after each step.
- Verified: `check.sh`; task rows in headless Chromium with stubs show tokens only.

## 2026-11-08 — Claude Code (T-058 what a task spent)

- The task list shows each task's cost (Claude's own estimate) or tokens (Codex, Cursor); the task header shows input, cached and output tokens (D-068). Remote tasks report their total back.
- Verified: adapter and core tests, `check.sh`, mocked screenshot. Not verified: real CLI output (no real models), Cursor's usage format, anything on the owner's computers.
- Statuses shortened, with commas («Готово за 3 мин, $0.42»; the owner does not want dots between words). T-060: Claude on Ноут-Linux was blue because a chat had once used Messenger's tools there and that connection never expires; blue now means a running task only (D-069). T-059 (Russian/English window) recorded.

## 2026-11-08 — Claude Code (T-056 one switch for commands, login and logout on cards)

- The owner turned «Любые команды» on everywhere but it did not help. Cause found in code: an answer or a follow-up ran with the setting its task started with. Now such runs take the agent's current setting. Logs on the owner's computers were not read: they are not reachable from this session.
- One «Любые команды» switch for every agent of the computer on the Agents tab (D-067); the per-agent switch is gone from the dialog.
- Cards: «Войти» when no login is confirmed, «Выйти» (asks first) after a confirmed login; an agent with the standard profile is checked too, which used to leave «Не проверен» without a button. «Проверить» moved out of the card: the window checks on open and when the owner returns after signing in or out. Card actions are one row: «Задачи», «Настроить».
- preview.40 was released earlier today (T-055).
- Later the same day: «Настройки» on cards; dialog statuses stay on their button's line; click a computer in the sidebar to filter tasks; agent icon in the name line so chat text starts at the edge; a run without a report keeps the worker's last message.
- Ноут-Linux Codex: the owner's screenshot shows the cause, `~/.local/bin/codex-code-mode-host` is missing (Codex installed incompletely), so the Messenger tools can not run. Not a Messenger fault. Advised to reinstall Codex.
- Owner's idea of fetching logs from any computer recorded as T-057 / Q-018.
- Then: a slider for «Любые команды», new worker agents start with it on, the agent dialog saves by itself, the update is one «Обновить» button. Not done yet: the Server tab's on/off buttons are not sliders yet.
- Verified: tests, `check.sh`, mocked screenshots. Not verified: `claude auth logout` and the other sign-out commands on real CLIs, anything on the owner's computers.

## 2026-11-08 — Claude Code (T-055 confirm popup; project settings in the selector)

- Removing a project now asks in a popup «Удалить проект «X»?» with «Отмена» and «Удалить» instead of a second click on the same button (owner).
- The project selector has «Настроить проект…»: it opens the project dialog with the folder and rules. The folder bar on the Agents tab is gone; the current folder is shown in that dialog.
- «Любые команды» is a two-button switch «Не разрешены / Разрешены» instead of a checkbox.
- Verified: `node --check`, `check.sh`, a static screenshot of both dialogs in headless Chromium (no running client). Not verified: the flow against a live client, on Mac, Windows or Linux.

## 2026-11-08 — Claude Code (T-054 full access for workers, T-055 remove a project)

- preview.38 released from PR #68. The owner's coordinator found that only Cursor on PC-1 could commit: Claude workers wait for command approval, Codex's sandbox has no network. «Разрешить любые команды» per agent (D-065). The owner's missing task on the Mac was another project selected in the window, not a bug.
- «Удалить проект» in the project dialog (D-066): refused while a task of the project is unfinished; history kept.
- Verified here: adapter, core and window route tests, `check.sh`. Not verified: real CLIs with the switch; the dialogs in a browser.

## 2026-11-08 — Claude Code (sidebar without «координатор»; T-052, T-053 planned)

- PRs #66 and #67 merged. The sidebar no longer marks agents as «координатор» (D-064). The owner asked for a 6-digit invitation (T-052, Q-017) and a normal Windows installation with removal from Settings (T-053); both planned on `claude/t052-t053-plan`.
- Verified here: JS syntax, `check.sh`. Not verified: the sidebar on Mac and Windows.

## 2026-11-08 — Claude Code (T-051 continue a task; agent card lines)

- `continue_task` resumes a finished task's conversation with a follow-up (D-063), kept simple: no recovery after a restart; unclear or remote tasks refused. Agent cards: «Выполняет задачи» removed, each state is one short line in one font (owner's feedback on preview.37).
- Verified here: core and coordinator tests, `check.sh`, a headless Chromium screenshot of the cards. Not verified: real CLIs resuming a finished conversation.

## 2026-11-08 — Claude Code (T-050 worker rules, short reports)

- preview.37 release started from `main` da18160 (PR #65). The owner's coordinator proposed ways to save tokens; points 1 and 2 are done (D-062): per-project worker rules copied into each task (also remote), a built-in short-report rule, and `report` fields `changes` / `tests` / `files` stored as labelled lines. Continuing a finished task is T-051, kept simple at the owner's request.
- Verified here: core tests (rules stored, copied, kept, sent to the other computer, its own rules not used), report composition, MCP argument test, window route test, `check.sh`. Not verified: real CLIs filling the new fields; the dialog in a browser.

## 2026-11-08 — Claude Code (T-048 computer names)

- The owner's Linux laptop joined as «Ноут-Linux», but its own window showed the system name, and a rename in «Имя в сети» never reached the others (D-039 kept the joining name). Now each report carries the window's name and the broker renames the device (D-061); a computer that joined earlier takes its joining name. The coordinator's wishlist from the owner's project is recorded as T-049; the shared task list is T-047.
- Verified here: broker test (rename reaches the host's roster and device list, the connection file keeps it), core test of taking the joining name, `check.sh`. Not verified on the owner's computers.

## 2026-11-08 — Claude Code (T-046 owner's UI feedback)

- After updating to preview.36 the owner found: the restart button on the left and «Перезапускаем…» written in it; agent statuses empty until the setup dialog's check; stretched dialog buttons; «Подключить исполнителем» unclear; on Linux the page opened in the browser (no Chromium-based browser installed, Q-016). Changed as in D-060.
- Verified here: core test of disconnecting, window route test, `check.sh`, headless Chromium screenshots of the restart dialog, the cards (three automatic checks) and the setup dialog. Not verified: a real Windows, Mac or Linux window.

## 2026-11-08 — Claude Code (T-045 icons, Windows update restart)

- The owner's Linux agent said there is no menu entry: our doc was stale (the first `messenger open` adds one since preview.35), but the entry had no icon. Now Linux gets the icon (SVG in the user's icon folder) and the page has a favicon. Windows had no icon at all: `scripts/gen-icon` draws the Mac icon with the standard library and writes `cmd/messenger/rsrc_windows_{amd64,arm64}.syso` (a hand-written COFF resource; an external resource tool was not allowed to run here); the window loads it.
- The owner pressed «Обновиться» on Windows: «Обновлено. Окно откроется заново.», then nothing. The window was closed by `Terminate` from another goroutine; it posts WM_QUIT to the calling thread, so it was lost. Now it runs on the window's thread.
- Verified here: the PE files built for amd64 and arm64 contain the icon resources at relocated addresses (read back with `debug/pe`); tests of the generator and the Linux entry; `check.sh`. Not verified: the icon and the restart on a real Windows; the icon in a Linux desktop.

## 2026-11-08 — Claude Code (T-044 limits per agent)

- Each agent's last reported usage limit is kept and given to the coordinator as a `limit` field of `list_agents`, shown on the agent card («Лимит») and sent to the other computers (D-059). The coordinator asked for a field, not text in the report.
- Verified here: `check.sh`, Windows vet, parser tests (Claude and Codex shapes), core and a full supervisor run with a fake CLI that hits its limit. Not verified: what the real CLIs print (Q-015); a real network.

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

## 2026-11-08 — design drafts saved

- Saved the draft mockups that are still being agreed into `docs/mockups/` (settings dialog, per-card switch, Windows window without the system title bar) and wrote the points already agreed into T-061; added T-062 for the title bar. Nothing built; waiting for the owner's approval.

## 2026-11-08 — T-063 widths and empty-click

- The sidebar and the task list can be resized by dragging their edge (double click restores, width remembered). A click on empty sidebar space shows all computers' tasks again. Checked in headless Chromium against the page assets only; not in the native windows.

## 2026-11-08 — T-064 tab «Проект»

- The project's settings moved from the project list to a new tab «Проект» (D-071). The project list now holds only projects and «Новый проект». Ellipses dropped from all button labels. Checked in Chromium with a stubbed window API; not against a live client or in the native windows.

## 2026-11-08 — T-064 auto-save, T-065 project rename

- The «Проект» tab saves by itself and shows only errors; «Удалить проект» stands on the right; the folder label is «Папка проекта».
- A project can be renamed on the tab. Only the shown name changes (D-072); Q-019 asks whether a rename should reach other computers and the coordinator. Core and route tests added; the page checked in Chromium with a stubbed API, not against a live client.
- Owner idea recorded as T-066: the update offer as a blue button at the top right.

## 2026-11-08 — «Любые команды» text

- The explanation under «Любые команды» on the Agents tab is normal-size muted text, not a small hint (owner). Checked in Chromium.

## 2026-11-08 — T-067 Server tab, variant Б

- The owner chose variant Б: three cards in one column, the accept switch inside «Этот компьютер», the name saves by itself, no «Сохранить», «Проверить связь» or side note, one computer name. The owner also approved the agent cards with the per-card switch and the settings dialog (T-061, not built yet). Checked in Chromium with a stubbed API only.

## 2026-11-08 — T-066 update icon, version name

- An available update is a blue round arrow next to the version at the bottom of the sidebar (owner's variant А); tooltip names the version. The window shows «v0.41»; tags stay 0.0.0-preview.N (D-073, Q-020). Checked in Chromium with a stubbed status only.
