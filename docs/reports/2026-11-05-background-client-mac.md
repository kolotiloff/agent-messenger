# Background client: verification on Mac

The change was prepared on top of `7a560fd2a4c2e371789dcd23dd61727d11e4de52` (PR #4). macOS arm64, Go 1.22.2 with the external linker and local ad-hoc signing, Claude Code 2.1.318. A `start` command was prepared: a separate background process, a private readiness confirmation and a mandatory pause at startup. No services or autostart were installed.

## Why releasing the reservation when the parent exits was not added

On Mac, continuous tracking of child processes via kqueue was investigated. In the source of [Apple XNU, `filt_procattach`](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/kern_event.c), combinations with `NOTE_TRACK`, `NOTE_TRACKERR`, `NOTE_CHILD` are rejected with `ENOTSUP`. A native attempt to register `NOTE_TRACK` for its own process on the tested Mac returned errno 45 (`ENOTSUP`), without running a model. Therefore, porting the descendant-tracking recipe from other BSDs does not provide the required proof here.

[Endpoint Security](https://developer.apple.com/documentation/endpointsecurity) provides process events but requires a separate integration with the Endpoint Security entitlement. Such an integration is not implemented at this stage and is not considered a finished solution. An ordinary process list, the exit of the main PID and an empty group still do not prove that detached descendants have stopped.

The decision for this stage: keep the live session owner when the user's terminal is closed. The rules for releasing a stopped CLI were not relaxed. This enables reconnecting to a running client, not safely restarting an exited CLI within the same boot.

## Automated checks without models

The full suite passed on Mac:

```sh
go test -race -tags streamprobe -ldflags=-linkmode=external -count=1 ./...
```

The new checks cover:

- Background start without model permission, a saved message after `resume`, no attempt to launch the CLI.
- A new start with model permission on top of a non-empty previously resumed queue: the queue is paused again, no attempt is created.
- A second start with different permissions and with a different root: refused; the first client's PID/pause/permissions are not changed.
- Loss of the parent channel after the readiness message but before confirmation: the process exits, the lock is available to the next start.
- A corrupted database and unknown flags: the start is not reported as successful.
- Exit of a separate `start` command: its output channels are closed, and the client stays available for subsequent connections.

`CGO_ENABLED=0` builds for linux/amd64 and windows/amd64 succeeded. Tests and real CLIs were not run on those OSes; background launch on Windows is explicitly unsupported. Warnings from the old Mac linker about LC_DYSYMTAB were present; there were no build or test errors.

## Real Claude after closing the terminal

A new separate test CLI session was created via `create-claude-session`. Previous sessions and their reservations were not released. The existing test database was used, since a different root must not bypass the shared protection. Before delivery, it was verified that the old pending messages belong to occupied sessions.

`start --claude-stream --allow-model-calls` was launched from a real test PTY. The command exited with code 0 and the PTY was closed. The background client stayed available and was paused.

The first new message carried a private marker and received `ACK:BACKGROUND-one`, linked to it and with `provider_ok=1`. The queue was then paused. Eight seconds without an open control connection created no new attempts, replies or channel changes. This is a short state check, not a measurement of provider billing.

A repeated `start` was refused; the active client did not change. From a new connection, a second message was sent asking to recall the marker. The reply `ACK:BACKGROUND-two|` with the correct earlier marker is linked to the second message, `provider_ok=1`.

Both messages had the same Claude PID, background client PID and stream run ID. In total, two user messages, two attempts and two replies were created; there were no automatic retries. Each message may involve several model/tool turns, not a single API call.

The previous history and all previous reservations remained unchanged. After the check, a pause and `shutdown` were performed, followed by a check via a launch without model permission and another `shutdown`. The new channel remained `uncertain`, `exit_observed=0`, `exit_code=null`; the reservation was kept. A process exit with code 0 is not claimed.

## Scope

Accepted: closing the terminal and reconnecting to the same live local client on Mac, with sequential delivery of two messages to one Claude test session.

Not accepted: releasing the reservation after the CLI has fully exited without a reboot, recovery after a background client crash, persistent Codex/Cursor operation, Windows/Linux in operation, a network broker, the interface, a service, an installer and automatic restart. The background client is not required to survive the user logging out of macOS or a reboot.

Private IDs, the marker, profiles, the database, backups and CLI logs are not included in Git. The test processes were stopped; autostart is not installed.
