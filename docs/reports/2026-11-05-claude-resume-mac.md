# Basic check of Claude CLI resume on Mac

Status: success.

Two sequential CLI processes continued one test conversation by exact ID. The second process recovered the control information from the first request without it being sent again.

This confirms the basic mechanism for saving and continuing history. Working conversations were not used.

MCP, parallel launches, a wrong ID, recovery after failures and waiting remain unverified. The Messenger client is not yet implemented; its restart was not tested by this experiment. Messenger savings were not measured.

Account data, the conversation ID, the control phrase and usage metrics are not included in the public report.
