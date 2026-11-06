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

### Q-008: Switch to "one task, one session" with the coordinator in the owner's app?
Proposal version 3 in [RETHINK.md](RETHINK.md): the coordinator is the owner's own Claude or Codex chat with Messenger connected; every task gets a new worker session that reports and closes; the coordinator waits with a blocking `wait` tool. Replaces versions 1–2, which Codex reviewed in [RETHINK_REVIEW.md](RETHINK_REVIEW.md). It would replace D-005 and D-016 for new work and relax D-004 in part. The owner decides:
1. **The model itself**: accept, accept with changes, or keep persistent processes.
2. **Plan approval**: approve each `start_task` in the app, or approve the whole plan once in the Messenger window.
3. **Worker permissions**: full tools in their workspace, or a sandbox limiting writes and network.
4. **Ownership statement**: accept that a process left behind by a finished task is cleaned up on a best-effort basis (a strong guarantee needs an OS sandbox).

**Blocks:** T-009 and the stages after it. T-022 does not depend on it.

## Answered

None yet.
