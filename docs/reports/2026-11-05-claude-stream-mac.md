# Claude streaming: two messages in one process on Mac

Base: `09d2e3d`; a separate bench `experiments/claude_stream`, enabled by the `streamprobe` tag. Claude Code 2.1.318, macOS arm64, temporary Go 1.22.2. The product `serve` is unchanged.

## Real runs

| Check | First run | Fixed run |
|---|---|---|
| CLI launches | 1 | 1 |
| Input messages | 2 | 2 |
| Verified successful results | 1 | 2 |
| Bound replies | `ACK:PAYLOAD-one` | `ACK:PAYLOAD-one`, `ACK:PAYLOAD-two` |
| Wait with no new model events | 20 seconds | 40 seconds: two pauses of 20 |
| Same process for both verified turns | not counted | confirmed |
| Observed CLI exit | 0 | 0 |
| Automatic retries | 0 | 0 |

The first run was stopped because of a bench bug: a repeated `system/init` before the second turn was wrongly treated as a new session. Its second reply was not counted. After the fix and a regression test, a separate run was done in a new test session. In total, four input messages were sent across two runs; this is not the number of internal model API requests.

In the second run, for both turns the exact session ID, the MCP sequence `read_current → reply`, a successful provider result without permission denials, the receipt and the reply binding were verified. Between turns the process was checked by PID, group, start time and boot ID. After completion, no process with the test ID on its command line was found. This is an additional observation, not proof that no detached descendants remain.

Both bench reservations and the earlier reservation from the successful one-shot probe were preserved. The previous session's history was not deleted. Work/GUI chats, Codex and Cursor were not launched.

## Checks without a model

The full `go test -race -tags streamprobe -ldflags=-linkmode=external -count=1 ./...` passed on Mac. New tests cover MCP being closed until the ID is confirmed, a stale receipt on the next turn, conflicting parallel replies and an idempotent duplicate, a log write error, a wrong/missing reply, HTTP authorization, activity/EOF during the wait, and a repeated init with the correct/wrong ID.

A check of a separate `serve` alongside the first run was not counted: it started only after the bench had finished and timed out; the child process was terminated. Its model was disabled. This run is not proof of competing-lock blocking. The bench uses the existing shared user lock, which is covered by product tests.

## What was proven and what remains

A persistent Claude can receive two consecutive messages via stdin, using one process and one session. During the two observed pauses, the bench sent no requests and received no new model events. Billing was not measured; this experiment does not prove the claim that "any idle time is guaranteed to be free".

This is a research check of transport and addressing. The two messages are in the bench's separate log; the product SQLite stores the process owner. The queue, pause/resume, crash recovery and termination of the persistent process still need to be wired into the regular `serve`, separating the state of the process from that of an individual message. There is no automatic release of the session on an empty group here.

UUIDs, receipts, raw logs and private directories are not included in this report.

After the real run, error reporting for the final SQLite write was strengthened: such an error also clears the counted flag. The full test suite was rerun; no additional model call was made for this error handling.
