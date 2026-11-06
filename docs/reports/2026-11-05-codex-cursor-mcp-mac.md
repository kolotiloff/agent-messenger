# Codex ↔ Cursor: local exchange via MCP on Mac

Date: 2026-11-05. Result: **PASS** for the scenario described below.

## Scenario and evidence

Codex CLI 0.155.1 and Cursor CLI 2026.11.05-86047df were used.
A separate test MCP server in Python using only the standard library provided three tools: `send_message`, `read_messages`, `reply`.
Each server process had a fixed sender; messages were stored in a shared local SQLite database.

1. A new Codex conversation received a random control string and sent it to Cursor via `send_message`. The Codex process exited.
2. A separate Cursor process received only an instruction to check its inbox — with no control string in the prompt. It read the message via `read_messages` and sent `ACK:` with the received string via `reply`.
3. A new Codex process resumed the original conversation by exact ID. The prompt contained only a notification that a reply was available. Via `read_messages`, Codex received and returned Cursor's exact reply.

All three runs exited with code 0. The Codex conversation ID before and after resuming matched. The database contains exactly two messages; the reply is linked to the original message. The server log contains the sequence:

```text
codex  send_message
cursor read_messages
cursor reply
codex  read_messages
```

The CLI JSONL logs additionally confirmed successful tool calls and turn completion. Cursor also used its internal MCP tool discovery. Before running models, a separate protocol check confirmed initialization, the tool list, delivery, no duplicate on a repeated key, and refusal to reply to a message outside its own inbox.

## Scope of the result

This is an experimental local MCP + SQLite setup, not a finished Messenger. The runs were executed sequentially by a test controller; automatic event-driven wake-up is not implemented yet. The reply content was not manually copied into the Codex prompt.

Part of H5 is confirmed: headless MCP calls in Codex/Cursor and message persistence across separate processes. The check of the local idempotency key does not close H8 with a lost network ACK and a broker restart.

Not verified: Claude in this exchange; Windows/Linux; VPS and network; concurrent session owners; crash recovery; access revocation; coordinator role handover; permission denial; long waits without calling the model. Production authorization between machines is absent here.

The test does not measure token savings. Models were run for three substantive turns; polling the model while waiting for a reply was not used. This does not allow concluding that reading and resuming saved history is free.

## Restricting permissions

For Codex, a separate launch configuration with a single test MCP was used; shell, apps, plugins, hooks and subagents were disabled. For Cursor, a separate test folder was created with local MCP settings and permissions for the three tools, with shell/file read/file write and web fetch denied. At launch time there was no global `~/.cursor/mcp.json`; `--approve-mcps` was used for this test set. This flag must not be carried over without verification into an environment with other servers.

The test used existing CLI authorization. Raw logs, session IDs, control strings and account details are not included in this report.

## Next step

Repeat the exchange with Claude, then separately verify locking of a single session and handling of an uncertain result on failure. After that, move on to a minimal broker and clients.

Configuration sources: [Cursor permissions](https://cursor.com/docs/cli/reference/permissions), [Codex MCP](https://developers.openai.com/codex/mcp), [MCP stdio](https://modelcontextprotocol.io/specification/2025-06-18/basic/transports).
