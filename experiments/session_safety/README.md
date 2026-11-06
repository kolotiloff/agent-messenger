
# Reply binding and session ownership check

An experimental test bench, not an installable Messenger. Python 3.9+ and the standard library; no extra packages. Verified on macOS with Python 3.9.6. Windows and Linux are not yet verified.

As of November 5, 2026 this directory remains a research source of scenarios. The persistent client and the future broker are written in Go, with no planned rewrite of a separate Python client. The product distribution does not call this bench and does not require Python to be installed. Go client boundaries are in [docs/CLIENT.md](../../docs/CLIENT.md).

From the root of the downloaded repository:

```sh
python3 -m unittest discover -s experiments/session_safety -v
```

If on Windows the interpreter is available as `py`, the expected command is `py -3 -m unittest discover -s experiments/session_safety -v`. It has not yet been confirmed to work on Windows.

Expected result: `Ran 19 tests ... OK`. The tests use a temporary folder, SQLite and ordinary Python processes that simulate the executor. Real Claude/Codex/Cursor are not launched; no network, keys or authorization are needed. The checks deliberately terminate only the test processes they created. No autostart or background services are installed.

## What changed after the Claude experiment

In reply mode the agent sees only `read_current()` and `reply(text)`. The controller binds the MCP process to a specific execution and incoming message in advance. The server itself determines the sender, recipient, `reply_to` and the deduplication key. Attempts to call `send_message` or substitute an ID/address are rejected.

One request gets one immutable reply. A repeat with the same text returns the stored reply; different text in the same context is rejected. A mode for independently sending new tasks must be a separate interface; this bench does not have one. This restriction applies to handling a specific incoming message, not to the whole future Messenger.

## How the session is held

`runtime.py` stores the queue and execution state in a single local SQLite database. The session key is built from the provider, the canonical profile and the exact conversation ID. The project folder does not create a second lock for the same ID.

The reservation is recorded atomically before the child process is created. A launch intent is written before launch; the PID is written after the process is created. The real adapter enables `launching` mode: MCP is unavailable until the exact ID is confirmed by a structured CLI event. Synthetic processes may use the earlier simplified mode. Successful completion is recorded only by trusted controller code, after waiting for its own child process to exit and checking the stored reply.

A stored reply by itself does not release the session. A failure, a missing reply or uncertainty does not lift the lock. There is no lease expiry after which the session can be automatically handed to another process. `quarantine` moves the execution to `uncertain`, blocks further writes from that context and keeps the session occupied.

## Files

- `runtime.py`: queue, reservation, bound reply, states.
- `mcp_server.py`: minimal stdio MCP for a single incoming message.
- `process_fixture.py`: artificial executor and stop points for tests.
- `test_safety.py`: 15 checks, including real competing processes and a crash of the controller process.
- `test_adapters.py`: 4 checks of CLI service events, including wrong IDs, results and tools.
- `resume_probe.py`: a separate experiment with real Codex/Cursor; requires explicit permission for model calls.

## Guarantee boundaries

All participants must use the same local database and the same canonical identifiers. This is not a distributed lock and does not protect against manually launching the CLI bypassing the client. Moving/copying conversation history between machines and simultaneous access from a GUI are not covered.

The execution token is a local binding, not a complete authorization system. The database and launch parameters are assumed to be accessible only to the trusted client; network access and file permissions are yet to be designed. Public `Store` methods, including completing an execution, are not exposed to the agent via MCP.

Clearing `uncertain` and safe manual recovery are not implemented yet: when in doubt, the bench blocks new launches. The reduced availability here is intentional. Do not delete the database to bypass the lock of a real running session.

The main test suite uses synthetic processes. Separately, the new interface was verified with real Claude on a Mac: two tasks, one session ID and blocking of a competing reservation. The first experiment was stopped because of a mismatch in the control text format; a new refined experiment passed. Codex and Cursor also passed two tasks each, with the ID preserved, bound replies and blocking of a competing reservation. There is no VPS, network ACK, reconnection, OS process manager, production CLI adapter or hour-long wait test.

[Run report](../../docs/reports/2026-11-05-session-safety-mac.md).

## Separate experiment with real Claude

It is not run by the regular unittest command. **It spends tokens** and uses the existing local Claude login. Verified only on macOS; stdout handling and process group termination in this prototype are designed for POSIX.

```sh
python3 experiments/session_safety/claude_probe.py \
  --claude /absolute/path/to/claude \
  --run-dir /absolute/private/path/outside-repository/new-run \
  --allow-model-calls
```

Specify real absolute paths. The `--run-dir` folder must be new: a repeat run in it is rejected. The script creates a separate test session, processes up to two messages and does not retry an uncertain attempt. Do not send the data to Git/Plastic. The saved test history stays with Claude; no background services are installed.

`summary.json` contains an anonymized summary; the other files may contain local identifiers and CLI details and should not be published automatically. Success confirms this scenario and does not mean readiness of process tree recovery or of a production client.

[Real experiment report](../../docs/reports/2026-11-05-guarded-claude-mac.md).

## Separate experiments with Codex and Cursor

`resume_probe.py` uses an **existing, free test CLI session**, its exact UUID and the existing authorization. Do not use work conversations or sessions open in another CLI/GUI. The regular unittest does not run these experiments.

```sh
python3 experiments/session_safety/resume_probe.py \
  --provider codex \
  --cli /absolute/path/to/codex \
  --session-id TEST_SESSION_UUID \
  --run-dir /absolute/private/path/outside-repository/new-run \
  --allow-model-calls
```

For Cursor, replace the provider with `cursor` and the CLI path with the installed `agent`. Specify real paths and UUID. The probe creates a separate working folder and does not overwrite user settings. For Cursor, the presence of a global MCP configuration or `CURSOR_CONFIG_DIR` stops the test before the CLI is launched: such a profile must first be checked separately.

Each probe makes up to two real launches, with no automatic retries. A new folder does not lift the lock on the old session: **never run the same ID simultaneously through different bench databases**. These programs do not replace the unified session registry of the future app. The prototype was verified only on a Mac.

[Codex/Cursor report](../../docs/reports/2026-11-05-guarded-codex-cursor-mac.md).
