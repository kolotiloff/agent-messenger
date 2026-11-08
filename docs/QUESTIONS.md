# Open questions

Questions that need the owner's answer or a discussion before work can go on. Agents add questions here instead of guessing. When a question is answered, write the answer and the date, record the result in [DECISIONS.md](DECISIONS.md) if it is a decision, and move the entry to **Answered**.

Format: ID, question, why it matters, options, which tasks it blocks.

## Open

### Q-001: Which language should the app UI use?
The window and CLI messages are in Russian; code and docs are now in English. **Options:** keep Russian; switch to English; support both with a language setting. **Blocks:** T-019.

### Q-002: May the old remote branches be deleted?
There are about 26 `codex/*` and `cursor/*` branches. Their content is in `main`. Four of them have a separate history from before `main` was rebuilt, so they hold the original commit history. **Options:** delete all; delete only the branches already merged into `main` and keep the four; keep all. **Blocks:** T-018.

### Q-003: Is there an Apple Developer ID account for signing and notarization?
Without one, macOS can block the downloaded preview on another Mac. **Blocks:** T-013.

### Q-004: When can Cursor CLI be turned on?
Stage 2 requires all three CLIs. Until then Cursor stays off (D-011). **Blocks:** T-012.
**Answer (2026-11-07):** there is no such rule; the earlier request was a one-off. Cursor is set up as a task worker like Claude and Codex (T-012, D-049). Real model calls still happen only in the owner's acceptance runs.

### Q-005: How are acceptance runs on the Mac done?
Cloud sessions run on Linux. **Options:** the owner runs the steps from an agent's instructions and sends back the output; or the owner connects the Mac to the cloud session through Remote Control; or the owner works from a local tool (Cursor, Claude Code on the Mac). **Blocks:** T-004 and every later Mac acceptance.

### Q-006: Which Windows and Linux machines are available for stages 5–6, and when?
**Blocks:** acceptance of T-016 on real machines. The owner will test the preview.32 archives on Windows and Linux computers (2026-11-07); CI covers what it can (see [WINDOWS_LINUX.md](WINDOWS_LINUX.md#what-is-verified)).

### Q-007: How does the managed Codex profile sign in?
Copying the owner's credentials into the managed profile is not implemented and must not be logged or published. A likely option is a standard login into the managed profile done by the owner. **Blocks:** T-004.

### Q-010: Should workers load Messenger tools without Claude's ToolSearch?
Claude Code 2.1.319 defers MCP tools behind `ToolSearch` by default. A real model should find `messenger_task` through it, but this was not verified ([report](reports/2026-11-06-task-workers-linux.md)). `ENABLE_TOOL_SEARCH=false` for workers makes them visible at once but changes the owner's normal session behaviour (D-023). **Chosen for now:** Claude's default; check in T-027.

### Q-012: Remove the persistent Claude stream and Codex app-server now (T-028)?
The owner asked to clean the program of old code. The old "message a live session" paths (persistent Claude stream, Codex app-server owners, their window pages and commands) are the biggest part of it. Tasks and the coordinator do not use them. But sessions registered on the owner's Mac through them keep their reservations in `active-runs.json`, which are never released automatically. **Options:** remove them in a separate release after the owner closes the old sessions (the client then refuses old commands); or remove them now and leave old reservations to manual closing; or keep them. **Chosen for now:** keep them; only unused code and `experiments/` were removed (D-032). **Blocks:** T-028.

### Q-013: Where should a computer keep its broker device key?
DEPLOYMENT.md asks for the OS secure storage (Keychain on the Mac). Today the key is in `server-connection.json` in the data directory with mode 0600, readable by the owner's account only; the window and `status` never show it. **Options:** Keychain on the Mac and the Windows credential store, with the file keeping only a reference; or keep the 0600 file. **Chosen for now:** the 0600 file, because nothing crosses computers yet and the Keychain needs Mac-side work that cannot be tested here. **Blocks:** the rest of T-015 (before tasks travel between computers).

### Q-023: Should «Удалить» on Windows offer to delete the data too?
Asked 2026-11-08 (T-053). Removing the program keeps `%LOCALAPPDATA%\AgentMessenger` (tasks, history, settings, `active-runs.json`) and says where it is. **Options:** keep it always (now); or ask in the uninstaller whether to delete it too, when no task run is reserved. **Chosen until answered:** keep it always; the owner can delete the folder by hand.

### Q-022: How should «Удалить» and «CLI неисправен» on agent cards work?
Asked 2026-11-08 (T-061). The approved cards run the agent's own removal command in a terminal after a confirmation, and show «CLI неисправен» when a task proved a file missing. Removal commands differ by how the agent was installed (installer script, npm, Homebrew), and guessing one could remove the wrong thing; tasks do not yet report a missing file in a form the window can read. **Cautious choice:** «Удалить» opens the agent's official page after a confirmation; «неисправен» is not shown. **Owner:** keep this, or name the commands you use for each agent? **Owner (2026-11-08):** the agents were installed by an agent (Cursor), not by the owner, so the install method is not known. Keep the cautious choice: open the official page.

## Answered

### Q-019: Should a project rename reach other computers and the coordinator chat?
The owner asked to rename a project on the «Проект» tab. The project's name links it to other computers (a task for another computer finds the project there by that name) and to the coordinator chat (it connects with that name). The careful variant built now (T-065, D-072): a rename changes only the name this computer's window shows; the original name keeps working underneath, and the tab says which name other computers and the coordinator use. **Options:** keep it local; or rename everywhere, which needs the other computers to agree and the coordinator chat to reconnect. **Owner (2026-11-08):** keep it local, on this computer only.

### Q-020: Should the release numbering itself change (for example 0.1.0-alpha.N)?
The window now says «v0.41» (D-073), but releases are still tagged 0.0.0-preview.N, and the Mac updater and the Windows/Linux self-updater compare that. Renumbering needs a version that still sorts above every 0.0.0-preview.N already installed. The careful variant until answered: keep the tags, change only the shown name. **Owner (2026-11-08):** «v0.42» as agreed; tags stay.

### Q-021: Wording of the server connection status
Asked in chat 2026-11-08 (T-067). **Owner (2026-11-08):** «Подключено», «Нет связи», «Не подключено» are fine.


### Q-017: How short may the invitation be (T-052)?
The owner wants a 6-digit code to type from memory instead of the long one. The long code carries the host's address, its certificate fingerprint and a 32-byte secret; six digits cannot carry them, so the joining computer has to find the host on the local network and both sides have to prove the code without sending it (a password-authenticated key exchange), or anyone on the network could try all million codes or pose as the host. **Options:** (a) 6 digits with discovery on the local network and the key exchange, 3 tries, 10 minutes; the long code stays for other networks; (b) 6 digits plus the host's address typed by hand; (c) keep only the long code. **Chosen until answered:** (a), the most careful one that still gives a short code. **Blocks:** T-052. **Built (2026-11-08):** (a), D-081.

### Q-016: A real program window on Linux without Chrome?

Asked 2026-11-08 (T-046). On Linux the window is a Chrome, Chromium, Edge or Brave window in app mode (D-054). With none of them installed (Ubuntu ships Firefox, which has no app mode) the page opens in the default browser, as the owner saw. A built-in window needs the system's WebKitGTK through cgo, so every Linux build (x64 and ARM) would be built with GTK libraries and would not start on a system without them. **Cautious choice:** keep D-054; tell the owner to install Chromium (`sudo snap install chromium` on Ubuntu) for the own window. **Owner:** keep this, or build Linux with WebKitGTK? **Answer (2026-11-08):** the owner asked for a real window; built as D-078 without cgo, the browser stays the fallback.

### Q-015: Which limit fields do the real CLIs print?

Asked 2026-11-08 (T-044). Not checked on real agents. Claude's `rate_limit_event` is read for its status and reset time; a share used is not taken from it because its units are not known here. For Codex the `rate_limits` object (`used_percent`) is read wherever an event carries it; whether `codex exec --json` prints it is not known. Cursor prints no limit that Messenger knows. **Cautious choice:** show only what was read, mark it as the state at the end of the last task, never act on it automatically. **Owner:** after a real task of each agent, tell whether the card shows «Лимит».

### Q-014: Should a computer run tasks of other computers by default?
With T-037 a coordinator on one computer can start a worker on another computer of the network. **Chosen (cautious):** off until the owner turns on «Задачи других компьютеров» on the «Сервер» tab of the computer that would run them; a refused task fails with the reason. **Alternative:** on by default for every computer the owner invited. **Settled (D-046):** the owner asked the agent to decide open questions on its own; the cautious choice stays. The owner can still change it. **Changed (D-056, 2026-11-08):** after testing two computers the owner found the default refusal confusing; tasks are now accepted by default and the switch turns them off.

### Q-009: How does the window allow task launches?
The client launches task workers only with `serve --allow-tasks` (cautious default, T-024). The client the window starts today runs without model permission, so tasks stay recorded but are not launched until the client is started with the flag. **Options:** a confirmed "allow tasks" switch in the window, remembered per data directory; or start the window's client with `--allow-tasks` always; or keep the explicit command. **Chosen for now:** nothing is launched without the explicit flag; the window shows that tasks are waiting for permission. **Blocks:** nothing in code; affects T-027. **Answer (2026-11-07):** the window's client always allows tasks and is resumed when the window starts it (D-029).


### Q-011: May a Claude worker edit files in the project folder by default?
In the T-027 run on the Mac, a Claude worker (`claude -p`, worker profile) could not edit `hello.txt`: edits need permission, and a headless run cannot ask. The owner allowed it with the project's `.claude/settings.local.json` (`"allow": ["Edit","Write"]`), and the resumed run succeeded. Codex workers write to the folder through `--sandbox workspace-write`. **Options:** start Claude workers with `--permission-mode acceptEdits` (edits in the project folder, shell commands still under the project's rules), like Codex; or a per-project switch in the window that writes this setting; or keep it to the owner's own settings. **Answer (2026-11-07):** `acceptEdits` for the worker profile (D-028).

### Q-008: Switch to "one task, one session" with the coordinator in the owner's app?
Proposal version 3 in [RETHINK.md](RETHINK.md): the coordinator is the owner's own Claude or Codex chat with Messenger connected; every task gets a new worker session that reports and closes; the coordinator waits with a blocking `wait` tool. Replaces versions 1–2, which Codex reviewed in [RETHINK_REVIEW.md](RETHINK_REVIEW.md). It would replace D-005 and D-016 for new work and relax D-004 in part. The owner decides:
1. **The model itself**: accept, accept with changes, or keep persistent processes.
2. **Plan approval**: approve each `start_task` in the app, or approve the whole plan once in the Messenger window.
3. **Worker permissions**: full tools in their workspace, or a sandbox limiting writes and network.
4. **Ownership statement**: accept that a process left behind by a finished task is cleaned up on a best-effort basis (a strong guarantee needs an OS sandbox).

**Blocks:** T-009 and the stages after it. T-022 does not depend on it.

**Answer (2026-11-06):** accepted. The coordinator is Claude for now (Codex or Cursor later); planning and approval are agreed in chat under each project's own rules; workers run with normal tools and their project's settings; a clarification resumes the same task session, a new task gets a new session; leftover processes are cleaned up on a best-effort basis. Recorded as D-021, D-022, D-023.

## Q-018 (2026-11-08) Logs of other computers

The owner suggested fetching any computer's Messenger log over the network for debugging. The careful variant until answered: only a computer's own owner-approved coordinator can ask; the log is shortened (last N lines), paths and task texts stay, session IDs and tokens are removed; nothing is sent without the computer's switch «Отдавать журнал по сети» on. Open: should the switch be on by default, and how many lines?
