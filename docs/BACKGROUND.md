# Background client and reconnecting

Normally the window (the Mac app or `messenger open`) starts the background client by itself; `start` is the command-line way. `start` launches the regular local Messenger in the background. The command exits; you can close the terminal. Subsequent `status`, `inspect`, `pause`, `resume` and `enqueue` connect to the same process through a local socket. No service is installed, and there is no autostart after login or reboot and no automatic restart.

It works on macOS, Linux and Windows (D-043; on Windows with `LockFileEx` and job objects). CI starts and stops the background client on all three.

## Running without models

```sh
messenger --root /absolute/data/path start --claude-stream
messenger --root /absolute/data/path status
```

The data directory must be persistent; for regular use, do not put it in `/tmp`. Without `--root`, the user's AgentMessenger directory is used. If an earlier trial already created protective reservations, you must use its previous root: a new directory does not bypass the locks.

Without `--allow-model-calls`, no CLIs are launched, even after `resume`. The command prints the background client's PID and the path of a separate log with `0600` permissions. Model permission applies only to this process launch; it is not saved in settings and is not inherited by a new `start`.

## Running with message delivery allowed

Only for a separate, registered test CLI session:

```sh
messenger --root /absolute/data/path start --claude-stream --allow-model-calls
messenger --root /absolute/data/path status
# Check the relevant queue and sessions before allowing delivery:
messenger --root /absolute/data/path resume
```

**Every successful `start` pauses the queue**, including when models are explicitly allowed. This prevents stored messages from being processed before the user has checked them. After `resume`, the normal queue mechanism runs; between messages Messenger sends no keep-alive requests to the model.

`start` does not switch the mode of an already running client. A second instance exits with an error, does not change the pause/permissions of the existing process and does not launch a second CLI. Use `status` first. Changing the permission requires shutting down and relaunching the client, with the same session recovery limitations as before.

## Closing the terminal and stopping the client

- Closing the terminal: the background client and the Claude it owns keep running. You can open another terminal and run `status`; no new CLI is created.
- `pause`: subsequent messages stay in the queue; the current reply may finish. The process and the lock are kept.
- `shutdown`: terminates Messenger itself. It keeps the protective reservations; the next `start` does not release the stopped CLI session. This is a separate operation from closing the terminal.
- Recovery after a confirmed Mac reboot is still the explicit `recover-stream`. Background mode does not change that command's conditions.

So closing the user's connection does not require a reboot. **Fully terminating the CLI and safely releasing its reservation within the same OS boot are not implemented by this change.** The window connects to this same client and does not call `shutdown` when it closes.

## Launch confirmation

The parent and the new process use a separate inherited channel, not a shared PID file. First the new process acquires the global lock, checks reservations and any unfinished recovery, opens the database, pauses it and creates the control socket. Only then does it confirm readiness. The parent verifies the PID of its own child process specifically and confirms the launch; the delivery loop starts after this exchange.

If the parent is lost before confirmation, the launch stops before the delivery loop. A timeout does not release the session lock. A stale control socket or another instance that is already running does not count as launch confirmation. The internal `_background-serve` command requires the inherited channel and is not intended for manual use.

Logs from different launches are not overwritten. Cleaning and rotating them is still manual; history is never deleted automatically.

Native verification and limits: [Mac report](reports/2026-11-05-background-client-mac.md).
