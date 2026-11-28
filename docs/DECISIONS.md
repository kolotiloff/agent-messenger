# Decisions

Decisions that are already made, so that no tool reopens them by accident. Each entry is short and links to the document with the reasoning. To change a decision, add a new entry that supersedes the old one, and mark the old one `superseded by D-NNN`. Do not edit old entries otherwise.

Format: ID, date, decision, why, where it is discussed.

| ID | Date | Decision | Why | Details |
|---|---|---|---|---|
| D-001 | 2026-11-05 | The product client and the future broker are written in Go. Python stays in `experiments` as research only. | One static binary per OS, no runtime to install. | [MVP.md](MVP.md), [CLIENT.md](CLIENT.md) |
| D-002 | 2026-11-05 | The local queue is SQLite through the pure-Go `modernc.org/sqlite` driver, with `synchronous(FULL)`. | Durable local state without cgo or a separate service. | [CLIENT.md](CLIENT.md) |
| D-003 | 2026-11-05 | Agents are reached through adapters that own existing CLI sessions (Claude Code, Codex, Cursor), not through a network-agent gateway. Network delivery and execution are kept separate, so an API agent can be added later behind the same address. | Keeps each provider's working folder, tools, login and context. | [PLAN.md](PLAN.md) Refined by D-021: Messenger starts new task sessions; it does not attach to existing chats. |
| D-004 | 2026-11-05 | Fail closed. An uncertain result is never retried automatically, and unknown is not success. A reservation is never released by a timeout, a single PID or an empty process group. | A duplicate model call or two owners of one session is worse than a stuck queue. | [MVP.md](MVP.md), [AGENTS.md](../AGENTS.md) Partly superseded by D-021 for task runs. |
| D-005 | 2026-11-05 | Persistent Claude runs as one `stream-json` process that receives messages one turn at a time. The owner is not released between turns. | A one-shot run cannot prove that detached descendants have exited. | [CLAUDE_STREAM.md](CLAUDE_STREAM.md) Superseded by D-021 for new work. |
| D-006 | 2026-11-05 | The coordinator is a role assigned to one of the agents. The broker has no model of its own. The broker can run on any of the computers or on a VPS; a VPS is optional. | Keeps the broker simple and provider-neutral. | [DEPLOYMENT.md](DEPLOYMENT.md) |
| D-007 | 2026-11-05 | Mac first. Network, Windows and Linux follow once the local client is accepted. | The owner's main machine is a Mac. | [MVP.md](MVP.md), [RELEASE_PLAN.md](RELEASE_PLAN.md) |
| D-008 | 2026-11-06 | Mac app updates use Sparkle with signed packages from public GitHub Releases. The signing key stays in the owner's Keychain. Updating the shell does not replace a running background client. | Standard, verified update path; no home-grown binary replacement. | [UPDATES.md](UPDATES.md) |
| D-009 | 2026-11-06 | The window keeps the chosen `design/v1` look with light, dark and system themes. `design/v1` is frozen. UI tweaks ship together with the next functional step, not as separate builds. | The owner approved this design. | [design/README.md](../design/README.md) |
| D-010 | 2026-11-06 | Persistent Codex uses a separate stdio `codex app-server` with a managed profile. The effective config is checked with `config/read` before a thread is created; missing or `null` fields do not count as disabled. | `-c` flags and an empty MCP table do not prove isolation. | [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md) |
| D-011 | 2026-11-05 | Cursor CLI is not launched until the owner says otherwise. | The owner's request. | [HANDOFF.md](HANDOFF.md) Superseded by D-049 (2026-11-07): the request was one-off. |
| D-012 | 2026-11-06 | Code, documentation, commit messages and PRs are in English. The app UI and CLI messages stay in Russian for now. The owner is addressed in Russian. | English suits every tool and future contributors; the UI language is a separate decision (Q-001). | [AGENTS.md](../AGENTS.md) |
| D-013 | 2026-11-06 | The repository is the only memory shared between tools. Rules live in `AGENTS.md` (`CLAUDE.md` imports it). State lives in `HANDOFF.md`, `TASKS.md`, `DECISIONS.md`, `QUESTIONS.md` and `LOG.md`. All work is pushed before switching tools. | The owner switches between Claude Code, Cursor and Codex. | [README.md](README.md) |
| D-014 | 2026-11-06 | A Codex session is created through `codex_setups`, not by registering a known ID. The thread is created with the Messenger dynamic tools and kept by its live app-server, and no `sessions` row exists until delivery to that owner is implemented. Every failure after launching is uncertain and keeps the alias and reservation. | Codex issues the thread ID, tools can only be declared at `thread/start`, and an empty thread is not saved to disk. | [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md) |
| D-015 | 2026-11-06 | A created Codex thread is registered as a `codex_owned` session with no CLI path. Only its owner's controller claims its messages. While a turn runs, server requests other than `item/tool/call` are refused with an error rather than ending the turn; while idle, any server request makes the owner uncertain. | Keeps the generic loop and older clients from starting a second process on the thread, and keeps an unexpected approval prompt from being granted. | [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md) |
| D-016 | 2026-11-06 | An uncertain Codex owner is released only by `recover-codex` after a proven reboot of the same host, like `recover-stream`. The session is then closed without retry and its thread is not resumed. | A missing PID or empty group does not prove that descendants stopped; resuming would need a new owner, and an empty thread is not saved. | [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md) Superseded by D-021 for new work. |
| D-017 | 2026-11-06 | A reply whose recipient is another open local alias is queued for it in the same transaction as the reply. At most 3 automatic replies per chain. Replies to the user or to unregistered senders are only stored. | Lets Claude and Codex talk without a separate send tool, and stops endless acknowledgements as required in PLAN.md. | [ROUTING.md](ROUTING.md) |
| D-018 | 2026-11-06 | The database records `schema_version` (now 1). A client refuses to open a newer one before changing anything. Codex config fingerprints are taken over canonical JSON with sorted keys and checked again before every turn. | Protects data from older clients in future upgrades. app-server reorders config keys between calls, so raw bytes cannot be compared. | [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md) |
| D-019 | 2026-11-06 | Each persistent Claude session has its own controller, with at most 4 concurrent streams. `stop-job` without an alias stops every live stream. | Removes starvation between sessions; a bound keeps the process count predictable. Stopping all at once is the conservative reading of the old single-job command. | [CLAUDE_STREAM.md](CLAUDE_STREAM.md) |
| D-020 | 2026-11-06 | A persistent Claude session may start messages to other local agents with `send`, which is bound to the turn's receipt. Sends count toward the chain's automatic-step limit, with at most 3 per turn, and the turn still has to `reply`. The window may authorize up to 4 test sessions per client run. | Coordination needs an agent to start a conversation. Tying `send` to the receipt and the limits keeps it bounded, and the required `reply` keeps the result of the current message bound to it. | [ROUTING.md](ROUTING.md) |
| D-021 | 2026-11-06 | The product becomes "one task, one session". The coordinator is the owner's own chat (Claude first) with Messenger connected over MCP; it waits for reports with a blocking `wait`. Each task gets a new worker session (Claude or Codex) that reports and closes. A clarification within a task resumes the same session by exact ID; a new task always gets a new session. A task run is finished after an observed exit, an empty process group (after signalling it) and a revoked receipt; this is best-effort cleanup, accepted by the owner. Unclear evidence blocks only that task; nothing is retried automatically. The coordinator connection is provider-neutral so Codex or Cursor can coordinate later. | The owner's real workflow; the persistent model blocked on every failure until a reboot. The T-023 pilot showed that a Claude-app coordinator waits, wakes by itself on reports and loses nothing. | [RETHINK.md](RETHINK.md), [RETHINK_REVIEW_V3.md](RETHINK_REVIEW_V3.md), [pilot report](reports/2026-11-06-wait-pilot-claude-mac.md), Q-008 |
| D-022 | 2026-11-06 | How work is planned and handed out lives in each project's own documentation, agreed between the owner and the coordinator in chat. Messenger does not approve plans and does not limit how many workers run at once; parallelism is the coordinator's call under the project's rules. Its skill only explains its tools. Messenger only prevents technical duplicates (a repeated `start_task` or `answer` after a lost response must not open a second session). | Every project has different rules and priorities; Messenger stays project-neutral. The owner relies on the coordinator, not on hard limits. | Q-008 |
| D-023 | 2026-11-06 | A worker runs as if the owner had opened a session in its folder: the provider's normal tools and that project's own settings and permissions, plus the Messenger worker tools. The stripped `messenger-only` profile stays for automated tests. | Workers must do real work, as the owner's sessions do today. | Q-008, [RETHINK.md](RETHINK.md) 4.4 |
| D-024 | 2026-11-06 | The window becomes an observation panel. The sidebar lists computers; the one running the coordinator is marked "Coordinator · <agent>"; selecting a computer shows its tasks. A "Tasks" tab: a task list like a chat list (active first, then finished, filter by agent), and each task shown as its worker session, rendered like a session in the Claude app (coordinator and owner messages as user turns, the worker's text, and its tool actions as collapsible lines, all in one feed), the same agent cards with role and task counts. The owner talks to the coordinator in the Claude chat, not in the window. A worker ends each run with a report, a question to the coordinator, or a question to the owner; which question route is allowed is the project's rule. A question to the owner marks the task with a yellow dot and appears in that task's conversation; an answer field appears there only while the worker waits for the owner, and the answer resumes the worker's session. The coordinator does not read or wake on that exchange. Look and themes stay. | The owner works through the coordinator chat; some projects let workers ask the owner directly to save the coordinator's context. | [window comparison](https://claude.ai/artifact/LcrW6vqaDu1kqYsG6QUVw4), clickable mockup approved by the owner on 2026-11-06: [docs/mockups/2026-11-06-task-window.html](mockups/2026-11-06-task-window.html) (layout reference only; the window keeps its own styles; details in T-029), D-022 |
| D-025 | 2026-11-06 | Each computer has one working folder, chosen by the owner in that computer's window ("Рабочая папка"); it cannot be changed remotely. All agents on that computer work there. The coordinator chooses the computer; the worker runs with the working folder as its working directory, so the CLI loads the project's own instructions and settings (`CLAUDE.md`, `AGENTS.md`, `.claude/settings.json`, `.codex/config.toml`). A computer without a working folder refuses tasks. `list_agents` shows each computer's folder. Changing the folder affects only new tasks; each task records its folder. Messenger checks whether the folder is trusted by each agent and warns if project settings (Codex project config, Claude project MCP servers) would not load in a background run. Temporary folders are refused. Several folders per computer (a folder per task, switching between projects) are not supported for now; they can be added later without changing the task model, since each task already records its folder. | Workers are CLIs started by Messenger, so the folder must be known in advance. The owner works on one project at a time and wants one folder to choose, not a list of named projects. A coordinator must not be able to send a worker into an arbitrary folder. | D-021, D-023, D-024 |
| D-026 | 2026-11-06 | Several projects from the start. A project has a name, one coordinator connection (a Claude chat connected to Messenger with that project name in its MCP server arguments), a working folder per computer, and its tasks. D-025 applies per project: each computer has at most one folder per project, chosen in that computer's window; a computer without a folder for the project refuses its tasks. `wait`, `tasks` and `list_agents` answer for the caller's project only. Agents are shared between projects: one Codex can run tasks for two projects in two folders at once. Window: a project switcher at the top of the sidebar (a plain list of project names, no statuses); tasks, task headers and the folder on the Agents tab follow the selected project; agent status dots in the sidebar show the agent's state across all projects. With one project the switcher shows one entry. A project is created from the last item of the switcher, "Новый проект…": a name and the project folder on this computer. Until a coordinator chat connects with that project name, the task area shows how to connect it (open the folder in the Claude app and run the `claude mcp add --scope local …` command shown, with a copy button). | The owner may develop two games at once with two coordinator chats; the cost in the task core is small if it is there from the start. | D-021, D-025, [mockup](mockups/2026-11-06-task-window.html) |
| D-027 | 2026-11-06 | Task workers are launched only by a client started with `--allow-tasks`, and not while the queue is paused. A run that was launching or running when the client stopped becomes `interrupted`, and its reservation stays. A run whose exit and empty process group (after a signal, waiting up to 3 s) were observed retires its reservation, whether finished, failed or cancelled. The coordinator's own actions (`started`, its `answered`) and the owner's exchange with a worker do not wake its `wait`. | The most cautious reading of D-021 and D-022 that still lets tasks run without per-task approval; it keeps model calls behind an explicit switch (Q-009). | [TASK_MODEL.md](TASK_MODEL.md) |
| D-028 | 2026-11-07 | Claude workers in the worker profile run with `--permission-mode acceptEdits`: they edit files in the project folder without asking, like Codex workers with `workspace-write`. Shell commands and everything else still follow the project's own permission rules. Answers Q-011 (owner: "иначе зачем мы делаем программу"). |
| D-029 | 2026-11-07 | The client the window starts runs task workers (`--allow-tasks`) and the window resumes it at once; it still authorizes no session's model calls. The pause stays as a manual stop for new launches. Answers Q-009 (owner: a separate "allow tasks" button has no point). |
| D-030 | 2026-11-07 | The coordinator may choose a worker's model: `start_task(…, model?)`. The name is validated, stored on the task and passed to every run of it (Claude `--model`, Codex `-c model=…`), so an answer never switches the model. No model means the CLI's default. The coordinator's instructions and skill say: default for most tasks; Claude `opus` for hard work, `sonnet` for ordinary coding, `haiku` for small mechanical work; never guess a name. The window shows the model in the task header. | Owner's request: the coordinator should be able to start the right model when needed. | [TASK_MODEL.md](TASK_MODEL.md) |
| D-031 | 2026-11-07 | The window's stop button cancels at once, like a chat's stop (owner: "кнопка остановить как у тебя"). It uses the same `CancelTask` as the coordinator; a running worker is stopped by the normal cancel path (signal, observed exit and empty group), never by releasing a reservation early. | Same behaviour the owner knows from the chat; the cancel path already keeps D-021. | [TASK_MODEL.md](TASK_MODEL.md) |
| D-032 | 2026-11-07 | Cleanup without T-028: code that nothing calls was removed (including an unused helper that released a reservation by a single PID, which the rules forbid), and the `experiments/` research stands were removed from the tree; documents link to them at commit `41e0e6e` on `main`. The persistent Claude stream and Codex app-server owners stay until the owner explicitly agrees to T-028 (Q-012). | Owner asked to clean up old code; removing live session owners needs their consent first. | Q-012 |
| D-033 | 2026-11-07 | The broker for "run a server on this computer" runs inside the background client's process, not as a separate service. Off by default, local only by default, LAN on the owner's explicit choice; it stops with the client. | One process to install, update and keep alive on the Mac for now; the broker code is a separate package with its own lock and data folder, so moving it into its own service later changes packaging, not the protocol. DEPLOYMENT.md asked for a separate service; this is a deliberate first step. | [BROKER.md](BROKER.md) |
| D-034 | 2026-11-07 | Trust between a computer and a broker is a pinned certificate fingerprint carried in a one-time invitation; the broker identity and protocol are checked before any secret is sent; a computer belongs to one broker and joining another needs an explicit disconnect. | Works for a home computer and an IP-only VPS without a domain or a public CA, and never disables certificate checks. | [BROKER.md](BROKER.md) |
| D-035 | 2026-11-07 | The standalone VPS broker and the SSH install wizard are not part of this step. | The hosted broker and joining had to work first; the VPS package reuses the same `internal/broker` code. | [DEPLOYMENT.md](DEPLOYMENT.md) |
| D-036 | 2026-11-07 | One set of style rules for the window: spacing by meaning (8 within a control group, 12 inside a card, 16 between cards, 24 between regions), four radii (4/8/12/16), three control heights (32/28/24, as in the Claude app; first 36/32/24, reduced on the owner's request), five text sizes. Agent cards and server sections share one card component. | Owner: spacing differed between the sidebar and the cards; "the style is scattered, keep one set of rules". | [UI_STYLE.md](UI_STYLE.md) |
| D-037 | 2026-11-07 | The desktop window restarts an older background client. While the client is older, a modal dialog covers the whole window with two choices: «Перезапустить» or «Выйти» (quits the app only; the background client keeps running). Esc does not close it. The client reports its version in `status`; the window offers the restart only when the client is older (or predates version reporting), never to replace a newer one. The restart uses the client's own `shutdown`, starts the new client only after the old one stops answering, starts it the way the window does (`--claude-stream --allow-tasks`), and keeps an owner's pause. If tasks run, or the window cannot tell, it asks first: an interrupted run stays blocked until the owner decides. A browser window shows the command instead. | The owner does not want to use a terminal after an update. `shutdown` was already the documented way; a button runs the same command with the same guarantees. |
| D-038 | 2026-11-07 | We change only what saves tokens or adds reliability: bounded `wait` answers (T-031), a lean `tasks` list (T-032), no repeated task text on resumed runs (T-033), failure reasons with reset times (T-034). Not taken: input pasted into terminals, automatic recovery or approvals, P2P over SSH (kept as an option for T-015), phone, voice. | Owner's filter: tokens, simplicity, reliability. Our fixed instructions are already ~1.2k tokens; the cost is in variable answers. Nothing may weaken D-004 or D-021. | [TOKEN_REVIEW.md](TOKEN_REVIEW.md) |
| D-039 | 2026-11-07 | Every computer of a network reports its agents to the broker every 15 s (agent program, running and waiting-for-owner counts, coordinator program). The broker answers with the list of computers; the window shows them in the sidebar «Компьютеры» like this computer. A computer counts as reachable while reports arrive (45 s). A device keeps the name it joined with; reports are size- and format-checked. Reporting is a status update replaced by the next one, so a failed round is shown and not retried early. | Owner: «слева все компьютеры сети со всеми агентами и статусами». | [BROKER.md](BROKER.md) |
| D-040 | 2026-11-07 | A VPS is optional. Computers on one local network use a computer-hosted broker. Computers in different networks need the broker to be reachable: an open port on the router, a VPN between the computers, or a VPS. The standalone VPS package moves to the backlog (T-036). | Owner: «теперь VPS нам нужен вообще?» — in one network it is not. | [DEPLOYMENT.md](DEPLOYMENT.md) |
| D-041 | 2026-11-07 | Tasks between computers go through the broker's mailbox. The computer that starts a task keeps a remote task with no runs; the computer that runs it keeps an ordinary task that reports its events back. Every message is written in the same transaction as its change. A computer runs other computers' tasks only when its owner allows it. | Keeps one place that launches and verifies a run, and no change without its message. | [TASK_MODEL.md](TASK_MODEL.md#tasks-between-computers-t-037), [BROKER.md](BROKER.md) |
| D-042 | 2026-11-07 | A task message the broker did not confirm is sent again with the same operation ID; the broker stores an operation once and the receiver applies it once. A message refused because the recipient left the network is final: a start fails its task, an answer or cancel blocks it. | Repeating a delivery is not repeating an action; unknown outcomes on the other computer stay unknown. | [TASK_MODEL.md](TASK_MODEL.md#tasks-between-computers-t-037) |
| D-043 | 2026-11-07 | On Windows a run's process group is a named job object (`Local\AgentMessenger-run-<pid>`): the worker starts suspended, joins its job, then runs; children cannot leave it; the client keeps each job's handle open for the whole run (Windows drops a job's name when its last handle closes, even while its processes run), so only a job it sees with no active process is an empty group; a job it cannot reach is not proven, never empty; the stop is `TerminateJobObject` (a hard stop). The boot identity is the registry `BootId` (or the last shutdown time). | D-021 needs a group the run cannot leave and a check that it is empty; a job gives both, more strictly than a Unix process group. | [WINDOWS_LINUX.md](WINDOWS_LINUX.md#process-control-on-windows-d-043) |
| D-044 | 2026-11-07 | On Windows only `.exe` agent programs are started. `.cmd` and `.bat` scripts are refused; for Codex from npm, the `codex.exe` inside the package is found and used. | A script runs through cmd.exe, which reparses the JSON and paths passed as arguments. | [WINDOWS_LINUX.md](WINDOWS_LINUX.md#agents) |
| D-045 | 2026-11-07 | Windows and Linux get the window in the default browser (`messenger open`, a double click on Windows) and zip/tar.gz archives in every release, not signed and without self-update for now. A native window, installers and updates there are later work. | The owner wants to test on Windows and Linux now; the page is the same one the Mac app shows. | [WINDOWS_LINUX.md](WINDOWS_LINUX.md) |
| D-046 | 2026-11-07 | Q-014 settled by the agent on the owner's instruction to decide open questions: a computer runs other computers' tasks only after its owner turns on «Задачи других компьютеров» (off by default). | The cautious choice; one switch per computer. | [QUESTIONS.md](QUESTIONS.md) |
| D-047 | 2026-11-07 | Downloads without archives, behind buttons. Each release publishes the Windows and Linux programs as plain files and a copy of the signed Mac app zip, all under fixed names (`AgentMessenger-Mac.zip`, `AgentMessenger-Windows-x64.exe`, `AgentMessenger-Linux-x64`, ARM64 variants), and is marked as the latest release (not a pre-release), so the README buttons can link to `/releases/latest/download/<name>`. The Mac app stays a zip because an app is a folder; the checksum-named zip stays for the updater, which reads every release regardless of the flag. | The owner asked for plain «Download» buttons for Mac, Windows and Linux instead of archives. Linux users run `chmod +x` once, because a browser download drops the executable bit. | [DOWNLOAD.md](DOWNLOAD.md) |
| D-048 | 2026-11-07 | Windows and Linux update themselves from the window. The release signs one archive per system and architecture with the Mac update key (same `updatefeed` format: `update-<os>-<arch>.json` + `.sig` + `<sha256>.zip` with only the program inside). The window checks every 5 minutes; «Обновиться» verifies the signature again, runs the new program's `version` and requires the signed version and revision, then replaces only the program file the person started (Windows: rename the running file aside, remove it later) or installs a new version for a `messenger install` installation, and reopens the window. The background client is not restarted by the update; the existing «Перезапустить» does that, as on the Mac. | The owner asked for signed updates from the app. Replacing only the started file and leaving the client and its tasks alone keeps an update from ever interrupting a run; the version check catches a wrong or damaged program before anything changes. | [UPDATES.md](UPDATES.md#windows-and-linux) |
| D-049 | 2026-11-07 | Cursor runs tasks like Claude and Codex. A run is `cursor-agent -p --output-format stream-json --trust --approve-mcps --force` (`--resume=<chat id>` for an answer), in the project folder. Cursor takes MCP servers only from its own `mcp.json`, so before a Cursor run Messenger keeps one entry, `messenger_task`, in `~/.cursor/mcp.json` (and in the agent's profile folder when it has one): `messenger task-mcp --from-env` with `AGENT_MESSENGER_TASK` passed through `${env:...}`. The run's database, run ID and receipt travel in that variable; outside a Messenger run the server lists no tools. Only that entry is written; a file that is not valid JSON is left alone and the run does not start. The chat ID is bound from the first run's `system/init` event, as for Codex. On Windows Messenger starts its node.exe with index.js instead of the `.cmd` script (D-052). | The owner asked to set Cursor up. Cursor has no command-line MCP option, and writing into the project's `.cursor` folder would change the owner's repository; one stable entry in Cursor's own settings changes nothing else. Event shapes and `--approve-mcps` come from the 2026-11-05 Mac research runs; `${env:...}` in `mcp.json` was not verified with the real CLI. | [TASK_MODEL.md](TASK_MODEL.md), Q-004 |
| D-050 | 2026-11-07 | The window connects the coordinator itself (T-039). Right after a project is created or its folder changes, and on «Подключить к Claude», the client runs Claude's own command `claude mcp add --scope local messenger -- <messenger> --root <data> coordinator --project <name>` in the project folder, with the registered Claude Code program and the arguments passed directly (no shell), so the call is the same on macOS, Windows and Linux. Messenger never writes Claude's settings itself; it only reads `~/.claude.json` (or `$CLAUDE_CONFIG_DIR/.claude.json`) to see whether the folder's `messenger` entry starts this Messenger for this project. It never replaces a `messenger` entry that starts something else. One attempt per action, bounded at 30 s; an unclear result is shown with Claude's message and the manual command, never retried. Every worker run carries `AGENT_MESSENGER_WORKER_RUN`; the coordinator server started with it lists no tools and does not open the database, so a worker that loads the project's coordinator entry cannot start or answer tasks. | The owner asked not to use the Terminal. Claude's own command keeps its settings format Claude's concern. | D-022, D-026, T-025 |
| D-051 | 2026-11-08 | «Проверить версию и вход» runs on Windows and Linux as on macOS, and asks for the login status for any recognized version, not only the one version first observed. Only the exact known answers count; anything else stays unknown. | On the owner's Windows computer the check showed nothing: it was limited to macOS and to one exact version. The login request only reads the CLI's own status. | D-050 |
| D-052 | 2026-11-08 | Cursor on Windows is started without its script. Its installer keeps each version in `%LOCALAPPDATA%\cursor-agent\versions\<version>\` as `node.exe` and `index.js`; `agent.cmd` only runs that pair through PowerShell. Messenger registers the `node.exe` of a version folder and, at every start and check, runs the newest version folder's `node.exe index.js …` (the same choice Cursor's script makes), with `CURSOR_INVOKED_AS=agent.cmd`. Any other path is started as it is. | D-044 forbids `.cmd` scripts; the owner's Windows computer showed this layout. | D-044, D-049 |
| D-053 | 2026-11-08 | «Выбрать…» next to the agent program, the profile folder and the project folder opens the system's own dialog: the local server runs it on the same computer (PowerShell's file dialog on Windows, `osascript` on macOS, `zenity` or `kdialog` on Linux) and returns the chosen path into the field. The start folder is passed in an environment variable, not in the script. The choice only fills the field; saving still goes through the usual checks. Without a dialog program the window says to paste the path. The Mac app keeps its native folder dialog for the project folder. | A page in a browser cannot learn a chosen file's full path; the owner asked for a button instead of typing paths. | D-044, D-045 |
| D-054 | 2026-11-08 | The window is a program window on Windows and Linux too. Windows: the page in the program's own window through WebView2, the engine Windows 10/11 ship with (library `github.com/jchv/go-webview2`, pure Go, no cgo); the page key goes to the engine, not to a command line; the console Windows opens for a double click is closed (a terminal the owner typed in is kept); its data in `<data>\window`. Linux: the first of Chrome, Chromium, Edge or Brave in app mode (`--app`, no tabs or address bar) with a profile of its own per window (a browser started with an open profile hands the page over and exits at once), given only the private forwarding file; plus an entry in the application menu. Closing the window closes the page; the client and tasks keep running. Without these the default browser, as before (D-045). | The owner did not want a browser tab and a console. A webview on Linux needs cgo and GTK libraries for every architecture; app mode gives a window now. | D-045 |
| D-055 | 2026-11-08 | The computer that started a task on another computer sees that it was accepted and the worker's progress lines (text and tool actions, up to 200 per run, 1000 characters each), sent in the same transaction as each line. They are for the window only: the coordinator gets no event for them, and nothing about verification changes. The window explains a known refusal in Russian. | The owner tested Mac → Windows and saw nothing on the Mac while Codex worked on Windows. Replaces the D-041 note that the worker's own lines stay on the running computer. | [TASK_MODEL.md](TASK_MODEL.md#tasks-between-computers-t-037) |
| D-056 | 2026-11-08 | A computer accepts tasks of other computers of its network unless its owner turns «Задачи других компьютеров» off. The owner names the computer for the network on the «Сервер» tab («Этот компьютер»); the name given when joining becomes that name, and the hosting computer no longer shows only the system's name. | The owner tested two computers: the default refusal was confusing, and the host could not be named. Every computer of a network joined with an invitation the owner created. Replaces Q-014's cautious default. | [BROKER.md](BROKER.md), [TASK_MODEL.md](TASK_MODEL.md#tasks-between-computers-t-037) |
| D-057 | 2026-11-08 | The agent card offers «Войти в аккаунт» when the check finds no login and «Установить …» when the program is not found. «Войти» opens a terminal window with the CLI's own login command (`claude auth login`, `codex login`, `cursor-agent login`) and the agent's profile variable; Messenger never sees the credentials. «Установить» opens the CLI's official install page; nothing is downloaded or run by Messenger. On Windows the window has no inner frame and its title bar follows the program's light or dark theme (DWM caption colour). Codex trust is read from the table, inline and dotted forms of `config.toml`, for the folder or its git root, case-insensitively on macOS and Windows. | The owner asked for these buttons, did not like the white title bar and the "window in a window" look on Windows, and saw «Не доверяет» for folders Codex trusts. | D-051, D-054 |
| D-058 | 2026-11-08 | The owner lists the model names each agent accepts, on the agent's card (one per line, up to 12). `list_agents` shows them, for the other computers of the network too (their presence report carries the list, left out when empty so an older host still accepts it). For Codex and Cursor `start_task` takes only a listed model and refuses any other name with the list in the reason; with no list it takes no model. Claude keeps its own names (opus, sonnet, haiku) and any full name. The computer that runs the task checks its own list. | The coordinator may not guess model names (an unknown name fails the run), and the owner wants it to choose models itself. Messenger cannot read the CLIs' model lists reliably here, so the owner writes them. | [TASK_MODEL.md](TASK_MODEL.md) |
| D-059 | 2026-11-08 | Messenger keeps the last usage-limit state each agent reported during a task run and gives it to the coordinator as a separate `limit` field of `list_agents` (state `ok`, `warning` or `exhausted`, `used_percent` when the agent gave a share, `window`, `resets_at`, `observed_at`), not as text in the report. Claude: its `rate_limit_event` (status and reset time; no share is taken from it until a real run shows its units). Codex: a `rate_limits` object with `primary`/`secondary` windows and `used_percent`, wherever an event carries it. A run that failed on its limit records `exhausted`. Cursor reports nothing usable yet. The card shows «Лимит»; other computers get it in the presence report (left out when there is none). It only informs: runs are judged and never retried as before. | The owner wants the coordinator to know each agent's limits, and the coordinator asked for a field it can read without guessing. Asking the agent for its limit would cost a model call; this reuses what the CLIs already print. | [TASK_MODEL.md](TASK_MODEL.md), Q-015 |
| D-060 | 2026-11-08 | Agent cards: the window checks the version and login of every known agent by itself when it opens (the same check as before, one at a time, no model call); each card has «Проверить», and a found but unconnected agent has «Подключить» (saves the found program and profile, then checks). The setup dialog has «Проверить», «Сохранить» (then checks) and «Отключить» (refused while the agent has an unfinished task; its tasks keep their own copy of the program). The card says «Выполняет задачи» instead of «Исполнитель». In dialogs the main action is on the right even when the other buttons are hidden; a busy restart shows its text in the dialog, not in the button. | The owner's feedback on preview.36: checking needed the dialog, statuses were empty at start, «Подключить исполнителем» was unclear, buttons were stretched or on the left. | D-057 |
| D-061 | 2026-11-08 | A computer's name in the network is the one set in its own window («Имя в сети», or the name given when joining), and it can be changed at any time: each presence report carries it, and the broker replaces the device's name with it, so every computer shows the new name within one report (15 s). A computer that joined before its window kept the joining name takes that name instead of reporting the system's. A host before preview.37 ignores the name and keeps the joining one. Replaces "a device keeps the name it joined with" of D-039. | Owner: the Linux laptop joined as «Ноут-Linux» but the list showed the system name; «чтобы название можно было сменить в любой момент и у всех поменялось». | [BROKER.md](BROKER.md) |
| D-062 | 2026-11-08 | Fewer tokens per task. (1) The owner writes «Общие указания исполнителям» once per project in the window; each new task copies them (a task keeps the rules it started with), they travel with a task sent to another computer (that computer's own rules are not used for it), and `read_task` returns them as `rules`. (2) The built-in worker instruction asks for a report of at most 8 lines with long details in a file. (3) `report` takes optional `changes`, `tests` and `files`; Messenger stores one text with labelled lines («Изменения:», «Тесты:», «Файлы:»), so it travels and shows like any report and older computers need nothing new. The coordinator is told not to ask for «taken» or limit lines. | The owner's coordinator: points 1 and 2 save tokens on every task. | [TASK_MODEL.md](TASK_MODEL.md) |
| D-063 | 2026-11-08 | `continue_task(task_id, op_id, text)`: the coordinator gives a finished task (done or failed) of this computer a follow-up, and a new run resumes the same recorded conversation (like an answer), with `read_task` returning `follow_up`. Kept simple at the owner's request: nothing is recovered after a restart or reboot; if the agent no longer has the conversation, the run fails like any other and nothing is retried. Cancelled, blocked or interrupted tasks are refused (unknown is not success); tasks of other computers are refused for now (start a new task). | Owner: «если правки пришли… сессия продолжает… не надо заморачиваться на супер восстановление». | [TASK_MODEL.md](TASK_MODEL.md) |
| D-064 | 2026-11-08 | The sidebar no longer marks a computer's agent as «координатор». There is no coordinator computer: any chat with Messenger's tools on any computer of the network can give tasks, so the mark only said that such a chat exists and read like a chosen main computer. The chat program is still listed among the computer's agents. Workers still cannot give tasks (D-050). Replaces the sidebar mark of D-024. | Owner: «как такового координатора у нас нет». | D-024, D-050 |
| D-065 | 2026-11-08 | «Разрешить любые команды» per agent of a computer, off by default, in the agent's settings. On: Claude runs with `--permission-mode bypassPermissions`, Codex with `--dangerously-bypass-approvals-and-sandbox` (no sandbox, network open), Cursor with `--force --sandbox disabled`. Each new task copies the switch; a started task keeps what it started with. A task from another computer runs with the switch of the computer that runs it. The card shows «Команды: Любые»; `list_agents` shows `full_access` for this computer's agents only (presence does not carry it, so an older host keeps accepting reports). The test profile never runs with it. | The owner's workers could not run `cm`, Python or commit scripts: Claude asks for each command and nobody can answer in a background run, Codex's sandbox closes the network. The owner chose full access over a list of allowed commands. | D-023, D-028 |
| D-066 | 2026-11-08 | A project can be removed from the window («Удалить проект» in the project dialog, two clicks). Removal is refused while any task of the project is unfinished (running, waiting, blocked, interrupted) or has a run that is not closed. The project row is marked removed; its folders and coordinator mark are dropped; its tasks, runs and events stay in the database. A removed project refuses new tasks, also from other computers. Creating a project with the same name brings it back with its history. | Owner: remove projects that are no longer needed. Nothing that may still run or hold a reservation is hidden, and no history is deleted. | D-026, AGENTS.md rules |
| D-067 | 2026-11-08 | «Любые команды» is one switch on the Agents tab for every agent of this computer; the per-agent switch is gone from the agent dialog (the stored per-agent value stays, and the switch shows «разрешены не всем» when they differ). Each computer sets its own; one computer cannot open commands on another. An answer or a follow-up (`continue_task`) runs with the agent's current setting, not the one its task started with; a new run is the owner's or coordinator's request made now. Replaces "a started task keeps what it started with" of D-065 for those runs. The agent card shows «Войти» whenever no login is confirmed (never «Не проверен») and «Выйти» after a confirmed login; «Выйти» asks in a popup and opens the CLI's own sign-out (`claude auth logout`, `codex logout`, `cursor-agent logout`) in a terminal. An agent connected with its standard profile is checked against its standard profile folder. The window checks an agent again when the owner returns to it after «Войти» or «Выйти». The card button is «Настройки»; in the dialog each status stays on its button's line (check result next to «Проверить», save result next to «Сохранить»), so nothing moves. A click on a computer in the sidebar shows only the tasks that computer runs, with a chip to clear it. An agent's chat messages carry its icon in the name line and start at the pane's edge. A run that fails without a report keeps the worker's last message (up to 300 characters) in its reason, for the window and the coordinator. Later the same day the owner asked: the switch is a small on/off slider (one switch style for the window); a newly connected worker agent has «Любые команды» on (saving it again keeps the owner's choice; the test profile stays off); the agent dialog saves each change by itself (no «Сохранить»); an available update is just the button «Обновить», with no line above it. | Owner: rights turned on did not help (tasks they answered or continued kept the old setting); opening rights per agent per computer was tedious; «Не проверен» without a button; the settings looked messy. | D-057, D-060, D-065 |
| D-068 | 2026-11-08 | Each task shows what its runs spent, as the agents reported it: Claude's price (`total_cost_usd`, its own estimate, which for a subscription is not money actually billed) and tokens; Codex and Cursor tokens only, with no price invented from a rate table. Statuses are short, with commas and no dots between words (owner): «Работает 3 мин», «Готово за 3 мин, $0.42», «Ошибка через 2 мин, 12 тыс. токенов», «<1 мин» under a minute. Dollars use a decimal point. The price, or the total tokens when no price was reported, follows on the same line in the row and in the task header; the tooltip shows input (with the cached part) and output tokens. Nothing is shown when no run reported usage. Usage is added per run when the run is finished, whatever its outcome; it never changes a run's result. A task run on another computer sends its total with each event, and the starting computer keeps the latest total; a computer before this change ignores it. Cursor's usage block is not documented; it is read when present in camel case (`inputTokens`, `outputTokens`, `cacheReadTokens`, `cacheWriteTokens`). | Owner: «интересно было бы, чтобы в задачах в списке мы видели, сколько потрачено». | D-021 |
| D-069 | 2026-11-08 | In the sidebar a blue dot means the agent is running a task, nothing else. A chat program that used Messenger's tools (the coordinator connection) is still listed (D-064) but with the idle dot: the connection is recorded once and never expires, and an idle chat sends nothing, so Messenger cannot tell whether it is still open. | Owner: Claude on Ноут-Linux was blue though nothing ran there. | D-064 |
| D-070 | 2026-11-08 | Agent cards on the Agents tab follow the owner-approved mockup [docs/mockups/agent-cards.png](mockups/agent-cards.png) ([HTML](mockups/agent-cards.html)): the agent's icon and name with two icon buttons on the right (check again, settings gear); four rows of the same height: «{Agent} CLI установлен» (version; «Удалить», or «Установить» when missing; «{Agent} CLI неисправен» with a «?» tooltip when a task proved a file missing), «Вход выполнен» («Выйти»/«Войти»), «Работает с Messenger» (set only by a task's outcome), and «Лимит N%» with a ring coloured by level (green below 60%, yellow to 85%, red above) and the reset time and today's tokens under it. Steps that cannot be checked yet are grey and numbered. Under the rows one button «Задачи: N идёт, M сегодня». All card buttons are the same grey; no dots between words; one line per explanation with the full text on hover; no dollars. Install and remove open the CLI's own official command in a terminal after a confirmation. Not built yet. | Owner: «вот этот макет утверждаю». | D-067 |
| D-071 | 2026-11-08 | The current project is set up on its own tab «Проект» (name, folder, instructions for workers, save, remove). «Настроить проект…» leaves the project list; «Новый проект» stays there and opens the dialog. The tab has no save button (owner): the instructions save shortly after typing stops, the folder when its field is left, Enter is pressed or a folder is picked, so a half-typed path is not stored. | Owner: a settings item inside the project list looked out of place; chose the tab from three drafts. | D-066 |
| D-072 | 2026-11-08 | Renaming a project changes only the name shown on this computer. The name it was created with stays its key for tasks, the coordinator chat and other computers; the «Проект» tab shows it under the new name. Names shown in the list stay unique, case-insensitively. | Owner asked to rename projects; a full rename would break tasks sent to other computers and the coordinator connection. Q-019 asks whether to go further. | D-071 |
| D-073 | 2026-11-08 | The window names a build the usual short way: «v0.41» for 0.0.0-preview.41, with «alpha» and the exact tag in its tooltip. The release tag and the version the updaters compare stay 0.0.0-preview.N; only the shown name changed. An available update is a blue round arrow next to the version, not a button. | Owner: «Предварительная версия» is too long; chose the icon (variant А). Changing the tag scheme itself would touch Sparkle and the self-updater, so it is left as a question (Q-020). | D-048 |
| D-074 | 2026-11-08 | No dollars in the window: a task shows tokens only, in the row, the header and the tooltip. The price an agent reports is still stored with the task, unused. | Owner: «Расход в долларах не надо. Мы же не знаем его… Мы сидим на лимитах.» | D-068 |
| D-075 | 2026-11-08 | The switch on an agent card turns the agent off without forgetting it: its program, profile, models and «Любые команды» stay; it takes no new tasks or follow-ups (the client refuses them), and it is hidden from the coordinator's `list_agents` and from the other computers; tasks already running go on. An agent the window has not connected yet is connected by turning the switch on; one that is not installed has the switch disabled. «Удалить» on the program step opens the agent's official page after a confirmation instead of running a removal command (Q-022). | Owner approved the per-card switch in place of «Отключить» in the dialog. | D-070 |
| D-076 | 2026-11-08 | On Windows the window has no system title bar: Messenger takes the caption away (its own handling of `WM_NCCALCSIZE`) and keeps the system's side and bottom borders, so resizing, snapping and the shadow stay as Windows draws them; a maximised window starts below the screen edge by the frame's width. The page draws the three window buttons at 46×32 px like the system's, the close button red on hover, and drags the window through the system's own caption drag (`WM_NCLBUTTONDOWN`). If the bridge to the page is missing, nothing changes. | Owner: the sidebar should run to the top edge as in other desktop apps; draft approved by the owner's instruction to build the design items first. | D-054 |

## D-077 — Token statistics from the tasks table (T-068)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

The «Статистика» tab sums `tasks.usage_*` on request; no new table, no background job. A task counts on the day it finished (or was last updated while it still runs), in the window computer's local days. Tokens are input plus output; cache is shown as part of the input. Periods: today, 7 days, 30 days. Bars use one neutral ink so none reads as a status. The server does not keep or share totals yet: that needs a protocol change and T-047 (shared task list) will bring the other computers' tasks anyway.

## D-078 — Linux window from WebKitGTK loaded at run time (T-069)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple; owner asked for a real Linux window, Q-016).

`messenger open` on Linux loads `libgtk-3.so.0` and `libwebkit2gtk-4.1.so.0` (or `-4.0.so.37`) with `purego` and shows the page in a GTK window. No cgo and no second program: the release stays one static file per architecture, self-update is unchanged, and a system without the libraries falls back to D-054 (Chromium-type browser in app mode, then the default browser). The page gets `window.amDesktop = "linux"` (no inner frame) and `window.amTitleBar`, which sets GTK's dark-theme preference like the Windows title bar. Rejected: a cgo build (would not start without the libraries and needs native ARM64 builders) and a separate helper program (two files to ship and update).

## D-079 — Window language through one dictionary (T-059)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

The Russian texts stay in the code as they are and are the dictionary's keys: app.js wraps each of its texts in `tr('…')`, `assets/i18n.js` maps them to English and translates the static texts of index.html once at load. A test fails when a text has no English entry. Text the owner, agents or the client write is never translated. The choice («Язык» in the sidebar) is kept in the window's local storage like the theme and applies by reloading the page. Numbers and dates follow the language. Messages produced by the Go client (errors, check details), the CLI and the Mac shell stay Russian for now.

## D-080 — Shared task lists as one snapshot per computer (T-047)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

Every computer posts the tasks it started in the last 7 days (at most 50, title up to 200 characters, the first 800 characters of the report, tokens) to the broker's new `POST /v1/tasks` about every 15 seconds and gets the latest lists of the other computers in the answer. The broker keeps only each computer's latest list, like presence, and drops revoked computers. The task mailbox is not used, so its limits stay free for real task messages. A separate endpoint keeps older brokers working: they answer 404 and the window simply shows no lists. A task that another computer started and that runs here is shared only by its origin, and the window hides another computer's task that already has a mirror here, so no task shows twice. The lists are read-only («На компьютере «…»» in the task list); answering or cancelling stays with the computer that runs the task. Leaving or changing the network drops the stored lists.

## D-081 — Short invitation: local discovery and SPAKE2 (T-052, Q-017 a)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

Every invitation also gets a 6-digit code that lives 10 minutes and allows 3 tries; the long code stays for other networks. The joining computer broadcasts a fixed question on UDP port 47100; a host answers with its port only while it has a usable short code. The answer is not trusted. Both sides then run SPAKE2 over edwards25519 (`filippo.io/edwards25519`, the curve library the Go standard library itself uses; M and N are derived from fixed strings) through `POST /v1/pair/start` and `/v1/pair/finish`. The transcript includes the certificate fingerprint the joining computer saw, so a computer in the middle cannot complete it. Every started exchange counts as a try. Only after both confirmations does the host send the long invitation's secret, encrypted with the agreed key (AES-GCM); then the usual registration follows and the certificate is pinned as before. A listener learns nothing to test codes against; an active attacker gets one guess per try, three in all. A wrong code is not retried on another host. If the discovery port is taken, only the long code works.

## D-082 — Windows installs itself for the current user (T-053)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

A versioned download started with a double click copies itself to `%LOCALAPPDATA%\Programs\Agent Messenger\AgentMessenger.exe`, adds a Start menu shortcut (written by PowerShell's WScript.Shell, no new dependency) and a per-user entry in «Приложения» (`HKCU\…\Uninstall\AgentMessenger`, no administrator rights), and opens the installed copy. An equal or newer installed version is opened instead of being replaced; an installed copy that is running is moved aside the way updates do. Any failure leaves the program working from where it is. `install` does the same without opening the window. «Удалить» runs `AgentMessenger.exe uninstall`: it refuses while `active-runs.json` holds any reservation, stops the background client, removes the entry and the shortcut under the installation lease, and removes the program folder a few seconds after it exits. The data folder is never deleted; the owner can delete it by hand (the cautious reading of "only if the owner explicitly asks", Q-023).

## D-083 — Another computer's log through the mailbox, off by default (T-057, Q-018)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

The coordinator's `get_log(computer, lines)` asks another computer of the network for the end of its Messenger log. The request (`log_request`) and the reply (`log`) travel through the task mailbox, so no new broker endpoint is needed and an older computer simply never answers (the tool then says so). The answering computer replies only when its owner turned on «Отдавать журнал по сети» on the «Сервер» tab, off by default; otherwise it replies with a refusal. It sends the last lines (200 by default, at most 1000) of its newest background log, at most 64 KB, with session ids (UUIDs), invitation codes, long hex strings, values after words such as token, password or authorization, and long letter-and-digit tokens replaced. Paths and task texts stay. Only the coordinator, which the owner connected to this computer, can ask; nothing is shown in the window. The tool waits up to 20 seconds and otherwise returns a request id to ask again with; answers are kept for a day.

## D-084 — The coordinator sees a task's last action and tokens (T-071, T-049)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

`tasks` adds, for an open task, the worker's last action (its last text, or «Tool: input» for a tool call, at most 300 characters) and when it happened, and, for every task, the tokens used so far. It reads only what the window already keeps: the worker's event items on this computer, or the progress lines the running computer forwards (D-055) for another computer's task. No new message and no new storage. The instructions tell the coordinator that a long quiet time is a reason to tell the owner, never to cancel or restart a task by itself.

## D-085 — Readiness travels with presence (T-070, T-049)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

Every presence report (every 15 seconds) now carries `ready`: whether the computer takes tasks from other computers and the names of the projects that have a folder on it (at most 64). `list_agents` shows another computer's `accepts_tasks` and `has_project`, and the note says when a task there would fail; a computer that did not report them stays "unknown" as before. A host before this version refuses a report with an unknown field, so a refused report is sent once more without `ready`, and `ready` is left out for 10 minutes before it is tried again. A presence report is a status, not an action, so sending it again cannot start or repeat anything. Environment checks (tools such as Unity or Python) are left to the workers.

## D-086 — Resource locks keep the later task waiting (T-072, T-049)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

`start_task` takes an optional `lock`, a short name such as `unity-editor`, compared without case and extra spaces. On one computer, a task with a lock does not launch while an earlier task with the same lock is open there (not done, failed or cancelled); the earliest open one goes first, so two waiting tasks never hold each other. Nothing is refused and nothing is retried: the later task's launch intent simply is not picked until the lock is free. A blocked task keeps its lock, because whether its worker still uses the resource is unknown; the owner resolves it. The lock travels with a task sent to another computer and is applied there; a computer before this version ignores it. The coordinator sees `waiting_for_lock` with the task it waits for, the window «Ждёт ресурс «…»». Locks are per computer only; there is no network-wide lock.

## D-087 — A task's result travels as its report and through version control (T-073, T-049)

2026-11-08, Claude Code (owner asleep, asked to decide alone and keep it simple).

No new channel for result files. A worker's report (up to 20 000 characters) already comes back whole from another computer in the task's events, and the coordinator reads it through `wait`. Files, diffs and builds stay where the project keeps them: the worker commits or checks them in to the project's version control, or writes them to a folder the owner shares, and names them in the report's «Изменения:» and «Файлы:» lines. Copying files between computers through the broker would need size limits, storage and the owner's consent for each project, and would duplicate what version control already does.

## D-088 — The coordinator catches up after the owner interrupts its wait (T-074)

2026-11-09, Claude Code, from the owner's report: while the coordinator chat waits it does not read the owner; after the owner interrupts, it answers the owner and forgets the workers' questions that came meanwhile.

The coordinator server remembers the highest cursor the chat reached through `wait` (only events actually sent count). Every other tool's answer gets a second text when newer events exist: their count and the cursor to pass to `wait`. The note never moves the cursor, so nothing is skipped. The instructions tell the chat to call `wait` with its last cursor and a 1-second timeout whenever it wakes because of the owner, before answering. `wait` now lasts 120 s by default and at most 300 s instead of 600 s and 3600 s, so a message the owner sends during a wait reaches the chat when the wait returns; an empty wait costs one small call and keeps the chat's prompt cache warm. A hook in the owner's Claude settings would make the check certain, but it changes files Messenger does not own; not done.

## D-089 — A worker commits, or asks and stays open (T-075)

2026-11-09, Claude Code, from the owner's report: two workers on another computer could not commit, the coordinator asked one for its whole report, a cut piece came back, the task was finished and could not be continued there.

The owner's rule: work reaches the project only through the worker's own commit, always; a text copy is no substitute. The worker's first message now says: commit (or check in) the work before reporting; when the commit fails (files locked by another agent, the copy behind the server) and the worker cannot fix it within its task, use `ask` instead of `report`, so the conversation stays open, and try again when the answer comes. The coordinator's instructions say a report is short (at most 20 000 characters reach it), never to ask for the full text of an audit instead of a commit, and how to handle such a question: when another task holds the files, answer later, after that task is done, telling the worker to update and commit; otherwise tell it what to do or tell the owner; never let it finish without committing. An open question has no time limit. Continuing a finished task of another computer stays out of scope.

## D-090 — Statistics of the whole network, sent only when it changes (T-076)

2026-11-09, Claude Code, with the owner: statistics should be seen on any computer, and a click on a computer in the sidebar should filter statistics and agents as it filters tasks. Usage must not load the network.

Each computer adds to its presence report its token usage of the last 31 local days, summed by day and agent, for the tasks that ran on it (started there or received from another computer); no titles or texts. The broker client sends it only when the sums changed since the broker last accepted them, and repeats it every 10 minutes in case the host lost it; a report without usage keeps the stored one. The broker names each version with a hash and, in its answer, leaves out the days of every computer whose version the asking computer already holds (`usage_seen`). A host before T-076 refuses the new fields; the client then reports without them for 10 minutes, as with readiness.

The window sums this computer's own tasks and the others' reports; a task this computer sent to another one counts on the computer that reported it, so nothing is counted twice. The costliest tasks list only tasks this computer knows. In the sidebar, a click on a computer filters the statistics or the agents tab when one of them is open, and opens the tasks otherwise. Another computer's agents are view only: status, last limit and models; they are set up on that computer. Limits are not shown in the sidebar (owner).

## D-091 — The window restarts the old client by itself (T-077)

2026-11-09, Claude Code, from the owner: after every update the window asked to restart the background client, and the answer was always yes.

The window now asks the client to restart once, without the owner, as soon as it sees an older client. The client keeps its old rule: without confirmation it refuses while task runs are going or their count cannot be read, because a restart interrupts them and they are never repeated by themselves. Only then the window shows the question as before. While restarting it shows «Обновляем Messenger». A failed restart is shown and not retried automatically.

## D-092 — A start screen instead of the restart popup (T-079)

2026-11-09, Claude Code, with the owner, who chose it from three mockups: a thin bar, a start screen, a bar that waits for tasks.

Until the background part first answers, and while it installs an update, the whole window is covered by the app's name, a spinner and one status line, as applications do while they start. The window first learns the client's version, so an update starts behind the screen and the old window never flashes. If nothing answers within 20 seconds, the screen goes and the usual notice says what is wrong. When tasks are running, the question from D-091 stays: a restart would interrupt them. Waiting for the tasks to finish instead was offered and not chosen.

## D-093 — Show which limit, keep what the agent said about it (T-080)

2026-11-09, Claude Code, after the owner saw no limits on the Mac and only «близко к пределу» without a percent on the other computers.

A limit is recorded only from a worker task that ran on that computer; the coordinator's own chat is not a task run, so a computer that only coordinates shows nothing. Claude's stream gives a state, a window and a reset time; a share used was not taken from it because its units are not known. The cards now name the window and the reset, and the empty card says where the limit comes from. The plain fields (text, number, yes/no) of the agent's limit report are kept, at most 480 characters, and the coordinator sees them in `list_agents` as `limit.reported`. After a real run they tell whether a share is sent and in which units; only then is a percent shown. Nothing acts on a limit automatically.

## D-094 — The coordinator is warned about a limit, not stopped by it (T-081)

2026-11-09, Claude Code, after the owner asked that the coordinator use the limits it already sees when choosing where to send a task.

`list_agents` already gave every computer's last limit. Now `start_task` also answers with `limit_note` when the chosen agent last reported a limit close or exhausted and its reset time has not passed, and `list_agents` marks `reset_passed` when the reset time is behind. The task is still started: the state is what the agent said at the end of its last run, it may be stale, and refusing on it would be acting automatically on unclear evidence. The coordinator tells the owner and may choose another agent or computer.

## D-095 — The page runs to the top edge on every system (T-082)

2026-11-09, Claude Code, after the owner found the system title bar still on the Mac and asked that the program look the same everywhere.

Mac: the window keeps its title bar but makes it transparent and hides the title, so the page runs under it. The red, yellow and green buttons stay the system's own at the top left, where Mac users expect them; the sidebar leaves room for them. Web content takes every click, so the page sends the app the rectangles of its empty top (the tab row up to the edge, the sidebar above its first item) and of the controls in them; the app moves the window on a press there and follows the system's double-click setting. Linux: GTK gets an empty, zero-height title bar, which keeps GTK's own border for resizing, and the page draws the Windows-style buttons and asks GTK to move, maximise, minimise or close. Rejected: Windows-style buttons on the Mac (foreign there, and full screen would be lost).

## D-096 — The owner's message to a working worker waits for its current run (T-085)

2026-11-09, Claude Code, owner's answer (a) to Q-026.

While a task of this computer is running or waits for the coordinator, the window's box writes to the worker. The note is kept (`task_owner_notes`, idempotent by operation id) and the coordinator gets an `owner_note` event. When the current run ends: with a question, the note travels with the answer; with a report, the report is held (shown as an interim report, not sent to the coordinator) and the task continues at once in the same conversation with a `note` run, whose `read_task` gives the notes and the held report. The next run of any kind takes every waiting note. A task that stops (done, failed, cancelled, blocked, interrupted) marks its waiting notes undelivered; nothing sends them again by itself. Refused: tasks between computers (the message would have to cross the network; later), a question to the owner (answer it instead). Rejected for now: interrupting the run (b) and live delivery (c).

## D-097 — Tasks of other computers run without «Любые команды» by default (T-094)

2026-11-09, Claude Code, after the security review: with «Любые команды» on (D-067) and tasks of the network accepted (D-056), any computer of the network, or anyone holding its device key, could run any command on every other computer. Now a task from another computer runs with the agent's own permission checks even when the agent has «Любые команды» here; the owner can allow it with «Любые команды для задач других компьютеров» on «Сервер» (off by default). Answers and follow-ups of such a task follow the same rule. Accepting tasks of the network stays on (D-056): a task with permission checks still does useful work. Cursor without «Любые команды» now runs with `--sandbox enabled` instead of the owner's own Cursor setting, so such a Cursor worker may be unable to commit; the owner can allow full access per computer.

## D-098 — The worker's last message is kept whole and readable by the coordinator (T-087)

2026-11-09, Claude Code, after a Codex audit on ПК-2 came back as a six-line report while its full text was only in the worker's last message, and the file it had written was deleted. Messenger now keeps each run's last worker message in full (up to 60 000 characters) on the computer that ran it; a task of another computer sends it back before its report (an `output` message; older computers ignore it). When it says more than the report, the report or failure tells the coordinator so, and the coordinator reads it with `task_output` in parts of 20 000 characters. Reports stay short (D-087); this only makes sure nothing the worker wrote is lost.

## D-099 — The newest coordinator chat that acts is the coordinator (T-090)

2026-11-09, Claude Code, coordinator's list item 4. Each coordinator chat gets its own id. The newest chat that waits, starts, answers, continues or cancels becomes the project's coordinator; the wait of the chat before ends at once with `superseded`, and that chat is told to stop and tell the owner. The new chat is told it replaced another one. Nothing is refused: refusing would leave the owner stuck when the old chat is gone without a goodbye; replacing matches the owner's plan of a fresh chat each day and night.

## D-100 — Updating another computer from the window (T-078)

2026-11-09, Claude Code, owner's answer (a) to Q-025.

The right-click menu of another computer in the sidebar has «Обновить программу» while that computer is on line. The request and its answers travel through the broker's mailbox like the log request (T-057). The other computer refuses while any of its tasks is not finished or has a run that is not closed, because the update restarts its background client; it checks again right before installing. Partly superseded by D-133: live work queues the update instead of refusing. It installs only the newest release of the signed update channel (Ed25519 signature, checksum and the program's own `version` check, D-048), with the same updater as its window. A Mac refuses and says to update it in its own app: Sparkle updates the app bundle, which a background client cannot replace. The background client runs from a copy, so the window records the program file the owner installed (`program.json`, 0600); without it the request is refused. A helper started from that program installs the update, restarts the client from the installed program with the old client's shutdown command (a pause the owner set stays) and writes its result, which the new client sends. Nothing is retried on its own: a helper that did not start or never reported is answered as failed with the reason, and the owner decides. There is no switch to allow it, unlike the log or tasks of other computers: the network holds only the owner's own computers, a request can only install the signed release that computer would offer anyway, and it never runs while tasks do. Every computer reports its program version to the network; the cards show it. An older computer reports none and would never answer, so it gets no menu item and a request to it is refused: update it by hand once.


## D-101 — Starting model lists for Codex and Cursor (T-098)

2026-11-09, Claude Code, owner's request.

A newly connected agent starts with a model list, so the coordinator can choose a model without guessing:

- Claude: `opus`, `sonnet`, `haiku`. These are Claude Code's own short names; each always means the newest model of that line, so the list does not age. Strongest first.
- Codex: `gpt-6-luna`, `gpt-6.1-terra`, `gpt-6.1-sol`, `gpt-6-astra` (recommended to the owner by Codex).
- Cursor: `composer-2.5-fast`, `grok-4.7-high-fast`, `claude-opus-5-thinking-high` (the owner's choice).

A list the owner already saved is never replaced. For an agent connected earlier whose list is still empty, the settings dialog shows these names and they are saved when the owner presses Save; nothing changes without that. Messenger does not check the Codex and Cursor names against the CLIs: an unknown name fails the run, and the owner edits the list on the card. Which model each CLI uses when the task names none is the CLI's own setting; Messenger does not know it. Follows D-058.

Agents connected before this change kept an empty list; the next version fills every empty list once when it first opens the data (T-109, a `meta` key marks it done), so a list the owner empties afterwards stays empty.

## D-102 — The coordinator sees questions to the owner (T-099)

2026-11-09, Claude Code, coordinator's list item 13, passed on by the owner.

A worker's question to the owner (`ask_owner`) and the owner's answer also reach the coordinator, as `owner_question` and `owner_answered` events of `wait`. They are for reading: the question stays the owner's, and `answer` from the coordinator is still refused for it. Before, the coordinator saw only «waiting for an answer» and could not plan around it. The copy is written where the task is managed: for a task run on another computer, on the computer that started it.

## D-103 — Update checks within GitHub's limit (T-103)

2026-11-09, Claude Code, owner's agreement.

GitHub answers 60 API requests an hour per address without an account, and all of the owner's computers reach it through one address. A check made three API requests (the release list and two files) every 5 minutes on each computer, so the limit ran out and even a manual check failed. Now only the release list uses the API; the release files are downloaded from the release's own download address, which must be exactly `https://github.com/kolotiloff/agent-messenger/releases/download/<tag>/<name>`. Checks run every 30 minutes. When GitHub refuses for its limit, the window says so and when to try again; nothing retries on its own beyond the usual next check. Signature and checksum checks are unchanged.

## D-105 — Short version for people (T-104)

2026-11-09, Claude Code, owner's request.

People see «0.58» instead of «0.0.0-preview.58»: in the release title, the Mac app's short version (About and update dialog), and the window. The full version stays in the release tag, the signed manifests and the program's `version`, because installed programs check exactly that form before they accept an update; switching the inner form needs one release that accepts both forms, and every computer on it, before releases use the new form.

## D-106 — Updates of a Linux installation while the client runs (T-107)

2026-11-09, Claude Code, owner's report: on Linux the update failed with «client is already running».

An update of a program installed with `messenger install` (window or another computer's request) adds the new version folder and moves the `messenger` link without the supervisor and startup leases. The running background client keeps its own version folder, which is never changed or removed, so nothing it uses moves; it goes on with its version until it is restarted, as on Windows (D-048). A lock file in the installation's library folder keeps two installs apart. A manual `messenger install` keeps the leases and the check of `active-runs.json`.

## D-107 — The coordinator follows updates (T-117)

2026-11-09, Claude Code, coordinator's report: its chat had only six tools although `continue_task`, `task_output` and `get_log` were released.

The coordinator's entry in Claude started the program the client ran from, which is the client's copy of one version (`runtimes/<digest>/messenger`). Copies stay on disk, so the chat kept that version through every update; the window never said so because the chat was connected. Now:

- The entry starts the program file the owner installed (recorded by the window on every system, `program.json`), or the `bin/messenger` link of an installation, which updates replace.
- An entry of this Messenger (same data folder and project) that starts another file is replaced once per client start, with Claude's own `mcp remove` and `mcp add`. Entries of anything else are never touched. A failure is shown, not retried.
- `messenger coordinator` is a relay: it runs `coordinator-serve` from that file and passes MCP lines. When the file changes and the new program answers `version`, the relay initializes it with the chat's own initialize request and the same chat id, sends new calls there, lets running calls finish in the old one and tells Claude `notifications/tools/list_changed`. A new program that fails is dropped and the old one keeps serving. A chat open before this version still runs its old copy: it needs one reconnect or a new chat, after which it follows updates by itself.

## D-108 — Continue an interrupted task after checking its worker is gone (T-118, Q-029)

2026-11-09, owner's answer to Q-029 («попробуй проверить»).

An interrupted run (the client restarted while it ran) was a dead end: nobody saw its worker end, so by D-021 the task stayed as it was. Now `continue_task` may continue an interrupted task of this computer. It is never automatic: only the coordinator's explicit call. Messenger first checks, for every interrupted run of the task, that its worker is certainly gone: the computer restarted since (another boot id), or neither the recorded process (same id and start time) nor any process of its group is running. If the process was never recorded, the snapshot fails or anything still runs, the call is refused and the task stays as it was. When the check passes, those runs are closed as failed («closed for a follow-up») and the same conversation goes on with the coordinator's text. Their reservations stay where they are; `active-runs.json` is not touched. A task of another computer is not covered.

## D-109 — Other computers' tasks take «Любые команды» by default; the switch is on «Агенты» (T-122)

2026-11-09, Claude Code, owner's decision. Changes the default of D-097. The owner found two switches in two places confusing: «Любые команды» on «Агенты» and «Любые команды для задач других компьютеров» on «Сервер». Both are about what agents may do, so the second one is now under the first on «Агенты» («И для задач с других компьютеров»). It is on by default: a new computer, set up by someone who will not look for it, works as it is, and the network holds only the owner's own computers. A computer where the owner turned it off keeps that. With it on, any computer of the network, or anyone holding its device key, can run any command on a computer whose agents have «Любые команды»; the owner accepted that.

## D-110 — The owner's message answers a question to the coordinator (T-125)

2026-11-09, Claude Code, owner's request («Разве автор не главнее»). A worker that asked the coordinator has ended its run and waits; an owner note (T-085) used to wait for the coordinator's answer too. Now the owner's message to such a task answers its open question at once, as the owner, and the same conversation continues. The coordinator gets an owner_answered event for that question and its own answer is refused as already given. Pending notes travel with that answer as before. Tasks of other computers still take no notes.

## D-111 — A computer hosts its network or joins another one, not both (T-127)

2026-11-09, Claude Code, owner's request («Тут либо создать, либо подключиться»). A computer that hosted a server and joined another one sent tasks through its own while the owner watched the other. Now «Запустить сервер» is refused while connected to another server, and «Подключить» while the own server is on (also when it failed to start); the window shows only how to switch. A setup that already has both keeps working as before and the window says so. «Отключить» also removes the device from the server it leaves; when that server cannot be reached the device stays there, but this computer remembers it and does not list it. A broker leaves an older device out of its list when a newer one with the same name reported later: that is the same computer that joined again. It is only hidden, never revoked, so a different computer that happens to have the same name shows again as soon as it reports.

## D-112 — The coordinator may answer a question to the owner (T-130)

2026-11-10, Claude Code, owner's decision («не надо запрещать»). Changes T-099, where a question to the owner was the owner's alone. The owner often answers in the coordinator's chat or asks the coordinator to answer for him, and the worker then waited for the window. The coordinator's `answer` is now taken for such a question too; the instructions tell it to answer only with the owner's words or on the owner's request. The window shows the answer as the coordinator's, so the owner sees what was said for him.

## D-113 — Continuing another computer's finished task (T-113)

2026-11-10, Claude Code, owner's wish that the coordinator can pick an old task up again. `continue_task` now works for a done or failed task that ran on another computer of the same network: a new `continue` message asks that computer to resume the same worker conversation. To avoid a task that looks busy forever, the follow-up is refused before sending when that computer is not in this network any more or reports a version older than 0.60 (which would drop the message). If that computer cannot continue it (no conversation, paused agent, cancelled task), it answers `continue_refused` and the task goes back to done or failed with the reason; a follow-up the server could not deliver does the same. An interrupted run there is not closed this way: D-108 needs a check of the worker's process on that computer. Tasks of an earlier network cannot be continued (the computer IDs are new); the coordinator starts a new task that names what is already done.

## D-114 — Stopping, closing and removing tasks (T-131)

2026-11-10, Claude Code, owner's request: stuck tasks could not be stopped or removed, and «остановить» and «удалить из списка» must not be confused. Stop (the square button) ends work that can still be reached. An own task on another computer that is out of reach, blocked or interrupted has no input and no stop button, since nothing would arrive: a line says since when there is no contact and that the outcome there is unknown, with one button «Закрыть» (after a confirm). It closes the task on this computer only; the cancellation is sent if contact returns, and later reports for it are only logged. Nothing local is touched, so D-021 does not apply; local blocked or interrupted tasks still follow D-021 and D-108. «Удалить из списка» is in the right-click menu of a task (no button in the header) and only hides a task that no longer runs: an own finished task leaves the window and the shared list, so other computers drop it with the next list; another computer's task leaves only this window (on its own computer it is removed for everyone). A running task is stopped first. The task stays in the database. Buttons: in dialogs the leaving or destructive choice is on the left and the main action on the right; in headers and cards, action icons sit on the right; a dialog closes with a cross at the top right, never with a «Закрыть» button.

## D-115 — Tasks follow a computer into a new network (T-132)

2026-11-10, Claude Code, after the owner moved the server from ПК-1 to ПК-2 and tasks stayed lost on one side and waiting on the other. A network gives every computer a new peer id, so a task cannot be bound to peer ids alone. Every computer keeps a lasting random id (meta `computer_id`), reports it with its presence and puts it on every message. Tasks store the other side's id (`remote_computer`, `origin_computer`). When that computer shows up fresh in a new network, or a message from it arrives, the task's links move to its new network and peer id, and messages still queued for the old ids go there. A start is never re-sent through the new network: if the old network delivered it, a second start would run a second worker (D-021: unknown is not success). Answers, cancels and reports are re-sent; the receiver applies each message once by its op id, under any network. A host before 0.61 refuses the unknown presence field, so the client retries without it. Tasks from networks before 0.61 have no computer id and are closed by hand (D-114).

## D-116 — Refused answers, unaccepted tasks and idle computers (T-134)

2026-11-10, Claude Code, coordinator list rows 24, 26, 30, 31. A refused answer no longer blocks a task when the other computer says its question is still open (or, before 0.61, refused it as «for the owner»): the question opens again and the task waits for another answer. Nothing is re-sent on its own; the coordinator or the owner answers again. An answer refused for any other reason (already answered there, no longer open) still blocks the task (D-021: unknown is not success). A task not accepted within 5 minutes is reported once to the coordinator with the likely reason; it is not retried or cancelled by the program. Idle computers and an out-of-date network list are told to the coordinator in the results it already reads (`wait`, `list_agents`), not as new stored events, so the event stream stays the same for existing chats.

## D-117 — Feedback journal on each computer (T-135)

2026-11-10, Claude Code, owner's request: a way for the owner, a friend using the program, and the coordinator to note what is wrong, without building a server. Remarks stay in each computer's database (table `feedback`) with the program version; nothing is sent anywhere by the program. They leave the computer only as one zip file the owner exports from the window or asks the coordinator for (`export_feedback`), written to the Downloads folder (or the data directory). The coordinator is told to record any Messenger problem with `report_feedback` at once, without being asked. Pictures are scaled in the window to at most 1600 px JPEG, under the 2 MB limit. Collecting remarks from other computers through the network is the next step, like `get_log`.

## D-118 — The Windows client runs from one path for every version (T-136)

2026-11-10, Claude Code, after the owner updated the server computer (Windows) remotely and the whole network went silent. The client ran from its pinned copy `runtimes/<digest>/messenger.exe` (a new folder per version). Windows Firewall remembers an allowed program by its path, so the new version asked «Разрешить доступ» on that computer and blocked the server until someone answered there; nobody was at it.

Now, on Windows only, the client is started from `runtimes\messenger.exe`, the same path for every version. On each start the file gets the pinned copy's bytes (written to a temporary file, checked by SHA-256, renamed into place); a file an earlier client still runs is renamed aside to `.old-*`, never overwritten, and deleted on a later start once nothing runs it. A lock (`runtimes/stable.lock`) is held until the started client confirms, so two starts cannot swap the file under each other; a start that finds it busy starts the pinned copy as before. Workers and their MCP processes still start the pinned copy (`AGENT_MESSENGER_PINNED_RUNTIME`, used only when its bytes equal the client's own), so a later update does not change a running worker's program. The pinned copies stay on disk as before. The first start of this version still asks once, for the new path; after that, updates, including remote ones of the server computer, keep the permission. Mac and Linux are unchanged.

## D-119 — A full mailbox never holds back messages to other computers (T-137)

2026-11-12, Claude Code, from the coordinator's feedback journal (row 34): Ноут-Linux received tasks and ran them, but none of its replies reached the Mac, from the server outage on 11-10 until a restart and an update did not help. A computer sends its outbox in batches of 50, oldest first, and skips a recipient whose mailbox on the server is full. Messages for a listed computer that nobody reads (an old entry of a computer that joined again stays listed, T-127) fill its mailbox; once 50 or more of them waited, every batch held only them, and nothing else was ever sent. Now the next batch leaves out recipients found full, so the others' messages go. The waiting messages for the full mailbox stay queued and are tried again each round; nothing is dropped or retried differently.

Until a task is accepted, the window says «Нет ответа от «X»», not «Работает», because it may run there with its replies lost. The coordinator's `not_accepted` event names that case when the computer reports the task's agent busy, and its instructions say not to start such a task again (D-021: unknown is not success).

## D-120 — An explicit cancel closes a stuck task where it ran (T-145)

2026-11-12, Claude Code, owner's request («закрыть задачи — он их останавливает все, у всех, удалённо»). ПК-1 refused an update for three tasks after the owner had closed them on the Mac: «Закрыть» closed them only on the Mac, and ПК-1 refused the cancel because they were interrupted or blocked there (D-021). Now an explicit cancel of a blocked or interrupted task (the owner's «Закрыть» in the window of either computer, the coordinator's `cancel`, or the cancel the giving computer sends) closes it on the computer that ran it, with the D-108 check for every stuck run: the computer restarted since, or neither the recorded process (same id and start time) nor any process of its group runs. Those runs are closed as cancelled, the task too, and the giving computer gets the event. If a run may still be starting, its process was never recorded, the snapshot fails or anything still runs, the cancel is refused, the task stays as it is, and the giving computer's coordinator gets a `cancel_refused` event with the reason. Nothing is closed without such a command; reservations and `active-runs.json` are not touched. A refused update now names up to three of the tasks that hold it and their states.

## D-121 — A worker's reply is never dropped (T-146)

2026-11-12, Claude Code, owner's request that reports always arrive. When the server refused a message because its recipient was not in the network, the message was deleted. For a worker's reply about another computer's task (the report event, acceptance, refusals, output) that lost the report whenever the giving computer had left the network or rejoined under a new id. Now such a reply is parked: it stays in the outbox, is not sent while parked, and goes again when that computer is listed under the same id or moves with it to a new id (D-115). Sending a parked reply again is a delivery retry that the receiver applies once by its op id, so it cannot start, answer or cancel anything twice. Other messages keep their handling (a start fails, an answer or cancel blocks the task).

## D-122 — Notes and worker state cross computers (T-149, T-150)

2026-11-13, Claude Code, coordinator feedback (a remote task silent for 55 minutes; a report after a cancel; replies late by hours). The coordinator's `note_task` now reaches a worker on another computer as a "note" message, kept there like a local note and read when the worker's current run ends; a refusal comes back as `note_refused`, and nothing is resent. It goes only to a computer that reports 0.67 or newer and is in contact, since an older one would drop it. Each computer adds two fields to its presence: `outbox` (how many messages it has not handed to the server for listed computers, and the age of the oldest) and `workers` (the other computers' tasks whose worker process it runs now, with the seconds since that worker's last output). A host before 0.67 refuses them and the client reports without them. These only inform: no state changes, nothing is closed, cancelled or retried because of them. A report that comes after a task was cancelled here is shown to the coordinator as `late_report` and does not reopen the task.

## D-123 — Codex can be the coordinator (T-160)

2026-11-14, Cursor, owner's request («чтобы кодекс мог быть координатором»). The coordinator connection was already provider-neutral (D-021); only Claude was wired (D-050). Codex has no project-scoped `mcp add` (`codex mcp add` writes the user config, shared by every project). Messenger writes only the `[mcp_servers.messenger]` table in the project folder's `.codex/config.toml`, and only when that table is missing or is this Messenger's own entry for an older program file. A different `messenger` entry, one the owner turned off (`enabled = false`), a symlink of `.codex` or `config.toml`, or a file that cannot be read safely (a multiline string, a broken quote, or a file larger than 1 MB) is left byte for byte. The table sets `default_tools_approval_mode = "approve"` and `tool_timeout_sec = 180`, because Codex stops a tool call at 60 seconds and `wait` may block for 120. It does not set `required`. Messenger does not run the Codex CLI; the Codex program must be registered first. Codex loads the file only for a trusted folder. A worker in that folder loads it too; `AGENT_MESSENGER_WORKER_RUN` still makes the coordinator server list no tools. The window's «Координатор» row shows Claude Code and Codex separately, and a new project connects each agent that has a program and is not already connected. One chat remains the project's coordinator (T-090). Cursor is unchanged.


## D-124 — Waiting follows the coordinator client's delivery mechanism (T-161)

2026-11-14, Codex, after the owner reported that the Codex coordinator only responded to worker questions after an owner message. The observed chat received events through Messenger and its background watch exited, but that shell completion did not resume the idle chat. T-120's unconditional promise that ending the turn would be followed by a wake-up was wrong for this client.

Only the known `claude-code` MCP client receives background-watch guidance and the `watch` command. Codex, unknown clients and connections without a watch command receive active-wait guidance: keep the turn active while following authorized work, call `wait` with a 30-second timeout, process events, and continue waiting. An empty timeout is not completion. End monitoring when the owner stops it, the chat is superseded, or the requested work finishes; if a turn must end, disclose that automatic monitoring stops. The mode is repeated in tool results so truncated initialization instructions do not hide it. There is no new idle-chat wake integration, external message sender or automatic worker retry. Real Codex acceptance remains outstanding.


## D-125 — Cursor can coordinate through the project MCP configuration (T-162)

2026-11-14, Codex, owner's request to add Cursor as a coordinator alongside Claude Code and Codex. The window writes a `messenger` stdio entry under `mcpServers` in the project's `.cursor/mcp.json`, the documented configuration shared by Cursor's editor and CLI. It requires a registered Cursor program and never launches a model to connect. The entry explicitly forwards `AGENT_MESSENGER_WORKER_RUN` with Cursor's environment interpolation, so workers loading the same project configuration receive no coordinator tools.

Only a missing entry or an existing entry with the same coordinator arguments and an older program path is written. Unknown settings, other servers and additional options in an existing owned entry are preserved as JSON values. Foreign or disabled entries are not replaced. Invalid JSON, duplicate keys in edited objects, files over 1 MB, non-regular files and symlinks of the config or `.cursor` directory are refused. A concurrent edit detected before replacement is not overwritten. Cursor's own approval and disabled-server lists are untouched; the window says the configuration is saved and asks the owner to approve Messenger in Cursor. It does not claim that saving the file proves the chat has connected.

Cursor receives D-124's active-wait guidance. Protocol tests show its waiting call receives a worker question and the later report; they do not prove a model follows instructions or that an idle Cursor chat wakes itself. The owner will perform the real chat acceptance.


## D-126 — Welcome appears before project creation (T-164)

2026-11-14, Codex, owner's correction: the first installation must show the welcome introduction, not only «Создайте проект». Reuse the existing welcome page before the first project and while an existing project has no selected task. Its first step creates a project; the coordinator step remains unavailable until a project exists. The empty task pane and its toggle are hidden before project creation so they do not consume the welcome area or cover its actions at narrow widths. Once a project exists, the task pane follows its usual expanded/collapsed state. No project, model session or coordinator connection is started merely by showing the welcome page.


## D-127 — Install and find CLIs through the owner's terminal (T-153)

2026-11-14, Codex, owner's repeated request. Replace the install-page action with the official standalone installers for Claude Code, Codex and Cursor on macOS/Linux/Windows, in a visible terminal. A CLI already found there is connected without reinstalling it. Find uses the same terminal PATH without downloading anything. Validate the returned absolute executable path; resolve known Windows npm/Cursor shims to their underlying executables. Preserve a registered agent's profile. On confirmed completion, register once and run the existing version/sign-in diagnostic. This does not sign in or start a model.

Store private, per-attempt scripts and atomic result receipts under the product data directory. Reopening the window resumes reading results, not installation. A durable claim prevents repeated registration after an unclear reply. No receipt means waiting/unknown, never success or an automatic retry; manual file selection remains available. Keep uninstall on a separate official-page endpoint (Q-022). Sources and limitations are in the acceptance report.

## D-128 — Folder trust uses the agent's CLI confirmation (T-165)

2026-11-14, Codex, owner has computers without graphical agent apps. The local agent card offers Trust for the selected project's folder. Show the exact folder and explain that project settings and commands become available, then open the registered CLI there with its registered profile and no model prompt. The CLI itself asks for trust; Messenger never edits trust files or answers the prompt. Refuse a changed folder or unregistered agent. Refresh trust on return to the window. This is a new interactive terminal session, not a working chat or a background worker.

## D-129 — Report terminal launch uncertainty and LAN discovery failures (T-166/T-167)

2026-11-14, Codex, owner reported a silent sign-in button and failed six-digit joining between two Windows computers. The exact failures on those computers are not reproduced. Windows opens its interactive PowerShell child via Start-Process so a GUI Go process's null standard handles are not passed into the login CLI. macOS/Windows launchers are awaited briefly; immediate launcher errors are visible, and an unfinished launcher is reported as unconfirmed. Linux uses the owner's interactive login shell to include nvm/custom PATH. An unconfirmed installation launch is not marked failed or restarted automatically.

LAN discovery repeats only its public UDP question within the existing bounded search. It never repeats registration or code proof. A busy/unavailable UDP discovery port is shown explicitly and the host offers only its usable long invitation. No-host guidance identifies LAN scope, a fresh invitation, Windows firewall UDP 47100 and Wi-Fi/VPN isolation; it does not alter firewall rules or bypass certificate/code checks. Real Windows-machine acceptance remains necessary.

## D-130 — Recover setup display through explicit discovery (T-168)

2026-11-14, Codex, owner confirmed that closing the Codex installer terminal on another Mac left 0.71 waiting. Opening a terminal is not evidence that the installer still runs. Missing receipts and POSIX interruption receipts report unknown; no timeout or PID can turn that into success or trigger a reinstall. Offer Find directly beside the pending installation. It only discovers an existing executable through the terminal; successful one-time registration marks the old install display recovered while preserving its files and unknown exit. Late installer output cannot register again. A lost registration reply stays unconfirmed. This affects setup only, never task runs, reservations or active-runs.json.

## D-131 — First login may create the standard profile (T-170)

2026-11-14, Codex, owner clarified the sign-in problem concerns Cursor on Windows and recalled the generic invalid-data message. Discovery supplies a standard profile path even when no profile exists yet. Login command construction required an existing directory, preventing the very first login from creating it. Permit only a missing standard profile, leaving its creation to the CLI. Do not create directories or fall back from a missing custom profile; preserve custom-profile selection. Return actionable local path errors as JSON so the UI does not replace them with the generic HTTP-400 hint. No credentials, authentication URLs or CLI output are collected or logged by this change.

## D-132 — Explicit researched model selections (T-171)

2026-11-14, Codex, owner's request. Offer a short task-oriented catalog with exact pinned IDs and effort. The `model` field accepts a Messenger suffix `[effort=medium]` (also low/high/xhigh/max) for Codex/Claude; adapters split it into the native model and effort settings, including continuations. Cursor uses verified CLI IDs directly. Share validation between storage and launch. Keep plain IDs compatible. Configured lists are not account entitlement claims; do not probe with inference, automatically retry or change models after failure. Return optional catalog guidance alongside existing model strings. Upgrade only the exact previous default lists once; preserve custom lists, empty lists and recorded tasks. Sources and limitations live in MODELS.md.

## D-133 — Queue a Messenger update until live work ends (T-155)

2026-11-14, Cursor, owner's request that the coordinator update Messenger on a computer between tasks.

Amends D-100: a request to install the newest signed Messenger (the program only) no longer refuses while tasks still run. Running tasks are never interrupted. The computer answers `queued`, takes no new tasks (`start_task` here or from the network, and it is left out of `idle_computers`), shows «ждёт обновления», and starts the same `_remote-update` helper as soon as no task still runs or waits. Blocked and interrupted tasks do not hold it (they survive a restart and are closed only by an explicit command, D-120). `continue_task`, `answer`, `note_task` and `cancel` still work. A Mac still refuses and says to press «Обновиться» in its app (Sparkle). The coordinator tool is `update_messenger` (this computer, or another of the network); the window's update icon uses the same queue. Each answer is a `messenger_update` event of the project that asked (empty `task_id`) and a window notice. Nothing is retried on an unclear result. Presence reports `update_pending`; an older host that refuses the field still gets the rest of the report.

## D-134 — Worker pictures travel with the report or question (T-175)

2026-11-14, Claude Code, coordinator list row 23 (still open). `report`, `ask` and `ask_owner` take an optional `images` list of paths (relative to the task folder or absolute), at most 4 files of at most 5 MB. The worker's tool server reads them once; only content detected as PNG, JPEG, GIF or WebP is kept, as a copy in the database with the run (a later change or deletion of the file does not matter). They are published with the report or question, after the run's verification, never before. The coordinator's event says «[N picture(s) attached: read them with task_images]»; the tool `task_images` returns the latest pictures as MCP image content. The window shows them under the report or question, fetched with the page's key (the page's policy allows `blob:` images). For a task of another computer, each picture goes in one mailbox message before the event; a picture over 180 KB is sent as a smaller JPEG (longer side 1600 px down to 640 px); one that cannot be decoded (a large WebP) is not sent, and the report still arrives. A computer before this version ignores the message. At most 40 received pictures are kept per task.

## D-135 — Continue an interrupted task of another computer (T-159)

2026-11-15, Claude Code, owner's go-ahead for T-159. `continue_task` also takes an interrupted task that ran on another computer of the network (it died or restarted while the task ran). The giver sends the usual `continue` message; the computer that ran the task applies D-108 itself: it closes the interrupted run only when its own check confirms the worker is gone (another boot, or neither its process nor its group runs), and otherwise answers `continue_refused`, so the task goes back to interrupted with the reason. Nothing closes such a run on its own; it happens only on the coordinator's explicit command. The giver refuses before sending when that computer reports a version older than 0.75, which would refuse anyway. The lock an interrupted task holds is released only by this continue or by an explicit cancel (D-120), as before.

## D-136 — The coordinator sees the newest release from this computer's update channel (T-158)

2026-11-15, Claude Code. `list_agents` adds `newest_release` (short and full version, notes up to 4000 characters) from the signed release this computer's own update check last downloaded and verified, and `update_available` on each computer that reports an older version. The coordinator's server fetches nothing; without a downloaded release the fields are left out. The notes are release text, data for the coordinator, not instructions.

## D-137 — An internet scope for the hosted broker (T-182)

2026-11-15, Claude Code, owner's request: the server must be reachable over the internet, not only on the local network. The owner's server will be a home computer that is always on and has a public address. A third access scope «Интернет» is added next to «Только этот компьютер» and «Локальная сеть». It listens on every interface and also lets in peers from public addresses; the owner enters the external address (IP or domain, optional external port), which becomes the first invitation address. The program does not open router ports, guess the public address or get around NAT. Protection stays the same as on the local network (TLS 1.3 with a pinned fingerprint, one-time 256-bit invitations, 256-bit device keys, the registration limit, revocation) and adds: public peers get at most 32 of 64 connection slots and 8 per source, and the six-digit pairing answers only this computer and private networks, so from the internet only the long invitation joins. Rejected for now: a standalone VPS package (T-036 stays in the backlog) and a relay through a third-party service.

## D-138 — A server on a VPS is installed with one command and managed from the terminal (T-036)

2026-11-15, Claude Code, owner's request after trying to run the server on a VPS: the window had to be opened through an SSH tunnel, its «Запустить сервер» answered «Действие не подтверждено», and SSH did not reach the VPS from a computer with its own VPN. The owner wants the server set up "almost with one command", with an easy way to get invitations.

- The server is the same program, published in the same release: no separate build or branch. `scripts/install-server.sh` is a release asset; `curl -fsSL …/releases/latest/download/install-server.sh | sh` (as root) downloads the Linux program of the same release, checks it against the release's SHA-256 file, installs it as `/opt/agent-messenger/messenger` with data in `/var/lib/agent-messenger`, runs it as the systemd service `agent-messenger` (`messenger server run`: the client in the foreground, paused, no tasks or models, update requests from the network refused), turns the server on for the internet with the address of the network card the computer reaches the internet through (or `AM_PUBLIC`), opens the port in ufw/firewalld when they are active and prints an invitation. Running it again replaces the program, restarts the service and keeps the data and the devices.
- `agent-messenger-server invite|status|enable|disable|revoke` (a wrapper for `messenger --root /var/lib/agent-messenger server …`) does the rest; the same `server` commands work with any `--root`. They are the window's server actions over the control socket, with the same rules.
- The installer does not stop another Messenger client of the same user (one started with `start`): it names it and asks the owner to stop it with `shutdown`.
- The owner first asked for a "server only" mode that hides the computer from the list, then decided against it: the server stays an ordinary computer (with agents it takes tasks, without them it does nothing). Instead, the computer that runs the server is labelled «сервер» in the window's computer list, and `list_agents` marks it with `server: true`.

## D-139 — The VPS server updates from the window like any computer (T-183)

2026-11-15, Claude Code, owner's request: update the server "by the button, from any computer". In 0.77 the service client refused update requests from the network.

- Under systemd (`INVOCATION_ID` set) `server run` records its program file and accepts update requests as every Windows and Linux computer does (T-078, D-100): the same checks (no live tasks, one update at a time), the same signed update channel. The helper is started with `--service agent-messenger`: it installs the update over `/opt/agent-messenger/messenger`, writes the result first, then asks `systemctl restart --no-block agent-messenger`, which also ends the helper. The restarted client sends the result. If systemctl fails, the result is rewritten as failed with the command to run on the server.
- Started by hand (no systemd), `server run` still refuses: nothing would start it again.
- Risk accepted: an update that does not start leaves the server down until the owner runs the installer again over SSH. The update is the same signed program every computer installs.
- `server rename --name ИМЯ` sets the name the server computer reports to the others (the window's «rename» action); by default it is the VPS host name.

## D-140 — The window offers an update and one button downloads, installs and restarts (T-187)

2026-11-15, Claude Code, owner's request: the window downloaded an update by itself (a ring around the icon), then showed the same icon again to install it, which looked like the update had to be done twice. The owner asked for one icon, and a dialog with the download, the install and the restart.

- Windows and Linux: the window's check every 30 minutes only finds a newer signed release (`FindGitHub`: the signed manifest is fetched and verified, the archive is not downloaded and the channel is not changed, and the download lock is not taken, so a remote update is never held up). A found release shows the icon. Its click (`/api/self-update` action `update`) downloads the archive, verifies it and installs it as before, then restarts the window; a dialog shows «Загрузка… N%», «Установка…», «Перезапуск…». A failed download or install says so in the dialog, leaves the program as it was and offers «Повторить». The dialog is not closed halfway.
- An update already downloaded (for example by an earlier version) is installed without another download.
- Remote updates (T-078, D-139) still download through `Check`; nothing changed there.
- Mac: Sparkle still installs, with its own confirmation window. The window no longer shows the icon while the background download runs, only once the update is ready, so the icon appears once. The Mac flow is not otherwise changed in this step.

## D-141 — The feedback journal exports what is new and is cleared once passed on (T-189)

2026-11-16, Claude Code, owner's request: every export repeated all old remarks (38, of which 2 were new), and once the file is handed over there is no reason to keep them.

- An export (window or `export_feedback`) takes only the remarks no earlier export had and marks them exported. With nothing new it writes all remarks again, so the owner can always get a file.
- «Очистить журнал» (shown after an export in the feedback dialog) and the coordinator's `clear_feedback` delete the exported remarks after a confirmation; remarks written after the export stay. The coordinator clears only when the owner asks. Nothing is deleted on its own.

## D-142 — A restarted coordinator chat is not reported as a takeover (T-190)

2026-11-16, Claude Code, coordinator feedback of 2026-11-15: after every update or restart of the agent the coordinator chat was told that "another coordinator chat was active before", though it was the same chat: a restart starts a new process with a new chat id.

- The active chat's record keeps the process serving it (pid, start token, boot id). A new chat that takes over is told about another chat only while that process still runs on this computer. A gone process means the same chat restarted (or an old chat ended): it takes over quietly. D-099 is otherwise unchanged: the newest acting chat is the coordinator, and a live old chat's wait still ends with `superseded`.
- When the process was not recorded (a record from an earlier version) or the process list cannot be read, the note is given as before, so a real second chat is never hidden.

## D-143 — Codex usage limits are read from the account every 10 minutes (T-191)

2026-11-15, Claude Code, owner's request: the coordinator must spread an overnight queue by how much of each agent's limit is left, without the owner typing numbers in.

- The background program starts `codex app-server` with the owner's normal Codex profile and sign-in every 10 minutes (first one minute after start), sends `initialize`, `initialized` and `account/rateLimits/read`, and stops the process with its group. No thread is created and no model is called; tools, plugins and MCP servers are turned off on the command line. A paused agent is not checked. A failed read keeps the last stored limit, writes one journal line and is tried again only at the next round.
- Both windows (about five hours and weekly) are stored with their percent and reset time; the window used the most leads. 90% or more is «warning», 100% (or a reached limit) «exhausted». The limit carries `source: "check"` and `windows` in `list_agents` and goes to the other computers in presence; a host before 0.80 refuses those two fields and gets the plain limit instead.
- The check is not a CLI run on a session: it takes no reservation in `active-runs.json`. Tests use a fake reader and a fake app-server; the real check happens only in the background program.
- Claude's task stream gives only status and reset time, and its percent is shown only to interactive sessions; Cursor has no known way. Both stay as before until a source is found.

## D-144 — A bright program icon: a dark speech bubble with eyes on yellow (T-194)

2026-11-17, Claude Code, owner's choice («самый первый вариант»): of six bright variants (agent with eyes, relay arrows, terminal prompt, network nodes, two bubbles, typing dots) the owner chose the yellow square with a dark speech bubble with two eyes. The shapes are the same in `internal/localui/assets/icon.svg`, `scripts/gen-icon` (Windows resources) and `desktop/macos/icon.swift`: a 464 px yellow `#FFC531` square with 104 px corners, a `#1B1B2F` bubble with a tail at the bottom left, white eyes with dark pupils looking right.

## D-145 — The coordinator computer supplies the model list (T-196)

2026-11-18, Codex, owner's correction. The coordinator configures its lists in the existing agent settings, including for providers installed only on other computers. For 0.82 executors, the coordinator advertises its own model choices in `list_agents` for that executor. It sends the chosen model in the existing task message with a flag marking coordinator selection. The executor validates syntax/provider and preserves its usual admission and launch checks. Its own CLI and account resolve the model and effort. Earlier executors and legacy task messages retain local-list behavior. No remote settings command is shipped.

## D-146 — The tray, one window per computer, and no Sparkle windows (T-184, T-199)

- Closing the window hides it in the tray (Windows notification area, Linux tray where the desktop has one, Mac menu bar). «Выйти» in the tray menu closes only the window, as closing it did before: the background client and its tasks keep running. Without a tray on Linux, closing closes as before.
- A second launch on Windows or Linux brings the open window forward instead of opening a second one (Windows: the window's class and title; Linux: `window.sock` in the data folder). The window is destroyed before the program reopens after an update, so the new copy does not find the old one.
- On a Mac the app is Sparkle's user driver: Sparkle still verifies, installs and relaunches, but shows no window; the page's update dialog shows the steps, as on Windows and Linux. Nothing is retried; an error ends the run and the old program stays.

## D-147 — A run closes after its report once the worker's own process exited (T-205, Q-030)

2026-11-18, Claude Code, owner's answer to Q-030: (b).
- A task run whose worker recorded its report or question, and whose own process (the launched CLI) was observed to exit, is closed even when processes it started still run in its group after the group was signalled. Their names and process ids go into the run's detail and a `leftover` log event; Messenger does not stop them further. The reservation is retired as for a clean run.
- If the worker's own process is still there, nothing changes (T-172: the run is described, not closed). Without a recorded report or question, or when the group's members cannot be listed, the run stays blocked as before (D-021). A cancelled run with leftovers stays blocked too.
- Cause found on the way: Messenger waited for the end of the worker's error output, which a leftover child inherited, so `Wait` never returned after the worker exited. Waiting for that output is now limited to a few seconds after the exit (`WaitDelay`), and output a leftover keeps open after a completed turn is no failure.

## D-148 — Larger window text (T-219)

2026-11-19, owner request for more readable text. The embedded window uses 15 px / 21 px for chat and composer text, and 14 px for the base UI and sidebar agent names. Secondary text and metadata use 13 and 12 px. Existing families and control sizes are retained. See [UI_STYLE.md](UI_STYLE.md).

## D-149 — Visible task event and finish times (T-223)

2026-11-19, owner request. Display recorded times next to report closure, questions and outcomes. Use the stored finish time in the right-hand task list when the task ended. Active rows keep their creation time (T-121); a completed row without a known finish time has no date. Local task history has the per-run timestamps. Remote events in a local thread carry the time this computer recorded them.

## D-150 — Owner notes to a running worker on another computer (T-224)

2026-11-19, owner request. Extend D-096 and D-122 to owner notes from the computer that gave the task. The existing network note message is idempotent and accepts an optional owner marker; a receiver before this change still queues it as a coordinator note. The sender records the author and notifies the original coordinator once that it queued the note. The worker applies the same state checks and reads the note when its current run ends. A peer that no longer accepts the note reports a refusal.

## D-151 — Readable action captions in task history (T-225)

2026-11-19, owner approved short captions with type icons and the existing right-hand disclosure arrows. The group count sits above the category breakdown. Use the provider description when supplied. Old history and actions without descriptions get conservative captions based on the recorded tool or command. Expanding an action shows the saved text.

## D-152 — One-line command group headings (T-226)

2026-11-20, owner chose status and elapsed time in the same size in the group heading. Count completed shell commands, or show the action count in a mixed group. The window shows live time only from a recorded shell start; remote time begins when the starter receives that event. The timing entries do not consume the visible feed allowance or appear in coordinator messages. Keep the existing tool rows, icons, command text and disclosure arrows.

## D-153 — On-demand agent CLI versions for the coordinator (T-227)

2026-11-20, owner asked for a separate coordinator command. `agent_version` runs the registered CLI's fixed `--version` command in a neutral folder on the selected computer. It neither opens a worker session nor checks account access, and returns only a validated CLI version, check time and result state. Remote requests use the existing network mailbox. Each request is claimed once. When the tool returns `waiting`, use the request ID to read its answer. Older remote clients ignore the new request.

## D-154 — Separate update arrow (T-228)

2026-11-20, owner chose a separate amber arrow with a round hover and keyboard-focus background. The installed version remains plain, muted text. Keep the local and remote update flows.

## D-155 — Running agent label and elapsed time in the sidebar (T-229)

2026-11-20, owner chose the soft background option. A running agent's name and model share a blue highlight that travels across the text. The elapsed time sits beside the model at the same type size, on a quiet blue background. With reduced motion or forced colors, the label is static. Use the existing running state for both local and remote agents.

## D-156 — Reconcile the computer version with an update result (T-230)

2026-11-20, owner saw a success notice while the same computer still showed its earlier version. Project the matching, recent successful result onto the broker's presence snapshot before sending the window response. The coordinator's roster uses the same reconciliation. A newer reported version wins. Failed and refused replies cannot change the version.

## D-157 — Use the project task row across the window (T-231)

2026-11-20, owner asked for timely task updates across the window. The project view supplies the task state, model and start time to the list and open conversation. The conversation fetch supplies history. The sidebar uses the same project rows for known work and preserves the local counts for other projects. A confirmed outcome takes a remote task out of a stale reported running count when the report identifies that worker. Unmatched remote activity still follows presence. Elapsed labels share a clock tick without redrawing the conversation.

## D-158 — Keep live command timing on the latest worker group (T-232)

2026-11-20, owner reported a timer on an earlier command group after the worker posted a new message. Only the latest worker action group can show a live timer. A later completion event can still give an earlier group its verified completed-command count. A previous group with an unmatched command start displays an action count. Command group headings and their timers use muted text. The running label uses a neutral sweep with the sidebar animation's timing. It remains static with reduced motion or forced colors. The elapsed time and earlier headings remain static.

## D-159 — Lighter conversation typography (T-233)

2026-11-20, owner found the conversation text visually heavy. Use the existing system font at 14 px regular weight with a 21 px line height for the message body and input. Use grayscale text smoothing where supported. The rest of the window keeps its type scale.

## D-160 — Compact agent rows in the computer list (T-234)

2026-11-20, owner chose the compact preview and then selected the variant with a comma and muted time. Use the task list's agent label size for the sidebar's agent name, model and timing. Place the muted running duration after a comma and space when the model is shown, with no background. Preserve the blue highlight on the agent and model, and let long labels give way to the duration.

The owner also asked for a consistent size within each task card. Use 12 px throughout the card and weight 500 for the task title.

## D-161 — Static blue running labels in the computer list (T-235)

2026-11-20, owner requested a static activity label. Use the task list's running status color for the sidebar's running agent and model in both themes. The elapsed time stays muted. The command status animation in the conversation follows D-158.

## D-162 — Show running work and reported worker models (T-236)

2026-11-20, owner saw a yellow duration and missing model on cards of remote computers. Running work takes precedence in the sidebar when the agent also has a task waiting. The count includes running and waiting tasks. The timer reflects running work, and the tooltip gives the waiting count separately. A waiting-only row labels the wait explicitly. Forward a valid model reported by the worker to the computer that assigned the task. This is display metadata and never changes the requested model or task state. Older clients can ignore the message. If the worker did not report a model and the coordinator did not select one, show that the model was not specified on known task rows in the sidebar.

## D-163 — Inline waiting status (T-237)

2026-11-20, owner asked to align the sidebar waiting label with the running duration. Place it after the model with the same comma separator, and use the waiting color for the name, model, separator and status. The task card shows its wait text in the regular status position without a dot.

## D-164 — Match typography for equivalent window elements (T-238)

2026-11-20, owner asked to align computer and task names and review the window's typography. Use a shared 14 px medium style for computer and task names in the lists and statistics. Their metadata remains 12 px. Confirmation headings use the settings section style. Project instructions are prose and use the same system text as other plain-text fields.

## D-165 — Label the live local coordinator (T-239)

2026-11-20, owner approved the lowercase «координатор» label beside the computer name. The window shows it on this computer’s card while the active chat of the selected project (D-099) has a live local process. Reuse the server label style. Historical connections and peer presence do not establish this role. The label refines D-064 for this computer. A handover between local chats leaves network task routing unchanged.

## D-166 — Sort statistics from the column headers (T-241)

2026-11-22, owner chose clickable date, duration and token headers. Default to newest first. Each header reverses the order on a second click. Rank the filtered period before taking ten rows. Duration is elapsed time from the first worker launch to the latest task completion, including pauses between runs. The executing computer sends the elapsed value on completion, so network delays and clock skew do not affect it. Leave missing timing blank and exclude it from duration ranks. A completed task with known timing can appear in the duration ranks even when the agent reported no usage. The token totals and existing date and token lists keep their usage-based scope.

## D-167 — A Codex reconnect is provisional (T-242)

2026-11-22, owner reported a completed change whose saved report was attached to a run that Messenger could not confirm. A Codex error notification in the form `Reconnecting... N/M` is provisional. A later explicit `turn.completed` clears that notification. A terminal or unknown error still fails verification. Without completion the run keeps its actual failure state. Classify the model-capacity diagnostic as overload and show that the provider did not confirm completion. The saved report remains available as unverified text. Preserve D-021 and D-147 and never retry the run automatically.

## D-168 — Attribute the original task message to its sender (T-243)

2026-11-22, owner saw a task from Claude labelled as Codex. The project connection record describes the last initialized client and cannot attribute messages. Record the assigning chat's provider at `start_task` and carry it with the task. The optional `coordinator_model` is supplied by the coordinator only when it knows the active model. The worker model is separate. Use the same provider icon as in the worker header when the sender is known. Show the generic coordinator label when the sender is unknown, including existing history and follow-ups with no attribution.

## D-169 — Keep a manual order for the computer list (T-244)

2026-11-22, owner asked to rearrange computers by dragging their cards. Save the order in this window's local storage, across projects. Use the lasting computer ID where available. New computers follow the existing order. The keyboard can move the focused card with Alt and the up or down arrow. Clicking a card still filters by that computer.

## D-170 — Reopen the window after a remote update (T-240)

2026-11-22, owner requested that the window catch up after a remote update. On Windows and Linux the window observes the local background client, including when hidden in the tray. It reopens itself only after the client reports the newer version in the signed channel and the recorded program file matches that signed archive byte for byte. The existing window relaunch path closes the old desktop UI before starting the new program. This handoff does not stop or restart the background client. Older windows lack the check and must be exited and reopened once.

On Windows, the update relaunch and the installer's handoff to the installed app use a process creation flag that suppresses the console. The native desktop window opens normally.

## D-171 — Use saved remote progress when elapsed time is missing (T-247)

2026-11-22, owner reported a remote task that ran for about three hours but had no duration in statistics. The executor's exact elapsed time takes precedence. For a completed or failed remote task without it, estimate from the first worker activity received to the recorded completion, and mark the value with ≈ in the table. No estimate is made for local cancellations or records without worker activity. The date and duration views include timed tasks without reported usage; token rankings and totals use reported usage.

## D-172 — Attribute each coordinator message separately (T-248)

2026-11-22, owner asked for the same sender name and icon on subsequent coordinator messages. The messaging tools record the calling chat's provider and its optional explicit model on each answer, continuation and note. Send this metadata with the existing remote messages, reuse the message header from D-168, and show the origin's recorded continuation in its task history. Older records and unrecognized clients keep the coordinator role label. The worker's conversation and the owner's own message attribution follow the existing rules.

## D-173 — Notify the coordinator when Codex is slow to exit (T-249)

2026-11-22, two Windows workers still held their task locks long after their completed turns. If the recorded Codex worker process is observed alive after the one-minute grace period, publish one `worker_still_running` event per run through `wait`. Use the task's existing event route for remote tasks. The observation describes a delayed exit and can be followed by the ordinary verified result. The run remains subject to D-021 and D-147. The event does not mark its computer idle or trigger a cancel or retry.


## D-182 — Stage project data before switching the live queue (T-258)

2026-11-23, owner requested a database per project and a database for the computer's common state. Use a project-scoped SQLite snapshot as the first inspectable migration component. A snapshot contains the selected project's history and has a format marker that prevents ordinary queue startup. It leaves the computer's existing queue intact. Live migration, network delivery and the archive workflow are tracked in PROJECT_STORAGE.md. D-066 remains in effect until the archive lifecycle is implemented.

## D-184 — Start broker write transactions before reading mailbox quotas (T-262)

2026-11-23, concurrent mailbox sends reproduced SQLite error 517 on the 0.95 broker. Begin broker write transactions with `BEGIN IMMEDIATE`, so contention waits at transaction start under the existing busy timeout. Keep the quota checks, duplicate check and insert together and preserve the existing operation IDs and delivery rules.

## D-185 — Finish revision reads and discard expired SQLite connections (T-263)

2026-11-23, a Mac running 0.95 repeatedly returned SQLite 517 while its legacy database remained active. The pinned SQLite driver can abandon live rows when interrupted while returning a result, leaving a stale WAL snapshot in a persistent pool. Keep revision reads cancellable while acquiring a connection, then finish the indexed query before returning the connection. For store transactions, discard a connection reporting 517 after rollback. Retry once only when `BEGIN` failed, before running any operation. If the first window snapshot fails, show that the projects are unavailable. Do not change migration, session ownership or receipt rules.

## D-186 — Loading indicator instead of the welcome; the welcome only on first run (T-265)

2026-11-23, the owner saw the welcome flash while switching projects, and a server without agents always showed it. The window now chooses the top task before drawing the new project, and while the project or the chosen conversation is loading the pane shows the start screen's spinner after 200 ms. The welcome is shown only until this computer has seen its first task (its own or another computer's) or serves the network; this is remembered in the window's local storage, so a cleared storage shows it once more. Afterwards an empty project only says «Задач пока нет».

## D-187 — Keep shown data in place while new data loads (T-266)

2026-11-23, after the window became faster the owner saw blocks jump: choosing a computer, a period or a project emptied the statistics and the task list for a moment, so the page shrank, its scroll bar vanished and both came back. The statistics now keep the shown numbers until the new ones arrive (the computers card follows the numbers on screen), and a project switch keeps the previous rows in the task list, inert and marked busy, until the new project's view arrives. Nothing new is shown as loaded before it is, and the old rows cannot be chosen.

## D-188 — Unroutable network messages are skipped; unpublished tasks are closed (T-267)

2026-11-23, a review of the 0.95 per-project storage found that one network message without a task route stopped the whole inbox for good: the cursor never advanced, so no later message from any computer applied. The single database skipped such a message, and the per-project path does again: a request or a reply that no route matches, even after rebinding the peer, is acknowledged and dropped; a route whose project database lacks the task is skipped the same way. A start for a project that is visible but missing from the storage catalog, or whose database belongs to another project, is refused, so the computer that gave it learns why. A task whose explicit start failed between the project commit and the computer commit (no route) holds no resource and cannot launch; at the next start of the client it is failed with a reason and its route is published, and a start held for another computer is dropped. It is never started on its own, because its start reported an error. Tasks received from the network are left to the redelivered start. Starting a run no longer reports a failure of the computer commit, which only held the writer lock. If activation of per-project storage stops before it selects a generation, the client keeps the single database and tries again on its next start; once a generation is selected it never falls back.

## D-189 — The chosen computer is shown only by its card; the project page is a column of cards (T-268 to T-271)

2026-11-24, the owner found the computer chip redundant: the sidebar already highlights the chosen computer, and the chip moved the task list down. The chip is gone from the tasks, the agents and the statistics; a second click on the card or a click on empty sidebar space shows every computer, as before. Because nothing else shows the choice, a chosen computer whose card leaves the list stops filtering. On the statistics the period tabs and the lists are one control height and sit from the left edge. The project page is split like the server page: settings (name and folder), the MCP plugin, the shared worker instructions with their own save status, and removal. A worker's picture opens in a large viewer (wheel, buttons, keys and a click zoom; a zoomed picture is dragged; arrows step through the pictures of one message) instead of growing inside the chat.

## D-190 — Current work through state indexes; computer order by lasting id; a quiet hidden window (T-272 to T-276)

2026-11-24, owner asked that the update's work count read only current tasks and that the program not waste resources. The work conditions select task ids from the tasks and runs in a current state (`tasks_state`, `task_runs_state`) instead of testing every task; the meaning is unchanged. A project database made earlier gets the indexes when the client opens it; a failure there only leaves the slower plan.

The computer order file keeps, beside the order, which lasting computer id each network peer id had. A report briefly without the lasting id (the fallback after a refused report) no longer changes the card's key.

The window names every model through one rule (`modelName`), the statistics filter included. While the window is hidden it draws no timers, does not ask for statistics and asks for the network once a minute; the task list and the change watch keep running, because notices depend on them. Between words the window uses a comma or a colon, not «·».

## D-191 — Automatic updates per computer, set only by its owner (T-277)

2026-11-24, owner's request. Each computer has «Автообновление» in «Настройки программы», off by default. It is stored in that computer's client and changes only through its window or command line: no network message, coordinator tool or other computer turns it on. While it is on, a Windows or Linux client (and the Linux server service under systemd) looks for a newer signed release every 30 minutes, as the window does, and queues one update of this computer for each newer release through the queue of T-155: no new tasks while it waits, the install when no task runs, then the client restarts. A release whose update was queued once is never queued again on its own, whatever the result (no automatic retry); a newer release is. A failed look changes nothing. The Mac is updated by its app and keeps its button (Q-031).

## D-192 — A task of an earlier network belongs to no current computer (T-278)

2026-11-24, owner's report. Peer ids are per network and «host» names whichever computer runs the server, so after the network was made again an old task's peer id can name another computer: tasks sent to ПК-2 when it served the network showed on the VPS card. Tasks from before 0.61 have no lasting computer id and are never rebound (D-115). Such a task (its `remote_broker` or origin broker is not the current network) now has no computer id in the window: it is named by the stored name or «компьютер прежней сети» and shown as without contact, as before. The computer filter and «Убрать все» match a task's peer only in the current network; the shared list leaves such tasks out; the usage page counts them under their own old computer, never under the current one. Nothing in the task, its runs or its reservation changes, and a cancel or answer still goes only through the existing paths.

## D-193 — Cancel only the chosen queued update (T-280)

2026-11-24, the owner reported that an already-updated computer still refused new tasks while another project was running. A queued update is machine-wide. The running build can finish a known target it already satisfies, or a request whose signed channel check shows the running build is current. An unavailable or unverified channel leaves the queue intact. The coordinator can cancel an observed request by its exact ID through the authenticated mailbox. Only the supervisor's update loop removes it, with any launched helper or result fenced. Running tasks, process reservations and the automatic-update setting are unchanged.

## D-194 — Order release candidates by preview build (T-282)

2026-11-24. The GitHub release list can put a new build behind older builds. Sort preview tags by descending numeric build before selecting the platform package. The signed manifest must match the chosen tag. Keep the existing single API request and signature, archive and downgrade checks.

## D-195 — Verify upgrade discovery from installed clients (T-283)

2026-11-24. Validate both the published release and the first signed package visible to the legacy updater. A route through 0.101 or newer is sufficient for 0.99 to recover numeric release ordering. Recovery is limited to renaming discovery manifests in 0.98 and 0.99 after checking replacement packages.

## D-196 — Public release 1.0 (T-284)

Show 1.0 in the window, About panel and release title. Its signed version and tag retain the three-component version and numbered preview suffix understood by installed clients. Compare the sequential build for upgrades.

## D-197 — Queued updates and their shared controls (T-287)

Give an accepted update five minutes to start on the receiving computer. Only expiry of the waiting request releases the update gate. The installer owns an update once it starts. Publish request identity and phase through the broker for every open window, independent of the selected project. An available update uses a download arrow. Waiting and installation use a rotating circular arrow. Where cancellation is supported, the waiting arrow switches to a red cross on hover or keyboard focus.

## D-198 — Reconcile update state by request ID and numeric build (T-289)

The window orders numbered builds by their numeric suffix, following D-196. A receiving computer can clear a shared queued or installing mark by reporting an empty queue after it has reported that request's ID as pending. The broker commits both changes together. A result can advance the reported build while an independent update remains pending under its own ID. Neither queue removal nor a notification substitutes for evidence of an installed version.

## D-199 — Compact computer cards and macOS menu bar clicks (T-292, T-293)

Use a shared gray header, the same gray body on every computer card, and an accent tint on the local header. Selection changes the border. A gray outlined monitor has a green dot while online and a bold red cross while offline. Both markers occupy the same 8 by 8 px area at the lower-right corner. Keep the monitor outline visible through the cross. Mute offline names and use a 10 px gap after the monitor. The macOS menu bar button opens or restores the window on primary click. Its context menu supplies Open Window and Quit on secondary click.

## D-200 — Show a queued update cancellation (T-294)

The window replaces the cancel control with an hourglass during submission and while the local client has the exact cancellation queued. The server status carries pending cancellation targets, so redraws and reopenings preserve the indicator. A new window submission for the same pending cancellation reuses its operation, including after a lost reply. The target's actual update state still determines when the download arrow returns.

## D-201 — Window tooltip wording (T-295)

Use sentence case without an ending period for short tooltips. Keep normal punctuation for multi-sentence explanations. A button tooltip names the action. A noninteractive update indicator describes its state, and a cancellation button exposes the state as an accessible description.

## D-202 — Observe quiet Codex workers (T-290)

After ten minutes without output, publish one advisory per silent period if the recorded Codex process is still alive and there is no tracked command or ended turn. New output starts a new observation period. Keep the one-minute exit warning from D-173 for a completed turn. Both events follow the same task and project as the report. They do not complete, stop, retry, or unlock a task.

The coordinator's task list reports elapsed time since the latest worker output and names a currently observed command when available. Command timing comes from the existing task feed and stays on that task's route. For a remote command, the time is when this computer received its start. A full timing history or an unknown worker state omits the active command rather than guessing.

## D-203 — Stop from the task list (T-291)

An eligible task row offers Stop the task in its context menu without changing the selected conversation. Both entry points share the existing stop request and pending state. Finished tasks, read-only tasks and unreachable workers have no stop item. While a stop request is pending, its item is disabled.

## D-204 — Resolve the update card against the exact reply (T-296)

A completion reply used by an update notice also advances the phase and confirmed version in the same window render. For the same request ID, a final reply takes precedence over older shared watch or presence data, and an installation reply takes precedence over a waiting phase. The broker still distributes update changes to all windows. A reply cannot close a different request. The cancel cross stays available while a supported request waits. The installing indicator shows when the installer owns the request.

## D-205 — Tooltips for clipped notification text (T-297)

Keep the footer's three-line limit from T-181. Expose the full text on that part of the notification only if it is clipped at the current window width. Recheck when the footer changes size.

## D-206 — Exact update cancellation tolerates stale presence (T-298)

The update watch can already identify a waiting request while the roster is late. Keep its cancellation control for a supported device and queue the exact request through the current broker. The receiver checks the request and decides whether cancellation still applies. The implicit coordinator command still needs fresh presence to select a request, and the window hides cancellation during installation.

## D-207 — Retain useful helper logs (T-299)

On the first journal write after startup and at most once per day thereafter, remove generated startup and remote-update logs last modified more than seven days ago. Keep the current process's output files and the newest nonempty file of each kind. The event journal uses its existing size-based rotation.

## D-208 — Close the idle coordinator timer proposal (T-300)

The owner removed the proposal from the plan. On macOS, `codex queue --resume` started a new turn in the same finished chat with a reachable isolated App Server and simulated model responses. Both the standalone CLI and the CLI bundled with the desktop app passed. The desktop setup under investigation did not expose the default control socket. The isolated no-shared-server run warned that the thread could not be resumed, returned exit code zero, and started no new turn. Keep the waiting behavior in D-124.

## D-209 — Read idle computers without a write lock (T-301)

After reading events for `wait`, look up idle computers in read-only SQLite transactions. Read task state from the selected project and computer identity and names from machine storage. The legacy layout uses the same read snapshot for both. Batches without a completed-task event skip the lookup. In the isolated MCP timing probe, the writer-held case fell from about 1.27 seconds to 0.23 seconds for both client identities. This measures delivery of a committed report through an active wait.

## D-210 — Expand parallel agent tasks in computer cards (T-302)

For multiple active tasks of one agent, show the agent and localized task count. A disclosure reveals each task's confirmed model and elapsed time or waiting status. A task title identifies a run whose model is still unknown. Group opening and focus survive redraws within the current project and network. Single-task lines keep their compact format.

## D-211 — Show the stop request in place of the task controls (T-303)

While stopping a task, show the submission, queue or sent status in place of the composer field and its controls. The task context menu shows the same status as text. Resolve a retained menu listener against the current task row so a broker acknowledgement cannot bring back the stop action. Keep the local acknowledgement through refresh until the task records an outcome. An explicit refusal returns the composer with its draft.

## D-212 — Mute idle agents and fit both computer roles (T-304)

Use the same muted color for idle agents and other secondary text in computer cards. When the local computer hosts the network and coordinates the selected project, join the two role labels with a comma. Keep the group beside the computer name if it fits with the presence mark, version, and update action. Otherwise place the group below the name, keeping both labels readable at the minimum width. A card with one role keeps its layout.

## D-213 — Compact waiting rows and computer roles (T-306)

The owner replaced the second-line role layout in D-212. Show both roles on the heading line. Use the full labels when they fit, abbreviate the coordinator label when needed, and show labeled icons when the abbreviation also needs more space. Use a single icon labeled with both roles if the update control leaves less room. Reassess after resizing or an update control changes the available space. In an expanded agent group, the model or task title and the short waiting status share one yellow line when space permits. The detailed reason, including who should reply, stays in the tooltip and in the task pane.

## D-214 — Align the computer-card heading (T-307)

Give the computer name and roles a common text baseline. The labels share a 21 px line box, with 13 px for roles and versions, and 14 px for the computer name. Center the version and draw the monitor, role and update glyphs in centered 14 px squares. Keep the larger clickable update control and the matched online dot and offline cross on the monitor.

## D-215 — Separate computer contact from task launch evidence (T-308)

The broker reports authenticated contact age separately from agent report age. Both ages continue to increase while the viewer receives no new roster. The computer card and a task's remote contact state can use either fresh signal. Worker state still requires a fresh agent report. Presence reads take a read-only SQLite snapshot. The task list uses a fresh worker entry from the assigned computer for the exact task when confirmation of the launch has yet to arrive. This changes the display and leaves task state, reports and resource ownership under the existing mailbox and supervisor rules.

## D-216 — Show available actions in task context menus (T-309)

The task context menu includes Remove from list when the existing removal rules allow it. If no action is available, the menu stays closed. An existing stop request still shows its status.

## D-217 — Show the agent while its model is unknown (T-310)

Replace the task-title fallback in expanded computer rows from D-210 and D-213 with the agent name. In a running row, append the confirmed model to the blue agent label and follow it with a muted comma and elapsed time. A waiting row uses its confirmed model or the agent name before the yellow waiting status.

## D-218 — Export the evidence and measure model efficiency (T-311–T-313)

Export one versioned JSON file on demand from Statistics. Apply the selected project, local calendar period, computer, agent and model filters. Use the finish time or, when none exists, the last update time. Stream the matching task records known to this computer beyond the ranked window limit. Include task IDs, a remote origin ID when recorded, titles, state, timestamps, duration and task-level usage. Use the last reported model for the model filter when present and fall back to the requested model, as on the page. Keep both identities and their effort levels distinct in the file. An absent usage total or reported model is unknown. Cache reads and writes are parts of input. Reported price, when present, may cover only part of the task. Export adds no background reads, network traffic, or schema migration.

Capture task type and difficulty as optional structured fields on the coordinator's assignment. Keep the labels set before work starts and out of the worker's prompt. Record explicit acceptance or return separately from execution state. Distinguish an implementation mistake, missing scope, unclear assignment, changed requirements, a technical failure and a model limit. Link a replacement task explicitly to its original within that project. Treat an independent follow-up as new work. For older tasks, leave absent labels, reviews and attempt usage unknown.

Capture provider-reported model, effort and usage once per run using its durable identity, including runs with partial usage and another model picked for the retry. Deduplicate mirrored results. Keep run attribution and issue-level totals separate. Evaluate the strategy of picking an initial model by counting all linked attempts through acceptance, including a stronger model finishing a cheaper model's work. Count the outcomes that ended without acceptance and the fraction that was reviewed alongside the accepted cohort. Use active runtime and elapsed time through acceptance as different measures.

Compare within project, task type and difficulty. Show sample and coverage counts next to average, median and nearest-rank P90. Count first-pass acceptance among reviewed original tasks and show how many are still awaiting review. Distinguish uncached input, output, cache reads and writes. Cache behavior and model prices differ, so raw token counts alone do not establish monetary savings. Keep tasks where the actual model is unknown in their own coverage count. Where enough comparable work exists, rotate eligible models on low-risk assignments with the same acceptance checks before adjusting the model guide. The historical task totals and execution states are suitable for a first baseline. They cannot establish first-pass quality or recover usage by run.

`start_task` accepts optional `task_type`, `difficulty` and `retry_of`. `review_task` takes `task_id`, `op_id` and `result`. For `returned`, include `reason`. Pass `report_at` from `tasks.finished_at` to guard against reviewing a newer outcome. Clients use the fields and tools advertised by their connected MCP server. One explicit decision belongs to one outcome. The task state and retry policy are independent of that decision.

Three additional tables hold assignments, reviews and attempts in the corresponding project database. Migration creates the tables without rewriting existing task or run rows. A legacy database or a project snapshot can be read before those tables exist. On the wire, assignment and outcome associations are optional. A small attempt record follows the existing durable mailbox and older executors can complete work without it. The worker prompt and base task/result fields stay compatible. Review and model comparison do not require a model call.

The comparison uses the original task's creation date and initial reported model. If historical evidence contains exactly one local execution and an observed model, use that model for its initial identity. Otherwise leave missing initial identity unknown. The acceptance rate divides accepted tasks by reviewed tasks. When a model filter is set, the requested selection can find these unknown cases. Include linked tasks even after a model or machine change. Display first-pass results only when comparable attempt evidence exists. A cumulative token or runtime sample requires continuous attempt coverage and an outcome matching the latest attempt for each task in the chain. Token samples also reconcile with the recorded task totals. Timing from assignment to review is available separately. Missing telemetry leaves the sample out. The window limits the displayed groups and the JSON includes all of them. There is no background comparison refresh.

The period tabs, agent and model filters stay together on the left. The chart and upward export buttons sit on the right and can wrap together in a narrow window. Each has a localized tooltip and accessible name. The chart button opens a table grouped by type and difficulty. Each row shows the group task count in muted text beside the model name, accepted and first-pass percentages, median token usage, and median time to acceptance. The count represents the entire group. Each percentage keeps its own denominator. Hover or focus a percentage to see the exact fraction. Groups share one background and have a rule between them, with matching vertical insets above and below the rule. The leading row arrow opens a rounded panel with the outgoing message background and radius. Its table shows the mean, median, 90% threshold and sample size for token usage, active runtime and time to acceptance. The threshold is the value at or below which at least 90% of measured tasks fell. The JSON carries the original counts and `p90` field.

## D-219 — Show one centered update indicator (T-314)

The circular update arrow uses a centered arc and a rotation origin at the center of its SVG view box. Remote update cancellation keeps its existing behavior. During a local Mac download, installation or restart, the visible glyph switches from the download arrow to the circular arrow. Download percentages remain in the progress dialog. Clicking its backdrop keeps the dialog open while the update is active. After an error, the dialog can be dismissed and the download arrow returns if the release is still offered.
## D-220 — Close and copy pictures in the viewer (T-315)

The Mac title bar pauses its drag zones while a modal is open. This lets the modal receive pointer input across its header and backdrop, including the picture viewer's close button. The main window's drag zones return when the last modal closes.

The picture viewer's Copy button puts a PNG of the selected image at its original pixel dimensions on the system clipboard. The current zoom and position belong to the viewer. Success and failure are reported inside the viewer.
## D-221 — Show two selected projects in one computer list (T-316)

Show the split control beside the project heading when there are at least two available projects. Each side has its own project picker. Both selections share the computer list and order. Inside each card, an inset divider separates the two groups of agent names, models and statuses. Expanded agent rows use the computer card's surface. The right pane shows one active project and computer selection. A change to the inactive picker preserves the active conversation and its draft. Closing the split retains the active project and restores the normal layout.

The inactive column reads only current local and shared activity from the existing project records. Reported project folders and observed work determine which peers are shown as part of each project. A peer with no readiness report keeps its normal agent inventory.

With the split open, Statistics starts at Selected projects. Its project picker can narrow the calculations and JSON export to either project without changing the active conversation. A combined request carries both project keys. Totals and task order are calculated across their available records, and projectless peer totals are excluded. Model comparison preserves separate cohorts per project. The combined JSON includes the selection in metadata and identifies each task and model cohort by project. Single-project export keeps its existing shape.

## D-222 — Keep sidebar card spacing when showing two projects (T-317)

The computer card body owns the vertical inset in both modes. Project columns add horizontal padding only. Collapsed agent summaries and project status labels truncate within their columns. The taller column determines the space a card needs when there are additional agent rows or expanded parallel task details.

## D-223 — Fill the selected computer and project (T-318)

In split mode the selected project's half is filled from top to bottom, including the shared computer header. Each half selects that project's computer from either the header or the agent area. Controls within the card keep their own actions. In single-project mode the same fill covers the full card. Selection changes the background without changing card geometry.

The window remembers the last computer filter for each project while it is open. Selecting an active half again clears the filter for that project. The task list and conversation still follow one active project and computer.

## D-224 — Use one agent roster per computer in split mode (T-319)

Both columns show the computer's enabled agents even when it has no folder for one of the projects. A paused agent with active tasks in either column remains listed in both. A computer that reports no agents shows the existing no-agents message in both columns. Project activity decorates those rows with running or waiting status, model, and time. If activity is still loading or unavailable, the rows remain visible with a status tooltip.

This replaces the sidebar's folder-based filtering in D-221. Reporting configured folders to the coordinator and validating a task's destination continue to use the existing readiness data.

## D-225 — Distinguish the project halves with color (T-320)

The right half uses a neutral tint in place of the divider. A neutral hover highlight covers the project half under the pointer, including its portion of the shared header. Hover and click use the same card midpoint. The blue selection fill takes precedence. Card geometry and single-project styling stay the same.

## D-226 — Label both visible projects' live local coordinator (T-321)

In the split sidebar, the shared header shows the coordinator label when one or both displayed projects have a live local coordinator. The tooltip lists those projects. Both snapshots use D-165's process identity check. Server roles still come from network status.

## D-227 — Let the workspace stay open while an update is waiting (T-322)

Choosing to wait dismisses the restart prompt and shows a status button. It opens the waiting details and their close button only dismisses those details. The window keeps submitting unforced restart checks while the older client is running. In split storage this checks all projects irrespective of screen selection, and an unknown count keeps waiting. Exiting the desktop app stops the window's checks until it opens again.

## D-228 — Separate the project tiles with the picker gap (T-323)

Each computer keeps one row in the split sidebar. Two independently rounded tiles use matching neutral colors and the project pickers' gap from top to bottom. The left header holds the computer name and roles, and the right holds the update control and version. Hover brightens the pointed tile, and selection fills the chosen tile blue. The gap is inactive. This replaces the right-half tint and shared header from D-225.

## D-229 — Switch the Statistics project when a computer tile is chosen (T-324)

Statistics opens with the combined project scope until the user picks a project in Statistics or clicks a split computer tile while on that tab. A tile applies its project to the existing Statistics project picker and its computer to the computer filter. Clicking the active tile again leaves the project selected and clears the computer filter. The picker still offers Selected projects for combined totals. The same scope applies to model comparison and JSON export.

## D-230 — Show both displayed projects' tasks with no computer selection (T-325)

In split mode the right panel groups tasks by the two displayed project titles whenever the computer filter is clear. Each group includes all computers, the usual task status sections and its own completed-history pages. The active conversation remains singular. A task from the other group opens in its own project while the combined list remains visible. Selecting a computer tile scopes the list to its project and computer. Deselecting it restores both groups. Bulk actions remain scoped to the group that contains the button.

## D-231 — Preview the computer-grouped task sidebar (T-326)

The chosen layout groups current tasks by computer under Projects, with an All or single-project scope. Show the project name on task rows in the All scope. Use a neutral gray row for selection, a visible gap between tasks, blue for the model while working and yellow for the model and waiting label. Keep the chat centered and use the existing icon to open the archive on the right, initially collapsed. Mark the local computer through the existing monitor outline and preserve its connection indicator and header spacing. Verify changes on the real window assets using fictional browser fixtures before bundling them in a later update.

Keep the same leading inset for every task title and subtitle as the left pane is resized. Extra width reduces wrapping. Use the section spacing token above and below grouped command summaries in the conversation, keeping the individual command lines compact. The loopback preview includes two command groups so the expanded and collapsed states can be compared.

Put the project picker beside the Projects heading. Use the compact spacing token within and between task rows, with the same gap between computer groups, whether they are collapsed or expanded. In the All scope, put the muted project name and a comma before the task title. Both use the same regular weight. Reserve the second line for the model and elapsed time or waiting state.

Use whitespace between computer groups. The full computer header is the disclosure button when it has tasks, with a row-wide hover and focus fill and separate update controls. Match the gap below an expanded header to the compact gap between task rows. Statistics has its own project and computer selectors, covering all registered projects and the whole network by default. Keep efficiency and export controls at the right edge of the filters. Agents has its own computer selector and starts on the local computer. Its remote view remains read-only.

Leave a control gap below the Projects divider and use the same trailing inset for computer headers and task rows. Join the agent and model with a space and append the role after a comma. Use the same typography throughout the author label, with a muted role. In computer headings, append each role after a comma in the same typography as the name, with the coordinator before the server when both apply. Use a six-tooth gear icon for Settings and group it with Feedback at the start of the desktop sidebar footer. Give each loopback task its own conversation with coordinator and executor messages and two command groups for reviewing line wrapping and paragraph spacing. Animate the task list height when disclosing a computer. Omit the disclosure arrow and keep the full header interactive, with the selected task fill already present. Use immediate updates in reduced-motion mode.

Show only the centered creation date and time in the preview chat header. Keep the displayed date and conversation in place while a selected task loads, including across projects. Its controls are inactive during the transition. Replace the date and messages together when the selected task is ready. Use the loading pane when there is no conversation to keep. Keep worker inactivity notices in the conversation.

Use the same compact vertical padding on computer headers and task rows. Share task card padding, radius, and hover and selection fills between the two task lists. Keep the compact gap between neighboring cards. Drag by the computer header to reorder the whole group, or use Alt+Up/Down while the header has focus. Show the visible header with the computer name, icon, and version as the drag image, using a translucent fill and a blue insertion line between computer groups. A click still discloses the group. Save the order through the existing machine-order endpoint. The loopback preview retains fixture ordering in memory across page reloads.

Let computer headings blend into the sidebar at rest, with their translucent drag image preserved. Use the same neutral hover fill for computer headings, unselected task rows and quiet icon buttons. Keep selection brighter. Give each task row and its section label equal 12px horizontal padding. A wider task panel makes the card wider and the text less likely to wrap; the title remains at the left inset and the date at the right inset. Outgoing message bubbles use 12px on all sides.

Remove the tab strip from the conversation and place the list toggle at the top right, directly before the window controls in the frameless layout. Open General, Agents, Project, Server, and Statistics from the footer gear in a large modal with internal navigation, preserving the conversation behind it. Keep the local version and updater in the footer, with the right-aligned version matching remote version typography. Keep Settings and Feedback at the left and the updater immediately before the version, using the same glyph size, button size and neutral rounded-square hover fill for all three. The footer and computer rows use the same 4px gap between the update button and version. Keep a compact gap between a project heading and its first archived task. Use matching icon and hover-area sizes on remote update controls without increasing computer header height. The matching left and right task card spacing includes a 2px gap between text lines. Reveal the remote version and update control on computer heading hover or keyboard focus, with persistent visibility on touch-only layouts.

The left panel contains running, waiting, planned and attention-needed tasks. Archive contains finished, failed and cancelled tasks, grouped by project in the All scope. Its agent filter and pagination apply to the archive. Extend the sidebar dividers under Projects and above the desktop footer to the panel edges while keeping their content inset. Match the Projects and Archive header alignment, spacing, and filter styling. Extend the Archive divider to both panel edges and the vertical border to both workspace edges on desktop. Use the shared 16 px composer surface radius and 8 px control radius.

Start Archive cards with the subdued completion time, using capitalized Today and Yesterday with the time and a calendar date with the time for older tasks. Show the computer in the time tooltip along with the task dates. Follow the time with the sidebar-style task title, including the comma-separated project prefix in All mode. Show agent and model, duration or terminal status, and reported tokens below, separated by commas. Abbreviate the archive token unit to t. and keep the complete count in the tooltip. Use the same compact provider logo in each task list. Both lists ellipsize long titles and metadata in one line and expose the full text in tooltips. Keep project grouping for pagination, without separators. Put Clear archive above the rows. It applies to the selected project and agent scope and retains records used by statistics. Use a compact inset in General settings.

Keep the owner composer editable in an active task while Send is temporarily unavailable, including a lost remote connection. Preserve the draft when the same open task changes between a message and a question. Keep routing through the owner window and use the confirmed task state when sending.

In the Projects sidebar, align active-task text with the computer name. Include the computer icon width and label gap in the task's leading inset. Keep the hover, focus, and selection surfaces full width. Align the chat heading with Projects and Archive, showing the full task name above the date or New Task in the draft. Show individual message times on hover and keyboard focus, above the worker header on the left and below outgoing messages on the right.

## D-232 — Start a task in the owner window (T-327)

Put one always-visible New Task plus immediately to the right of the Projects picker. It opens an empty conversation with the project and computer below the message field and the model aligned to their right. Selecting a task leaves the composer. The draft project defaults to the selected filter or current conversation project and the model picker uses the agent models listed by the chosen computer. Sending uses the same task admission and operation ID as other task starts, with the window as the assigning owner.

A direct question or report goes to the owner window. Finishing a report closes the task through the usual run outcome handling. An older worker can still send a question with the coordinator audience; the initiating window owns the answer and forwards it with the audience the worker expects. Coordinator assignments retain their existing routing. The preview uses fictional conversations and a simulated report.

## D-233 — Open Statistics separately from Settings (T-334)

Put a chart icon beside Settings and Feedback. It opens a Statistics dialog with the existing charts, filters, efficiency report, task links and JSON export. Settings opens at General and lists Server, Project and Agents below it. Use the existing outline icons for the section labels. Both dialogs are centered and share a 900 by 640 px cap, with proportional margins on smaller screens and internal scrolling. Closing either dialog returns to the open conversation and keeps its draft. The consumption and efficiency reports occupy the same position below persistent filters and use the shared card heading. Changing filters keeps the selected report open. Model names open the details directly without a disclosure arrow. Models are indented one control gap from the group title and the details panel one more. The Agents section uses an outlined robot icon.

Use the caption height for the Settings, Statistics, agent settings and model picker headers. Agent settings place controls opposite each label and use the standard divider between rows. The model picker uses the standard input style and scrolls its list independently with spacing beside the scrollbar.

## D-234 — Balance message and task insets (T-334)

Use the content spacing token vertically and the section spacing token horizontally in outgoing bubbles to account for the text line's internal space. Use the compact inset for archive rows and a control gap above and below active task text. Active task titles still start at the same position as the computer name, and both lists' highlights span the available row. Use 16 px glyphs for Send and Stop in the existing controls with a local hover fill against the composer background.

## D-235 — Put task context below the composer (T-335)

The New Task composer starts with a one-line draft between Add and Send. Put project and computer selectors below the field on the left, and the model selector on the right. Use the reply model’s secondary text style for all three selector values. Search configured models for the chosen computer in a popover with a common horizontal inset for its search field and option rows. Mark selected options with a check and reserve the row fill for pointer hover and keyboard focus. Add inserts a path as a line of task text. Offer the system file and folder picker only when the selected worker is local. Preserve task start routing and configured model availability in D-232. Use the composer radius for controls inside the field. Outgoing messages show a text copy action beside their time on hover or keyboard focus.

## D-236 — Use compact controls and clarify composer actions (T-336)

Match ordinary actions, one-line inputs and picker triggers to the compact Projects control height and a neutral gray fill. Preserve quiet controls and keep the destructive action distinct. Both composers use the same field padding, focus border and upward Send arrow. The active task retains its separate Stop control. The composer keeps its larger hit areas with corners inset from the field radius. Use its Add menu in both the draft and the reply, with separate file, folder and manual path actions. Insert paths as text. File and folder pickers use the local system dialog.

Show the active model and a neutral ring that opens the token popover below the reply field. Use 4 px vertical insets for computer headings and active tasks and retain 8 px in the Archive. Distinguish the current turn counter, recorded task usage and model context occupancy. Show a missing value when the window has no context occupancy data. Use the same neutral icon for the local computer and its peers, retaining their connection status. Show the local access setting on a draft and the stored access mode for a local run. For a remote worker, explain where its access is configured. These indicators sit at the left above either composer. Show the outgoing message timestamp and copy action together under the message. Omit the worker message's hover timestamp. Apply display-only syntax colors to recognized shell commands. While a new efficiency filter loads, show its loading state with the existing toolbar in place.

## D-237 — Simplify model rows and align the network name (T-336)

Separate model choices with their row padding and keep the model search and list scrolling independent. Place the network-name control at the end of its label row using the 12 em width shared with language and theme. Cap it at half the available row so it stays beside the label on smaller windows.

## D-238 — Align settings openings and inset statistics highlights (T-336)

Place the section headings above their settings cards with a common gap. Place the first control opposite its label on the same baseline across sections. Reserve the same scrollbar space on every settings page so the control's right edge stays fixed. Keep the project name's explanation below the inline field and leave the path control full width. Statistics task links and efficiency model buttons share the computer row's neutral hover fill and 4 by 12 px insets. Keep the task text aligned to its column heading. Use the compact dialog header for Feedback and New Project as well as Settings, Statistics, agent settings and model choices.

## D-239 — Give report rows one action and show measured context use (T-336)

Use the short Settings section names above their cards. Highlight the full row when hovering a Statistics task or an Efficiency model and activate the row's existing action from any column. Keep keyboard actions on native buttons. Use symmetric row insets and keep the date and time together. Use a target for Efficiency and retain the chart for Statistics.

Draw the reply context ring from the used and capacity counts in a context snapshot. Put the percentage on the first line and the used and capacity figures on the second. Keep the ring neutral when a snapshot is unavailable and label any reported cumulative usage by its scope. The loopback preview can use a fictional snapshot.

## D-240 — Refine conversation and report spacing (T-336)

Left-align the context popover, put the percentage beside its label, and separate used tokens and capacity with a slash. Keep the task title as the sole conversation heading and center the date at the start of its scrollable thread. While a computer is collapsed, append a muted parenthesized count of tasks under the current project filter.

## D-241 — Align pane headers and normalize surface insets (T-336)

Align the conversation header divider with Projects and Archive. Keep the title to one line and expose its full text on hover. Use the content gap between the heading, body text and actions in confirmations. Use equal surface insets on all sides, with 12 px for report cards, efficiency details and outgoing message bubbles, 8 px for menus, and 16 px for settings cards and confirmation dialogs.

## D-242 — Span the task toolbar above Archive (T-336)

Use a single toolbar above the conversation and Archive, with its divider aligned to Projects. Place the toggle at the right end of the toolbar and give the Archive its own header below it. Frameless native windows keep the toggle beside the window controls. Match footer text to the visible icon inset. This updates the pane layout in D-241.

## D-243 — Align expanded efficiency details with their model row (T-336)

Use the section inset on the leading side of the expanded panel and align its trailing edge with the parent row. Keep the shared content inset inside the panel and align the first metric with the model label. Use a speedometer for the Efficiency action, superseding its target icon in D-239.

## D-244 — Use a visible Clear Archive button (T-336)

Use the compact filled action and align the button itself with the archive rows. Keep the confirmation and filtered scope.

## D-245 — Guide first-run task setup in the task pane (T-336)

Use the shared card and setting rows for the first-run checklist. Give each button an action label and link it directly to its setup location. Creating a project and choosing an available agent and model enables direct tasks. The network and coordinator can be configured separately. When no conversation is selected and the current project has a dispatch route with an enabled model, open the standard New Task composer. This refines the empty-pane behavior in D-186.

## D-246 — Order the settings sections by setup flow (T-336)

Open Settings at General, followed by Project, Agents and Server. Project and agent setup enable local tasks. Server settings are available after those sections for optional networking. This supersedes the section order in D-233.

## D-247 — Keep the original repository and installation identity through the Kotel bridge (T-337)

Use Kotel in the window, documentation and distribution labels. Keep the existing Ed25519 key, platform bundle and program paths, database directories, broker protocol, active-run registry and MCP keys. This allows mixed-version operation and preserves custom integration commands. Use the original GitHub repository ID for update discovery. A repository rename follows an explicit inventory of every installation running the bridge and the owner's command. The download names for both brands are byte-identical for each binary. Apply release corrections as a higher signed build.

The owner approved the rounded golden tile with a black pot, tilted lid raised on the right and solid right-facing pupils. The window, platform packages and repository use this artwork.

## D-248: Complete the Kotel naming transition after the bridge

Build 116 preserves the existing integrations so installed clients can update. The owner wants the old names retired from the active product, including existing MCP coordinators. Migrate app-managed entries on reconnect, preserve permissions and conversation identity, and resolve custom entries or name collisions with their owner. Verify one tool namespace and successful calls for each actual coordinator before removing its old aliases. Apply the same acceptance to the CLI, installers, services, paths and current downloads. Retire an alias when every dependent installation and offline profile has migrated. Preserve Git history, previously signed releases and the user's existing messages.

The directory move is a separate step in the signed transition. Fresh profiles use the new Kotel directory. Existing profiles keep one physical data directory and one supervisor lock shared by both paths. Move the complete directory on the same volume, including SQLite and its sidecar files, device keys, the pending event queue and the untouched run registry. Quiesce the old desktop app, services and MCP sessions before the move, hold the migration lock across the handoff and defer while a run is active or blocked. A restart must distinguish a completed move from two conflicting directories. Keep the old path as a temporary alias and check the previous signed build, the new build and rollback against the same data. Verify device identity, project access, chat history and pending events on a pilot before enabling the move across the fleet. Change the GitHub address and retire aliases only after the installed clients can resolve the signed update and their coordinator connections have been verified, including computers that were offline during the rollout.

## D-249 — Worker reply actions and default archive width

The owner wants a worker reply to offer the same timestamp and full-message copy action as an outgoing message. Place the controls below the reply on the worker's side. Archive and Projects start at equal widths and retain their own saved widths. The revised Mac header remains a proposal.

## D-250 — Put Projects below the Mac caption

On a wide Mac window, place Projects one control gap below the task title bar. Keep the divider under the title and place an independent toggle for the computer and task sidebar at its top right. Closing the sidebar gives the width to the open task and preserves the archive state. Keep the existing stacked layout on narrow windows.

## D-251 — Align the project and archive controls

Extend the Mac caption divider over the Projects sidebar. Use the common 16 px section inset for both Mac sidebar header rows. Keep the archive header on its pane background without a divider below it. Keep the conversation date centered above the scrolling history. Place the archive clear icon beside the agent filter and identify the scope and history retention in the existing confirmation dialog.

## D-252 — Give New Task its own row above Projects

Place the icon and New Task text at the leading edge of the sidebar. Use the shared quiet button feedback and keep the archive toggle icon-only. Both sidebar filters use the Settings width of 12 em capped at half the available row. In narrow panels the filter takes a full row.

## D-253 — Keep live task lists and archive headings stable

Animate visible membership changes using a short fade and vertical movement. Keep a stable list still on status polls. Respect reduced motion. Count all configured projects independently of the current filter. Place Clear directly before the agent filter and keep the existing scope confirmation. The selected Mac header has a divider under the task only. This updates D-251.

The task model menu and Settings use the same family, numeric generation and effort ordering within each provider. Keep the suggested selection in its normal position. Preserve unknown model IDs and the stored configuration.

## D-254 — Scroll the conversation date with the messages

Keep only the title in the fixed task header. Place the centered date first in the scrolling conversation with the same top inset for each task. This supersedes the date placement in D-251.

## D-255 — Use Graphite for dark appearance

Use the shared Graphite colors for primary text (#c2c2c0), secondary text (#9b9b99), running (#8eadd2) and waiting (#ceb271). Apply the same palette when system appearance selects dark mode.

## D-256 — Compact sidebar grouping and presence

Keep computer grouping as the default. In project folders prefix each task with its executing computer. In the nested view the enclosing computer and project supply both labels. Use the same filter glyph on both sides and a separate grouping control. See [UI_STYLE.md](UI_STYLE.md) for spacing and computer presence.

## D-257 — Ship the combined post-117 release and retain the local updater

The next public release includes the approved window, worker features and managed MCP migration. The owner kept the local computer's version and update button in the bottom-left footer beside the existing controls. Network settings also offer local updates, and provide the versions, update and cancel actions for paired computers. The repository rename still requires the separate instruction recorded in D-247.

## D-258 — Publish the completed work in a preliminary update

The owner revised the release scope in D-257 on November 27. Show the Settings window before submitting the update, include the approved interface and completed functionality, and carry the unfinished provider migration and task steering into subsequent work. The existing signing, native update and public channel checks apply.

## D-259 — Keep the My network label inside its card

Use the same typography as Connection mode. Place this computer and its peers in one list with the same vertical spacing between all rows. Keep the connection controls in their existing layout.

## D-261 — Keep computer update notices outside the conversation

Place update notices above the conversation in a fixed area outside its scrollable history. General defaults to manual dismissal and offers automatic dismissal after eight seconds. Hover or focus keeps the notice visible. The three-notice limit applies to both modes. Keep confirmations modal.

## D-262 — Manage computers from the sidebar

Keep the original connection layout in Network settings and remove the duplicate roster. The context menu and local sidebar footer align the shared update control and installed version together at the trailing edge. Keep the same arrow, spinner and cancel icon. Project folders use the enclosing computer, or this computer in the project-only view. Open project settings with the selected project.


## D-263 — Brighten dark appearance and apply font preferences throughout the interface

Use #1e1e1e for the sidebar, #dededc for primary text and #a5a5a3 for secondary text in dark appearance. Keep the existing task fills during the visual comparison. Font selection applies to all ordinary interface text. System high contrast keeps the same font and size with brighter text in dark appearance and darker text in light appearance.

## D-264 — Keep task lifecycle with the originating computer

The computer that created a task writes its display title to the task record. The server already distributes titles in the shared task list. Reuse that distribution when the title changes. A rename initiated elsewhere requires an authenticated request to the originating computer, a durable result and a capability check for older installations. Archive is a reversible lifecycle state for finished tasks. Keep permanent deletion in Archive with explicit confirmation and define how it reaches disconnected copies. See T-371.

## D-265 — Open concurrent task views inside the main window

Use tabs for switching and closing task views. Use Open alongside to keep concurrent conversations visible in the same workspace, with side-by-side, stacked and grid layouts. Every pane addresses an existing task and keeps its own draft and scroll position. See T-372.

## D-266 — Stop a turn and keep the conversation available

The owner or coordinator can send another message in the same saved worker conversation after a completed, failed, stopped or interrupted turn. Resuming a blocked or interrupted turn requires proof that its previous worker can no longer run. A local close on an unreachable computer needs a terminal acknowledgement before that conversation can resume. Nothing retries automatically. A new turn clears the previous stop request and can be stopped independently. See T-373.

## D-267 — Project reference tabs

A project file link opens a tab in the existing task toolbar and a reader alongside the conversation. Explicit separate windows display a read-only text preview. File resolution continues to use the executor task and its registered local workspace. Relative references inside a document use that document's folder. A remote reference shows which computer holds the file.

## D-268 — Keep permission details beside the model in an existing conversation

The new task composer keeps its access choice. An existing conversation shows its access mode or executor computer beside the model below the reply field. Its scrolling viewport ends at the footer. Stop ends a turn and adds an unboxed system event. The original conversation can accept a follow-up when its recorded continuation capability allows it. See T-375.

## D-269 — Recognize a coordinator registration through the installed bundle link

A registration pointing to the migrated macOS bundle remains connected when the installer’s relative link leads to the current executable and the coordinator arguments match. This preserves the server name and configuration. A different target remains a conflict. See T-376.


## D-270 — Group tasks in either order and choose a folder from a draft

Offer project-then-computer alongside the previous grouping modes. Remember a computer's disclosure state separately inside each project. A task draft can open a folder through the system picker and create its project using the existing form. The task remains scoped to a registered project and a computer that can execute it.

## D-271 — Keep version and update actions in computer menus

The version footer and update actions in context menus belong to the selected computer. Project folder menus contain only project actions in every sidebar grouping. This updates the project menu behavior in T-365 and T-374.

## D-272 — Keep the composer editable while stopping a turn

The task owner can draft the next message while a stop is pending and after the turn ends. The send control respects the stop state and the saved continuation capability. Closing a stuck task remains an explicit recovery action with the existing process checks. See T-381.

## D-273 — Optional first-run setup

Offer a short setup on a new installation and let users close or skip it at any step. Use the same settings for language and appearance, agent and model, optional workspace, network and coordinator, and notifications. Direct chat is available after skipping. Keep the dismissal on that installation and reopen setup from Settings. Upgrades and migrated installations retain their existing configuration and do not trigger setup. See T-384. The owner paused the wizard after native review. Hide both entry points until the step layout and the agent selection flow have been reviewed again. Leave saved preferences and direct chat available.

## D-274 — Keep personal chats open after a reply

Keep an owner-initiated conversation in Projects until its owner archives it. Archive and restore preserve the conversation and its continuation capability. Coordinators can archive their own finished tasks through the project-scoped MCP action. Existing coordinator task history keeps its automatic placement.

Show turn activity while waiting for public text and use partial text when the provider supplies it. Keep process ownership and verification independent of the displayed reply. Show a process warning only if the recorded final event is at least one minute old and the run is still active. See T-385 and T-386.

## D-275 — Task shortcuts and composer inputs

A computer heading opens a draft for that computer. A project heading opens a draft for that project. Show the shortcut only on the outermost groups in the current sidebar grouping. Reveal it on row hover or keyboard focus and use the row highlight. Use the standard row inset. Keep it visible on devices without hover. Keep the existing project, model and availability rules. The context indicator labels the reported window percentage consistently across live and saved turns.

Agent settings group providers by detected installation on the local computer. Installation remains explicit and remote settings remain read-only. Installed rows show the provider, its enable switch and its settings action. In the agent dialog, use the standard divider after Models and status marks for CLI installation and sign-in in place of step numbers. Use 16 px provider artwork in window controls and retain the compact metadata marks.

The composer can add selected paths and UTF-8 browser drops as removable draft inputs. The selected or dropped local path is a reference for the executor on that computer. A path on another computer must be supplied explicitly. The slash picker runs supported window actions. Provider-native commands and opaque attachment transfer are tracked by T-388 and T-389.

## D-276 — Agent limits in the sidebar

Show measured usage windows in a popover beside App settings. Group snapshots by the reporting computer and display a subscription only when the provider reports it. Opening the popover reads the window's existing snapshots. Mark disconnected computers and passed reset times so an old measurement cannot read as a confirmed refill. See T-390.

## D-277 — Update checks and window notices

Expose a manual check in General settings, the window menu and a computer’s context menu. A selected computer’s menu displays its version without repeating its name. Review the list of eligible connected computers before requesting a network update. Each target verifies that its own signed artifact is compatible. Keep update feedback in the window’s notification area across navigation. See T-391.

## D-278 — Sleep modes

Expose system sleep, awake while a local agent runs, and always awake next to Settings. Persist the selected mode locally and release the operating system assertion when its condition ends. See T-392.

## D-279 — Composer capabilities

Offer a provider-native action only when the chosen provider and route can execute it. Keep the local Codex usage action backed by the provider’s account limits request. Send binary attachments with a new task only after explicit selection, require a compatible remote executor, and expose an isolated copy through the receipt-scoped worker tool. Follow-up binary uploads require their own transport. See T-388 and T-389.
