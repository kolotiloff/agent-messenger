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

### Q-008: Switch to one-shot runs that resume a saved conversation?
Proposal version 2 in [RETHINK.md](RETHINK.md), after the Codex [review](RETHINK_REVIEW.md) (accept the direction with changes). It would relax D-004 in part and replace D-005 and D-016. The owner decides three things:
1. **Ownership guarantee.** Accept "one managed launch per conversation at a time, best-effort cleanup" instead of "no overlap until a proven reboot"? A process that escaped its group could keep working. With worker tools this is true of the current model too; only an OS sandbox gives a strong guarantee.
2. **Worker permissions.** With which permission mode do unattended workers run: full tools in their workspace (as today by hand), or a sandbox with limited write and network access?
3. **Loop budget.** Only the owner starts a new task with a fresh automatic-step budget in the first version; the coordinator cannot reset it. Acceptable?

**Options:** accept; accept with changes; keep persistent processes. **Blocks:** T-009 and the stages after it. T-022 does not depend on it.

## Answered

None yet.
