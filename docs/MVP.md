# Staged MVP v0.3: Mac app first

Updated November 5, 2026 at the user's request. The interface, statuses, conversation history and installation from GitHub are part of the target MVP. A single broker supports hosting on your own computer or on a VPS; a VPS is optional. [Settings and boundaries](DEPLOYMENT.md). While only a Mac is available, checks and the first user version are done locally. Mac↔Windows remains the required next network acceptance.

The current sequence, the product client's results and the open release conditions are collected in the [plan to release](RELEASE_PLAN.md). The original MVP breakdown is kept below; the early test bench results do not mean acceptance of all CLIs in the persistent Go client.

## Where we are now

On Mac, basic resume of Claude/Codex/Cursor and real Codex–Cursor and Codex–Claude pairs over the earlier MCP have been verified. In the new local test bench, 19 model-free tests of reply linking, the queue, the single owner and the ID check passed. The new mechanism passed two consecutive jobs with real Claude in one session; concurrent reservation was blocked. Codex and Cursor also passed two jobs each with ID preservation and blocking of concurrent reservation. There is no finished application or installer. Current results are in [VALIDATION.md](VALIDATION.md).

On November 5, 2026 the persistent client and the future broker were settled on Go. The Python test bench remains for research. Stage 2 starts with a local client without GUI and network; progress and boundaries are in [CLIENT.md](CLIENT.md).

## Stages

| Stage | Result and exit condition |
|---|---|
| 1. CLI under client control — basic scenario verified on Mac | The new reply context and reservation work with a real CLI. Two consecutive jobs continue the exact ID; a concurrent launch is blocked; a wrong ID/no result is not counted as success. The three CLIs passed separate experiments; the negative ID/result cases are synthetic so far. |
| 2. Recovery and local core — next | Explicit states, a log, queue, pause and stop. On failure there is no automatic retry; recovery checks the identity/process tree and the recorded reply. The UI gets a documented local API for events/commands. The broker is separated from the client; the configuration distinguishes running a local server from connecting to an external one, with a broker_id check. |
| 3. First window on Mac | Agent cards, detection of CLIs and logins, clear install/authorization actions, a list of problems, conversation history and a send form; a choice between connecting to a server or the local server role. Real data, not demo statuses. |
| 4. Local release for Mac | An installable build in GitHub Releases, instructions for a human and an agent, verification of clean install/update/uninstall. Queues and secrets are not included in the build. |
| 5. Network broker and Mac↔Windows | The broker on one of the computers; the same component is verified separately on a VPS. A setup wizard for a supported VPS over SSH, a connection check and device registration are part of the network stage. Device authorization, addressing, reliable network delivery, retry after offline, token revocation, broker recovery. The same conversation history and statuses are visible in the app. |
| 6. Network MVP reliability | Two consecutive cross-machine exchanges with resume, failure tests, an hour of waiting without model launches, no duplicates, and a safe manual session handover. |
| 7. Four computers and the coordinator | Windows x2 and Linux, installation packages, autostart/sleep, switching the coordinator role with a log and preservation of active tasks. |
| 8. Optional additions | A Telegram mirror and additional agent adapters; after the main acceptance. |

Stages may partially overlap: a window mockup can be made once the states are defined, without waiting for the external network node to be ready. The project's working sessions are not automatically connected in place of test sessions.

## First local user scenario

Install the app → choose a local server (or connect to an existing one) → see three agent cards → install a missing CLI/sign in the standard way → create a managed test session → choose a recipient and send a message → see the queue, the processing and the linked reply → send a second message to the same session.

An open interactive conversation gets the manual-ownership status. Automatic resume waits for an explicit handover of control to the client.

## Acceptance

The local release requires stages 1–4 and the [interface](APP.md) checks. The network Mac↔Windows MVP additionally requires 5–6 and an available broker; a VPS is not a mandatory condition. Moving the broker and changing its identity are verified separately, without two copies running at the same time. Support for all four computers and switching the coordinator are confirmed separately in stage 7.

Ready means: one session owner; messages and replies are persisted; duplicates do not call the model; losing the network does not lose a message; an uncertain result requires investigation; waiting and the UI do not call the model; errors and the required actions are clear to a human.

Attachments, transferring internal history between providers, project management and automatic roadmap maintenance are not part of the MVP. A simple status/conversation interface is now included.

## Recovery from uncertain — a stage 2 requirement

1. Stop issuing new jobs to this binding and revoke the old result-writing context.
2. Match the specific execution against the profile, the conversation ID, the working folder and the identity of the processes, including descendants. A PID number or an expired timer is not enough.
3. If it cannot be proven that the execution has stopped, keep the lock and show the reason.
4. Reconcile the persisted reply and the CLI events. Separate a persisted reply from a confirmed process exit; an incomplete log is not counted as success.
5. Record the decision in the log: accept the verified result, finish with an error without retry, or explicitly create a new attempt after investigation. The old result is not overwritten; side effects are not assumed to be reversible.
6. Allow subsequent jobs only after the stop has been verified and the result decided. Automatic continuation on another machine does not replace this procedure.

Lifting the lock and retrying are not yet implemented in the test core; the current test bench deliberately blocks doubtful executions.

Source: [plan](PLAN.md), [interface and installation](APP.md).
