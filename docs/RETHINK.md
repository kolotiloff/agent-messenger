# Rethinking the product: persistent conversations, not persistent processes

Status: **proposal, version 2, not a decision.** Version 1 (`59d58eb`) was written by Claude Code on 2026-11-06 at the owner's request. Codex reviewed it in [RETHINK_REVIEW.md](RETHINK_REVIEW.md) (`23edb72`): accept the direction with changes. This version takes in that review; section 9 lists what changed. The owner decides ([Q-008](QUESTIONS.md)). Until then every rule in [AGENTS.md](../AGENTS.md) and [DECISIONS.md](DECISIONS.md) stays in force. No product code was changed for this document.

## 1. Summary

The owner already coordinates several agents by hand: a coordinator session hands out tasks, and worker sessions do them and report back. Agent Messenger should automate this across Claude Code and Codex, and later across computers.

The project keeps each agent's **process** alive and guards it, so that two processes never own one session. The owner's workflow does not need a live process. It needs a live **conversation**, and the CLI already stores that on disk.

Proposal:

1. Run an agent **once per incoming message**, resuming its saved conversation by exact ID, and let the process exit. The client already has this path.
2. Guarantee **one managed launch per conversation at a time, with best-effort cleanup** of the process group. This is weaker than today's rule and must be accepted by the owner explicitly (4.3).
3. A worker's **report reaches the coordinator only after its run is verified** (4.5).
4. An unclear result is never retried automatically. The window shows it with explicit actions (4.4).
5. Three kinds of messages: **task** (coordinator → worker), **report** (worker → coordinator) and **question** (any agent → the owner, outside the coordinator's context).
6. Workers get real tools, with validation that expects them (4.6). Today every launch allows only `read_current` and `reply`, so an agent can answer but cannot do any work.

This reuses and adapts the queue, the database, routing, the window and the updates. The persistent Claude stream and the persistent Codex owner stay until existing sessions are migrated (section 5). It is a direction for several steps, not a small unlock patch.

## 2. The owner's workflow

As described by the owner on 2026-11-06. Today it runs in Claude Code over Remote Control:

- One **coordinator** session and several **persistent worker** sessions, each with a moderate context (about 150K tokens).
- Workers do every task through sub-agents, so their own context grows slowly. Delegation does not keep it fixed: instructions and returned reports still accumulate.
- The coordinator hands out tasks and makes sure workers do not edit the same files.
- When a worker finishes, it reports to the coordinator, which may give it more tasks.
- Conflicts between workers are settled through the coordinator.
- Questions for the owner go straight to the owner, to save the coordinator's context.

| Need | Meaning for the product |
|---|---|
| Workers remember the project between tasks | Keep the **conversation**; resume it by exact ID |
| Workers do real work | Normal tools for workers, with explicit permissions |
| Coordinator ↔ worker messages | A queue per agent; the report goes to the sender after verification |
| Questions to the owner | A reserved owner recipient, shown in the window |
| No tokens spent while waiting | Already true today; must stay true |
| Mixed providers, later several computers | The reason to build this at all (section 7) |

File conflicts stay the coordinator's job.

## 3. Where the complexity comes from today

The client already has a one-shot path. `Supervisor.runClaimed` (`internal/supervisor/supervisor.go`) launches a CLI with exact resume (`ClaudeArgs`, `CodexArgs` in `internal/adapters/adapters.go`), waits for it and validates its events. But after **every** finished run the session stays reserved:

```go
// groupEmpty is recorded and does not unlock the session.
_, _ = s.store.FinishObserved(token, exitCode, groupEmpty, providerOK, detail)
```

The reason is the rule "an empty process group is not proof that all descendants exited" (D-004): a descendant could leave the group with `setsid`. A reservation is released only after a proven reboot. So a one-shot session can process exactly one message, which is why the project moved to one long-lived process per session (D-005). That brought the persistent Claude stream, the persistent Codex app-server owner, and recovery commands that need a proven reboot (D-016).

The reboot rule made the `/private/tmp` incident ([report](reports/2026-11-06-reboot-temp-root-mac.md)) hard to recover from. The incident itself was a storage problem, not a consequence of persistent processes; durable data directories are needed in any model.

The second gap is tools. Every launch path strips the agent down (Claude: `--tools ""`, only Messenger tools allowed; Codex: `--sandbox read-only`, shell and multi-agent disabled). That suits a safety test bench, but no current path can run a worker that edits files.

## 4. Proposal

### 4.1 An agent is a saved conversation

An agent binding stays as it is today: alias, provider, exact conversation ID, profile, workspace, CLI path. It also gets:

- a **role**: `coordinator` or `worker` (one coordinator per client to start with);
- a **tool profile**: `messenger-only` (today's profile, for tests) or `worker` (4.6).

### 4.2 A turn is one process run

When an agent has a queued message and is free:

1. Claim **one** message as one attempt, as today. Batching several messages needs a new store and protocol design (per-message results, routing lineage, partial failure) and is left for later.
2. Launch the CLI with exact resume (`claude -p --resume <id>`, `codex exec ... resume <id>`) and a per-run MCP config bound to the attempt's receipt.
3. The agent reads the message, does the work, may call `send`, and must call `reply` once.
4. The process exits. The client checks identity, provider completion, exit status and the stored reply, then publishes the report (4.5).

### 4.3 Ownership: what we guarantee and what we give up

**Today's guarantee:** no two processes ever work on one conversation, because a reservation is held until a reboot proves every descendant gone.

**Proposed guarantee:** one **managed launch** per conversation at a time, with best-effort cleanup of its process group. A conversation is free again only after a **cleanly observed** run:

- the launch intent, PID, process group and start identity were recorded;
- the exit was observed;
- the process group is empty, or became empty after the client signalled it (identity is re-checked before every signal, against PID reuse);
- the receipt is revoked, and the result and revocation are durable.

Only then is the reservation retired. The order of these writes across SQLite and `active-runs.json` must be crash-safe: start-up finishes an interrupted transition idempotently, and a crash between writes never relaunches the same message or lets another data directory start a competing launch.

**What we give up**, stated plainly: a descendant that escaped the process group can keep calling the model, run another CLI on the same conversation, change provider history, make network requests or start more processes. Revoking the receipt only blocks its Messenger calls. With unrestricted worker permissions, the local MCP server and the database are not isolated from it either.

Note: once workers have real tools, the same is true of today's persistent model. A worker with a shell can start its own `claude --resume` or `codex exec resume`, and a reservation cannot stop it. A strong guarantee needs an OS-level sandbox for workers in either model. This is a product choice for the owner (Q-008), not something the process model alone can fix.

**What stays strict** (no relaxation):

- A pending launch with no recorded PID, an incomplete observation, a failed inspection, a corrupt registry or a missing database is **not** evidence of a stopped run. The conversation stays blocked and the window explains why.
- A missing data directory is not evidence of termination. Recovery or rebinding is an explicit, scoped action; a possibly live owner is never silently forgotten.
- If the client had to kill remaining processes, the run is recorded as **cleaned up by force**, not as a clean completion, and its report is not published as success.

Windows has no process ownership implementation yet (group setup, observation and locking return unsupported). It needs its own design, for example Job Objects, and native acceptance.

### 4.4 When something goes wrong

The rule "unknown is not success; never retry automatically" stays.

| Situation | Status | What happens |
|---|---|---|
| Exit error, invalid events, or no valid reply, after a cleanly observed run | **failed** | Owner chooses **Retry** or **Skip**. The window shows what the run already did: stored provisional reply, `send` messages, questions. |
| Client stopped or crashed during a run; process gone and fully observed | **interrupted** | Same choices. Outcome is unknown, so it is shown separately from **failed**. |
| Process still alive, or observation incomplete | **still running / blocked** | **Stop** signals the verified group. Nothing else runs on this conversation. |
| A run takes long | **running N min** | **Stop** is offered. No automatic kill, no automatic retry. |

After **failed** or **interrupted**, further messages for that agent stay paused until the owner acts, so dependent work does not start on top of an unclear result.

Retry needs a new contract: today's failed-attempt recovery refuses an attempt that already has a stored reply.

### 4.5 Messages, reports and questions

| Tool | Effect |
|---|---|
| `read_current()` | Returns this attempt's message. |
| `reply(text)` | Exactly once per run. Stored **provisionally**. Published to the sender (routed) only after the run is verified, in the same transaction that marks the attempt complete. A reply to the owner is published the same way. |
| `send(to, text)` | `to` is a registered alias or the reserved **owner** recipient (not an alias, so it cannot collide). Delivered immediately, recorded with the attempt, and **not** rolled back if the run fails. The window shows this history. |

Today `replyTx` routes the reply in the transaction that stores it, before the run is validated. This affects the existing persistent paths too, so fixing it is useful whatever the owner decides on Q-008 (T-022).

**Questions to the owner** get a correlation ID. Waiting rule for the first version: after asking, the agent finishes its run with a `reply` saying it is waiting; the owner's answer arrives as a new message to that agent, linked to the question.

**Loop limits.** The current chain limit (3 automatic steps, at most 3 sends per turn) stays. The coordinator cannot reset it. In the first version, only an owner action starts a new task with a fresh budget. Later: task IDs and an overall automatic-work budget per task.

### 4.6 Tool profiles and validation

- `messenger-only`: today's launch arguments and validators. Used in tests.
- `worker`: the provider's normal tools plus the Messenger tools.
  - Claude: no `--tools ""`; a permission mode chosen by the owner; deliberate choice of which settings and hooks are inherited.
  - Codex: `--sandbox workspace-write` instead of `read-only`; shell and multi-agent enabled.
  - Sub-agents are the CLI's own feature.

Validation must change with the profile, or valid work will be rejected after it has happened (and a retry could repeat it). Today `ValidateClaude` requires exactly `read_current, reply`, and `ValidateCodex` rejects other completed actions. Split validation into three parts:

1. provider identity and completion (unchanged strictness);
2. Messenger protocol: one `read_current`, bounded `send`, exactly one `reply`, bound to the receipt;
3. the tool profile: which other actions are expected, and how a denied or approval-requiring tool is reported.

The run prompt changes too: today it forbids normal tools.

A working directory is not a sandbox. The owner chooses the permission mode for unattended workers (Q-008). That is what the owner does today by hand.

### 4.7 Optional, later

- Batching several messages into one run (4.2).
- Context size per agent from usage events.
- A file-claim table, if the coordinator needs help with conflicts.
- The owner's interactive session as coordinator.

## 5. Code: reuse, adapt, migrate

Estimates from reading the code.

**Reuse and adapt:** `internal/core` (store, queue, outbox, routing, timeline), `internal/localui`, `desktop/macos` and updates, `internal/diagnostics`, `internal/platform`, the one-shot adapters and validators, `internal/mcp` (add `send`, owner recipient).

**Change:**

- Publication of the final reply after verification (4.5).
- Completion, reservation retirement and crash recovery in `Supervisor.runClaimed` and `runs.go` (4.3).
- Statuses and actions in the window (4.4).
- Tool profiles and profile-aware validators (4.6).
- Codex `exec`: re-establish what the app-server path checks (section 6).

**Migrate before removing.** Existing Codex sessions are `codex_owned` with no CLI path, and the one-shot path deliberately refuses them. Existing Claude stream sessions hold their own reservations. Before any persistent code is removed:

- define migration of bindings, pending messages, replies, attempts, reservations and setup intent, with a schema version and tested upgrade;
- keep recovery and read support for old state until it is migrated; never treat an old uncertain reservation as a clean completion;
- accept `codex exec resume` on a thread that was created by app-server.

New Codex sessions still need durable setup intent: the first `codex exec` is a model call, and a failure before or after the thread ID is saved must not silently create a replacement conversation.

## 6. Risks

| Risk | Assessment |
|---|---|
| Escaped descendants (4.3) | Accepted only if the owner agrees; strict blocking on unclear evidence stays; OS sandbox needed for a strong guarantee in either model. |
| Report published, then the run fails | Removed by provisional replies (4.5). `send` messages are delivered immediately and shown as such. |
| `codex exec resume` lacks the app-server checks | No automatic parity. Re-verify for the supported CLI version: exact ID, profile, ignored user config, foreign tools, read-only vs worker sandbox, required MCP failure, stale token, changed per-run tools on the same thread, and a migrated app-server thread. |
| Start-up time and context reload per run | Measure input, cached and output tokens and latency against the persistent path on equal multi-turn tasks before claiming any savings. |
| Workers with real tools damage a workspace | Explicit profile and permission mode, chosen by the owner. |
| Owner opens the same conversation by hand during a run | Unsupported, as today. |

## 7. Is the product still worth building?

Claude Code itself can now pass messages between the owner's Claude sessions, including over Remote Control. For a Claude-only setup that covers much of this workflow. Agent Messenger adds:

- mixing providers: Claude and Codex now, Cursor later;
- several computers: Mac and Windows;
- one window with the whole conversation, statuses and questions to the owner;
- delivery and waiting done by code, without model calls.

For the network stage, what matters are the invariants, not the persistent transport: single ownership per conversation on its host, setup intent, exact identity, receipt fencing, crash reconciliation, deduplication and correlated delivery. A broker must separate "message received" from "agent finished", must not rerun uncertain work, and must not treat a lost connection as proof that an agent stopped.

## 8. Order of work, if Q-008 is accepted

Follows the review's first slice.

0. **Independent of Q-008:** publish the final reply only after the run is verified (T-022). Fixes the existing paths too.
1. Record the decision: the ownership guarantee (4.3), the worker permission mode, the loop budget rule. Update `AGENTS.md`.
2. One message per attempt, `messenger-only` profile: crash-safe completion and reservation retirement; persistent paths stay for old sessions.
3. Fake-CLI tests: two messages in a row on one conversation; competing aliases, path spellings and data directories; reply then failure; crash before the PID is saved and around each completion write; stale receipt; partial registry or database failure; detached child and group cleanup; no automatic retry; no early report.
4. Verify Codex `exec` against the list in section 6. Then `send`, owner questions and bounded task cycles; test duplicates and late deliveries.
5. Worker profiles and profile-aware validators. Tests: a real edit or command and its report; partial work then failure; denied permission; child lifetime. Then a Mac acceptance with real models in `~/AgentMessengerTest/`: a Claude coordinator and a Codex worker, two full task cycles.
6. Migration of existing sessions; only then remove the persistent code.
7. Separately: Windows process ownership; then the network stage (T-015).

T-009 waits for Q-008.

## 9. Changes from version 1

| Review finding | Change |
|---|---|
| R1: receipt revocation does not stop leftover processes | 4.3 names the weaker guarantee, keeps strict blocking on unclear evidence, adds crash-safe ordering, forced-cleanup status, Windows note, and the note that worker tools weaken today's model too. |
| R2: report routed before verification | Provisional `reply`, published after verification (4.5); `send` is immediate and not rolled back; new T-022, useful for the existing paths too. |
| R3: batching does not fit the schema | One message per attempt; batching moved to 4.7. |
| R4: validators reject worker actions | Three-part validation and a new prompt (4.6). |
| R5: migration needed | Section 5 "migrate before removing"; setup intent kept for new Codex sessions. |
| R6: coordinator resets defeat loop limits | Coordinator cannot reset; only the owner starts a fresh budget (4.5). Owner is a reserved recipient; questions have a correlation ID. |
| Cost and scope | Waiting cost is no longer claimed as an advantage over persistent processes; savings must be measured; "keep" became "reuse and adapt"; the `/tmp` incident is described as a storage problem. |

Answers to the questions of version 1 are in [RETHINK_REVIEW.md](RETHINK_REVIEW.md#answers-to-section-9).
