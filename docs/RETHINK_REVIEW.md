# Review of RETHINK.md — T-021

Reviewed by Codex on 2026-11-06 against **59d58eb81424d62132f63790fb04d15cd3381e68**, branch `claude/vigilant-darwin-34d88b`.

**Recommendation: accept the direction with changes; do not implement the proposal unchanged.** Saved conversations with one process per message are a reasonable product model. Keeping a process alive is not a requirement for remembering a conversation. However, this proposal changes the ownership guarantee and several message contracts, not just process lifetime. Q-008 remains an owner decision; this review does not supersede any existing rule.

The findings below are code-derived counterexamples and design gaps, not claims that a new real-model experiment reproduced them. No product code was changed and no real agent/model was launched for this review.

## Findings, in priority order

### R1 — P1: receipt revocation does not preserve exclusive conversation ownership

Sections 4.3–4.4 accept escaped descendants, but understate what that gives up. A descendant with the relevant permissions can continue model calls, run another CLI against the same saved conversation/profile, alter provider history, invoke network services, or start further processes. None of these operations needs a Messenger receipt. This can overlap the next Messenger run even though its predecessor's process group is empty. Revocation only rejects subsequent calls through the bound Messenger API; it does not undo actions or deliveries already performed.

Evidence: [process proof and identity checks](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/platform/process.go#L26), [MCP authorization](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/core/store.go#L1087), and [launch reservation before process creation](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/supervisor/launch.go#L102). The local MCP server opens the database directly. A worker allowed unrestricted same-user filesystem access is not isolated from that control plane merely by choosing a working directory.

Required amendment:

- Name the proposed guarantee accurately: one managed launcher at a time, with best-effort process-group cleanup; not proof that every process capable of using the conversation has stopped. The owner may accept that tradeoff explicitly. A stronger guarantee needs an enforced process/permission boundary.
- Separate normal observed completion from ambiguous crash recovery. A pending launch with no recorded PID, incomplete observation, failed process inspection, corrupt registry, or missing database cannot become evidence of a stopped run. Retain identity checks against PID/PGID reuse before signaling.
- Define crash-safe ordering across SQLite and the separate registry: durable result/revocation, reservation retirement, and eligibility for another claim. Recovery must finish an interrupted transition idempotently. A crash between these writes must neither relaunch the same request nor allow another root to launch a competing owner.
- Missing data is not termination evidence. Offer an explicitly scoped recovery/rebinding policy, without silently forgetting a possibly live owner. Keep durable data directories regardless of process model.
- If cleanup has to terminate remaining worker processes, distinguish that from clean task completion. Otherwise a report can say success while an unfinished delegated operation is being killed.

Windows is not covered by the current implementation: [group setup](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/platform/group_windows.go), process observation and locking return unsupported errors. It needs a separately tested ownership implementation, for example Job Objects with explicit child/breakaway and handle-lifetime policies. Job membership is useful but is not a universal security boundary around externally started services. [Microsoft Job Objects documentation](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects).

### R2 — P1: a report can reach the coordinator before the worker is verified

Section 4.5 treats a routed `reply` as the worker's finished report while section 5 keeps routing unchanged. Today [replyTx](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/core/store.go#L533) calls [routeReply](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/core/routing.go#L17) in the transaction that stores the reply. Provider validation happens later in [runClaimed](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/supervisor/supervisor.go#L279).

Counterexample: worker calls `reply("done")`; coordinator consumes the routed message; worker then exits nonzero or emits an invalid action. Revoking the worker's receipt cannot retract the coordinator's work. Parallel one-shot workers would inherit this behavior. Agent `send` also creates queued work immediately.

Choose an explicit contract before implementation. Recommended: store the final report provisionally, then publish it exactly once only after identity, provider completion, exit status and the bound reply pass validation. Publish and mark completion in one durable transaction, with recovery for the separate reservation transition. Deliberately immediate messages/questions need separate semantics and delivery history; do not imply they are rolled back on failure.

Retry/Skip must account for already committed effects and messages. Current [failed-attempt recovery](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/core/store.go#L807) rejects an attempt with a stored reply; the proposed general Retry button cannot reuse that contract unchanged. Also distinguish a failed task from an interrupted/unknown outcome. Define whether later queued tasks remain paused after an ambiguous result; simply freeing the conversation can start dependent work without retrying the original message.

### R3 — P1: batching is not compatible with the existing receipt/reply schema

The [schema](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/core/store.go#L44) binds an attempt to one unique `request_id`, and a reply to one unique `reply_to`. [CurrentMessage](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/core/store.go#L504) returns one message; `replyTx` derives its recipient from that message's sender. Routing and send limits inherit that single request's lineage and hop count.

Two reports from different senders claimed together have no single automatic reply recipient. Concatenating their text loses per-message acknowledgement, partial-failure handling and routing lineage; even same-sender batching needs an explicit acknowledgement contract.

Recommendation: **one message per attempt in the first replacement path**. Later batching needs immutable batch membership, bounded size, per-item disposition/reply correlation, retry rules for partially handled batches, and preserved hop/task budgets. Keep recipient selection in trusted code. This is a store/protocol change, not an adapter-only optimization.

### R4 — P1: enabling worker tools without replacing validators rejects valid work after it happens

Section 5 cannot keep the one-shot validators unchanged. [ValidateClaude](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/adapters/adapters.go#L178) requires exactly the read/reply tool sequence. [ValidateCodex](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/adapters/adapters.go#L250) rejects completed non-MCP actions other than messages, reasoning and plan updates. Both also need changes for `send`.

Changing launch flags alone lets a worker edit a file or run a command, then marks the turn invalid; retry can repeat the actual work. Split validation into provider identity/completion, Messenger protocol/correlation, and the chosen tool profile. Permit the worker's expected action events without weakening receipt checks or treating unknown completion as success. Update the prompt too: the existing one-shot instructions prohibit normal working tools.

Specify actual filesystem/network permissions and how a denied or approval-requiring tool is represented. A working directory is not by itself a sandbox. The permission profile and which settings/hooks are inherited must be deliberate, especially for unattended workers.

### R5 — P1: removing persistent code needs a migration, not just new-session acceptance

Existing Codex bindings are explicitly `codex_owned`, with `sessions.cli_path = NULL`; the CLI path is in `codex_setups`. [Setup registration](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/core/codex_setup.go#L240) and [generic claims](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/core/store.go#L384) intentionally prevent the one-shot path from taking them. Global reservations also distinguish persistent stream/setup owners.

Before removal, specify migration of bindings, pending requests, replies, attempts, reservations and setup intent. Preserve exact profile/ID and history, establish that the old owner cannot race the new launcher under the accepted policy, and test crash/restart during migration. Keep recovery/read support for old state until migrated; do not reinterpret old uncertain reservations as new clean completions. Version the schema and test supported upgrade/rollback behavior.

The proposed initial `codex exec` introduction is a model call, not a free ID allocator. Durable setup intent, failure before/after ID persistence, and refusal to silently create a replacement conversation still matter. Existing app-server-created threads with saved tools also need explicit exec-resume migration acceptance; a newly created exec-only conversation does not cover them.

### R6 — P2: coordinator resets can defeat the loop bound

The current [routing limits](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/core/routing.go#L10) count reply/send hops and cap sends per turn. If the coordinator can declare each response a new task and reset the counter, an automatic loop can continue indefinitely while each individual chain obeys the limit.

Define task identity, reset authority and an overall automatic-work budget before introducing this feature. The first version can require an owner action to start a fresh budget. An owner address also needs to be a reserved recipient type, not a potentially colliding agent alias. A question needs a correlation ID and a defined answer/resume or waiting policy, rather than just a text message in the window.

## Answers to section 9

1. **Is exit + empty group + revoked receipt enough?** Enough only for the explicitly weaker, best-effort managed-launcher policy after a clean, fully observed run. It is not enough to preserve the current no-overlap guarantee against escaped descendants, and cannot be applied blindly to crashes or missing data. Preserve launch intent, process identity and durable transition checks (R1). Windows is unimplemented and requires native acceptance, not cross-compilation.

2. **Can a leftover process do harm beyond file edits?** Yes: further model usage/cost, external requests and side effects, new subprocesses, or another writer to the same provider conversation. Already delivered messages and external actions survive receipt revocation (R1–R2). What is possible depends on the worker's actual permissions; the receipt is an application capability, not an OS isolation mechanism.

3. **Does `codex exec resume` keep all app-server guarantees?** No automatic parity. Exact-ID resume is supported; keep checking the emitted ID and fail closed on mismatch/missing history. Official documentation says `--ignore-user-config` excludes `$CODEX_HOME/config.toml`; exec-policy rules have a separate option. Required MCP startup failure is an error. Those documented capabilities do not prove equivalence to this application's checks. [Official non-interactive documentation](https://learn.chatgpt.com/docs/non-interactive-mode).

   The current [CodexArgs](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/adapters/adapters.go#L67) supplies a fresh bound MCP configuration. It does not perform the app-server's [effective-config validation](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/adapters/codex_appserver.go#L40) or [per-turn fingerprint check](https://github.com/kolotiloff/agent-messenger/blob/59d58eb81424d62132f63790fb04d15cd3381e68/internal/supervisor/codex_setup.go#L502). The disabled-feature sets differ too. A separate app-server preflight is not proof of the effective config of a subsequent exec process.

   Re-establish the required properties for the supported CLI version: exact ID, correct profile, foreign/project configuration and tools, read-only versus worker permissions, required-MCP failure, current versus stale token, and changed per-run tools on the same saved conversation. Include a migrated app-server-created thread. Earlier [exec experiments](reports/2026-11-05-guarded-codex-cursor-mac.md) are useful evidence, but do not accept the proposed worker/migration path. Do not claim `send` works merely because a fresh config was constructed.

4. **Is anything slated for removal needed for networking?** The persistent RPC/stream transport is not inherently required. Its invariants and durable state handling are: single ownership of the canonical provider/profile/conversation key on its host, setup intent, exact identity, receipt fencing, crash reconciliation, deduplication and correlated delivery. Keep compatibility/recovery code until old states migrate (R5). A broker must distinguish durable receipt of a message from successful agent execution, retry transport delivery idempotently without automatically rerunning uncertain work, authenticate devices and addresses, and preserve task budgets. A lease timeout or disconnected computer is not evidence its agent stopped. These requirements apply with either process model; the local token is not network authorization.

5. **Is batching compatible with current checks?** No, not without changing the store and protocol (R3). Start with one request, one attempt, one correlated final reply. Design batching separately after the lifecycle works.

## Suggested first implementation slice, if Q-008 is accepted

1. Record the revised ownership/permission decision; define completion, provisional reports and crash recovery before changing unlock behavior.
2. Keep one message per attempt and the messenger-only profile. Implement transactional final-report publication and restart-safe reservation retirement. Keep existing persistent paths available for old bindings.
3. Fake-CLI acceptance: two consecutive messages in the same conversation; competing aliases/path spellings/roots; reply followed by failure; crash before PID persistence and around each completion/registry write; stale receipt; partial registry/database failure; detached child and group cleanup; no automatic retry; no premature routed report.
4. Verify the installed Codex exec configuration and exact-resume contract. Then add `send`, owner questions and bounded task cycles; test duplicates and late deliveries.
5. Add worker profiles and profile-aware validators. Test a real edit/command and its report, partial work followed by failure, denied permissions and child lifetime. Only then run the approved isolated Claude ↔ Codex Mac acceptance, in durable test directories.
6. Accept migration/recovery of existing bindings before removing persistent code. Native Windows ownership and Mac ↔ Windows network acceptance remain separate gates.

## Cost and scope corrections

- Idle token usage can be zero with either model. The current persistent implementation already waits without periodic model prompts; the background Messenger client also still runs under this proposal. Compare actual input/cached/output usage and latency over equivalent multi-turn tasks before claiming savings from process restarts or batching.
- Subagents can reduce what the parent needs to retain, but parent instructions, delegations and returned reports still accumulate. A fixed context size is not guaranteed by delegation alone.
- The deleted temporary directory incident establishes a durable-storage/recovery requirement, not proof that persistent processes caused the data loss.
- The keep/change list should say **reuse and adapt**, especially for core, routing, platform support, validators and UI state. This is a useful simplification direction, not a small unlock patch.

## Validation performed

On macOS arm64 with Go 1.27.3, at the reviewed commit: `bash scripts/check.sh` passed (formatting check, module verification, `go vet`, and all package tests with the race detector). Tests used their fake providers; no real Claude, Codex or Cursor session/model was launched. This validates the existing baseline, not the proposed lifecycle or Windows behavior. No new production code, tests, reservations, session data or app releases were changed by this review.
