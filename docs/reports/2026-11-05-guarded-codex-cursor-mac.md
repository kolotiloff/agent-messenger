# Codex and Cursor under queue control and a bound MCP

Date: 2026-11-05. macOS / Python 3.9.6.

**PASS for both CLIs:** two sequential jobs each in a saved test session, correct linked replies and refusal of a competing reservation. In addition, 19 tests without models passed.

| CLI | Version | Jobs | ID preserved | Linked replies | Competing attempts |
|---|---|---|---|---|---|
| Codex | 0.155.1 | 2 successful | Same ID | 2 | Both rejected |
| Cursor Agent CLI | 2026.11.05-86047df | 2 successful | Same ID | 2 | Both rejected |

## Scenario

Separate idle test conversations from the earlier resume experiments were used. No work chats were used. For each provider, a new private local data folder and a separate queue were created; within each experiment, all competing processes used the same database.

Two messages were written to the queue in advance. The controller reserves the session, records the launch intent and launches the CLI with the exact ID. MCP cannot read the message or reply until the ID is confirmed by a service event: `thread.started` for Codex, `system/init` for Cursor.

The agent receives the message text via `read_current()` and calls `reply(text)`. The recipient, sender and reply link are determined by the server. The control string is not included in the launch prompt.

While the CLI is running, another process tries to take the same session. It is refused and does not launch an additional CLI. After the main process exits, the controller checks the events, the reply and the ID, marks completion, closes and reopens SQLite, and then runs the second message.

Each experiment ended with two `completed` states, two executions and two replies; the active reservation was released. In total, four real CLI launches were performed, with no automatic retries and no polling of the model while waiting.

## Adapter differences

Codex was launched with a separate configuration, a single MCP, and shell, apps, plugins, hooks, multi-agent and browser/computer use disabled. The checks cover turn completion, absence of errors, ownership of MCP calls and their sequence.

Cursor used a separate test working folder with an MCP configuration, permissions for two tools and prohibitions on shell/file read/file write/web fetch. Before launch, the absence of a global `~/.cursor/mcp.json` and of an overridden `CURSOR_CONFIG_DIR` was checked. `--approve-mcps` was applied only in this controlled environment. Service discovery of MCP tools is acceptable in the log; all other actions must be `read_current` and `reply`.

For Cursor, the IDs of all events in stdout and the result of the MCP call are additionally checked. This check of actually performed actions does not prove that all other tools are absent from the catalog.

The common execution loop was reused from the Claude experiment. The Claude result check was extracted into a separate function; its behavior was verified without a model. There was no new real Claude launch in this work.

## Tests and limits

19 tests passed in 1.360 seconds: the previous 15 checks plus the adapter checks. Negative cases reject a wrong/missing ID, a missing or duplicate final result, an MCP error, a foreign server and unexpected actions. These negative cases are synthetic.

Together with the [Claude experiment](2026-11-05-guarded-claude-mac.md), basic integration of the new mechanism is confirmed for three CLIs on Mac. This is partial H1/H3/H5/H6. Networking, Windows/Linux, real crash recovery, manual bypass of the client, the full process hierarchy and the hour-long H9 remain unverified. The new test is not three agents communicating with each other: it is a separate check of each adapter.

Each run of the test rig has its own database. Different databases do not coordinate the same ID: these probes must not be run simultaneously on one CLI session in different folders. The future client must have a single registry and database per machine.

Raw logs, login data, real IDs and control strings are not published. [Code and instructions](../../experiments/session_safety/README.md). The next stage is safe recovery and a single local client/API for the interface.

Sources for parameters and formats: [Codex non-interactive](https://learn.chatgpt.com/docs/non-interactive-mode), [Cursor CLI parameters](https://cursor.com/docs/cli/reference/parameters), [Cursor permissions](https://cursor.com/docs/cli/reference/permissions).
