# Rethinking the product: one task, one session; the coordinator in the owner's app

Status: **proposal, version 3, not a decision.** Written by Claude Code on 2026-11-06 together with the owner. It replaces versions 1 (`59d58eb`) and 2 (`6fa7f84`), which proposed one-shot runs over long-lived saved worker conversations. Codex reviewed version 1 in [RETHINK_REVIEW.md](RETHINK_REVIEW.md); section 8 maps that review onto this version. The owner decides ([Q-008](QUESTIONS.md)). Until then every rule in [AGENTS.md](../AGENTS.md) and [DECISIONS.md](DECISIONS.md) stays in force. No product code was changed for this document.

## 1. Summary

1. **The coordinator is the owner's own chat** in the Claude app, Claude Code or Codex, with Messenger connected as an MCP server plus a Messenger skill. Messenger never launches or owns the coordinator.
2. **One task, one new worker session.** The coordinator proposes a plan, the owner approves it, and Messenger starts a new session of the chosen agent on the chosen computer. The worker does the task, may ask questions, reports, and the session is closed for good. A new task gets a new session.
3. **The coordinator waits for reports with a blocking `wait` tool.** No model tokens are spent while it waits. When a report or question arrives, `wait` returns, the coordinator acts, and calls `wait` again.
4. **A worker's report is published only after its run is verified** (identity, completion, exit status, one bound reply).
5. **Messenger's own window** is the panel for tasks, statuses, questions and the full history.

Compared with today, this removes long-lived ownership of worker conversations: there is no session to guard between tasks, nothing to free for reuse, and no reuse to migrate.

## 2. How the owner works with it

1. Once: connect Messenger to the Claude or Codex app as an MCP server and install the Messenger skill.
2. In the usual chat, by text or dictation: "we need X". The coordinator calls `list_agents` and proposes a plan: "on the Mac, Claude does A in folder P; on Windows, Codex does B in folder Q".
3. The owner approves. The coordinator calls `start_task` for each part.
4. Workers run on their computers. The coordinator calls `wait`.
5. A worker asks the coordinator something: `wait` returns the question, the coordinator answers with `answer`, and calls `wait` again. A question for the owner goes to the Messenger window and a Mac notification; the owner answers there or through the coordinator.
6. A worker reports. After verification, `wait` returns the report. The coordinator gives the next task or tells the owner it is done.
7. Something breaks: the task shows **failed** or **interrupted** in the window. The coordinator or the owner decides whether to start it again as a new task. Nothing is retried automatically.

## 3. The coordinator

### 3.1 Tools Messenger gives the coordinator (MCP)

| Tool | Effect |
|---|---|
| `list_agents()` | Computers, installed agents, their workspaces and whether they are online. No model call. |
| `start_task(computer, agent, workspace, text)` | Creates a task and its new worker session. Returns a task ID. Subject to the approval rule in 3.3. |
| `wait(timeout)` | Blocks until there is at least one new report, question or owner message for the coordinator, or until the timeout. Returns everything new at once. |
| `tasks()` | Status of every task: queued, running, waiting for an answer, done, failed, interrupted. |
| `answer(task_id, text)` | Answers a worker's question; the worker continues. |
| `cancel(task_id)` | Stops a task. |

### 3.2 Waiting without spending tokens

While `wait` is open, the model generates nothing, so no tokens are spent. Facts and limits to verify:

- **Tool-call time limits.** Clients limit how long an MCP tool call may run (Codex has a per-server tool timeout; Claude Code has its own limit). `wait` therefore returns "nothing yet" after a bounded time, for example 10 minutes, and the skill tells the coordinator to call it again. Each repeat is a small model turn.
- **Every wake-up is a normal turn.** The model re-reads its conversation, as if the owner had written "what's new?". Prompt caching makes this cheaper if wake-ups are close together. Reports that arrive together are returned together.
- **The chat is busy while waiting.** To say something, the owner stops the call (or writes in the Messenger window, which ends `wait` with the owner's message). After the owner's turn, the coordinator calls `wait` again.
- **The chat must stay open and the Mac awake.** If the app is closed, reports stay queued in Messenger and are returned by the next `wait` or `tasks`.

Optional later improvements for Claude only, after a Mac test: an `asyncRewake` hook or a Claude Code channel could wake an idle chat without a blocking call. Channels are a research preview documented for the terminal CLI. Neither is needed for the first version.

### 3.3 Approval of the plan

`start_task` launches a model and, for workers with real tools, changes files. The first version requires the owner's approval:

- **Option A:** the client's own permission prompt for `start_task` (the owner approves each call in the app).
- **Option B:** the coordinator submits the whole plan with `propose_plan`, and the owner approves it once in the Messenger window.

Which one is the owner's decision (Q-008).

### 3.4 The skill

A short skill (and the MCP `instructions`) tells the coordinator: list agents before planning; propose and get approval; start tasks; then loop on `wait`; answer worker questions; tell the owner when everything is done; never start a task again by itself after **failed** or **interrupted**.

## 4. Workers: one task, one session

### 4.1 Lifecycle

1. `start_task` records the task durably **before** anything is launched.
2. Messenger on the target computer starts a **new** conversation of the chosen agent in the chosen workspace (`claude -p --session-id <new uuid>`; for Codex, a new `codex exec` thread whose ID is read from its events). The task text is the first message.
3. The worker works with real tools (section 4.4) and its own sub-agents. It ends its run with one of:
   - `report(text)`: the task is done;
   - `ask(text)`: a question for the coordinator; the run ends and the task is **waiting for an answer**;
   - `ask_owner(text)`: the same, but for the owner.
4. On an answer, Messenger resumes **the same task conversation** by exact ID (`--resume`, `codex exec resume`) with the answer as the next message. This is the only reuse, and only within one task.
5. After a verified `report`, the task is **done** and its conversation is never resumed again.

Questions end the run instead of keeping a process waiting, so no worker process lives while it waits for an answer.

### 4.2 Verification before publishing

A worker's `report` or `ask` is stored provisionally and becomes visible to the coordinator (through `wait`) only after the run is verified: the expected conversation ID, provider completion, exit status, the process group, and exactly one final `report`/`ask` bound to the run's receipt. If verification fails, the task becomes **failed** and the text is shown in the window marked as unverified.

Today `replyTx` routes a reply in the transaction that stores it, before the run is checked (T-022). The fix is needed in any case.

### 4.3 Ownership and failures

- A task conversation has at most one managed run at a time: Messenger never starts the next run (an answer) while the previous one is not cleanly finished.
- Unclear evidence still blocks: no recorded PID, incomplete observation, failed inspection or a corrupt registry keeps the task **blocked** and the window says why.
- **failed** (cleanly observed, but invalid) and **interrupted** (client crashed, outcome unknown) are separate statuses. Neither is retried automatically. Starting again means a **new task with a new session**, decided by the owner or the coordinator, with the old task's history visible.
- A finished task needs no "release". An escaped descendant could still act, as with any process given real tools. Because the next task uses a new conversation, it cannot collide with that conversation. A strong guarantee against leftover processes needs an OS sandbox (Q-008).

This is the main simplification: the reservation that today lasts until a reboot is replaced by "this task is blocked", which does not stop other tasks or agents.

### 4.4 Tool profiles and validation

- `messenger-only`: today's stripped profile, for automated tests.
- `worker`: the provider's normal tools plus the Messenger worker tools (`read_task`, `report`, `ask`, `ask_owner`). Claude without `--tools ""`; Codex with `--sandbox workspace-write` and shell enabled. The permission mode is the owner's decision (Q-008).

Validators must check provider identity, completion and the Messenger protocol strictly, but allow the profile's normal actions. Today's validators require exactly `read_current, reply` and would reject real work after it was done (review R4). The run prompt changes accordingly. The reply limit of 2000 characters in `internal/mcp/server.go` is too small for reports and needs a larger, still bounded, limit.

### 4.5 Context

Each task starts with a clean context, so worker context cannot pile up. The cost is that a worker learns the project again for each task. The coordinator puts what is needed into the task text, and the project keeps its own state documents (as this repository does with `HANDOFF.md`).

## 5. Several computers

The coordinator's Messenger (the one its MCP server talks to) creates tasks. A task for another computer goes through the broker (stage 5, T-015) to that computer's Messenger, which runs the worker locally and sends back the verified report. Rules from the review stay: the broker separates "task received" from "task finished", never reruns uncertain work, and does not treat a lost connection as proof that a worker stopped. Until the broker exists, everything runs on one Mac.

## 6. What happens to the code

Estimates from reading the code.

**Reuse and adapt:** `internal/core` (store, queue, outbox, timeline; routing becomes task routing), `internal/localui` (tasks, questions, statuses), `desktop/macos` and updates, `internal/diagnostics`, `internal/platform`, the one-shot adapters and validators, `internal/mcp` (two tool sets: coordinator over a local socket or HTTP, worker per run).

**New:** a `tasks` table and lifecycle; coordinator MCP tools including `wait`; worker tools `report`, `ask`, `ask_owner`; new-conversation creation per task; the skill; notifications on the Mac.

**No longer needed for new work:** persistent Claude stream owners, persistent Codex app-server owners, long-lived worker bindings, reboot-based release of worker reservations. They stay in the code until existing sessions and state are handled (closed or migrated, with schema versioning), then they are removed.

## 7. Order of work, if Q-008 is accepted

0. **Independent of Q-008:** publish a reply only after verification (T-022).
1. Record the decision: the task model, the plan approval option (3.3), the worker permission mode, the ownership statement (4.3). Update `AGENTS.md`.
2. Tasks table and lifecycle on one computer, `messenger-only` profile, fake CLIs: start, report, ask/answer/resume, failed, interrupted, blocked, no automatic retry, no early report.
3. Coordinator MCP tools and `wait`, tested against fake workers; check the tool-call time limits of the installed Claude Code and Codex.
4. Worker profile and validators: a real edit or command, partial work then failure, a denied permission.
5. Mac acceptance with real models in `~/AgentMessengerTest/`, new test sessions only: a coordinator chat in the Claude app (then in Codex) with the skill, two workers (Claude, Codex), a question to the coordinator, a question to the owner, two full tasks.
6. Close or migrate old persistent sessions; remove persistent code.
7. Broker and Mac ↔ Windows (T-015); Windows process ownership.

T-009 waits for Q-008.

## 8. The version 1 review and this version

| Review finding | In version 3 |
|---|---|
| R1: receipt revocation does not stop leftover processes | Still true for the runs of one task. Strict blocking on unclear evidence stays (4.3). Leftovers cannot collide with the next task, which uses a new conversation. Strong isolation needs an OS sandbox. |
| R2: report routed before verification | Provisional report, published after verification (4.2); T-022. |
| R3: batching does not fit the schema | No batching of worker runs. `wait` returns several verified reports at once to the coordinator, which is a read, not a combined run. |
| R4: validators reject worker actions | Profile-aware validators and prompt (4.4). |
| R5: migration needed | Much smaller: no long-lived worker bindings are reused. Old persistent sessions are closed or migrated before their code is removed (6). |
| R6: coordinator resets defeat loop limits | Workers cannot start tasks; only the coordinator can, and only with the owner's approval (3.3). A plan has a bounded number of tasks; going beyond it needs a new approval. |

## 9. Questions for the reviewer

1. Is a blocking `wait` MCP call workable in Claude Code (CLI and desktop app) and in Codex (CLI and app): time limits, behaviour when the owner interrupts, behaviour after a client restart? What is the safest bounded duration?
2. Is ending a worker's run on `ask` and resuming the same task conversation for the answer safe under the rules in 4.3? Is anything lost compared with the review's requirements?
3. Is creating a new conversation per task with a known UUID (Claude) and a reported thread ID (Codex) safe against silently creating a different conversation? What must be recorded before launch?
4. Which of the plan approval options in 3.3 fits the clients best, given that MCP tool approval prompts differ between Claude and Codex?
5. What in the current schema and code can be reused for `tasks`, and what is the minimal first slice that proves the model end to end?
6. Anything in the review (R1–R6) that this version still gets wrong.
7. **Overall opinion**, compared with today's persistent model and with the owner's current manual setup (Claude Code sessions over Remote Control):
   - **convenience** for the owner: giving tasks, following progress, answering questions;
   - **token cost**: waiting, wake-ups of the coordinator, a fresh context per task versus long-lived workers; where the model saves or wastes tokens, with rough estimates where possible;
   - **effectiveness**: how well the coordinator and workers get the job done, speed, risk of conflicts and of lost or duplicated work.
   Is this the right direction, and what would you change?
