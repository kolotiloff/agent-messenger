# Basic Codex CLI resume check on Mac

Status: success.

The first CLI process created a separate test conversation and received a random marker. After it exited, a second process resumed the conversation by exact ID via `codex exec resume`. The marker was not passed in the second request.

- The conversation ID was preserved.
- The control information was recovered exactly.
- Both processes exited successfully.
- There were no tool calls in the events of either run.
- No working conversations were used; the test folder did not require a Git repository.

This confirms the basic part of H3 on Mac. MCP, parallel runs, a wrong ID, failures and long waits have not been verified yet. The Messenger client is not implemented yet. Messenger savings were not measured by this experiment.

Account data, the conversation ID, the control phrase and usage metrics are not included in the public report.
