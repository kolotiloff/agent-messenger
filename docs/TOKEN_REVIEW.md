# Token and reliability review

Date: 2026-11-07. Author: Claude Code, at the owner's request.

The owner's filter: change only what **saves tokens**, **makes work simpler** or **makes it more reliable**. Ideas that were considered and left out are listed with the reason, so the discussion does not have to be repeated. Nothing here weakens D-004 or D-021.

## Where our tokens go

Measured on `main` at `5881844`:

| Part | Size | Paid |
|---|---|---|
| Coordinator instructions | 1,450 chars | once per coordinator session |
| Coordinator tool schemas | 2,702 chars | once per coordinator session |
| Worker first prompt | 529 chars | once per run |
| `wait` result | up to 20 events × up to 20,000 chars each (~400k chars) | every time events arrive |
| `tasks` result | up to 200 tasks, finished ones included, each with folder and timestamps | every call |
| `read_task` on a resumed run | the full task text again (up to 20,000 chars), although the resumed conversation already has it | every question round |

The fixed part is already small (~1.2k tokens); shortening it further would save almost nothing. The variable parts are where tokens go, and they are also a reliability risk: one large `wait` answer can push the coordinator's context into compaction and lose earlier decisions.

## Taken (new tasks)

| Task | What we do | Why |
|---|---|---|
| T-031 | `wait` returns events until a size budget (about 24,000 chars) is reached, always at least one event, and says `more: true`; the cursor points at the last returned event, so nothing is lost or skipped | tokens, reliability |
| T-032 | `tasks` returns only open tasks by default (`state: "all"` for everything), a limit, no folder per row, and answers `unchanged` when nothing changed since the caller's last list | tokens |
| T-033 | a resumed run's `read_task` returns the question and the answer, not the task text again; `read_task` with `include_task: true` returns it when the worker needs it (for example after its context was compacted) | tokens |
| T-034 | a failed run records a reason class (usage limit, rate limit, overload, sign-in, other) and, when the CLI reports it, the reset time; the window and the coordinator's `failed` event show it ("лимит, сброс в 18:00"). The coordinator instruction says: on a limit, tell the owner the reset time. **No automatic retry** (unknown is not success; starting again stays the owner's or coordinator's decision) | reliability, simpler for the owner |

Before T-034 a failed Claude run said only `Claude result <subtype>` and a Codex one `Codex turn.failed`, so neither the owner nor the coordinator could tell a limit from a bug, and a coordinator could spend a turn guessing.

## Considered and not taken

| Idea | Why not |
|---|---|
| Agents in an interactive terminal, input pasted into it | No proof of delivery; contradicts D-021 and "unknown is not success". |
| Automatic recovery (continue after overload or limit on a schedule) | Automatic input after an unclear result; our rules forbid it. T-034 shows the reason and the reset time instead. |
| Automatic approvals | Not needed: workers already run with `acceptEdits` / `workspace-write`, and it adds risk. |
| A compiled agent guide split into topics | Our fixed instructions are ~1.2k tokens; savings would be negligible. |
| Session reference links and an `inspect` tool | Useful when equal agents message each other. Our coordinator already addresses tasks by ID. |
| A sender envelope around routed messages | Applies to the persistent routing path (ROUTING.md), which T-028 may retire. Revisit only if it stays. |
| Proving the sender by process ancestry | Our worker tools are already bound to the run and its receipt. |
| Waits with tickets (begin / ack / finish) | Our `wait` with a replayable cursor and verified dispositions covers this with less machinery. |
| Peer-to-peer over SSH instead of our broker | Possibly simpler for two computers the owner already reaches by SSH, but the broker is built and audited (D-033–D-035), and SSH needs Remote Login and keys set up on each Mac. Kept as an option to revisit if carrying tasks through the broker turns out heavy. |
| Phone apps, voice, more providers, account limit dashboards | Product scope, not tokens, simplicity or reliability. |
