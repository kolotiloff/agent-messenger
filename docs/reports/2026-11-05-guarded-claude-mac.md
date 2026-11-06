# Real Claude under queue control and bound MCP

Date: 2026-11-05. macOS, Python 3.9.6, Claude Code 2.1.318.

**Refined experiment PASS:** two sequential jobs in one new test CLI session, two linked replies, concurrent reservation blocked. Additionally, 15 synthetic tests passed. The production adapter and the interface are not ready yet.

## What was verified

A new test session was created with a predefined UUID. The local queue contains two messages from the start. Before each launch, the reservation and the launch intent are recorded. The `launching` state does not allow MCP to read the message or save a reply.

The controller reads Claude's structured output. After the initialization event, it checks the exact session ID and the list of available tools, then allows `read_current()` and `reply(text)`. The agent itself does not choose the recipient, the original message ID or the retry key.

While the real CLI is alive, a separate process tries to reserve the same session. In both cases it is refused. It does not launch another CLI.

The first Claude reads the message only via MCP and replies. After the main process exits, the controller checks the successful result, the exact ID, the sequence of tool calls and the saved linked reply, then completes the execution. The SQLite connection is closed and reopened. The second launch uses `--resume` with the same ID and processes the second message.

Result of the refined experiment:
- 2 real CLI launches; both completed successfully.
- The same session in both results.
- 2 execution records, 2 replies with the correct sender, recipient and link.
- Both messages are in the `completed` state; the session's busy state was cleared after the check.
- 2 concurrent reservation attempts rejected.
- No automatic retries; the waiting window does not poll the model.

## The first experiment is kept as a failure

The original test instruction asked to prepend `ACK:` to the received text. Claude added an extra space. Addressing and linking were correct, but the exact format check stopped the controller.

The session got `uncertain`, the second message stayed in the queue; the saved reply was not deleted and the job was not retried. After analyzing the log, the instruction was refined: the extra space was explicitly forbidden and an example was given. The new check was run in another new session and another database.

In total, this work involved 3 real Claude launches: 1 in the first experiment and 2 in the refined one. This is not a hidden automatic retry. Comparing the control string is a test condition; it does not mean Messenger will be able to automatically prove the correctness of an arbitrary task.

## Additional checks without models

Two tests were added: before the ID is confirmed, MCP does not read the message and does not reply; a mismatched ID does not grant access and does not release the slot. The existing 13 tests for the queue, duplicates and crash states also pass. Total: 15 tests, 1.361 seconds.

## Reproduction and scope

[Instructions and code](../../experiments/session_safety/README.md). The real experiment is run separately via `claude_probe.py` and requires an explicit `--allow-model-calls`; the regular unittest does not call models. Existing Claude authorization and a new folder outside the repository for data are required.

The test does not change global CLI settings and does not attach the coordinator's working conversation. Raw logs, conversation IDs and the database stay local and are not published.

Parts of H1/H5/H6 are confirmed on Mac for the current CLI and this mode. There is no verification of real crash recovery, the full CLI descendant hierarchy, native Windows, network or hour-long waits. The controller waits for the main process and checks the result; this is not yet a universal process-tree manager. A wrong ID was verified synthetically; deliberate substitution of real history was not performed. Codex/Cursor have not been run on the new interface yet.

Clearing `uncertain` is not implemented: an unknown state stays blocked. The new Python code is an experimental integration, not a background service or an installable application.

The parameters were checked against the installed CLI's help and the [official Claude reference](https://code.claude.com/docs/en/cli-reference).
