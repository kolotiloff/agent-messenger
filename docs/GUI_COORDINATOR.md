# Optional scenario: a graphical Claude window

> **Historical.** Superseded by D-024: the coordinator is the owner's Claude Code chat (in the Claude app's Code tab or a terminal) with Messenger's MCP server ([TASK_MODEL.md](TASK_MODEL.md)).

On November 5, 2026 the user described a personal convenience: talking to the coordinator in the Code tab of the Claude app on the Mac and using their usual dictation. This is a note about a possible setup, not a product requirement, a priority in the main plan, or a release condition.

Messenger remains a universal transport. The coordinator role can belong to Claude, Codex, Cursor or another supported agent, and can be handed between them. The choice of provider and of whichever window is convenient for the human must not dictate the architecture of the broker, the queue or addressing.

If this optional scenario is ever needed, a Desktop-managed session connected to the Messenger transport can be tested separately. Parallel resume is forbidden. Reading mail manually through MCP does not count as automatic delivery. Until a test succeeds, the integration is not promised; the main development does not depend on it.

## What the documentation confirms

The Code tab in Claude Desktop differs from a regular chat. The documentation describes moving a CLI conversation to Desktop via `/desktop`, or continuing a closed CLI session via `/resume` in the app. The history continues in the same session; this path does not move a session that is running in a terminal or in the background. This is a handoff of control, not permission to run one ID under two owners at the same time. [Claude Desktop](https://code.claude.com/docs/en/desktop#coming-from-the-cli).

Remote Control gives the browser and mobile Claude a window into a locally running Claude Code session; the process stays on the computer. This is a separate potential path for live graphical control. [Remote Control](https://code.claude.com/docs/en/remote-control).

The `--remote-control` documentation describes an interactive session, while the current Messenger adapter uses print mode with stream-json. Compatibility of these modes is not confirmed here. One cannot simply add the flag and declare the task solved. [CLI reference](https://code.claude.com/docs/en/cli-reference).

## Hypotheses for a separate test session

1. Can the official graphical viewer attach to a single process managed by Messenger, without a second resume and without losing the MCP contract? Check the exact installed version, launch modes, session identity and the single owner.
2. How are user input from the GUI and agent messages serialized? External input must not mix up a linked reply, interrupt an unfinished delivery or cause an automatic retry.
3. If Desktop becomes the owner, how is Messenger connected as a transport/MCP? Tool availability by itself does not prove that a message can wake an idle session. Event-driven delivery without periodic model calls is required.
4. How can ownership be handed over safely CLI↔Desktop, given the unresolved release within the same Mac boot? A Messenger `shutdown` alone is not enough. Do not remove protective reservations just to open the GUI.
5. How do window close, sleep and connection loss survive? Confirm there is no second process and no re-execution of a message.
6. Check the user's usual dictation in the relevant input field. The built-in voice mode and text dictation are different features; the presence of the former in the chosen Code interface is not promised. Voice recording does not start automatically.

Messenger's own window remains the shared conversation, queue and status panel for all supported agents. Possibly connecting an external graphical window is a separate optional integration. Conclusions about Claude do not automatically carry over to other providers.

At this stage only the documentation has been studied; real GUI sessions, Remote Control and dictation were not turned on. Working chats are not used for experiments.
