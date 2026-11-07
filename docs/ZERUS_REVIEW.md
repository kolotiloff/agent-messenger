# Zerus review: what to take

Date: 2026-11-07. Author: Claude Code, at the owner's request. Source: [ufna/zerus](https://github.com/ufna/zerus) at `7a4dd7cf` (desktop 0.49.0), read from code and docs; the app was not run.

The owner's filter: take only what **saves tokens**, **makes work simpler** or **makes it more reliable**. Everything else is listed below with the reason it was left out, so the discussion does not have to be repeated.

## What Zerus is

An "agent development environment": Rust core (~300k lines), Tauri desktop (~480k lines with tests and assets), Android and iOS apps, a self-hostable relay, speech recognition. Providers: Claude Code, Codex, Kimi, DeepSeek, OpenCode, local models. Machines are peers over SSH with a replicated project catalog. Public, MIT, created 2026-10-07.

The core difference from us: Zerus keeps each agent in an interactive terminal (tmux) and observes it through hooks and transcripts; messages and tasks are **pasted into the terminal**. Its own docs say a sent task is not checked for receipt ("Zerus does not check that the agent received it"). We run a worker per task headless and finish a task only on verified evidence (D-021). Our verification model stays; nothing below weakens it.

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

The fixed part is already small (~1.2k tokens); Zerus's "compiled guide with topics" (`hgs guide send|wait|launch`) would save almost nothing here. The variable parts are where tokens go, and they are also a reliability risk: one large `wait` answer can push the coordinator's context into compaction and lose earlier decisions.

## Taken (new tasks)

| Task | Idea in Zerus | What we do | Why |
|---|---|---|---|
| T-031 | Bounded pages everywhere: four previews per waiter, at most 50 records per page, explicit cursors | `wait` returns events until a size budget (about 24,000 chars) is reached, always at least one event, and says `more: true`; the cursor points at the last returned event, so nothing is lost or skipped | tokens, reliability |
| T-032 | `task list --stage`, `--limit`, `--if-revision` (no transfer when nothing changed) | `tasks` returns only open tasks by default (`state: "all"` for everything), a limit, no folder per row; `if_cursor`: when no event happened since that cursor, answer `{"unchanged": true}` | tokens |
| T-033 | — (their tasks are pasted once into a live conversation) | a resumed run's `read_task` returns the question and the answer, not the task text again; `read_task` with `include_task: true` returns it when the worker needs it (for example after its context was compacted) | tokens |
| T-034 | Error classes (overload, rate limit, session usage limit with reset time, sign-in, quota) shown in the session, with the reset time in the toolbar | a failed run records a reason class and, when the CLI reports it, the reset time; the window and the coordinator's `failed` event show it ("лимит, сброс в 18:00"). The coordinator instruction says: on a limit, tell the owner the reset time. **No automatic retry** (unknown is not success; starting again stays the owner's or coordinator's decision) | reliability, simpler for the owner |

Today a failed Claude run says only `Claude result <subtype>` and a Codex one `Codex turn.failed` (`internal/adapters/tasks.go`), so neither the owner nor the coordinator can tell a limit from a bug, and a coordinator may spend a turn guessing.

## Considered and not taken

| Idea | Why not |
|---|---|
| Agents in tmux, input pasted into the terminal | No proof of delivery; contradicts D-021 and "unknown is not success". |
| Automatic recovery (continue after overload or limit on a schedule) | Automatic input after an unclear result; our rules forbid it. T-034 shows the reason and the reset time instead. |
| Automatic approvals | Not needed: workers already run with `acceptEdits` / `workspace-write`, and it adds risk. |
| Compiled agent guide with topics | Our fixed instructions are ~1.2k tokens; savings would be negligible. |
| Session reference URIs (`zerus://session?...`) and `inspect` | Useful when equal agents message each other. Our coordinator already addresses tasks by ID. |
| Sender envelope `<zerus-message from=...>` | Applies to the persistent routing path (ROUTING.md), which T-028 may retire. Revisit only if it stays. |
| Proving the sender by process ancestry | Our worker tools are already bound to the run and its receipt. |
| Waits with tickets (`hgs wait begin/ack/finish`) | Our `wait` with a replayable cursor and verified dispositions covers this with less machinery. |
| Peer-to-peer over SSH instead of our broker | Possibly simpler for two computers the owner already reaches by SSH, but the broker is built and audited (D-033–D-035), and SSH needs Remote Login and keys set up on each Mac. Kept as an option to revisit if carrying tasks through the broker turns out heavy. |
| Phone apps, voice, more providers, account limit dashboards | Product scope, not tokens, simplicity or reliability. |

## Where we are stronger

- A task is finished only after an observed exit, an empty process group and a revoked receipt; Zerus does not confirm that a sent task was received.
- A worker answers through exactly one structured disposition (`report`, `ask`, `ask_owner`), not free text in a terminal.
- Much smaller and easier to audit.
