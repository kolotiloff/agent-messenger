# Persistent Claude through regular serve: Mac

Base: `3994901` (PR #2 test bench). This time `cmd/messenger` itself was tested, without the `streamprobe` tag, with the new explicit mode `serve --claude-stream`. Claude Code 2.1.318, macOS arm64, Go 1.22.2.

## Real probe

The whole scenario passed in one run, without automatic retries and without signing in to Claude again:

1. `serve --claude-stream` was started without model permission. A new separate session was created via `create-claude-session`, and the first message was enqueued. No CLI attempts appeared.
2. A second `serve`, while the first was alive, was refused due to the user lock.
3. After a restart with `--allow-model-calls`, the first message received a linked `ACK:PAYLOAD-one`, `provider_ok=1`.
4. `pause` was run and a second message was enqueued. For 20 seconds it remained `queued`, the attempt count stayed at 1, and the process owner was `idle`.
5. After `resume`, a linked `ACK:PAYLOAD-two`, `provider_ok=1` was received. The PID of both attempts is the same; there was one process, and the session ID did not change.
6. While paused, a third control message was enqueued and `shutdown` was run. After a new `serve --claude-stream --allow-model-calls` and `resume`, it remained in the queue: still 2 attempts, 2 replies, owner `uncertain`.
7. The test Messenger was stopped. In a subsequent process check, the test CLI was not found by its exact ID. All protective reservations were preserved, including reservations from earlier experiments.

Two messages actually called the model. The third was only enqueued to check preservation and was not run. Codex, Cursor and working/GUI chats were not used.

In `Verifications`, both turns have `provider_ok=1`, `exit_observed=0`, `exit_code=null`. This is the correct separation between a successful reply and the exit of a persistent process. On the fast shutdown, the client did not manage to record the CLI exit code: `StreamRuns` also has `exit_observed=0`, `exit_code=null`. Therefore exit 0 is **not attributed** to this run, and the reservation is not released. A process search after the test is not proof that all detached descendants have stopped.

## Tests without a model

The full suite passed on Mac:

```sh
go test -race -tags streamprobe -ldflags=-linkmode=external -count=1 ./...
```

Product tests and the earlier research tests were run. New scenarios: two messages/one PID; launch without model permission; pause before launch and between messages; an old receipt; an erroneous/duplicate init; an erroneous provider result; HTTP authorization and the ban on choosing an address; stop; shutdown with a hung fake CLI; launch with a different database; restart without repetition; forced termination of the fake Messenger itself with a live CLI, followed by restart and stop-job.

The first test run revealed a race in the hung-CLI test scenario: shutdown ran before the fixture had entered the hung state. The fixture was fixed and the suite was rerun. The real model was not run during these tests.

## Open limitations

- Resuming the persistent process after a shutdown/crash is blocked; safe release of the reservation is not yet implemented.
- A real `--resume` of an existing free session in the new persistent mode was not tested separately; here the session was new.
- For now, one persistent session is served at a time, without switching between several owners.
- The pause check is limited, billing was not measured, and the one-hour H9 is not closed.
- Cross-builds with `CGO_ENABLED=0` for Linux amd64 and Windows amd64 succeeded; this code was not run on Windows/Linux. There is no interface, installer or network broker.

Raw logs, UUIDs, receipts, the local database and directories are not included in the report.

After the real probe, only the advertised name of the HTTP MCP server was changed from the research name to `agent-messenger` (version `0.2`), with no change to the transport or tools. The full test suite was rerun; the model was not called again.
