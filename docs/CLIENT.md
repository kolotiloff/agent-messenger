# Local client: decision and work plan

The original decision was recorded on November 5, 2026. Current instructions for launching the window, installation and checks are in [LOCAL_UI.md](LOCAL_UI.md), [INSTALL.md](INSTALL.md) and [CI.md](CI.md). The history of the first one-shot client is also kept below; its old limitations should not be carried over to every later version.

## Former addendum: persistent Claude

> Removed in T-028 ([D-104](DECISIONS.md)); kept as a record.

`serve --claude-stream` serves Claude messages one at a time in a single process. The model still requires a separate `--allow-model-calls`. Two messages, pause/resume and the queue surviving shutdown/restart were verified on a Mac. Turn state and process-owner state are separate; a successful turn does not release the reservation. [Instructions and limitations](CLAUDE_STREAM.md), [report](reports/2026-11-05-claude-stream-product-mac.md).

The original decision and the history of the one-shot mode are kept below. Statements that a second message does not start apply to that mode, not to the new explicitly enabled mode.

## Decision

The persistent local client and the future broker are written in Go. Python remains the research test bench `experiments/session_safety` and the source of already verified scenarios. No separate rewrite of a finished client from Python to Go is planned: the product code is in Go from the start.

The unfinished Python client is kept in `experiments/python_client_draft` as a research draft. The product client is Go. The test bench is not deleted and is not retroactively declared obsolete. Historical reports in `docs/reports/` keep the wording of their day, including the early "13 tests" and "Codex/Cursor have not been run yet". Later results are in [VALIDATION.md](VALIDATION.md), the [Claude report](reports/2026-11-05-guarded-claude-mac.md) and the [Codex/Cursor report](reports/2026-11-05-guarded-codex-cursor-mac.md).

## History of the first stage (November 5)

One local client per user, with no window. A future GUI only sends it commands and does not launch CLIs itself. No network broker, installer, autostart, LaunchAgent or VPS wizard. This delivery does not enable launching real Claude, Codex and Cursor: model verification remains a separate step on a Mac.

Shared core: queue, session registry, attempt states, deduplication, log. Platform operations are separate: a single instance of the controller process, local commands, processes, permissions and the data directory.

## Storage

SQLite via [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) v1.34.5 (a BSD-licensed package; SQLite itself is in the public domain). It is a translation of SQLite to Go, with no cgo and no C compiler.

Why not the cgo driver `mattn/go-sqlite3`, even though cgo itself is not a mistake: on macOS, Windows and Linux, cgo requires a C toolchain for every target, breaks ordinary cross-compilation and ties the build to a compiler. The cost of modernc is a heavier build and a larger binary. For three OSes without a mandatory C compiler, that is an acceptable cost. The driver does not become part of the message protocol.

The data directory is created with `0700` permissions, the database and the command socket with `0600`. Defaults:

| OS | Directory |
|---|---|
| macOS | `~/Library/Application Support/AgentMessenger` |
| Linux | `$XDG_DATA_HOME/agent-messenger` or `~/.local/share/agent-messenger` |
| Windows | `%AppData%\AgentMessenger`, if the directory is known |

Tests pass their own temporary `--root` and do not write to these directories.

## Work plan

1. Record this decision in the documentation.
2. Port the meaning of the test bench checks to the Go core and cover commands, aliases, MCP revocation, restart and launch boundaries with test processes, without models.
3. Build the binary for linux, darwin and windows. A build, `go test` on a specific OS and a real CLI run are three different levels. An unconfirmed level is called exactly that.
4. A platform operation that has no implementation must end with an explicit unsupported error, not a successful response.
5. Real CLIs only later, on a Mac, in separate test sessions.

## Guarantees the code must preserve

1. One store per user. A second controller process does not get the file lock. `BEGIN IMMEDIATE` in SQLite additionally prevents two connections from reserving the same session.
2. Session key: provider, canonical path of an existing CLI environment directory, exact conversation ID. A mail alias is only a name for this key. Two aliases or two spellings of the same path do not open a second launch.
3. Adapter arguments contain only an explicit ID. There is no "last session" mode. MCP reads the message and writes the reply only after the ID is confirmed.
4. The reply route and the retry key are set by code. Retrying the same text returns the stored reply. Different text with the same context is rejected.
5. An unknown outcome stays `uncertain`. There is no automatic retry, no timer-based lease, and no success based only on the exit code or only on the reply text.
6. A new launch is forbidden until the old execution is recognized as stopped. A single PID and cancelling `CommandContext` do not count as stopping descendants.
7. After a job is stopped or moved to `uncertain`, the old MCP context loses read and write access.
8. `fail` closes the session to new attempts and keeps the history. `retry` is a separate attempt and only together with the phrase `actions-may-repeat`, after proof of stopping. Until the platform can prove that all descendants have stopped, `retry` and `accept` remain blocked. An empty process group does not complete the attempt and does not release the session lock.
9. Three different commands: pause new jobs, stop the active job, shut down the client.
10. Waiting and status polling do not call the model and do not call the adapter. Working GUI chats are not connected.

## One-shot mode commands

For the persistent Claude process, the semantics of stopping and revoking the current MCP are refined in [CLAUDE_STREAM.md](CLAUDE_STREAM.md).

| Command | Queue | Active process | MCP of the current attempt |
|---|---|---|---|
| `pause` | stays; new launches are not reserved | is not signaled and is not marked finished | stays valid while the attempt is `running` |
| `resume` | free sessions can be reserved again | unchanged | unchanged |
| `stop-job` | other messages stay | the signal is sent only if pid, start time, boot id and group are still the same process; the group number is not used without this check | MCP is revoked before the signal; state is `uncertain`; the slot stays occupied |
| `shutdown` | kept on disk | is not killed and is not marked successful | not revoked by this command |
| `resolve fail` | new messages to this session are rejected | is not declared stopped | revoked; the slot is not given to a new attempt |
| `resolve accept` | does not create a new attempt | requires separate proof of stopping and verification of the result, not the reply text itself | in this version the command returns a refusal: proof for descendants is not implemented |
| `resolve retry` | on success would create a new message referencing the old one | the same proof of stopping and an explicit phrase about possible repetition of actions | in this version the command returns a refusal |

`serve` holds one lock per user, not per `--root` directory. A second `serve` with a different data directory does not start. Other commands go to this process's socket. `--root` is given before the command: `messenger --root DIR serve`.

`recover-failed --confirm actions-may-repeat` is run when `serve` is not running. The command does not call the model. It releases the reservation only if every record has a complete observation, the recorded process is no longer the same one, the group is empty, and the database has neither a reply nor `provider_ok=1`. The old attempt stays `uncertain`. A new message with a `retry_of` reference appears in the same directory. The log states explicitly that actions may repeat and that descendants outside the group are not proven. An incomplete, live or corrupted record and an observation error do not rewrite the file. If the database has already been updated but replacing the file failed, repeating the same command completes the registry and does not create a second message.

After `shutdown`, the queue stays. The next `serve` immediately revokes MCP for attempts it did not launch itself and does not release their lock. Session registration is `register-session` on an already running client: provider, environment directory, exact ID, alias and CLI path. After that, the queue may launch exactly this executable. An empty process group after it exits does not complete the attempt.

The unfinished Python draft is kept in [experiments/python_client_draft](https://github.com/kolotiloff/agent-messenger/tree/41e0e6e7e3f20275d480f6b064a881e7b5f4cf9e/experiments/python_client_draft/README.md). It is not a product and not a step that still has to be rewritten in Go. The test bench's executable scenarios are still in `experiments/session_safety`.

## Confirmation levels of the first stage

They must not be mixed:

- a `GOOS=darwin` / `GOOS=windows` build is compilation, not verification on that OS;
- `go test` counts only on the OS where the command actually ran;
- a real CLI is a separate check on a Mac. On this Go client one Claude message was accepted; Codex and Cursor were not run with it.

## Historical verification of the first stage

Done in `cmd/messenger` and `internal/`: the queue and registry core, one controller-process lock per user, local commands, `register-session`, the reply MCP and launching the bound CLI through the Claude, Codex and Cursor adapters. A normal process exit does not release the session lock. The old MCP keeps no rights after a new `serve`. `stop-job` revokes MCP first and does not signal someone else's process group. A separate profile directory is passed as `CLAUDE_CONFIG_DIR`, `CODEX_HOME` or `CURSOR_CONFIG_DIR`. The standard `~/.claude`, `~/.codex` or `~/.cursor` does not get this variable and does not inherit someone else's value, so that the normal login stays visible. `shutdown` does not wait for a hung process and does not hold the lock. An erroneous provider result cannot be accepted through `resolve accept`. The shared execution reservation is written atomically and flushed to disk before the process is launched. An empty or corrupted `active-runs.json` blocks launching and is not treated as an empty list. The reservation is not removed if the original PID has disappeared or observation failed. While the reservation exists, `serve` with a different `--root` is rejected, including when only a descendant is alive. A failed attempt with no reply and no successful provider result is released only by `recover-failed --confirm actions-may-repeat`, and only if the recorded process and its group are no longer running. History is kept, and the model is not launched. A successful result is not accepted by this command.

Verified in this Linux environment, Go 1.22.2:

- `go test -race -count=1 ./...` — the `adapters`, `core`, `mcp`, `platform` and `supervisor` packages passed;
- a rerun of `python3 -m unittest discover -s experiments/session_safety` — the existing 19 test bench tests pass. This does not replace the reports made on a Mac;
- with `CGO_ENABLED=0`, the build and test compilation passed for `darwin/arm64`, `darwin/amd64` and `windows/amd64`. The tests themselves were not run on those OSes.

On a Mac, for commit `47ad026`, the user confirmed `go test -race -count=1 ./...` and one Claude message. `recover-failed --confirm actions-may-repeat` kept the failed attempt `m1` and queued exactly one new message. Claude replied `ACK:PAYLOAD-one`; the reply is linked to this new message. `provider_ok=1`, exit code `0`. There is one new attempt, with no automatic retries. After Messenger was stopped and the test Claude process finished, the protective reservation was kept and the attempt stays `uncertain`: an empty process group still does not release the lock and does not launch the next message. Codex and Cursor were not run with this client. The session ID and CLI logs were not added to the repository.

Not verified and not considered ready: `go test` on Windows, Codex and Cursor on this Go client, a second message in the same session, the window, the installer, autostart, the network broker.

On Windows, the single-instance lock, the process snapshot, the new group and the group signal return an explicit unsupported error. On macOS, the snapshot code via `ps` and `sysctl` compiles but was not executed here. Comparing `Profile` and `profile` on a case-insensitive volume is covered by a substitution in a test; APFS itself was not mounted in this environment. `resolve accept` and `resolve retry` stay blocked until the snapshot proves stopping. An empty group is not such proof. A boot id change in a pure function means that processes from the previous boot are gone; there was no real reboot in this environment.

The control loop runs one attempt at a time. The database lock, however, is separate for each real session. Launching the CLI directly, bypassing the client, is still outside the guarantee.

## Verification on a Mac without models

Use a supported stable Go version. Do not use real models, working chats or GUI sessions. Do not install a service or autostart. If a client or a protective reservation already exists, do not create another database to get around it; use the documented recovery.

```sh
git clone https://github.com/kolotiloff/agent-messenger.git
# Choose a verified branch/revision: the preview changes are still in a draft PR.
cd agent-messenger
bash scripts/check.sh
go build -o /tmp/agent-messenger ./cmd/messenger
ROOT="$(mktemp -d /tmp/agent-messenger-check.XXXXXX)"
printf 'Check directory: %s\n' "$ROOT"
/tmp/agent-messenger --root "$ROOT" serve
```

In the second terminal, first set the same path: variables from the first terminal are not passed there automatically. `--root` goes before the command:

```sh
ROOT="PATH_FROM_FIRST_TERMINAL"
/tmp/agent-messenger --root "$ROOT" status
/tmp/agent-messenger --root "$ROOT" pause
/tmp/agent-messenger --root "$ROOT" status
/tmp/agent-messenger --root "$ROOT" resume
/tmp/agent-messenger --root "$ROOT" shutdown
```

Expected: tests pass; `status` shows the client with no calls to Claude, Codex or Cursor; pause does not delete the queue; `shutdown` ends the `serve` process, and the `ROOT` directory stays. Running `serve` again in the same directory brings back the saved state. A second `serve` while the first is alive exits with a lock refusal.

On the original Go 1.22.2/macOS, local runs required `CGO_ENABLED=1`, `-ldflags=-linkmode=external` and a local ad-hoc signature of the self-built binary. The compile command by itself does not prove that macOS will run the file; check `version`. For an archive, use the [preview builder](INSTALL.md), which signs its own output and verifies that it launches. Commands to disable Gatekeeper or remove quarantine are not required and are not suggested here.

This check does not confirm resume of real CLIs. With a normal `serve`, `status` shows `cli_launch=not-authorized`: messages stay in the queue and the model is not called.

## One test CLI session on a Mac

This is a separate level. It spends tokens. It needs an already existing free test session, not a working conversation and not a GUI chat. Do not put the ID, logs or `summary` into git. The `--root` directory must be new, outside the repository and working directories.

A normal `serve` does not launch models. For a single probe, `--allow-model-calls` is added to it. After the first message the session stays occupied: an empty process group does not allow a second message by itself.

```sh
ROOT="$(mktemp -d /tmp/agent-messenger-cli.XXXXXX)"
/tmp/agent-messenger --root "$ROOT" serve --allow-model-calls
```

In the second terminal, substituting the same ROOT, your own paths and a test UUID (the working directory must already exist):

```sh
ROOT="PATH_FROM_FIRST_TERMINAL"
/tmp/agent-messenger --root "$ROOT" register-session \
  --provider claude \
  --environment "$HOME/.claude" \
  --conversation-id 'TEST_SESSION_UUID' \
  --alias claude-test \
  --cli /absolute/path/to/claude \
  --workspace /tmp/agent-messenger-claude-workspace
/tmp/agent-messenger --root "$ROOT" enqueue \
  --alias claude-test --id m1 --from test-coordinator --text 'PAYLOAD-one'
/tmp/agent-messenger --root "$ROOT" status
```

For regular Claude, `--environment` stays `$HOME/.claude`, but the process does not get `CLAUDE_CONFIG_DIR`: this preserves the existing login. There is no need to sign in again. A separate directory other than the standard profile is still passed via the variable. For Codex, the standard directory is `$HOME/.codex`. For Cursor, specify `--workspace` and do not run if `~/.cursor/mcp.json` exists or someone else's `CURSOR_CONFIG_DIR` is set in the environment. Success for this probe: one launch, a linked reply and `provider_ok` equal to 1. `held=1` alone is not enough. A second message to the same session does not start by itself. Then `shutdown`. Codex and Cursor are separate probes after a successful Claude run.

If a previous attempt remained in the shared reservation, a new `serve` with a different directory refuses to start. Do not delete the reservation file manually. First, with no `serve` running and no model:

```sh
/tmp/agent-messenger recover-failed --confirm actions-may-repeat
```

The command prints the old attempt's directory and the id of the new message. `m1` stays a failed attempt. The retry is a new message in the same directory. Then run `serve --allow-model-calls` with exactly the printed directory and do not enqueue the message again: it is already queued. A new `mktemp` after recovery does not send this retry. If the command refused, the reservation is in place; deleting the file bypasses the live-process check.

This probe was accepted on commit `47ad026`: a linked reply `ACK:PAYLOAD-one` to the new message, `provider_ok=1`, exit code `0`, one new attempt. A successful result does not release the reservation. The next message in the same session does not start by itself.
