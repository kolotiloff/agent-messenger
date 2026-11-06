# Resuming a saved Codex thread with a new owner (T-009)

2026-11-06, cloud session on Linux (x86-64), Go 1.24.7, Codex CLI 0.155.1 installed from npm into a scratch directory. No real model was called. The Mac part of T-009 is still to be run ([runbook](../ACCEPTANCE_CODEX_RESUME_MAC.md)).

## What was built

- `recover-codex --resume`: after a proven reboot the setup becomes `resumable` instead of `retired`. The session stays open, the old owner's attempt stays uncertain, and the queue is paused. Refused for a thread with no verified turn.
- `resume-codex-session --alias NAME [--cli FILE]`: a new app-server for the same thread. The reservation is written before exec, the config is checked, the `resume_requested` intent is committed, and `thread/resume` is sent once. Only the exact stored thread is accepted, and it must be idle.
- Schema version 2 (`recovery_mode` column; new states).

Details: [CODEX_ADAPTER_NEXT.md](../CODEX_ADAPTER_NEXT.md#resuming-a-saved-thread-t-009).

## Probes on the real Codex 0.155.1 app-server, with a fake model

The managed profile's `config.toml` pointed Codex at a local HTTP server that imitates the Responses API. It records each request and answers with fixed items. It is not a model. Empty `HOME` and `CODEX_HOME`, no login, and no working chats were used. The thread IDs stayed local.

| Check | Result |
|---|---|
| `thread/start` with the `messenger_bound` tools, then one turn | Turn `completed`; a rollout file appeared under the profile's `sessions/` |
| `thread/resume` of that thread in a **new** process | Same ID, `idle`, read-only, approval `never`, earlier turns present |
| A turn after the resume | The request to the model again offered `messenger_bound` with `read_current` and `reply`, plus the earlier history |
| `thread/resume` alone, then closing stdin | No request reached the fake model |
| `thread/resume` of an unknown ID | Error `-32600`, "no rollout found" |
| Process killed in the middle of a turn, then resumed | Thread `idle`; the cut-off turn is reported as `interrupted` |
| `excludeTurns: true` | Same fields, `turns` empty, about 2 KB response |
| Item format of a namespaced tool call from the model | `{"type":"function_call","namespace":"messenger_bound","name":"read_current",...}` reaches the client as `item/tool/call` |

## Product path against the installed Codex

`TestInstalledCodexResumeWithFakeModel` (opt-in, `MESSENGER_CODEX_PROBE=<codex>`) drives the real client code against the installed Codex and a fake model that calls `read_current` and then `reply`:

1. `create-codex-session`, then message `m1` is answered `ACK:one` with a verified turn.
2. The client stops. On the next start the owner is uncertain, and the client stops again.
3. With a simulated new boot, `recover-codex --resume` removes the reservation.
4. `resume-codex-session` starts a second app-server with a new PID. The setup is `created` again with the same thread ID.
5. Message `m2` is answered `ACK:two` in the resumed thread. Over the whole run the fake model got 2 prompts and 6 requests, and every request offered the Messenger tools.

It passed, also with `-race -count=3`. `TestInstalledCodexCreateSession` passed as well.

## Tests with the fake app-server (part of `scripts/check.sh`)

- Resume continues the same thread with a new process and reservation. Resuming starts no turn and does not open a paused queue. Messages queued while resumable are answered after `resume`. Two turns run in the new owner, and a second resume is refused.
- A message lost with the old owner stays `uncertain`, keeps its one reply, and is not resent. Later messages are answered.
- `--resume` is refused for a thread without a verified turn; the reservation stays. The close mode still works, and a closed session cannot be resumed.
- A half-done recovery cannot be finished in the other mode. Startup is refused while it is pending.
- Each failure is uncertain, with the reservation kept and no retry: unknown thread, different thread, active thread, no response (timeout) and rejected config. `thread/start` is never sent instead.
- Refusals: no model permission, an alias outside `--only-session`, or a registry entry for the same thread. In all of them nothing is launched. A definite exec refusal returns the setup to `resumable`.
- `CodexRPC.ResumeThread` writes only `thread/resume` for a valid ID and rejects a different thread. The general setup allowlist still refuses `thread/resume`.
- A schema-1 database is upgraded to 2.

## Checks

- `bash scripts/check.sh`: gofmt, `go mod verify`, `go vet`, race tests: passed.
- `bash scripts/check-builds.sh`: all targets compiled.

## Not verified

- **On the Mac with a real model.** Recovery and resume after a real reboot, then a real Codex turn in the resumed thread. The boot proof (`ProvePreviousBoot`) exists only on macOS, so on Linux the reboot was simulated in tests.
- **The window.** It has no resume action yet; resuming is a CLI step.
- **Releasing an owner without a reboot.** Resume still needs the reboot proof (T-010).
- **Codex ↔ Claude with real models.**
