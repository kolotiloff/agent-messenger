# Basic check of Cursor CLI resume on Mac

Status: saving and resuming a test conversation confirmed.

A separate empty CLI conversation was created. The first process received a random marker and replied READY. After the process exited, a second launch with the same ID via `--resume` exactly recovered the marker, which was absent from the second request.

- Both launches completed successfully.
- The conversation ID was preserved.
- There were no tool calls in the event stream.
- print/stream-json, ask mode and enabled execution sandboxing were used.
- Working chats and project files were not used.

## Observation when creating the conversation

The first `create-chat` attempt with output capture did not finish within 30 seconds and was stopped by timeout. A repeat attempt in terminal mode returned an ID and completed successfully. Both subsequent model launches with output capture succeeded. The cause of the first delay was not established; this experiment does not support concluding that create-chat necessarily requires a terminal.

## Check boundaries

This is a preliminary check of the Cursor CLI mechanism on Mac. H2 for Windows remains unverified. Resuming a GUI chat, MCP, concurrent launches, a wrong ID, failures, restarting the future Messenger client and long waits have not been verified yet. Messenger's savings were not measured.

Account data, the conversation ID, the control phrase and usage metrics are not included in the report.
