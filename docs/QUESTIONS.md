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

### Q-005: How are acceptance runs on the Mac done?
Cloud sessions run on Linux. **Options:** the owner runs the steps from an agent's instructions and sends back the output; or the owner connects the Mac to the cloud session through Remote Control; or the owner works from a local tool (Cursor, Claude Code on the Mac). **Blocks:** T-004 and every later Mac acceptance.

### Q-006: Which Windows and Linux machines are available for stages 5–6, and when?
**Blocks:** T-015, T-016.

### Q-007: How does the managed Codex profile sign in?
Copying the owner's credentials into the managed profile is not implemented and must not be logged or published. A likely option is a standard login into the managed profile done by the owner. **Blocks:** T-004.

### Q-010: Should workers load Messenger tools without Claude's ToolSearch?
Claude Code 2.1.319 defers MCP tools behind `ToolSearch` by default. A real model should find `messenger_task` through it, but this was not verified ([report](reports/2026-11-06-task-workers-linux.md)). `ENABLE_TOOL_SEARCH=false` for workers makes them visible at once but changes the owner's normal session behaviour (D-023). **Chosen for now:** Claude's default; check in T-027.

## Answered

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
