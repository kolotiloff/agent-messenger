# Review of RETHINK.md version 3 — T-021

Reviewed by Codex on 2026-11-06 against **7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4**, branch `claude/vigilant-darwin-34d88b`.

**Verdict: a better product direction, with prerequisites before implementation.** Keeping the coordinator in the owner's familiar app and giving a bounded task its own worker conversation is a sensible default. It is more useful than making the owner manage permanent Messenger-owned sessions. It is not yet a proven replacement for the owner's working Remote Control setup, and it does not eliminate ownership, recovery or resume requirements.

I recommend a small task-oriented pilot, with application-enforced plan approval and durable, replayable events. Prove the coordinator integration in the actual desktop clients before retiring the persistent adapters. Do not promise lower token usage until measured. Q-008 remains undecided; this review authorizes no implementation or relaxation of existing rules.

The comparison from `59d58eb` to `7d02b62` changes documentation only. The code findings from the first review therefore still describe the implementation; version 3 changes the intended contracts, not the code. Evidence links below are pinned to the requested commit. Official documentation was checked on the review date; documented capabilities are distinguished from acceptance on installed clients.

## Issues to resolve first

| Priority | Finding | Required amendment |
|---|---|---|
| P1 | `ask` → `answer` reuses the very same conversation. An escaped descendant can overlap that resumed run. | Name and accept the weaker ownership guarantee for **every within-task resume**, or keep one owned process through the question. A new ID for the next task does not solve this case. |
| P1 | A lost `start_task` response can create a second task/session when the caller repeats it. `answer(task_id, text)` can answer the wrong later question. | Stable operation/plan-item IDs, immutable arguments, question IDs, state versions and transactional deduplication. A fresh session is not an execution deduplication mechanism. |
| P1 | Neither a skill nor a client permission prompt proves that a particular bounded plan was approved. | Store and enforce approval in Messenger, including launch count, target, workspace, tool profile and allowed continuations. Make retries consume explicit authorization. |
| P1 | “Everything new” has no durable delivery/acknowledgement contract. An interrupted `wait` can lose an event or cause its action to repeat. | Replayable events, subscriber identity, cursor/ack semantics and idempotent actions. Cancelling a subscription must not cancel tasks. |
| P2 | The proposal assumes a uniformly blocking ten-minute call, but client timeouts and backgrounding differ. | A tested client capability matrix; bounded waits below actual limits; no duplicate subscription when a call backgrounds. |
| P2 | A new conversation does not isolate the workspace or stop old commands; `cancel` and `blocked` are underspecified. | Keep process evidence, explicit cancellation states and a workspace concurrency policy. Distinguish protocol success from task correctness. |

## 1. Blocking `wait`: feasibility, limits, interruption and restart

**Feasible as an MCP subscription implemented by ordinary code; not a universal promise that an open chat will remain in one blocked call or automatically resume forever.** A skill guides behavior but cannot override client cancellation, permission decisions, compaction, turn limits, application shutdown or a model choosing to stop.

### What the documentation establishes

| Client/surface | Documented support and limits | What remains to verify |
|---|---|---|
| Claude Code CLI, interactive | Per-server `timeout` is a hard tool-call limit. Long main-conversation calls normally background after two minutes; a result then arrives by notification. Ordinary background calls do not survive exiting the session. [Claude MCP](https://code.claude.com/docs/en/mcp#time-limits-on-mcp-tool-calls). | The installed version's actual transport, timeout, background notification and interruption behavior with Messenger. |
| Claude Code, local Desktop **Code tab** | Local Code sessions support configured MCP servers and skills. Desktop and standalone CLI configuration precedence differs, including Desktop's additional config source. [Desktop configuration](https://code.claude.com/docs/en/desktop#shared-configuration). | Test the embedded Code runtime separately from the installed CLI. Do not extrapolate these results to ordinary Claude chat or a cloud session. |
| Codex CLI | MCP supports stdio/Streamable HTTP. `tool_timeout_sec` defaults to **60 seconds** and is configurable. [Codex MCP](https://learn.chatgpt.com/docs/extend/mcp). | Long waits, interrupt handling, and whether an interrupted call's eventual result is ignored. |
| Codex desktop host | Official docs describe shared MCP configuration across local desktop/CLI/IDE clients on the same Codex host. [Codex MCP](https://learn.chatgpt.com/docs/extend/mcp). | The specific installed app/version must pass the same tests; shared configuration does not prove identical scheduling or restart behavior. |

Claude also documents idle limits: five minutes for network transports and thirty minutes for stdio; a per-server timeout can raise that idle window. `CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS=0` disables automatic backgrounding. These are version-sensitive controls, not settings to change globally without considering other tools. [Claude environment variables](https://code.claude.com/docs/en/env-vars).

**Duration recommendation:** start compatibility probes at **30 seconds**, leaving headroom under Codex's documented default. This is a conservative test value, not a guarantee for unknown hosts and not the desired all-day production polling interval. Once verified, use an explicit longer profile, for example a 300-second application wait with a client hard limit of at least 360 seconds and compatible idle/transport limits. Ten minutes is reasonable only after configuring and testing the client accordingly. Keep the server wait below every effective deadline. For Claude, either handle backgrounded calls or deliberately configure foreground waiting; never interpret “background task created” as “no event, call wait again.”

The document's statement that the owner must stop a wait to type is too absolute. Backgrounding can free the conversation. Conversely, closing a view is not necessarily quitting its agent process. Record which surface and action was tested. Claude's MCP task extension is another possible transport, but the current docs exclude Desktop from that extension; it is not a cross-client shortcut. [Claude MCP tasks and backgrounding](https://code.claude.com/docs/en/mcp#automatic-backgrounding-of-long-tool-calls).

### Required server contract

- The long-lived Messenger daemon owns workers and storage. The MCP process is an adapter to it. Exiting the coordinator or killing its stdio server must not delete tasks or kill workers as an accidental consequence.
- Use `wait(subscription_id, after_cursor, timeout, limit)` or an equivalent replayable contract. Event IDs are stable; include reports, questions, owner messages **and failure/interruption/block/cancellation events**. The proposed list omits the failures the coordinator needs to react to.
- Returning an event does not prove the coordinator received or acted on it. Retain it for replay; acknowledge deliberately, and make subsequent actions idempotent. Bound returned text and paginate. A status-only `tasks()` response is not a substitute for missed question/report bodies.
- Cancelling `wait` releases only that subscription. Cancellation may race a completed result, which the client can discard. [MCP cancellation](https://modelcontextprotocol.io/specification/2025-06-18/basic/utilities/cancellation). Do not advance a destructive queue pointer on a socket write.
- Keep one effective subscription per coordinator identity, or explicitly deduplicate concurrent readers. On restart/reconnect, resume from durable state; do not claim the old MCP call itself resumes.
- Keep permission to launch and answer separate from permission to observe. Bind coordinator identity to a project/plan, not an arbitrary chat string. A later coordinator handoff needs a new authority generation so the old chat cannot keep issuing commands.

The current [MCP loop](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/mcp/server.go#L32) processes calls synchronously and ignores notifications with no request ID. Putting a blocking wait inside it would prevent that connection from handling cancellation or another call. The [UI proxy](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/localui/server.go#L173) also defaults to a three-second request deadline. A coordinator endpoint needs its own asynchronous/cancellable handling; do not simply expose the existing worker server or reuse the short UI deadline.

**Acceptance matrix, all four clients:** an event before waiting; an event during waiting; empty timeout; user interrupt just before delivery; two concurrent waits; close/reopen client; daemon restart; disconnect after the task commits but before the response arrives. Verify event replay, one effective launch, no unexpected model wake-ups, and owner-message behavior. No such client acceptance was performed in this review.

## 2. Is `ask` followed by exact resume safe?

**Conditionally, with the explicitly weaker process-group policy; not under the current strong reservation rule.** This is the same restart problem as version 1, reduced to question rounds within a task. “No session to guard between tasks” is true only between distinct tasks, not between `ask` and `answer`.

Example: a worker leaves a detached child capable of accessing its profile, calls `ask`, and exits. The group becomes empty. An answer resumes the same ID while that child can still use it. New task IDs do not remove this counterexample. Revoking Messenger access does not revoke provider credentials, reverse external side effects, or isolate the shared files.

An OS filesystem sandbox alone does not prove process-tree termination either. Stronger isolation requires an explicit process-supervision boundary and appropriate access restrictions; changing Codex to `workspace-write` is not that proof.

For the proposed policy, require all of the following:

1. Exactly one terminal disposition per run: report, coordinator question, or owner question. Calling the tool stores a provisional disposition; **it does not itself end the CLI process**. Publish only after valid provider completion and the accepted process-exit/group checks. Do not kill immediately after `ask` and then call that a clean exit. Account for delegated/background work and disallow additional business actions after a terminal disposition.
2. Commit verification, receipt revocation, question creation and task state together. Reconcile the separate process registry idempotently across crashes. Late old-token calls must fail. A signal, timeout, missing PID or failed observation is still not clean completion.
3. Resume only the expected persisted profile/ID, with a fresh per-run capability. Use a unique question ID, an answer operation ID and an expected task/question version; an answer from the owner and one from the coordinator must not launch two runs. Reject late answers after completion/cancellation or after a newer question exists.
4. Preserve the accepted ownership policy across restart, and show blocked/unknown honestly. Failure after partial work is not a safe fresh start merely because a new session would be used. Link deliberate replacements with `retry_of`/`supersedes`, warn about repeated effects, and require authorization for replacement work.

If the owner does not accept best-effort same-conversation resume, retain one owned worker process **for the lifetime of that task**, including its question waits, then retire it. That is a bounded alternative using existing adapters, not a requirement to keep permanent worker conversations. Process lifetime should remain an adapter choice rather than the product's central abstraction.

## 3. New conversation identity and pre-launch records

The CLI interfaces support this direction, but flags alone are not proof of correct identity. Claude documents a caller-supplied UUID with `--session-id`, exact-ID resume, and options that disable persistence. Codex documents new exec runs, exact-ID resume and `--ephemeral`. Persistence-disabling options must be excluded for question/answer continuation. [Claude CLI reference](https://code.claude.com/docs/en/cli-reference), [Codex CLI reference](https://learn.chatgpt.com/docs/cli/reference).

Before launching, durably record:

- stable task ID and command idempotency key; approved plan ID/version/item; immutable task text or its stored payload/hash;
- target device identity, provider, canonical profile/workspace, resolved CLI, selected tool/permission configuration and supported version;
- run ID/ordinal, fresh receipt/capability, creation-versus-resume mode, expected prior state, authorization consumption and launch intent;
- Claude's generated UUID, or Codex's **unbound pending identity**; a launch reservation must exist even when no provider ID/PID is known yet.

After start, persist process identity and bind the provider ID from the expected startup event before opening worker MCP access. For Claude, require equality with the stored UUID. For Codex creation, bind once to the ID emitted by this launched process and reject conflicting/later identities or an ID already bound to another task. For resume, require equality with the existing binding. Never fall back to latest-session selection, a fork, or a fresh conversation when resume fails.

Prefer a bootstrap prompt that tells the worker to call `read_task` after identity binding, instead of giving it the whole actionable task before verification. This reduces pre-binding work; it does not make untrusted executable/configuration startup safe. Keep the same profile and explicitly checked MCP/tool policy on every question round. `--ignore-user-config` is not proof that every configuration source is excluded; the current app-server effective-config checks do not automatically transfer to exec. [Codex non-interactive behavior](https://learn.chatgpt.com/docs/non-interactive-mode).

A crash after Codex starts but before its ID is saved is **unknown**, not permission to launch another new thread. Preserve the intent and evidence. Missing/unresumable history blocks continuation. A supplied Claude UUID is not an idempotency guarantee for relaunching a process either.

Reusable evidence: [SetupNewClaude](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/core/setup.go#L17) already binds immutable setup data; [AdapterLauncher](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/supervisor/launch.go#L102) reserves before starting; [identity collection](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/supervisor/supervisor.go#L325) currently compares against an already-known ID. It must gain a controlled first-bind path. The generic launcher currently rejects new Claude sessions; new-Claude support exists in the stream adapter, so new one-shot creation is not already wired up.

## 4. Plan approval: prefer option B

**Approve a bounded plan once in Messenger and enforce it there.** This is more consistent across clients and less tedious for several workers. The first plan can contain just one task, so the initial UI can remain small.

Option A is usable for a manual prototype with a verified permission configuration. It is a weak universal product contract: Claude permission modes/allow rules affect prompts, and Codex offers per-tool approval modes and automatic approval review. A successful MCP call does not inherently carry evidence that a human approved this exact plan. [Claude permissions](https://code.claude.com/docs/en/permissions), [Codex approval configuration](https://learn.chatgpt.com/docs/config-file/config-reference).

Recommended shape: `propose_plan` stores a versioned manifest; the owner approves it through the trusted UI; `start_task(plan_id, item_id, operation_id)` consumes an approved item exactly once. The target resolves approved workspace/provider references rather than accepting arbitrary replacement paths or executables. Altering the task, permissions, destination or allowed work requires a new approved version. The coordinator must not receive an API that can manufacture owner approval.

Budget both task starts **and question/resume rounds**, concurrency and model usage where measurable. A finite number of tasks alone does not bound an infinite ask/answer loop. Define when an answer is clarification within the approved task and when it introduces new work. Coordinator answers cannot impersonate owner approval or expand worker permissions. Skills and tool descriptions assist the workflow; enforcement belongs in code.

A denied native file/shell permission is also not automatically a Messenger `ask_owner`. Choose a deliberately unattended policy or implement a permission escalation path; do not promise that the worker's free-text questions capture every native prompt. Worker tools must not expose coordinator `start_task` or approval capabilities. Protect the daemon's data and credentials according to the worker permission boundary; a full-access same-user shell can bypass a mere MCP tool-list distinction.

## 5. Reuse, schema changes and the smallest end-to-end slice

| Existing component | Reuse | Necessary change |
|---|---|---|
| SQLite store and canonical session identity | Transactions, stable IDs, provider/profile/ID key | Task and plan identity above sessions; one task has multiple runs when questions occur. |
| Requests, attempts, verifications | One input and one attempt per run; execution evidence | Separate task/run states and a terminal disposition kind; do not store task completion in a session's `decision` alone. |
| Replies/routing | Correlation and duplicate rejection | Provisional report/question, then atomic verified publication; task events replace blind alias reply chains. |
| UI outbox | Immutable envelopes, lost-ack recovery pattern | Use the same principles for plan-item launch and answer operations; do not expose it unchanged as a coordinator inbox. |
| Events/timeline | Durable history and presentation | Structured event payloads, per-coordinator replay cursor/ack, bounded reads and question correlation. Current diagnostic event text is insufficient. |
| Adapters/platform | Argument construction, event parsing, process identity | New one-shot creation, profile-aware terminal validation, explicit within-task resume policy; Windows remains a separate implementation. |
| Local UI/shell/updates | Existing presentation and delivery infrastructure | Tasks and questions, approval UI, accurate cancellation/blocked states and coordinator connection status. |

Code anchors: [schema](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/core/store.go#L20), [UI outbox](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/core/ui_outbox.go#L33), [immediate reply routing](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/core/store.go#L533), [global root reconciliation](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/supervisor/runs.go#L268).

The promise that one blocked task never blocks unrelated work is a design change: current root reconciliation rejects another data directory while reservations remain, and corrupt global state blocks startup. Do not delete those protections or reinterpret corrupt global state as one known isolated task. Scope recoverable per-task evidence and genuinely global uncertainty separately.

**T-022 correction:** publish after the completion evidence appropriate to each path. One-shot runs require observed exit; persistent turns do not have a successful process exit. They use [CompleteStreamTurn](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/core/stream.go#L213) and [CompleteCodexTurn](https://github.com/kolotiloff/agent-messenger/blob/7d02b626f3bc94e0ebcdbd4826b5b73f6e2e95b4/internal/core/codex_setup.go#L464). Requiring exit status before every existing persistent reply would stall delivery until the owner process ends. Transactional publication belongs at verified turn completion there; retain its live owner.

**First slice:** before major restructuring, test the actual coordinator clients with a tiny read-only MCP event source, including long-wait/background/cancel/restart behavior. This acceptance may itself need a few approved model turns, even with fake workers. Then implement one approved task on one Mac: one coordinator, one fake worker, ask → one answer → exact resume → verified report → replayable event. Repeat launch and answer calls after simulated lost responses and prove there is still only one corresponding run. Crash at each persistence boundary; the task must be recoverable or explicitly blocked, never silently replayed.

Only after that add real isolated Claude/Codex workers, a bounded edit/test/report task, owner questions and a second parallel worker in a separate workspace. Keep old paths until closure/migration and rollback are accepted. Archiving is not deletion: preserve transcripts and task artifacts. Do not promise a full worker transcript merely from keeping final reports; define which sanitized tool/progress events the window retains. No real-model acceptance or implementation is part of this review.

## 6. Status of R1–R6 from the first review

| Prior finding | Assessment of version 3 |
|---|---|
| R1: leftover processes | **Partly addressed.** Different tasks avoid accidental same-ID reuse, but questions reuse the ID and unrelated tasks can still share files, ports or external resources. The strong guarantee is not restored. |
| R2: early reports | **Correctly addressed in the proposal, unimplemented.** Include asks and failure events, atomic publication and the persistent-turn distinction above. |
| R3: batching | **Resolved at the design level.** Combining already-verified event reads is different from combining worker attempts. Add output bounds and replay cursors. |
| R4: worker validation | **Correct direction, unimplemented.** Terminal report/ask semantics, native permission handling, allowed actions and report limits must be enforced consistently in MCP and core. |
| R5: migration | **Reduced, not eliminated.** Existing live owners, pending messages, receipts and history still exist. Closing a task in the UI does not prove an old process stopped. |
| R6: unbounded loops | **Partial.** Server-enforced approved plan items can bound task creation; prompting alone cannot. Bound within-task ask/answer runs and deliberate replacements too. |

Also make `cancel(task_id)` mean “cancellation requested” until its result is established. Handle queued, running and waiting-for-answer tasks separately, revoke further capabilities and reject late answers. A completed report may race cancellation; choose and persist one state transition. Unknown cleanup must not be shown as a proven stop.

## 7. Overall opinion: convenience, token cost and effectiveness

### Convenience

Version 3 fits the owner's preference for dictating to a coordinator in a familiar app. One place for plans/results and a Messenger task list can remove manual copying and make mixed-provider work easier to follow. Prefer one plan approval, concise notifications and a visible “coordinator disconnected/not listening” state. If the coordinator changes, the next chat should load a durable plan/task summary, not require importing the entire old conversation.

The cost is operating two interfaces, integration setup, potential busy/background calls and more worker session records. The owner's existing Claude-only Remote Control workflow already has a mature view into running sessions; replacing that solely to reproduce it is not an immediate gain. Remote Control keeps execution and tools on the originating machine and requires its local process to remain available. [Official Remote Control documentation](https://code.claude.com/docs/en/remote-control). Messenger's additional value is provider-neutral dispatch, durable task accounting, controlled approvals and eventually several computers. Network value is still future work.

Do not restrict the product permanently to this coordinator surface. Make coordinator MCP an adapter over the task service so another coordinator, including a later managed one, can use the same contracts.

This is also a scope decision: version 3 is a task runner with messaging, rather than only a messenger between existing agents. That can be the right initial workflow, but record it explicitly in Q-008 and avoid presenting attachment to arbitrary existing chats as already supported.

### Token cost: no guaranteed saving

Waiting in ordinary program code costs no model tokens. **Returning an empty wait result and asking the model to wait again does.** Existing idle persistent workers also do not need model polling. Process lifetime alone is not the source of savings. Background notifications and permission interactions can add turns too.

Illustrative upper-bound cadence for eight hours with no useful events, ignoring model/network latency:

| Application wait | Empty model resumptions over 8 hours | Input presented if each resumption carries 50,000 tokens |
|---|---:|---:|
| 30 seconds | about 960 | 48 million |
| 5 minutes | about 96 | 4.8 million |
| 10 minutes | about 48 | 2.4 million |

These are **hypothetical input-volume estimates, not measured usage, prices or subscription charges**. They assume the client returns every timeout to the model and it keeps reissuing the tool. Caching reduces processing cost but does not make those calls free. For illustration only, a 95% cache-hit share at a hypothetical 0.1 cached/uncached price ratio gives `50,000 × (0.05 + 0.95 × 0.1) = 7,250` uncached-input-price-equivalent tokens per resumption, before output and cache writes: about 348,000 equivalents for 48 resumptions, versus 6.96 million for 960. Actual model/account rates and cache behavior must replace these assumptions.

Claude documents different cache lifetimes by request/account mode: the main subscription conversation normally requests one hour within included usage, while API/usage-credit defaults are five minutes. Cross-session reuse depends on matching prefixes; a new session does not necessarily mean a completely cold cache. A ten-minute interval can exceed a five-minute cache lifetime. [Claude Code prompt caching](https://code.claude.com/docs/en/prompt-caching). Do not send empty model turns just to keep a cache warm without a measured benefit.

Fresh tasks save irrelevant historical context and can reduce compaction. They also repeat exploration, setup and project reads. For scale, ten workers each needing 20,000 tokens of project background present 200,000 tokens of background before other work; that is not necessarily 200,000 uncached tokens. A long-lived worker with 150,000 tokens is not automatically more expensive than a fresh 20,000-token worker if most of the former is cached. Conversely, a clean small task can be much cheaper than repeatedly carrying a large irrelevant history. Compare equivalent work and cache hits, not just context sizes.

Use bounded **coherent work packages**, such as “implement and test this feature,” rather than a fresh session for every edit or command. Allow question rounds inside the package, send concise structured reports with artifact references, and fetch details on demand. The coordinator's own context still grows; one-task sessions do not solve that. Count coordinator input/cached/output tokens, workers and subagents, repeated exploration, failed work and latency for the same multi-task benchmark. Under subscriptions, report observed usage/limits rather than inventing a dollar conversion.

### Effectiveness and speed

The model works well for independent, clearly specified tasks with acceptance criteria and a stable workspace snapshot. Fresh workers reduce stale assumptions and bound history. Parallel workers can improve throughput; startup and repeated exploration make tiny sequential tasks slower. The coordinator needs real dependencies and workspace targets, not merely a list of prose instructions.

Different session IDs do not prevent two workers or an escaped old command from editing the same files. For the first pilot, use separate workspaces or serialize writers per workspace; retain coordinator scheduling for declared shared resources. A blocked old writer must remain visible when scheduling replacement work into its workspace. No source-control product needs to be mandated.

Verified `report` means the provider/protocol/run passed checks, **not that the requested change is correct**. Ask workers for changed artifacts, tests run, failures and remaining limitations; keep owner/coordinator review or independent tests as the acceptance step. Durable intent, idempotency and event replay prevent Messenger-generated duplicate dispatch, but arbitrary filesystem/network effects cannot be made exactly-once merely by assigning task IDs.

**Recommended direction:** adopt task-oriented orchestration as the product interface, keep transport/lifetime choices behind provider adapters, prefer approved plans and coarse tasks, and prove desktop waiting plus one ask/answer cycle first. Version 3 can be more convenient and reliable for mixed-provider work, but is not yet demonstrated to be cheaper, faster or safer than the owner's current setup. Preserve the working setup during the pilot rather than replacing it before those claims are tested.

## Verification and limits of this review

- Reviewed the exact proposal and branch documents at `7d02b62`; GitHub comparison confirms product code, tests and build scripts are unchanged from the previously reviewed `59d58eb` code. Reused that local checkout for code inspection and checks; its product files were verified unmodified.
- Ran `scripts/check.sh` on that identical code with Go 1.27.3, macOS arm64: formatting, module verification, vet and all race tests passed. An initial restricted run could not bind test sockets or inspect native process/boot state; the subsequent native-permission run completed successfully. That restriction was not treated as a product failure.
- No product code, tests, CLI settings, reservations, user sessions or releases were changed. No real model calls, coordinator UI integration, Windows execution, or new task lifecycle were tested. The official documentation establishes supported interfaces, not acceptance of the proposed integration on the owner's installed app versions.
