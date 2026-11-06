# Reply binding and session ownership: local checks

Date: 2026-11-05. **PASS: 13 tests on macOS / Python 3.9.6**, final run — 1.291 seconds. Synthetic Python processes were used; there were no model calls and no agent CLI launches.

[Code and instructions](../../experiments/session_safety/README.md).

## Confirmed in the test bench

- MCP provides only reading the current message and replying with text. The recipient and the link to the original message are set programmatically. The old way of mistakenly sending via `send_message` is rejected.
- Repeating a reply after the call result was lost returns the same record. Changing the text on a repeat is rejected.
- A finished or made-up execution context cannot send a reply.
- Six separate processes request one session simultaneously: one gets the reservation, the rest get no right to launch.
- The second message stays in the queue while the first artificial executor is alive, even if the reply is already saved. After it exits, the second message is processed.
- Redelivery of an incoming message does not create a second execution; a content conflict with the same ID is rejected.
- Independent sessions can have independent owners; one reservation cannot be moved to launch again.
- Exit code 0 without a saved reply and a non-zero code with a saved reply are not marked as successful completion.
- Forcibly killing the controlling process after the reservation, after the launch intent, after creating the child but before saving the PID, and also after the child exited but before the outcome was recorded, does not allow a second launch.
- In the lost-PID case, the artificial child process really kept working after the controlling process died: this was verified by an activity counter that kept changing. A new execution was blocked both while it was running and after it exited.
- An explicit transition to `uncertain` keeps the lock and rejects new writes from the old context.

## Conclusion for the architecture

The loss of `reply_to` found in the earlier Codex–Claude experiment is fixed in the bench's new interface: the model no longer chooses the address and ID for the reply. This is not yet a check of the new interface with real CLIs.

For the local queue, an atomic reservation and persistent state in the database are sufficient. Releasing a session by timer, by a missing PID, or by a single saved reply is unsafe. Uncertainty requires investigation, not an automatic retry.

Transactions use `BEGIN IMMEDIATE`; separate processes coordinate writes through one SQLite database. The SQLite documentation describes writer serialization and the behavior of these transactions: [SQLite transactions](https://sqlite.org/lang_transaction.html). Creating and waiting for child processes is done via the standard [Python subprocess](https://docs.python.org/3/library/subprocess.html).

## What remains open

H6/H7 are partially confirmed only for an artificial local executor. H8 is partially verified at the level of local identifiers and repeated MCP calls; the network, the broker and loss of network ACKs were not verified. H5 remains partial: the new MCP must be repeated with agents. H9 with an hour-long wait was not performed.

Not covered: Windows/Linux, distributed ownership, manually bypassing the client, an open GUI session with the same ID, real CLI descendants, OS/CLI updates, power loss, database corruption, network file systems, restoring from a backup, authorization, recovery from `uncertain`.

The tests do not guarantee the absence of a parallel launch in every environment. The bench's guarantee holds for cooperating processes that use one local database and one canonical session binding. The waiting mechanism does not call models, but hourly and per-provider statistics were not measured by this short run.
