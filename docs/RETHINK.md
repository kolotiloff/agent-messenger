# Rethinking the product: persistent conversations, not persistent processes

Status: **proposal for review, not a decision.** Written 2026-11-06 by Claude Code at the owner's request, for review by Codex and a decision by the owner ([Q-008](QUESTIONS.md)). Until the owner decides, every rule in [AGENTS.md](../AGENTS.md) and [DECISIONS.md](DECISIONS.md) stays in force. Nothing in the code was changed for this document.

## 1. Summary

The owner already coordinates several agents by hand: a coordinator session hands out tasks, worker sessions do them and report back. Agent Messenger should automate exactly this, across Claude Code and Codex, and later across computers.

Most of the project's complexity comes from keeping each agent's **process** alive and guarding it, so that two processes never own one session. The owner's workflow does not need a live process. It needs a live **conversation**: the agent's context, which the CLI already stores on disk.

Proposal:

1. Run an agent **once per incoming message (or batch)**, resuming its saved conversation by exact ID, and let the process exit. The client already has this path.
2. A conversation is busy while its process runs and free after the process has exited and its process group is empty. No reservation survives a finished run, and no reboot is needed to free a session.
3. An interrupted run is never retried automatically. The window shows it with **Retry** and **Skip**.
4. Three kinds of messages: **task** (coordinator → worker), **report** (worker → coordinator, the routed reply) and **question** (any agent → the owner, shown in the window and never sent through the coordinator's context).
5. Agents get real working tools in their workspace. Today every launch is stripped down to `read_current` and `reply`, so an agent can answer but cannot do any work.

This keeps the queue, the database, routing, the window and the updates. It parks the persistent Claude stream, the persistent Codex app-server owner and the reboot-based recovery commands.

## 2. The owner's workflow

As described by the owner on 2026-11-06. Today it runs in Claude Code over Remote Control:

- One **coordinator** session and several **persistent worker** sessions. Each keeps a moderate context (about 150K tokens).
- Workers do every task through sub-agents, so their own context does not grow.
- The coordinator hands out tasks and makes sure workers do not edit the same files.
- When a worker finishes, it reports to the coordinator. The coordinator may give it more tasks.
- Conflicts between workers are settled through the coordinator.
- Questions for the owner go straight to the owner, to save the coordinator's context. Routing them through the coordinator is also acceptable if done well.

Requirements that follow:

| Need | Meaning for the product |
|---|---|
| Workers remember the project between tasks | Keep the **conversation**, resume it by exact ID |
| Workers do real work | Normal tools in the worker's workspace: files, shell, sub-agents |
| Coordinator ↔ worker messages | Queue per agent; reply goes back to the sender automatically |
| Questions to the owner | A `send` to the owner alias; shown in the window |
| No tokens spent while waiting | Nothing runs while the queue is empty |
| Mixed providers, later several computers | The reason to build this at all (see section 7) |

File conflicts stay the coordinator's job. The program does not need to understand files.

## 3. Where the complexity comes from today

The client already has a one-shot path. `Supervisor.runClaimed` (`internal/supervisor/supervisor.go`) launches a CLI with an exact resume (`ClaudeArgs`, `CodexArgs` in `internal/adapters/adapters.go`), waits for it and validates its events. But after **every** finished run the session stays reserved:

```go
// groupEmpty is recorded and does not unlock the session.
_, _ = s.store.FinishObserved(token, exitCode, groupEmpty, providerOK, detail)
```

The reason is the rule "an empty process group is not proof that all descendants exited" (D-004): a descendant could have left the group with `setsid`. A reservation in `active-runs.json` is released only after a proven reboot of the same host. So a one-shot session can process exactly one message. That is why the project moved to one long-lived process per session (D-005), and then had to build:

- a persistent Claude `stream-json` owner with a controller per session, at most 4 at once (D-019);
- a persistent Codex `app-server` owner, a setup table, RPC matching, turn guards and config re-checks (D-010, D-014, D-015, D-018);
- `recover-stream`, `recover-codex` and `recover-missing-root`, all of which require a proven reboot (D-016);
- uncertain states that block a session until that reboot.

The cost showed up in practice. After the Mac froze and rebooted, preview.19 would not open, because every reservation pointed to a deleted directory in `/private/tmp` ([report](reports/2026-11-06-reboot-temp-root-mac.md)). Meanwhile, about 11,900 lines of non-test Go exist, and a real Claude ↔ Codex exchange has not run yet.

The second gap is tools. Every launch path, one-shot and persistent, strips the agent down:

- Claude: `--tools ""`, `--allowedTools` limited to `read_current`, `reply` (and `send` for the stream);
- Codex: `--sandbox read-only`, shell, multi-agent and other features disabled.

That is right for a safety test bench. But it means no current path can run the owner's workflow, where workers edit files and use sub-agents.

## 4. Proposal

### 4.1 An agent is a saved conversation

An agent binding stays as it is today: alias, provider, exact conversation ID, profile, workspace, CLI path. It also gets:

- a **role**: `coordinator` or `worker` (one coordinator per client to start with);
- a **tool profile**: `messenger-only` (today's stripped profile, for tests) or `worker` (normal tools in the workspace, see 4.6).

### 4.2 A turn is one process run

When an agent has queued messages and is free:

1. Claim all its queued messages (or up to a limit) as one attempt, so that several reports arriving together cost one run.
2. Launch the CLI with an exact resume: `claude -p --resume <id>` or `codex exec ... resume <id>`, with a per-run MCP config bound to the attempt's receipt token. This is today's `AdapterLauncher`.
3. The agent reads the messages with `read_current`, does the work, and must call `reply` once. It may call `send`.
4. The process exits. The client validates the events and the stored reply, as today.

Nothing runs while the queue is empty, so waiting costs no tokens and needs no keep-alive.

### 4.3 When a conversation is free again

A conversation is **busy** from the claim until the run is finished. The run is **finished** when:

- the CLI process has exited and its exit was observed, and
- its process group is empty (if not, the client signals the group, waits briefly and checks again), and
- the attempt's receipt token is revoked, so a leftover process can no longer read the message, reply or send.

Then the conversation is free and the next message may run. `active-runs.json` stays the record of **running** processes. An entry is removed only on the evidence above, never on a timer alone.

What we accept: a descendant that escaped the process group with `setsid` may survive. Its danger was writing a second reply or starting more work under the session's name. Revoking the receipt removes the first; the second is the same risk as any background job the user starts by hand. The window can list such leftovers if the platform code can find them. This is a deliberate relaxation of D-004 and must be decided by the owner.

### 4.4 When something goes wrong

The rule "unknown is not success; never retry automatically" stays.

| Situation | What happens |
|---|---|
| The CLI exits with an error, or without a valid reply | The message is marked **failed**. The window offers **Retry** and **Skip**. |
| The client stopped or crashed during a run | On the next start: if the recorded process is gone and its group is empty, the message becomes **interrupted** (Retry/Skip) and the conversation is free. If the process is still alive, it is shown as **still running** with a **Stop** button that signals its group. |
| A run takes too long | Shown as **running for N minutes** with **Stop**. No automatic kill, no automatic retry. |
| The data directory is missing | The binding is shown as broken. No reboot is needed to reconfigure it. |

The owner, or the coordinator through a future command, decides what happens next. Nothing is replayed silently.

### 4.5 Messages and tools

| Tool | Who | Effect |
|---|---|---|
| `read_current()` | any agent | Returns the messages of this attempt. |
| `reply(text)` | any agent, exactly once per run | Stored and routed to the sender of the current message, as today ([ROUTING.md](ROUTING.md)). For a worker this is the **report**. |
| `send(to, text)` | any agent | `to` is a registered alias or `owner`. A message to `owner` is only shown in the window (later: Telegram, push). Counts toward the chain limit. |

Because every run gets a fresh MCP config, Codex gets `send` too. Today Codex only has `read_current` and `reply`, because app-server threads fix their tools at `thread/start`.

The chain limit (3 automatic steps, [ROUTING.md](ROUTING.md)) stays. A normal cycle is task → report → next task, so the coordinator may need a higher limit or a reset when the owner or the coordinator starts a new task. This is a detail to settle during implementation.

### 4.6 Tool profiles

- `messenger-only`: today's launch arguments. Used by automated tests and acceptance runs.
- `worker`: the provider's normal tools inside the binding's workspace, plus the Messenger MCP tools.
  - Claude: no `--tools ""`; a permission mode chosen by the owner, and the owner's settings or a managed settings file.
  - Codex: `--sandbox workspace-write` instead of `read-only`, and multi-agent/shell left enabled.
  - Sub-agents are the CLI's own feature; the program does not manage them.

Which permission mode a worker runs with is the owner's decision ([Q-008](QUESTIONS.md)). Unattended workers with full permissions in a workspace are what the owner does today by hand.

### 4.7 Optional, later

- Show each agent's context size from the CLI's usage events, so the coordinator can decide when a worker needs a fresh conversation.
- A file-claim table, if the coordinator turns out to need help avoiding conflicts.
- The coordinator may be the owner's interactive session instead of a managed one. That needs a way to wake an interactive session and is out of scope here.

## 5. What happens to the code

Estimates from reading the code, not measured.

**Keep as is:** `internal/core` store, queue, outbox, routing, timeline; `internal/localui`; `desktop/macos` and Sparkle updates; `internal/diagnostics`; `internal/platform` process-group code; the one-shot adapters and their event validators; `internal/mcp` (extended with `send`).

**Change:**

- `Supervisor.runClaimed` and `runs.go`: free the conversation on the evidence in 4.3; revoke the receipt; run several agents in parallel with at most one run per conversation.
- Recovery at start-up: replace "proven reboot" with the table in 4.4.
- Adapters: tool profiles (4.6); batch several messages into one attempt.
- Window: statuses **running / failed / interrupted** with **Stop / Retry / Skip**; questions to the owner.

**Park, then remove once the new path is accepted on the Mac:**

- persistent Claude stream: `internal/supervisor/stream*.go`, `internal/core/stream*.go`, `internal/adapters/stream.go`, `recover-stream`;
- persistent Codex owner: `codex_rpc.go`, `codex_setup.go`, `codex_turn.go`, `codex_appserver.go` (the `config/read` isolation check may be worth keeping for `codex exec`), `recover-codex`;
- `recover-missing-root` and the other checks that require a proven reboot.

Session setup changes too. Codex issues its own thread ID, so creating a Codex worker becomes "run `codex exec` once with an introduction message and record the `thread_id` it reports", which the early experiments already did.

## 6. Risks and what we give up

| Risk | Assessment |
|---|---|
| An escaped descendant (`setsid`) keeps running after a run | Accepted; receipt revoked; same as a background job started by hand (4.3). |
| Start-up time per run (seconds) and reloading context | Measure on the Mac. Provider prompt caching should make repeated context cheap; batching reduces the number of runs. |
| The owner opens the same conversation by hand while a run is active | Same as today: unsupported. The window shows the conversation as busy. |
| `codex exec resume` and `claude -p --resume` change behaviour between CLI versions | Same as today; `check-cli` and the event validators stay. |
| Workers with real tools can damage a workspace | That is the owner's workflow today. Profiles and the workspace boundary are explicit, and the owner chooses the permission mode. |

## 7. Is the product still worth building?

Claude Code itself can now pass messages between the owner's Claude sessions, including over Remote Control (the session that wrote this document has such tools). For a Claude-only setup, that covers much of this workflow already. Agent Messenger is worth building for what it adds:

- mixing providers: Claude and Codex now, Cursor later;
- several computers: Mac and Windows;
- one window with the whole conversation, statuses and questions to the owner;
- no tokens spent on waiting or delivery.

The simpler model serves all four and makes the network stage easier: a remote message becomes one more queued item for a one-shot run.

## 8. Proposed order

1. **Decide** ([Q-008](QUESTIONS.md)). If accepted: a new decision that supersedes D-004 (in part), D-005 and D-016, and an update of the rules in `AGENTS.md`.
2. **Free a conversation after a finished run** (4.3) and revoke the receipt. Tests with fake CLIs: two messages in a row to one conversation, a crash mid-run, a leftover process, no retry.
3. **`send` and the owner alias** for Claude and Codex one-shot runs.
4. **Coordinator ↔ worker** with fake CLIs: task → report → next task, a question to the owner, the chain limit.
5. **Window**: running / failed / interrupted, Stop / Retry / Skip, questions.
6. **Mac acceptance with real models**, in `~/AgentMessengerTest/`, with new test sessions: a Claude coordinator and a Codex worker, two full task cycles, the `messenger-only` profile first, then `worker`.
7. **Park and remove** the persistent code.
8. Then the network stage (T-015).

T-009 (resume a persistent Codex thread) should wait for this decision, since the proposal would make it unnecessary.

## 9. Questions for the reviewer

1. Is the relaxation in 4.3 (exit observed + empty group + revoked receipt) enough, or is there a failure mode on macOS or Windows that it misses?
2. Can a leftover process do harm after its receipt is revoked, other than continuing to edit files?
3. Does `codex exec resume` keep everything the app-server path guaranteed: exact thread ID, isolation from the user config, per-run MCP tools?
4. Is anything in "park, then remove" still needed for the network stage?
5. Is batching several messages into one attempt compatible with the current receipt and reply checks?
