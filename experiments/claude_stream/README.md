# Persistent Claude: a limited Go probe

This test rig verifies two sequential messages in a single Claude process. It does not change `messenger serve` and is excluded from the regular build by the `streamprobe` tag.

Base: `09d2e3d`. It verifies a new, separate test session, not a continuation of a work or GUI chat. The UUID is created by the rig itself; passing an existing ID is not possible. The existing authorization in the regular Claude profile is kept.

## What is verified

- One CLI launch with `--input-format stream-json`, two input messages, the same PID, start time, boot ID and session ID.
- For each message: `read_current`, then `reply`; one successful provider result and the corresponding reply.
- The second message is sent only after the first reply has been checked and a 20-second wait. After the second reply there is another 20-second wait.
- While waiting, the rig writes nothing to the CLI's stdin and checks that there are no new model events. This is a limited protocol observation, not a billing measurement or a universal guarantee that the provider makes no background requests.
- In the verified version, Claude sends `system/init` before every turn. A repeated init is allowed only for the same ID and after checking that it is the same process.
- Closing stdin ends the probe. Process exit does not release the protective reservation and is not taken as proof that all its descendants have stopped.

## Running

The tests do not call models:

```sh
go test -race -tags streamprobe -count=1 ./...
go build -tags streamprobe -o /tmp/agent-messenger-stream-probe ./experiments/claude_stream
```

On the verified Mac with a temporary Go 1.22.2, `-ldflags=-linkmode=external` and a local ad-hoc signature of the built file were required. This is a quirk of the build environment used, not a change to system security settings.

A real probe requires explicit permission and may spend tokens on two messages:

```sh
/tmp/agent-messenger-stream-probe \
  --root /absolute/path/to/existing-messenger-data \
  --cli /absolute/path/to/claude \
  --private /absolute/path/to/new-private-directory \
  --allow-model-calls
```

`serve` must be stopped. If there is an old global reservation, the same data directory is used. The rig does not delete old reservations and takes the shared per-user lock. The new session also gets a durable reservation before the CLI is launched, and an owner lock in the product SQLite. The regular `serve` does not get the CLI path for this experimental session. Direct CLI launches that bypass the client remain outside the guarantee.

## Data and limitations

The private directory stores `binding.json`, the process observation, `turns.json`, provider events, stdout/stderr and the final `result.json`. These files contain the session ID and access receipts; they must not be committed. Only the source code and an anonymized report go to GitHub.

`turns.json` is a separate experimental log of the two messages. The product queue keeps a single owner record for the probe. Therefore a successful probe **does not mean** that the regular `messenger serve` can already serve two messages in one session.

MCP runs only on a random loopback port with a bearer capability. The tools are closed until the ID is confirmed. `read_current` issues a random receipt for the current message. `reply` accepts only the text and this receipt; the address and `reply_to` are assigned by code. An old receipt does not work for the next message. Between turns and after stopping, the tools are closed. Every accepted reply also leaves a mark in the product owner record so that `recover-failed` does not mistake the probe for a run without replies.

A timeout, a result error or an ID mismatch stops the probe without an automatic retry. On shutdown stdin is closed; after waiting, a signal may be sent only to a process with a matching observation. The reservation and history remain even after exit 0. Receipts separate replies, but do not prove that an arbitrary CLI or its descendants have stopped file operations.

## Next step: integration into the product

1. Separate the process owner (`run`) from message processing (`turn`). A successful turn does not release the run.
2. Store separate states for the message and the process. A turn's `provider_ok` does not stand in for the process's `exit_observed`.
3. Record the intent to send before writing to stdin. After a crash, do not resend an unfinished turn automatically.
4. Move receipts, reply binding and closing MCP between turns into the product SQLite, with checks of old capabilities after stop/restart.
5. On pause, stop new turns; on stop/shutdown, revoke access; do not launch a second CLI for this session until the previous one is proven stopped.
6. Verify two messages, pause/resume, stopping during a reply, a client crash and relaunch, this time through the regular `serve`. Then separately investigate persistent channels for Codex and Cursor: the Claude results do not apply to them.

## Why we do not release the lock on an empty group

An empty process group does not rule out a detached descendant. The [Apple kernel header](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/event.h) states explicitly that `NOTE_TRACK`, `NOTE_TRACKERR` and `NOTE_CHILD` have not been supported since macOS 10.5. The fork-tracking guarantee from FreeBSD cannot be carried over to macOS.

[Anthropic's documentation](https://code.claude.com/docs/en/agent-sdk/streaming-vs-single-mode) describes a persistent process with incoming messages and sequential turns. The [official SDK transport](https://github.com/anthropics/claude-agent-sdk-python/blob/main/src/claude_agent_sdk/_internal/transport/subprocess_cli.py) and the local help of the installed CLI confirm streaming input. Here the CLI is used directly from Go; the SDK is not installed.
