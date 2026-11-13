# Persistent Claude in the local client

> **Superseded.** The old session path; the window and task workers do not use it ([TASK_MODEL.md](TASK_MODEL.md)).

`messenger serve --claude-stream` supports sequential Claude messages in a single process. It is an explicit mode, enabled separately from the permission to call the model. In this mode Cursor stays in the queue; Codex sessions are served by their own owners (see [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md)).

**Several Claude sessions.** Each persistent Claude session gets its own controller and process, and up to 4 run at the same time. A live stream for one session no longer blocks another. `stop-job` and shutdown stop every live stream. A session whose stream is uncertain stays locked; the others keep working.

## Build and run

```sh
go test -race -count=1 ./...
go build -o /tmp/agent-messenger ./cmd/messenger
/tmp/agent-messenger --root /absolute/data/path serve --claude-stream
```

Without `--allow-model-calls`, no CLIs are launched. Control commands go through a local socket.

To close the terminal while keeping the client and the live CLI, use [background `start`](BACKGROUND.md). Every such start begins paused. Reconnecting does not launch a second process; `shutdown` still terminates the client itself and keeps the protective reservations.

On the verified Mac with a temporary Go 1.22.2, `-ldflags=-linkmode=external` was used; the built binary was signed with a local ad-hoc signature. Whether this workaround for the old toolchain is needed in another environment is verified separately.

If a global reservation from an earlier trial already exists, the previous `--root` directory is required. Changing the directory does not bypass the reservation. Do not delete the `active-runs.json` file manually.

## A new separate session without calling the model

With `serve --claude-stream` running:

```sh
messenger --root /absolute/data/path create-claude-session \
  --environment /absolute/path/to/.claude \
  --alias claude-test \
  --cli /absolute/path/to/claude \
  --workspace /absolute/isolated/workspace
```

The profile and workspace directories must exist. The command generates a new exact UUID and saves the configuration; Claude is not launched yet. The first launch uses `--session-id` with that UUID. No work or GUI chat is substituted. The existing login of the regular `~/.claude` is kept.

For an already existing free CLI session, `register-session --provider claude ... --conversation-id EXACT_ID` remains available; the first process uses an exact `--resume`. The initial verification was done with a new session. A separate trial of real Claude on Mac then confirmed an exact resume of a saved test session, the return of the earlier marker and one linked reply with `provider_ok=1`: [report](reports/2026-11-05-claude-stream-exact-resume-mac.md). Opening the same ID directly, bypassing Messenger, is outside the locking guarantee.

## Delivery and control

After registration, for an allowed model trial, stop the client that was used only for setup and run:

```sh
messenger --root /absolute/data/path serve --claude-stream --allow-model-calls
```

In another terminal:

```sh
messenger --root /absolute/data/path enqueue --alias claude-test --id m1 --from coordinator --text 'Reply exactly ACK:PAYLOAD-one'
messenger --root /absolute/data/path pause
messenger --root /absolute/data/path enqueue --alias claude-test --id m2 --from coordinator --text 'Reply exactly ACK:PAYLOAD-two'
messenger --root /absolute/data/path resume
messenger --root /absolute/data/path inspect
```

`pause` does not interrupt the current reply. It stops sending the next messages; the active process stays. `resume` continues the queue in the same live process. An empty queue is polled by local code; no prompts, "are you there?" messages or timer-driven calls are sent to the model. Observing the pause does not replace verifying billing or the hour-long H9.

A message succeeds with a linked `reply_to`, a confirmed exact ID and `provider_ok=1`. `Attempts.state=turn_completed`, `Requests.state=completed`. The turn has `exit_observed=0`, `exit_code=null`: the process is alive. After the reply, `StreamRuns.state=idle`, MCP is closed until the next confirmed turn, and the session lock is kept.

| Command/event | Behavior |
|---|---|
| `pause` | Subsequent messages stay in the queue; the current reply may finish |
| `resume` | Resumes only the live process's queue; does not clear `uncertain` |
| `stop-job` | First revokes access in SQLite, then cancels the channel, and may signal only an observed owned process group |
| `shutdown` | Revokes access, closes stdin/HTTP, does not wait indefinitely for a hung CLI; the queue and reservation remain |
| CLI, ID or result error, EOF or timeout | The unfinished turn becomes `uncertain`; no next message and no automatic retry |
| Restart after a crash | The old channel is revoked, completed turns are kept, unsent messages stay in the queue, no second CLI for this session is launched |

The process and the turn have separate records. If the exit was observed within a short shutdown, `StreamRuns` gets `exit_observed=1` and the code. Otherwise the value stays unknown. Neither a timeout, nor a missing group, nor a successful reply releases the reservation.

CLI exit is observed independently of stdout: even if a descendant holds the channel open, MCP is revoked and the unfinished turn becomes `uncertain`.

During an unfinished launch, `shutdown` responds and terminates the controller without waiting for the launch operation to return. A separate launch lock is held until it finishes: a repeated `serve` with any `--root`, `recover-failed` and the research probe are refused during that time. If the process does start later, its input is closed without sending a message; its reservation is kept. This is not a mechanism for releasing unknown processes. If Messenger itself crashes, the preliminary reservation and the owner saved in SQLite remain the protection.

A separate case is a proven OS refusal before the CLI executes (no execute permission, the file is gone, wrong format, or a path component is not a directory). Only then is Messenger's own preliminary reservation removed, and the attempt and message are saved as `not_started`. Unused channel records are deleted and the session lock is released. After fixing the path, you can explicitly enqueue a new message; the failed one is not retried automatically. Unknown launch errors and registry save errors still leave the protection in place.

These boundaries were verified on Mac with synthetic processes and the race detector after the PR #3 audit. No new real model calls were made during the fix.

`recover-failed` and `resolve accept/retry` do not recover the persistent channel. For new launches, a separate restricted command `recover-stream` was added, described below. After the earlier real trial, a third control message that was never sent to the model is kept in the queue; the new command does not release that trial's old reservation.

## Explicit recovery after a new Mac boot

This is a first, restricted version. It does not allow recovery within the same OS boot, even if the PID is gone, the group is empty or the CLI exited successfully. The boot identifier `kern.bootsessionuuid` saved before the launch must differ from the current one, and the computer fingerprint must match. Apple creates the UUID at boot: [XNU source code](https://github.com/apple-oss-distributions/xnu/blob/main/iokit/Kernel/IOPMrootDomain.cpp#L3847-L3862). The earlier `kern.boottime` is not used as evidence for this command. The computer identifier is stored as a hash, is not printed in the command output and is not sent over the network.

The command requires a stopped `serve`, the exact directory, alias and `StreamRuns` record ID, a full match between the observation in SQLite and in the registry, and at least one completed turn with a confirmed reply. After a reboot has already happened, for a matching new launch:

```sh
messenger --root /absolute/data/path recover-stream \
  --alias claude-test --run EXACT_STREAM_RUN_ID \
  --confirm preserve-uncertain
```

The run ID is taken from `inspect` before stopping; it is not the Claude conversation ID. The command itself does not reboot the computer, launch the CLI, send signals or send messages. It archives the channel records, keeps attempts, replies and uncertain outcomes, releases only the matching reservation and prepares an **exact `--resume` of the same conversation**. Old receipts are closed. No new messages are created. The queue stays paused: running `serve --claude-stream --allow-model-calls` after recovery does not by itself resume it; sending is allowed by a separate `resume`.

The recovery record first gets the `pending` state, during which `serve` and the research probe are forbidden in this directory. Then the reservation file is replaced and the recovery is completed in a transaction. On failure, repeat **the same command with the same run ID**; the repeat does not duplicate history, does not create messages and does not remove other reservations. `inspect` now includes `ArchivedStreamRuns` and `StreamRecoveries`.

Old runs without `recovery_epoch` and `stream_run_id` recorded in advance, incomplete records, no confirmed reply, a different computer, a read error and a corrupted registry all result in a refusal. Boot information is not filled in retroactively. The command is designed for local data on a single machine; moving databases between machines, cloned VMs and snapshot rollbacks are not covered by the guarantee. Launching the same ID directly outside Messenger is still outside the guarantee.

Tests with the race detector were run on Mac: the positive boot change was **simulated in the test**; there was no real reboot and no real Claude resume in this verification. The fake CLI verified `--resume` instead of creating a new session, preservation of `uncertain`, and delivery of only the next message after unpausing. Reading the real Mac identifiers was verified separately without printing their values. On Linux/Windows the identity source for this command currently returns an explicit "unsupported"; this does not prevent the client from compiling.

Later, on the same Mac, a [real reboot and positive acceptance](reports/2026-11-05-claude-stream-reboot-mac.md) were carried out: the standard recovery kept the history and the pause, then one new message went through an exact resume with preserved context and `provider_ok=1`. Old messages were not repeated. Repeating recovery of the old generation did not affect the new reservation; the new generation cannot be recovered within the same boot. The client was stopped before the reboot: a crash interrupting an active reply was not verified by this trial.

## Durability and access

- `stream_runs` binds one process owner to a session; `stream_turns` binds individual attempts to the owner.
- The intent to send is recorded before writing to stdin. If the client process crashes in between, the outcome is unknown and the message is not retried automatically.
- Moving to the next message happens in a single transaction while the owner is kept. A different alias does not give a second launch of the same session.
- `read_current` returns a random receipt for the current message. `reply(text, receipt)` checks it in SQLite. The address and reply link are assigned by code; an old receipt does not work for the next turn.
- HTTP MCP listens only on loopback, uses a separate bearer capability, rejects Origin and checks the state in the database on every call.
- `inspect` contains IDs, texts and internal tokens. The output, databases and profiles stay local and never go into Git.

This is a local implementation without networking, an installer or a window. Windows platform operations are explicitly unsupported for now. Linux/Windows cross-compilation is not the same as running tests or the CLI on those OSes.

The SQLite schema is extended when the database is opened. Going back to an old binary for a database with persistent sessions is not supported; the old client does not know the `turn_completed` states or the stream owner.
